package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Display preferences keep the look of the interface on the server, so it
// survives a browser clearing its site data. They have their own endpoint
// because PUT /api/settings replaces the whole settings object. The payload is
// an opaque JSON object whose keys belong to the frontend; the server only
// checks that it is an object of reasonable size.

// maxDisplayPrefsBytes caps the stored blob; the real payload is around 300
// bytes.
const maxDisplayPrefsBytes = 16 << 10

// handleGetDisplayPrefs serves GET /api/display-prefs. With nothing stored it
// returns an empty object and stored:false, so the client seeds the server from
// the browser instead of resetting to defaults.
func (h *Handler) handleGetDisplayPrefs(w http.ResponseWriter, _ *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		log.Printf("api: display prefs: settings read failed: %v", err)
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	raw := s.DisplayPrefs
	if raw == "" {
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"prefs": map[string]any{}, "stored": false}))
		return
	}
	var prefs map[string]any
	if err := json.Unmarshal([]byte(raw), &prefs); err != nil {
		// Treat an unreadable value as nothing stored: the page must still
		// render, and the next save replaces it.
		log.Printf("api: display prefs: stored value is not a JSON object, ignoring it: %v", err)
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"prefs": map[string]any{}, "stored": false}))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"prefs": prefs, "stored": true}))
}

// handlePutDisplayPrefs serves PUT /api/display-prefs. The body is the
// preferences object itself, not an envelope.
func (h *Handler) handlePutDisplayPrefs(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxDisplayPrefsBytes+1))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if len(body) > maxDisplayPrefsBytes {
		http.Error(w, "display preferences too large", http.StatusRequestEntityTooLarge)
		return
	}
	// Must be an object: a bare number or string would break the client that
	// spreads it.
	var prefs map[string]any
	if err := json.Unmarshal(body, &prefs); err != nil {
		http.Error(w, "display preferences must be a JSON object", http.StatusBadRequest)
		return
	}
	// Merge per key instead of replacing. A browser whose site data was just
	// cleared knows none of the keys, and a replace would let it wipe the
	// stored look. Every key is always written with an explicit value, so an
	// absent key never means delete.
	//
	// Reading and writing inside MutateSettings keeps a concurrent settings
	// write, or a second tab's save, from being lost in between.
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		merged := map[string]any{}
		if s.DisplayPrefs != "" {
			// An unreadable stored value is replaced rather than merged into:
			// it is already lost, and refusing the write would strand the client.
			if err := json.Unmarshal([]byte(s.DisplayPrefs), &merged); err != nil {
				log.Printf("api: display prefs: stored value is not a JSON object, replacing it: %v", err)
				merged = map[string]any{}
			}
		}
		for k, v := range prefs {
			merged[k] = v
		}
		// Re-encode instead of storing the received bytes, so the column holds
		// canonical JSON.
		canonical, mErr := json.Marshal(merged)
		if mErr != nil {
			return mErr
		}
		s.DisplayPrefs = string(canonical)
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"stored": true}))
}
