# 自动扩缩容架构

[English](README.md) | [中文](README_zh.md)

本包将控制器拥有的观测转换为 `ModelPool` 容量。用户通过 `ModelService.spec.autoscaling` 配置自动扩缩容；配置和状态使用方式见[自动扩缩容指南](../../../../docs/autoscaling_zh.md)。

## 职责归属

`ModelService` 控制器负责调度、观测采集、扩缩目标发现、状态发布，以及将容量写入 `ModelPool`。算法保持无副作用：只评估一个完整观测并返回容量建议。

聚合目标扩缩一个 Pool。E/P/D 目标扩缩一个 `EPDPipelineScope`，将相同容量写入 encoder、prefill 和 decode Pool。

## 评估流水线

```text
控制器轮询
→ ScalingSnapshot
→ TriggerDecision
→ ReplicaRecommendation
→ ReplicaAdjustment
→ ScalingDecision
→ ModelPool 容量和 ModelService 状态
```

控制器向流水线提供完整、近期的观测。`periodic` 接受这些观测，不拥有时间间隔或重新入队循环。即使缺少观测，Resolver 仍会应用最小和最大副本数硬限制；目标处于转换中时保持容量。

`step` 的稳定窗口使用当前控制器进程保存的近期建议。历史刻意保存在运行时本地，因此重启或 leader 切换不会恢复尚未结束的缩容延迟。

## 扩展边界

内置算法位于 `algorithm/`。Trigger、Decision 和 Adjustment 实现返回领域结果，不读取 Kubernetes 资源、不修改容量，也不调度工作。新增实现只有在它代表当前独立负责的建议策略时才有意义；控制器生命周期行为保留在 `core` 和 ModelService reconciler 中。

`algorithm/registry.go` 在一个静态注册表中声明全部内置构造函数，实现文件不再通过 `init()` 自注册。新增决策策略时，实现容量建议和参数构造函数，再将构造函数加入决策注册表。构造函数接收 `decision.parameters` JSON 对象，负责显式解码、默认值和校验，返回使用后端中立快照的策略对象。现有整数参数策略共用显式字段解码器；其他参数类型由实际使用它的策略负责。

CRD 只携带算法名称和必填的参数对象，不列举策略专属字段。控制器每轮协调只构造一次所选流水线，并复用解析后的轮询配置，在写入 Pool 前拒绝无效配置；compiler 只编译模型部署意图。新增决策策略不需要增加 API、compiler 或控制器分发分支，但需要在用户指南中说明参数和行为。

触发调度与调整历史继续由现有控制器和流水线负责，它们的通用配置独立于决策参数。

## 验证

修改本包后运行控制面验证：

```bash
make -C control-plane verify
```
