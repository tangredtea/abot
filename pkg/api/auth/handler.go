package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	apierrors "abot/pkg/api/errors"
	"abot/pkg/api/middleware"
	"abot/pkg/api/response"
	"abot/pkg/api/validation"
	"abot/pkg/types"
)

// Deps holds dependencies for auth handlers.
type Deps struct {
	AccountStore    types.AccountStore
	AccTenantStore  types.AccountTenantStore
	TenantStore     types.TenantStore
	WorkspaceStore  types.WorkspaceStore
	JWTConfig       JWTConfig
	DB              *gorm.DB
}

// Handler returns an http.Handler for auth endpoints.
func Handler(deps Deps) http.Handler {
	mux := http.NewServeMux()
	h := &authHandler{deps: deps}

	// Apply rate limiting to auth endpoints (20 requests per minute with burst of 5).
	rateLimiter := middleware.RateLimit(middleware.RateLimitConfig{
		RequestsPerMinute: 20,
		Burst:             5,
	})

	mux.Handle("POST /api/v1/auth/register", rateLimiter(http.HandlerFunc(h.register)))
	mux.Handle("POST /api/v1/auth/login", rateLimiter(http.HandlerFunc(h.login)))
	mux.HandleFunc("POST /api/v1/auth/refresh", h.refresh)

	return mux
}

// MeHandler returns a handler for the authenticated /me endpoint.
func MeHandler(deps Deps) http.HandlerFunc {
	h := &authHandler{deps: deps}
	return h.me
}

type authHandler struct {
	deps Deps
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

type meResponse struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Role        string   `json:"role"`
	Tenants     []string `json:"tenants"`
}

func (h *authHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.HandleError(w, apierrors.BadRequest("invalid request body"))
		return
	}

	// Validate email.
	if err := validation.ValidateEmail(req.Email); err != nil {
		apierrors.HandleError(w, apierrors.ValidationError(err.Error()))
		return
	}

	// Validate password.
	if err := validation.ValidatePassword(req.Password); err != nil {
		apierrors.HandleError(w, apierrors.ValidationError(err.Error()))
		return
	}

	// Check if email already exists.
	if _, err := h.deps.AccountStore.GetByEmail(r.Context(), req.Email); err == nil {
		apierrors.HandleError(w, apierrors.New(
			apierrors.CodeEmailAlreadyExists,
			"email already registered",
			http.StatusConflict,
			nil,
		))
		return
	}

	// Hash password.
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		apierrors.HandleError(w, apierrors.InternalError(fmt.Errorf("hash password: %w", err)))
		return
	}

	accountID := uuid.New().String()
	tenantID := "t-" + uuid.New().String()[:8]
	now := time.Now()

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Email
	}

	// Wrap registration in a transaction to ensure atomicity.
	err = h.deps.DB.Transaction(func(tx *gorm.DB) error {
		// Create account.
		account := &types.Account{
			ID:           accountID,
			Email:        req.Email,
			PasswordHash: string(hash),
			DisplayName:  displayName,
			Status:       "active",
			Role:         "user",
			CreatedAt:    now,
		}
		if err := h.deps.AccountStore.Create(r.Context(), account); err != nil {
			return fmt.Errorf("create account: %w", err)
		}

		// Create tenant.
		tenant := &types.Tenant{
			TenantID:  tenantID,
			Name:      displayName + "'s workspace",
			CreatedAt: now,
		}
		if err := h.deps.TenantStore.Put(r.Context(), tenant); err != nil {
			return fmt.Errorf("create tenant: %w", err)
		}

		// Link account to tenant.
		at := &types.AccountTenant{
			AccountID: accountID,
			TenantID:  tenantID,
			Role:      "owner",
			CreatedAt: now,
		}
		if err := h.deps.AccTenantStore.Create(r.Context(), at); err != nil {
			return fmt.Errorf("link account to tenant: %w", err)
		}

		// Seed workspace with default docs.
		defaultDocs := map[string]string{
			"IDENTITY": "You are a helpful AI assistant.",
			"RULES":    "Be helpful, harmless, and honest.",
		}
		for docType, content := range defaultDocs {
			if err := h.deps.WorkspaceStore.Put(r.Context(), &types.WorkspaceDoc{
				TenantID: tenantID,
				DocType:  docType,
				Content:  content,
				Version:  1,
			}); err != nil {
				return fmt.Errorf("seed workspace doc %s: %w", docType, err)
			}
		}

		return nil
	})
	if err != nil {
		apierrors.HandleError(w, apierrors.InternalError(fmt.Errorf("create account: %w", err)))
		return
	}

	// Generate token.
	token, err := GenerateToken(h.deps.JWTConfig, accountID, "user", []string{tenantID})
	if err != nil {
		apierrors.HandleError(w, apierrors.InternalError(fmt.Errorf("generate token: %w", err)))
		return
	}

	response.Created(w, tokenResponse{Token: token})
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeAuthError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	account, err := h.deps.AccountStore.GetByEmail(r.Context(), req.Email)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(req.Password)); err != nil {
		writeAuthError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if account.Status != "active" {
		writeAuthError(w, http.StatusForbidden, "account is suspended")
		return
	}

	// Get tenants.
	ats, err := h.deps.AccTenantStore.ListByAccount(r.Context(), account.ID)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "failed to load tenants")
		return
	}
	tenantIDs := make([]string, len(ats))
	for i, at := range ats {
		tenantIDs[i] = at.TenantID
	}

	token, err := GenerateToken(h.deps.JWTConfig, account.ID, account.Role, tenantIDs)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeAuthJSON(w, http.StatusOK, tokenResponse{Token: token})
}

func (h *authHandler) refresh(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractBearerToken(r)
	if tokenStr == "" {
		writeAuthError(w, http.StatusUnauthorized, "missing token")
		return
	}

	claims, err := ValidateToken(h.deps.JWTConfig, tokenStr)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	// Re-fetch account to ensure it's still active.
	account, err := h.deps.AccountStore.GetByID(r.Context(), claims.AccountID)
	if err != nil || account.Status != "active" {
		writeAuthError(w, http.StatusForbidden, "account not found or suspended")
		return
	}

	// Re-fetch tenants.
	ats, err := h.deps.AccTenantStore.ListByAccount(r.Context(), account.ID)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "failed to load tenants")
		return
	}
	tenantIDs := make([]string, len(ats))
	for i, at := range ats {
		tenantIDs[i] = at.TenantID
	}

	newToken, err := GenerateToken(h.deps.JWTConfig, account.ID, account.Role, tenantIDs)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeAuthJSON(w, http.StatusOK, tokenResponse{Token: newToken})
}

func (h *authHandler) me(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromCtx(r.Context())
	if claims == nil {
		writeAuthError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	account, err := h.deps.AccountStore.GetByID(r.Context(), claims.AccountID)
	if err != nil {
		writeAuthError(w, http.StatusNotFound, "account not found")
		return
	}

	writeAuthJSON(w, http.StatusOK, meResponse{
		ID:          account.ID,
		Email:       account.Email,
		DisplayName: account.DisplayName,
		Role:        account.Role,
		Tenants:     claims.Tenants,
	})
}

func writeAuthJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	writeAuthJSON(w, status, map[string]string{"error": msg})
}
