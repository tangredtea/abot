# ABot

基于 Google ADK-Go 构建的无状态多租户 AI Agent 框架。所有状态存数据库，Agent 进程本身无状态，重启即健康。

## 核心特性

- **无状态架构** — Session、记忆、Workspace 全部持久化到 MySQL + Qdrant + JSONL，进程崩溃重启零数据丢失
- **多租户隔离** — 以 `tenant_id` 为维度隔离 Workspace、Skills、记忆，支持 10 万+ 租户规模
- **多 LLM Provider** — Anthropic 原生 + OpenAI-compatible 兼容层，内置 Fallback Chain 自动故障切换
- **Prompt Caching** — Anthropic 端自动注入 `cache_control`，缓存 system prompt + tools 定义，降低重复调用成本
- **Reasoning Model** — OpenAI-compatible 端支持解析 `reasoning_content`（DeepSeek-R1、Kimi 等）
- **ADK-Go 原生集成** — 不改 ADK-Go 一行代码，直接使用 `llmagent`、`runner`、`functiontool`、`mcptoolset`、`plugin` 等原生能力
- **MCP 协议支持** — 内置 MCP Client（stdio + HTTP），自动发现并包装 MCP Server 工具为 ADK tool.Tool
- **A2A 协议支持** — 通过 ADK-Go `adka2a` 暴露 Agent 为 A2A 服务，支持跨进程 Agent-to-Agent 调用
- **多 Channel** — CLI 终端 + 企业微信 Webhook，Channel 适配器可扩展
- **Session 持久化** — JSONL 文件写穿 + 内存双写，重启自动恢复，超 10MB 自动 compaction
- **Plugin 体系** — 审计日志、Token 用量追踪、记忆自动整合三个内置 Plugin
- **Skill 系统** — 全局注册表 + 按租户安装，支持 builtin/global/group 三层优先级，BOS/S3 存储 + 本地 lazy pull 缓存

## 架构概览

```
                    ┌─────────────┐
                    │  Bootstrap   │  ← YAML 配置解析 + 组件组装
                    └──────┬──────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
┌────────▼──┐  ┌───────▼────────┐  ┌──▼──────────┐
│  Channels  │  │   Scheduler    │  │  A2A Server  │
│  CLI + 扩展 │  │  Cron+Heartbeat│  │ agent2agent  │
└────────┬──┘  └───────┬────────┘  └──┬──────────┘
         │             │              │
         └─────────────┼──────────────┘
                       │
                ┌──────▼──────┐
                │ Message Bus  │  ← Go channel 异步解耦
                └──────┬──────┘
                       │
                ┌──────▼──────┐
                │  Agent Core  │  ← 路由 + Loop + ADK Runner
                └──┬───┬───┬──┘
                   │   │   │
      ┌────────────┘   │   └────────────┐
      │                │                │
┌─────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐
│  Providers  │  │  ADK Tools   │  │  Workspace  │
│ Anthropic   │  │ functiontool │  │ Context +   │
│ OpenAI-compat│  │ + MCP toolset│  │ Skill (多租户)│
└─────┬──────┘  └──────┬──────┘  └──────┬──────┘
      │                │                │
      └────────────────┼────────────────┘
                       │
              ┌────────▼────────┐
              │   Cache Layer    │  ← 进程内 LRU
              └────────┬────────┘
                       │
      ┌────────────────┼────────────────┐
      │                │                │
┌─────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐
│   MySQL     │  │  BOS/S3     │  │   Qdrant    │
│  关系数据   │  │  Skill 包   │  │  向量搜索   │
└────────────┘  └─────────────┘  └─────────────┘
```

## 目录结构

