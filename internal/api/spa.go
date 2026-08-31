package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// NewSPAHandler returns an http.Handler that:
//   - delegates any /api/ request to apiRouter;
//   - delegates the API-owned non-/api pages (/metrics, /widget) to apiRouter
//     too — they live outside /api by design (Prometheus scrape convention /
//     an embeddable page URL) and must never fall back to the SPA index;
//   - serves a matching static file from spaFS for everything else;
//   - falls back to index.html for unmatched, non-/api/ routes so client-side
//     routing (deep links, refresh) works.
func NewSPAHandler(spaFS fs.FS, apiRouter http.Handler) http.Handler {
	fileServer := http.FileServerFS(spaFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API routes are owned by the JSON router (it 404s unknown routes itself,
		// so an unknown /api/ path never falls back to the SPA index).
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") ||
			r.URL.Path == "/metrics" || r.URL.Path == "/widget" {
			apiRouter.ServeHTTP(w, r)
			return
		}

		// Serve an existing static asset directly.
		if p := strings.TrimPrefix(r.URL.Path, "/"); p != "" {
			if f, err := spaFS.Open(p); err == nil {
				_ = f.Close()
				setCacheHeaders(w, p)
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Fallback: serve index.html for client-side routes.
		serveIndex(w, spaFS)
	})
}

// setCacheHeaders splits the bundle into the two things it actually is ([333]).
//
// Vite gives every built asset a content hash in its name, so
// `assets/index-B7f2c9.js` is immutable by construction: a new build produces a
// new NAME, never new bytes under the old one. Those can be cached for a year
// and it is free.
//
// index.html is the opposite and must never be cached. It is the one file whose
// name never changes and whose content changes on every deploy, since it
// carries the script tags naming the current hashed bundles. Cached, a browser
// keeps loading yesterday's shell, which asks for yesterday's bundles, and the
// app looks not-updated in a way no amount of deploying fixes.
//
// This is not hypothetical here. Verifying a deploy on 2026-08-31, the page
// reported sha-9f627b5 while sha-f745c04 was demonstrably running and healthy,
// and the same confusion had already cost a round of chasing a "bug" that was a
// stale page ([266]). Without an explicit header a browser applies its own
// heuristic to a 200 with a Last-Modified and no max-age, which is precisely
// where "it works on my machine after a hard reload" comes from.
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
	// Every client-side route lands here, so this is the path that actually
	// serves the shell in normal use - /index.html by name is the rarer one.
	// Both need it; setting it in only one was the first cut of this fix.
	setCacheHeaders(w, "index.html")
	_, _ = w.Write(data)
}
