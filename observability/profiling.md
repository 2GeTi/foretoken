<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# Profiling

English | [简体中文](profiling_zh.md)

Use PyTorch Profiler to inspect CPU/GPU execution during inference. Profiling currently supports vLLM on NVIDIA GPUs and requires a [source-installed](../docs/custom-deployment.md) CLI and platform. Results use persistent RuntimeCache storage, which the Quick Start already configures.

## Capture a benchmark workload

Run from the repository root:

```bash
pip install -e '.[bench]'
foretoken bench examples/quickstart \
  --profile --profile-engine pytorch --profile-duration 15s \
  --number 2 --max-tokens 128 --output local
```

This mode supports a single generated workload from a Kustomize deployment with the default `--rate -1`.

## Capture

To record traffic on a service that is already running:

```bash
foretoken profile examples/quickstart \
  --profile-engine pytorch --profile-duration 15s
```

This command does not generate requests. Use `--model MODEL_ID` for a multi-model deployment. `--profile-duration` sets the maximum recording time; benchmark capture also stops when the workload finishes early.

## Inspect results

Download the captured `.pt.trace.json` files to your computer and drag them into [Perfetto](https://ui.perfetto.dev/) to view the execution timeline.

Profiled benchmarks retain results and their storage. After saving the traces you need, clean up with:

```bash
foretoken delete examples/quickstart
```

Profiling adds overhead. Use a separate run without `--profile` for latency and throughput comparisons.
