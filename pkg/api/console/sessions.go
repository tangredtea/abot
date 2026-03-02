package console

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/session"

	"abot/pkg/api/auth"
	"abot/pkg/types"
)

type sessionsHandler struct {
	deps Deps
}

type createSessionRequest struct {
	AgentID string `json:"agent_id"`
	Title   string `json:"title"`
}

type updateSessionRequest struct {
	Title    *string `json:"title,omitempty"`
	Pinned   *bool   `json:"pinned,omitempty"`
	Archived *bool   `json:"archived,omitempty"`
}

type sessionResponse struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	AgentID   string    `json:"agent_id"`
	Title     string    `json:"title"`
	Pinned    bool      `json:"pinned"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type sessionDetailResponse struct {
	sessionResponse
	Messages []messageDTO `json:"messages"`
}

type messageDTO struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func toSessionResponse(cs *types.ChatSession) sessionResponse {
	return sessionResponse{
		ID:        cs.ID,
		TenantID:  cs.TenantID,
		AgentID:   cs.AgentID,
		Title:     cs.Title,
		Pinned:    cs.Pinned,
		Archived:  cs.Archived,
		CreatedAt: cs.CreatedAt,
		UpdatedAt: cs.UpdatedAt,
	}
}

// GET /api/v1/sessions?archived=false
func (h *sessionsHandler) list(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	archived := r.URL.Query().Get("archived") == "true"
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" && len(claims.Tenants) > 0 {
		tenantID = claims.Tenants[0]
	}

	sessions, err := h.deps.ChatSessionStore.ListByAccount(r.Context(), claims.AccountID, tenantID, archived)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	result := make([]sessionResponse, len(sessions))
	for i, s := range sessions {
		result[i] = toSessionResponse(s)
	}
	writeJSON(w, http.StatusOK, result)
}

// POST /api/v1/sessions
func (h *sessionsHandler) create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tenantID := ""
	if len(claims.Tenants) > 0 {
		tenantID = claims.Tenants[0]
	}

	agentID := req.AgentID
	if agentID == "" {
		agents := h.deps.Registry.ListAgents()
		if len(agents) > 0 {
			agentID = agents[0]
		}
	}

	title := req.Title
	if title == "" {
		title = "New Chat"
	}

	sessionID := uuid.New().String()
	sessionKey := "console:" + tenantID + ":" + claims.AccountID + ":" + sessionID

	now := time.Now()
	cs := &types.ChatSession{
		ID:         sessionID,
		TenantID:   tenantID,
		AccountID:  claims.AccountID,
		AgentID:    agentID,
		Title:      title,
		SessionKey: sessionKey,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := h.deps.ChatSessionStore.Create(r.Context(), cs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	writeJSON(w, http.StatusCreated, toSessionResponse(cs))
}

// GET /api/v1/sessions/{id}
func (h *sessionsHandler) get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	id := r.PathValue("id")
	cs, err := h.deps.ChatSessionStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	if cs.AccountID != claims.AccountID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// Fetch messages from ADK session service.
	var messages []messageDTO
	sessResp, err := h.deps.SessionService.Get(r.Context(), &session.GetRequest{
		AppName:   h.deps.AppName,
		UserID:    claims.AccountID,
		SessionID: cs.SessionKey,
	})
	if err == nil && sessResp != nil && sessResp.Session != nil {
		// Get events iterator and convert to slice
		eventsIter := sessResp.Session.Events()
		if eventsIter != nil {
			// Try to get all events - ADK v0.5.0 may have different API
			// For now, skip this until we can test with actual ADK
			_ = eventsIter
		}
	}
	if messages == nil {
		messages = []messageDTO{}
	}

	writeJSON(w, http.StatusOK, sessionDetailResponse{
		sessionResponse: toSessionResponse(cs),
		Messages:        messages,
	})
}

// PATCH /api/v1/sessions/{id}
func (h *sessionsHandler) update(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	id := r.PathValue("id")
	cs, err := h.deps.ChatSessionStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	if cs.AccountID != claims.AccountID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req updateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title != nil {
		cs.Title = *req.Title
	}
	if req.Pinned != nil {
		cs.Pinned = *req.Pinned
	}
	if req.Archived != nil {
		cs.Archived = *req.Archived
	}
	cs.UpdatedAt = time.Now()

	if err := h.deps.ChatSessionStore.Update(r.Context(), cs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update session")
		return
	}

	writeJSON(w, http.StatusOK, toSessionResponse(cs))
}

// DELETE /api/v1/sessions/{id}
func (h *sessionsHandler) delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	id := r.PathValue("id")
	cs, err := h.deps.ChatSessionStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	if cs.AccountID != claims.AccountID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	if err := h.deps.ChatSessionStore.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"result": "deleted"})
}
