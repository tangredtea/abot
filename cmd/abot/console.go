package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"abot/pkg/agent"
	"abot/pkg/api/auth"
	"abot/pkg/api/console"
	"abot/pkg/bootstrap"
	mysqlstore "abot/pkg/storage/mysql"

	"google.golang.org/adk/tool"
)

func runConsole(cfg *agent.Config) error {
	result, err := bootstrap.BuildFullDeps(cfg)
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}
	result.Deps.Channels = nil

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := agent.Bootstrap(ctx, *cfg, *result.Deps)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	if err := app.RunServices(ctx); err != nil {
		return fmt.Errorf("run services: %w", err)
	}

	// JWT config.
	jwtSecret := cfg.Console.JWTSecret
	if jwtSecret == "" {
		slog.Warn("console.jwt_secret not configured; using ephemeral secret (set console.jwt_secret for production)")
		jwtSecret = "abot-ephemeral-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	jwtCfg := auth.JWTConfig{
		Secret: jwtSecret,
		Expiry: 24 * time.Hour,
	}

	// Initialize AgentManager with shared InstructionProvider.
	agentDefStore := mysqlstore.NewAgentDefinitionStore(result.DB)
	agentManager := console.NewAgentManager(
		app.ExportRegistry(),
		agentDefStore,
		result.Deps.SessionService,
		result.Deps.LLM,
		convertTools(result.Deps.Tools),
		result.Deps.Toolsets,
		cfg.AppName,
		result.Deps.InstructionProvider,
	)

	// Load agents from database (pass empty tenantID to load all)
	if err := agentManager.LoadAllAgents(ctx, ""); err != nil {
		slog.Warn("failed to load agents from database", "err", err)
	}

	consoleDeps := console.Deps{
		AgentLoop:          app.ExportLoop(),
		Registry:           app.ExportRegistry(),
		SessionService:     result.Deps.SessionService,
		AccountStore:       result.Stores.Account,
		AccTenantStore:     result.Stores.AccountTenant,
		ChatSessionStore:   result.Stores.ChatSession,
		TenantStore:        result.Stores.Tenant,
		WorkspaceStore:     result.Stores.Workspace,
		UserWorkspaceStore: result.Stores.UserWorkspace,
		JWTConfig:          jwtCfg,
		AppName:            cfg.AppName,
		DB:                 result.DB,
		EncryptionSecret:   jwtSecret,
		AllowedOrigins:     cfg.Console.AllowedOrigins,
		AgentManager:       agentManager,
	}

	mux := http.NewServeMux()

	// Mount console API.
	consoleHandler := console.Handler(consoleDeps)
	mux.Handle("/api/", consoleHandler)

	// Mount SPA static files.
	staticDir := cfg.Console.StaticDir
	if staticDir == "" {
		staticDir = "web/out"
	}
	mux.Handle("/", console.StaticHandler(staticDir))

	addr := cfg.Console.Addr
	if addr == "" {
		addr = ":3000"
	}

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Handle signals for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("console: shutting down")
		cancel()
		server.Shutdown(context.Background())
	}()

	slog.Info("console server started", "addr", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("console server: %w", err)
	}

	return app.Shutdown(context.Background())
}

// convertTools converts []any to []tool.Tool
func convertTools(tools []any) []tool.Tool {
	result := make([]tool.Tool, 0, len(tools))
	for _, t := range tools {
		if tt, ok := t.(tool.Tool); ok {
			result = append(result, tt)
		}
	}
	return result
}
