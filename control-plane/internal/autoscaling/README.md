# Autoscaling Architecture

[English](README.md) | [中文](README_zh.md)

This package turns controller-owned observations into `ModelPool` capacity. Users configure autoscaling through `ModelService.spec.autoscaling`; configuration and status usage are documented in the [autoscaling guide](../../../../docs/autoscaling.md).

## Ownership

The `ModelService` controller owns scheduling, observation collection, target discovery, status publication, and writing capacity to `ModelPool`. Algorithms are side-effect-free: they only evaluate one complete observation and return a recommendation.

An aggregate target scales one Pool. An E/P/D target scales one `EPDPipelineScope`, applying the same capacity to its encoder, prefill, and decode Pools.

## Evaluation pipeline

```text
controller polling loop
→ ScalingSnapshot
→ TriggerDecision
→ ReplicaRecommendation
→ ReplicaAdjustment
→ ScalingDecision
→ ModelPool capacity and ModelService status
```

The controller supplies complete, fresh observations to the pipeline. `periodic` accepts those observations; it does not own an interval or requeue loop. The resolver applies hard min/max bounds even when observations are missing, and holds capacity while a target is transitioning.

`step` stabilization uses recent recommendations retained by the current controller process. The history is intentionally runtime-local, so a restart or leader change does not restore a pending scale-down delay.

## Extension boundary

Built-in algorithms live under `algorithm/`. Trigger, decision, and adjustment implementations return domain results and do not read Kubernetes resources, mutate capacity, or schedule work. Add a new implementation only when it represents a current, independently owned recommendation policy; controller lifecycle behavior remains in `core` and the ModelService reconciler.

`algorithm/registry.go` declares all built-in factories in one static registry. Implementations do not self-register through `init()`. To add a decision policy, implement its recommendation and parameter constructor, then add its factory to the decision registry. The constructor receives the `decision.parameters` JSON object, owns explicit parameter decoding, defaults and validation, and returns a policy using backend-neutral snapshots. Existing integer policies share an explicit field decoder; other parameter types belong to the policy that consumes them.

The CRD carries an algorithm name and a required parameters object without listing policy-specific fields. The controller constructs the selected pipeline once per reconciliation and reuses its resolved polling configuration. It rejects invalid configuration before writing Pools; the compiler only compiles model deployment intent. Adding a decision policy does not require API, compiler or controller dispatch branches. Update the user guide with the policy's parameters and behavior.

Trigger scheduling and adjustment history remain owned by the existing controller and pipeline boundaries. Their common configuration is independent of decision parameters.

## Validation

Use the control-plane verification target after changing this package:

```bash
make -C control-plane verify
```
