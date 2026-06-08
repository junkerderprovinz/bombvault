package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// NewSPAHandler returns an http.Handler that:
//   - delegates any /api/ request to apiRouter;
//   - serves a matching static file from spaFS for everything else;
//   - falls back to index.html for unmatched, non-/api/ routes so client-side
//     routing (deep links, refresh) works.
func NewSPAHandler(spaFS fs.FS, apiRouter http.Handler) http.Handler {
	fileServer := http.FileServerFS(spaFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API routes are owned by the JSON router (it 404s unknown routes itself,
		// so an unknown /api/ path never falls back to the SPA index).
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			apiRouter.ServeHTTP(w, r)
			return
		}

		// Serve an existing static asset directly.
		if p := strings.TrimPrefix(r.URL.Path, "/"); p != "" {
			if f, err := spaFS.Open(p); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Fallback: serve index.html for client-side routes.
		serveIndex(w, spaFS)
	})
}

// serveIndex writes index.html from spaFS as the SPA entry point.
func serveIndex(w http.ResponseWriter, spaFS fs.FS) {
	data, err := fs.ReadFile(spaFS, "index.html")
	if err != nil {
		http.Error(w, "SPA index not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
