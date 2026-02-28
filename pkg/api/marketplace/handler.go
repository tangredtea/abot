// Package marketplace provides HTTP handlers for the skill marketplace API,
// including skill browsing, installation, and promotion proposals.
package marketplace

import (
	"encoding/json"
	"net/http"
	"strconv"

	"abot/pkg/types"
)

// Deps holds dependencies for marketplace handlers.
type Deps struct {
	Skills    types.SkillRegistryStore
	Tenants   types.TenantSkillStore
	Proposals types.SkillProposalStore
}

// Handler returns an http.Handler for the marketplace API.
func Handler(deps Deps) http.Handler {
	mux := http.NewServeMux()
	h := &handler{deps: deps}

	mux.HandleFunc("GET /api/skills", h.listSkills)
	mux.HandleFunc("GET /api/skills/{name}", h.getSkill)
	mux.HandleFunc("POST /api/skills/{name}/install", h.installSkill)
	mux.HandleFunc("DELETE /api/skills/{name}/uninstall", h.uninstallSkill)
	mux.HandleFunc("GET /api/tenants/{tenant}/skills", h.listInstalled)
	mux.HandleFunc("POST /api/proposals", h.createProposal)
	mux.HandleFunc("GET /api/proposals", h.listProposals)
	mux.HandleFunc("POST /api/proposals/{id}/review", h.reviewProposal)

	return mux
}

type handler struct {
	deps Deps
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// GET /api/skills?tier=&status=
func (h *handler) listSkills(w http.ResponseWriter, r *http.Request) {
	opts := types.SkillListOpts{
		Tier:   types.SkillTier(r.URL.Query().Get("tier")),
		Status: r.URL.Query().Get("status"),
	}
	skills, err := h.deps.Skills.List(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

// GET /api/skills/{name}
func (h *handler) getSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	skill, err := h.deps.Skills.Get(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
	writeJSON(w, http.StatusOK, skill)
}

// POST /api/skills/{name}/install — body: {"tenant_id":"..."}
func (h *handler) installSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body struct {
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	skill, err := h.deps.Skills.Get(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
	ts := &types.TenantSkill{TenantID: body.TenantID, SkillID: skill.ID}
	if err := h.deps.Tenants.Install(r.Context(), ts); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": "installed"})
}

// DELETE /api/skills/{name}/uninstall — body: {"tenant_id":"..."}
func (h *handler) uninstallSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body struct {
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	skill, err := h.deps.Skills.Get(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
	if err := h.deps.Tenants.Uninstall(r.Context(), body.TenantID, skill.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": "uninstalled"})
}

// GET /api/tenants/{tenant}/skills
func (h *handler) listInstalled(w http.ResponseWriter, r *http.Request) {
	tenant := r.PathValue("tenant")
	if tenant == "" {
		writeError(w, http.StatusBadRequest, "tenant is required")
		return
	}
	installed, err := h.deps.Tenants.ListInstalled(r.Context(), tenant)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, installed)
}

// POST /api/proposals — body: {"skill_name":"...", "proposed_by":"...", "object_path":"..."}
func (h *handler) createProposal(w http.ResponseWriter, r *http.Request) {
	var p types.SkillProposal
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if p.SkillName == "" || p.ProposedBy == "" {
		writeError(w, http.StatusBadRequest, "skill_name and proposed_by are required")
		return
	}
	p.Status = "pending"
	if err := h.deps.Proposals.Create(r.Context(), &p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// GET /api/proposals?status=pending
func (h *handler) listProposals(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	proposals, err := h.deps.Proposals.List(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, proposals)
}

// POST /api/proposals/{id}/review — body: {"status":"approved|rejected", "reviewed_by":"..."}
func (h *handler) reviewProposal(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid proposal id")
		return
	}
	var body struct {
		Status     string `json:"status"`
		ReviewedBy string `json:"reviewed_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Status != "approved" && body.Status != "rejected" {
		writeError(w, http.StatusBadRequest, "status must be approved or rejected")
		return
	}
	if err := h.deps.Proposals.UpdateStatus(r.Context(), id, body.Status, body.ReviewedBy); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": body.Status})
}
