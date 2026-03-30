# abot-agent

交互式 CLI。无需 MySQL 即可使用（内存或 JSONL session）。

## 快速开始

```bash
cp config.agent.example.yaml config.yaml   # 填入 API Key
go run ./cmd/abot-agent -config config.yaml
# 或: make agent && ./bin/abot-agent -config config.yaml
```

便利：`abot-agent init`（向导生成配置）、`abot-agent --quick`（无配置文件试跑）、`abot-agent --debug`。

## REPL 命令（节选）

`/help`、`/agents`、`/switch <id>`、`/session new`、`/exit` 等；多行可用 `"""` 或反斜杠续行。

## 配置

见仓库根目录 `config.agent.example.yaml`：至少一个 `providers` 条目。

## 选项

`-config`、`--quick`、`--debug`、`--api-key`（quick 模式）、`-tenant`、`-user`。
