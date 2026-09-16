<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# Slack 告警通知

[English](README.md) | 简体中文

通过 Alertmanager 原生 Slack 接收器，将 Foretoken 告警发送到 Slack 频道。通知逐条展示触发或解除状态、资源标签、时间、详情和排障链接，默认使用英文注解。

## 接入 Slack

先安装 Foretoken 平台，启用需要的[服务告警](../../README_zh.md#告警)。按照 [Slack 指南](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/)创建 Slack 应用，开启 **Incoming Webhooks**，并为目标频道授权 webhook。接收频道由 webhook 决定。

使用 CLI 管理的监控栈时，在 `foretoken-platform` 中创建 Secret。将占位符替换为 incoming webhook URL：

```bash
kubectl create secret generic foretoken-slack-webhook \
  --namespace foretoken-platform \
  --from-literal=url='<SLACK_INCOMING_WEBHOOK_URL>'
```

在平台 values 文件（如 `platform-values.yaml`）中添加：

```yaml
observability:
  notifications:
    slack:
      webhookSecret:
        name: foretoken-slack-webhook
        key: url
```

沿用原来的安装方式更新平台：

```bash
foretoken install --values platform-values.yaml
# 如果平台由源码安装，在仓库根目录执行：
# foretoken install -e . --values platform-values.yaml
```

Helm 会创建接收器和匹配 Foretoken 告警的路由。CLI 管理的 Alertmanager 允许自身命名空间中的接收器接收工作负载告警。Slack 可与可选的 [Lark 接收器](../lark/README_zh.md) 并用。

如果 Secret 已存在，按现有凭据管理流程更新。不要将 webhook 写入 values 文件或版本库。Secret 由你管理，平台安装和卸载均不会删除它。

## 使用已有 Alertmanager

已有监控栈仍由其管理员维护。它必须接收 Foretoken 所用 Prometheus 的告警，并选中生成的 `AlertmanagerConfig`。将 Secret 放在该 Alertmanager 所在命名空间，同时把 `observability.notifications.namespace` 设为这个命名空间。

如果管理员要求配置带有特定标签，补充匹配标签。例如，Alertmanager 位于 `monitoring`，选择器要求 `team=inference` 时，在前面的 values 中添加：

```yaml
observability:
  notifications:
    namespace: monitoring
    additionalLabels:
      team: inference
```

管理员还需要允许路由匹配工作负载命名空间。在支持的版本中，`spec.alertmanagerConfigMatcherStrategy.type: OnNamespaceExceptForAlertmanagerNamespace` 允许与 Alertmanager 同命名空间的配置匹配工作负载告警。参阅 [Operator 告警指南](https://prometheus-operator.dev/docs/developer/alerting/)。Foretoken 不会修改复用的 Alertmanager。

## 验证投递

确认接收器已在通知命名空间中生成：

```bash
kubectl get alertmanagerconfig foretoken-control-plane-slack \
  --namespace foretoken-platform
```

使用外部 Alertmanager 时，改为前面配置的命名空间。在 Alertmanager 中确认配置已加载，并将带有 `service=foretoken` 标签的告警路由到 Slack。测试告警使用实际 Foretoken 工作负载命名空间，确认频道收到触发和解除通知，且分组消息包含各个受影响资源。

没有收到消息时，检查 Prometheus 与 Alertmanager 的连接、配置选择器、命名空间匹配、Secret 引用和 Alertmanager 投递日志。抓取失败告警解除也可能表示目标已消失，不一定代表原 Pod 或节点恢复。

## 移除集成

将 values 文件中的 `observability.notifications.slack.webhookSecret.name` 设为空字符串，再次执行相同的安装命令。Helm 会删除 Slack 接收器，保留其他通知渠道。Secret 不再使用时单独删除：

```bash
kubectl delete secret foretoken-slack-webhook --namespace foretoken-platform
```

使用外部 Alertmanager 时，改为其命名空间。`foretoken uninstall` 也会删除 Helm 管理的接收器，并保留 Secret。
