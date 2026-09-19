// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright contributors to the Foretoken project

package decision

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// decodeParameters reads the selected policy's explicit integer fields, preserving caller-owned defaults.
func decodeParameters(raw json.RawMessage, fields map[string]*int64) error {
	var parameters map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parameters); err != nil {
		return fmt.Errorf("autoscaling decision.parameters must be an object: %w", err)
	}
	if parameters == nil {
		return fmt.Errorf("autoscaling decision.parameters must be an object")
	}
	for name, value := range parameters {
		field, ok := fields[name]
		if !ok {
			return fmt.Errorf("unknown autoscaling decision parameter %q", name)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("autoscaling decision parameter %q must be an integer", name)
		}
		if err := json.Unmarshal(value, field); err != nil {
			return fmt.Errorf("autoscaling decision parameter %q must be an integer: %w", name, err)
		}
	}
	return nil
}
