<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# 在受限网络中使用 Foretoken

[English](restricted-network.md)

Foretoken 默认使用公共官方源。源码构建可以自动选择更快的匿名镜像；用户显式配置的 registry 和包源始终优先。

## 安装发布版本

可以访问 PyPI 和 GHCR 时，直接安装 Python 包和 Kubernetes 平台：

```bash
python -m pip install foretoken
foretoken install
```

Foretoken 必须先下载 CLI 包，才能执行来源选择。构建机无法访问 PyPI 时，应先配置 `PIP_INDEX_URL` 或对应的 uv 设置。

目前没有经过核实、同时包含全部 Foretoken 发布镜像和 CLI 管理的 Helm OCI Chart 的公共镜像。无法访问 GHCR 时，先把发布制品同步到一个 OCI registry，再统一配置：

```bash
export FORETOKEN_OCI_REGISTRY=registry.example.com/mirror
foretoken install
```

如需只覆盖一次安装，可使用命令行参数：

```bash
foretoken install --oci-registry registry.example.com/mirror
```

OCI Chart 在镜像库中保留原来源 host。例如 Foretoken Chart 的地址为：

```text
oci://registry.example.com/mirror/ghcr.io/shiweijiezero/foretoken/charts/foretoken
```

发布镜像会将原 registry host 替换为该前缀。同一配置也覆盖 Prometheus、Envoy Gateway、MetalLB 和 DCGM Exporter 的 Chart 与镜像。安装前需要先同步这些制品；私有 registry 使用 `helm registry login`、`docker login` 和 Kubernetes `imagePullSecrets`。Foretoken 不会探测显式配置的 registry，也不会回退到公共源。

## 从源码构建

获取源码并安装 CLI 后，通过现有安装命令构建平台镜像：

```bash
git clone https://github.com/shiweijiezero/foretoken.git
cd foretoken
python -m pip install -e .
foretoken install -e .
```

editable 构建开始前，Foretoken 会并行读取真实包、源码归档或 OCI manifest 中最多 64 KiB 的内容。每个探测的连接、响应和传输预算为 4 秒，整组探测并行完成且不重试。官方源不可用，或匿名候选同时快至少 30% 和 250 ms 时，才会选择候选；否则继续使用官方源。

自动候选如下：

| 来源 | 匿名候选 |
| --- | --- |
| Docker Hub、GCR 和 GHCR 构建镜像 | DaoCloud 公共镜像 |
| 镜像内部安装的 PyPI 包 | 清华 TUNA |
| Go module | GOPROXY.CN |
| crates.io 包 | RSProxy |

GitHub 源码归档、Git 依赖和 Helm OCI Chart 继续使用官方源，因为当前没有启用与这些制品兼容的公共候选。需要时应显式配置。

构建前会打印被选中的镜像及测量耗时。如果官方源和候选在探测预算内均不可用，正常构建仍可复用已有本地缓存；构建最终失败时，错误会列出来源选择阶段不可用的服务。

## 显式配置构建来源

只保存当前网络需要替换的地址，并在构建前加载：

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

`FORETOKEN_GITHUB_MIRROR` 会替换 Foretoken submodule、Cargo Git 依赖和项目维护的源码归档构建中的 `https://github.com`。镜像服务需要保留后续的组织、仓库和归档路径。只有硬件专用构建还需要其他包索引时，才设置 `UV_EXTRA_INDEX_URL`。

这些值只包含服务地址，不包含凭据。主机工具继续使用各自的原生凭据配置。源码构建只传递服务地址，不复制主机凭据文件；构建容器的认证访问应由网络策略或构建系统的 secret mount 提供。匿名探测使用直连 HTTPS，不携带 proxy、registry、包源或源码凭据。

`--registry` 仍用于显式指定源码构建镜像的分发位置，与获取构建输入的 registry 相互独立。

## Kubernetes 节点完全离线

如果节点无法访问任何 registry，可在联网构建机上构建三个 Foretoken 镜像，再导入所有可能运行工作负载的节点。使用现有的[手动镜像导入流程](development/source-image-lifecycle_zh.md#直接导入本地镜像)；CLI 面向 kind 和 k3d 的源码流程复用同一类本地镜像导入路径。

Foretoken 不配置宿主机级 Docker 或 containerd mirror，不安装 k3s，也不替换基础镜像内部使用的操作系统软件源。这些配置分别由宿主机、Kubernetes 发行版或基础镜像管理员负责。