```
abot/
├── cmd/
│   └── abot/main.go              # 入口：配置加载、依赖组装、启动
├── pkg/
│   ├── types/                        # 共享接口与类型（零业务逻辑）
│   ├── bus/                          # Message Bus（Go channel 实现）
│   ├── agent/                        # Agent Core：Loop、Registry、Subagent、Compression、A2A、Bootstrap
│   ├── routing/                      # 路由逻辑：Agent ID 解析、Session Key 派生、Route 匹配
│   ├── providers/                    # LLM Provider 层
│   │   ├── anthropic/               #   Anthropic Messages API（支持 Prompt Caching）
│   │   ├── openaicompat/            #   OpenAI ChatCompletion 兼容层（支持 Reasoning Model）
│   │   └── fallback/                #   Fallback Chain + Cooldown + 错误分类
│   ├── tools/                        # 内置工具（ADK functiontool）
│   ├── mcp/                          # MCP Client：stdio/HTTP 传输，JSON-RPC 2.0，tool.Tool 包装
│   ├── workspace/                    # 多层 Context Builder（动态 System Prompt）
│   ├── skills/                       # Skill 加载器 + 远程注册表
│   │   └── clawhub/                 #   ClawHub 远程 Skill 市场客户端
│   ├── session/                      # Session 持久化：JSONL 文件写穿 + 自动 compaction
│   ├── scheduler/                    # Cron 定时任务 + Heartbeat 心跳（支持 LLM 决策）
│   ├── channels/                     # Channel 适配器 + Registry
│   │   ├── cli/                     #   CLI 终端交互
│   │   └── wecom/                   #   企业微信 Webhook（AES 解密 + 消息去重）
│   ├── storage/                      # 存储层
│   │   ├── mysql/                   #   MySQL GORM 实现（租户、Workspace、Skill、Scheduler）
│   │   ├── objectstore/             #   对象存储抽象（本地文件 / S3）
│   │   ├── vectordb/                #   向量数据库抽象 + OpenAI-compatible Embedder
│   │   │   └── qdrant/              #     Qdrant gRPC 实现
│   │   └── cache/                   #   进程内 LRU 缓存
│   └── plugins/                      # ADK Plugin
│       ├── auditlog/                #   LLM/Tool 调用审计
│       ├── tokentracker/            #   Token 用量追踪
│       └── memoryconsolidation/     #   记忆自动整合（向量沉淀 + 语义去重）
└── go.mod
```

## 存储选型

| 组件 | 选型 | 职责 | 理由 |
|------|------|------|------|
| 关系数据库 | MySQL (GORM) | 租户、Workspace 文档、Skill 注册表、Cron Job | 成熟稳定，GORM 降低开发成本 |
| 向量数据库 | Qdrant | 记忆语义搜索，per-tenant collection 隔离 | 支持 filtered vector search，Go gRPC SDK 完善 |
| 对象存储 | S3/BOS | Skill 内容包存储 | 大文件存储标准方案，本地开发可用文件系统替代 |
| 缓存 | 进程内 LRU | 租户配置、Skill 内容本地路径缓存 | 单进程架构无需 Redis，重启自动重建 |
| Session | JSONL 文件 + 内存 | Agent 会话持久化（write-through 双写） | 重启后从磁盘恢复，自动 compaction 控制文件大小 |
| Embedding | OpenAI-compatible | 文本向量化（支持 Ollama 本地模型） | 兼容 OpenAI / Ollama / 任意 `/v1/embeddings` 端点 |

**为什么不用 Redis？** ABot 是单进程架构，不存在跨进程缓存共享需求。进程内 LRU 延迟更低、运维更简单，重启后缓存自然重建，首次访问从 MySQL 加载。

## LLM Provider 层

支持两类 Provider，通过统一的 `model.LLM` 接口对接 ADK-Go：

- **Anthropic 原生** — 直接调用 Messages API，支持流式响应、Tool Use、多模态、Prompt Caching
- **OpenAI-compatible** — 兼容 OpenAI ChatCompletion 协议，覆盖 OpenAI / DeepSeek / MiniMax / Gemini / OpenRouter 等，支持 Reasoning Model（`reasoning_content` 字段解析）

两个 Provider 各自实现 `genai.Content` 双向转换（请求转换 + 响应转换），覆盖 system instructions、tool declarations、function calls/responses、流式 SSE。

### Prompt Caching（Anthropic）

开启 `prompt_caching: true` 后，自动给 system 消息最后一个 block 和 tools 列表最后一个 tool 注入 `cache_control: {type: "ephemeral"}`。Anthropic API 会缓存这些不变的内容，后续请求命中缓存可节省约 90% 的 input token 费用。

