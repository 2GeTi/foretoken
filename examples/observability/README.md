<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# Observability example

English | [简体中文](README_zh.md)

Runs the [Quick Start](../quickstart/README.md) model service with metrics and the Grafana dashboard. Alerts stay disabled unless their names are listed in `observability.alerts.rules` in `observability.yaml`; the same file holds thresholds and notification language. From the repository root:

```bash
foretoken install --values examples/observability/observability.yaml
foretoken deploy examples/quickstart
```

Send a few requests, then open Grafana and select **Foretoken System Overview**. Enabled alert thresholds appear as dashed lines on the matching panels. The [observability guide](../../observability/README.md) explains how to find Grafana, reuse an existing monitoring stack, and read the alerts.
