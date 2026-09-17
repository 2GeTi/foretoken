<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# 推理参数

[English](inference-parameters.md) | 简体中文

在 `ModelService.spec.inference` 中配置模型精度、上下文长度和批处理等常用选项。例如，为量化模型设置精度与上下文长度：

```yaml
spec:
  model: Qwen/Qwen2.5-7B-Instruct-AWQ
  inference:
    maxModelLen: 8192
    dtype: float16
    quantization: awq
```

只填写需要调整的字段；省略的选项沿用引擎默认行为。修改模型配置后，使用原来的 `foretoken deploy` 命令重新部署。

## 常用选项

| 字段 | 用途 |
| --- | --- |
| `maxModelLen` | 输入和输出合计的最大 token 数 |
| `dtype` | 模型计算精度，例如 `auto`、`float16`、`bfloat16` |
| `quantization` | 权重量化方法；预量化模型通常可由引擎自动识别 |
| `kvCacheDType` | KV Cache 精度，例如 `auto`、`fp8`，与权重量化独立 |
| `gpuMemoryUtilization` | 每个引擎实例可使用的设备显存比例，范围为大于 0 且不超过 1 |
| `maxNumSeqs` | 每次调度最多处理的序列数 |
| `maxNumBatchedTokens` | 每次调度最多处理的 token 数 |
| `enforceEager` | `true` 使用 eager 执行并禁用计算图捕获；`false` 允许引擎使用计算图 |
| `speculativeDecoding` | 推测解码方法、草稿模型及每轮推测长度 |

精度、量化方法和推测解码的可用取值由所选引擎镜像决定，并需与模型和硬件匹配。`dtype: float16` 不表示启用 INT4 等权重量化。

## 推测解码

下面的片段为 Llama 3.1 8B 配置与其匹配的 EAGLE3 草稿模型：

```yaml
spec:
  model: meta-llama/Meta-Llama-3.1-8B-Instruct
  inference:
    speculativeDecoding:
      method: eagle3
      model: yuhuili/EAGLE3-LLaMA3.1-Instruct-8B
      numSpeculativeTokens: 2
```

`method` 使用引擎原生名称，例如 `draft_model`、`eagle3`、`ngram` 或 `mtp`。`numSpeculativeTokens` 指定每轮最多提出的草稿 token 数；不需要单独草稿权重的方法可以省略 `model`。

草稿模型由 vLLM 下载和加载，并复用已挂载的 RuntimeCache。`model` 可以是 Hub 模型 ID，也可以是容器内可见的绝对目录。`spec.source: modelscope` 同时适用于主模型和草稿模型的 Hub ID。

## 参数透传

高级引擎选项放在 `spec.extraArgs`。每项是一个 `--flag` 或 `--flag=value` 参数，不经过 shell 拆分；值可以包含空格、JSON 或换行：

```yaml
spec:
  extraArgs:
    - --gpu-memory-utilization=0.85
    - --enforce-eager
    - '--limit-mm-per-prompt={"image": 2}'
```

同一选项可使用结构化字段或透传，但不能在两处同时配置。例如，填写 `inference.gpuMemoryUtilization` 后，就不要再传 `--gpu-memory-utilization`。模型标识、启动端点、并行拓扑和传输连接器仍由 Foretoken 管理。

需要更多推测解码选项时，省略 `inference.speculativeDecoding`，改为传递完整的引擎配置：

```yaml
spec:
  extraArgs:
    - >-
      --speculative-config={"method": "eagle3",
      "model": "yuhuili/EAGLE3-LLaMA3.1-Instruct-8B",
      "num_speculative_tokens": 2, "draft_tensor_parallel_size": 1}
```

透传也支持 vLLM 的点号字段写法，例如 `--compilation-config.mode=3`。引擎在启动时校验参数名称、取值和组合；参数说明见 [vLLM engine arguments](https://docs.vllm.ai/en/latest/configuration/engine_args/)。
