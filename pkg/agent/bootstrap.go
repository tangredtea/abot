package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"

	"abot/pkg/channels"
	"abot/pkg/scheduler"
	"abot/pkg/types"
)

// Config is the top-level application configuration.
type Config struct {
	AppName       string                     `yaml:"app_name"`
	MySQLDSN      string                     `yaml:"mysql_dsn"`
	ObjectStore   ObjectStoreConfig          `yaml:"object_store"`
	VectorDB      VectorDBConfig             `yaml:"vector_db"`
	Embedding     EmbeddingConfig            `yaml:"embedding"`
	Cache         CacheConfig                `yaml:"cache"`
	Providers     []ProviderConfig           `yaml:"providers"`
	Agents        []AgentDefConfig           `yaml:"agents"`
	Plugins       PluginsConfig              `yaml:"plugins"`
	Scheduler     SchedulerConfig            `yaml:"scheduler"`
	MCP           map[string]MCPServerConfig `yaml:"mcp_servers"`
	WeCom         WeComChannelConfig         `yaml:"wecom,omitempty"`
	Telegram      TelegramChannelConfig      `yaml:"telegram,omitempty"`
	Discord       DiscordChannelConfig       `yaml:"discord,omitempty"`
	Feishu        FeishuChannelConfig        `yaml:"feishu,omitempty"`
	Session       SessionConfig              `yaml:"session,omitempty"`
	A2A           A2AConfig                  `yaml:"a2a"`
	Sandbox       SandboxConfig              `yaml:"sandbox,omitempty"`
	Console       ConsoleConfig              `yaml:"console,omitempty"`
	SkillCacheDir string                     `yaml:"skill_cache_dir"`
	ContextWindow int                        `yaml:"context_window"`
	BusBufferSize int                        `yaml:"bus_buffer_size"`
	HealthAddr    string                     `yaml:"health_addr,omitempty"` // e.g. ":8081", empty = disabled

	// Runtime overrides (set via CLI flags, not YAML).
	CLITenantID   string `yaml:"-"`
	CLIUserID     string `yaml:"-"`
	CLINoMarkdown bool   `yaml:"-"`
}

// Sub-config types.
type ObjectStoreConfig struct {
	Type   string `yaml:"type"` // "local" or "s3"
	Bucket string `yaml:"bucket"`
	Region string `yaml:"region"`
	Dir    string `yaml:"dir"` // for local type
}

type VectorDBConfig struct {
	Addr       string `yaml:"addr"`
	Collection string `yaml:"collection"`
}

type EmbeddingConfig struct {
	APIBase   string `yaml:"api_base"`
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	Dimension int    `yaml:"dimension"` // vector dimension, default 768
}

type CacheConfig struct {
	TenantSize int `yaml:"tenant_size"`
	SkillSize  int `yaml:"skill_size"`
}

type ProviderConfig struct {
	Name           string `yaml:"name"`
	APIBase        string `yaml:"api_base"`
	APIKey         string `yaml:"api_key"`
	Model          string `yaml:"model"`
	Proxy          string `yaml:"proxy,omitempty"`
	MaxTokensField string `yaml:"max_tokens_field,omitempty"`
	PromptCaching  bool   `yaml:"prompt_caching,omitempty"`
}

type SchedulerConfig struct {
	HeartbeatInterval string `yaml:"heartbeat_interval"`
	HeartbeatChannel  string `yaml:"heartbeat_channel"`
	DecisionMode      string `yaml:"decision_mode"` // "passive" (default) or "llm"
}

// MCPServerConfig describes a single MCP server to connect to.
type MCPServerConfig struct {
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	Env         map[string]string `yaml:"env,omitempty"`
	URL         string            `yaml:"url,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty"`
	ToolTimeout int               `yaml:"tool_timeout"`
}

// WeComChannelConfig holds WeCom Bot channel configuration.
type WeComChannelConfig struct {
	Token          string   `yaml:"token"`
	EncodingAESKey string   `yaml:"encoding_aes_key"`
	WebhookURL     string   `yaml:"webhook_url"`
	WebhookHost    string   `yaml:"webhook_host"`
	WebhookPort    int      `yaml:"webhook_port"`
	WebhookPath    string   `yaml:"webhook_path"`
	ReplyTimeout   int      `yaml:"reply_timeout"`
	AllowFrom      []string `yaml:"allow_from"`
	TenantID       string   `yaml:"tenant_id"`
	UserID         string   `yaml:"user_id"`
}

