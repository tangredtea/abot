package console

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"abot/pkg/api/auth"
	"abot/pkg/types"
)

// WorkspaceDocRequest is the request body for creating/updating workspace docs.
type WorkspaceDocRequest struct {
	DocType string `json:"doc_type"`
	Content string `json:"content"`
}

// WorkspaceDocResponse is the response for workspace doc operations.
type WorkspaceDocResponse struct {
	TenantID  string `json:"tenant_id"`
	UserID    string `json:"user_id"`
	DocType   string `json:"doc_type"`
	Content   string `json:"content"`
	Version   int64  `json:"version"`
	UpdatedAt string `json:"updated_at"`
}

// workspaceDocsHandler wraps Deps for workspace docs handlers.
type workspaceDocsHandler struct {
	deps Deps
}

// handleGetWorkspaceDoc handles GET /api/v1/workspace/docs/:doc_type
func (h *workspaceDocsHandler) handleGetWorkspaceDoc(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := auth.ClaimsFromCtx(ctx)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	docType := r.PathValue("doc_type")
	if docType == "" {
		writeError(w, http.StatusBadRequest, "doc_type is required")
		return
	}

	// Get tenant_id from account
	tenantID, err := h.getTenantIDForAccount(ctx, claims.AccountID)
	if err != nil {
		slog.Error("get tenant for account", "account_id", claims.AccountID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get tenant")
		return
	}

	// Try user-level doc first
	userDoc, err := h.deps.UserWorkspaceStore.Get(ctx, tenantID, claims.AccountID, docType)
	if err == nil && userDoc != nil {
		writeJSON(w, http.StatusOK, WorkspaceDocResponse{
			TenantID:  userDoc.TenantID,
			UserID:    userDoc.UserID,
			DocType:   userDoc.DocType,
			Content:   userDoc.Content,
			Version:   userDoc.Version,
			UpdatedAt: userDoc.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
		return
	}

	// Fallback to tenant-level doc
	tenantDoc, err := h.deps.WorkspaceStore.Get(ctx, tenantID, docType)
	if err != nil {
		slog.Error("get workspace doc", "tenant_id", tenantID, "doc_type", docType, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get workspace doc")
		return
	}
	if tenantDoc == nil {
		// Return empty doc
		writeJSON(w, http.StatusOK, WorkspaceDocResponse{
			TenantID: tenantID,
			UserID:   claims.AccountID,
			DocType:  docType,
			Content:  "",
			Version:  0,
		})
		return
	}

	writeJSON(w, http.StatusOK, WorkspaceDocResponse{
		TenantID:  tenantDoc.TenantID,
		UserID:    claims.AccountID,
		DocType:   tenantDoc.DocType,
		Content:   tenantDoc.Content,
		Version:   tenantDoc.Version,
		UpdatedAt: tenantDoc.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}

// handleUpdateWorkspaceDoc handles PUT /api/v1/workspace/docs/:doc_type
func (h *workspaceDocsHandler) handleUpdateWorkspaceDoc(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := auth.ClaimsFromCtx(ctx)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	docType := r.PathValue("doc_type")
	if docType == "" {
		writeError(w, http.StatusBadRequest, "doc_type is required")
		return
	}

	var req WorkspaceDocRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Get tenant_id from account
	tenantID, err := h.getTenantIDForAccount(ctx, claims.AccountID)
	if err != nil {
		slog.Error("get tenant for account", "account_id", claims.AccountID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get tenant")
		return
	}

	// Store as user-level doc
	doc := &types.UserWorkspaceDoc{
		TenantID: tenantID,
		UserID:   claims.AccountID,
		DocType:  docType,
		Content:  req.Content,
		Version:  1,
	}

	if err := h.deps.UserWorkspaceStore.Put(ctx, doc); err != nil {
		slog.Error("update workspace doc", "tenant_id", tenantID, "doc_type", docType, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to update workspace doc")
		return
	}

	slog.Info("workspace doc updated", "tenant_id", tenantID, "user_id", claims.AccountID, "doc_type", docType)

	writeJSON(w, http.StatusOK, WorkspaceDocResponse{
		TenantID: tenantID,
		UserID:   claims.AccountID,
		DocType:  docType,
		Content:  req.Content,
		Version:  doc.Version,
	})
}

// handleListWorkspaceDocs handles GET /api/v1/workspace/docs
func (h *workspaceDocsHandler) handleListWorkspaceDocs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := auth.ClaimsFromCtx(ctx)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	// Get tenant_id from account
	tenantID, err := h.getTenantIDForAccount(ctx, claims.AccountID)
	if err != nil {
		slog.Error("get tenant for account", "account_id", claims.AccountID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get tenant")
		return
	}

	// List all doc types
	docTypes := []string{"IDENTITY", "SOUL", "RULES", "AGENT"}
	var docs []WorkspaceDocResponse

	for _, docType := range docTypes {
		// Try user-level doc first
		userDoc, err := h.deps.UserWorkspaceStore.Get(ctx, tenantID, claims.AccountID, docType)
		if err == nil && userDoc != nil {
			docs = append(docs, WorkspaceDocResponse{
				TenantID:  userDoc.TenantID,
				UserID:    userDoc.UserID,
				DocType:   userDoc.DocType,
				Content:   userDoc.Content,
				Version:   userDoc.Version,
				UpdatedAt: userDoc.UpdatedAt.Format("2006-01-02 15:04:05"),
			})
			continue
		}

		// Fallback to tenant-level doc
		tenantDoc, err := h.deps.WorkspaceStore.Get(ctx, tenantID, docType)
		if err == nil && tenantDoc != nil {
			docs = append(docs, WorkspaceDocResponse{
				TenantID:  tenantDoc.TenantID,
				UserID:    claims.AccountID,
				DocType:   tenantDoc.DocType,
				Content:   tenantDoc.Content,
				Version:   tenantDoc.Version,
				UpdatedAt: tenantDoc.UpdatedAt.Format("2006-01-02 15:04:05"),
			})
		} else {
			// Return empty doc
			docs = append(docs, WorkspaceDocResponse{
				TenantID: tenantID,
				UserID:   claims.AccountID,
				DocType:  docType,
				Content:  "",
				Version:  0,
			})
		}
	}

	writeJSON(w, http.StatusOK, docs)
}

// getTenantIDForAccount retrieves the tenant_id for an account.
func (h *workspaceDocsHandler) getTenantIDForAccount(ctx context.Context, accountID string) (string, error) {
	// Get tenant from account_tenants
	tenants, err := h.deps.AccTenantStore.ListByAccount(ctx, accountID)
	if err != nil {
		return "", err
	}
	if len(tenants) == 0 {
		return types.DefaultTenantID, nil
	}
	return tenants[0].TenantID, nil
}
