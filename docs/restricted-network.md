<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# Use Foretoken on a restricted network

[中文](restricted-network_zh.md)

Foretoken uses public upstream sources by default. Source builds can select a faster anonymous mirror automatically, while explicit registry and package settings always take precedence.

## Install a release

Install the Python package and Kubernetes platform normally when PyPI and GHCR are reachable:

```bash
python -m pip install foretoken
foretoken install
```

The CLI package must be downloaded before Foretoken can perform source selection. Configure `PIP_INDEX_URL` or the corresponding uv setting when the build host cannot reach PyPI.

There is no verified public mirror that carries every Foretoken release image and CLI-managed Helm OCI chart. If GHCR is unavailable, mirror the release artifacts into an OCI registry and configure it once:

```bash
export FORETOKEN_OCI_REGISTRY=registry.example.com/mirror
foretoken install
```

A command-line value overrides the environment for one installation:

```bash
foretoken install --oci-registry registry.example.com/mirror
```

The registry keeps each OCI chart under its original source host. For example, the Foretoken chart is read from:

```text
oci://registry.example.com/mirror/ghcr.io/shiweijiezero/foretoken/charts/foretoken
```

Release image registry hosts are replaced with the configured prefix. The same configuration covers the Prometheus, Envoy Gateway, MetalLB, and DCGM Exporter charts and their images. Mirror these artifacts before installation. Private registries use `helm registry login`, `docker login`, and Kubernetes `imagePullSecrets`; Foretoken does not probe an explicitly configured registry or fall back to a public source.

## Build from source

Get the source, install the CLI, and build the platform images through the existing installation command:

```bash
git clone https://github.com/shiweijiezero/foretoken.git
cd foretoken
python -m pip install -e .
foretoken install -e .
```

Before an editable build starts, Foretoken concurrently reads up to 64 KiB from real package, archive, or OCI manifest resources. Each probe has a four-second connection, response, and transfer budget, and the complete group runs in parallel. The official source remains selected unless it is unavailable or an anonymous candidate is both at least 30% faster and 250 ms faster. Probes are not retried.

The automatic candidates are:

| Source | Anonymous candidate |
| --- | --- |
| Docker Hub, GCR, and GHCR build images | DaoCloud public image mirror |
| PyPI packages installed inside images | Tsinghua TUNA |
| Go modules | GOPROXY.CN |
| crates.io packages | RSProxy |
| GitHub archives and public Git sources | ghproxy.net |

Helm OCI charts stay on their official sources because no compatible public candidate is enabled for those artifacts. Configure an OCI registry explicitly when necessary.

Selected mirrors and measured times are printed before the build. If both the official source and candidate are unavailable during the bounded probe, the normal build can still use an existing local cache; a failed build reports which sources were unavailable during selection.

## Configure explicit build sources

Save only the endpoints required by the network, then source the file before building:

```bash
export FORETOKEN_OCI_REGISTRY=registry.example.com/mirror
export FORETOKEN_GITHUB_MIRROR=https://source.example.com/github.com
export PIP_INDEX_URL=https://python.example.com/simple
export UV_DEFAULT_INDEX=https://python.example.com/simple
export GOPROXY=https://go.example.com
export GOSUMDB=sum.example.com
export FORETOKEN_CARGO_REGISTRY=sparse+https://cargo.example.com/index/
export CARGO_NET_GIT_FETCH_WITH_CLI=true
```

`FORETOKEN_GITHUB_MIRROR` replaces `https://github.com` for Foretoken submodules, Cargo Git dependencies, and maintained source-archive builds. The mirror must preserve the remaining owner, repository, and archive path. Set `UV_EXTRA_INDEX_URL` only when a hardware-specific build requires another package index.

These values contain endpoints, not credentials. Host tools keep their native credential configuration. Source builds forward endpoints but not host credential files, so authenticated build-container access must come from network policy or secret mounts managed by the build system. Anonymous probes use direct HTTPS requests without proxy, registry, package, or source credentials.

`--registry` remains the explicit destination for images built from source; it is separate from the registries used to obtain build inputs.

## Download Hugging Face models

`source: hf` keeps the model, tokenizer, configuration, and chat-template lifecycle introduced by the model-source API. Foretoken keeps the official Hub unless a platform endpoint is configured. The public mirror candidate tested for automatic selection served model metadata, configuration, tokenizer files, and weight bytes, but failed the complete ETag-based download contract used by the frontend client, so it is not enabled as an automatic source.

Configure a Hugging Face-compatible endpoint that has been verified with the deployed frontend and inference engine:

```yaml
runtime:
  vllm:
    modelSource:
      endpoint: https://hub.example.com
```

Use the existing `tokenSecret` configuration together with an explicit endpoint for authenticated repositories. Foretoken does not send that credential to anonymous source probes. `source: local` and `source: modelscope` remain independent and never fall back to Hugging Face.

## Offline Kubernetes nodes

If the nodes cannot reach any registry, build the three Foretoken images on a connected host and import them into every eligible node. Use the existing [manual image import workflow](development/source-image-lifecycle.md#import-local-images-directly); the CLI's kind and k3d source workflow uses the same local image-import path.

Foretoken does not configure a host-wide Docker or containerd mirror, install k3s, or replace operating-system package repositories inside base images. Those remain responsibilities of the host, Kubernetes distribution, or base image administrator.
