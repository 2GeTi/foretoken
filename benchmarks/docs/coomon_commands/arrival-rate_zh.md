# 到达模式与并发

[English](arrival-rate.md) | 简体中文 · [常用命令](../examples_zh.md)

完成[准备步骤](../examples_zh.md#准备)后，按平均每秒 5 个请求发送，同时最多允许 16 个请求在途：

```bash
foretoken bench examples/quickstart \
  --prompt "你好" --request-rate 5 --max-concurrency 16 --num-prompts 100 \
  --output local,wandb
```

`--request-rate` 控制目标请求速率，`--max-concurrency` 限制在途请求数。默认 `--arrival-pattern poisson` 使用泊松到达；`constant` 使用固定间隔，`gamma` 配合 `--burstiness` 表达突发程度；需要按时间戳回放时单独使用 `--trace`。`--request-rate -1` 表示尽快发送，`--max-concurrency -1` 表示取消并发上限。默认不限速、并发为 1。生成式 constant 和 Gamma 到达目前要求单一数据集和单轮请求。

去掉并发上限：

```bash
foretoken bench examples/quickstart \
  --prompt "你好" --request-rate 5 --max-concurrency -1 --num-prompts 100 \
  --output local,wandb
```

`--request-rate -1 --max-concurrency -1` 会尽快启动请求预算内的请求。多轮数据要求 `--request-rate -1`；`--num-prompts` 仍表示 HTTP 请求预算，`--max-concurrency` 限制同时执行的对话数。

使用固定间隔或 Gamma 到达：

```bash
foretoken bench examples/quickstart \
  --prompt "你好" --request-rate 5 --arrival-pattern constant \
  --max-concurrency 16 --num-prompts 100 --output local

foretoken bench examples/quickstart \
  --prompt "你好" --request-rate 5 --arrival-pattern gamma \
  --burstiness 0.5 --max-concurrency 16 --num-prompts 100 --output local
```

## 输出示例

以下为较低到达率的小规模运行：

![命令行输出](../imgs/arrival-rate-cli.png)

![W&B 运行页面](../imgs/arrival-rate-wandb.png)
