// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright contributors to the Foretoken project

// Resolves ModelService autoscaling configuration and applies capacity decisions.

package controllers

import (
	"context"
	"fmt"
	"time"

	inferencev1alpha1 "github.com/shiweijiezero/foretoken/control-plane/api/v1alpha1"
	"github.com/shiweijiezero/foretoken/control-plane/internal/autoscaling"
	"github.com/shiweijiezero/foretoken/control-plane/internal/autoscaling/core"
	"github.com/shiweijiezero/foretoken/control-plane/internal/compiler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type modelScalingConfig struct {
	Autoscaler      *autoscaling.Autoscaler
	Limits          core.ReplicaLimits
	PollingInterval time.Duration
	MetricsMaxAge   time.Duration
}

// scalingConfig resolves one ModelService autoscaling configuration into runtime algorithms and limits.
func (reconciler *ModelServiceReconciler) scalingConfig(service *inferencev1alpha1.ModelService) (modelScalingConfig, error) {
	config := modelScalingConfig{
		Autoscaler:      autoscaling.Manual(),
		Limits:          core.ReplicaLimits{MinReplicas: 0, MaxReplicas: maxDesiredReplicas},
		PollingInterval: defaultScalingPollInterval,
		MetricsMaxAge:   3 * defaultScalingPollInterval,
	}
	autoscalingConfig := service.Spec.Autoscaling
	if autoscalingConfig == nil {
		return config, nil
	}

	pollingInterval, err := durationOrDefault(triggerInterval(autoscalingConfig.Trigger), defaultScalingPollInterval)
	if err != nil {
		return modelScalingConfig{}, fmt.Errorf("autoscaling trigger.interval: %w", err)
	}
	metricsMaxAge := 3 * pollingInterval
	scaleUpWindow, err := nonNegativeDurationOrDefault(scaleUpStabilizationWindow(autoscalingConfig.Adjustment), 0)
	if err != nil {
		return modelScalingConfig{}, fmt.Errorf("autoscaling adjustment.scaleUp.stabilizationWindow: %w", err)
	}
	scaleDownWindow, err := nonNegativeDurationOrDefault(scaleDownStabilizationWindow(autoscalingConfig.Adjustment), 5*time.Minute)
	if err != nil {
		return modelScalingConfig{}, fmt.Errorf("autoscaling adjustment.scaleDown.stabilizationWindow: %w", err)
	}

	selected, err := autoscaling.New(autoscaling.Configuration{
		DecisionAlgorithm:   autoscaling.DecisionAlgorithmName(autoscalingConfig.Decision.Algorithm),
		TriggerAlgorithm:    autoscaling.TriggerAlgorithmName(triggerAlgorithm(autoscalingConfig.Trigger)),
		AdjustmentAlgorithm: autoscaling.AdjustmentAlgorithmName(adjustmentAlgorithm(autoscalingConfig.Adjustment)),
		Decision:            decisionConfig(autoscalingConfig.Decision),
		Adjustment: core.AdjustmentConfig{
			ScaleUpStabilizationWindow:   scaleUpWindow,
			ScaleDownStabilizationWindow: scaleDownWindow,
			History:                      reconciler.autoscalingRecommendationHistory(),
		},
	})
	if err != nil {
		return modelScalingConfig{}, err
	}
	config.Autoscaler = selected
	config.Limits = core.ReplicaLimits{MinReplicas: autoscalingConfig.MinReplicas, MaxReplicas: autoscalingConfig.MaxReplicas}
	config.PollingInterval = pollingInterval
	config.MetricsMaxAge = metricsMaxAge
	return config, nil
}

func triggerAlgorithm(config *inferencev1alpha1.ModelAutoscalingTriggerConfig) inferencev1alpha1.AutoscalingTriggerAlgorithm {
	if config == nil {
		return ""
	}
	return config.Algorithm
}

