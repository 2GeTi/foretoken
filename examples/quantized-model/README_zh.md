<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# 部署量化模型

[English](README.md) | [中文](README_zh.md)

本示例通过 Foretoken 部署 Qwen 官方的 [Qwen2.5-0.5B-Instruct-AWQ](https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-AWQ)，加载采用 AWQ 4-bit 量化的权重。示例使用已有量化模型，不执行训练或模型量化。

## 准备环境

按照仓库[快速开始](../../README_zh.md)安装平台。使用已安装的 vLLM 运行时及其 [AWQ 实现](https://docs.vllm.ai/en/stable/features/quantization/)均支持的 NVIDIA GPU，例如 A100。其他量化格式和加速器是否可用，取决于运行时镜像。

示例共申请 1 张 GPU、3 核 CPU 和 9 GiB 主机内存，平台还需要额外资源。这些数值是调度申请量，不是实测内存需求。GPU 显存还要容纳激活值和 KV cache；权重采用 4-bit 不代表整个推理过程都使用 4-bit。

按照[模型存储](../../docs/model-storage_zh.md)准备缓存。`cache.yaml` 与快速开始共用仓库根目录的 `data/`。连接远程集群时，将 `spec.directory` 改为目标节点可访问的绝对路径。网络受限时，参见[模型来源](../../docs/model-sources_zh.md)。

## 部署并发送请求

在仓库根目录执行：

```bash
foretoken deploy examples/quantized-model --timeout 20m
FRONTEND_URL="$(foretoken endpoint examples/quantized-model)"

curl --fail-with-body "$FRONTEND_URL/v1/chat/completions" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "Qwen/Qwen2.5-0.5B-Instruct-AWQ",
    "messages": [{"role": "user", "content": "Explain quantization in one sentence."}],
    "max_tokens": 64,
    "temperature": 0
  }'
```

部署命令等待 `foretoken-quantized-demo` 命名空间中的服务就绪。请求成功后返回量化模型生成的聊天补全结果。使用 Gateway 模式时，按根目录快速开始配置前端域名和请求的 Host 头。

## 更换模型

修改 [`model.yaml`](model.yaml)，并在请求中使用相同的模型标识。示例在 `spec.extraArgs` 中通过 `--quantization=awq` 显式选择 AWQ，通过 `--dtype=half` 使用 FP16 激活值，通过 `--max-model-len=4096` 将上下文长度设为 4096 个 token。

如果模型自带 vLLM 能识别的量化元数据，可以省略 `--quantization`，让引擎选择实现。切换量化格式时，应根据模型和运行时要求修改或移除 AWQ 与 dtype 参数。给普通权重添加 `--quantization=awq` 不会将其转换为 AWQ 模型。每个附加参数应写成独立的 `--name=value` 字符串。

请求成功仅证明服务可以推理，不代表质量或速度达标。选择量化模型前，应在目标 GPU 上比较任务质量，测量延迟、吞吐和显存占用，参见[基准测试](../../benchmarks/README_zh.md)。

## 清理

```bash
foretoken delete examples/quantized-model
```

该命令删除示例命名空间及服务资源。目录缓存中的已下载文件会保留，供后续部署复用。
