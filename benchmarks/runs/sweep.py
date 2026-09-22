# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright contributors to the Foretoken project

"""Read JSONL sweep points, expand them, and run each one against a single model service."""

from __future__ import annotations

import itertools
import logging
import os
from collections import Counter
from dataclasses import replace
from pathlib import Path
from typing import Any, Callable

from benchmarks.config.benchmark import (
    BenchmarkConfig,
    ParameterSweepConfig,
    normalize_output_token_limit,
)
from benchmarks.datasets.conversations import iter_jsonl_rows
from benchmarks.model_service import ModelService
from benchmarks.results.console import log_sweep_results
from benchmarks.results.output import (
    BenchmarkRun,
    result_directory_path,
    wandb_group_name,
    write_json,
)
from benchmarks.results.pareto import plot_sweep_pareto
from benchmarks.results.sweep import summarize_sweep, write_sweep_csv
from benchmarks.runs.dispatch import run_benchmark_point
from benchmarks.runs.slo import SloAutoTuneBenchmark

logger = logging.getLogger(__name__)

SweepPoint = dict[str, object]
_BENCHMARK_NAME = "_benchmark_name"
_PARAMETER_GROUP = "_parameter_group"


def sweep_point_name(point: SweepPoint) -> str:
    """Return the explicit name or build a workload point name in parameter order."""
    if _BENCHMARK_NAME in point:
        return str(point[_BENCHMARK_NAME])
    return "-".join(
        f"{key}={value}"
        for key, value in point.items()
        if key not in {_BENCHMARK_NAME, _PARAMETER_GROUP}
    )


def sweep_directory_name(name: str) -> str:
    """Convert a parameter point name into an existing result directory name."""
    return name.replace("/", "_").replace("..", "__").strip("'\"")


def _dataset_selectors(value: Any) -> list[str]:
    if isinstance(value, list):
        return [str(item) for item in value]
    return [str(value)]


def _preserve_value(value: Any) -> Any:
    return value


# One deployment experiment may change only request and workload choices; the service, credentials, traces, and output ownership remain fixed.
_SWEEP_FIELDS: dict[str, tuple[str, str, Callable[[Any], Any]]] = {
    "max_concurrency": ("load", "max_concurrency", int),
    "num_prompts": ("load", "request_count", int),
    "warmup_requests": ("load", "warmup_requests", int),
    "request_rate": ("load", "arrival_rate", float),
    "arrival_pattern": ("load", "arrival_pattern", str),
    "burstiness": ("load", "burstiness", float),
    "duration": ("load", "duration_seconds", float),
    "max_tokens": ("generation", "max_tokens", normalize_output_token_limit),
    "min_output_length": ("generation", "min_output_length", int),
    "max_output_length": ("generation", "max_output_length", int),
    "stream": ("generation", "stream", _preserve_value),
    "top_p": ("generation", "top_p", _preserve_value),
    "top_k": ("generation", "top_k", _preserve_value),
    "min_p": ("generation", "min_p", _preserve_value),
    "temperature": ("generation", "temperature", _preserve_value),
    "frequency_penalty": ("generation", "frequency_penalty", _preserve_value),
    "presence_penalty": ("generation", "presence_penalty", _preserve_value),
    "repetition_penalty": ("generation", "repetition_penalty", _preserve_value),
    "extra_body": ("generation", "extra_body", dict),
    "dataset": ("workload", "dataset_selectors", _dataset_selectors),
    "dataset_offset": ("workload", "row_offset", int),
    "tokenizer_path": ("workload", "tokenizer", str),
    "random_seed": ("workload", "random_seed", int),
    "min_prompt_length": ("workload", "minimum_prompt_tokens", int),
    "max_prompt_length": ("workload", "maximum_prompt_tokens", int),
    "prefix_length": ("workload", "shared_prefix_tokens", int),
    "apply_chat_template": ("workload", "apply_chat_template", _preserve_value),
    "prompt": ("workload", "fixed_prompt", str),
    "max_turns": ("workload", "max_turns", int),
}


def _load_axis_values(record: dict[str, object], key: str, caster: Callable[[Any], Any]) -> list[Any] | None:
    if key not in record:
        return None
    value = record[key]
    values = value if isinstance(value, list) else [value]
    if not values:
        raise ValueError(f"Sweep axis {key!r} cannot be empty")
    return [caster(item) for item in values]


