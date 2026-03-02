package console

import (
	"encoding/json"
	"net/http"
	"strings"

	"abot/pkg/api/auth"
	"abot/pkg/security"
)

type providersHandler struct {
	deps Deps
}

// Deps needs EncryptionSecret for API key encryption.
// (Already defined in handler.go, but we reference it here.)

type providerSettingsDTO struct {
	APIBase string `json:"api_base"`
	APIKey  string `json:"api_key"` // masked on read
	Model   string `json:"model"`
}

// GET /api/v1/settings/providers
func (h *providersHandler) get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	tenantID := ""
	if len(claims.Tenants) > 0 {
		tenantID = claims.Tenants[0]
	}

	tenant, err := h.deps.TenantStore.Get(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}

	cfg := tenant.Config
	if cfg == nil {
		cfg = map[string]any{}
	}

	// Decrypt API key for display (masked).
	encryptedKey := getString(cfg, "provider_api_key")
	decryptedKey := ""
	if encryptedKey != "" && h.deps.EncryptionSecret != "" {
		decryptedKey, _ = security.DecryptAPIKey(encryptedKey, h.deps.EncryptionSecret)
	}

	result := providerSettingsDTO{
		APIBase: getString(cfg, "provider_api_base"),
		APIKey:  maskKey(decryptedKey),
		Model:   getString(cfg, "provider_model"),
	}
	writeJSON(w, http.StatusOK, result)
}

// PUT /api/v1/settings/providers
func (h *providersHandler) update(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	tenantID := ""
	if len(claims.Tenants) > 0 {
		tenantID = claims.Tenants[0]
	}

	var req providerSettingsDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tenant, err := h.deps.TenantStore.Get(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}

	if tenant.Config == nil {
		tenant.Config = map[string]any{}
	}

	if req.APIBase != "" {
		tenant.Config["provider_api_base"] = req.APIBase
	}
	// Only update key if it's not masked (i.e., user actually changed it).
	if req.APIKey != "" && !strings.Contains(req.APIKey, "****") {
		// Encrypt the API key before storing.
		if h.deps.EncryptionSecret != "" {
			encrypted, err := security.EncryptAPIKey(req.APIKey, h.deps.EncryptionSecret)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to encrypt API key")
				return
			}
			tenant.Config["provider_api_key"] = encrypted
		} else {
			tenant.Config["provider_api_key"] = req.APIKey
		}
	}
	if req.Model != "" {
		tenant.Config["provider_model"] = req.Model
	}

	if err := h.deps.TenantStore.Put(r.Context(), tenant); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save settings")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"result": "updated"})
}

func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
