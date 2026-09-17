# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright contributors to the Foretoken project

"""Serve retained capture history on loopback using the caller's cluster access."""

from __future__ import annotations

import json
import secrets
import socket
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from importlib import resources
from typing import Any
from urllib.parse import parse_qs, urlsplit

from foretoken.arguments import ProfileViewCommand
from foretoken.kubernetes import Kubectl, load_deployment, timeout_seconds
from foretoken.manifest import DeploymentError
from foretoken.profile_storage import ProfileStorage


class ProfileHistory:
    """Select retained runs by deployment identity without requiring serving Pods."""

    def __init__(self, kubectl: Kubectl, path: str) -> None:
        self.kubectl = kubectl
        deployment = load_deployment(path, kubectl)
        self.namespace = deployment.namespace
        self.services = frozenset(deployment.models)
        self.runs: dict[str, dict[str, Any]] = {}

    def refresh(self) -> list[dict[str, Any]]:
        """Return newest-first history; model names come only from capture snapshots."""
        records = self.kubectl.list_resources(
            ("profileruns.inference.foretoken.io",), self.namespace
        )
        self.runs = {
            run["metadata"]["uid"]: run
            for run in records
            if run["spec"]["modelServiceRef"]["name"] in self.services
        }
        entries = []
        for uid, run in self.runs.items():
            metadata, spec, status = run["metadata"], run["spec"], run.get("status", {})
            entries.append(
                {
                    "id": uid,
                    "name": metadata["name"],
                    "service": spec["modelServiceRef"]["name"],
                    "model": (status.get("plan") or {}).get("model"),
                    "time": status.get("startedAt") or metadata["creationTimestamp"],
                    "status": status.get("phase", "Pending"),
                    "has_result": bool(status.get("artifact")),
                }
            )
        return sorted(
            entries, key=lambda entry: (entry["time"], entry["id"]), reverse=True
        )

    def artifact(self, uid: str) -> tuple[str, str, str]:
        """Resolve only runs listed for this deployment, never browser-supplied PVCs."""
        run = self.runs.get(uid)
        if run is None:
            raise FileNotFoundError("Capture is no longer listed; refresh the history.")
        artifact = run.get("status", {}).get("artifact")
        if not artifact:
            raise FileNotFoundError("This capture has no published result.")
        return self.namespace, artifact["claimName"], artifact["path"]


class ProfileViewer(ThreadingHTTPServer):
    """Accept browser preconnections while serializing cluster access and draining on exit."""

    daemon_threads = False

    def __init__(self, history: ProfileHistory, storage: ProfileStorage) -> None:
        self.cluster_lock = threading.Lock()
        self.connection_lock = threading.Lock()
        self.connections: set[socket.socket] = set()
        self.closing = threading.Event()
        self.history = history
        self.storage = storage
        self.session_path = "/" + secrets.token_urlsafe(24) + "/"
        self.page = (
            resources.files("foretoken").joinpath("profile_view.html").read_bytes()
        )
        super().__init__(("127.0.0.1", 0), ProfileHandler)
        self.origin = f"http://127.0.0.1:{self.server_port}"

    def get_request(self) -> tuple[socket.socket, Any]:
        """Track accepted sockets so idle browser connections cannot prevent shutdown."""
        connection, address = super().get_request()
        with self.connection_lock:
            self.connections.add(connection)
        return connection, address

    def shutdown_request(self, request: socket.socket) -> None:
        """Forget completed connections before the server drains its remaining threads."""
        with self.connection_lock:
            self.connections.discard(request)
        super().shutdown_request(request)

    def server_close(self) -> None:
        """Close browser connections and join request handlers before reader cleanup."""
        self.closing.set()
        with self.connection_lock:
            for connection in self.connections:
                try:
                    connection.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass  # The browser may already have disconnected.
        super().server_close()


class ProfileHandler(BaseHTTPRequestHandler):
    """Expose deployment history and authenticated trace bytes to the local browser."""

    server: ProfileViewer

    def log_message(self, format: str, *args: object) -> None:
        # URLs contain the local session credential.
        pass

    def _headers(
        self, status: int, content_type: str, length: int | None = None
    ) -> None:
        self.send_response(status)
        if length is not None:
            self.send_header("Content-Length", str(length))
        self.send_header("Content-Type", content_type)
        self.send_header("Cache-Control", "no-store")
        self.send_header("Referrer-Policy", "no-referrer")
        self.send_header("X-Content-Type-Options", "nosniff")
        self.send_header(
            "Content-Security-Policy",
            "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; frame-src https://ui.perfetto.dev; frame-ancestors 'none'; base-uri 'none'; form-action 'none'",
        )
        self.end_headers()

    def _json(self, status: int, value: object) -> None:
        self._headers(status, "application/json; charset=utf-8")
        self.wfile.write(json.dumps(value).encode())

    def do_GET(self) -> None:
        """Handle same-origin viewer requests and stream one selected trace."""
        # Reject DNS rebinding and cross-origin requests before acquiring cluster resources.
        if self.headers.get("Host") != urlsplit(self.server.origin).netloc:
            self.send_error(403)
            return
        if self.headers.get("Origin", self.server.origin) != self.server.origin:
            self.send_error(403)
            return
        parsed = urlsplit(self.path)
        if not parsed.path.startswith(self.server.session_path):
            self.send_error(404)
            return
        with self.server.cluster_lock:
            if self.server.closing.is_set():
                return
            try:
                self._serve(parsed.path[len(self.server.session_path) :], parsed.query)
            except (BrokenPipeError, ConnectionResetError):
                self.close_connection = True

    def _serve(self, route: str, query_string: str) -> None:
        """Handle one authorized request with exclusive access to the capture inventory."""
        streaming = False
        try:
            if not route:
                self._headers(200, "text/html; charset=utf-8")
                self.wfile.write(self.server.page)
            elif route == "api/runs":
                self._json(200, self.server.history.refresh())
            elif route in {"api/files", "api/trace"}:
                query = parse_qs(query_string)
                uid = query.get("run", [""])[0]
                artifact = self.server.history.artifact(uid)
                files = self.server.storage.list_files(*artifact)
                if route == "api/files":
                    self._json(200, files)
                    return
                name = query.get("file", [""])[0]
                selected = next((item for item in files if item["name"] == name), None)
                if selected is None:
                    raise FileNotFoundError("Trace file is no longer available.")
                self._headers(200, "application/octet-stream", selected["size"])
                streaming = True
                self.server.storage.stream_file(*artifact, name, self.wfile)
            else:
                self.send_error(404)
        except (FileNotFoundError, DeploymentError) as error:
            if streaming:
                # A partial trace must not be followed by a JSON error body.
                self.close_connection = True
            else:
                self._json(
                    404 if isinstance(error, FileNotFoundError) else 503,
                    {"error": str(error)},
                )


def view(command: ProfileViewCommand) -> None:
    """Print a loopback viewer URL and retain readers until the user exits."""
    if timeout_seconds(command.timeout) <= 0:
        raise DeploymentError("--timeout must be positive")
    kubectl = Kubectl()
    # A long-lived viewer must not switch clusters when another terminal changes context.
    context = kubectl.run(["config", "current-context"]).stdout.strip()
    kubectl = Kubectl(context=context)
    history = ProfileHistory(kubectl, command.kustomize_path)
    history.refresh()
    with (
        ProfileStorage(kubectl, timeout=command.timeout) as storage,
        ProfileViewer(history, storage) as server,
    ):
        print(f"Profile viewer: {server.origin}{server.session_path}", flush=True)
        print("Press Ctrl+C to stop.", flush=True)
        server.serve_forever()
