# Parameter sweeps

English | [简体中文](sweep_zh.md) · [Common commands](../examples.md)

After [setup](../examples.md#setup), deploy the service and warm it up before comparing concurrency levels:

```bash
foretoken deploy examples/quickstart --timeout 20m
for parallel in 1 2 4; do
  foretoken bench examples/quickstart \
    --dataset random --tokenizer-path Qwen/Qwen3-0.6B \
    --min-prompt-length 128 --max-prompt-length 256 --random-seed 0 \
    --min-output-length 256 --max-output-length 256 \
    --parallel "$parallel" --number 16 --output local
done
```

Run the sweep with the same inputs. The parameter file compares concurrency 1, 2, and 4, with 384 requests per point and 256 output tokens per request:

```bash
foretoken bench examples/quickstart \
  --dataset random --tokenizer-path Qwen/Qwen3-0.6B \
  --min-prompt-length 128 --max-prompt-length 256 --random-seed 0 \
  --sweep benchmarks/examples/sweep.jsonl \
  --experiment-name quickstart-sweep \
  --output local,wandb
```

The [parameter file](../../examples/sweep.jsonl) contains JSONL rows. List values for `parallel`, `number`, or `rate` expand into points. Only one of `parallel` and `rate` may be a multi-value list in a row; a multi-value `number` list must match that axis's length.

Each row may change load, generation, or dataset settings, including output-length bounds. Service identity, credentials, trace source, and output destinations stay fixed. Sweeps cannot be combined with trace replay or multiple datasets. `--num-runs` repeats each point.

Each point has a result directory. `sweep_points.json` records all results, and `pareto/PARETO.png` compares output token throughput per configured user with throughput per GPU when enough points are available. Choose a fresh `--experiment-name` for another experiment, or omit it to use an automatically created directory.

Delete the service after finishing with `foretoken delete examples/quickstart`.

## Example output

These results use Qwen3-0.6B on one A100 80GB PCIe GPU. The concurrency-1 run retained one failed request (383/384 successful); the other two completed 384/384.

![Recorded sweep output](../imgs/sweep-cli.png)

The W&B chart shows E2EL p95 for requests completed in each one-second window. Use the aggregate results and Pareto plot to compare whole-run throughput.

![E2EL p95 over elapsed time, in one-second completion windows](../imgs/sweep-wandb.png)

![Measured Pareto frontier](../imgs/sweep-pareto.png)
