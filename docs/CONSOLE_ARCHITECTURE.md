# Console 架构设计

## 设计目标

1. **可插拔**: Console 作为独立模块，可以完全移除而不影响核心功能
2. **解耦**: Console 通过标准接口与核心交互，不直接依赖内部实现
3. **简单**: Console 像 CLI 命令一样简单，只是封装了 HTTP API

## 架构分层

```
┌─────────────────────────────────────────────────┐
│              Web UI (React/Next.js)             │
│  - Agent 管理界面                                │
│  - 会话聊天界面                                  │
│  - 配置管理界面                                  │
└────────────────┬────────────────────────────────┘
                 │ HTTP/WebSocket
┌────────────────┴────────────────────────────────┐
│           Console API Layer (可插拔)             │
│  pkg/console/                                   │
│  ├── api/          # HTTP handlers              │
│  ├── service/      # 业务逻辑封装                │
│  └── adapter/      # 核心接口适配器              │
└────────────────┬────────────────────────────────┘
                 │ 标准接口
┌────────────────┴────────────────────────────────┐
│              Core Interfaces                    │
│  pkg/types/                                     │
│  ├── AgentManager    # Agent CRUD               │
│  ├── SessionManager  # 会话管理                 │
│  ├── ConfigManager   # 配置管理                 │
│  └── ChatService     # 对话服务                 │
└────────────────┬────────────────────────────────┘
                 │
┌────────────────┴────────────────────────────────┐
│              Core Engine                        │
│  pkg/agent/      # Agent 核心逻辑               │
│  pkg/bus/        # 消息总线                     │
│  pkg/providers/  # LLM 提供商                   │
│  pkg/tools/      # 工具系统                     │
│  pkg/session/    # 会话持久化                   │
└─────────────────────────────────────────────────┘
```

## 核心接口定义

### AgentManager - Agent 管理接口

```go
type AgentManager interface {
    // CRUD operations
    CreateAgent(ctx context.Context, def AgentDefinition) (*AgentDefinition, error)
    GetAgent(ctx context.Context, id string) (*AgentDefinition, error)
    ListAgents(ctx context.Context, filter AgentFilter) ([]*AgentDefinition, error)
    UpdateAgent(ctx context.Context, id string, updates AgentUpdates) error
    DeleteAgent(ctx context.Context, id string) error
    
    // Configuration
    GetAgentConfig(ctx context.Context, id string) (*AgentConfig, error)
    UpdateAgentConfig(ctx context.Context, id string, config *AgentConfig) error
    
    // Channels
    GetAgentChannels(ctx context.Context, id string) ([]*ChannelConfig, error)
    UpdateAgentChannels(ctx context.Context, id string, channels []*ChannelConfig) error
    
    // Runtime control
    StartAgent(ctx context.Context, id string) error
    StopAgent(ctx context.Context, id string) error
    RestartAgent(ctx context.Context, id string) error
}
```

### ChatService - 对话服务接口

```go
type ChatService interface {
    // Send message to agent
    SendMessage(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    
    // Stream chat (for WebSocket)
    StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error)
    
    // Get chat history
    GetHistory(ctx context.Context, sessionID string, limit int) ([]*Message, error)
}
```

### ConfigManager - 配置管理接口

```go
type ConfigManager interface {
    // Provider settings
    GetProviders(ctx context.Context) ([]*ProviderConfig, error)
    UpdateProviders(ctx context.Context, providers []*ProviderConfig) error
    
    // System settings
    GetSystemConfig(ctx context.Context) (*SystemConfig, error)
    UpdateSystemConfig(ctx context.Context, config *SystemConfig) error
}
```

## Console 服务层

Console 不直接操作数据库或核心组件，而是通过服务层：

```go
// pkg/console/service/agent_service.go
type AgentService struct {
    manager types.AgentManager
    store   types.AgentDefinitionStore
}

func (s *AgentService) CreateAgent(ctx context.Context, req CreateAgentRequest) (*AgentResponse, error) {
    // 1. 验证请求
    // 2. 调用 manager 创建 agent
    // 3. 转换为响应格式
}
```

## 启动模式

### 1. Gateway 模式（原有模式）
```bash
./abot gateway -config config.yaml
```
- 启动所有 channels (Telegram, Discord, etc.)
- 不启动 Console API
- 纯后台服务

