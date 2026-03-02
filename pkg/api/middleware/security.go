package middleware

import (
	"net/http"
)

// SecurityConfig holds security header configuration.
type SecurityConfig struct {
	ContentSecurityPolicy   string
	XFrameOptions           string
	XContentTypeOptions     string
	StrictTransportSecurity string
	ReferrerPolicy          string
}

// DefaultSecurityConfig returns secure default configuration.
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		ContentSecurityPolicy:   "default-src 'self'",
		XFrameOptions:           "DENY",
		XContentTypeOptions:     "nosniff",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		ReferrerPolicy:          "strict-origin-when-cross-origin",
	}
}

// SecurityHeaders adds security headers to all responses.
func SecurityHeaders(cfg SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
			}
			if cfg.XFrameOptions != "" {
				w.Header().Set("X-Frame-Options", cfg.XFrameOptions)
			}
			if cfg.XContentTypeOptions != "" {
				w.Header().Set("X-Content-Type-Options", cfg.XContentTypeOptions)
			}
			if cfg.StrictTransportSecurity != "" && r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", cfg.StrictTransportSecurity)
			}
			if cfg.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
			}

			next.ServeHTTP(w, r)
		})
	}
}
