<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# Inference parameters

English | [简体中文](inference-parameters_zh.md)

Set common options directly in `ModelService.spec`. For example, configure an AWQ checkpoint:

```yaml
spec:
  model: Qwen/Qwen2.5-7B-Instruct-AWQ
  backend: vllm
  maxModelLen: 8192
  dtype: float16
  quantization: awq
  gpuMemoryUtilization: 0.85
```

Omitted options retain engine defaults. Apply changes with the same `foretoken deploy` command used to deploy the service.

## Common options

| Field | Purpose |
| --- | --- |
| `maxModelLen` | Maximum combined input and output token count |
| `dtype` | Model compute precision, such as `auto`, `float16`, or `bfloat16` |
| `quantization` | Weight quantization method; engines can usually detect prequantized checkpoints |
| `kvCacheDType` | KV cache precision, such as `auto` or `fp8` |
| `gpuMemoryUtilization` | Fraction of device memory per engine instance, greater than 0 and at most 1 |
| `maxNumSeqs` | Maximum sequences scheduled per iteration |
| `maxNumBatchedTokens` | Maximum tokens scheduled per iteration |
| `enforceEager` | `true` disables graph capture; `false` allows the engine to use graphs |
| `speculativeDecoding` | Speculative decoding settings, shown below |

Precision, quantization and speculative methods must match the engine image, model and hardware.

## Speculative decoding

```yaml
spec:
  model: Qwen/Qwen3-0.6B
  backend: vllm
  speculativeDecoding:
    method: ngram
    num_speculative_tokens: 2
    prompt_lookup_max: 4
```

`speculativeDecoding` accepts the complete native engine dictionary without a Foretoken field allowlist. `method` uses a native strategy name such as `draft_model`, `eagle3`, `ngram`, or `mtp`; `num_speculative_tokens` sets the maximum proposal length. Methods without separate draft weights can omit `model`.

vLLM downloads, loads and caches draft models. `model` accepts a Hub ID or an absolute directory visible inside the container. Selecting `spec.source: modelscope` applies to both target and draft Hub IDs.

## Native engine options

`engineArgs` uses option names from the selected backend without `--`. Supply YAML booleans, numbers, strings, lists and objects directly rather than embedding JSON strings:

```yaml
spec:
  backend: vllm
  maxModelLen: 8192
  enforceEager: false
  speculativeDecoding:
    method: ngram
    num_speculative_tokens: 2
    prompt_lookup_max: 4
  engineArgs:
    max-model-len: 4096
    enforce-eager: true
    limit-mm-per-prompt:
      image: 2
    speculative-config:
      method: ngram
      num_speculative_tokens: 5
      prompt_lookup_max: 8
```

Explicit `spec` fields take precedence: this example uses an 8192-token context and `enforceEager: false`. `speculativeDecoding` replaces the entire `engineArgs.speculative-config` dictionary, using two speculative tokens and `prompt_lookup_max: 4`; the dictionaries are not merged. A `null` value omits a native option.

Native options belong to the selected backend and may need changing when switching engines. Foretoken manages model identity, launch endpoints, parallel topology, transfer connectors and profiling. The current backend is vLLM; see its [engine argument reference](https://docs.vllm.ai/en/latest/configuration/engine_args/) for available options.
