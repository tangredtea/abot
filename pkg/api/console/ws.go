package console

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"abot/pkg/agent"
	"abot/pkg/api/auth"
	"abot/pkg/types"
)

type wsHandler struct {
	deps     Deps
	writeMu  sync.Mutex
	upgrader websocket.Upgrader
}

func makeOriginChecker(allowed []string) func(*http.Request) bool {
	if len(allowed) == 0 {
		return func(r *http.Request) bool { return true }
	}
	allowedSet := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		allowedSet[o] = true
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		return allowedSet[origin]
	}
}

type wsIncomingMessage struct {
	Type      string `json:"type"`                // "auth", "message", or "stop"
	Token     string `json:"token,omitempty"`      // JWT token (for "auth" type)
	SessionID string `json:"session_id,omitempty"` // chat session ID
	Content   string `json:"content,omitempty"`    // message text
}

type wsOutgoingMessage struct {
	Type    string         `json:"type"`              // "auth_ok", "text_delta", "tool_call", "tool_result", "done", "error"
	Content string         `json:"content,omitempty"` // text or tool name
	Args    map[string]any `json:"args,omitempty"`    // tool call args
	Error   string         `json:"error,omitempty"`   // error message
}

// handle upgrades to WebSocket and processes chat messages.
// Auth is performed via the first message ("auth" type) instead of URL query parameter
// to avoid leaking the JWT token in logs, Referer headers, and browser history.
// Legacy fallback: if ?token= is provided in URL, it is still accepted for backward compat.
func (h *wsHandler) handle(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws: upgrade failed", "err", err)
		return
	}
	defer conn.Close()

	// Legacy support: authenticate via query parameter if provided
	var claims *auth.Claims
	if tokenStr := r.URL.Query().Get("token"); tokenStr != "" {
		claims, err = auth.ValidateToken(h.deps.JWTConfig, tokenStr)
		if err != nil {
			h.safeWriteJSON(conn, wsOutgoingMessage{Type: "error", Error: "invalid token"})
			return
		}
	}

	// If not authenticated via URL, require first-message auth within 30s
	if claims == nil {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.SetReadDeadline(time.Time{}) // reset deadline

		var authMsg wsIncomingMessage
		if err := json.Unmarshal(data, &authMsg); err != nil || authMsg.Type != "auth" || authMsg.Token == "" {
			h.safeWriteJSON(conn, wsOutgoingMessage{Type: "error", Error: "first message must be auth with token"})
			return
		}
		claims, err = auth.ValidateToken(h.deps.JWTConfig, authMsg.Token)
		if err != nil {
			h.safeWriteJSON(conn, wsOutgoingMessage{Type: "error", Error: "invalid token"})
			return
		}
	}

	h.safeWriteJSON(conn, wsOutgoingMessage{Type: "auth_ok"})

	var (
		cancelMu    sync.Mutex
		cancelFuncs = make(map[string]context.CancelFunc)
	)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Error("ws: read error", "err", err)
			}
			break
		}

		var msg wsIncomingMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			h.safeWriteJSON(conn, wsOutgoingMessage{Type: "error", Error: "invalid message format"})
			continue
		}

		switch msg.Type {
		case "stop":
			cancelMu.Lock()
			if fn, ok := cancelFuncs[msg.SessionID]; ok {
				fn()
				delete(cancelFuncs, msg.SessionID)
			}
			cancelMu.Unlock()

		case "message":
			if msg.SessionID == "" || msg.Content == "" {
				h.safeWriteJSON(conn, wsOutgoingMessage{Type: "error", Error: "session_id and content are required"})
				continue
			}

			cs, err := h.deps.ChatSessionStore.GetByAccountID(r.Context(), msg.SessionID, claims.AccountID)
			if err != nil {
				h.safeWriteJSON(conn, wsOutgoingMessage{Type: "error", Error: "session not found"})
				continue
			}

			if cs.AgentID == "" {
				h.safeWriteJSON(conn, wsOutgoingMessage{Type: "error", Error: "session has no agent assigned"})
				continue
			}

			tenantID := cs.TenantID
			if tenantID == "" && len(claims.Tenants) > 0 {
				tenantID = claims.Tenants[0]
			}

			ctx, cancel := context.WithCancel(r.Context())
			cancelMu.Lock()
			if prevFn, ok := cancelFuncs[msg.SessionID]; ok {
				prevFn()
			}
			cancelFuncs[msg.SessionID] = cancel
			cancelMu.Unlock()

			inbound := types.InboundMessage{
				Channel:    "web",
				TenantID:   tenantID,
				UserID:     claims.AccountID,
				ChatID:     msg.SessionID,
				AgentID:    cs.AgentID,
				Content:    msg.Content,
				SessionKey: cs.SessionKey,
			}

			go func() {
				defer func() {
					cancelMu.Lock()
					delete(cancelFuncs, msg.SessionID)
					cancelMu.Unlock()
				}()

				err := h.deps.AgentLoop.ProcessDirectStream(ctx, inbound, func(ev agent.StreamEvent) {
					out := wsOutgoingMessage{
						Type:    ev.Type,
						Content: ev.Content,
						Args:    ev.Args,
						Error:   ev.Error,
					}
					if writeErr := h.safeWriteJSON(conn, out); writeErr != nil {
						slog.Error("ws: write error", "err", writeErr)
					}
				})
				if err != nil && ctx.Err() == nil {
					h.safeWriteJSON(conn, wsOutgoingMessage{Type: "error", Error: err.Error()})
				}
			}()

		default:
			h.safeWriteJSON(conn, wsOutgoingMessage{Type: "error", Error: "unknown message type: " + msg.Type})
		}
	}
}

// safeWriteJSON writes a JSON message to the WebSocket with mutex protection.
func (h *wsHandler) safeWriteJSON(conn *websocket.Conn, v any) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	data, _ := json.Marshal(v)
	return conn.WriteMessage(websocket.TextMessage, data)
}
