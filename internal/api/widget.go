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

// ---------------------------------------------------------------------------
// Embeddable dashboard widget — GET /widget (page) + GET /api/widget/data
// (feed) + POST/DELETE /api/widget/token (admin management).
//
// The widget is a tiny, self-contained, dark-only HTML page showing ONLY the
// mini activity log (the flat docker-logs list), meant to be iframed into
// Homepage/Organizr/Heimdall or any dashboard. Because an embedding iframe
// cannot carry the session cookie, both the page and its feed bypass authGate
// (public allowlist) and gate themselves on the stored widget token instead:
// no stored token (the default) = feature OFF = both answer 403, fail closed.
// The token grants READ-ONLY access to the activity log + schedule preview —
// nothing else.
// ---------------------------------------------------------------------------

// widgetPage is the self-contained widget HTML (inline CSS/JS, no SPA bundle —
// the 2 MB dist stays out of the iframe; this page is a few KB).
//
//go:embed widget.html
var widgetPage []byte

// widgetRunLimit caps the feed at roughly one tile-screen of history — the
// widget is a glanceable log, not the dashboard.
const widgetRunLimit = 40

// widgetErrorMax truncates run error text in the feed: a widget line can only
// show the head of an error anyway, and the full text stays in the app.
const widgetErrorMax = 200

// widgetTokenOK reports whether the request carries the stored widget token,
// via the X-Widget-Token header or the ?token= query parameter (the header
// wins when both are present). Constant-time compare; an EMPTY stored token
// always fails (feature off = fail closed), even for an empty presented one.
func widgetTokenOK(r *http.Request, stored string) bool {
	if stored == "" {
		return false
	}
	got := r.Header.Get("X-Widget-Token")
	if got == "" {
		got = r.URL.Query().Get("token")
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(stored)) == 1
}

// widgetGate loads settings and enforces the widget token, mirroring how
// /metrics self-gates inside its handler. It returns false after writing the
// refusal: 503 on a store error (fail closed, like authGate), 403 on a
// missing/mismatched token or when no token is stored (feature off).
func (h *Handler) widgetGate(w http.ResponseWriter, r *http.Request) bool {
	s, err := h.store.GetSettings()
	if err != nil {
		log.Printf("api: widget: settings read failed: %v", err)
		http.Error(w, "widget unavailable", http.StatusServiceUnavailable)
		return false
	}
	if !widgetTokenOK(r, s.WidgetToken) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

// handleWidgetPage serves the embeddable widget page (GET /widget?token=…).
// The framing headers are handled by securityHeaders (frame-ancestors * for
// exactly this path); everything dynamic comes from /api/widget/data, so the
// page bytes themselves are static.
func (h *Handler) handleWidgetPage(w http.ResponseWriter, r *http.Request) {
	if !h.widgetGate(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write(widgetPage); err != nil {
		log.Printf("api: widget: write page: %v", err)
	}
}

// widgetRun is the slim per-run row of GET /api/widget/data — just what the
// widget needs to compose its English log lines (see widget.html): no
// snapshot ids, no hooks, error text truncated. Domain vocabulary matches
// runView/activityLog.ts: items carry "container"/"vm"/"files"/"flash"/
// "config"; domain-scoped ops (prune/verify/offsite/drill/drdrill/tamper/
// export) carry the domain literal their run was recorded against
// ("containers"/"vms"/…, or the flash/config singleton).
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

// widgetDomainOpKind mirrors activityLog.ts's isDomainOpKind: these kinds are
// recorded against the reserved DOMAIN target id, so their TargetID is the
// domain literal itself rather than a resolvable item id.
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

// handleWidgetData serves the widget feed (GET /api/widget/data?token=…): the
// last widgetRunLimit runs (reusing ListRuns + the runView target resolution),
// the schedule-next preview and the app version — everything the page needs
// for one refresh in one round trip.
func (h *Handler) handleWidgetData(w http.ResponseWriter, r *http.Request) {
	if !h.widgetGate(w, r) {
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
			d = run.TargetID // domain-scoped op: the target id IS the domain literal
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

// handleWidgetTokenGenerate handles POST /api/widget/token — generates a fresh
// random 32-hex token, stores it (replacing any previous one — regenerate ==
// revoke old + issue new) and returns it ONCE. The token is never echoed
// again afterwards (settingsView follows the MetricsToken blank-and-report-
// is-set contract), so the Settings card shows the widget URL only right
// after generating. Session-protected via authGate: NOT on the public
// allowlist — only a logged-in admin can mint or rotate the token.
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

// handleWidgetTokenDisable handles DELETE /api/widget/token — clears the
// stored token, so both widget endpoints immediately fail closed (403) again.
// Session-protected like the generate endpoint.
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
