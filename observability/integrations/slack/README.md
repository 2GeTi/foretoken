<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# Slack alert notifications

English | [简体中文](README_zh.md)

Send Foretoken alerts to a Slack channel through Alertmanager's native Slack receiver. Messages contain alert states, resource labels, timestamps, English summaries, and runbook links.

## Connect Slack

Start with an installed Foretoken platform. Follow the [Slack guide](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/) to create an incoming webhook for the destination channel.

Create the Secret in Alertmanager's namespace: `foretoken-platform` for the CLI-managed stack, or the namespace of your existing Alertmanager.

For an existing stack, its administrator must select the Foretoken `AlertmanagerConfig` and allow workload namespaces. The `OnNamespaceExceptForAlertmanagerNamespace` matcher strategy permits this for configurations in Alertmanager's own namespace; see the [Operator API reference](https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.AlertmanagerConfigMatcherStrategy).

Set `ALERTMANAGER_NAMESPACE` to that namespace and replace the webhook placeholder:

```bash
ALERTMANAGER_NAMESPACE=foretoken-platform
kubectl create secret generic foretoken-slack-webhook \
  --namespace "$ALERTMANAGER_NAMESPACE" \
  --from-literal=url='<SLACK_INCOMING_WEBHOOK_URL>'
```

Add the following to `platform-values.yaml`, merging into any existing `observability.notifications` mapping. If the namespace differs from `foretoken-platform`, set `namespace` to the same value as `ALERTMANAGER_NAMESPACE`. Add `additionalLabels` only when the existing Alertmanager requires matching labels:

```yaml
observability:
  notifications:
    slack:
      webhookSecret:
        name: foretoken-slack-webhook
    # For an existing Alertmanager in monitoring selecting team=inference:
    # namespace: monitoring
    # additionalLabels:
    #   team: inference
```

Update the platform using the same installation mode as before:

```bash
foretoken install --values platform-values.yaml
# For a platform installed from source, run from the repository root:
# foretoken install -e . --values platform-values.yaml
```

Enable the [service alerts](../../README.md#alerts) you want delivered. Slack can run alongside the [Lark receiver](../lark/README.md). Keep the webhook in the Secret; update an existing Secret through your usual secret-management process.

## Verify delivery

Check the receiver and find a Pod of the Alertmanager that selects it:

```bash
kubectl get alertmanagerconfig foretoken-control-plane-slack \
  --namespace "$ALERTMANAGER_NAMESPACE"
kubectl get pods --namespace "$ALERTMANAGER_NAMESPACE" \
  --selector app.kubernetes.io/name=alertmanager
```

Replace `<ALERTMANAGER_POD>` with a Pod from that instance and set `WORKLOAD_NAMESPACE` to a deployed Foretoken service's namespace. This uses the Pod's bundled `amtool` to send a temporary alert directly to Alertmanager, expiring after two minutes:

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

The channel should receive a firing notification, then a resolved notification after expiry. The test also reaches other receivers matching `service=foretoken`. If delivery fails, inspect the Alertmanager logs:

```bash
kubectl logs --namespace "$ALERTMANAGER_NAMESPACE" "$ALERTMANAGER_POD" \
  --container alertmanager --since=5m
```

## Remove the integration

Set `observability.notifications.slack.webhookSecret.name` to an empty string and repeat the install command. Helm removes the Slack receiver and leaves other notification channels unchanged. Delete the Secret when no longer needed:

```bash
kubectl delete secret foretoken-slack-webhook --namespace "$ALERTMANAGER_NAMESPACE"
```

`foretoken uninstall` also removes the receiver and preserves the Secret.