def expand_load_points(item: SweepPoint) -> list[SweepPoint]:
    """Expand every list-valued execution field as a Cartesian sweep axis."""
    record = dict(item)
    axes = {
        key: _load_axis_values(record, key, caster)
        for key, (_, _, caster) in _SWEEP_FIELDS.items()
    }
    active_axes = {key: values for key, values in axes.items() if values is not None}
    base_name = record.get(_BENCHMARK_NAME)
    rest = {
        key: value
        for key, value in record.items()
        if key not in _SWEEP_FIELDS and key not in {_BENCHMARK_NAME, _PARAMETER_GROUP}
    }
    parameter_group = str(base_name) if base_name is not None else sweep_point_name(rest) or "default"
    results: list[SweepPoint] = []
    keys = list(active_axes)
    value_sets = [active_axes[key] for key in keys]
    for values in itertools.product(*value_sets) if value_sets else [()]:
        point = {**rest, **dict(zip(keys, values))}
        point[_PARAMETER_GROUP] = parameter_group
        if base_name is not None:
            suffix = "-".join(f"{key}={point[key]}" for key in keys)
            point[_BENCHMARK_NAME] = f"{base_name}-{suffix}" if suffix else str(base_name)
        results.append(point)
    return results


def load_sweep_points(path: str) -> list[SweepPoint]:
    """Read JSONL and expand it into executable HTTP benchmark points."""
    if not path:
        raise ValueError("Parameter sweep requires --sweep PATH")

    points: list[SweepPoint] = []
    directory_names: list[str] = []
    for _, line_no, _, record in iter_jsonl_rows(path, allow_comments=True):
        if not isinstance(record, dict):
            raise TypeError(
                "Each sweep JSONL line must be an object, "
                f"got {type(record)} on line {line_no}"
            )
        expanded = expand_load_points(record)
        points.extend(expanded)
        directory_names.extend(
            sweep_directory_name(sweep_point_name(point)) for point in expanded
        )

    duplicates = {
        name for name, count in Counter(directory_names).items() if count > 1
    }
    if duplicates:
        names = ", ".join(sorted(duplicates))
        raise ValueError(f"Duplicate sweep output directories: {names}")
    return points


def apply_sweep_point(
    benchmark: BenchmarkConfig,
    sweep_point: SweepPoint,
) -> BenchmarkConfig:
    """Copy the benchmark configuration and apply an allowlisted parameter point."""
    section_updates: dict[str, dict[str, Any]] = {}
    for raw_key, raw_value in sweep_point.items():
        if raw_key in {_BENCHMARK_NAME, _PARAMETER_GROUP}:
            continue
        field = _SWEEP_FIELDS.get(str(raw_key))
        if field is None:
            allowed = ", ".join(sorted(_SWEEP_FIELDS))
            raise ValueError(
                f"Unsupported sweep key {raw_key!r}. "
                "Only fields that change request execution may be swept; "
                f"allowed keys: {allowed}"
            )
        section, attribute, coerce = field
        section_updates.setdefault(section, {})[attribute] = coerce(raw_value)

    updated_benchmark = benchmark
    for section, updates in section_updates.items():
        section_value = getattr(updated_benchmark, section)
        updated_benchmark = replace(
            updated_benchmark,
            **{section: replace(section_value, **updates)},
        )
    return updated_benchmark


