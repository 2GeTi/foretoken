# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright contributors to the Foretoken project

"""Serve authenticated, read-only capture files from a mounted RuntimeCache."""

from __future__ import annotations

import hashlib
import hmac
import json
import os
import shutil
import stat
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Final
from urllib.parse import parse_qs, urlsplit
from uuid import UUID

READER_PORT: Final = 8080
CAPTURE_MOUNT_PATH: Final = "/captures"
READER_SECRET_ENV: Final = "FORETOKEN_PROFILE_READER_SECRET"


def capture_path(value: str) -> str:
    """Accept only the controller's retained capture root, not arbitrary PVC paths."""
    parts = value.split("/")
    if len(parts) != 3 or parts[:2] != ["profiles", "runs"]:
        raise ValueError("invalid capture path")
    if str(UUID(parts[2])) != parts[2]:
        raise ValueError("invalid capture identity")
    return value


def capture_token(secret: str, path: str) -> str:
    """Bind a reader access credential to one controller-selected capture root."""
    return hmac.new(secret.encode(), path.encode(), hashlib.sha256).hexdigest()


def _open_directory(root_fd: int, parts: list[str]) -> int:
    """Walk beneath an open volume without following replaceable symlinks."""
    current = os.dup(root_fd)
    try:
        for part in parts:
            following = os.open(
                part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=current
            )
            os.close(current)
            current = following
        return current
    except BaseException:
        os.close(current)
        raise


def _trace_name(value: str) -> tuple[str, str]:
    """Validate a runtime-relative exported trace or manifest filename."""
    parts = value.split("/")
    if len(parts) != 2 or str(UUID(parts[0])) != parts[0]:
        raise ValueError("invalid trace path")
    filename = parts[1]
    if filename in {"", ".", ".."} or "\\" in filename or "\x00" in filename:
        raise ValueError("invalid trace filename")
    if filename != "manifest.json" and not filename.endswith(".pt.trace.json"):
        raise ValueError("not a capture file")
    return parts[0], filename


class CaptureReader(ThreadingHTTPServer):
    """Own the mounted root descriptor and process-local authentication secret."""

    def __init__(self, address: tuple[str, int], root: str, secret: str) -> None:
        self.root_fd = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        self.secret = secret
        try:
            super().__init__(address, CaptureHandler)
        except BaseException:
            os.close(self.root_fd)
            raise

    def server_close(self) -> None:
        """Release sockets and the volume descriptor when the reader stops."""
        try:
            super().server_close()
        finally:
            os.close(self.root_fd)


class CaptureHandler(BaseHTTPRequestHandler):
    """Expose only run-scoped listings and regular exported files; never log credentials."""

    server: CaptureReader

    def log_message(self, format: str, *args: object) -> None:
        """Keep request URLs and their access credentials out of Pod logs."""

    def _json(self, status: int, value: object) -> None:
        body = json.dumps(value).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        """Authenticate the capture scope before listing or streaming its files."""
        request = urlsplit(self.path)
        query = parse_qs(request.query)
        try:
            path = capture_path(query.get("run", [""])[0])
        except ValueError:
            self._json(400, {"error": "invalid_capture"})
            return
        expected = capture_token(self.server.secret, path)
        if not hmac.compare_digest(query.get("token", [""])[0], expected):
            self._json(403, {"error": "forbidden"})
            return
        if request.path not in {"/list", "/file"}:
            self._json(404, {"error": "not_found"})
            return
        try:
            root = _open_directory(self.server.root_fd, path.split("/"))
            try:
                if request.path == "/list":
                    self._json(200, {"files": self._files(root)})
                else:
                    self._file(root, query.get("name", [""])[0])
            finally:
                os.close(root)
        except (BrokenPipeError, ConnectionResetError):
            return
        except FileNotFoundError:
            # A successful list response distinguishes missing files from an API proxy 404.
            self._json(200 if request.path == "/list" else 404, {"error": "not_found"})
        except ValueError:
            self._json(400, {"error": "invalid_file"})
        except OSError:
            self._json(200 if request.path == "/list" else 403, {"error": "unreadable"})

    def _files(self, root: int) -> list[dict[str, str | int]]:
        """List regular PyTorch traces under runtime directories without following links."""
        files = []
        for runtime in sorted(os.listdir(root)):
            try:
                if str(UUID(runtime)) != runtime:
                    continue
            except ValueError:
                continue
            directory = _open_directory(root, [runtime])
            try:
                for name in sorted(os.listdir(directory)):
                    if not name.endswith(".pt.trace.json"):
                        continue
                    info = os.stat(name, dir_fd=directory, follow_symlinks=False)
                    if stat.S_ISREG(info.st_mode):
                        files.append(
                            {"name": f"{runtime}/{name}", "size": info.st_size}
                        )
            finally:
                os.close(directory)
        return files

    def _file(self, root: int, name: str) -> None:
        """Stream a regular capture file using descriptors that cannot escape its run."""
        runtime, filename = _trace_name(name)
        directory = _open_directory(root, [runtime])
        try:
            descriptor = os.open(
                filename, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=directory
            )
        finally:
            os.close(directory)
        with os.fdopen(descriptor, "rb") as source:
            info = os.fstat(source.fileno())
            if not stat.S_ISREG(info.st_mode):
                raise ValueError("not a regular capture file")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(info.st_size))
            self.send_header("Cache-Control", "no-store")
            self.end_headers()
            try:
                shutil.copyfileobj(source, self.wfile, length=1024 * 1024)
            except OSError:
                # Headers already describe the trace; never append a JSON error to its bytes.
                self.close_connection = True


def main() -> None:
    """Run the packaged reader inside the temporary read-only PVC Pod."""
    secret = os.environ[READER_SECRET_ENV]
    with CaptureReader(("0.0.0.0", READER_PORT), CAPTURE_MOUNT_PATH, secret) as server:
        server.serve_forever()


if __name__ == "__main__":
    main()
