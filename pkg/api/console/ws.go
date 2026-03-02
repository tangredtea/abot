package console

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"abot/pkg/agent"
	"abot/pkg/api/auth"
	"abot/pkg/types"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsHandler struct {
	deps    Deps
	writeMu sync.Mutex
}

type wsIncomingMessage struct {
	Type      string `json:"type"`       // "message" or "stop"
	SessionID string `json:"session_id"` // chat session ID
	Content   string `json:"content"`    // message text
}

type wsOutgoingMessage struct {
	Type    string         `json:"type"`              // "text_delta", "tool_call", "tool_result", "done", "error"
	Content string         `json:"content,omitempty"` // text or tool name
	Args    map[string]any `json:"args,omitempty"`    // tool call args
	Error   string         `json:"error,omitempty"`   // error message
}

// handle upgrades to WebSocket and processes chat messages.
// Auth is via query parameter: /api/v1/chat/ws?token=<jwt>
func (h *wsHandler) handle(w http.ResponseWriter, r *http.Request) {
	// Authenticate via query token.
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	claims, err := auth.ValidateToken(h.deps.JWTConfig, tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws: upgrade failed", "err", err)
		return
	}
	defer conn.Close()

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

			// Look up the chat session.
			cs, err := h.deps.ChatSessionStore.Get(r.Context(), msg.SessionID)
			if err != nil {
				h.safeWriteJSON(conn, wsOutgoingMessage{Type: "error", Error: "session not found"})
				continue
			}
			if cs.AccountID != claims.AccountID {
				h.safeWriteJSON(conn, wsOutgoingMessage{Type: "error", Error: "access denied"})
				continue
			}

			// Validate agent exists
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
				prevFn() // Cancel previous generation for this session.
			}
			cancelFuncs[msg.SessionID] = cancel
			cancelMu.Unlock()

			inbound := types.InboundMessage{
				Channel:    "web",
				TenantID:   tenantID,
				UserID:     claims.AccountID,
				ChatID:     msg.SessionID,
				AgentID:    cs.AgentID, // Use agent from session
				Content:    msg.Content,
				SessionKey: cs.SessionKey,
			}

			// Stream the response.
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

func writeWSError(conn *websocket.Conn, msg string) {
	out := wsOutgoingMessage{Type: "error", Error: msg}
	data, _ := json.Marshal(out)
	conn.WriteMessage(websocket.TextMessage, data)
}

// safeWriteJSON writes a JSON message to the WebSocket with mutex protection.
func (h *wsHandler) safeWriteJSON(conn *websocket.Conn, v any) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	data, _ := json.Marshal(v)
	return conn.WriteMessage(websocket.TextMessage, data)
}
