<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# Profile an existing service

English | [简体中文](profiling_zh.md)

Collect a short PyTorch CPU/GPU timeline while an existing model service handles requests. This experimental feature requires a source installation and currently supports the vLLM PyTorch profiler on NVIDIA GPUs.

## Capture

Use the Kustomize directory that identifies the deployed service:

```bash
foretoken profile examples/quickstart \
  --profile-engine pytorch \
  --profile-duration 15s
```

The command reads the directory to identify the deployed service without applying it. If it contains several models, select one with `--model MODEL_ID`. It does not generate traffic; send requests through the normal frontend while capture is running.

The selected ModelService must use persistent RuntimeCache storage. The maintained Quick Start already declares it in `cache.yaml`; other deployments can follow [Model storage](../docs/model-storage.md). Profiling writes to the `profiles/` directory on the same RuntimeCache PVC. A service without persistent RuntimeCache storage must be redeployed with one before capture.

The runtime starts the recording after profiler startup, stops after the requested duration, and then exports the files. Export may take longer than recording. Normal completion leaves the model serving. The command prints the ProfileRun name and, on completion, the RuntimeCache PVC and path containing the results.

| Option | Meaning |
|---|---|
| `--profile-engine pytorch` | Required profiler selection; only PyTorch is available |
| `--profile-duration 15s` | Required recording duration; excludes startup and export |
| `--model MODEL_ID` | Select one model from a multi-model directory |
| `--timeout 10m` | How long the CLI observes the run, not how long the runtime records |

Ctrl-C requests cancellation and retains available output. After a lost terminal or observation timeout, capture still ends at its original deadline. Use the printed inspection command to check progress.

## Capture a benchmark workload

To deploy or reuse a service, generate requests and capture them in one command, install the benchmark dependencies from source (`pip install -e '.[bench]'`):

```bash
foretoken bench examples/quickstart \
  --profile --profile-engine pytorch --profile-duration 15s \
  --number 2 --max-tokens 128 --output local
```

The command deploys missing services or reuses an existing deployment. The service must use persistent RuntimeCache storage. Requests start after capture is active. When the workload finishes, the command ends capture and waits for trace export; if the recording duration ends first, the benchmark continues. Ctrl-C or a workload failure requests cancellation and waits for it to complete before cleanup.

Temporary serving resources are removed after capture completes. Once serving is ready, profiling cleanup retains the deployment's storage resources and namespace so results survive, including cancelled or failed captures. Existing deployments are left unchanged. If capture stop cannot be confirmed, the command fails and retains the deployment for inspection. Use the printed ProfileRun and PVC references to inspect output; after saving the traces you need, run `foretoken delete PATH` to explicitly remove the remaining deployment resources.

This mode supports one generated workload with the default `--rate -1`; it does not support `--url`, trace replay, sweeps or multiple datasets. `--wait-timeout` limits each wait for capture startup and export. Profiling adds overhead, so use a separate benchmark without `--profile` for latency and throughput measurements.

## Inspect results

Each runtime stores one manifest and its native `.pt.trace.json` files below:

```text
profiles/runs/<run-uid>/<runtime-id>/
```

The Quick Start examples store results under the repository-root `data/profiles/runs/`. For other deployments, use the PVC and relative path printed by the command.

Profiling adds CPU/GPU overhead and can produce large files even in a short window. Use a small diagnostic deployment and a short duration. A native profiler failure may terminate that runtime, so use a service where interruption is acceptable.
