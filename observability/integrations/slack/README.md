<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# Slack alert notifications

English | [简体中文](README_zh.md)

Send Foretoken alerts to a Slack channel through Alertmanager's native Slack receiver. Notifications include each alert's firing or resolved state, resource labels, timestamps, details, and runbook link. Messages use English annotations.

## Connect Slack

Start with an installed Foretoken platform and enable the [service alerts](../../README.md#alerts) you need. Create a Slack app, enable **Incoming Webhooks**, and authorize a webhook for the destination channel following the [Slack guide](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/). The webhook determines the channel.

For the CLI-managed monitoring stack, create the Secret in `foretoken-platform`. Replace the placeholder with the incoming webhook URL:

```bash
kubectl create secret generic foretoken-slack-webhook \
  --namespace foretoken-platform \
  --from-literal=url='<SLACK_INCOMING_WEBHOOK_URL>'
```

Add this to your platform values file, for example `platform-values.yaml`:

```yaml
observability:
  notifications:
    slack:
      webhookSecret:
        name: foretoken-slack-webhook
        key: url
```

Update the platform using the same installation mode as before:

```bash
foretoken install --values platform-values.yaml
# For a platform installed from source, run from the repository root:
# foretoken install -e . --values platform-values.yaml
```

Helm creates the receiver and its Foretoken alert route. The CLI-managed Alertmanager accepts workload alerts through receivers in its own namespace. Slack can coexist with the optional [Lark receiver](../lark/README.md).

If the Secret already exists, update it through your usual secret-management process. Keep the webhook out of values files and version control. The Secret remains under your ownership; installation and uninstallation do not delete it.

## Use an existing Alertmanager

The existing monitoring stack remains under its administrator's control. It must receive alerts from the Prometheus used by Foretoken and select the generated `AlertmanagerConfig`. Put the Secret in that Alertmanager's namespace and set `observability.notifications.namespace` to the same namespace.

If the administrator requires a label selector, add the matching labels. For example, with Alertmanager in `monitoring` and a selector for `team=inference`, extend the values above:

```yaml
observability:
  notifications:
    namespace: monitoring
    additionalLabels:
      team: inference
```

The administrator must also allow workload namespaces on this route. On versions supporting it, `spec.alertmanagerConfigMatcherStrategy.type: OnNamespaceExceptForAlertmanagerNamespace` allows configurations beside Alertmanager to match workload alerts. See the [Operator alerting guide](https://prometheus-operator.dev/docs/developer/alerting/). Foretoken does not patch a reused Alertmanager.

## Verify delivery

Check that the generated configuration exists in the notification namespace:

```bash
kubectl get alertmanagerconfig foretoken-control-plane-slack \
  --namespace foretoken-platform
```

For an external Alertmanager, use the namespace selected above. In Alertmanager, confirm that the configuration is loaded and routes alerts labeled `service=foretoken` to Slack. Use a test alert with an actual Foretoken workload namespace, then confirm the channel receives both firing and resolved notifications, including each affected resource when alerts are grouped.

If no message arrives, check Prometheus's Alertmanager connection, the configuration selector, namespace matching, Secret reference, and Alertmanager delivery logs. A resolved scrape failure can also mean that the target disappeared; it does not necessarily mean the original Pod or node recovered.

## Remove the integration

Set `observability.notifications.slack.webhookSecret.name` to an empty string in the values file and run the same install command again. Helm removes the Slack receiver; other notification channels remain configured. Remove the Secret separately when it is no longer used:

```bash
kubectl delete secret foretoken-slack-webhook --namespace foretoken-platform
```

For an external Alertmanager, use its namespace. `foretoken uninstall` also removes the Helm-managed receiver and preserves the Secret.