### Reasoning Model（OpenAI-compatible）

支持解析 DeepSeek-R1、Kimi K2.5 等模型返回的 `reasoning_content` 字段，作为 metadata 附加到 LLMResponse，不直接暴露给用户。

### Fallback Chain

多 Provider 自动故障切换机制：

```
Primary Provider → (失败) → Candidate 1 → (失败) → Candidate 2 → ...
```

- **错误分类器** — 区分 auth / rate_limit / timeout / format / overloaded 五类错误，决定是否切换
- **Cooldown 退避** — 失败的 Provider 进入指数退避冷却期，避免反复冲击
- **Provider Registry** — 按 model name 关键词自动路由到对应 Provider（如 `claude-*` → Anthropic）

## 内置工具

所有工具通过 ADK-Go `functiontool.New[TArgs, TResult]()` 创建，返回原生 `tool.Tool`，由 Bootstrap 注入 `llmagent.Config.Tools`。

| 工具 | 说明 |
|------|------|
| `read_file` / `write_file` / `edit_file` / `append_file` / `list_dir` | 文件操作，Workspace 沙箱限制 |
| `exec` | Shell 命令执行，带超时 + 命令黑名单安全过滤 |
| `web_search` / `web_fetch` | Web 搜索 + 网页抓取 |
| `message` | 跨 Channel 消息发送 |
| `spawn` / `subagent` / `list_tasks` | 子任务派生 + 后台 Agent 管理 |
| `cron` | 定时任务管理（add / list / remove / enable / disable） |
| `find_skills` / `install_skill` / `create_skill` / `promote_skill` | Skill 搜索、安装、创建、审核提升 |

MCP 工具通过 `pkg/mcp` 包实现，支持两种传输模式：

- **stdio** — 通过 `exec.Command` 启动 MCP Server 进程，stdin/stdout 通信
- **HTTP** — 连接远程 MCP Server（Streamable HTTP）

启动时自动执行 `initialize` → `tools/list` → 逐个包装为 ADK `tool.Tool`，名称格式 `mcp_{server}_{tool}`。每个工具调用有独立超时控制（默认 30s）。

## Channel 适配器

Channel 是消息入口，所有 Channel 实现 `types.Channel` 接口，通过 `channels.Registry` 统一管理生命周期。

| Channel | 说明 |
|---------|------|
| **CLI** | 终端交互，stdin/stdout，开发调试用 |
| **WeCom** | 企业微信 Webhook，HTTP Server 监听回调，AES-CBC 解密 + 签名验证 + 5 分钟 TTL 消息去重 |

WeCom Channel 支持 `allow_from` 白名单过滤发送者，`reply_timeout` 控制回复超时。消息解密使用标准企业微信 AES-CBC + PKCS7 方案。

## Agent Core

### 消息处理流程

```
Channel → Bus.PublishInbound
    → AgentLoop.ConsumeInbound
    → Registry.ResolveRoute (channel + chatID 匹配)
    → EnsureSession (注入 tenant_id / user_id / channel / chat_id)
    → Runner.Run (ADK-Go)
    → CollectResponse
    → Bus.PublishOutbound
    → Channel.Send
```

### 多 Agent 路由

`AgentRegistry` 支持按 channel + chatID 的优先级匹配：

- Score 2：channel + chatID 精确匹配
- Score 1：仅 channel 匹配
- Score 0：通配（无条件匹配）
- 无匹配时 fallback 到第一个注册的 Agent

### Session 压缩

ADK-Go 使用 event-based session（append-only）。当 session events 超过 context window 的 75% 时自动触发压缩：

1. **正常压缩** — 提取旧 events 文本，调用 SummaryLLM 生成摘要，创建新 session 替换
2. **强制压缩** — 正常压缩失败时，直接丢弃最旧 50% events
3. **Context Overflow 重试** — Agent 调用遇到 context overflow 错误时，自动压缩后重试（最多 2 次）

## Workspace & Context Builder

