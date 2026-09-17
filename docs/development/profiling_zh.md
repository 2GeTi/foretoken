<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# Profiling 生命周期

[English](profiling.md) | 简体中文

`ProfileRun` 管理一次限时采集，独立于发起命令的进程。操作和结果查看见[性能剖析](../../observability/profiling_zh.md)。

## 执行与恢复

控制器重启后沿用已保存的参与实例和存储绑定。服务版本改变或参与实例被替换时，取消本次采集。每个 model-server 同时只允许一次采集；同一运行的重试保持幂等，取消不可撤销。

全部参与实例开始记录后才发布 `Capturing`，全部结果导出后才能成功。删除活动运行会请求取消，确认停止后才释放 finalizer。原生停止失败时，supervisor 关闭请求准入并终止引擎进程组。

## Benchmark 集成

评测请求等待采集就绪后发送。负载完成时请求 `Finish`，失败或中断时请求 `Cancel`。确认采集结束后才清理临时服务；停止未确认时保留服务，供后续恢复。

## 结果保留

采集要求持久 RuntimeCache 存储。各实例校验 worker trace 后才发布结果，GPU 活动与 trace 有效性分别报告。删除 ProfileRun 不删除结果文件；带采集的评测在服务清理后保留结果存储及其命名空间。

原生错误上报和重复采集由 [vLLM profiling 补丁](../../data-plane/patches/vllm-python-profiling.patch)支持。
