<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: Copyright contributors to the Foretoken project
-->

# Profiling lifecycle

English | [简体中文](profiling_zh.md)

`ProfileRun` manages a bounded capture independently of the initiating command. Usage and results are covered in [Profiling](../../observability/profiling.md).

## Execution and recovery

The controller retains the capture's participant and storage bindings across restarts. A changed serving generation or replaced participant cancels the run. Each model-server accepts one active capture; retries for the same run are idempotent, and cancellation is irreversible.

`Capturing` requires every participant to be recording. Success requires all results to be exported. Deleting an active run requests cancellation and waits for confirmed shutdown before releasing its finalizer. If native shutdown fails, the supervisor closes admission and terminates the engine process group.

## Benchmark integration

Benchmark requests wait for capture readiness. Workload completion requests `Finish`; failure or interruption requests `Cancel`. Temporary services are removed only after capture termination is confirmed. An unconfirmed stop leaves the services available for recovery.

## Results

Capture requires persistent RuntimeCache storage. Results are published after worker traces are validated; GPU activity and trace validity are reported separately. Deleting a ProfileRun does not delete its files. Profiled benchmarks retain result storage and its namespace after serving cleanup.

Native error reporting and repeated captures are supported by the [vLLM profiling backport](../../data-plane/patches/vllm-python-profiling.patch).