每次 Agent 调用时，`ContextBuilder` 动态组装 6 层 System Prompt：

| 层级 | 来源 | 说明 |
|------|------|------|
| Layer 1 | 硬编码 | 系统基础规则（工具使用、记忆保存等） |
| Layer 2 | MySQL WorkspaceStore | 租户人格：IDENTITY + SOUL + RULES 文档 |
| Layer 3 | Qdrant VectorStore | 租户级向量记忆（按分类沉淀，语义去重） |
| Layer 4 | MySQL UserWorkspaceStore + Qdrant | 用户偏好文档 + 用户级向量记忆 |
| Layer 5 | SkillsLoader | Skills：always_load 全文注入 + 其余 XML 摘要 |
| Layer 6 | 运行时 | 时间、平台、Channel、Chat ID |

通过 `llmagent.Config.InstructionProvider` 注入，每次调用动态生成，从 session state 读取 `tenant_id` 和 `user_id` 实现多租户隔离。

## Skill 系统

Skill 是可复用的知识包，按 4 级优先级加载（高优先级覆盖同名低优先级）：

1. **tenant-installed** — 租户主动安装的 Skill（最高优先级）
2. **group** — 租户所属分组的默认 Skill
3. **global** — 全局可用 Skill
4. **builtin** — 随代码发布的内置 Skill（最低优先级）

存储分离设计：
- **MySQL** — 全局注册表（`SkillRegistryStore`）+ 租户安装关系（`TenantSkillStore`）
- **S3/BOS** — Skill 内容包（tar.gz）
- **本地磁盘** — lazy pull 缓存，三级查找：进程内 LRU → 本地文件 → S3 拉取

## Plugin 系统

基于 ADK-Go 六层 Hook 体系（UserMessage → Run → Agent → Model → Tool → Event）实现横切关注点：

| Plugin | Hook 点 | 功能 |
|--------|---------|------|
| **auditlog** | BeforeModel / AfterModel / BeforeTool / AfterTool | 记录每次 LLM 和工具调用的输入、输出、耗时、错误 |
| **tokentracker** | AfterModel | 从 `LLMResponse.UsageMetadata` 提取 token 用量，支持全局 + per-agent 统计 |
| **memoryconsolidation** | AfterRun | session events 超阈值时自动整合记忆，按自由分类沉淀到向量库，search-before-write 语义去重 |

Plugin 通过 `runner.Config.PluginConfig` 注入 ADK-Go Runner，在 Bootstrap 阶段装配。

## 定时调度

### Cron Service

支持三种调度模式：

- **at** — 一次性定时，到时间后投递消息，可选执行后自动删除
- **every** — 固定间隔周期执行
- **cron** — 标准 cron 表达式，支持 IANA 时区

触发时构造 `InboundMessage` 通过 MessageBus 投递，Agent 像处理普通消息一样处理定时任务。Job 持久化到 MySQL，重启后自动恢复。

### Heartbeat Service

周期性（默认 30 分钟）遍历所有配置了 `HEARTBEAT.md` 的租户，支持两种决策模式：

- **passive**（默认）— 直接将心跳内容投递给 Agent，Agent 自行决定 skip 或执行
- **llm** — 先调用 LLM（优先用 SummaryLLM 降低成本）判断是否需要执行，返回 `{"action":"run"}` 时才投递

## 配置示例

