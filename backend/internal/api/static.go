package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// mountStatic serves the built SvelteKit SPA from cfg.StaticDir, falling back to
// index.html for client-side routes (SPA history mode).
func (s *Server) mountStatic(r chi.Router) {
	root := s.cfg.StaticDir
	fileServer := http.FileServer(http.Dir(root))

	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		// API routes are handled elsewhere; never fall through to the SPA.
		if strings.HasPrefix(req.URL.Path, "/api/") {
			http.NotFound(w, req)
			return
		}

		clean := filepath.Clean(req.URL.Path)
		candidate := filepath.Join(root, clean)

		// Serve an existing static asset directly.
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, req)
			return
		}

		// Otherwise serve the SPA entrypoint so the client router can take over.
		http.ServeFile(w, req, filepath.Join(root, "index.html"))
	})
}
