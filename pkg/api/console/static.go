package console

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// StaticHandler serves the SPA static files from the given directory.
// Known files are served directly; all other paths fall back to index.html for SPA routing.
func StaticHandler(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip API routes.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Try to serve the file directly.
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}

		// Fallback to index.html for SPA routing.
		indexPath := filepath.Join(dir, "index.html")
		if _, err := os.Stat(indexPath); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}