class ParameterSweepBenchmark:
    """Own parameter expansion, repeated runs, W&B grouping, and Pareto artifacts."""

    def __init__(
        self,
        benchmark: BenchmarkConfig,
        service: ModelService,
    ) -> None:
        self.benchmark = benchmark
        self.service = service

    def run(self) -> BenchmarkRun:
        """Run every repetition, preserve individual results, and publish per-point summaries."""
        sweep = self.benchmark.sweep
        if sweep.num_runs < 1:
            raise ValueError(f"--num-runs must be >= 1, got {sweep.num_runs}")

        combinations = load_sweep_points(sweep.path)
        if not combinations:
            raise ValueError("Parameter sweep contains no combinations")

        experiment_name = sweep.experiment_name.strip().replace("/", "-")
        experiment_dir = result_directory_path(
            self.benchmark,
            os.path.join(self.benchmark.outputs.output_dir, experiment_name)
            if experiment_name
            else None,
        )
        local_enabled = self.benchmark.outputs.includes("local")
        wandb_group = (
            wandb_group_name(self.benchmark, self.service)
            if self.benchmark.outputs.includes("wandb")
            else None
        )

        plan = {
            "mode": "parameter_sweep",
            "sweep": sweep.path,
            "num_runs": sweep.num_runs,
            "wandb_group": wandb_group,
            "combinations": [
                {
                    "dir": sweep_directory_name(sweep_point_name(point)),
                    "bench": dict(point),
                }
                for point in combinations
            ],
            "base": self.benchmark.to_dict(),
        }
        if local_enabled:
            # Unnamed directories are already reserved. Named experiments cannot
            # replace a previous plan before the engine checks its database.
            if experiment_name:
                try:
                    os.makedirs(experiment_dir)
                except FileExistsError as error:
                    raise ValueError(
                        f"Experiment directory already exists: {experiment_dir}; "
                        "choose a new --experiment-name"
                    ) from error
            write_json(experiment_dir, "config.json", plan)

        all_points: list[dict[str, Any]] = []
        for combination in combinations:
            combination_name = sweep_directory_name(sweep_point_name(combination))
            combination_root = os.path.join(experiment_dir, combination_name)
            point_benchmark = apply_sweep_point(self.benchmark, combination)
            point_benchmark.validate()
            point_benchmark = replace(
                point_benchmark,
                sweep=ParameterSweepConfig(),
            )

            for run_number in range(sweep.num_runs):
                logger.info(
                    "Sweep %s run=%s/%s bench=%s",
                    combination_name,
                    run_number + 1,
                    sweep.num_runs,
                    dict(combination),
                )
                run_dir = os.path.join(combination_root, f"run={run_number}")
                label = (
                    f"{combination_name}-run{run_number}"
                    if sweep.num_runs > 1
                    else combination_name
                )
                if point_benchmark.slo.params:
                    result = SloAutoTuneBenchmark(
                        point_benchmark,
                        self.service,
                        label=label,
                        output_dir=run_dir,
                    ).run()
                else:
                    result = run_benchmark_point(
                        point_benchmark,
                        self.service,
                        label=label,
                        output_dir=run_dir,
                        wandb_group=wandb_group,
                    )
                point = dict(result.metrics)
                point["combination"] = combination_name
                point["parameter_group"] = str(combination["_parameter_group"])
                point["run_number"] = run_number
                point["gpu_count"] = self.service.gpu_count
                point["bench"] = dict(combination)
                point["label"] = (
                    f"{combination_name}|c={point_benchmark.load.max_concurrency}"
                )
                all_points.append(point)

        artifacts: dict[str, Path] = {}
        if len(all_points) > 1:
            if local_enabled:
                fig_path = plot_sweep_pareto(all_points, experiment_dir)
                if fig_path is not None:
                    artifacts["pareto"] = fig_path
                    logger.info("Pareto plot: %s", fig_path)
            if not self.benchmark.outputs.includes("quiet"):
                log_sweep_results(all_points)

        if local_enabled:
            artifacts["sweep_points"] = write_json(experiment_dir, "sweep_points.json", all_points)
            summary = summarize_sweep(all_points)
            artifacts["sweep_summary"] = write_json(experiment_dir, "sweep_summary.json", summary)
            artifacts["sweep_summary_csv"] = write_sweep_csv(summary, experiment_dir)
            logger.info("Repeated-run summaries: %s/sweep_summary.csv", experiment_dir)
        logger.info(
            "Sweep done: %s combinations, %s points, output_dir=%s",
            len(combinations),
            len(all_points),
            experiment_dir,
        )
        return BenchmarkRun(
            record=plan,
            # The caller needs completion counts, not a cherry-picked workload's
            # latency or throughput presented as a whole-experiment measurement.
            metrics={
                name: sum(point[name] for point in all_points)
                for name in ("request_num", "success_num", "failed_num")
            },
            measurements=None,
            artifacts=artifacts,
        )
