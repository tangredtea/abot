package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"gopkg.in/yaml.v3"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"abot/pkg/agent"
	"abot/pkg/api/marketplace"
	"abot/pkg/bus"
	"abot/pkg/channels/discord"
	"abot/pkg/channels/feishu"
	"abot/pkg/channels/telegram"
	"abot/pkg/channels/wecom"
	"abot/pkg/mcp"
	"abot/pkg/plugins/memoryconsolidation"
	"abot/pkg/providers"
	"abot/pkg/providers/fallback"
	"abot/pkg/scheduler"
	abotsession "abot/pkg/session"
	"abot/pkg/skills"
	mysqlstore "abot/pkg/storage/mysql"
	"abot/pkg/storage/objectstore"
	"abot/pkg/storage/seeder"
	"abot/pkg/storage/vectordb"
	"abot/pkg/storage/vectordb/qdrant"
	"abot/pkg/tools"
	"abot/pkg/types"
	"abot/pkg/workspace"

	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/session"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Detect subcommand before flag.Parse().
	subcmd := ""
	if len(os.Args) > 1 && (os.Args[1] == "agent" || os.Args[1] == "console") {
		subcmd = os.Args[1]
		os.Args = append(os.Args[:1], os.Args[2:]...) // strip subcommand for flag parsing
	}

	configPath := flag.String("config", "config.yaml", "path to config file")
	tenantFlag := flag.String("tenant", types.DefaultTenantID, "tenant ID for CLI/agent mode")
	userFlag := flag.String("user", types.DefaultUserID, "user ID for CLI/agent mode")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	cfg.CLITenantID = *tenantFlag
	cfg.CLIUserID = *userFlag

	if subcmd == "agent" {
		return runAgent(cfg)
	}
	if subcmd == "console" {
		return runConsole(cfg)
	}

	return runGateway(cfg)
}

func runGateway(cfg *agent.Config) error {
	deps, err := buildDeps(cfg)
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}

	ctx := context.Background()
	app, err := agent.Bootstrap(ctx, *cfg, *deps)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	slog.Info("starting gateway", "app", cfg.AppName)
	return app.Run(ctx)
}

