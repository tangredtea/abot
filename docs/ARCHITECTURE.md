# Architecture

## Overview

abot is designed with a modular, pluggable architecture that separates concerns into distinct layers. The system uses a multi-binary approach where three independent executables share common core libraries, enabling flexible deployment while maintaining code reusability.

## Design Philosophy

1. **Separation of Concerns**: Each binary serves a specific purpose (CLI, API, Web)
2. **Zero Code Duplication**: Shared logic lives in `pkg/bootstrap` and core packages
3. **Pluggable Components**: Easy to swap storage backends, LLM providers, and channels
4. **Production Ready**: Built for scale with multi-tenancy, auth, and monitoring
5. **Developer Friendly**: Clear structure, comprehensive tests, and documentation

## System Layers

### 1. Interface Layer (`cmd/`)

Three independent binaries that share the same core engine:

#### abot-agent (CLI)
- **Purpose**: Personal assistant with REPL interface
- **Dependencies**: Minimal (no database required)
- **Use Case**: Quick testing, personal use, development
- **Features**:
  - Interactive command-line interface
  - In-memory or JSONL session storage
  - Lightweight tool set
  - Fast startup

#### abot-server (API)
- **Purpose**: HTTP API server for integration
- **Dependencies**: Full (requires MySQL)
- **Use Case**: Embed in applications, custom frontends
- **Features**:
  - RESTful API endpoints
  - WebSocket streaming
  - JWT authentication
  - Multi-tenant support
  - Full tool ecosystem

#### abot-web (Web Console)
- **Purpose**: Web-based UI for teams
- **Dependencies**: Full (requires MySQL + static files)
- **Use Case**: Team collaboration, production deployment
- **Features**:
  - Next.js-based web interface
  - User management and RBAC
  - Conversation history
  - Real-time updates
  - Admin dashboard

### 2. Bootstrap Layer (`pkg/bootstrap/`)

Shared dependency construction logic that eliminates code duplication:

**Core Functions:**

```go
// Configuration
LoadConfig(path string) (*agent.Config, error)
LoadConfigWithEnv(path string) (*agent.Config, error)

// Validation (mode-specific)
ValidateForAgent(cfg *agent.Config) error
ValidateForServer(cfg *agent.Config) error
ValidateForWeb(cfg *agent.Config) error

// Session Management
NewSessionService(cfg *agent.Config) (abotsession.Service, error)

// LLM Providers
BuildProviders(cfg *agent.Config) (map[string]llm.Provider, error)

// Database
NewDatabase(cfg *agent.Config) (*gorm.DB, error)
NewStores(db *gorm.DB) (*Stores, error)

// Channels
NewChannels(cfg *agent.Config, msgBus *bus.Bus) ([]channel.Channel, error)

// Tools
BuildLightweightTools(cfg *agent.Config) ([]any, error)
BuildFullTools(cfg *agent.Config, stores *Stores) ([]any, error)

// Dependencies
BuildCoreDeps(cfg *agent.Config) (*agent.Dependencies, error)
BuildFullDeps(cfg *agent.Config) (*agent.Dependencies, error)
```

**Design Decisions:**

- `BuildCoreDeps`: No database, suitable for CLI mode
- `BuildFullDeps`: Includes MySQL and all features
- Mode-specific validation ensures correct configuration
- Pluggable session storage (memory, JSONL, MySQL)

### 3. Core Engine (`pkg/agent/`)

The heart of abot, responsible for:

- **Agent Lifecycle Management**: Create, configure, and manage multiple agents
- **Message Routing**: Route messages to appropriate agents
- **Tool Execution**: Execute tools with proper context and error handling
- **Plugin System**: Load and manage plugins dynamically
- **Event Bus**: Publish/subscribe for cross-component communication

**Key Components:**

```go
// Agent definition
type Agent struct {
    ID           string
    Name         string
    Model        string
    SystemPrompt string
    Tools        []Tool
    Provider     llm.Provider
}

// Core engine
type Engine struct {
    Agents    map[string]*Agent
    Sessions  session.Service
    Tools     []Tool
    Bus       *bus.Bus
    Scheduler *cron.Scheduler
}

// Bootstrap function
func Bootstrap(ctx context.Context, cfg Config, deps Dependencies) (*Engine, error)
```

### 4. Storage Layer (`pkg/storage/`)

Pluggable storage backends with a common interface:

**Supported Backends:**

1. **MySQL** (Production)
   - Full ACID compliance
   - Relational data model
   - Multi-tenant isolation
   - Query optimization

2. **JSONL** (Development)
   - File-based storage
   - Human-readable format
   - No database setup required
   - Easy debugging

3. **In-Memory** (Testing)
   - Fast performance
   - No persistence
   - Ideal for unit tests
   - Zero configuration

**Storage Interface:**

