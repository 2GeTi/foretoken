<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# Deploy a quantized model

[English](README.md) | [中文](README_zh.md)

Choose prequantized AWQ weights or online FP8 conversion of an ordinary checkpoint. Both configurations serve Qwen2.5-0.5B-Instruct with a 4096-token context and request one GPU, 3 CPU and 9 GiB of host memory, plus platform capacity.

Install the [platform from this source checkout](../../docs/custom-deployment.md) with `foretoken install -e .`.

## Choose a configuration

Run from the repository root. The default loads the official AWQ checkpoint with FP16 activations on a GPU supported by vLLM's [AWQ implementation](https://docs.vllm.ai/en/stable/features/quantization/), such as an A100:

```bash
EXAMPLE=examples/quantized-model
MODEL=Qwen/Qwen2.5-0.5B-Instruct-AWQ
```

Alternatively, [online FP8](online/kustomization.yaml) loads the ordinary BF16 checkpoint and converts weights during loading using `quantization: fp8_per_tensor`. Use a vLLM image exposing that option and a GPU supported by its FP8 kernels, such as an H100; see [online quantization](https://docs.vllm.ai/en/latest/features/quantization/online/).

```bash
EXAMPLE=examples/quantized-model/online
MODEL=Qwen/Qwen2.5-0.5B-Instruct
```

These are alternatives for the same service, not two concurrent deployments. Online conversion does not export a new checkpoint.

Both configurations share the repository-root `data/` directory. For a remote cluster, set an absolute node-visible directory in [`base/cache.yaml`](base/cache.yaml), or in the cache patch in [`online/kustomization.yaml`](online/kustomization.yaml) when using online FP8. See [model storage](../../docs/model-storage.md).

## Deploy and send a request

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

In Gateway mode, configure the hostname and request Host header as shown in the root Quick Start. Native engine settings are described in [inference parameters](../../docs/inference-parameters.md).

## Clean up

```bash
foretoken delete "$EXAMPLE"
```

The namespace and serving resources are removed; downloaded files remain in the directory cache.
