<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# Use Foretoken on a restricted network

[中文](restricted-network_zh.md)

Foretoken keeps network selection with the tool that owns each download. One OCI registry option covers release images and CLI-managed Helm charts. pip, uv, Git, Go, and Cargo continue using their native configuration, which the source build forwards into its containers.

## Configure the build host once

Save the endpoints provided by the network or registry administrator in a shell file, then source it before installing or building Foretoken:

```bash
export FORETOKEN_OCI_REGISTRY=registry.example.com/mirror
export FORETOKEN_GITHUB_MIRROR=https://source.example.com/github.com
export PIP_INDEX_URL=https://python.example.com/simple
export UV_DEFAULT_INDEX=https://python.example.com/simple
# Set UV_EXTRA_INDEX_URL when a hardware-specific build needs another package index.
export GOPROXY=https://go.example.com
export GOSUMDB=sum.example.com
export CARGO_REGISTRIES_CRATES_IO_INDEX=sparse+https://cargo.example.com/index/
export CARGO_NET_GIT_FETCH_WITH_CLI=true
```

These values are endpoints, not credentials. Use each tool's native credential configuration on the host, and keep tokens out of repository files and endpoint URLs. Source builds forward the endpoints but not host credential files, so build-container access must come from the network or secret mounts managed by the build system.

Without these settings, every command continues to use its public upstream source. When a mirror is set, Foretoken uses it directly and does not probe or fall back to a public source.

## Install a release

Install the Python package through the configured pip or uv index, then install the platform normally:

```bash
python -m pip install foretoken
foretoken install
```

`FORETOKEN_OCI_REGISTRY` replaces the registry host for release images. CLI-managed OCI charts are read from the same registry prefix with their original source host retained in the chart path. For example, the Foretoken chart is read from:

```text
oci://registry.example.com/mirror/ghcr.io/shiweijiezero/foretoken/charts/foretoken
```

The same rule covers the Prometheus, Envoy Gateway, MetalLB, and DCGM Exporter charts selected by `foretoken install`. Their images use the configured registry prefix while retaining each image repository. Mirror the charts and images before installation, sign in with `helm registry login` and `docker login` when required, and create Kubernetes `imagePullSecrets` for private runtime images.

A command-line value overrides the environment for one installation:

```bash
foretoken install --oci-registry registry.example.com/mirror
```

A missing mirrored chart fails through Helm. A missing runtime image remains visible as the Kubernetes image-pull error; Foretoken does not silently switch registries.

## Build from source

A GitHub-compatible archive mirror can provide the Foretoken checkout before the CLI is installed:

```bash
curl --fail --location \
  --output foretoken.tar.gz \
  "$FORETOKEN_GITHUB_MIRROR/shiweijiezero/foretoken/archive/refs/heads/main.tar.gz"
mkdir foretoken

tar --extract --gzip --strip-components=1 \
  --file foretoken.tar.gz --directory foretoken
cd foretoken
python -m pip install -e .
foretoken install -e .
```

`foretoken install -e .` uses the OCI mirror for default builder and runtime images. It also forwards the configured GitHub mirror, uv index, Go proxy and checksum database, and Cargo registry and Git settings to the owning build tools. `--registry` remains the explicit destination used to distribute the images built from source.

`FORETOKEN_GITHUB_MIRROR` replaces `https://github.com` for Foretoken submodules, Cargo Git dependencies, and maintained source-archive builds. The mirror must preserve the remaining owner, repository, and archive path.

## Offline Kubernetes nodes

If the nodes cannot reach any registry, build the three Foretoken images on a connected host and import them into every eligible node. Use the existing [manual image import workflow](development/source-image-lifecycle.md#import-local-images-directly); the CLI's kind and k3d source workflow uses the same local image-import path.

This guide does not configure a host-wide Docker or containerd mirror, install k3s, or replace operating-system package repositories used inside base images. Those remain responsibilities of the host, Kubernetes distribution, or base image administrator.
