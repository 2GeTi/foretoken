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

该命令不发送请求。多模型部署用 `--model MODEL_ID` 选择模型。`--profile-duration` 指记录时长；等待完成的时限由 `profile --timeout` 或 `bench --wait-timeout` 设置。

## 查看结果

按命令输出的位置找到 `.pt.trace.json` 文件，用 [Perfetto](https://ui.perfetto.dev/) 打开。快速开始示例将结果保存在项目根目录的 `data/profiles/runs/`；其他存储配置见[模型存储](../docs/model-storage_zh.md)。

评测结束后会清理临时服务，保留采集结果及其存储；已有服务保持运行。保存所需结果后，清理该部署留下的资源：

```bash
foretoken delete examples/quickstart
```

Profiling 会增加开销，延迟和吞吐量对比请使用不带 `--profile` 的评测。
