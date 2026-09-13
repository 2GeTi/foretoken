<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# 在受限网络中使用 Foretoken

[English](restricted-network.md)

Foretoken 将网络来源交给实际执行下载的工具管理。一个 OCI registry 配置同时覆盖发布镜像和 CLI 管理的 Helm Chart；pip、uv、Git、Go 和 Cargo 继续使用各自的原生配置，源码构建会把这些配置传入构建容器。

## 一次配置构建主机

将网络或制品库管理员提供的地址保存为 shell 文件，在安装或构建 Foretoken 前加载：

```bash
export FORETOKEN_OCI_REGISTRY=registry.example.com/mirror
export FORETOKEN_GITHUB_MIRROR=https://source.example.com/github.com
export PIP_INDEX_URL=https://python.example.com/simple
export UV_DEFAULT_INDEX=https://python.example.com/simple
# 硬件专用构建需要其他包索引时，再设置 UV_EXTRA_INDEX_URL。
export GOPROXY=https://go.example.com
export GOSUMDB=sum.example.com
export CARGO_REGISTRIES_CRATES_IO_INDEX=sparse+https://cargo.example.com/index/
export CARGO_NET_GIT_FETCH_WITH_CLI=true
```

这些值只填写服务地址，不填写凭据。主机侧认证由各工具的原生凭据配置管理，不要把 token 写入仓库文件或地址。源码构建只传递服务地址，不复制主机凭据文件；构建容器所需访问权限应由网络策略或构建系统的 secret mount 提供。

未设置这些变量时，所有命令继续使用默认公共源。显式设置镜像源后，Foretoken 会直接使用该来源，不会探测或回退到公共源。

## 安装发布版本

先通过配置好的 pip 或 uv 源安装 Python 包，再按默认命令安装平台：

```bash
python -m pip install foretoken
foretoken install
```

`FORETOKEN_OCI_REGISTRY` 会替换发布镜像的 registry host。CLI 管理的 OCI Chart 也使用同一 registry 前缀，并在 Chart 路径中保留原来源 host。例如 Foretoken Chart 的地址为：

```text
oci://registry.example.com/mirror/ghcr.io/shiweijiezero/foretoken/charts/foretoken
```

同一规则会覆盖 `foretoken install` 选择的 Prometheus、Envoy Gateway、MetalLB 和 DCGM Exporter Chart；这些 Chart 使用的镜像会改为配置的 registry 前缀，并保留各自的 repository。安装前需要先同步 Chart 和镜像；私有 registry 按需使用 `helm registry login`、`docker login`，并为 Kubernetes 运行时镜像创建 `imagePullSecrets`。

如需只覆盖一次安装，可使用命令行参数：

```bash
foretoken install --oci-registry registry.example.com/mirror
```

镜像库中缺少 Chart 时，Helm 会直接报错。缺少运行时镜像时，Kubernetes 会保留实际的 image pull 错误；Foretoken 不会静默切换 registry。

## 从源码构建

在安装 CLI 之前，可以从兼容 GitHub 归档路径的镜像下载 Foretoken 源码：

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

`foretoken install -e .` 会使用 OCI 镜像库中的默认构建和运行基础镜像，并将 GitHub 镜像、uv 包索引、Go proxy 与 checksum database、Cargo registry 与 Git 配置传给对应的构建工具。`--registry` 仍用于显式指定源码构建镜像的分发位置。

`FORETOKEN_GITHUB_MIRROR` 会替换 Foretoken submodule、Cargo Git 依赖和项目维护的源码归档构建中的 `https://github.com`。镜像服务需要保留后续的组织、仓库和归档路径。

## Kubernetes 节点完全离线

如果节点无法访问任何 registry，可在联网构建机上构建三个 Foretoken 镜像，再导入所有可能运行工作负载的节点。使用现有的[手动镜像导入流程](development/source-image-lifecycle_zh.md#直接导入本地镜像)；CLI 面向 kind 和 k3d 的源码流程复用同一类本地镜像导入路径。

本指南不配置宿主机级 Docker 或 containerd mirror，不安装 k3s，也不替换基础镜像内部使用的操作系统软件源。这些配置分别由宿主机、Kubernetes 发行版或基础镜像管理员负责。
