<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# 部署量化模型

[English](README.md) | [中文](README_zh.md)

可选择加载预量化 AWQ 权重，或将普通 checkpoint 在线转换为 FP8。两种配置均提供 Qwen2.5-0.5B-Instruct 服务，使用 4096-token 上下文，请求 1 张 GPU、3 核 CPU 和 9 GiB 主机内存；平台需要额外资源。

使用 `foretoken install -e .` 从当前源码[安装平台](../../docs/custom-deployment_zh.md)。

## 选择配置

在仓库根目录执行。默认配置加载官方 AWQ checkpoint，使用 FP16 激活值。GPU 需受 vLLM 的 [AWQ 实现](https://docs.vllm.ai/en/stable/features/quantization/)支持，例如 NVIDIA A100：

```bash
EXAMPLE=examples/quantized-model
MODEL=Qwen/Qwen2.5-0.5B-Instruct-AWQ
```

也可选择[在线 FP8 配置](online/kustomization.yaml)：加载普通 BF16 checkpoint，由 `quantization: fp8_per_tensor` 在加载阶段转换权重。需要提供该选项的 vLLM 镜像，以及其 FP8 算子支持的 GPU，例如 H100，详见[在线量化](https://docs.vllm.ai/en/latest/features/quantization/online/)。

```bash
EXAMPLE=examples/quantized-model/online
MODEL=Qwen/Qwen2.5-0.5B-Instruct
```

两种配置用于切换同一个服务，不同时部署。在线转换不会导出新的 checkpoint。

两种配置共用项目根目录的 `data/`。远程集群需填写节点可访问的绝对目录：AWQ 修改 [`base/cache.yaml`](base/cache.yaml)，在线 FP8 修改 [`online/kustomization.yaml`](online/kustomization.yaml) 中的缓存补丁。详见[模型存储](../../docs/model-storage_zh.md)。

## 部署并发送请求

```bash
foretoken deploy "$EXAMPLE" --timeout 20m
FRONTEND_URL="$(foretoken endpoint "$EXAMPLE")"

curl --fail-with-body "$FRONTEND_URL/v1/chat/completions" \
  -H 'Content-Type: application/json' \
  --data-binary @- <<EOF
{
  "model": "$MODEL",
  "messages": [{"role": "user", "content": "Explain quantization in one sentence."}],
  "max_tokens": 64,
  "temperature": 0
}
EOF
```

使用 Gateway 模式时，按根目录快速开始配置域名和请求 Host 头。原生引擎设置见[推理参数](../../docs/inference-parameters_zh.md)。

## 清理

```bash
foretoken delete "$EXAMPLE"
```

命名空间和服务资源会被删除，目录缓存中的已下载文件保留。
