// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright contributors to the Foretoken project

// Compiles validated ModelPool templates and ModelGroup contracts for vLLM.

package vllm

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	inferencev1alpha1 "github.com/shiweijiezero/foretoken/control-plane/api/v1alpha1"
)

// EffectiveConfig contains typed vLLM values and approved backend arguments.
type EffectiveConfig struct {
	Model             string
	Source            inferencev1alpha1.ModelSource
	Revision          string
	Tokenizer         string
	TokenizerRevision string
	Parallelism       inferencev1alpha1.CompiledParallelism
	EngineArgs        inferencev1alpha1.EngineArguments
}

// LaunchPlanV1 is the versioned, private Go-to-Rust launch contract. Rust is
// the only component that renders this contract into vLLM command-line flags.
type LaunchPlanV1 struct {
	Version                               int                               `json:"version"`
	NodeCount                             int32                             `json:"nodeCount"`
	Artifacts                             LaunchArtifacts                   `json:"artifacts"`
	Parallelism                           LaunchParallelism                 `json:"parallelism"`
	KV                                    LaunchKVPlan                      `json:"kv"`
	EC                                    *LaunchECPlan                     `json:"ec,omitempty"`
	Lifecycle                             LaunchLifecycle                   `json:"lifecycle"`
	InternalGenerateRequestBodyLimitBytes int64                             `json:"internalGenerateRequestBodyLimitBytes"`
	EngineArgs                            inferencev1alpha1.EngineArguments `json:"engineArgs,omitempty"`
}

type LaunchArtifacts struct {
	Model             string                        `json:"model"`
	Source            inferencev1alpha1.ModelSource `json:"source"`
	Revision          string                        `json:"revision"`
	Tokenizer         string                        `json:"tokenizer"`
	TokenizerRevision string                        `json:"tokenizerRevision"`
}

type LaunchParallelism struct {
	TP  int32             `json:"tp"`
	PP  int32             `json:"pp"`
	DP  int32             `json:"dp"`
	PCP int32             `json:"pcp"`
	DCP int32             `json:"dcp"`
	EP  *LaunchExpertPlan `json:"ep,omitempty"`
}

type LaunchExpertPlan struct {
	Backend string `json:"backend,omitempty"`
	EPLB    bool   `json:"eplb"`
}

