package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Display preferences: the look of the interface, kept on the server so it
// survives a browser (issue #191).
// ---------------------------------------------------------------------------
// Reported by manilx: clearing Firefox's site data fixes an unrelated problem
// with the VM VNC console, and every time he does it BombVault forgets its
// theme, its view mode and everything else, because all of that lived in
// localStorage. His argument for moving it server-side is that in nearly every
// installation exactly one person configures BombVault, which is true, and the
// per-browser split was never a decision so much as the default of where a
// frontend puts things.
//
// This is a SEPARATE endpoint rather than fields on PUT /api/settings, and
// deliberately so: that handler is a full-object replace over ~86 fields, so a
// theme toggle would have to send the entire configuration back, and two tabs
// toggling different things would each overwrite the other's unrelated fields.
// A dedicated endpoint touches one column.
//
// The payload is stored and returned VERBATIM as an opaque JSON object. The
// server does not know what a "theme" is and should not learn: the axes are the
// frontend's, they grow with the interface, and a backend that validated them
// would need a release every time a new one appears. What it does enforce is
// that the body is a JSON OBJECT and is not unreasonably large, so the column
// cannot be turned into free storage or fed something that later breaks a
// client trying to parse it.

// maxDisplayPrefsBytes caps the stored blob. The real payload is around 300
// bytes; the cap is generous enough that no plausible growth of the interface
// hits it, and small enough that this column cannot become a place to park data.
const maxDisplayPrefsBytes = 16 << 10

// handleGetDisplayPrefs serves the stored look, or an empty object when nothing
// has been stored yet. GET /api/display-prefs
//
// An empty object rather than an error or a null is what lets the client tell
// "the server has no opinion, seed me from this browser" apart from "the server
// says the defaults", which is the difference between keeping a user's current
// look on upgrade and resetting it.
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
		// Stored by an older or broken client. Report it as "nothing stored"
		// rather than failing the request: the page must still render, and the
		// next save overwrites the unreadable value anyway.
		log.Printf("api: display prefs: stored value is not a JSON object, ignoring it: %v", err)
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"prefs": map[string]any{}, "stored": false}))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"prefs": prefs, "stored": true}))
}

// handlePutDisplayPrefs stores the look. PUT /api/display-prefs
// The body is the object itself, not an envelope around it.
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
	// Must be an OBJECT. A bare number or string would round-trip through the
	// column fine and then break the client that expects to spread it.
	var prefs map[string]any
	if err := json.Unmarshal(body, &prefs); err != nil {
		http.Error(w, "display preferences must be a JSON object", http.StatusBadRequest)
		return
	}
	// Re-encode from the parsed form rather than storing the bytes as received,
	// so whatever lands in the column is known-canonical JSON and cannot carry
	// trailing junk that happened to parse.
	canonical, err := json.Marshal(prefs)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	// MutateSettings, never a read-modify-write around UpdateSettings: the
	// latter writes the whole row, so it would revert every column another
	// writer touched in between. A settings page saving a schedule while a
	// second tab flips the theme is exactly the collision this endpoint invites,
	// and there is a guard test in internal/store that refuses the full-row
	// writer in production code for precisely that reason.
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.DisplayPrefs = string(canonical)
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"stored": true}))
}
