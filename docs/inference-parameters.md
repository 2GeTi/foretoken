<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# Inference parameters

English | [简体中文](inference-parameters_zh.md)

Use `ModelService.spec.inference` for common precision, context-length, and batching options. For example, configure an AWQ checkpoint with a context limit:

```yaml
spec:
  model: Qwen/Qwen2.5-7B-Instruct-AWQ
  inference:
    maxModelLen: 8192
    dtype: float16
    quantization: awq
```

Set only the options you need to change. Omitted options retain the engine's default behavior. Apply changes with the same `foretoken deploy` command used to deploy the service.

## Common options

| Field | Purpose |
| --- | --- |
| `maxModelLen` | Maximum combined input and output token count |
| `dtype` | Model compute precision, such as `auto`, `float16`, or `bfloat16` |
| `quantization` | Weight quantization method; engines can usually detect prequantized checkpoints |
| `kvCacheDType` | KV cache precision, such as `auto` or `fp8`, independent of weight quantization |
| `gpuMemoryUtilization` | Fraction of device memory available to each engine instance, greater than 0 and at most 1 |
| `maxNumSeqs` | Maximum sequences scheduled per engine iteration |
| `maxNumBatchedTokens` | Maximum tokens scheduled per engine iteration |
| `enforceEager` | `true` disables graph capture and uses eager execution; `false` allows the engine to use graphs |
| `speculativeDecoding` | Speculative method, draft model, and proposal length |

Available precision, quantization, and speculative decoding settings depend on the engine image, model, and hardware.

## Speculative decoding

This fragment pairs Llama 3.1 8B with a matching EAGLE3 draft checkpoint:

```yaml
spec:
  model: meta-llama/Meta-Llama-3.1-8B-Instruct
  inference:
    speculativeDecoding:
      method: eagle3
      model: yuhuili/EAGLE3-LLaMA3.1-Instruct-8B
      numSpeculativeTokens: 2
```

`method` uses the engine's native name, such as `draft_model`, `eagle3`, `ngram`, or `mtp`. `numSpeculativeTokens` sets the maximum proposal length per decoding step. Methods without separate draft weights can omit `model`.

vLLM downloads and loads draft models using the mounted RuntimeCache. Set `model` to a Hub model ID or an absolute directory visible inside the container. Selecting `spec.source: modelscope` applies to both target and draft Hub IDs.

## Argument passthrough

Pass advanced engine options through `spec.extraArgs`. Each item is one `--flag` or `--flag=value` argument, without shell splitting. Values may contain spaces, JSON, or line breaks:

```yaml
spec:
  extraArgs:
    - --gpu-memory-utilization=0.85
    - --enforce-eager
    - '--limit-mm-per-prompt={"image": 2}'
```

Configure each option in either its structured field or passthrough, not both. Foretoken manages model identity, launch endpoints, parallel topology, transfer connectors, and profiling.

For additional speculative options, omit `inference.speculativeDecoding` and pass the complete engine configuration instead:

```yaml
spec:
  extraArgs:
    - >-
      --speculative-config={"method": "eagle3",
      "model": "yuhuili/EAGLE3-LLaMA3.1-Instruct-8B",
      "num_speculative_tokens": 2, "draft_tensor_parallel_size": 1}
```

Passthrough also accepts vLLM's dotted fields, such as `--compilation-config.mode=3`. The engine validates argument names, values, and combinations during startup. See the [vLLM engine argument reference](https://docs.vllm.ai/en/latest/configuration/engine_args/) for available options.
