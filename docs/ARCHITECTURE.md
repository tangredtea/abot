# ABot 架构指南

> 面向新手的项目核心要点梳理。基于当前代码，参考同目录下的 NanoBot（Python）和 ADK-Go（SDK）。

## 一句话定位

ABot 是一个用 Go 写的多渠道 AI Agent 框架，基于 Google ADK-Go SDK 构建。
它做的事情和 NanoBot（Python 版）一样——把 LLM 接入各种聊天平台——但用 Go 重写，追求更强的类型安全和并发性能。

## 项目结构

```
abot/
├── cmd/abot/main.go        # 入口：加载配置 → 组装依赖 → 启动应用
├── pkg/
│   ├── agent/               # 核心：Agent 循环、注册表、压缩、引导
│   ├── bus/                 # 消息总线（Go channel 实现）
│   ├── channels/            # 渠道适配器（CLI、Telegram、Discord、飞书、企微）
│   ├── tools/               # 内置工具（Shell、文件系统、Web、Cron、子Agent等）
│   ├── skills/              # 技能系统（可热加载的 prompt 模板）
│   ├── providers/           # LLM 提供商 + Fallback 链
│   ├── plugins/             # ADK 插件（审计日志、Token 追踪、记忆整合）
│   ├── mcp/                 # MCP 协议客户端（外部工具服务器）
│   ├── routing/             # 消息路由规则
│   ├── session/             # 会话持久化（JSONL）
│   ├── storage/             # 存储层（MySQL、S3、Qdrant 向量库）
│   ├── scheduler/           # 定时任务 + 心跳
│   ├── types/               # 共享接口和类型定义
│   ├── workspace/           # 工作空间 + 上下文构建
│   └── api/                 # HTTP API（技能市场等）
├── skills/                  # 内置技能文件（weather、github、cron 等）
└── config.yaml              # 运行配置
```

## 核心数据流

理解 ABot 只需要记住一条主线：

```
用户消息 → Channel → Bus(入站) → AgentLoop → ADK Runner → LLM → Bus(出站) → Channel → 用户
```

展开来看：

```
┌──────────┐   ┌──────────┐   ┌──────────┐
│ Telegram │   │ Discord  │   │   CLI    │   ... 更多渠道
└────┬─────┘   └────┬─────┘   └────┬─────┘
     │              │              │
     └──────────────┼──────────────┘
                    ▼
            ┌───────────────┐
            │  MessageBus   │  ← 入站队列（buffered channel）
            └───────┬───────┘
                    ▼
            ┌───────────────┐
            │  AgentLoop    │  ← 消费消息、路由到 Agent、处理响应
            │  ├─ Registry  │  ← 多 Agent 注册 + 路由匹配
            │  ├─ Runner    │  ← ADK-Go 的执行引擎
            │  └─ Compressor│  ← 上下文溢出时自动压缩
            └───────┬───────┘
                    ▼
            ┌───────────────┐
            │  MessageBus   │  ← 出站队列
            └───────┬───────┘
                    ▼
            ┌───────────────┐
            │ChannelAdapter │  ← 消费出站消息，分发到对应渠道
            └───────────────┘
```

## 六个核心概念

### 1. MessageBus — 解耦的关键

`pkg/bus/bus.go` 用两个 buffered Go channel 实现了一个进程内消息总线：

- `inbound` — 渠道/定时器把用户消息丢进来
- `outbound` — Agent 处理完把回复丢进来

为什么不直接调用？因为解耦。Telegram 适配器不需要知道 Agent 怎么跑，Agent 也不需要知道消息从哪来。
这和 NanoBot 的 `bus/` 模块设计思路完全一致，只是 NanoBot 用 Python asyncio queue，ABot 用 Go channel。

```go
// 生产者（任何渠道）
bus.PublishInbound(ctx, InboundMessage{Channel: "telegram", Content: "你好"})

// 消费者（AgentLoop）
msg, _ := bus.ConsumeInbound(ctx)  // 阻塞等待
```

### 2. AgentLoop — 大脑的主循环

`pkg/agent/loop.go` 是整个系统的心脏，一个无限循环：

1. 从 Bus 消费一条入站消息
2. 通过 Registry 路由到正确的 Agent
3. 确保 Session 存在（不存在就创建）
4. 调用 ADK Runner 执行 Agent（LLM 推理 + 工具调用）
5. 把回复发到 Bus 出站队列
6. 检查是否需要压缩 Session

关键设计：单条消息的 panic 不会打垮整个循环。`safeProcessMessage` 用 `defer/recover` 兜底。

### 3. AgentRegistry — 多 Agent 路由

`pkg/agent/registry.go` 管理多个 Agent 实例，根据消息来源路由到正确的 Agent。

