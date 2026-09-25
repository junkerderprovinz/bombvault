package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// NewSPAHandler hands /api/, /metrics, /widget and /mcp to apiRouter, serves
// existing files from spaFS and answers every other path with index.html, so
// deep links and reloads work. Those three sit outside /api because Prometheus,
// embedding pages and MCP clients expect those URLs.
func NewSPAHandler(spaFS fs.FS, apiRouter http.Handler) http.Handler {
	fileServer := http.FileServerFS(spaFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The API router answers unknown /api/ paths with a 404 itself, and it
		// registers the exact /mcp only, so /mcp/x never gets the shell: its
		// answer is a 404, or a 401 while a login password is set.
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") ||
			r.URL.Path == "/metrics" || r.URL.Path == "/widget" ||
			r.URL.Path == mcpEndpointPath || strings.HasPrefix(r.URL.Path, mcpEndpointPath+"/") {
			apiRouter.ServeHTTP(w, r)
			return
		}

		// An MCP client that got a 401 asks for OAuth metadata next. The shell
		// would answer 200 with markup and the client would report a parse
		// error instead of "unauthorized"; BombVault has no such document.
		if strings.HasPrefix(r.URL.Path, "/.well-known/") {
			http.NotFound(w, r)
			return
		}

		if p := strings.TrimPrefix(r.URL.Path, "/"); p != "" {
			if f, err := spaFS.Open(p); err == nil {
				_ = f.Close()
				setCacheHeaders(w, p)
				fileServer.ServeHTTP(w, r)
				return
			}

			// Everything under assets/ is hashed build output, never a client
			// route. A stale index.html asking for an old chunk would otherwise
			// get HTML back and fail with "Unexpected token '<'".
			if strings.HasPrefix(p, "assets/") {
				http.NotFound(w, r)
				return
			}
		}

		serveIndex(w, spaFS)
	})
}

// setCacheHeaders lets browsers keep the files under assets/ for a year: Vite
// puts a content hash in their names, so a new build never changes an old
// file. index.html keeps its name and lists the current bundles, so it is never
// cached. Without a header the browser would guess from Last-Modified and keep
// loading the previous deploy.
func setCacheHeaders(w http.ResponseWriter, path string) {
	if path == "index.html" {
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		return
	}
	if strings.HasPrefix(path, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
}

// serveIndex writes index.html from spaFS as the SPA entry point.
func serveIndex(w http.ResponseWriter, spaFS fs.FS) {
	data, err := fs.ReadFile(spaFS, "index.html")
	if err != nil {
		http.Error(w, "SPA index not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Client-side routes reach the shell here, not through the file server.
	setCacheHeaders(w, "index.html")
	_, _ = w.Write(data)
}
