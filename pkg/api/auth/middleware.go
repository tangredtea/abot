package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const claimsKey contextKey = "auth_claims"

// AuthMiddleware returns an HTTP middleware that validates JWT tokens
// and injects claims into the request context.
func AuthMiddleware(cfg JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearerToken(r)
			if tokenStr == "" {
				http.Error(w, `{"error":"missing authorization token"}`, http.StatusUnauthorized)
				return
			}
			claims, err := ValidateToken(cfg, tokenStr)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromCtx extracts the JWT claims from a request context.
// Returns nil if no claims are present.
func ClaimsFromCtx(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

// AccountFromCtx returns the account ID from the request context.
func AccountFromCtx(ctx context.Context) string {
	c := ClaimsFromCtx(ctx)
	if c == nil {
		return ""
	}
	return c.AccountID
}

// TenantsFromCtx returns the tenant IDs from the request context.
func TenantsFromCtx(ctx context.Context) []string {
	c := ClaimsFromCtx(ctx)
	if c == nil {
		return nil
	}
	return c.Tenants
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}
