<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# Foretoken alert runbooks

[简体中文](alerts_zh.md) | English

Foretoken alerts are sustained warning signals. They do not trigger remediation
and do not by themselves prove a user-visible outage. Start with the labels on
the alert, then confirm the signal against the current Kubernetes state.

Select rules in the corresponding service's `spec.observability.alerts.rules`:

| Alert | Service | Signal | Persistence |
| --- | --- | --- | --- |
| `ForetokenMetricsTargetDown` | FrontendService or ModelService | A discovered `/metrics` target cannot be scraped | 1 minute |
| `ForetokenFrontendHTTPResponseStart5xxRatioHigh` | FrontendService | Response-start 5xx ratio is high while traffic exists | 2 minutes |
| `ForetokenModelServerSchedulerBacklog` | ModelService | Aggregated vLLM stage scheduler waiting queue is nonzero | 2 minutes |
| `ForetokenModelServerKVCachePressureHigh` | ModelService | Maximum vLLM KV-cache usage is high | 2 minutes |
| `ForetokenNVIDIAGPUTemperatureHigh` | ModelService | GPU temperature exceeds the configured threshold | 2 minutes |
| `ForetokenNVIDIAGPUPowerUsageHigh` | ModelService | GPU power exceeds an explicitly configured threshold | 5 minutes |

Inspect the resources in the affected namespace:

```bash
NAMESPACE=foretoken-demo
kubectl get pods,services,endpointslices --namespace "$NAMESPACE" -o wide
```

If a recording series disappears, inspect the raw `up` metric and Pod health
first. Missing metrics mean unavailable data, not a value of zero.

Accelerator alerts use recording series attributed to Foretoken workloads by
the exporter's model-group and model-role Pod labels. Samples without those
labels are excluded, including from temperature and power alerts.

## ForetokenMetricsTargetDown

Prometheus has continuously failed to scrape an already discovered Frontend or
model-server target for one minute.

1. Open the Prometheus Targets page and inspect the target's `lastError`.
2. Check whether the selected Pod is running and whether its `/metrics`
   endpoint responds from a monitoring Pod.
3. Check the Service endpoint, named port, ServiceMonitor selector, and
   NetworkPolicy.
4. If the application is unhealthy, inspect its logs and recent rollout events.

This alert checks scrape reachability only. It does not detect a target that
vanishes completely from service discovery, and `up == 0` is not equivalent to
request readiness or a confirmed user outage.

## ForetokenFrontendHTTPResponseStart5xxRatioHigh

More than 5% of Frontend HTTP response starts have been 5xx for two minutes,
while traffic has remained at or above 0.1 response starts per second.

1. Filter the recorded response-start metrics to the alert's namespace and
   Frontend service, then split them by handler and status.
2. Inspect Frontend logs and recent configuration or routing changes.
3. Check backend availability if the affected handler performs inference.

The metric records the status when the HTTP response starts. A stream that
fails later may still have started with 2xx, so this is not an inference failure
ratio or SLO.

## ForetokenModelServerSchedulerBacklog

The aggregated vLLM stage scheduler queue has remained nonzero for two minutes.

1. Inspect running and waiting requests for the alert's model group and role.
2. Check KV-cache pressure, Pod health, accelerator utilization, and recent
   traffic changes.
3. Compare the affected prefill or decode role separately; do not interpret
   stage counts as user-level request counts.

This is a vLLM stage queue, not the number of users and not the Frontend
admission queue. A transient nonzero queue is normal; the alert requires a
continuous two-minute backlog.

## ForetokenModelServerKVCachePressureHigh

The highest KV-cache usage among engines in a model group has remained at or
above the configured threshold (95% by default) for two minutes.

1. Confirm the group, role, and model labels, then inspect scheduler waiting and
   running requests.
2. Check request lengths, concurrency, workload configuration, and replica
   health before changing capacity.
3. Compare individual Pods or engines to find the hotspot.

The recorded value is the maximum across engines at each evaluation, not a
fleet average; the engine contributing the maximum can change over time. Tune
`spec.observability.alerts.thresholds.kvCacheUsageRatio` from measured workload
behavior.

## ForetokenNVIDIAGPUTemperatureHigh

An NVIDIA DCGM temperature reading has stayed above the configured threshold.
Check node airflow, device health, power usage, and workload placement. No alert
fires without a Foretoken-attributed NVIDIA DCGM temperature series.

## ForetokenNVIDIAGPUPowerUsageHigh

Select this rule in [`spec.observability.alerts.rules`](../README.md#alerts) and set
`spec.observability.alerts.thresholds.nvidiaPowerWatts` to a positive value in watts.
Remove its name from the list to disable it; power metrics remain available.

An NVIDIA DCGM power reading has stayed above the configured threshold. Compare
the reading with the device power limit and workload, then inspect thermal and
node health before changing capacity. No alert fires without a
Foretoken-attributed NVIDIA DCGM power series.