### 2. Console 模式（管理模式）
```bash
./abot console -config config.yaml
```
- 启动 Console API + Web UI
- 启动核心 Agent 引擎
- 可选启动 channels（用于测试）

### 3. All-in-One 模式
```bash
./abot all -config config.yaml
```
- 同时启动 Gateway + Console
- 适合小规模部署

## 配置示例

```yaml
# config.yaml

# 核心配置
app_name: abot
mysql_dsn: "user:pass@tcp(127.0.0.1:3306)/abot"

# LLM 提供商
providers:
  - name: primary
    api_base: "https://api.minimaxi.com/anthropic"
    api_key: "xxx"
    model: "MiniMax-M2.5"

# Agent 定义（可选，也可以通过 Console UI 创建）
agents:
  - id: default-bot
    name: "默认助手"
    description: "通用 AI 助手"
    config:
      provider: primary
      model: "MiniMax-M2.5"
      system_prompt: "你是一个有帮助的 AI 助手"
      temperature: 0.7
      max_tokens: 2048
    channels:
      - channel: web
        enabled: true
      - channel: telegram
        enabled: false

# Console 配置（仅 console 模式需要）
console:
  enabled: true
  addr: ":3001"
  jwt_secret: "your-secret-key"
  allowed_origins:
    - "http://localhost:3000"
  static_dir: "web/out"

# Channels 配置（仅 gateway 模式需要）
telegram:
  token: "YOUR_BOT_TOKEN"
  tenant_id: "default"

discord:
  token: "YOUR_BOT_TOKEN"
  tenant_id: "default"
```

## 数据库设计

### agent_definitions 表（已存在）
```sql
CREATE TABLE agent_definitions (
    id VARCHAR(255) PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    avatar VARCHAR(255),
    status ENUM('active', 'inactive') DEFAULT 'active',
    
    -- 模型配置
    provider VARCHAR(255),
    model VARCHAR(255),
    
    -- Agent 配置（JSON）
    config JSON,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_tenant (tenant_id),
    INDEX idx_status (status)
);
```

### agent_channels 表（已存在）
```sql
CREATE TABLE agent_channels (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    agent_id VARCHAR(255) NOT NULL,
    channel VARCHAR(255) NOT NULL,
    enabled BOOLEAN DEFAULT true,
    config JSON,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    UNIQUE KEY uk_agent_channel (agent_id, channel),
    INDEX idx_agent (agent_id)
);
```

### provider_settings 表（新增）
```sql
CREATE TABLE provider_settings (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    api_base VARCHAR(512),
    api_key_encrypted TEXT,
    model VARCHAR(255),
    priority INT DEFAULT 0,
    enabled BOOLEAN DEFAULT true,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    UNIQUE KEY uk_tenant_name (tenant_id, name),
    INDEX idx_tenant (tenant_id)
);
```

## 前端功能

### 1. Agent 管理
- ✅ 列表展示（已完成）
- ✅ 创建 Agent（已完成）
- ✅ 编辑配置（已完成）
- 🔲 启动/停止 Agent
- 🔲 删除 Agent
- 🔲 克隆 Agent

### 2. 会话聊天
- 🔲 选择 Agent 进行对话
- 🔲 实时流式响应
- 🔲 工具调用展示
- 🔲 会话历史
- 🔲 多会话管理

### 3. 配置管理
- 🔲 Provider 配置（API Key 加密存储）
- 🔲 系统设置
- 🔲 Channel 配置

## 实现步骤

### Phase 1: 接口定义（本次）
- [x] 定义核心接口
- [ ] 实现 AgentManager
- [ ] 实现 ChatService
- [ ] 实现 ConfigManager

### Phase 2: Console 服务层
- [ ] 创建 console/service 包
- [ ] 实现业务逻辑封装
- [ ] 添加权限控制

### Phase 3: API 完善
- [ ] Agent 启动/停止接口
- [ ] 实时聊天 WebSocket
- [ ] Provider 配置接口

### Phase 4: 前端完善
- [ ] Agent 聊天界面
- [ ] 配置管理界面
- [ ] 实时状态监控

## 优势

1. **解耦**: Console 可以完全移除，不影响核心功能
2. **可测试**: 接口清晰，易于单元测试
3. **可扩展**: 可以轻松添加新的管理功能
4. **多实例**: 可以多个 Console 连接同一个核心
5. **安全**: 通过接口层控制权限，不直接暴露内部实现
