package api

import (
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/group"
)

// NodeRouter is all a node answers: its health for the container's check,
// and the direct calls of its home instance when direct is not nil.
func NodeRouter(direct http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": Version})
	})
	if direct != nil {
		mux.Handle("POST "+group.DirectPath, direct)
	}
	return mux
}

// NewNodeServer returns a Server for a node, which has no pages to serve.
func NewNodeServer(cfg config.Config, router http.Handler) *Server {
	return &Server{cfg: cfg, handler: securityHeaders(router)}
}
