<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# Deploy a quantized model

[English](README.md) | [中文](README_zh.md)

Serve the official [Qwen2.5-0.5B-Instruct-AWQ](https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-AWQ) checkpoint with 4-bit AWQ weights, FP16 activations and a 4096-token context. This example loads prequantized weights.

Install the platform using the repository [Quick Start](../../README.md). Use an NVIDIA GPU supported by the installed vLLM runtime and its [AWQ implementation](https://docs.vllm.ai/en/stable/features/quantization/), such as an A100. The example requests one GPU, 3 CPU and 9 GiB of host memory, plus platform capacity.

`cache.yaml` shares the repository-root `data/` directory with the Quick Start. For a remote cluster, set `spec.directory` to an absolute path available on its nodes; see [model storage](../../docs/model-storage.md).

## Deploy and send a request

Run from the repository root:

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

In Gateway mode, configure the hostname and request Host header as shown in the root Quick Start.

To use another checkpoint, update [`model.yaml`](model.yaml) and the model identifier in requests. Set `quantization`, `dtype` and `maxModelLen` under `spec` to match the checkpoint; see [inference parameters](../../docs/inference-parameters.md) for these fields and native engine options.

## Clean up

```bash
foretoken delete examples/quantized-model
```

The example namespace and serving resources are removed; downloaded files remain in the directory cache.