func loadConfig(path string) (*agent.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg agent.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// storeBundle groups all MySQL-backed stores for easy passing.
type storeBundle struct {
	tenant        *mysqlstore.TenantStore
	workspace     *mysqlstore.WorkspaceDocStore
	userWorkspace *mysqlstore.UserWorkspaceDocStore
	skillRegistry *mysqlstore.SkillRegistryStoreMySQL
	tenantSkill   *mysqlstore.TenantSkillStoreMySQL
	scheduler     *mysqlstore.SchedulerStoreMySQL
	proposal      *mysqlstore.SkillProposalStoreMySQL
	memoryEvent   *mysqlstore.MemoryEventStoreMySQL
	account       *mysqlstore.AccountStore
	accountTenant *mysqlstore.AccountTenantStore
	chatSession   *mysqlstore.ChatSessionStore
}

func newDatabase(cfg *agent.Config) (*gorm.DB, error) {
	if cfg.MySQLDSN == "" {
		return nil, fmt.Errorf("mysql_dsn is required")
	}
	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	if err := mysqlstore.AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	slog.Info("mysql connected, tables migrated")
	return db, nil
}

func newStores(db *gorm.DB) *storeBundle {
	return &storeBundle{
		tenant:        mysqlstore.NewTenantStore(db),
		workspace:     mysqlstore.NewWorkspaceDocStore(db),
		userWorkspace: mysqlstore.NewUserWorkspaceDocStore(db),
		skillRegistry: mysqlstore.NewSkillRegistryStore(db),
		tenantSkill:   mysqlstore.NewTenantSkillStore(db),
		scheduler:     mysqlstore.NewSchedulerStore(db),
		proposal:      mysqlstore.NewSkillProposalStore(db),
		memoryEvent:   mysqlstore.NewMemoryEventStore(db),
		account:       mysqlstore.NewAccountStore(db),
		accountTenant: mysqlstore.NewAccountTenantStore(db),
		chatSession:   mysqlstore.NewChatSessionStore(db),
	}
}

func newSessionService(cfg *agent.Config) (session.Service, error) {
	switch cfg.Session.Type {
	case "jsonl":
		dir := cfg.Session.Dir
		if dir == "" {
			dir = "data/sessions"
		}
		svc, err := abotsession.NewJSONLService(dir)
		if err != nil {
			return nil, fmt.Errorf("jsonl session: %w", err)
		}
		slog.Info("session service configured", "type", "jsonl", "dir", dir)
		return svc, nil
	default:
		slog.Info("session service configured", "type", "in-memory")
		return session.InMemoryService(), nil
	}
}

func newProviders(cfg *agent.Config) (primary model.LLM, summary model.LLM, err error) {
	if len(cfg.Providers) == 0 {
		return nil, nil, fmt.Errorf("at least one provider is required")
	}

	// Create all LLM instances.
	entries := make([]fallback.LLMEntry, 0, len(cfg.Providers))
	for i, p := range cfg.Providers {
		llm, modelID, err := providers.CreateModelFromConfig(providers.ModelFactoryConfig{
			Model:         p.Model,
			APIKey:        p.APIKey,
			APIBase:       p.APIBase,
			PromptCaching: p.PromptCaching,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("create LLM [%d] %s: %w", i, p.Name, err)
		}
		entries = append(entries, fallback.LLMEntry{
			Provider: p.Name,
			Model:    modelID,
			LLM:      llm,
		})
	}

	// Wrap in FallbackLLM for automatic failover.
	primary = fallback.NewFallbackLLM(entries, nil)
	slog.Info("providers configured", "count", len(entries))

	// Use last provider as summary LLM (typically the cheapest).
	if len(entries) > 1 {
		summary = entries[len(entries)-1].LLM
	}

	return primary, summary, nil
}

func newChannels(cfg *agent.Config, msgBus types.MessageBus) (map[string]types.Channel, error) {
	chans := make(map[string]types.Channel)
	if cfg.WeCom.Token != "" {
		wc, err := wecom.NewWeComChannel(wecom.WeComConfig{
			Token:          cfg.WeCom.Token,
			EncodingAESKey: cfg.WeCom.EncodingAESKey,
			WebhookURL:     cfg.WeCom.WebhookURL,
			WebhookHost:    cfg.WeCom.WebhookHost,
			WebhookPort:    cfg.WeCom.WebhookPort,
			WebhookPath:    cfg.WeCom.WebhookPath,
			ReplyTimeout:   cfg.WeCom.ReplyTimeout,
			AllowFrom:      cfg.WeCom.AllowFrom,
			TenantID:       cfg.WeCom.TenantID,
			UserID:         cfg.WeCom.UserID,
		}, msgBus)
		if err != nil {
			return nil, fmt.Errorf("wecom channel: %w", err)
		}
		chans[wecom.ChannelName] = wc
		slog.Info("wecom channel configured")
	}
	if cfg.Telegram.Token != "" {
		tc, err := telegram.NewTelegramChannel(telegram.TelegramConfig{
			Token:       cfg.Telegram.Token,
			AllowFrom:   cfg.Telegram.AllowFrom,
			TenantID:    cfg.Telegram.TenantID,
			PollTimeout: cfg.Telegram.PollTimeout,
		}, msgBus)
		if err != nil {
			return nil, fmt.Errorf("telegram channel: %w", err)
		}
		chans[telegram.ChannelName] = tc
		slog.Info("telegram channel configured")
	}
	if cfg.Discord.Token != "" {
		dc, err := discord.NewDiscordChannel(discord.DiscordConfig{
			Token:     cfg.Discord.Token,
			AllowFrom: cfg.Discord.AllowFrom,
			TenantID:  cfg.Discord.TenantID,
			GuildID:   cfg.Discord.GuildID,
		}, msgBus)
		if err != nil {
			return nil, fmt.Errorf("discord channel: %w", err)
		}
		chans[discord.ChannelName] = dc
		slog.Info("discord channel configured")
	}
	if cfg.Feishu.AppID != "" {
		fc, err := feishu.NewFeishuChannel(feishu.FeishuConfig{
			AppID:             cfg.Feishu.AppID,
			AppSecret:         cfg.Feishu.AppSecret,
			VerificationToken: cfg.Feishu.VerificationToken,
			EncryptKey:        cfg.Feishu.EncryptKey,
			WebhookHost:       cfg.Feishu.WebhookHost,
			WebhookPort:       cfg.Feishu.WebhookPort,
			WebhookPath:       cfg.Feishu.WebhookPath,
			AllowFrom:         cfg.Feishu.AllowFrom,
			TenantID:          cfg.Feishu.TenantID,
		}, msgBus)
		if err != nil {
			return nil, fmt.Errorf("feishu channel: %w", err)
		}
		chans[feishu.ChannelName] = fc
		slog.Info("feishu channel configured")
	}
	return chans, nil
}

func buildDeps(cfg *agent.Config) (*agent.BootstrapDeps, error) {
	ctx := context.Background()

	// Database.
	db, err := newDatabase(cfg)
	if err != nil {
		return nil, err
	}
	stores := newStores(db)

	// Session service.
	sessionSvc, err := newSessionService(cfg)
	if err != nil {
		return nil, err
	}

	// LLM providers.
	llm, summaryLLM, err := newProviders(cfg)
	if err != nil {
		return nil, err
	}

	// Message bus.
	bufSize := cfg.BusBufferSize
	if bufSize <= 0 {
		bufSize = bus.DefaultBufferSize
	}
	msgBus := bus.New(bufSize)

	// Object store.
	objStoreDir := cfg.ObjectStore.Dir
	if objStoreDir == "" {
		objStoreDir = "data/objects"
	}
	objStore := objectstore.NewLocalStore(objStoreDir)

	// Built-in tools.
	toolsDeps := &tools.Deps{
		Bus:                 msgBus,
		WorkspaceStore:      stores.workspace,
		UserWorkspaceStore:  stores.userWorkspace,
		SkillRegistryStore:  stores.skillRegistry,
		TenantSkillStore:    stores.tenantSkill,
		SchedulerStore:      stores.scheduler,
		ObjectStore:         objStore,
		ProposalStore:       stores.proposal,
		WorkspaceDir:        "workspace",
		DenyPatterns:        append([]string{".env", "*.key", "*.pem"}, cfg.Sandbox.ExtraDenyPatterns...),
		RestrictToWorkspace: cfg.Sandbox.RestrictToWorkspace,
		AllowedPaths:        cfg.Sandbox.AllowedPaths,
	}

	// Wire ExecLimits from sandbox config (fixes bug where these were never applied).
	if cfg.Sandbox.ExecMemoryMB > 0 || cfg.Sandbox.ExecCPUSeconds > 0 ||
		cfg.Sandbox.ExecFileSizeMB > 0 || cfg.Sandbox.ExecNProc > 0 {
		toolsDeps.ExecLimits = &tools.ExecLimits{
			MemoryMB:   cfg.Sandbox.ExecMemoryMB,
			CPUSeconds: cfg.Sandbox.ExecCPUSeconds,
			FileSizeMB: cfg.Sandbox.ExecFileSizeMB,
			NProc:      cfg.Sandbox.ExecNProc,
		}
	}

	// Wire SandboxOpts for Landlock kernel-level filesystem sandbox.
	if cfg.Sandbox.Level != "" && cfg.Sandbox.Level != "none" {
		toolsDeps.SandboxOpts = &tools.SandboxOpts{
			Level:        tools.SandboxLevel(cfg.Sandbox.Level),
			HelperBinary: cfg.Sandbox.SandboxBinary,
		}
	}

	// Wire per-tenant rate limiter.
	if cfg.Sandbox.RateLimit > 0 {
		burst := cfg.Sandbox.RateBurst
		if burst <= 0 {
			burst = 10
		}
		toolsDeps.RateLimiter = tools.NewTenantRateLimiter(cfg.Sandbox.RateLimit, burst)
	}

	// Wire tenant store for per-tenant tool permission checks.
	toolsDeps.TenantStore = stores.tenant

	builtTools, err := tools.BuildAllTools(toolsDeps)
	if err != nil {
		return nil, fmt.Errorf("build tools: %w", err)
	}
	toolsAsAny := make([]any, len(builtTools))
	for i, t := range builtTools {
		toolsAsAny[i] = t
	}
	slog.Info("built-in tools registered", "count", len(builtTools))

	// MCP tools.
	var mcpClients []*mcp.Client
	if len(cfg.MCP) > 0 {
		servers := make(map[string]mcp.ServerConfig, len(cfg.MCP))
		for name, sc := range cfg.MCP {
			servers[name] = mcp.ServerConfig{
				Command:     sc.Command,
				Args:        sc.Args,
				Env:         sc.Env,
				URL:         sc.URL,
				Headers:     sc.Headers,
				ToolTimeout: sc.ToolTimeout,
			}
		}
		clients, mcpTools, err := mcp.ConnectAll(ctx, servers)
		if err != nil {
			return nil, fmt.Errorf("mcp connect: %w", err)
		}
		mcpClients = clients
		for _, t := range mcpTools {
			toolsAsAny = append(toolsAsAny, t)
		}
		slog.Info("MCP tools registered", "tools", len(mcpTools), "servers", len(clients))
	}

	// Skills loader + builtin registration.
	cacheDir := cfg.SkillCacheDir
	if cacheDir == "" {
		cacheDir = "data/skill-cache"
	}
	skillsLoader := skills.NewSkillsLoader(
		stores.skillRegistry, stores.tenantSkill, stores.tenant, objStore, cacheDir,
	)
	builtinFS := os.DirFS("skills")
	if err := skills.RegisterBuiltins(ctx, stores.skillRegistry, builtinFS); err != nil {
		return nil, fmt.Errorf("register builtins: %w", err)
	}
	slog.Info("builtin skills registered")

	// Warmup always_load skills (preload to cache).
	if err := skillsLoader.WarmupAlwaysLoad(ctx); err != nil {
		slog.Warn("skill warmup failed", "error", err)
	}

	// Cleanup stale cache (30 days).
	if err := skillsLoader.CleanupStaleCache(30 * 24 * 60 * 60); err != nil {
		slog.Warn("cache cleanup failed", "error", err)
	}

	// Vector store + embedder (optional).
	var vectorStore types.VectorStore
	var embedder types.Embedder
	if cfg.VectorDB.Addr != "" {
		embDim := cfg.Embedding.Dimension
		if embDim <= 0 {
			embDim = 768
		}
		vs, err := qdrant.New(qdrant.Config{
			Addr:      cfg.VectorDB.Addr,
			Dimension: embDim,
		})
		if err != nil {
			return nil, fmt.Errorf("qdrant connect: %w", err)
		}
		vectorStore = vs
		if cfg.Embedding.APIBase != "" && cfg.Embedding.APIKey != "" {
			embedder = vectordb.NewOpenAIEmbedder(vectordb.OpenAIEmbedderConfig{
				BaseURL:   cfg.Embedding.APIBase,
				APIKey:    cfg.Embedding.APIKey,
				Model:     cfg.Embedding.Model,
				Dimension: embDim,
			})
		}
		slog.Info("vector store + embedder initialized")
	}

	// Inject into tools deps for save_memory / search_memory.
	toolsDeps.VectorStore = vectorStore
	toolsDeps.Embedder = embedder
	toolsDeps.MemoryEventStore = stores.memoryEvent
	toolsDeps.SkillsLoader = skillsLoader

	// Memory consolidation plugin (requires vector store + embedder).
	var plugins []*plugin.Plugin
	if vectorStore != nil && embedder != nil {
		consolidationLLM := summaryLLM
		if consolidationLLM == nil {
			consolidationLLM = llm
		}
		memPlugin, err := memoryconsolidation.New(memoryconsolidation.Config{
			ConsolidationLLM: consolidationLLM,
			VectorStore:      vectorStore,
			Embedder:         embedder,
			MemoryEventStore: stores.memoryEvent,
		})
		if err != nil {
			return nil, fmt.Errorf("memory plugin: %w", err)
		}
		plugins = append(plugins, memPlugin)
		slog.Info("memory consolidation plugin enabled")
	}

	// Context builder.
	ctxBuilder := workspace.NewContextBuilder(
		stores.workspace, stores.userWorkspace, skillsLoader, nil,
		vectorStore, embedder, builtinFS,
	)

	// Seed default data (wrapped in transaction for atomicity).
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return seeder.Seed(ctx, stores.tenant.WithTx(tx), stores.workspace.WithTx(tx), stores.userWorkspace.WithTx(tx))
	}); err != nil {
		return nil, fmt.Errorf("seed data: %w", err)
	}

	// Channels.
	chans, err := newChannels(cfg, msgBus)
	if err != nil {
		return nil, err
	}

	// Cron scheduler.
	cronSvc := scheduler.New(stores.scheduler, msgBus, slog.Default())
	toolsDeps.CronScheduler = cronSvc
	slog.Info("cron scheduler created")

	// Heartbeat (optional).
	var heartbeatSvc *scheduler.HeartbeatService
	if cfg.Scheduler.HeartbeatInterval != "" {
		interval, err := time.ParseDuration(cfg.Scheduler.HeartbeatInterval)
		if err != nil {
			return nil, fmt.Errorf("parse heartbeat interval: %w", err)
		}
		hbCfg := scheduler.HeartbeatConfig{
			Bus:            msgBus,
			WorkspaceStore: stores.workspace,
			Tenants:        stores.tenant,
			Interval:       interval,
			Channel:        cfg.Scheduler.HeartbeatChannel,
			DecisionMode:   cfg.Scheduler.DecisionMode,
		}
		if cfg.Scheduler.DecisionMode == "llm" {
			if summaryLLM != nil {
				hbCfg.LLM = summaryLLM
			} else {
				hbCfg.LLM = llm
			}
		}
		heartbeatSvc = scheduler.NewHeartbeat(hbCfg)
		slog.Info("heartbeat configured", "interval", interval, "mode", cfg.Scheduler.DecisionMode)
	}

	// Convert MCP clients to []io.Closer for lifecycle management.
	var mcpClosers []io.Closer
	for _, c := range mcpClients {
		mcpClosers = append(mcpClosers, c)
	}

	// Marketplace API handler.
	marketplaceHandler := marketplace.Handler(marketplace.Deps{
		Skills:    stores.skillRegistry,
		Tenants:   stores.tenantSkill,
		Proposals: stores.proposal,
	})

	deps := &agent.BootstrapDeps{
		Bus:                 msgBus,
		SessionService:      sessionSvc,
		LLM:                 llm,
		Tools:               toolsAsAny,
		Plugins:             plugins,
		InstructionProvider: ctxBuilder.InstructionProvider(),
		Channels:            chans,
		CronService:         cronSvc,
		HeartbeatSvc:        heartbeatSvc,
		MCPClients:          mcpClosers,
		APIHandler:          marketplaceHandler,
	}
	if summaryLLM != nil {
		deps.SummaryLLM = summaryLLM
	}
	return deps, nil
}
