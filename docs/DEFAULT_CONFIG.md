# 默认工具与模板（摘要）

技能**名称与安装**以根目录 [SKILLS.md](../SKILLS.md) 为准；本节为**工具能力**与 **workspace / MCP** 速查。

## 内置工具（类别）

| 类别 | 工具 |
|------|------|
| 文件 | `read_file`, `write_file`, `edit_file`, `append_file`, `list_dir` |
| 系统 | `exec`, `message` |
| 网络 | `web_search`, `web_fetch` |
| 任务 | `spawn`, `cron` |
| 技能 | `find_skills`, `install_skill`, `create_skill`, `promote_skill` |
| 记忆 | `save_memory`, `search_memory`（混合检索：语义 + BM25 + 时间等） |
| 子代理（若启用） | `subagent`, `list_tasks` |

## MCP

`pkg/mcp/`：客户端、多租户管理、工具包装；通过配置接入 MCP Server，工具注入 Agent。

## Workspace 模板（与 DB / 路由相关）

- 租户级：`IDENTITY.md`, `SOUL.md`, `AGENT.md`, `TOOLS.md`, `RULES.md`, `HEARTBEAT.md` 等  
- 用户级：`USER.md`, `EXPERIMENTS.md`, `NOTES.md`  

`read_file` / `write_file` / `list_dir` 与 MySQL 中 workspace 文档联动（见实现）。

## 最小配置示例

见仓库根目录 `config.agent.example.yaml` 与 `config.example.yaml`。
