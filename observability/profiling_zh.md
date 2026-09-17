<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# 性能剖析

[English](profiling.md) | 简体中文

使用 PyTorch Profiler 查看模型推理的 CPU/GPU 执行时间线。目前支持 NVIDIA GPU 上的 vLLM，需使用[源码安装](../docs/custom-deployment_zh.md)的 CLI 和平台。采集结果使用持久 RuntimeCache 保存，快速开始示例已配置好该存储。

## 同时运行 benchmark 和采集

从仓库根目录执行：

```bash
pip install -e '.[bench]'
foretoken bench examples/quickstart \
  --profile --profile-engine pytorch --profile-duration 15s \
  --number 2 --max-tokens 128 --output local
```

此模式支持 Kustomize 部署中的单个生成式负载，使用默认的 `--rate -1`。

## 开始采集

服务已经运行时，可单独采集一段现有流量：

```bash
foretoken profile examples/quickstart \
  --profile-engine pytorch --profile-duration 15s
```

该命令不发送请求。多模型部署用 `--model MODEL_ID` 选择模型。`--profile-duration` 设置最长记录时间，评测负载提前结束时也会停止采集。

## 查看结果

将采集生成的 `.pt.trace.json` 文件下载到本地，拖入 [Perfetto](https://ui.perfetto.dev/) 查看执行时间线。

带采集的评测会保留结果及其存储。保存所需文件后清理：

```bash
foretoken delete examples/quickstart
```

Profiling 会增加开销，延迟和吞吐量对比请使用不带 `--profile` 的评测。
