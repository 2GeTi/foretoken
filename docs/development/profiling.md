<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# Profiling lifecycle

English | [简体中文](profiling_zh.md)

`ProfileRun` owns a bounded capture independently of the initiating command. Usage and result access are covered in [Profiling](../../observability/profiling.md).

## Execution and recovery

The controller persists the serving generation, participant identities and RuntimeCache binding before starting capture. Reconciliation resumes that fixed plan after a restart; a replaced participant or changed serving cohort cancels the run. Kubernetes RBAC governs ProfileRun operations, and runtime control uses the existing internal HTTP boundary.

The model-server supervisor owns native start, recording, stop and export. It accepts one active capture, makes same-run retries idempotent and keeps cancellation irreversible. The recording timer starts after native startup; startup and stop/export have separate 30-second and 120-second budgets. `Finish` ends the recording early.

`Capturing` requires all selected participants to be recording; success requires every participant's exported result. A live run's deletion requests cancellation, and its finalizer remains until stop is confirmed. If native shutdown fails, the supervisor closes admission and terminates its engine process group, which can interrupt inference.

## Storage

Each participant writes to the ModelService's persistent RuntimeCache. A runtime using Pod-local fallback storage cannot capture. After stop/export, it validates the expected worker traces, writes a manifest and atomically renames its staging directory into `profiles/runs/<run-uid>/<runtime-id>/` on the same filesystem. GPU activity is reported separately from trace validity.

ProfileRun publishes the result location but does not own the stored files. Deleting the run record preserves them. The model-server adapter supplies the [vLLM profiling backport](../../data-plane/patches/vllm-python-profiling.patch) for native error reporting and repeated captures.

## Benchmark integration

Benchmark profiling reuses the ordinary service resolver and EvalScope executor. Prepared requests wait for capture readiness; completion requests `Finish`, while failures and interruption request `Cancel`. Capture must reach a terminal state before temporary serving resources are removed; an unconfirmed stop leaves them available for recovery.

The benchmark removes only resources it created. Once serving is ready, profiled runs retain the namespace and storage for result access; setup failures use ordinary cleanup. Existing deployments remain untouched.

## References

- [vLLM profiling](https://docs.vllm.ai/en/stable/contributing/profiling/)
- [PyTorch profiler](https://docs.pytorch.org/docs/stable/profiler.html)
