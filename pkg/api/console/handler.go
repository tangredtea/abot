// Package console provides HTTP handlers for the ABot web console.
package console

import (
	"log/slog"
	"net/http"

	"gorm.io/gorm"

	"abot/pkg/agent"
	"abot/pkg/api/auth"
	apierrors "abot/pkg/api/errors"
	"abot/pkg/api/middleware"
	"abot/pkg/api/response"
	"abot/pkg/types"

	"google.golang.org/adk/session"
)

// Deps holds dependencies for console handlers.
type Deps struct {
	AgentLoop          *agent.AgentLoop
	Registry           *agent.AgentRegistry
	SessionService     session.Service
	AccountStore       types.AccountStore
	AccTenantStore     types.AccountTenantStore
	ChatSessionStore   types.ChatSessionStore
	TenantStore        types.TenantStore
	WorkspaceStore     types.WorkspaceStore
	UserWorkspaceStore types.UserWorkspaceStore
	JWTConfig          auth.JWTConfig
	AppName            string
	DB                 *gorm.DB
	EncryptionSecret   string
	AllowedOrigins     []string
	AgentManager       *AgentManager // Dynamic agent management
}

// Handler returns the root http.Handler for the console, mounting all sub-routes.
func Handler(deps Deps) http.Handler {
	mux := http.NewServeMux()

	// Create middleware stack
	authMiddleware := auth.AuthMiddleware(deps.JWTConfig)
	corsMiddleware := middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: deps.AllowedOrigins,
	})
	securityMiddleware := middleware.SecurityHeaders(middleware.DefaultSecurityConfig())
	
	// Create logger
	logger := slog.Default()

	// Public auth endpoints with full middleware stack
	authDeps := auth.Deps{
		AccountStore:   deps.AccountStore,
		AccTenantStore: deps.AccTenantStore,
		TenantStore:    deps.TenantStore,
		WorkspaceStore: deps.WorkspaceStore,
		JWTConfig:      deps.JWTConfig,
		DB:             deps.DB,
	}
	authHandler := auth.Handler(authDeps)
	
	// Apply middleware: Recovery → RequestID → Logger → Security → CORS → Handler
	mux.Handle("/api/v1/auth/", 
		middleware.Recovery(
			middleware.RequestID(
				middleware.Logger(logger)(
					securityMiddleware(
						corsMiddleware(authHandler),
					),
				),
			),
		),
	)

	// Authenticated endpoints.
	protected := http.NewServeMux()

	// Sessions.
	sessH := &sessionsHandler{deps: deps}
	protected.HandleFunc("GET /api/v1/sessions", sessH.list)
	protected.HandleFunc("POST /api/v1/sessions", sessH.create)
	protected.HandleFunc("GET /api/v1/sessions/{id}", sessH.get)
	protected.HandleFunc("PATCH /api/v1/sessions/{id}", sessH.update)
	protected.HandleFunc("DELETE /api/v1/sessions/{id}", sessH.delete)

	// Agents.
	agentsH := &agentsHandler{deps: deps}
	protected.HandleFunc("GET /api/v1/agents", agentsH.list)
	protected.HandleFunc("POST /api/v1/agents", agentsH.create)
	protected.HandleFunc("GET /api/v1/agents/{id}", agentsH.get)
	protected.HandleFunc("PUT /api/v1/agents/{id}", agentsH.update)
	protected.HandleFunc("DELETE /api/v1/agents/{id}", agentsH.deleteAgent)
	protected.HandleFunc("GET /api/v1/agents/{id}/config", agentsH.getConfig)
	protected.HandleFunc("PUT /api/v1/agents/{id}/config", agentsH.updateConfig)
	protected.HandleFunc("GET /api/v1/agents/{id}/channels", agentsH.getChannels)
	protected.HandleFunc("PUT /api/v1/agents/{id}/channels", agentsH.updateChannels)

	// Workspace docs.
	wsDocsH := &workspaceDocsHandler{deps: deps}
	protected.HandleFunc("GET /api/v1/workspace/docs", wsDocsH.handleListWorkspaceDocs)
	protected.HandleFunc("GET /api/v1/workspace/docs/{doc_type}", wsDocsH.handleGetWorkspaceDoc)
	protected.HandleFunc("PUT /api/v1/workspace/docs/{doc_type}", wsDocsH.handleUpdateWorkspaceDoc)

	// Provider settings.
	provH := &providersHandler{deps: deps}
	protected.HandleFunc("GET /api/v1/settings/providers", provH.get)
	protected.HandleFunc("PUT /api/v1/settings/providers", provH.update)

	mux.Handle("/api/v1/", 
		middleware.Recovery(
			middleware.RequestID(
				middleware.Logger(logger)(
					securityMiddleware(
						corsMiddleware(
							authMiddleware(protected),
						),
					),
				),
			),
		),
	)
	
	// /me endpoint (authenticated)
	mux.Handle("/api/v1/auth/me", 
		middleware.Recovery(
			middleware.RequestID(
				middleware.Logger(logger)(
					securityMiddleware(
						corsMiddleware(
							authMiddleware(http.HandlerFunc(auth.MeHandler(authDeps))),
						),
					),
				),
			),
		),
	)

	// WebSocket chat — auth via query param.
	wsH := &wsHandler{deps: deps}
	mux.HandleFunc("/api/v1/chat/ws", wsH.handle)

	return mux
}

// writeJSON is deprecated, use response.JSON instead.
// Kept for backward compatibility with existing handlers.
func writeJSON(w http.ResponseWriter, status int, v any) {
	response.JSON(w, status, v)
}

// writeError is deprecated, use apierrors.HandleError instead.
// Kept for backward compatibility with existing handlers.
func writeError(w http.ResponseWriter, status int, msg string) {
	apierrors.HandleError(w, apierrors.New("ERROR", msg, status, nil))
}