// TelegramChannelConfig holds Telegram Bot channel configuration.
type TelegramChannelConfig struct {
	Token       string   `yaml:"token"`
	AllowFrom   []string `yaml:"allow_from,omitempty"`
	TenantID    string   `yaml:"tenant_id"`
	PollTimeout int      `yaml:"poll_timeout,omitempty"`
}

// DiscordChannelConfig holds Discord Bot channel configuration.
type DiscordChannelConfig struct {
	Token     string   `yaml:"token"`
	AllowFrom []string `yaml:"allow_from,omitempty"`
	TenantID  string   `yaml:"tenant_id"`
	GuildID   string   `yaml:"guild_id,omitempty"`
}

// FeishuChannelConfig holds Feishu Bot channel configuration.
type FeishuChannelConfig struct {
	AppID             string   `yaml:"app_id"`
	AppSecret         string   `yaml:"app_secret"`
	VerificationToken string   `yaml:"verification_token"`
	EncryptKey        string   `yaml:"encrypt_key,omitempty"`
	WebhookHost       string   `yaml:"webhook_host"`
	WebhookPort       int      `yaml:"webhook_port"`
	WebhookPath       string   `yaml:"webhook_path"`
	AllowFrom         []string `yaml:"allow_from,omitempty"`
	TenantID          string   `yaml:"tenant_id"`
}

// SessionConfig controls session persistence.
type SessionConfig struct {
	Type string `yaml:"type"` // "memory" (default) or "jsonl"
	Dir  string `yaml:"dir"`  // directory for jsonl files
}

// ConsoleConfig holds web console configuration.
type ConsoleConfig struct {
	Addr             string   `yaml:"addr"` // e.g. ":3000"
	JWTSecret        string   `yaml:"jwt_secret"`
	EncryptionSecret string   `yaml:"encryption_secret"` // separate key for API-key-at-rest encryption; falls back to jwt_secret
	StaticDir        string   `yaml:"static_dir"`        // e.g. "web/out"
	AllowedOrigins   []string `yaml:"allowed_origins"`   // CORS allowed origins
}

// SandboxConfig controls workspace security sandboxing.
type SandboxConfig struct {
	RestrictToWorkspace bool     `yaml:"restrict_to_workspace"`    // enforce all file ops within workspace
	AllowedPaths        []string `yaml:"allowed_paths"`            // absolute paths allowed outside workspace
	ExtraDenyPatterns   []string `yaml:"extra_deny_patterns"`      // additional shell command deny patterns
	ExecMemoryMB        int      `yaml:"exec_memory_mb"`           // ulimit -v per exec (default 512, 0=no limit)
	ExecCPUSeconds      int      `yaml:"exec_cpu_seconds"`         // ulimit -t per exec (default 30, 0=no limit)
	ExecFileSizeMB      int      `yaml:"exec_filesize_mb"`         // ulimit -f per exec (default 50, 0=no limit)
	ExecNProc           int      `yaml:"exec_nproc"`               // ulimit -u per exec (default 64, 0=no limit)
	RateLimit           float64  `yaml:"rate_limit"`               // tool calls per second per tenant (0=no limit)
	RateBurst           int      `yaml:"rate_burst"`               // max burst for rate limiter (default 10)
	Level               string   `yaml:"level,omitempty"`          // "none" / "standard" / "strict" / "container"
	SandboxBinary       string   `yaml:"sandbox_binary,omitempty"` // path to abot-sandbox (for Landlock modes)

	// Container sandbox options (used when level == "container").
	// Each exec call runs inside an isolated Docker/OCI container.
	// Recommended runtime: gVisor (runsc) for syscall-level isolation.
	ContainerImage   string `yaml:"container_image,omitempty"`   // sandbox image (default "abot/sandbox:latest")
	ContainerRuntime string `yaml:"container_runtime,omitempty"` // OCI runtime: "runsc" (gVisor) or empty (runc)
	ContainerBinary  string `yaml:"container_binary,omitempty"`  // docker/nerdctl/podman binary (auto-detected)
	ContainerMemMB   int    `yaml:"container_mem_mb,omitempty"`  // per-container memory limit (default 512)
	ContainerCPUs    string `yaml:"container_cpus,omitempty"`    // CPU quota, e.g. "0.5" (default "1")
	ContainerPids    int    `yaml:"container_pids,omitempty"`    // max PIDs per container (default 256)
	ContainerNetwork string `yaml:"container_network,omitempty"` // "none" / "host" / network name (default "none")
	ContainerTmpMB         int    `yaml:"container_tmp_mb,omitempty"`         // tmpfs /tmp size in MB (default 100)
	ContainerDiskMB        int    `yaml:"container_disk_mb,omitempty"`        // workspace overlay size (0 = direct bind-mount)
	ContainerWorkspaceRoot string `yaml:"container_workspace_root,omitempty"` // host-side workspace root for DooD (Docker-outside-of-Docker)

	// gVisor standalone mode options (used when level == "gvisor").
	// Runs "runsc do" directly — no Docker, no image, ~50ms startup.
	GVisorBinary  string `yaml:"gvisor_binary,omitempty"`  // path to runsc (auto-detected if empty)
	GVisorNetwork bool   `yaml:"gvisor_network,omitempty"` // allow host network (default: isolated)
}

