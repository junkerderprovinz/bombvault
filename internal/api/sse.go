package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// handleProgress streams live backup/restore progress as Server-Sent Events.
// The SPA opens a single EventSource on this endpoint and renders a per-target
// bar. Each message body is one progress.Event JSON object. A periodic comment
// line keeps idle connections (and any intermediary proxy) alive.
func (h *Handler) handleProgress(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // never let a reverse proxy buffer the stream

	if h.progress == nil {
		flusher.Flush() // valid (empty) event stream when progress is not wired
		return
	}

	ch, cancel := h.progress.Subscribe()
	defer cancel()

	// Replay current in-flight bars so a client connecting mid-operation sees them.
	for _, e := range h.progress.Snapshot() {
		writeSSEEvent(w, e)
	}
	flusher.Flush()

	ctx := r.Context()
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case e, open := <-ch:
			if !open {
				return
			}
			writeSSEEvent(w, e)
			flusher.Flush()
		case <-keepalive.C:
			_, _ = io.WriteString(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// writeSSEEvent writes one progress.Event as an SSE data frame.
func writeSSEEvent(w io.Writer, e progress.Event) {
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
}
