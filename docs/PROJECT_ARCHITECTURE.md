# abot 项目架构

仓库内**实际代码布局**、模块职责、安全与英文摘要。

## 1. 总览

abot 基于 **Google ADK (adk-go)**：多租户、可插拔存储与渠道、Skills/Tools、可选 Web/API。

| 形态 | 说明 |
|------|------|
| **多二进制** | `abot-agent`、`abot-server`、`abot-web`，`pkg/bootstrap` 组装依赖。 |
| **`cmd/abot`** | 统一入口：gateway / `agent` / `console`，与全量 MySQL 等集成。 |

## 2. 目录（顶层）

```
cmd/abot, abot-agent, abot-server, abot-web
pkg/          # agent、bootstrap、api、channels、providers、storage、skills、tools、workspace、scheduler…
skills/       # 内置 SKILL.md
web/          # 前端
workspace/    # 默认文档种子（勿删；与 seeder 相关）
tests/
```

## 3. `pkg/` 职责（节选）

| 包 | 职责 |
|----|------|
| **agent** | 循环、注册表、与 ADK 集成 |
| **bootstrap** | 配置、校验、`BuildCoreDeps` / `BuildFullDeps` |
| **api** | 控制台 HTTP、JWT、WebSocket |
| **channels** | Telegram、Discord、飞书、企微等 |
| **providers** | LLM、fallback 链 |
| **storage** | MySQL、向量、对象存储等 |
| **skills / tools / workspace** | 技能、工具、动态 prompt |
| **scheduler** | Cron、心跳、分布式锁 |
| **mcp** | MCP 客户端 |

## 4. 入口关系

- **abot-agent**：轻量 REPL。  
- **abot-server**：API（`runAPIServer`）。  
- **abot-web**：静态资源 + 控制台。  
- **abot**：网关或子命令。  

`make agent|server|web`。

## 5. 安全与沙箱

### 5.1 应用层（Landlock 模式：standard / strict）

| 能力 | 实现 |
|------|------|
| Shell 拦截、路径校验 | `pkg/tools/security.go` |
| 子进程 ulimit 限制 | `pkg/tools/shell*.go` |
| Landlock 内核文件系统沙箱 | `pkg/tools/sandbox.go` + `abot-sandbox` 助手 |
| 租户工具权限 / QPS | `pkg/tools/guard.go` |

### 5.2 gVisor 轻量沙箱（gvisor 模式，推荐）

| 能力 | 实现 |
|------|------|
| `runsc do` 直接沙箱化命令 | `pkg/tools/sandbox_gvisor.go` |
| 用户态内核拦截系统调用 | gVisor Sentry，~50ms 启动 |
| 无 Docker 依赖 | 仅需 `runsc` 二进制 |
| 网络隔离 | 默认隔离，可选 `--network=host` |
| 资源限制 | ulimit（非 cgroup） |

配置 `sandbox.level: "gvisor"`。

### 5.3 Docker 容器隔离（container 模式，最强隔离）

| 能力 | 实现 |
|------|------|
| 每次 exec → 独立 OCI 容器 | `pkg/tools/sandbox_container.go` |
| gVisor (runsc) 系统调用拦截 | `container_runtime: "runsc"` |
| cgroup 资源限制 (内存/CPU/PID) | Docker `--memory` / `--cpus` / `--pids-limit` |
| 只读根文件系统 + 隔离 tmpfs | `--read-only` + `--tmpfs` |
| 网络隔离 | `--network=none`（或自定义 Docker 网络） |
| 沙箱镜像 | `sandbox.Dockerfile`（Node + Python + Git） |

配置 `sandbox.level: "container"`。需要 Docker daemon。

## 6. CLI 便利功能（abot-agent）

- `abot-agent init`：交互生成配置  
- `abot-agent --quick`：无配置文件快速试跑（可用 `OPENAI_API_KEY`）  
- `abot-agent --debug`：更详细日志  

示例代码见 `examples/`。

## 7. 配置与默认值

- 示例：`config.example.yaml`、`config.agent.example.yaml`。  
- 内置工具列表、MCP、workspace 模板摘要见 [DEFAULT_CONFIG.md](./DEFAULT_CONFIG.md)；技能名列表见根目录 [SKILLS.md](../SKILLS.md)。

---

## English summary

Multi-tenant agent stack on **adk-go**. Three binaries share **`pkg/bootstrap`**; optional **`cmd/abot`** for monolithic deploy. Layers: thin `cmd/*` → **`pkg/agent`** loop → **providers** / **storage** / **channels** / **skills** / **tools**. **Health**: `GET /health` → 200 `OK`. **No** built-in Prometheus scrape endpoint on these servers. **DB**: AutoMigrate on startup. Tests: `go test ./...`.

---

*文档索引：[docs/README.md](./README.md)。部署与迁移操作见 [DEPLOYMENT.md](./DEPLOYMENT.md)。*
