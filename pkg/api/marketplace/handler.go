// Package marketplace provides HTTP handlers for the skill marketplace API,
// including skill browsing, installation, and promotion proposals.
package marketplace

import (
	"encoding/json"
	"net/http"
	"strconv"

	"abot/pkg/api/auth"
	"abot/pkg/types"
)

const maxBodySize = 1 << 20 // 1 MB

// Deps holds dependencies for marketplace handlers.
type Deps struct {
	Skills         types.SkillRegistryStore
	Tenants        types.TenantSkillStore
	Proposals      types.SkillProposalStore
	AccTenantStore types.AccountTenantStore
}

// Handler returns an http.Handler for the marketplace API.
// All mutating endpoints require JWT authentication.
func Handler(deps Deps, jwtCfg auth.JWTConfig) http.Handler {
	mux := http.NewServeMux()
	h := &handler{deps: deps}
	authMW := auth.AuthMiddleware(jwtCfg)

	// Public read-only endpoints
	mux.HandleFunc("GET /api/skills", h.listSkills)
	mux.HandleFunc("GET /api/skills/{name}", h.getSkill)

	// Authenticated + tenant-authorized endpoints
	protected := http.NewServeMux()
	protected.HandleFunc("POST /api/skills/{name}/install", h.installSkill)
	protected.HandleFunc("DELETE /api/skills/{name}/uninstall", h.uninstallSkill)
	protected.HandleFunc("GET /api/tenants/{tenant}/skills", h.listInstalled)
	protected.HandleFunc("POST /api/proposals", h.createProposal)
	protected.HandleFunc("GET /api/proposals", h.listProposals)
	protected.HandleFunc("POST /api/proposals/{id}/review", h.reviewProposal)

	mux.Handle("/api/skills/", authMW(protected))
	mux.Handle("/api/tenants/", authMW(protected))
	mux.Handle("/api/proposals", authMW(protected))
	mux.Handle("/api/proposals/", authMW(protected))

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

// verifyTenantAccess checks that the authenticated user has access to the given tenant.
func (h *handler) verifyTenantAccess(r *http.Request, tenantID string) bool {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		return false
	}
	for _, t := range claims.Tenants {
		if t == tenantID {
			return true
		}
	}
	return false
}

// GET /api/skills?tier=&status=
func (h *handler) listSkills(w http.ResponseWriter, r *http.Request) {
	opts := types.SkillListOpts{
		Tier:   types.SkillTier(r.URL.Query().Get("tier")),
		Status: r.URL.Query().Get("status"),
	}
	skills, err := h.deps.Skills.List(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list skills")
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
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var body struct {
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	if !h.verifyTenantAccess(r, body.TenantID) {
		writeError(w, http.StatusForbidden, "access denied to tenant")
		return
	}
	skill, err := h.deps.Skills.Get(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
	ts := &types.TenantSkill{TenantID: body.TenantID, SkillID: skill.ID}
	if err := h.deps.Tenants.Install(r.Context(), ts); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to install skill")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": "installed"})
}

// DELETE /api/skills/{name}/uninstall — body: {"tenant_id":"..."}
func (h *handler) uninstallSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var body struct {
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	if !h.verifyTenantAccess(r, body.TenantID) {
		writeError(w, http.StatusForbidden, "access denied to tenant")
		return
	}
	skill, err := h.deps.Skills.Get(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
	if err := h.deps.Tenants.Uninstall(r.Context(), body.TenantID, skill.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to uninstall skill")
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
	if !h.verifyTenantAccess(r, tenant) {
		writeError(w, http.StatusForbidden, "access denied to tenant")
		return
	}
	installed, err := h.deps.Tenants.ListInstalled(r.Context(), tenant)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list installed skills")
		return
	}
	writeJSON(w, http.StatusOK, installed)
}

// POST /api/proposals — body: {"skill_name":"...", "proposed_by":"...", "object_path":"..."}
func (h *handler) createProposal(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var p types.SkillProposal
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if p.SkillName == "" {
		writeError(w, http.StatusBadRequest, "skill_name is required")
		return
	}
	claims := auth.ClaimsFromCtx(r.Context())
	if claims != nil {
		p.ProposedBy = claims.AccountID
	}
	p.Status = "pending"
	if err := h.deps.Proposals.Create(r.Context(), &p); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create proposal")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// GET /api/proposals?status=pending
func (h *handler) listProposals(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	proposals, err := h.deps.Proposals.List(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list proposals")
		return
	}
	writeJSON(w, http.StatusOK, proposals)
}

// POST /api/proposals/{id}/review — body: {"status":"approved|rejected"}
// Only admin users can review proposals.
func (h *handler) reviewProposal(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil || claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid proposal id")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Status != "approved" && body.Status != "rejected" {
		writeError(w, http.StatusBadRequest, "status must be approved or rejected")
		return
	}
	if err := h.deps.Proposals.UpdateStatus(r.Context(), id, body.Status, claims.AccountID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update proposal")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": body.Status})
}
