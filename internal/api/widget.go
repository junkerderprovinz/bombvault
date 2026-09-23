package api

import (
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"log"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The embeddable dashboard widget is a small page showing the activity log,
// meant to be iframed into Homepage, Organizr, Heimdall and the like. An
// iframe cannot carry the session cookie, so the page and its feed skip
// authGate and check the stored widget token instead; with no token stored,
// both answer 403. The token grants read access to the activity log and the
// schedule preview, nothing else.

// widgetPage is the widget HTML with inline CSS and JS, so the iframe does not
// load the SPA bundle.
//
//go:embed widget.html
var widgetPage []byte

// widgetRunLimit caps the feed at about one screen of history.
const widgetRunLimit = 40

// widgetErrorMax truncates run errors in the feed; a widget line shows only
// the start of an error, and the full text stays in the app.
const widgetErrorMax = 200

// widgetTokenOK reports whether the X-Widget-Token header matches the stored
// token. An empty stored token never matches. Only the page accepts the token
// in the query (widgetPageTokenOK): an iframe cannot set a header on the
// document request, but the page's own fetches can, which keeps the token out
// of proxy access logs on every poll.
func widgetTokenOK(r *http.Request, stored string) bool {
	if stored == "" {
		return false
	}
	got := r.Header.Get("X-Widget-Token")
	return subtle.ConstantTimeCompare([]byte(got), []byte(stored)) == 1
}

// widgetPageTokenOK is widgetTokenOK that also accepts ?token=, for the page
// only.
func widgetPageTokenOK(r *http.Request, stored string) bool {
	if widgetTokenOK(r, stored) {
		return true
	}
	if stored == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("token")), []byte(stored)) == 1
}

// widgetGate checks the widget token with ok and reports false after writing
// the refusal: 503 when settings cannot be read, 403 when the token is
// missing, wrong or not set.
func (h *Handler) widgetGate(w http.ResponseWriter, r *http.Request, ok func(*http.Request, string) bool) bool {
	s, err := h.store.GetSettings()
	if err != nil {
		log.Printf("api: widget: settings read failed: %v", err)
		http.Error(w, "widget unavailable", http.StatusServiceUnavailable)
		return false
	}
	if !ok(r, s.WidgetToken) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

// handleWidgetPage serves GET /widget. securityHeaders allows framing for this
// path, and everything dynamic comes from /api/widget/data.
func (h *Handler) handleWidgetPage(w http.ResponseWriter, r *http.Request) {
	if !h.widgetGate(w, r, widgetPageTokenOK) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write(widgetPage); err != nil {
		log.Printf("api: widget: write page: %v", err)
	}
}

// widgetRun is one run in the widget feed, with only what widget.html needs
// for its log lines. Domain follows runView and activityLog.ts: items carry
// "container", "vm", "files", "flash" or "config", and domain-scoped
// operations carry the domain their run was recorded against.
type widgetRun struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt *int64 `json:"finishedAt"`
	Target     string `json:"target"`
	Domain     string `json:"domain"`
	Bytes      int64  `json:"bytes"`
	Error      string `json:"error"`
}

// widgetDomainOpKind mirrors isDomainOpKind in activityLog.ts. These kinds are
// recorded against the domain, so their TargetID is the domain name rather
// than an item id.
func widgetDomainOpKind(kind string) bool {
	switch kind {
	case "prune", "verify", "offsite", "drill", "drdrill", "tamper", "export":
		return true
	}
	return false
}

// truncateWidgetError caps an error message for the slim feed.
func truncateWidgetError(msg string) string {
	if len(msg) <= widgetErrorMax {
		return msg
	}
	return msg[:widgetErrorMax] + "…"
}

// handleWidgetData serves GET /api/widget/data: the last widgetRunLimit runs,
// the upcoming scheduled runs and the app version, all the page needs for one
// refresh.
func (h *Handler) handleWidgetData(w http.ResponseWriter, r *http.Request) {
	if !h.widgetGate(w, r, widgetTokenOK) {
		return
	}
	runs, err := h.store.ListRuns(widgetRunLimit)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	name, domain := h.runTargetMaps()
	views := make([]widgetRun, 0, len(runs))
	for _, run := range runs {
		d := domain[run.TargetID]
		if d == "" && widgetDomainOpKind(run.Kind) {
			d = run.TargetID
		}
		views = append(views, widgetRun{
			ID:         run.ID,
			Kind:       run.Kind,
			Status:     run.Status,
			StartedAt:  run.StartedAt,
			FinishedAt: run.FinishedAt,
			Target:     name[run.TargetID],
			Domain:     d,
			Bytes:      run.Bytes,
			Error:      truncateWidgetError(run.Error),
		})
	}
	var next []schedule.NextRun
	if h.scheduler != nil {
		next = h.scheduler.NextRuns()
	}
	if next == nil {
		next = []schedule.NextRun{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"version": Version,
		"runs":    views,
		"next":    next,
	}))
}

// handleWidgetTokenGenerate handles POST /api/widget/token. It stores a new
// random token, which revokes the previous one, and returns it. settingsView
// never returns the token, so the Settings card can show the widget URL only
// right after generating. Unlike the widget itself, this endpoint is behind
// authGate.
func (h *Handler) handleWidgetTokenGenerate(w http.ResponseWriter, _ *http.Request) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	token := hex.EncodeToString(buf)
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.WidgetToken = token
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"token": token}))
}

// handleWidgetTokenDisable handles DELETE /api/widget/token. Without a stored
// token both widget endpoints answer 403.
func (h *Handler) handleWidgetTokenDisable(w http.ResponseWriter, _ *http.Request) {
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.WidgetToken = ""
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
