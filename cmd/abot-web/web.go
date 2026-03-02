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

	"abot/pkg/agent"
	"abot/pkg/api/auth"
	"abot/pkg/api/console"
	"abot/pkg/bootstrap"
	mysqlstore "abot/pkg/storage/mysql"

	"google.golang.org/adk/tool"
)

func runWebConsole(ctx context.Context, cancel context.CancelFunc, cfg *agent.Config, app *agent.App, deps *agent.BootstrapDeps) error {
	// 1. Get database connection
	db, err := bootstrap.NewDatabase(cfg)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}

	// 2. Create stores
	stores := bootstrap.NewStores(db)

	// 3. Session service
	sessionSvc, err := bootstrap.NewSessionService(cfg)
	if err != nil {
		return fmt.Errorf("session service: %w", err)
	}

	// 4. JWT configuration
	jwtSecret := cfg.Console.JWTSecret
	if jwtSecret == "" {
		jwtSecret = "abot-default-secret-change-me"
		slog.Warn("using default JWT secret, set console.jwt_secret in config for production")
	}
	jwtCfg := auth.JWTConfig{
		Secret: jwtSecret,
		Expiry: 24 * time.Hour,
	}

	// 5. Initialize AgentManager
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

	// 6. Load agents from database
	if err := agentManager.LoadAllAgents(ctx, ""); err != nil {
		slog.Warn("failed to load agents from database", "err", err)
	}

	// 7. Build Console dependencies
	consoleDeps := console.Deps{
		AgentLoop:          app.ExportLoop(),
		Registry:           app.ExportRegistry(),
		SessionService:     sessionSvc,
		AccountStore:       stores.Account,
		AccTenantStore:     stores.AccountTenant,
		ChatSessionStore:   stores.ChatSession,
		TenantStore:        stores.Tenant,
		WorkspaceStore:     stores.Workspace,
		UserWorkspaceStore: stores.UserWorkspace,
		JWTConfig:          jwtCfg,
		AppName:            cfg.AppName,
		DB:                 db,
		EncryptionSecret:   jwtSecret,
		AllowedOrigins:     cfg.Console.AllowedOrigins,
		AgentManager:       agentManager,
	}

	// 8. Create HTTP router
	mux := http.NewServeMux()

	// Mount API routes
	consoleHandler := console.Handler(consoleDeps)
	mux.Handle("/api/", consoleHandler)

	// Mount static files (Web UI)
	staticDir := cfg.Console.StaticDir
	if staticDir == "" {
		staticDir = "web/out"
	}
	mux.Handle("/", console.StaticHandler(staticDir))

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 9. Configure server address
	addr := cfg.Console.Addr
	if addr == "" {
		addr = ":3000"
	}

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// 10. Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("shutting down web console")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "err", err)
		}
	}()

	// 11. Start server
	slog.Info("web console started", "addr", addr, "static", staticDir)
	fmt.Printf("\n🚀 Web Console available at http://localhost%s\n\n", addr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("web console: %w", err)
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