路由优先级（从高到低）：
- 精确匹配 channel + chatID（得分 2）
- 仅匹配 channel（得分 1）
- 通配 / 默认 agent（得分 0）

```yaml
# config.yaml 中定义多个 agent
agents:
  - id: customer-service
    name: 客服助手
    routes:
      - channel: telegram        # 所有 Telegram 消息走这个 agent
  - id: dev-assistant
    name: 开发助手
    routes:
      - channel: cli             # CLI 走这个 agent
```

这让你可以在同一个 ABot 实例里跑多个不同人设的 Agent，按渠道或聊天分流。

### 4. Channel 适配器 — 统一的消息入口

`pkg/channels/` 下每个子目录是一个渠道适配器。所有适配器实现同一个接口：

```go
type Channel interface {
    Name() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Send(ctx context.Context, msg OutboundMessage) error
    IsAllowed(senderID string) bool
    IsRunning() bool
}
```

目前支持：CLI、Telegram、Discord、飞书、企业微信。

每个适配器的职责很简单：
- 收到平台消息 → 转成 `InboundMessage` → 丢进 Bus
- 从 Bus 消费 `OutboundMessage` → 转成平台格式 → 发出去

`ChannelRegistry` 统一管理所有适配器的生命周期，启动时如果某个适配器失败会自动回滚已启动的。

### 5. Compressor — 上下文窗口的安全网

`pkg/agent/compression.go` 解决一个实际问题：聊天记录越来越长，迟早撑爆 LLM 的上下文窗口。

触发条件（满足任一）：
- Session 事件数 > 50
- 估算 token 数 > 上下文窗口的 75%

压缩策略：
1. 正常压缩：用一个便宜的 LLM 把旧的 75% 对话总结成一段摘要，保留最近 25%
2. 强制压缩：如果总结也失败了，直接丢掉最老的 50%（紧急降级）

AgentLoop 还有重试机制：如果 LLM 返回 `context_length_exceeded` 错误，自动压缩后重试，最多 2 次。

### 6. Bootstrap — 依赖组装

`cmd/abot/main.go` 的 `buildDeps()` 是整个系统的接线图，按顺序组装所有依赖：

```
config.yaml
    → MySQL (GORM) → 各种 Store
    → SessionService (内存 / JSONL)
    → LLM Providers → FallbackLLM（自动故障转移）
    → MessageBus (buffered channel)
    → Tools (Shell、文件、Web、Cron、子Agent...)
    → MCP 客户端（外部工具服务器）
    → Skills 加载器 + 内置技能注册
    → VectorDB + Embedder（可选，用于记忆整合）
    → Channels (CLI + 配置的平台)
    → Heartbeat（可选定时任务）
    → BootstrapDeps → agent.Bootstrap() → App.Run()
```

`agent.Bootstrap()` 拿到所有依赖后，创建 Agent、Runner、AgentLoop，组装成 `App`。
`App.Run()` 启动所有组件，阻塞等待信号或错误，优雅关闭。

## 内置工具一览

`pkg/tools/` 下的每个文件对应一个 Agent 可调用的工具：

| 文件 | 工具 | 作用 |
|------|------|------|
| `shell.go` | Shell | 执行系统命令（带安全沙箱） |
| `filesystem.go` | FileRead/Write/Edit | 文件操作 |
| `web.go` | WebFetch | 抓取网页内容 |
| `cron.go` | CronAdd/Remove/List | 管理定时任务 |
| `spawn.go` | SpawnAgent | 启动子 Agent 执行后台任务 |
| `subagent.go` | SubAgent | 子 Agent 管理 |
| `message.go` | SendMessage | 向指定渠道发消息 |
| `skills.go` | SkillInstall/List | 技能管理 |
| `state.go` | StateGet/Set | 读写 Session 状态 |
| `list_tasks.go` | ListTasks | 查看后台任务列表 |
| `security.go` | — | 路径遍历防护、命令注入检测 |

## 与参考项目的对比

同目录下有两个参考项目，理解它们的关系有助于理解 ABot 的设计选择。

### ADK-Go（`adk-go-main/`）— ABot 的地基

Google 官方的 Agent Development Kit Go SDK。ABot 不是从零造轮子，而是站在 ADK-Go 上面搭建。

ABot 用了 ADK-Go 的这些核心组件：
- `agent/llmagent` — LLM Agent 抽象（接收指令、调用工具、生成回复）
- `runner` — Agent 执行引擎（管理 Agent ↔ LLM ↔ Tool 的循环）
- `session` — 会话管理接口（内存/持久化）
- `model` — LLM 接口抽象
- `tool` — 工具接口定义
- `plugin` — 插件钩子（before/after model call）