func triggerInterval(config *inferencev1alpha1.ModelAutoscalingTriggerConfig) inferencev1alpha1.Duration {
	if config == nil {
		return ""
	}
	return config.Interval
}

func decisionConfig(config inferencev1alpha1.ModelAutoscalingDecisionConfig) core.DecisionConfig {
	decision := core.DecisionConfig{}
	if config.Queue != nil {
		decision.TargetAverageQueuedRequests = int64OrDefault(config.Queue.TargetAverageQueuedRequests, 1)
	}
	if config.QueueThreshold != nil {
		decision.ScaleUpQueuedRequests = int64OrDefault(config.QueueThreshold.ScaleUpQueuedRequests, 1)
		decision.ScaleDownQueuedRequests = int64OrDefault(config.QueueThreshold.ScaleDownQueuedRequests, 0)
	}
	return decision
}

func adjustmentAlgorithm(config *inferencev1alpha1.ModelAutoscalingAdjustmentConfig) inferencev1alpha1.AutoscalingAdjustmentAlgorithm {
	if config == nil {
		return ""
	}
	return config.Algorithm
}

func scaleUpStabilizationWindow(config *inferencev1alpha1.ModelAutoscalingAdjustmentConfig) inferencev1alpha1.NonNegativeDuration {
	if config == nil || config.ScaleUp == nil {
		return ""
	}
	return config.ScaleUp.StabilizationWindow
}

func scaleDownStabilizationWindow(config *inferencev1alpha1.ModelAutoscalingAdjustmentConfig) inferencev1alpha1.NonNegativeDuration {
	if config == nil || config.ScaleDown == nil {
		return ""
	}
	return config.ScaleDown.StabilizationWindow
}

func int32OrDefault(value *int32, fallback int32) int32 {
	if value == nil {
		return fallback
	}
	return *value
}

func int64OrDefault(value *int64, fallback int64) int64 {
	if value == nil {
		return fallback
	}
	return *value
}

func durationOrDefault(value inferencev1alpha1.Duration, fallback time.Duration) (time.Duration, error) {
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(string(value))
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("must be a positive duration")
	}
	return duration, nil
}

func nonNegativeDurationOrDefault(value inferencev1alpha1.NonNegativeDuration, fallback time.Duration) (time.Duration, error) {
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(string(value))
	if err != nil || duration < 0 {
		return 0, fmt.Errorf("must be a non-negative duration")
	}
	return duration, nil
}