```go
type Store interface {
    // Sessions
    CreateSession(ctx context.Context, session *Session) error
    GetSession(ctx context.Context, id string) (*Session, error)
    ListSessions(ctx context.Context, filter Filter) ([]*Session, error)
    UpdateSession(ctx context.Context, session *Session) error
    DeleteSession(ctx context.Context, id string) error

    // Messages
    CreateMessage(ctx context.Context, msg *Message) error
    ListMessages(ctx context.Context, sessionID string) ([]*Message, error)

    // Agents
    CreateAgent(ctx context.Context, agent *Agent) error
    GetAgent(ctx context.Context, id string) (*Agent, error)
    ListAgents(ctx context.Context) ([]*Agent, error)
    UpdateAgent(ctx context.Context, agent *Agent) error
    DeleteAgent(ctx context.Context, id string) error
}
```

### 5. Provider Layer (`pkg/providers/`)

LLM provider abstraction with fallback support:

**Supported Providers:**

- OpenAI (GPT-4, GPT-3.5)
- Anthropic (Claude)
- Azure OpenAI
- Custom providers (via API compatibility)

**Provider Interface:**

```go
type Provider interface {
    // Generate completion
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

    // Stream completion
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)

    // Provider info
    Name() string
    Models() []string
}
```

**Fallback Mechanism:**

```yaml
providers:
  - name: primary
    api_key: sk-xxx
    model: gpt-4o
  - name: fallback
    api_key: sk-yyy
    model: gpt-3.5-turbo
```

When primary fails, automatically falls back to secondary provider.

### 6. Channel Layer (`pkg/channels/`)

Multi-channel support for different communication platforms:

**Supported Channels:**

- **WeCom** (企业微信): Enterprise messaging
- **Telegram**: Bot API integration
- **Discord**: Bot with slash commands
- **Feishu** (飞书): Enterprise collaboration
- **Web**: WebSocket-based chat

**Channel Interface:**

```go
type Channel interface {
    // Start listening for messages
    Start(ctx context.Context) error

    // Send message
    Send(ctx context.Context, msg Message) error

    // Channel info
    Name() string
    Type() ChannelType
}
```

### 7. Tool System (`pkg/tools/`)

Extensible tool system for agent capabilities:

**Built-in Tools:**

- Web search (Google, Bing)
- File operations (read, write, list)
- Code execution (sandboxed)
- Database queries
- HTTP requests
- Image generation
- Document parsing
- Vector search

**Tool Interface:**

```go
type Tool interface {
    // Tool metadata
    Name() string
    Description() string
    Parameters() []Parameter

    // Execute tool
    Execute(ctx context.Context, params map[string]any) (any, error)
}
```

**Tool Registration:**

```go
// Lightweight tools (no database)
tools := bootstrap.BuildLightweightTools(cfg)

// Full tools (with database)
tools := bootstrap.BuildFullTools(cfg, stores)
```

## Component Diagram

```
┌─────────────────────────────────────────────────────────┐
│                    Interface Layer                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │   CLI    │  │   API    │  │   Web    │              │
│  │  (REPL)  │  │ (HTTP)   │  │  (UI)    │              │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘              │
└───────┼────────────┼─────────────┼────────────────────┘
        │            │             │
┌───────▼────────────▼─────────────▼────────────────────┐
│              Bootstrap Layer                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │  Config  │  │ Database │  │ Providers│            │
│  │  Loader  │  │  Setup   │  │  Init    │            │
│  └──────────┘  └──────────┘  └──────────┘            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │ Session  │  │ Channels │  │  Tools   │            │
│  │  Setup   │  │  Init    │  │  Build   │            │
│  └──────────┘  └──────────┘  └──────────┘            │
└────────────────────┬──────────────────────────────────┘
                     │
┌────────────────────▼──────────────────────────────────┐
│                 Core Engine                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │  Agent   │  │   Tool   │  │  Plugin  │            │
│  │ Registry │  │ Executor │  │  System  │            │
│  └──────────┘  └──────────┘  └──────────┘            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │ Message  │  │  Event   │  │  Cron    │            │
│  │  Router  │  │   Bus    │  │Scheduler │            │
│  └──────────┘  └──────────┘  └──────────┘            │
└────────────────────┬──────────────────────────────────┘
                     │
┌────────────────────▼──────────────────────────────────┐
│              Storage & Providers                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │  MySQL   │  │  OpenAI  │  │ Telegram │            │
│  │  Store   │  │ Provider │  │ Channel  │            │
│  └──────────┘  └──────────┘  └──────────┘            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │  JSONL   │  │ Anthropic│  │  WeCom   │            │
│  │  Store   │  │ Provider │  │ Channel  │            │
│  └──────────┘  └──────────┘  └──────────┘            │
└───────────────────────────────────────────────────────┘
```

## Data Flow

### CLI Mode (abot-agent)