```yaml
app_name: abot

mysql_dsn: "root:password@tcp(127.0.0.1:3306)/abot?parseTime=true"

object_store:
  type: local        # "local" 或 "s3"
  dir: /tmp/abot/objects

vector_db:
  addr: "localhost:6334"

embedding:
  api_base: "http://localhost:11434/v1"  # Ollama 本地模型
  api_key: "ollama"
  model: "nomic-embed-text"
  dimension: 768

session:
  type: "jsonl"      # "memory"（默认）或 "jsonl"
  dir: "data/sessions"

cache:
  tenant_size: 1000
  skill_size: 500

providers:
  - name: anthropic
    api_base: "https://api.anthropic.com"
    api_key: "${ANTHROPIC_API_KEY}"
    model: "claude-sonnet-4-20250514"
    prompt_caching: true
  - name: deepseek
    api_base: "https://api.deepseek.com/v1"
    api_key: "${DEEPSEEK_API_KEY}"
    model: "deepseek-chat"

agents:
  - id: default
    name: "ABot Assistant"
    description: "General-purpose AI assistant"
    model: "claude-sonnet-4-20250514"
    routes:
      - channel: "*"

mcp_servers:
  filesystem:
    command: "npx"
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    tool_timeout: 30

wecom:                          # 企业微信（可选）
  token: "${WECOM_TOKEN}"
  encoding_aes_key: "${WECOM_AES_KEY}"
  webhook_url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
  webhook_host: "0.0.0.0"
  webhook_port: 8080
  webhook_path: "/webhook/wecom"

plugins:
  audit_log: true
  token_tracker: true
  memory_consolidation: true

scheduler:
  heartbeat_interval: "30m"
  heartbeat_channel: "cli"
  decision_mode: "passive"      # "passive" 或 "llm"

a2a:
  enabled: false
  addr: ":8080"

skill_cache_dir: /tmp/abot/skills
context_window: 128000
bus_buffer_size: 100
```

## 快速开始

```bash
# 依赖
# - Go 1.24+
# - MySQL 8.0+
# - Qdrant (可选，记忆搜索，推荐本地二进制安装)
# - Ollama (可选，本地 embedding，推荐 nomic-embed-text 模型)

# 构建
cd abot
go build -o abot ./cmd/abot

# 运行
./abot --config config.yaml
```

## 设计决策

### 为什么选 ADK-Go？

ADK-Go 提供了完整的 Agent 运行时：`llmagent`（Agent 定义）、`runner`（执行引擎）、`session`（会话管理）、`functiontool`（工具创建）、`mcptoolset`（MCP 桥接）、`plugin`（六层 Hook）。ABot 不改 ADK-Go 一行代码，只实现它的接口 + 在外围构建应用层。

### 为什么无状态？

传统 Agent 框架将 session、记忆存在进程内存中，进程崩溃即数据丢失。ABot 将所有状态外置到 MySQL + Qdrant + S3，进程本身是纯计算节点。好处：

- `kill -9` 后重启，session 恢复、记忆可搜索、cron job 继续执行
- 理论上可水平扩展（当前单进程，未来可拆分）
- 运维简单：只需关心数据库备份

### 为什么多租户？

单实例服务多个独立"租户"（可以是论坛、群组、组织等任意概念），每个租户有独立的：

- Workspace 文档（IDENTITY / SOUL / RULES）
- 已安装 Skills
- 向量记忆（per-tenant Qdrant collection，按分类沉淀 + 语义去重）
- 用户级偏好和向量记忆

共享的是：全局 Skill 注册表、LLM Provider、Agent 定义。

## 扩展指南

| 扩展点 | 做法 |
|--------|------|
| 新增 LLM Provider | 实现 `model.LLM` 接口，注册到 Provider Registry |
| 新增 Channel | 实现 `types.Channel` 接口，注册到 Channel Registry |
| 新增工具 | `functiontool.New[TArgs, TResult]()`，追加到 `BuildAllTools()` |
| 新增 MCP Server | 在 `config.yaml` 的 `mcp_servers` 中添加配置（stdio 或 HTTP） |
| 新增 Plugin | 实现 ADK-Go `plugin.Config` 回调，注入 `BootstrapDeps.Plugins` |
| 新增 Skill | 上传到 S3，注册到全局 `SkillRegistryStore`，租户通过 `install_skill` 安装 |
| 新增租户 | INSERT `tenants` 表，配置 Workspace 文档，首次对话时 Qdrant collection 自动创建 |
| 新增存储后端 | 实现对应 Store 接口（`VectorStore` / `ObjectStore` / `Embedder` 等） |
| 切换 Session 存储 | 实现 `session.Service` 接口，在 `newSessionService()` 中注册（当前支持 memory / jsonl） |

## 项目规模

~21,000 行 Go 代码，覆盖 15 个核心模块，Go 1.24 + ADK-Go。