// applyScaling evaluates autoscaling targets and returns compiled pools with applied capacity.
func (reconciler *ModelServiceReconciler) applyScaling(ctx context.Context, service *inferencev1alpha1.ModelService, compiledPools []compiler.ModelPool) ([]compiler.ModelPool, []inferencev1alpha1.AutoscalingTargetStatus, error) {
	owned, err := reconciler.ownedPools(ctx, service)
	if err != nil {
		return nil, nil, err
	}
	byPoolName := make(map[string]*inferencev1alpha1.ModelPool, len(owned))
	for index := range owned {
		pool := &owned[index]
		byPoolName[pool.Spec.PoolName] = pool
	}
	var groupList inferencev1alpha1.ModelGroupList
	if err := reconciler.List(ctx, &groupList, client.InNamespace(service.Namespace)); err != nil {
		return nil, nil, fmt.Errorf("list ModelGroups: %w", err)
	}
	scaling, err := reconciler.scalingConfig(service)
	if err != nil {
		return nil, nil, err
	}

	// Every Pool owns its capacity independently. Multi-stage services retain one atomic
	// serving cohort, while each encoder, prefill, and decode Pool converges separately.
	evaluatedAt := metav1.Now()
	snapshots := make([]core.ScalingSnapshot, 0, len(compiledPools))
	for _, compiled := range compiledPools {
		pool := byPoolName[compiled.Name]
		current := compiled.DesiredGroups
		transitioning := false
		poolUID := ""
		if pool != nil {
			current = pool.Spec.DesiredGroups
			transitioning = modelPoolTransitioning(pool)
			poolUID = string(pool.UID)
		}
		target := core.TargetID{
			ServiceNamespace: service.Namespace,
			ServiceName:      service.Name,
			ServiceUID:       string(service.UID),
			Name:             compiled.Name,
			UID:              poolUID,
			Kind:             core.TargetPool,
			Role:             autoscalingRole(compiled.Template.Role),
		}
		replicaState := modelPoolReplicaState(service, pool, groupList.Items)
		replicaState.BaselineReplicas = compiled.DesiredGroups
		replicaState.RequestedReplicas = current
		replicaState.Transitioning = replicaState.Transitioning || transitioning
		finalizeReplicaState(&replicaState)
		// A Pool not yet created has no transition; its first controller
		// write remains eligible for the selected algorithm's bootstrap core.
		if pool == nil {
			replicaState.Transitioning = false
		}
		snapshots = append(snapshots, reconciler.scalingSnapshot(ctx, service, target, evaluatedAt, replicaState, scaling))
	}
	decisions, err := scaling.Autoscaler.Plan(snapshots)
	if err != nil {
		return nil, nil, err
	}
	if len(decisions) != len(snapshots) {
		return nil, nil, fmt.Errorf("autoscaler returned %d decisions for %d scaling targets", len(decisions), len(snapshots))
	}

	bySnapshotTarget := make(map[core.TargetID]core.ScalingSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		bySnapshotTarget[snapshot.Target] = snapshot
	}
	byTarget := make(map[core.TargetID]int32, len(decisions))
	statuses := make([]inferencev1alpha1.AutoscalingTargetStatus, 0, len(decisions))
	for _, decision := range decisions {
		if _, exists := byTarget[decision.Target]; exists {
			return nil, nil, fmt.Errorf("autoscaler returned duplicate decision for target %q", decision.Target.Name)
		}
		snapshot, exists := bySnapshotTarget[decision.Target]
		if !exists {
			return nil, nil, fmt.Errorf("autoscaler returned unknown target %q", decision.Target.Name)
		}
		byTarget[decision.Target] = decision.AppliedReplicas

		var trigger *inferencev1alpha1.AutoscalingStageStatus
		if algorithm := scaling.Autoscaler.TriggerAlgorithmName(); algorithm != "" {
			trigger = &inferencev1alpha1.AutoscalingStageStatus{
				Algorithm:   algorithm,
				Disposition: string(decision.Trigger.Disposition),
				Reason:      string(decision.Trigger.Reason),
				Message:     decision.Trigger.Message,
			}
		}
		var constraint *inferencev1alpha1.AutoscalingConstraintStatus
		if decision.Constraint != "" {
			constraint = &inferencev1alpha1.AutoscalingConstraintStatus{Reason: string(decision.Constraint), Message: decision.Message}
		}
		var observationEndAt *metav1.Time
		if !snapshot.Metrics.Window.End.IsZero() {
			value := metav1.NewTime(snapshot.Metrics.Window.End)
			observationEndAt = &value
		}
		adjustmentDisposition := "Hold"
		if decision.Adjustment.Reason != core.AdjustmentReasonHold {
			adjustmentDisposition = "Apply"
		}
		statuses = append(statuses, inferencev1alpha1.AutoscalingTargetStatus{
			ID:               fmt.Sprintf("%s/%s", decision.Target.Kind, decision.Target.Name),
			Kind:             string(decision.Target.Kind),
			Role:             string(decision.Target.Role),
			EvaluatedAt:      metav1.NewTime(snapshot.EvaluatedAt),
			ObservationEndAt: observationEndAt,
			ObservationState: string(snapshot.Metrics.State),
			Trigger:          trigger,
			Decision: inferencev1alpha1.AutoscalingDecisionStatus{
				AutoscalingStageStatus: inferencev1alpha1.AutoscalingStageStatus{
					Algorithm:   decision.DecisionAlgorithm,
					Disposition: string(decision.Recommendation.State),
					Reason:      string(decision.Recommendation.Reason),
					Message:     decision.Recommendation.Message,
				},
				DesiredReplicas: decision.Recommendation.Replicas,
			},
			Adjustment: inferencev1alpha1.AutoscalingAdjustmentStatus{
				AutoscalingStageStatus: inferencev1alpha1.AutoscalingStageStatus{
					Algorithm:   decision.AdjustmentAlgorithm,
					Disposition: adjustmentDisposition,
					Reason:      string(decision.Adjustment.Reason),
					Message:     decision.Adjustment.Message,
				},
				AdjustedReplicas: decision.Adjustment.Replicas,
			},
			Constraint:       constraint,
			Direction:        string(decision.Direction),
			AppliedReplicas:  decision.AppliedReplicas,
			ReadyReplicas:    snapshot.Replicas.ReadyReplicas,
			RoutableReplicas: snapshot.Replicas.RoutableReplicas,
		})
	}
	resolved := append([]compiler.ModelPool(nil), compiledPools...)
	for index := range resolved {
		compiled := resolved[index]
		poolUID := ""
		if pool := byPoolName[compiled.Name]; pool != nil {
			poolUID = string(pool.UID)
		}
		target := core.TargetID{ServiceNamespace: service.Namespace, ServiceName: service.Name, ServiceUID: string(service.UID), Name: compiled.Name, UID: poolUID, Kind: core.TargetPool, Role: autoscalingRole(compiled.Template.Role)}
		desired, exists := byTarget[target]
		if !exists {
			return nil, nil, fmt.Errorf("autoscaler omitted target %q", target.Name)
		}
		resolved[index].DesiredGroups = desired
	}
	return resolved, statuses, nil
}

