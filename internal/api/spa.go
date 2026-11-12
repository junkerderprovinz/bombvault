package api

import (
	"bytes"
	"html"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
)

// NewSPAHandler hands /api/, /metrics, /widget, /mcp and the OAuth endpoints
// to apiRouter, serves existing files from spaFS and answers every other path
// with index.html, so deep links and reloads work. The ones outside /api sit
// there because Prometheus, embedding pages and MCP clients expect those URLs.
// The consent page at /oauth/authorize is part of the shell.
func NewSPAHandler(spaFS fs.FS, apiRouter http.Handler) http.Handler {
	fileServer := http.FileServerFS(spaFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The API router answers unknown /api/ paths with a 404 itself, and it
		// registers the exact /mcp only, so /mcp/x never gets the shell: its
		// answer is a 404, or a 401 while a login password is set.
		switch p := r.URL.Path; {
		case p == "/api" || strings.HasPrefix(p, "/api/") ||
			p == "/metrics" || p == "/widget" || p == relayConnectPath ||
			p == mcpEndpointPath || strings.HasPrefix(p, mcpEndpointPath+"/"),
			p == oauthResourceMeta, p == oauthServerMetaPath,
			p == oauthRegisterPath, p == oauthTokenPath, p == oauthRevokePath:
			apiRouter.ServeHTTP(w, r)
			return
		}

		// An MCP client that got a 401 asks for OAuth metadata next. The shell
		// would answer 200 with markup and the client would report a parse
		// error instead of "unauthorized", so every other document there is a
		// 404.
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

		serveIndex(w, r, spaFS)
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

// indexBase is the base element index.html carries. Every address of the
// shell is relative to it, so that serveIndex can move the whole interface
// under a prefix.
var indexBase = []byte(`<base href="/"`)

// prefixPattern is a prefix BombVault can be served under: one path of plain
// segments that ends in a slash.
var prefixPattern = regexp.MustCompile(`^(/[A-Za-z0-9._~-]+)*/$`)

// forwardedPrefix is the path a proxy in front or the fortress serves
// BombVault under, as X-Forwarded-Prefix names it, and "/" without one or for
// one that is no plain path.
func forwardedPrefix(r *http.Request) string {
	p := strings.TrimSpace(r.Header.Get("X-Forwarded-Prefix"))
	if p != "" && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	if !prefixPattern.MatchString(p) {
		return "/"
	}
	return p
}

// serveIndex writes index.html from spaFS as the SPA entry point, its base at
// the prefix the request came under.
func serveIndex(w http.ResponseWriter, r *http.Request, spaFS fs.FS) {
	data, err := fs.ReadFile(spaFS, "index.html")
	if err != nil {
		http.Error(w, "SPA index not found", http.StatusInternalServerError)
		return
	}
	if p := forwardedPrefix(r); p != "/" {
		data = bytes.Replace(data, indexBase, []byte(`<base href="`+html.EscapeString(p)+`"`), 1)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Client-side routes reach the shell here, not through the file server.
	setCacheHeaders(w, "index.html")
	_, _ = w.Write(data)
}
