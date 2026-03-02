package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery recovers from panics and returns a 500 error.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace
				requestID := GetRequestID(r.Context())
				slog.Error("panic recovered",
					"error", err,
					"request_id", requestID,
					"path", r.URL.Path,
					"method", r.Method,
					"stack", string(debug.Stack()),
				)

				// Return 500 error
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, `{"error":{"code":"INTERNAL_ERROR","message":"Internal server error","request_id":"%s"}}`, requestID)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
