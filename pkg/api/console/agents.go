package console

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"abot/pkg/api/auth"
	"abot/pkg/storage/mysql"
)

type agentsHandler struct {
	deps Deps
}

func (h *agentsHandler) list(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	tenantID := ""
	if len(claims.Tenants) > 0 {
		tenantID = claims.Tenants[0]
	}

	if h.deps.DB != nil {
		store := mysql.NewAgentDefinitionStore(h.deps.DB)
		agents, err := store.List(r.Context(), tenantID)
		if err == nil && len(agents) > 0 {
			writeJSON(w, http.StatusOK, agents)
			return
		}
	}

	agentIDs := h.deps.Registry.ListAgents()
	result := make([]map[string]interface{}, len(agentIDs))
	for i, agentID := range agentIDs {
		entry, ok := h.deps.Registry.GetEntry(agentID)
		if !ok {
			continue
		}
		result[i] = map[string]interface{}{
			"id":          entry.Config.ID,
			"name":        entry.Config.Name,
			"description": entry.Config.Description,
			"model":       entry.Config.Model,
			"status":      "active",
			"avatar":      "🤖",
			"channels":    []string{"web"},
			"created_at":  time.Now().Format("2006-01-02"),
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *agentsHandler) get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	if h.deps.DB != nil {
		store := mysql.NewAgentDefinitionStore(h.deps.DB)
		agent, err := store.Get(r.Context(), agentID)
		if err == nil && agent != nil {
			writeJSON(w, http.StatusOK, agent)
			return
		}
	}

	entry, ok := h.deps.Registry.GetEntry(agentID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	result := map[string]interface{}{
		"id":          entry.Config.ID,
		"name":        entry.Config.Name,
		"description": entry.Config.Description,
		"model":       entry.Config.Model,
		"status":      "active",
		"avatar":      "🤖",
		"channels":    []string{"web"},
		"created_at":  time.Now().Format("2006-01-02"),
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *agentsHandler) create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	tenantID := ""
	if len(claims.Tenants) > 0 {
		tenantID = claims.Tenants[0]
	}

	var req struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Avatar      string                 `json:"avatar"`
		Model       string                 `json:"model"`
		Provider    string                 `json:"provider"`
		Config      map[string]interface{} `json:"config"`
		Channels    []struct {
			Channel string                 `json:"channel"`
			Enabled bool                   `json:"enabled"`
			Config  map[string]interface{} `json:"config"`
		} `json:"channels"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.Avatar == "" {
		req.Avatar = "🤖"
	}
	if req.Model == "" {
		req.Model = "MiniMax-M2.5"
	}
	if req.Provider == "" {
		req.Provider = "primary"
	}
	if req.Config == nil {
		req.Config = map[string]interface{}{
			"system_prompt": "你是一个友好、专业的 AI 助手。",
			"temperature":   0.7,
			"max_tokens":    2048,
			"top_p":         0.9,
		}
	}

	agent := &mysql.AgentDefinition{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Avatar:      req.Avatar,
		Model:       req.Model,
		Provider:    req.Provider,
		Status:      "active",
		Config:      req.Config,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if len(req.Channels) > 0 {
		for _, ch := range req.Channels {
			agent.Channels = append(agent.Channels, mysql.AgentChannel{
				AgentID: agent.ID,
				Channel: ch.Channel,
				Enabled: ch.Enabled,
				Config:  ch.Config,
			})
		}
	} else {
		agent.Channels = []mysql.AgentChannel{
			{
				AgentID: agent.ID,
				Channel: "web",
				Enabled: true,
				Config:  map[string]interface{}{},
			},
		}
	}

	store := mysql.NewAgentDefinitionStore(h.deps.DB)
	if err := store.Create(r.Context(), agent); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create agent")
		return
	}

	writeJSON(w, http.StatusCreated, agent)
}

func (h *agentsHandler) update(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	store := mysql.NewAgentDefinitionStore(h.deps.DB)
	existing, err := store.Get(r.Context(), agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get agent")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var req struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Avatar      string                 `json:"avatar"`
		Model       string                 `json:"model"`
		Provider    string                 `json:"provider"`
		Status      string                 `json:"status"`
		Config      map[string]interface{} `json:"config"`
		Channels    []struct {
			Channel string                 `json:"channel"`
			Enabled bool                   `json:"enabled"`
			Config  map[string]interface{} `json:"config"`
		} `json:"channels"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.Avatar != "" {
		existing.Avatar = req.Avatar
	}
	if req.Model != "" {
		existing.Model = req.Model
	}
	if req.Provider != "" {
		existing.Provider = req.Provider
	}
	if req.Status != "" {
		existing.Status = req.Status
	}
	if req.Config != nil {
		existing.Config = req.Config
	}
	if len(req.Channels) > 0 {
		existing.Channels = nil
		for _, ch := range req.Channels {
			existing.Channels = append(existing.Channels, mysql.AgentChannel{
				AgentID: agentID,
				Channel: ch.Channel,
				Enabled: ch.Enabled,
				Config:  ch.Config,
			})
		}
	}

	existing.UpdatedAt = time.Now()

	if err := store.Update(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update agent")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func (h *agentsHandler) deleteAgent(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	store := mysql.NewAgentDefinitionStore(h.deps.DB)
	if err := store.Delete(r.Context(), agentID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete agent")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "agent deleted"})
}

func (h *agentsHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	store := mysql.NewAgentDefinitionStore(h.deps.DB)
	agent, err := store.Get(r.Context(), agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get agent")
		return
	}
	if agent == nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	writeJSON(w, http.StatusOK, agent.Config)
}

func (h *agentsHandler) updateConfig(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	store := mysql.NewAgentDefinitionStore(h.deps.DB)
	agent, err := store.Get(r.Context(), agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get agent")
		return
	}
	if agent == nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var config map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	agent.Config = config
	agent.UpdatedAt = time.Now()

	if err := store.Update(r.Context(), agent); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update config")
		return
	}

	writeJSON(w, http.StatusOK, agent.Config)
}

func (h *agentsHandler) getChannels(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	store := mysql.NewAgentDefinitionStore(h.deps.DB)
	agent, err := store.Get(r.Context(), agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get agent")
		return
	}
	if agent == nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	writeJSON(w, http.StatusOK, agent.Channels)
}

func (h *agentsHandler) updateChannels(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	store := mysql.NewAgentDefinitionStore(h.deps.DB)
	agent, err := store.Get(r.Context(), agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get agent")
		return
	}
	if agent == nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var channels []struct {
		Channel string                 `json:"channel"`
		Enabled bool                   `json:"enabled"`
		Config  map[string]interface{} `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&channels); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	agent.Channels = nil
	for _, ch := range channels {
		agent.Channels = append(agent.Channels, mysql.AgentChannel{
			AgentID: agentID,
			Channel: ch.Channel,
			Enabled: ch.Enabled,
			Config:  ch.Config,
		})
	}

	agent.UpdatedAt = time.Now()

	if err := store.Update(r.Context(), agent); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update channels")
		return
	}

	writeJSON(w, http.StatusOK, agent.Channels)
}
