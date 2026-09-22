# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright contributors to the Foretoken project

"""Generate deterministic constant or Gamma arrivals and reuse trace replay execution."""

from __future__ import annotations

import json
import random
import shutil
import tempfile
from pathlib import Path

from benchmarks.config.benchmark import ArrivalTraceSchedule, BenchmarkConfig
from benchmarks.datasets.conversations import load_request_tasks, split_chat_conversation
from benchmarks.datasets.synthetic import generate_trace_random_requests
from benchmarks.model_service import ModelService
from benchmarks.results.output import BenchmarkRun, result_directory_path
from benchmarks.runs.trace import TraceReplayBenchmark


class GeneratedArrivalBenchmark:
    """Translate generated arrival schedules into the existing trace runner."""

    def __init__(
        self,
        benchmark: BenchmarkConfig,
        service: ModelService,
        *,
        label: str = "",
        output_dir: str | None = None,
        wandb_group: str | None = None,
    ) -> None:
        self.benchmark = benchmark
        self.service = service
        self.label = label
        self.output_dir = output_dir
        self.wandb_group = wandb_group

    def _tasks(self):
        workload = self.benchmark.resolved_workload
        if workload.dataset_selectors == ["random"]:
            tasks = generate_trace_random_requests(
                self.benchmark,
                self.service,
                request_count=self.benchmark.load.request_count,
            )
        else:
            tasks = load_request_tasks(self.benchmark)
        if any(len(split_chat_conversation(task.messages())) != 1 for task in tasks):
            raise ValueError(
                "generated constant and gamma arrivals require single-turn workloads"
            )
        return tasks

    def _write_trace(self, path: Path) -> None:
        rate = self.benchmark.load.arrival_rate
        pattern = self.benchmark.load.arrival_pattern
        generator = random.Random(self.benchmark.resolved_workload.random_seed)
        offset = 0.0
        with path.open("w", encoding="utf-8") as file:
            for index, task in enumerate(self._tasks()):
                if index:
                    if pattern == "constant":
                        interval = 1.0 / rate
                    else:
                        shape = self.benchmark.load.burstiness
                        interval = generator.gammavariate(
                            shape, 1.0 / (rate * shape)
                        )
                    offset += interval
                file.write(
                    json.dumps(
                        {
                            "timestamp": round(offset * 1000.0, 6),
                            "chatId": task.id,
                            "messages": task.messages(),
                            **task.metadata,
                        },
                        ensure_ascii=False,
                    )
                    + "\n"
                )

    def run(self) -> BenchmarkRun:
        """Run generated arrivals through the existing timestamp replay lifecycle."""
        local_output = self.benchmark.outputs.includes("local")
        temporary_dir: str | None = None
        if local_output:
            output_dir = self.output_dir or result_directory_path(self.benchmark)
        else:
            temporary_dir = tempfile.mkdtemp(prefix="foretoken-arrival-")
            output_dir = temporary_dir
        Path(output_dir).mkdir(parents=True, exist_ok=True)
        trace_path = Path(output_dir) / "arrival_trace.jsonl"
        self._write_trace(trace_path)
        replay = BenchmarkConfig(
            service=self.benchmark.service,
            workload=self.benchmark.workload.__class__(dataset_selectors=[str(trace_path)]),
            load=self.benchmark.load,
            generation=self.benchmark.generation,
            trace=ArrivalTraceSchedule(
                trace_selector=str(trace_path),
                max_concurrency=(
                    None
                    if self.benchmark.load.max_concurrency == -1
                    else self.benchmark.load.max_concurrency
                ),
            ),
            outputs=self.benchmark.outputs,
            wandb=self.benchmark.wandb,
            sweep=self.benchmark.sweep,
            slo=self.benchmark.slo,
            profile=self.benchmark.profile,
        )
        try:
            return TraceReplayBenchmark(
                replay,
                self.service,
                label=self.label,
                output_dir=output_dir,
                wandb_group=self.wandb_group,
            ).run()
        finally:
            if temporary_dir is not None:
                shutil.rmtree(temporary_dir, ignore_errors=True)
