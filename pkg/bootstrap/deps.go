package bootstrap

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"google.golang.org/adk/plugin"
	"gorm.io/gorm"

	"abot/pkg/agent"
	"abot/pkg/api/marketplace"
	"abot/pkg/bus"
	"abot/pkg/mcp"
	"abot/pkg/plugins/memoryconsolidation"
	"abot/pkg/scheduler"
	"abot/pkg/skills"
	"abot/pkg/storage/seeder"
	"abot/pkg/storage/vectordb"
	"abot/pkg/storage/vectordb/qdrant"
	"abot/pkg/types"
	"abot/pkg/workspace"
)

// BuildCoreDeps builds minimal dependencies (no MySQL).
// Used by abot-agent.
func BuildCoreDeps(cfg *agent.Config) (*agent.BootstrapDeps, error) {
	ctx := context.Background()

	// Session service
	sessionSvc, err := NewSessionService(cfg)
	if err != nil {
		return nil, err
	}

	// LLM providers
	llm, summaryLLM, err := NewProviders(cfg)
	if err != nil {
		return nil, err
	}

	// Message bus
	bufSize := cfg.BusBufferSize
	if bufSize <= 0 {
		bufSize = bus.DefaultBufferSize
	}
	msgBus := bus.New(bufSize)

	// Lightweight tools
	toolsAsAny, err := BuildLightweightTools(cfg)
	if err != nil {
		return nil, err
	}
	slog.Info("lightweight tools registered", "count", len(toolsAsAny))

	// MCP tools
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

	// Convert MCP clients to []io.Closer for lifecycle management
	var mcpClosers []io.Closer
	for _, c := range mcpClients {
		mcpClosers = append(mcpClosers, c)
	}

	deps := &agent.BootstrapDeps{
		Bus:            msgBus,
		SessionService: sessionSvc,
		LLM:            llm,
		Tools:          toolsAsAny,
		MCPClients:     mcpClosers,
	}
	if summaryLLM != nil {
		deps.SummaryLLM = summaryLLM
	}

	return deps, nil
}

// BuildFullDeps builds complete dependencies (with MySQL).
// Used by abot-server and abot-web.
func BuildFullDeps(cfg *agent.Config) (*agent.BootstrapDeps, error) {
	ctx := context.Background()

	// Database
	db, err := NewDatabase(cfg)
	if err != nil {
		return nil, err
	}
	stores := NewStores(db)

	// Session service
	sessionSvc, err := NewSessionService(cfg)
	if err != nil {
		return nil, err
	}

	// LLM providers
	llm, summaryLLM, err := NewProviders(cfg)
	if err != nil {
		return nil, err
	}

	// Message bus
	bufSize := cfg.BusBufferSize
	if bufSize <= 0 {
		bufSize = bus.DefaultBufferSize
	}
	msgBus := bus.New(bufSize)

	// Full tools
	toolsAsAny, err := BuildFullTools(cfg, stores, msgBus)
	if err != nil {
		return nil, err
	}
	slog.Info("built-in tools registered", "count", len(toolsAsAny))

	// MCP tools
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

	// Skills loader + builtin registration
	cacheDir := cfg.SkillCacheDir
	if cacheDir == "" {
		cacheDir = "data/skill-cache"
	}
	objStoreDir := cfg.ObjectStore.Dir
	if objStoreDir == "" {
		objStoreDir = "data/objects"
	}
	// Note: objStore is already created in BuildFullTools, but we need it here too
	// This is a minor duplication but keeps the function self-contained
	skillsLoader := skills.NewSkillsLoader(
		stores.SkillRegistry, stores.TenantSkill, stores.Tenant,
		nil, // objStore will be set by BuildFullTools
		cacheDir,
	)
	builtinFS := os.DirFS("skills")
	if err := skills.RegisterBuiltins(ctx, stores.SkillRegistry, builtinFS); err != nil {
		return nil, fmt.Errorf("register builtins: %w", err)
	}
	slog.Info("builtin skills registered")

	// Warmup always_load skills (preload to cache)
	if err := skillsLoader.WarmupAlwaysLoad(ctx); err != nil {
		slog.Warn("skill warmup failed", "error", err)
	}

	// Cleanup stale cache (30 days)
	if err := skillsLoader.CleanupStaleCache(30 * 24 * 60 * 60); err != nil {
		slog.Warn("cache cleanup failed", "error", err)
	}

	// Vector store + embedder (optional)
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

	// Memory consolidation plugin (requires vector store + embedder)
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
			MemoryEventStore: stores.MemoryEvent,
		})
		if err != nil {
			return nil, fmt.Errorf("memory plugin: %w", err)
		}
		plugins = append(plugins, memPlugin)
		slog.Info("memory consolidation plugin enabled")
	}

	// Context builder
	ctxBuilder := workspace.NewContextBuilder(
		stores.Workspace, stores.UserWorkspace, skillsLoader, nil,
		vectorStore, embedder, builtinFS,
	)

	// Seed default data (wrapped in transaction for atomicity)
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return seeder.Seed(ctx, stores.Tenant.WithTx(tx), stores.Workspace.WithTx(tx), stores.UserWorkspace.WithTx(tx))
	}); err != nil {
		return nil, fmt.Errorf("seed data: %w", err)
	}

	// Channels
	chans, err := NewChannels(cfg, msgBus)
	if err != nil {
		return nil, err
	}

	// Cron scheduler
	cronSvc := scheduler.New(stores.Scheduler, msgBus, slog.Default())
	slog.Info("cron scheduler created")

	// Heartbeat (optional)
	var heartbeatSvc *scheduler.HeartbeatService
	if cfg.Scheduler.HeartbeatInterval != "" {
		interval, err := time.ParseDuration(cfg.Scheduler.HeartbeatInterval)
		if err != nil {
			return nil, fmt.Errorf("parse heartbeat interval: %w", err)
		}
		hbCfg := scheduler.HeartbeatConfig{
			Bus:            msgBus,
			WorkspaceStore: stores.Workspace,
			Tenants:        stores.Tenant,
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

	// Convert MCP clients to []io.Closer for lifecycle management
	var mcpClosers []io.Closer
	for _, c := range mcpClients {
		mcpClosers = append(mcpClosers, c)
	}

	// Marketplace API handler
	marketplaceHandler := marketplace.Handler(marketplace.Deps{
		Skills:    stores.SkillRegistry,
		Tenants:   stores.TenantSkill,
		Proposals: stores.Proposal,
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
