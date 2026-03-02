package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"abot/pkg/agent"
	"abot/pkg/api/auth"
	"abot/pkg/api/console"
	mysqlstore "abot/pkg/storage/mysql"

	"google.golang.org/adk/tool"
)

func runConsole(cfg *agent.Config) error {
	// Initialize database and stores separately so we can use them for console deps.
	if cfg.MySQLDSN == "" {
		return fmt.Errorf("mysql_dsn is required")
	}
	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	if err := mysqlstore.AutoMigrate(db); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	stores := newStores(db)

	// Build deps using the shared buildDeps function.
	deps, err := buildDeps(cfg)
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}
	deps.Channels = nil // Console mode doesn't use gateway channels.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := agent.Bootstrap(ctx, *cfg, *deps)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	if err := app.RunServices(ctx); err != nil {
		return fmt.Errorf("run services: %w", err)
	}

	// JWT config.
	jwtSecret := cfg.Console.JWTSecret
	if jwtSecret == "" {
		jwtSecret = "abot-default-secret-change-me"
		slog.Warn("console: using default JWT secret, set console.jwt_secret in config for production")
	}
	jwtCfg := auth.JWTConfig{
		Secret: jwtSecret,
		Expiry: 24 * time.Hour,
	}

	// Session service (shared with buildDeps — but we need a handle here too).
	sessionSvc, err := newSessionService(cfg)
	if err != nil {
		return fmt.Errorf("session service: %w", err)
	}

	// Initialize AgentManager with shared InstructionProvider
	agentDefStore := mysqlstore.NewAgentDefinitionStore(db)
	agentManager := console.NewAgentManager(
		app.ExportRegistry(),
		agentDefStore,
		sessionSvc,
		deps.LLM,
		convertTools(deps.Tools),
		deps.Toolsets,
		cfg.AppName,
		deps.InstructionProvider,
	)

	// Load agents from database (pass empty tenantID to load all)
	if err := agentManager.LoadAllAgents(ctx, ""); err != nil {
		slog.Warn("failed to load agents from database", "err", err)
	}

	consoleDeps := console.Deps{
		AgentLoop:          app.ExportLoop(),
		Registry:           app.ExportRegistry(),
		SessionService:     sessionSvc,
		AccountStore:       stores.account,
		AccTenantStore:     stores.accountTenant,
		ChatSessionStore:   stores.chatSession,
		TenantStore:        stores.tenant,
		WorkspaceStore:     stores.workspace,
		UserWorkspaceStore: stores.userWorkspace,
		JWTConfig:          jwtCfg,
		AppName:            cfg.AppName,
		DB:                 db,
		EncryptionSecret:   jwtSecret, // Reuse JWT secret for encryption.
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
