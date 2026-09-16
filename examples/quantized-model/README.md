<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# Deploy a quantized model

[English](README.md) | [中文](README_zh.md)

Serve the official [Qwen2.5-0.5B-Instruct-AWQ](https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-AWQ) checkpoint with 4-bit AWQ weights through Foretoken. This example loads already quantized weights; it does not train or quantize a model.

## Before you start

Install the platform using the repository [Quick Start](../../README.md). Use an NVIDIA GPU supported by both the installed vLLM runtime and its [AWQ implementation](https://docs.vllm.ai/en/stable/features/quantization/), such as an A100. Support for other quantization formats and accelerators depends on the runtime image.

The example requests one GPU, 3 CPU, and 9 GiB of host memory in total, plus platform capacity. These are scheduling requests, not measured memory requirements. GPU memory also holds activations and the KV cache; 4-bit weights do not make the entire serving process 4-bit.

Prepare [model storage](../../docs/model-storage.md). `cache.yaml` shares the repository-root `data/` directory with the Quick Start. For a remote cluster, set `spec.directory` to an absolute directory available on the target nodes. For restricted networks, see [Model sources](../../docs/model-sources.md).

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

Deployment waits for readiness in `foretoken-quantized-demo`. A successful request returns a chat completion from the quantized model. In Gateway mode, configure the frontend hostname and request Host header as shown in the root Quick Start.

## Select another checkpoint

Edit [`model.yaml`](model.yaml) and use the same model identifier in requests. The example selects AWQ explicitly with `--quantization=awq`, FP16 activations with `--dtype=half`, and a 4096-token context with `--max-model-len=4096` under `spec.extraArgs`.

For checkpoints whose quantization metadata is recognized by vLLM, you can omit `--quantization` and let the engine select the implementation. When changing formats, update or remove the AWQ and dtype arguments according to the checkpoint and runtime requirements. Setting `--quantization=awq` on ordinary weights does not convert them into an AWQ checkpoint. Each extra argument must be a single `--name=value` string.

A successful request verifies serving, not quality or speed. Compare task quality and measure latency, throughput, and memory on the target GPU before choosing a quantized model; see [benchmarks](../../benchmarks/README.md).

## Clean up

```bash
foretoken delete examples/quantized-model
```

This removes the example namespace and serving resources. The directory-backed cache retains downloaded files for reuse.