// scalingSnapshot builds one replica and metrics input for the autoscaling pipeline.
func (reconciler *ModelServiceReconciler) scalingSnapshot(ctx context.Context, service *inferencev1alpha1.ModelService, target core.TargetID, evaluatedAt metav1.Time, replicas core.ReplicaState, scaling modelScalingConfig) core.ScalingSnapshot {
	metrics := core.MetricsSnapshot{State: core.MetricsUnavailable}
	if scaling.Autoscaler.Automatic() {
		metrics = reconciler.metricsSnapshot(ctx, target, scaling.MetricsMaxAge)
	}
	return core.ScalingSnapshot{
		Target:      target,
		EvaluatedAt: evaluatedAt.Time,
		Replicas:    replicas,
		Limits:      scaling.Limits,
		Metrics:     metrics,
	}
}

// metricsSnapshot fails closed: a missing or failed provider can never be interpreted as zero demand.
func (reconciler *ModelServiceReconciler) metricsSnapshot(ctx context.Context, target core.TargetID, maxAge time.Duration) core.MetricsSnapshot {
	if reconciler.MetricsProvider == nil {
		return core.MetricsSnapshot{State: core.MetricsUnavailable}
	}
	metrics, err := reconciler.MetricsProvider.Snapshot(ctx, target)
	if err != nil || metrics.State == "" {
		return core.MetricsSnapshot{State: core.MetricsUnavailable}
	}
	if metrics.State == core.MetricsFresh {
		if metrics.Window.End.IsZero() || metrics.Window.CollectedAt.IsZero() {
			return core.MetricsSnapshot{State: core.MetricsUnavailable}
		}
		age := time.Since(metrics.Window.End)
		if age < 0 || age > maxAge {
			metrics.State = core.MetricsStale
		}
	}
	return metrics
}

func autoscalingRole(role inferencev1alpha1.ModelRole) core.TargetRole {
	switch role {
	case inferencev1alpha1.ModelRoleEncoder:
		return core.RoleEncoder
	case inferencev1alpha1.ModelRolePrefill:
		return core.RolePrefill
	case inferencev1alpha1.ModelRoleDecode:
		return core.RoleDecode
	default:
		return core.RoleAggregate
	}
}
