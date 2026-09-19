<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# 模型服务自动扩缩容

[English](autoscaling.md) | [中文](autoscaling_zh.md)

自动扩缩容根据请求负载调整 `ModelService` 容量。先在服务配置中启用自动扩缩容，再在工作负载运行时查看服务状态。

## 容量单位

聚合模型服务的每个副本按照配置的资源和并行参数运行完整模型。对于将编码、预填充和解码分开运行的 E/P/D 服务，一个副本包含 encoder、prefill 和 decode 三个阶段，三者一起扩缩；所需资源为各阶段配置的资源之和。

`spec.replicas` 提供基线容量。配置 `autoscaling` 后，`minReplicas` 和 `maxReplicas` 从首次协调起就约束实际创建的容量。

## 配置队列自动扩缩容

在已有 `ModelService` 中添加以下 `spec` 配置。它从 1 个副本开始，在 1–8 个副本之间运行，每 5 秒评估一次近期队列负载，并且每次最多调整 1 个副本：

```yaml
spec:
  replicas: 1
  autoscaling:
    minReplicas: 1
    maxReplicas: 8
    trigger:
      algorithm: periodic
      interval: 5s
    decision:
      algorithm: queue
      parameters:
        targetAverageQueuedRequests: 1
    adjustment:
      algorithm: step
      scaleUp:
        stabilizationWindow: 0s
      scaleDown:
        stabilizationWindow: 300s
```

`periodic` 按配置的间隔评估队列负载。指标缺失、过期或不完整时，保持当前容量。自动扩缩容至少保留一个副本。

`queue` 根据每个副本的平均等待请求数计算容量。`queue_threshold` 则在配置的服务总积压边界按一次一个副本调整容量。`direct` 在应用最小和最大副本数限制后直接应用建议；`step` 每次评估最多调整一个副本，并可分别配置扩容和缩容稳定窗口。

缩容稳定窗口使用当前控制器进程保存的近期建议。控制器重启或 leader 切换不会保留这些历史，因此可能缩短等待缩容的延迟。

## 决策参数

`decision.parameters` 必须提供。使用 `parameters: {}` 可以接受所选算法的默认值。参数由该算法负责；控制器会在写入容量前拒绝未知字段、非整数值和不合法的范围。

| 算法 | 参数 | 默认值 | 约束 |
| --- | --- | --- | --- |
| `queue` | `targetAverageQueuedRequests` | `1` | 正整数 |
| `queue_threshold` | `scaleUpQueuedRequests` | `1` | 非负整数 |
| `queue_threshold` | `scaleDownQueuedRequests` | `0` | 非负整数，不超过 `scaleUpQueuedRequests` |

控制器必须包含对应名称的算法。未知算法名称或无效参数会使 ModelService 出现 `ScalingFailed` condition；Kubernetes 校验参数必须为对象，由选中的算法校验对象内容。

## 查看扩缩容决策

自动扩缩容结果发布在 `.status.autoscaling[]` 中，每个扩缩目标对应一项。使用以下命令查询维护中的多模型示例：

```bash
kubectl get modelservice multi-model-qwen3-0.6b \
  --namespace foretoken-multi-model-demo \
  -o json | jq '.status.autoscaling[] | {
    id,
    kind,
    role,
    observationState,
    direction,
    desiredReplicas: .decision.desiredReplicas,
    adjustedReplicas: .adjustment.adjustedReplicas,
    appliedReplicas,
    constraint: .constraint.reason
  }'
```

`desiredReplicas` 是算法建议，`adjustedReplicas` 是稳定窗口和速率限制后的结果，`appliedReplicas` 是生命周期与最小/最大副本数限制后写入目标的容量。`observationState`、各阶段 reason 和 `constraint` 用于说明容量为何保持或改变。

聚合模型服务的 `kind` 为 `Pool`。E/P/D 服务的 `kind` 为 `EPDPipelineScope`，`role` 为 `EPD`。

## 使用维护中的示例

[多模型示例](../examples/multi-model-quickstart/README_zh.md)部署一个按队列自动扩缩的 Qwen 服务和一个固定容量的 Llama 服务，其中包含有界并发负载和观察容量变化的状态命令。

## 迁移旧版决策配置

`v1alpha1` 决策配置现使用 `parameters`，替代算法专属的 `queue` 和 `queueThreshold` 块。将所选块中的内容移入 `parameters`，删除旧块；算法名称和参数值保持不变。例如：

```yaml
decision:
  algorithm: queue
  parameters:
    targetAverageQueuedRequests: 2
```

控制器和 CRD 需要配套更新，再重新提交迁移后的服务配置。迁移完成前，已有服务保持当前 Pool 容量并报告 `ScalingFailed`；缺少 `parameters` 会被拒绝，避免静默使用算法默认值。新提交的旧格式配置会被 schema 拒绝。回退控制器和 CRD 前，需要将配置恢复为对应的旧决策块。

## 维护者架构

控制器阶段、观测聚合、算法扩展边界和生命周期解析见[自动扩缩容维护者 README](../control-plane/internal/autoscaling/README_zh.md)。
