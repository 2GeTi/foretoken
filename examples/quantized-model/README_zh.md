<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# 部署量化模型

[English](README.md) | [中文](README_zh.md)

本示例加载 Qwen 官方的 [Qwen2.5-0.5B-Instruct-AWQ](https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-AWQ) 预量化模型，使用 AWQ 4-bit 权重、FP16 激活值和 4096-token 上下文。

按仓库[快速开始](../../README_zh.md)安装平台。GPU 需同时受所用 vLLM 运行时及其 [AWQ 实现](https://docs.vllm.ai/en/stable/features/quantization/)支持，例如 NVIDIA A100。示例请求 1 张 GPU、3 核 CPU 和 9 GiB 主机内存；平台需要额外资源。

`cache.yaml` 与快速开始共用项目根目录的 `data/`。远程集群需将 `spec.directory` 改为节点可访问的绝对路径，详见[模型存储](../../docs/model-storage_zh.md)。

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

使用 Gateway 模式时，按根目录快速开始配置域名和请求 Host 头。

更换模型时，修改 [`model.yaml`](model.yaml) 和请求中的模型标识，并在 `spec.engineArgs` 下设置匹配的原生参数 `quantization`、`dtype` 和 `max-model-len`。字段含义及原生引擎选项见[推理参数](../../docs/inference-parameters_zh.md)。

## 清理

```bash
foretoken delete examples/quantized-model
```

命名空间和服务资源会被删除，目录缓存中的已下载文件保留。
