<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# 对已有服务进行性能剖析

[English](profiling.md) | 简体中文

在已有模型服务处理请求时，用 PyTorch Profiler 采集一段 CPU/GPU 执行时间线。此功能处于实验阶段，需要源码安装，目前支持 NVIDIA GPU 上的 vLLM PyTorch profiler。

## 开始采集

使用已部署服务对应的 Kustomize 目录：

```bash
foretoken profile examples/quickstart \
  --profile-engine pytorch \
  --profile-duration 15s
```

命令只读取目录以定位已部署的服务，不重新应用配置。目录包含多个模型时，通过 `--model MODEL_ID` 选择一个。命令不会产生流量；采集期间通过正常的 Frontend 入口发送请求。

所选 ModelService 必须使用持久 RuntimeCache。维护中的快速开始示例已在 `cache.yaml` 中声明该存储，其他部署可参照[模型存储](../docs/model-storage_zh.md)。采集结果写入同一 RuntimeCache PVC 的 `profiles/` 目录。没有持久 RuntimeCache 的服务需要增加存储并重新部署后再采集。

runtime 在 profiler 启动后开始记录，到达指定时长后停止并导出文件，导出可能比记录耗时更长。正常完成不会停止模型推理。命令会输出 ProfileRun 名称，并在完成后输出结果所在的 RuntimeCache PVC 和路径。

| 参数 | 含义 |
|---|---|
| `--profile-engine pytorch` | 必填的 profiler 选择，目前只支持 PyTorch |
| `--profile-duration 15s` | 必填的记录时长，不含启动和导出时间 |
| `--model MODEL_ID` | 从多模型目录中选择一个模型 |
| `--timeout 10m` | CLI 等待进度的时间，不是 runtime 的采集时长 |

Ctrl-C 会请求取消并保留已有结果。终端断线或等待超时后，采集仍按原时限结束；可使用命令输出的查询指令查看进度。

## 同时运行 benchmark 和采集

从源码安装 benchmark 依赖（`pip install -e '.[bench]'`）后，可用一条命令完成服务部署或复用、发送请求和采集：

```bash
foretoken bench examples/quickstart \
  --profile --profile-engine pytorch --profile-duration 15s \
  --number 2 --max-tokens 128 --output local
```

命令会自动部署尚不存在的服务，或复用已有部署。服务需要使用持久 RuntimeCache。采集开始后才发送请求；负载完成后，命令会结束采集并等待 trace 导出。如果记录时长先结束，benchmark 仍会继续。按 Ctrl-C 或负载执行失败时会请求取消，并在取消完成后才清理资源。

采集完成后，命令会删除临时创建的服务资源。服务就绪后，profiling 的清理流程会保留部署的存储资源和命名空间，让结果在清理后仍可访问，也包括取消或失败的采集。已有部署保持不变。若无法确认采集已停止，命令会报错并保留部署供排查。按输出的 ProfileRun 和 PVC 信息查看结果；保存所需 trace 后，可运行 `foretoken delete PATH`，明确清理留下的部署资源。

此模式只支持一个生成式负载，并使用默认的 `--rate -1`；不支持 `--url`、轨迹回放、参数扫描或多个数据集。`--wait-timeout` 分别限制等待采集开始和导出的时间。Profiling 会增加开销，测量延迟和吞吐量时应另跑一次不带 `--profile` 的评测。

## 查看结果

每个 runtime 在以下目录中保存一份 manifest 和原生 `.pt.trace.json` 文件：

```text
profiles/runs/<run-uid>/<runtime-id>/
```

Quick Start 示例的结果位于项目根目录的 `data/profiles/runs/`。其他部署按命令输出的 PVC 和相对路径查找。

Profiling 会增加 CPU/GPU 开销，短窗口在高负载下仍可能产生很大文件。应使用规模较小的诊断部署和短窗口。原生 profiler 失败可能终止对应 runtime，因此服务需要允许这类中断。
