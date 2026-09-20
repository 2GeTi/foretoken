<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# Autoscale Model Services

[English](autoscaling.md) | [中文](autoscaling_zh.md)

Autoscaling changes the capacity of a `ModelService` from request demand. Configure it in the service manifest, then inspect the service status while a workload is running.

## Capacity units

For an aggregate model service, each Pool replica runs the complete model with its configured resources and parallelism. A service with separate encoder, prefill, and decode stages (E/P/D) has one Pool for each stage, and the three Pools own and scale their replica capacities independently. Requests still require a complete E/P/D route, and new Pool revisions enter service together as one complete cohort.

`spec.replicas` provides the aggregate shorthand's baseline capacity. Advanced `modelPools` use each Pool's `replicas`. When `autoscaling` is present, `minReplicas` and `maxReplicas` constrain every Pool from the first reconciliation onward.

## Configure queue autoscaling

Add this `spec` fragment to an existing `ModelService`. It starts at one replica, maintains one to eight replicas, evaluates recent queue demand every five seconds, and changes at most one replica per evaluation:

```yaml
spec:
  replicas: 1
  autoscaling:
    minReplicas: 1
    maxReplicas: 8
    trigger:
      algorithm: periodic
      interval: 5s
    decision:
      algorithm: queue
      queue:
        targetAverageQueuedRequests: 1
    adjustment:
      algorithm: step
      scaleUp:
        stabilizationWindow: 0s
      scaleDown:
        stabilizationWindow: 300s
```

`periodic` evaluates queue demand at the configured interval. Missing, stale, or incomplete observations keep the current capacity. Automatic scaling maintains at least one replica.

`queue` calculates capacity from the average queued requests per replica. `queue_threshold` instead changes capacity by one replica at configured total-backlog boundaries. `direct` applies a recommendation after min/max bounds; `step` applies at most one replica per evaluation and supports independent stabilization windows.

The scale-down window uses recent recommendations held by the current controller process. A controller restart or leadership change does not preserve that history, so it can shorten a pending scale-down delay.

## Observe a decision

Autoscaling results are published in `.status.autoscaling[]`, one entry for each scaling target. Query the maintained multi-model example with:

```bash
kubectl get modelservice multi-model-qwen3-0.6b \
  --namespace foretoken-multi-model-demo \
  -o json | jq '.status.autoscaling[] | {
    id,
    kind,
    role,
    observationState,
    direction,
    desiredReplicas: .decision.desiredReplicas,
    adjustedReplicas: .adjustment.adjustedReplicas,
    appliedReplicas,
    constraint: .constraint.reason
  }'
```

`desiredReplicas` is the algorithm recommendation. `adjustedReplicas` is the result after stabilization and rate limiting. `appliedReplicas` is the capacity written to the target after lifecycle and min/max constraints. `observationState`, the stage reasons, and `constraint` explain why capacity was held or changed.

`kind` is `Pool`. The `role` identifies whether that Pool serves Aggregate, Encoder, Prefill, or Decode work.

## Try the maintained example

The [multi-model example](../examples/multi-model-quickstart/README.md) deploys one queue-autoscaled Qwen service and one fixed-capacity Llama service. It includes a bounded concurrent workload and status commands for observing capacity changes.

## Maintainer architecture

The controller stages, observation aggregation, algorithm extension boundary, and lifecycle resolver are documented in [the autoscaling maintainer README](../control-plane/internal/autoscaling/README.md).
