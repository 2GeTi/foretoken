<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# 推理参数

[English](inference-parameters.md) | 简体中文

常用参数直接填写在 `ModelService.spec` 下。例如，部署 AWQ 模型：

```yaml
spec:
  model: Qwen/Qwen2.5-7B-Instruct-AWQ
  backend: vllm
  maxModelLen: 8192
  dtype: float16
  quantization: awq
  gpuMemoryUtilization: 0.85
```

省略的参数沿用引擎默认值。

## 常用选项

| 字段 | 用途 |
| --- | --- |
| `maxModelLen` | 输入和输出合计的最大 token 数 |
| `dtype` | 模型计算精度，例如 `auto`、`float16`、`bfloat16` |
| `quantization` | 权重量化方法；预量化模型通常可由引擎自动识别 |
| `kvCacheDType` | KV Cache 精度，例如 `auto`、`fp8`，与权重量化分别配置 |
| `gpuMemoryUtilization` | 每个引擎实例可使用的显存比例，大于 0 且不超过 1 |
| `maxNumSeqs` | 每轮调度的最大序列数 |
| `maxNumBatchedTokens` | 每轮调度的最大 token 数 |
| `enforceEager` | `true` 禁用计算图捕获；`false` 允许引擎使用计算图 |
| `speculativeDecoding` | 推测解码配置，见下例 |

具体精度、量化和推测方法需与引擎镜像、模型及硬件匹配。

## 推测解码

```yaml
spec:
  model: Qwen/Qwen3-0.6B
  backend: vllm
  speculativeDecoding:
    method: ngram
    num_speculative_tokens: 2
    prompt_lookup_max: 4
```

`speculativeDecoding` 使用引擎原生字段。`method` 使用原生方法名，例如 `draft_model`、`eagle3`、`ngram` 或 `mtp`；`num_speculative_tokens` 是每轮最多提出的草稿 token 数。不需要独立草稿权重时可以省略 `model`。

草稿模型由 vLLM 下载、加载和缓存。`model` 可填写 Hub 模型 ID 或容器内可见的绝对目录；`spec.source: modelscope` 同时适用于主模型和草稿模型的 Hub ID。

## 引擎原生参数

`engineArgs` 使用所选 backend 的原生参数名，不写 `--`。直接使用 YAML 布尔值、数字、字符串、列表和对象，不需要嵌入 JSON 字符串：

```yaml
spec:
  backend: vllm
  maxModelLen: 8192
  enforceEager: false
  engineArgs:
    max-model-len: 4096
    enforce-eager: true
    limit-mm-per-prompt:
      image: 2
```

`spec` 显式值优先，包括 `false`；上例使用 8192 的上下文并关闭 eager 模式。`speculativeDecoding` 整体替换 `engineArgs.speculative-config`，不合并子字段。原生选项设为 `null` 时不传给引擎。

原生选项只由对应 backend 解释，切换引擎时需调整。模型标识、启动端点、并行拓扑、传输连接器和性能剖析仍由 Foretoken 管理。当前支持 vLLM，完整参数见 [vLLM engine arguments](https://docs.vllm.ai/en/latest/configuration/engine_args/)。
