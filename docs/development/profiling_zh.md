<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# Profiling 生命周期

[English](profiling.md) | 简体中文

`ProfileRun` 管理一次限时采集，独立于发起命令的进程。操作方法和结果查看见[性能剖析](../../observability/profiling_zh.md)。

## 执行与恢复

控制器在采集开始前持久化服务版本、参与实例身份和 RuntimeCache 绑定，重启后继续使用同一份计划。参与实例被替换或服务实例集合改变时，取消本次采集。ProfileRun 操作受 Kubernetes RBAC 控制，原生采集通过已有的内部 HTTP 接口调用。

model-server supervisor 负责原生 profiler 的启动、记录、停止和导出。同时只允许一次采集，同一运行的重试保持幂等，取消不可撤销。记录时长从原生启动成功后计算；启动和停止导出分别有 30 秒、120 秒的时限，`Finish` 可提前结束记录。

全部参与实例开始记录后才发布 `Capturing`，全部结果导出后才能成功。删除活动的 ProfileRun 会请求取消，确认停止后才释放 finalizer。原生停止失败时，supervisor 关闭新请求准入并终止引擎进程组，可能中断推理。

## 存储

采集使用 ModelService 的持久 RuntimeCache，回退到 Pod 临时存储的 runtime 无法参与。各实例在停止导出后校验预期 worker 的 trace，写入 manifest，再在同一文件系统内将暂存目录原子重命名为 `profiles/runs/<run-uid>/<runtime-id>/`。GPU 活动与 trace 有效性分别报告。

ProfileRun 只发布结果位置，不拥有结果文件；删除运行记录不删除文件。model-server adapter 通过 [vLLM profiling 补丁](../../data-plane/patches/vllm-python-profiling.patch)处理原生错误上报和重复采集。

## Benchmark 集成

采集复用普通评测的服务解析和 EvalScope 执行器。请求准备好后等待采集就绪；负载完成时请求 `Finish`，失败或中断时请求 `Cancel`。确认采集结束后才清理临时服务；停止未确认时保留服务，供控制器继续恢复。

评测仅清理自己创建的资源。带采集的评测在服务就绪后保留命名空间和存储，供用户读取结果；就绪前的部署失败沿用普通清理流程。已有部署保持不变。

## 参考

- [vLLM profiling](https://docs.vllm.ai/en/stable/contributing/profiling/)
- [PyTorch profiler](https://docs.pytorch.org/docs/stable/profiler.html)