```
User Input
    │
    ▼
┌─────────┐
│  REPL   │
└────┬────┘
     │
     ▼
┌─────────────┐
│ Core Engine │
└────┬────────┘
     │
     ├──► Tool Execution
     │
     ▼
┌──────────────┐
│ LLM Provider │
└────┬─────────┘
     │
     ▼
┌─────────┐
│Response │
└────┬────┘
     │
     ▼
User Output
```

### API Mode (abot-server)

```
HTTP Request
    │
    ▼
┌──────────────┐
│ API Handler  │
└────┬─────────┘
     │
     ├──► Authentication (JWT)
     ├──► Authorization (RBAC)
     │
     ▼
┌─────────────┐
│ Core Engine │
└────┬────────┘
     │
     ├──► Tool Execution
     ├──► Database Queries
     │
     ▼
┌──────────────┐
│ LLM Provider │
└────┬─────────┘
     │
     ▼
┌──────────────┐
│HTTP Response │
└──────────────┘
```

### Web Mode (abot-web)

```
Browser
    │
    ▼
┌──────────────┐
│  Next.js UI  │
└────┬─────────┘
     │
     ▼
┌──────────────┐
│  WebSocket   │
└────┬─────────┘
     │
     ▼
┌──────────────┐
│ API Handler  │
└────┬─────────┘
     │
     ├──► Authentication
     ├──► Session Management
     │
     ▼
┌─────────────┐
│ Core Engine │
└────┬────────┘
     │
     ├──► Tool Execution
     ├──► Database Queries
     │
     ▼
┌──────────────┐
│ LLM Provider │
└────┬─────────┘
     │
     ▼
┌──────────────┐
│   Streaming  │
│   Response   │
└────┬─────────┘
     │
     ▼
Browser (Real-time)
```

## Key Design Decisions

### 1. Multi-Binary Architecture

