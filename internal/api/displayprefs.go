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
	// PER-AXIS MERGE, never a replace of the whole object.
	// -----------------------------------------------------------------------
	// A browser only knows the axes it has in its own localStorage, and a
	// browser whose site data was just cleared knows NONE of them. Replacing
	// meant that such a browser could publish its own emptiness: clear Firefox's
	// site data, touch one control before the page has reconciled with the
	// server, and that single axis became the entire stored look. Everything
	// else was gone, on the server as well, so reloading did not bring it back
	// and the user reconfigured from scratch. Reported as exactly that, three
	// times in a row, working in Brave and failing in Firefox because the two
	// differ in what they do to an open tab's running script when site data goes
	// (issue #191).
	//
	// Merging removes the whole class rather than narrowing the window: a PUT
	// says what it knows, never what it lacks. The axes are independent, each is
	// always written with an explicit value by its own setter, so there is no
	// case where "absent" has to mean "delete".
	//
	// The read and the write have to happen INSIDE MutateSettings, not around
	// it: reading the column here and writing it back through UpdateSettings
	// would revert every column another writer touched in between, and would
	// also lose a display-prefs write from a second tab that landed between the
	// two halves. A guard test in internal/store refuses the full-row writer in
	// production code for that reason.
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
		// Re-encode from the parsed form rather than storing the bytes as
		// received, so whatever lands in the column is known-canonical JSON and
		// cannot carry trailing junk that happened to parse.
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
