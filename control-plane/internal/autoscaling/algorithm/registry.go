// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright contributors to the Foretoken project

// Package algorithm holds the process-wide registry for statically linked autoscaling implementations.
package algorithm

import (
	"encoding/json"
	"fmt"

	"github.com/shiweijiezero/foretoken/control-plane/internal/autoscaling/algorithm/adjustment"
	"github.com/shiweijiezero/foretoken/control-plane/internal/autoscaling/algorithm/decision"
	"github.com/shiweijiezero/foretoken/control-plane/internal/autoscaling/algorithm/trigger"
	"github.com/shiweijiezero/foretoken/control-plane/internal/autoscaling/core"
)

// Built-in implementations are declared together and remain fixed for the process lifetime.
var triggers = map[string]func() (core.TriggerAlgorithm, error){
	"periodic": func() (core.TriggerAlgorithm, error) { return trigger.Periodic{}, nil },
}

var decisions = map[string]func(json.RawMessage) (core.DecisionAlgorithm, error){
	"manual":          func(json.RawMessage) (core.DecisionAlgorithm, error) { return decision.Manual{}, nil },
	"queue":           decision.NewQueue,
	"queue_threshold": decision.NewQueueThreshold,
}

var adjustments = map[string]func(core.AdjustmentConfig) (core.AdjustmentAlgorithm, error){
	"direct": func(core.AdjustmentConfig) (core.AdjustmentAlgorithm, error) { return adjustment.Direct{}, nil },
	"step":   adjustment.NewStep,
}

// BuildTrigger constructs a named trigger algorithm from the builtin registry.
func BuildTrigger(name string) (core.TriggerAlgorithm, error) {
	factory, ok := triggers[name]
	if !ok {
		return nil, fmt.Errorf("unknown autoscaling trigger algorithm %q", name)
	}
	return factory()
}

// BuildDecision constructs a named decision algorithm from the builtin registry.
func BuildDecision(name string, config json.RawMessage) (core.DecisionAlgorithm, error) {
	factory, ok := decisions[name]
	if !ok {
		return nil, fmt.Errorf("unknown autoscaling decision algorithm %q", name)
	}
	return factory(config)
}

// BuildAdjustment constructs a named adjustment algorithm from the builtin registry.
func BuildAdjustment(name string, config core.AdjustmentConfig) (core.AdjustmentAlgorithm, error) {
	factory, ok := adjustments[name]
	if !ok {
		return nil, fmt.Errorf("unknown autoscaling adjustment algorithm %q", name)
	}
	return factory(config)
}