// LaunchKVPlan uses a closed kind discriminator rather than an untyped vLLM
// config map. KV Events are the fixed controller configuration for single-DP groups.
type LaunchKVPlan struct {
	Kind        string `json:"kind"`
	Role        string `json:"role,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	DeviceName  string `json:"deviceName,omitempty"`
	CPUBytes    int64  `json:"cpuBytes,omitempty"`
	StoragePath string `json:"storagePath,omitempty"`
	Events      bool   `json:"events"`
}

// LaunchECPlan is the sole typed source for --ec-transfer-config. It exposes no
// arbitrary connector arguments, module paths, or peer address fields.
type LaunchECPlan struct {
	ProfileName       string `json:"profileName"`
	ProfileRevision   string `json:"profileRevision"`
	Connector         string `json:"connector"`
	Role              string `json:"role"`
	SharedStoragePath string `json:"sharedStoragePath"`
}

type LaunchLifecycle struct {
	StartupSeconds int64 `json:"startupSeconds"`
	DrainSeconds   int64 `json:"drainSeconds"`
}

const (
	// FilesystemOffloadMountPath is the writable volume target shared by workload and launch plan.
	FilesystemOffloadMountPath = "/var/lib/foretoken/kv-offload"
	maxKubernetesInt32Seconds  = int64(1<<31 - 1)

	kvNone              = "none"
	kvPD                = "pd"
	kvCPUOffload        = "cpuOffload"
	kvFilesystemOffload = "filesystemOffload"
	kvMooncakeStore     = "mooncakeStore"
	kvMultiConnector    = "multiConnector"
)

// Compile resolves native engine options and topology from the normalized template.
func Compile(template inferencev1alpha1.NormalizedPoolTemplate) (EffectiveConfig, error) {
	effective := EffectiveConfig{
		Model: template.Model, Source: template.Source, Revision: template.ModelRevision,
		Tokenizer: template.Tokenizer, TokenizerRevision: template.TokenizerRevision,
		Parallelism: copyParallelism(template.Parallelism),
	}
	if effective.Tokenizer == "" {
		effective.Tokenizer = effective.Model
	}
	if effective.TokenizerRevision == "" {
		effective.TokenizerRevision = effective.Revision
	}
	args, err := compileEngineArgs(template.EngineArgs, template.Inference)
	if err != nil {
		return EffectiveConfig{}, err
	}
	effective.EngineArgs = args
	if err := validateParallelism(effective.Parallelism); err != nil {
		return EffectiveConfig{}, err
	}
	capacity := int64(template.NodeCount) * int64(template.Resources.Requests.GPU.Count)
	ranks := int64(effective.Parallelism.PP) * int64(effective.Parallelism.TP) * int64(effective.Parallelism.PCP) * int64(effective.Parallelism.DP)
	if capacity != ranks {
		return EffectiveConfig{}, fmt.Errorf("vLLM topology requires %d workers but the Pool provides %d accelerators", ranks, capacity)
	}
	return effective, nil
}

// BuildLaunchPlan projects a verified ModelGroupSpec into the private launch wire contract.
func BuildLaunchPlan(group inferencev1alpha1.ModelGroupSpec) (LaunchPlanV1, error) {
	if group.NodeCount != 1 {
		return LaunchPlanV1{}, fmt.Errorf("model-server launch plan currently supports exactly one node")
	}
	startup, err := parsePositiveDuration(group.Timeouts.Startup, "startup")
	if err != nil {
		return LaunchPlanV1{}, err
	}
	drain, err := parsePositiveDuration(group.Timeouts.Drain, "drain")
	if err != nil {
		return LaunchPlanV1{}, err
	}
	if group.Artifacts.Model == "" || group.Artifacts.Source == "" || group.Artifacts.ModelRevision == "" || group.Artifacts.Tokenizer == "" || group.Artifacts.TokenizerRevision == "" {
		return LaunchPlanV1{}, fmt.Errorf("vLLM artifacts must be nonempty")
	}
	if err := validateParallelism(group.Parallelism); err != nil {
		return LaunchPlanV1{}, err
	}
	if group.Runtime.InternalGenerateRequestBodyLimitBytes < inferencev1alpha1.MinInternalGenerateRequestBodyLimitBytes || group.Runtime.InternalGenerateRequestBodyLimitBytes > inferencev1alpha1.MaxInternalGenerateRequestBodyLimitBytes {
		return LaunchPlanV1{}, fmt.Errorf("vLLM internal generate request body limit must be between %d and %d", inferencev1alpha1.MinInternalGenerateRequestBodyLimitBytes, inferencev1alpha1.MaxInternalGenerateRequestBodyLimitBytes)
	}
	parallelism := LaunchParallelism{TP: group.Parallelism.TP, PP: group.Parallelism.PP, DP: group.Parallelism.DP, PCP: group.Parallelism.PCP, DCP: group.Parallelism.DCP}
	if group.Parallelism.EP != nil {
		parallelism.EP = &LaunchExpertPlan{Backend: group.Parallelism.EP.Backend, EPLB: group.Parallelism.EP.EPLB}
	}
	kv, err := buildKVPlan(group)
	if err != nil {
		return LaunchPlanV1{}, err
	}
	ec, err := buildECPlan(group)
	if err != nil {
		return LaunchPlanV1{}, err
	}
	return LaunchPlanV1{Version: 1, NodeCount: group.NodeCount, Artifacts: LaunchArtifacts{Model: group.Artifacts.Model, Source: group.Artifacts.Source, Revision: group.Artifacts.ModelRevision, Tokenizer: group.Artifacts.Tokenizer, TokenizerRevision: group.Artifacts.TokenizerRevision}, Parallelism: parallelism, KV: kv, EC: ec, Lifecycle: LaunchLifecycle{StartupSeconds: startup, DrainSeconds: drain}, InternalGenerateRequestBodyLimitBytes: group.Runtime.InternalGenerateRequestBodyLimitBytes, EngineArgs: group.Runtime.EngineArgs.DeepCopy()}, nil
}

// JSON returns deterministic output because LaunchPlanV1 uses only ordered structs and slices.
func (plan LaunchPlanV1) JSON() (string, error) {
	bytes, err := json.Marshal(plan)
	return string(bytes), err
}

// buildKVPlan projects the ModelGroup cache runtime into the closed vLLM KV launch plan.
func buildKVPlan(group inferencev1alpha1.ModelGroupSpec) (LaunchKVPlan, error) {
	role := "kv_both"
	if group.Role == inferencev1alpha1.ModelRolePrefill {
		role = "kv_producer"
	} else if group.Role == inferencev1alpha1.ModelRoleDecode {
		role = "kv_consumer"
	}
	plan := LaunchKVPlan{Kind: kvNone, Events: group.Parallelism.DP == 1}
	if group.PDRuntime != nil && group.KVRuntime == nil {
		return LaunchKVPlan{Kind: kvPD, Role: role, Protocol: group.PDRuntime.Protocol, DeviceName: group.PDRuntime.RDMADeviceName, Events: plan.Events}, nil
	}
	if group.KVRuntime == nil {
		return plan, nil
	}
	if group.KVRuntime.Offload != nil {
		offload := group.KVRuntime.Offload
		if offload.CPUBytes < 1 {
			return LaunchKVPlan{}, fmt.Errorf("vLLM KV offload cpuBytes must be positive")
		}
		kind := kvCPUOffload
		storagePath := ""
		if offload.Filesystem {
			kind = kvFilesystemOffload
			storagePath = FilesystemOffloadMountPath
		}
		return LaunchKVPlan{Kind: kind, CPUBytes: offload.CPUBytes, StoragePath: storagePath, Events: plan.Events}, nil
	}
	if group.KVRuntime.MooncakeStore != nil {
		if group.PDRuntime != nil {
			return LaunchKVPlan{Kind: kvMultiConnector, Role: role, Protocol: group.PDRuntime.Protocol, DeviceName: group.PDRuntime.RDMADeviceName, Events: plan.Events}, nil
		}
		storeRole := "kv_both"
		if group.Role == inferencev1alpha1.ModelRoleDecode {
			storeRole = "kv_consumer"
		}
		return LaunchKVPlan{Kind: kvMooncakeStore, Role: storeRole, Events: plan.Events}, nil
	}
	return LaunchKVPlan{}, fmt.Errorf("vLLM KV runtime must select a backend")
}

// buildECPlan validates and projects EC runtime settings into the vLLM launch plan.
func buildECPlan(group inferencev1alpha1.ModelGroupSpec) (*LaunchECPlan, error) {
	if group.ECRuntime == nil {
		return nil, nil
	}
	ec := group.ECRuntime
	if ec.ProfileName == "" || ec.ProfileRevision == "" || ec.Connector != "ECExampleConnector" || ec.SharedStorageClaim == "" || ec.SharedStoragePath == "" {
		return nil, fmt.Errorf("vLLM EC runtime config is incomplete")
	}
	if group.Role == inferencev1alpha1.ModelRoleEncoder && ec.Role != inferencev1alpha1.ECTransferRoleProducer {
		return nil, fmt.Errorf("vLLM encoder requires EC producer runtime")
	}
	if group.Role == inferencev1alpha1.ModelRolePrefill && ec.Role != inferencev1alpha1.ECTransferRoleConsumer {
		return nil, fmt.Errorf("vLLM prefill requires EC consumer runtime")
	}
	if group.Role != inferencev1alpha1.ModelRoleEncoder && group.Role != inferencev1alpha1.ModelRolePrefill {
		return nil, fmt.Errorf("vLLM EC runtime is only valid for encoder and prefill")
	}
	return &LaunchECPlan{
		ProfileName: ec.ProfileName, ProfileRevision: ec.ProfileRevision,
		Connector: ec.Connector, Role: string(ec.Role),
		SharedStoragePath: ec.SharedStoragePath,
	}, nil
}

func parsePositiveDuration(value inferencev1alpha1.Duration, name string) (int64, error) {
	duration, err := time.ParseDuration(string(value))
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("vLLM %s timeout must be a positive duration", name)
	}
	seconds := int64(math.Ceil(duration.Seconds()))
	if seconds > maxKubernetesInt32Seconds {
		return 0, fmt.Errorf("vLLM %s timeout must not exceed %d seconds", name, maxKubernetesInt32Seconds)
	}
	return seconds, nil
}

func copyParallelism(input inferencev1alpha1.CompiledParallelism) inferencev1alpha1.CompiledParallelism {
	output := input
	if input.EP != nil {
		copied := *input.EP
		output.EP = &copied
	}
	return output
}

func validateParallelism(parallelism inferencev1alpha1.CompiledParallelism) error {
	if parallelism.TP < 1 || parallelism.PP < 1 || parallelism.DP < 1 || parallelism.PCP < 1 || parallelism.DCP < 1 {
		return fmt.Errorf("vLLM topology values must be positive")
	}
	if parallelism.PCP > 1 && parallelism.DP > 1 {
		return fmt.Errorf("vLLM prefill context parallelism greater than 1 requires data parallelism 1")
	}
	if parallelism.PCP == 1 {
		if parallelism.TP%parallelism.DCP != 0 {
			return fmt.Errorf("vLLM decode context parallelism must divide tensor parallelism")
		}
	} else if parallelism.DCP != 1 && parallelism.DCP != parallelism.PCP && parallelism.DCP != parallelism.TP*parallelism.PCP {
		return fmt.Errorf("vLLM decode context parallelism is incompatible with tensor and prefill context parallelism")
	}
	if parallelism.EP != nil && parallelism.EP.EPLB && parallelism.EP.Size == 1 {
		return fmt.Errorf("vLLM EPLB requires more than one expert-parallel rank")
	}
	return nil
}

var controllerOwnedArgs = []string{
	"--all2all-backend", "--api-server-count", "--code-revision", "--config", "--convert",
	"--data-parallel-address", "--data-parallel-backend", "--data-parallel-external-lb",
	"--data-parallel-hybrid-lb", "--data-parallel-multi-port-external-lb",
	"--data-parallel-rank", "--data-parallel-rpc-port", "--data-parallel-size",
	"--data-parallel-size-local", "--data-parallel-start-rank", "--decode-context-parallel-size",
	"--distributed-executor-backend", "--download-dir", "--ec-manager-config", "--ec-transfer-config",
	"--enable-elastic-ep", "--enable-eplb", "--enable-expert-parallel", "--enable-prefix-caching",
	"--grpc", "--headless", "--hf-token", "--host", "--kv-events-config", "--kv-transfer-config",
	"--master-addr", "--master-port", "--model", "--nnodes", "--node-rank",
	"--pipeline-parallel-size", "--port", "--prefill-context-parallel-size", "--profiler-config", "--revision",
	"--runner", "--served-model-name", "--tensor-parallel-size", "--tokenizer", "--tokenizer-revision",
}

var engineArgName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// compileEngineArgs applies explicit service choices once, before runtime publication.
// Backend values stay native; the Rust adapter renders the resolved map into argv.
func compileEngineArgs(input inferencev1alpha1.EngineArguments, inference inferencev1alpha1.InferenceParameters) (inferencev1alpha1.EngineArguments, error) {
	args := make(inferencev1alpha1.EngineArguments, len(input))
	names := make([]string, 0, len(input))
	for name := range input {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		key := strings.ReplaceAll(name, "_", "-")
		if !engineArgName.MatchString(name) || strings.HasPrefix(key, "no-") {
			return nil, fmt.Errorf("engineArgs key %q must be a full option name without --; use YAML booleans for switches", name)
		}
		for _, owned := range controllerOwnedArgs {
			if strings.HasPrefix(owned, "--"+key) {
				return nil, fmt.Errorf("engineArgs option %q is owned by Foretoken", name)
			}
		}
		if _, exists := args[key]; exists {
			return nil, fmt.Errorf("engineArgs repeats option %q with different spellings", key)
		}
		value := input[name]
		args[key] = *value.DeepCopy()
	}

	common := map[string]any{
		"max-model-len":          inference.MaxModelLen,
		"dtype":                  inference.DType,
		"quantization":           inference.Quantization,
		"kv-cache-dtype":         inference.KVCacheDType,
		"gpu-memory-utilization": inference.GPUMemoryUtilization,
		"max-num-seqs":           inference.MaxNumSeqs,
		"max-num-batched-tokens": inference.MaxNumBatchedTokens,
		"enforce-eager":          inference.EnforceEager,
	}
	for name, value := range common {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", name, err)
		}
		if string(encoded) == "null" || string(encoded) == `""` {
			continue
		}
		// An abbreviated native spelling must not override the explicit field later in argparse.
		for key := range args {
			if strings.HasPrefix(name, key) {
				delete(args, key)
			}
		}
		args[name] = apiextensionsv1.JSON{Raw: encoded}
	}
	if speculative := inference.SpeculativeDecoding; speculative != nil {
		// Remove equivalent CLI spellings so the merged configuration has one owner.
		for key := range args {
			if strings.HasPrefix("speculative-config", key) || strings.HasPrefix("spec-method", key) || strings.HasPrefix("spec-model", key) || strings.HasPrefix("spec-tokens", key) {
				delete(args, key)
			}
		}
		encoded, err := json.Marshal(speculative)
		if err != nil {
			return nil, err
		}
		args["speculative-config"] = apiextensionsv1.JSON{Raw: encoded}
	}
	if len(args) == 0 {
		return nil, nil
	}
	return args, nil
}
