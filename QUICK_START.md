# ABot 快速开始

## 启动服务

### 1. 启动后端 (端口 3001)
```bash
./abot console --config config.test.yaml
```

### 2. 启动前端 (端口 3000)
```bash
cd web
npm run dev
```

## 访问

- 前端: http://localhost:3000
- 后端 API: http://localhost:3001

## 测试账号

- 邮箱: `test@example.com`
- 密码: `Test123456`

## 完整使用流程

### 1. 登录系统
访问 http://localhost:3000，使用测试账号登录

### 2. 创建 Agent
- 进入 "Agents" 页面（点击左侧菜单的 🤖 图标）
- 点击 "创建 Agent" 按钮
- 填写 Agent 信息：
  - **名称**：例如 "客服助手"
  - **描述**：例如 "专业的客服 AI 助手"
  - **头像**：选择一个 Emoji，例如 🤖
  - **模型**：MiniMax-M2.5（默认）
  - **Provider**：primary (MiniMax)
- 点击创建
- **Agent 会立即注册到运行时**，无需重启服务

### 3. 配置 Agent（可选但推荐）
- 在 Agent 列表中点击 "配置" 按钮
- 进入配置页面，有 4 个标签页：

#### 基本信息
- 修改名称、描述、头像
- 选择模型和 Provider
- 设置状态（运行中/已停用）

#### 人设配置（重要！）
- 使用 **Workspace 文档系统** 定义 Agent 的人格和行为
- 3 个核心文档类型：
  - **IDENTITY**：Agent 的身份、角色定义
  - **SOUL**：Agent 的核心价值观、性格特征和行为准则
  - **AGENT**：Agent 的特殊能力和工作规则
- 例如 SOUL 文档：
  ```
  我是一个友好、专业的AI助手。

  核心价值观：诚实、专业、同理心
  性格特征：友好、耐心、幽默
  行为准则：始终保持礼貌、主动提供帮助
  ```
- **修改后点击"保存"，Agent 会在下次对话时自动加载新配置**
- 文档存储在 `user_workspace_docs` 表，支持版本控制

#### 通道配置
- 选择 Agent 可接入的通信渠道
- 目前支持：Web、企业微信、Telegram、Discord、飞书
- 至少启用 "Web 控制台" 才能在前端对话

#### 高级设置
- **Temperature**：控制回答的创造性（0-2）
  - 0：精确、确定性强
  - 1：平衡
  - 2：创造性强、随机性高
- **Max Tokens**：单次回复的最大长度
- **Top P**：控制采样的多样性

### 4. 开始对话
- 点击左侧菜单的 "💬" 图标或访问 /chat 页面
- 看到所有可用的 Agent 列表
- **点击一个 Agent 卡片**
- 系统自动创建新的对话会话并跳转到聊天界面
- 在输入框输入消息，按回车发送
- Agent 会实时流式响应（逐字显示）

### 5. 管理会话
- 左侧边栏显示所有对话会话，按 Agent 分组
- 点击会话可以切换到该对话
- 鼠标悬停在会话上可以看到删除按钮
- 点击 "新建对话" 返回 Agent 选择页面

## 核心概念

### Agent（智能体）
- Agent 是一个 AI 助手实例
- 每个 Agent 有独立的配置：
  - **模型**：使用哪个 LLM（如 MiniMax-M2.5）
  - **人设**：System Prompt 定义角色和行为
  - **通道**：可以接入哪些通信渠道
  - **参数**：Temperature、Max Tokens 等
- 可以创建多个 Agent 用于不同场景
- **Agent 创建后立即可用，修改配置后立即生效**

### Session（会话）
- Session 是用户与 Agent 的一次对话
- 每个 Session 关联一个 Agent（创建后不可更改）
- Session 保存完整的对话历史
- 可以创建多个 Session 与同一个 Agent 对话

### Channel（通道）
- Channel 是 Agent 接入的通信渠道
- 目前支持：
  - **Web**：前端聊天界面
  - **企业微信**：企业微信机器人
  - **Telegram**：Telegram Bot
  - **Discord**：Discord Bot
  - **飞书**：飞书机器人
- 可以为每个 Agent 单独配置启用的通道

## 技术架构

### 动态 Agent 管理
```
前端创建 Agent
    ↓
保存到 MySQL (agent_definitions 表)
    ↓
AgentManager 注册到 Registry
    ↓
创建 ADK Agent + Runner
    ↓
立即可用，无需重启
```

### 对话流程
```
用户选择 Agent
    ↓
创建 Session (chat_sessions 表)
    ↓
用户发送消息 → WebSocket 连接
    ↓
WebSocket 从 Session 获取 Agent ID
    ↓
AgentLoop.ProcessDirectStream 处理
    ↓
调用 LLM → 流式返回
    ↓
前端实时显示
```

### 配置更新流程
```
用户修改 Workspace 文档 (IDENTITY/SOUL/AGENT)
    ↓
保存到 user_workspace_docs 表
    ↓
Agent 下次对话时 ContextBuilder 自动加载
    ↓
注入到系统提示词
    ↓
立即生效
```

## 数据库

- MySQL 8.0
- 数据库: `abot`
- 用户: `root`
- 密码: `root123`

### 核心表
- `agent_definitions`：Agent 配置（model, provider, channels）
- `workspace_docs`：租户级 Workspace 文档（IDENTITY, SOUL, RULES, AGENT）
- `user_workspace_docs`：用户级 Workspace 文档（覆盖租户级）
- `chat_sessions`：对话会话
- `accounts`：用户账号
- `tenants`：租户

## 故障排查

### Agent 创建后无法对话
1. 检查 Agent 状态是否为 "运行中"
2. 确认 "Web 控制台" 通道已启用
3. 查看后端日志是否有错误
4. 确认 MiniMax API Key 配置正确

### 修改 System Prompt 后没有生效
1. 确认点击了 "保存" 按钮
2. 创建新的对话会话（旧会话使用旧配置）
3. 查看后端日志确认文档已保存
4. Workspace 文档在每次对话时动态加载

### WebSocket 连接失败
- 检查后端是否在 3001 端口运行
- 检查浏览器控制台是否有错误
- 确认 Token 是否有效（重新登录）

### Agent 不响应或响应很慢
- 检查 MiniMax API Key 是否正确
- 查看后端日志是否有 API 调用错误
- 检查网络连接
- MiniMax API 可能有速率限制

### 数据库连接失败
- 确认 MySQL 服务已启动
- 检查 config.test.yaml 中的 DSN 配置
- 确认数据库用户名和密码正确
- 确认数据库 `abot` 已创建

## 高级功能

### 多租户支持
- 系统支持多租户隔离
- 每个用户属于一个或多个租户
- Agent 和 Session 都关联租户

### 工具调用
- Agent 可以调用内置工具（17 个）
- 支持 MCP (Model Context Protocol) 工具
- 工具调用会在聊天界面显示

### 会话压缩
- 当对话历史过长时自动压缩
- 使用 LLM 总结历史消息
- 保持上下文的同时减少 Token 消耗

## 注意事项

- 必须先创建 Agent 才能开始对话
- 每个会话关联一个 Agent，创建后不能更改
- Agent 配置修改后立即生效，无需重启
- System Prompt 对 Agent 行为影响很大，建议仔细编写
- Workspace 文档（IDENTITY, SOUL, AGENT）在每次对话时动态加载
- 修改文档后需要创建新会话才能看到效果（旧会话使用旧配置）
- Temperature 越高回答越有创造性，但也可能不够准确
- 确保 Web 通道已启用，否则无法在前端对话