**Why?**
- Clear separation of concerns
- Smaller binary sizes (CLI doesn't need web assets)
- Independent deployment and scaling
- Better security (minimal attack surface per binary)
- Easier testing and maintenance

**Trade-offs:**
- More binaries to build and distribute
- Need shared code in `pkg/bootstrap/`
- Slightly more complex build process

**Alternatives Considered:**
- Single binary with subcommands (rejected: too large, security concerns)
- Microservices (rejected: too complex for this use case)

### 2. Bootstrap Package Design

**Why centralize dependency construction?**

Before refactoring, each binary (`cmd/abot/main.go`) had duplicate code for:
- Loading configuration
- Initializing database
- Setting up LLM providers
- Building tools
- Creating channels

This led to:
- Code duplication across binaries
- Inconsistent initialization logic
- Difficult to maintain and test

**Solution: `pkg/bootstrap/`**

All shared initialization logic moved to a single package with clear responsibilities:

```go
// Each file has a single responsibility
config.go      → Configuration loading
validation.go  → Mode-specific validation
session.go     → Session service setup
providers.go   → LLM provider initialization
database.go    → Database connection
channels.go    → Channel adapters
tools.go       → Tool registration
deps.go        → Dependency assembly
```

**Benefits:**
- Zero code duplication
- Single source of truth
- Easy to test (unit tests for each function)
- Clear dependency graph
- Mode-specific validation (agent/server/web)

### 3. Pluggable Storage

**Why?**
- Support different deployment scenarios
- Easy to test (in-memory)
- Easy to develop (JSONL)
- Production-ready (MySQL)
- Future-proof (can add PostgreSQL, MongoDB, etc.)

**Trade-offs:**
- Interface abstraction overhead
- Need to maintain multiple implementations
- Lowest common denominator API

**Implementation:**
```go
type SessionService interface {
    Create(ctx context.Context, session *Session) error
    Get(ctx context.Context, id string) (*Session, error)
    // ... other methods
}

// Implementations
type MemorySessionService struct { /* ... */ }
type JSONLSessionService struct { /* ... */ }
type MySQLSessionService struct { /* ... */ }
```

### 4. Provider Abstraction

**Why?**
- Support multiple LLM providers
- Easy to add new providers
- Fallback mechanism for reliability
- Cost optimization (use cheaper models when possible)

**Trade-offs:**
- Lowest common denominator API
- Provider-specific features may be lost
- Need to handle different error formats

**Fallback Strategy:**
```go
func (p *FallbackProvider) Complete(ctx context.Context, req Request) (*Response, error) {
    // Try primary provider
    resp, err := p.primary.Complete(ctx, req)
    if err == nil {
        return resp, nil
    }

    // Log failure and try fallback
    log.Warn("primary provider failed, using fallback", "error", err)
    return p.fallback.Complete(ctx, req)
}
```

### 5. Tool System Design

**Why Lightweight vs Full Tools?**

**Lightweight Tools** (CLI mode):
- No database required
- Fast startup
- Suitable for personal use
- Examples: web search, file operations, HTTP requests

**Full Tools** (Server/Web mode):
- Require database
- More features
- Suitable for teams
- Examples: user management, conversation history, analytics

**Tool Registration:**
```go
// pkg/bootstrap/tools.go
func BuildLightweightTools(cfg *agent.Config) ([]any, error) {
    tools := []any{
        websearch.New(cfg.WebSearch),
        fileops.New(cfg.FileOps),
        httprequest.New(),
    }
    return tools, nil
}

func BuildFullTools(cfg *agent.Config, stores *Stores) ([]any, error) {
    tools, _ := BuildLightweightTools(cfg)
    tools = append(tools,
        usermgmt.New(stores.Users),
        analytics.New(stores.Analytics),
        // ... more tools
    )
    return tools, nil
}
```

## Scalability

### Horizontal Scaling

**Web/Server Mode:**

```
┌─────────┐  ┌─────────┐  ┌─────────┐
│ abot-web│  │ abot-web│  │ abot-web│
│Instance1│  │Instance2│  │Instance3│
└────┬────┘  └────┬────┘  └────┬────┘
     │            │            │
     └────────────┴────────────┘
                  │
          ┌───────▼────────┐
          │  Load Balancer │
          └───────┬────────┘
                  │
          ┌───────▼────────┐
          │     MySQL      │
          └────────────────┘
```

**Considerations:**
- Stateless design (session in database, not memory)
- Shared MySQL database
- WebSocket sticky sessions (if needed)
- Distributed caching (Redis) for performance

### Vertical Scaling

**Optimize Single Instance:**
- Increase CPU/memory for LLM processing
- Use faster storage (NVMe SSD)
- Optimize database queries (indexes, query plans)
- Connection pooling
- Caching frequently accessed data

### Database Scaling

**Read Replicas:**
```
┌──────────┐
│  Primary │ ◄─── Writes
└────┬─────┘
     │
     ├──► Replica 1 ◄─── Reads
     ├──► Replica 2 ◄─── Reads
     └──► Replica 3 ◄─── Reads
```

**Sharding (Future):**
- Shard by tenant_id
- Separate databases per tenant
- Tenant routing layer

## Security

### Authentication

**JWT Tokens:**
```go
type Claims struct {
    UserID   string `json:"user_id"`
    TenantID string `json:"tenant_id"`
    Role     string `json:"role"`
    jwt.StandardClaims
}
```

**Password Hashing:**
```go
// Bcrypt with cost 12
hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), 12)
```

### Authorization

**Role-Based Access Control (RBAC):**

```go
type Role string

const (
    RoleAdmin  Role = "admin"
    RoleUser   Role = "user"
    RoleViewer Role = "viewer"
)
```

**Tenant Isolation:**
```go
// All queries filtered by tenant_id
func (s *MySQLStore) ListSessions(ctx context.Context, tenantID string) ([]*Session, error) {
    var sessions []*Session
    err := s.db.Where("tenant_id = ?", tenantID).Find(&sessions).Error
    return sessions, err
}
```

## Monitoring

### Metrics

**Key Metrics:**
- Request count (by endpoint, status code)
- Response time (p50, p95, p99)
- Error rate
- Token usage (by model, tenant)
- Active sessions
- Database query time
- LLM provider latency

### Logging

**Structured Logging (slog):**
```go
import "log/slog"

slog.Info("request received",
    "method", r.Method,
    "path", r.URL.Path,
    "user_id", userID,
)
```

### Health Checks

```go
// Basic health check
GET /health
Response: {"status": "ok"}

// Detailed health check
GET /health/detailed
Response: {
    "status": "ok",
    "database": "ok",
    "providers": {"openai": "ok"},
    "uptime": "24h30m"
}
```

## Testing Strategy

### Unit Tests

**Coverage Target: 80%+**

```go
func TestBuildCoreDeps(t *testing.T) {
    cfg := &agent.Config{
        Providers: []agent.ProviderConfig{
            {APIKey: "test", Model: "gpt-4"},
        },
    }

    deps, err := bootstrap.BuildCoreDeps(cfg)
    assert.NoError(t, err)
    assert.NotNil(t, deps)
}
```

### Integration Tests

```go
func TestAPIEndToEnd(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    server := startTestServer(t, db)
    defer server.Close()

    resp := httptest.NewRequest("POST", "/api/v1/sessions", body)
    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

## Future Enhancements

- [ ] Distributed Tracing (OpenTelemetry)
- [ ] Metrics Export (Prometheus)
- [ ] Caching Layer (Redis)
- [ ] Message Queue (RabbitMQ/Kafka)
- [ ] GraphQL API
- [ ] Plugin Marketplace

---

**Last Updated**: 2026-03-02
**Version**: 2.0
