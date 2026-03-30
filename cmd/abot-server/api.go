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
	"gorm.io/gorm"
)

func runAPIServer(ctx context.Context, cancel context.CancelFunc, cfg *agent.Config, app *agent.App, deps *agent.BootstrapDeps, db *gorm.DB, stores *bootstrap.StoreBundle) error {
	// JWT config — never fall back to a default secret
	jwtSecret := cfg.Console.JWTSecret
	if jwtSecret == "" {
		return fmt.Errorf("console.jwt_secret is required; refusing to start with an empty secret")
	}
	jwtCfg := auth.JWTConfig{
		Secret: jwtSecret,
		Expiry: 24 * time.Hour,
	}

	// Use a dedicated encryption key for API-key-at-rest encryption.
	// Falls back to jwt_secret only if encryption_secret is not configured.
	encryptionSecret := cfg.Console.EncryptionSecret
	if encryptionSecret == "" {
		encryptionSecret = jwtSecret
		slog.Warn("api-server: console.encryption_secret not set, falling back to jwt_secret (set a separate key for production)")
	}

	// Initialize AgentManager
	agentDefStore := mysqlstore.NewAgentDefinitionStore(db)
	agentManager := console.NewAgentManager(
		app.ExportRegistry(),
		agentDefStore,
		deps.SessionService,
		deps.LLM,
		convertTools(deps.Tools),
		deps.Toolsets,
		cfg.AppName,
		deps.InstructionProvider,
	)

	// Load agents from database
	if err := agentManager.LoadAllAgents(ctx, ""); err != nil {
		slog.Warn("failed to load agents from database", "err", err)
	}

	// Build console dependencies
	consoleDeps := console.Deps{
		AgentLoop:          app.ExportLoop(),
		Registry:           app.ExportRegistry(),
		SessionService:     deps.SessionService,
		AccountStore:       stores.Account,
		AccTenantStore:     stores.AccountTenant,
		ChatSessionStore:   stores.ChatSession,
		TenantStore:        stores.Tenant,
		WorkspaceStore:     stores.Workspace,
		UserWorkspaceStore: stores.UserWorkspace,
		JWTConfig:          jwtCfg,
		AppName:            cfg.AppName,
		DB:                 db,
		EncryptionSecret:   encryptionSecret,
		AllowedOrigins:     cfg.Console.AllowedOrigins,
		AgentManager:       agentManager,
	}

	mux := http.NewServeMux()

	// Mount console API only (no static files)
	consoleHandler := console.Handler(consoleDeps)
	mux.Handle("/api/", consoleHandler)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := cfg.Console.Addr
	if addr == "" {
		addr = ":3000"
	}

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("api-server: shutting down")
		cancel()
		server.Shutdown(context.Background())
	}()

	slog.Info("api server started", "addr", addr, "mode", "API-only")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api server: %w", err)
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