ABot 在 ADK-Go 之上增加的是：多渠道接入、消息总线、路由、压缩、技能系统、存储层。

### NanoBot（`nanobot-main/`）— ABot 的 Python 前身

NanoBot 是 Python 实现的同类项目，ABot 的很多设计直接借鉴自它。

| 维度 | NanoBot (Python) | ABot (Go) |
|------|-----------------|-----------|
| 语言 | Python + asyncio | Go + goroutine |
| LLM 接入 | LiteLLM（统一网关） | ADK-Go model 接口 + 自建 Fallback 链 |
| 消息总线 | asyncio.Queue | buffered Go channel |
| 渠道 | 10+（Telegram、Discord、WhatsApp、飞书、钉钉、Slack、QQ、Email...） | 5（CLI、Telegram、Discord、飞书、企微） |
| 配置 | JSON (`~/.nanobot/config.json`) | YAML (`config.yaml`) |
| 存储 | 文件系统为主 | MySQL + S3 + Qdrant |
| 技能 | Markdown 文件 + 动态加载 | 同，但加了 MySQL 注册表 |
| 多租户 | 无 | 有（tenant_id 贯穿全链路） |
| Agent 框架 | 自建 loop | 基于 ADK-Go Runner |

## 快速上手

### 最小配置

```yaml
# config.yaml
app_name: abot
mysql_dsn: "user:pass@tcp(127.0.0.1:3306)/abot?parseTime=true"

providers:
  - name: primary
    api_base: "https://api.openai.com/v1"
    api_key: "sk-YOUR-KEY"
    model: "gpt-4o-mini"

agents:
  - id: default-bot
    name: assistant
    description: "A helpful assistant"

context_window: 128000
```

### 启动

```bash
cd abot
go run ./cmd/abot/ -config config.yaml
```

启动后进入 CLI 交互模式，直接打字就能和 Agent 对话。

### 接入 Telegram

在 `config.yaml` 中加上：

```yaml
telegram:
  token: "YOUR_BOT_TOKEN"
  tenant_id: "default"
  allow_from: ["your_telegram_user_id"]  # 留空则允许所有人
```

重启即可。其他渠道（Discord、飞书、企微）类似，填对应的 token/key。

## 关键设计决策

### 为什么用 Go channel 做消息总线，而不是 Redis/Kafka？

当前是单进程架构，Go channel 零依赖、零延迟、类型安全。
如果未来需要多实例部署，Bus 接口已经抽象好了，换成 Redis Stream 只需要实现 `MessageBus` 接口。

### 为什么用 ADK-Go 而不是自建 Agent 循环？

ADK-Go 已经处理好了 Agent ↔ LLM ↔ Tool 的交互循环、流式响应、插件钩子等复杂逻辑。
ABot 专注于上层的多渠道接入、路由、压缩、技能管理，不重复造轮子。

### 为什么 Providers 用 Fallback 链？

生产环境中单一 LLM 提供商不可靠。配置多个 provider，第一个挂了自动切到第二个：

```yaml
providers:
  - name: primary
    model: "claude-sonnet-4-20250514"
    api_key: "sk-ant-..."
  - name: fallback
    model: "gpt-4o-mini"        # 主力挂了用这个兜底
    api_key: "sk-..."
```

最后一个 provider 还会被复用为 Summary LLM（用于会话压缩），通常配一个便宜的模型。

## 扩展指南

### 添加新渠道

1. 在 `pkg/channels/` 下新建目录，实现 `types.Channel` 接口
2. 在 `cmd/abot/main.go` 的 `newChannels()` 中根据配置创建实例
3. 在 `Config` 中加对应的配置结构体

核心就是两件事：收消息丢进 Bus，从 Bus 拿消息发出去。

### 添加新工具

1. 在 `pkg/tools/` 下新建文件
2. 实现 ADK-Go 的 `tool.Tool` 接口（通常用 `tool.NewFunctionTool`）
3. 在 `pkg/tools/builder.go` 的 `BuildAllTools()` 中注册

### 添加新插件

ADK-Go 的插件系统提供 before/after model call 钩子：

1. 在 `pkg/plugins/` 下新建目录
2. 用 `plugin.New()` 创建插件，注册回调
3. 在 `buildDeps()` 中加入 `plugins` 切片

### 接入外部工具（MCP）

不需要写代码，直接在 `config.yaml` 中配置：

```yaml
mcp_servers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  remote-api:
    url: "https://example.com/mcp/"
    headers:
      Authorization: "Bearer xxx"
    tool_timeout: 120
```

启动时自动连接 MCP 服务器，发现的工具会和内置工具一起注册给 Agent。