// PluginsConfig controls which plugins are enabled.
type PluginsConfig struct {
	AuditLog            bool `yaml:"audit_log"`
	TokenTracker        bool `yaml:"token_tracker"`
	MemoryConsolidation bool `yaml:"memory_consolidation"`
}

type AgentDefConfig struct {
	ID          string             `yaml:"id"`
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Model       string             `yaml:"model"`
	Routes      []types.AgentRoute `yaml:"routes"`
}

// Validate checks Config for common misconfigurations and returns an error
// describing all problems found. Call before buildDeps to fail fast.
func (c *Config) Validate() error {
	var errs []string
	add := func(msg string) { errs = append(errs, msg) }

	if c.MySQLDSN == "" {
		add("mysql_dsn is required")
	}
	if len(c.Providers) == 0 {
		add("at least one provider is required")
	}
	for i, p := range c.Providers {
		if p.APIKey == "" {
			add(fmt.Sprintf("providers[%d] (%s): api_key is required", i, p.Name))
		}
		if p.Model == "" {
			add(fmt.Sprintf("providers[%d] (%s): model is required", i, p.Name))
		}
	}
	if len(c.Agents) == 0 {
		add("at least one agent is required")
	}
	for i, a := range c.Agents {
		if a.ID == "" {
			add(fmt.Sprintf("agents[%d]: id is required", i))
		}
		if a.Model == "" {
			add(fmt.Sprintf("agents[%d] (%s): model is required", i, a.ID))
		}
	}
	if c.VectorDB.Addr != "" && c.Embedding.APIBase == "" {
		add("embedding.api_base is required when vector_db is configured")
	}
	if c.A2A.Enabled && c.A2A.Addr == "" {
		add("a2a.addr is required when a2a is enabled")
	}
	if c.Scheduler.DecisionMode == "llm" && len(c.Providers) < 1 {
		add("scheduler.decision_mode=llm requires at least one provider")
	}
	switch c.Sandbox.Level {
	case "", "none", "standard", "strict", "gvisor", "container":
		// valid
	default:
		add(fmt.Sprintf("sandbox.level %q is invalid; must be one of: none, standard, strict, gvisor, container", c.Sandbox.Level))
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// App holds all assembled components and manages their lifecycle.
type App struct {
	appName         string
	agentLoop       *AgentLoop
	registry        *AgentRegistry
	channelRegistry *channels.Registry
	a2aServer       *http.Server
	healthServer    *http.Server
	bus             types.MessageBus
	cronService     *scheduler.CronService
	heartbeatSvc    *scheduler.HeartbeatService
	plugins         []*plugin.Plugin
	mcpClients      []io.Closer
}

// Bootstrap assembles all components from config and returns a runnable App.
// Other tasks (storage, providers, tools, etc.) are injected via BootstrapDeps.
type BootstrapDeps struct {
	Bus                 types.MessageBus
	SessionService      session.Service
	LLM                 model.LLM                // primary LLM for agents
	SummaryLLM          model.LLM                // cheap LLM for compression
	Tools               []any                    // []tool.Tool — use any to avoid import cycle
	Toolsets            []tool.Toolset           // MCP toolsets and others
	Plugins             []*plugin.Plugin         // ADK-Go plugins (auditlog, tokentracker, etc.)
	Channels            map[string]types.Channel // named channels to register
	CronService         *scheduler.CronService
	HeartbeatSvc        *scheduler.HeartbeatService
	MCPClients          []io.Closer  // MCP client processes to close on shutdown
	APIHandler          http.Handler // optional HTTP API (marketplace, etc.)
	InstructionProvider func(adkagent.ReadonlyContext) (string, error)
}

func Bootstrap(ctx context.Context, cfg Config, deps BootstrapDeps) (*App, error) {
	appName := cfg.AppName
	if appName == "" {
		appName = "abot"
	}

	registry := NewAgentRegistry()

	// Create agents from config.
	for _, agentCfg := range cfg.Agents {
		if err := registerAgent(registry, agentCfg, deps, appName); err != nil {
			return nil, fmt.Errorf("register agent %q: %w", agentCfg.ID, err)
		}
	}

	// Compressor.
	ctxWindow := cfg.ContextWindow
	if ctxWindow <= 0 {
		ctxWindow = 128000
	}
	var comp *Compressor
	if deps.SummaryLLM != nil {
		comp = NewCompressor(deps.SummaryLLM, deps.SessionService, appName)
	}

	// Agent loop.
	loop := NewAgentLoop(deps.Bus, registry, deps.SessionService, comp, appName, ctxWindow, nil)

	// Channel registry.
	chanReg := channels.NewRegistry()
	for name, ch := range deps.Channels {
		chanReg.Register(name, ch, deps.Bus)
	}

	// A2A server.
	a2aServer, err := SetupA2AServer(cfg.A2A, registry)
	if err != nil {
		return nil, fmt.Errorf("setup a2a: %w", err)
	}

	// Health check server.
	var healthServer *http.Server
	if cfg.HealthAddr != "" {
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"ok","inbound_pending":%d,"outbound_pending":%d}`,
				deps.Bus.InboundSize(), deps.Bus.OutboundSize())
		})
		if deps.APIHandler != nil {
			mux.Handle("/api/", deps.APIHandler)
		}
		healthServer = &http.Server{Addr: cfg.HealthAddr, Handler: mux}
	}

	return &App{
		appName:         appName,
		agentLoop:       loop,
		registry:        registry,
		channelRegistry: chanReg,
		a2aServer:       a2aServer,
		healthServer:    healthServer,
		bus:             deps.Bus,
		cronService:     deps.CronService,
		heartbeatSvc:    deps.HeartbeatSvc,
		plugins:         deps.Plugins,
		mcpClients:      deps.MCPClients,
	}, nil
}

func registerAgent(registry *AgentRegistry, agentCfg AgentDefConfig, deps BootstrapDeps, appName string) error {
	// Cast []any tools to []tool.Tool.
	tools := make([]tool.Tool, 0, len(deps.Tools))
	for _, t := range deps.Tools {
		tt, ok := t.(tool.Tool)
		if !ok {
			return fmt.Errorf("tool %T does not implement tool.Tool", t)
		}
		tools = append(tools, tt)
	}

	// Create LLM agent via ADK-Go.
	agentConfig := llmagent.Config{
		Name:        agentCfg.Name,
		Description: agentCfg.Description,
		Model:       deps.LLM,
		Tools:       tools,
	}
	if len(deps.Toolsets) > 0 {
		agentConfig.Toolsets = deps.Toolsets
	}
	if deps.InstructionProvider != nil {
		agentConfig.InstructionProvider = deps.InstructionProvider
	}
	adkAgent, err := llmagent.New(agentConfig)
	if err != nil {
		return fmt.Errorf("create llmagent: %w", err)
	}

	// Create runner.
	runnerCfg := runner.Config{
		AppName:        appName,
		Agent:          adkAgent,
		SessionService: deps.SessionService,
	}
	if len(deps.Plugins) > 0 {
		runnerCfg.PluginConfig = runner.PluginConfig{
			Plugins: deps.Plugins,
		}
	}
	r, err := runner.New(runnerCfg)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}

	def := types.AgentDefinition{
		ID:          agentCfg.ID,
		Name:        agentCfg.Name,
		Description: agentCfg.Description,
		Model:       agentCfg.Model,
		Routes:      agentCfg.Routes,
	}

	registry.Register(&AgentEntry{
		ID:     agentCfg.ID,
		Agent:  adkAgent,
		Runner: r,
		Config: def,
	})
	return nil
}

// ProcessDirect delegates to the internal AgentLoop for synchronous processing.
func (a *App) ProcessDirect(ctx context.Context, msg types.InboundMessage) (string, error) {
	return a.agentLoop.ProcessDirect(ctx, msg)
}

// AppName returns the application name.
func (a *App) AppName() string {
	return a.appName
}

// ExportRegistry returns the underlying agent registry for external use (e.g., web console).
func (a *App) ExportRegistry() *AgentRegistry {
	return a.registry
}

// ExportLoop returns the underlying agent loop for external use (e.g., web console streaming).
func (a *App) ExportLoop() *AgentLoop {
	return a.agentLoop
}

// RunServices starts only cron + heartbeat (no channels, no bus event loop).
// Designed for agent CLI mode where the main loop is readline-driven.
func (a *App) RunServices(ctx context.Context) error {
	if a.cronService != nil {
		if err := a.cronService.Start(ctx); err != nil {
			return fmt.Errorf("start cron service: %w", err)
		}
		slog.Info("cron service started")
	}
	if a.heartbeatSvc != nil {
		if err := a.heartbeatSvc.Start(ctx); err != nil {
			return fmt.Errorf("start heartbeat service: %w", err)
		}
		slog.Info("heartbeat service started")
	}
	return nil
}

// Run starts all components and blocks until context is cancelled.
func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 3)

	// Start channel adapters.
	if a.channelRegistry != nil {
		if err := a.channelRegistry.StartAll(ctx); err != nil {
			return fmt.Errorf("start channels: %w", err)
		}
		slog.Info("channels started", "channels", a.channelRegistry.Names())
	}

	// Start A2A server if configured.
	if a.a2aServer != nil {
		go func() {
			slog.Info("A2A server listening", "addr", a.a2aServer.Addr)
			if err := a.a2aServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("a2a server: %w", err)
			}
		}()
	}

	// Start health check server if configured.
	if a.healthServer != nil {
		go func() {
			slog.Info("health server listening", "addr", a.healthServer.Addr)
			if err := a.healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("health server: %w", err)
			}
		}()
	}

	// Start scheduler services.
	if a.cronService != nil {
		if err := a.cronService.Start(ctx); err != nil {
			return fmt.Errorf("start cron service: %w", err)
		}
		slog.Info("cron service started")
	}
	if a.heartbeatSvc != nil {
		if err := a.heartbeatSvc.Start(ctx); err != nil {
			return fmt.Errorf("start heartbeat service: %w", err)
		}
		slog.Info("heartbeat service started")
	}

	// Start agent loop.
	go func() {
		slog.Info("agent loop started")
		if err := a.agentLoop.Run(ctx); err != nil {
			errCh <- fmt.Errorf("agent loop: %w", err)
		}
	}()

	// Wait for signal or error.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("received signal", "signal", sig)
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	return a.Shutdown(context.Background())
}

// Shutdown gracefully stops all components.
func (a *App) Shutdown(ctx context.Context) error {
	slog.Info("app: shutting down")

	if a.cronService != nil {
		if err := a.cronService.Stop(); err != nil {
			slog.Error("cron service stop error", "err", err)
		}
	}

	if a.heartbeatSvc != nil {
		if err := a.heartbeatSvc.Stop(); err != nil {
			slog.Error("heartbeat service stop error", "err", err)
		}
	}

	if a.channelRegistry != nil {
		if err := a.channelRegistry.StopAll(ctx); err != nil {
			slog.Error("channel registry stop error", "err", err)
		}
	}

	if a.a2aServer != nil {
		if err := a.a2aServer.Shutdown(ctx); err != nil {
			slog.Error("a2a server shutdown error", "err", err)
		}
	}

	if a.healthServer != nil {
		if err := a.healthServer.Shutdown(ctx); err != nil {
			slog.Error("health server shutdown error", "err", err)
		}
	}

	for _, c := range a.mcpClients {
		if err := c.Close(); err != nil {
			slog.Error("mcp client close error", "err", err)
		}
	}

	for _, p := range a.plugins {
		if err := p.Close(); err != nil {
			slog.Error("plugin close error", "plugin", p.Name(), "err", err)
		}
	}

	if a.bus != nil {
		if err := a.bus.Close(); err != nil {
			slog.Error("bus close error", "err", err)
		}
	}

	slog.Info("shutdown complete")
	return nil
}
