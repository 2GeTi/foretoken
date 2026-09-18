<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# Slack 告警通知

[English](README.md) | 简体中文

通过 Alertmanager 原生 Slack 接收器，将 Foretoken 告警发送到 Slack 频道。消息包含告警状态、资源标签、时间、英文摘要和排障链接。

## 接入 Slack

先安装 Foretoken 平台，再按照 [Slack 指南](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/)为目标频道创建 incoming webhook。

Secret 放在 Alertmanager 所在的命名空间。CLI 管理的监控栈使用 `foretoken-platform`；复用已有监控时，使用已有 Alertmanager 的命名空间。

已有监控栈的管理员需要让 Alertmanager 选中 Foretoken 的 `AlertmanagerConfig`，并允许接收工作负载命名空间的告警。`OnNamespaceExceptForAlertmanagerNamespace` 匹配策略可让 Alertmanager 自身命名空间中的配置接收这些告警，见 [Operator API 参考](https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.AlertmanagerConfigMatcherStrategy)。

将 `ALERTMANAGER_NAMESPACE` 设为选定的命名空间，并替换 webhook 占位符：

```bash
ALERTMANAGER_NAMESPACE=foretoken-platform
kubectl create secret generic foretoken-slack-webhook \
  --namespace "$ALERTMANAGER_NAMESPACE" \
  --from-literal=url='<SLACK_INCOMING_WEBHOOK_URL>'
```

在 `platform-values.yaml` 中添加以下配置；已有 `observability.notifications` 时合入同一处。命名空间不是 `foretoken-platform` 时，将 `namespace` 设为与 `ALERTMANAGER_NAMESPACE` 相同的值。只有已有 Alertmanager 要求匹配标签时，才填写 `additionalLabels`：

```yaml
observability:
  notifications:
    slack:
      webhookSecret:
        name: foretoken-slack-webhook
    # 已有 Alertmanager 位于 monitoring，且按 team=inference 选择配置时：
    # namespace: monitoring
    # additionalLabels:
    #   team: inference
```

沿用原来的安装方式更新平台：

```bash
foretoken install --values platform-values.yaml
# 源码安装的平台，在仓库根目录执行：
# foretoken install -e . --values platform-values.yaml
```

启用需要投递的[服务告警](../../README_zh.md#告警)。Slack 可与 [Lark 接收器](../lark/README_zh.md) 并用。Webhook 保存在 Secret 中；已有 Secret 按现有凭据管理流程更新。

## 验证投递

检查接收器，并查找选中它的 Alertmanager 实例所运行的 Pod：

```bash
kubectl get alertmanagerconfig foretoken-control-plane-slack \
  --namespace "$ALERTMANAGER_NAMESPACE"
kubectl get pods --namespace "$ALERTMANAGER_NAMESPACE" \
  --selector app.kubernetes.io/name=alertmanager
```

将 `<ALERTMANAGER_POD>` 替换为该实例的 Pod 名称，将 `WORKLOAD_NAMESPACE` 设为已部署 Foretoken 服务的命名空间。下面通过 Pod 自带的 `amtool` 向 Alertmanager 发送一条临时告警，两分钟后自动过期：

```bash
ALERTMANAGER_POD='<ALERTMANAGER_POD>'
WORKLOAD_NAMESPACE=foretoken-demo
ALERT_END="$(python -c 'from datetime import UTC, datetime, timedelta; print((datetime.now(UTC) + timedelta(minutes=2)).isoformat())')"
kubectl exec --namespace "$ALERTMANAGER_NAMESPACE" "$ALERTMANAGER_POD" \
  --container alertmanager -- \
  amtool --alertmanager.url=http://localhost:9093 alert add \
  ForetokenNotificationTest service=foretoken namespace="$WORKLOAD_NAMESPACE" \
  --annotation='summary="Foretoken notification test"' \
  --annotation='description="Temporary notification delivery check."' \
  --end="$ALERT_END"
```

频道应先收到触发通知，告警过期后再收到解除通知。测试也会投递到其他匹配 `service=foretoken` 的接收器。未收到消息时，查看 Alertmanager 日志：

```bash
kubectl logs --namespace "$ALERTMANAGER_NAMESPACE" "$ALERTMANAGER_POD" \
  --container alertmanager --since=5m
```

## 移除集成

将 `observability.notifications.slack.webhookSecret.name` 设为空字符串，再次执行安装命令。Helm 会删除 Slack 接收器，其他通知渠道保持不变。Secret 不再使用时单独删除：

```bash
kubectl delete secret foretoken-slack-webhook --namespace "$ALERTMANAGER_NAMESPACE"
```

`foretoken uninstall` 也会删除接收器，并保留 Secret。
