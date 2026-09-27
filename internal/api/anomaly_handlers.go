package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The HTTP surface of anomaly detection. Everything here parses a request and
// wraps an answer; the rules live in the engine and the numbers in the service.

// anomalyClosedWindow is how far back a listing of settled findings looks when
// the caller names no start. Open rows are never limited by time.
const anomalyClosedWindow = int64(30 * 24 * 3600)

var (
	anomalyStates        = []string{"open", "resolved", "acknowledged", "expected"}
	anomalyClosed        = []string{"resolved", "acknowledged", "expected"}
	anomalySeverities    = []string{"critical", "warning", "info"}
	anomalyDetectorNames = []string{
		detectorNewData, detectorSource, detectorDuration,
		detectorReliability, detectorIntegrity, detectorCapacity,
	}
	// anomalyFilterDomains holds both spellings: a row of an item carries the
	// singular domain, a domain or volume row the plural one.
	anomalyFilterDomains = []string{
		"container", "containers", "vm", "vms", "files", "zfs", "flash", "config",
	}
)

func (h *Handler) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	filter, err := anomalyFilterFrom(r.URL.Query(), time.Now().Unix())
	if err != nil {
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, "bad-filter"))
		return
	}
	page, err := h.svc.ListAnomalies(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"anomalies": page.Anomalies, "nextCursor": page.NextCursor,
	}))
}

func (h *Handler) handleAnomalySummary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"summary": h.svc.AnomalySummary(r.Context())}))
}

func (h *Handler) handleAnomaly(w http.ResponseWriter, r *http.Request) {
	view, found, err := h.svc.GetAnomaly(r.Context(), r.PathValue("id"))
	switch {
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	case !found:
		writeJSON(w, http.StatusOK, codedFailEnvelope(errors.New("no such entry"), "not-found"))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"anomaly": view}))
	}
}

// anomalyAction is the body of the two bulk actions.
type anomalyAction struct {
	IDs  []string `json:"ids"`
	Note string   `json:"note"`
}

func (h *Handler) handleAcknowledgeAnomalies(w http.ResponseWriter, r *http.Request) {
	h.settleAnomalies(w, r, h.svc.AcknowledgeAnomalies)
}

func (h *Handler) handleAnomaliesExpected(w http.ResponseWriter, r *http.Request) {
	h.settleAnomalies(w, r, h.svc.MarkAnomaliesExpected)
}

func (h *Handler) settleAnomalies(w http.ResponseWriter, r *http.Request, settle anomalySettleFunc) {
	var body anomalyAction
	if !decodeBody(w, r, &body) {
		return
	}
	if len(body.IDs) == 0 || len(body.IDs) > anomalyActionLimit {
		writeJSON(w, http.StatusOK, codedFailEnvelope(
			fmt.Errorf("name between 1 and %d findings", anomalyActionLimit), "bad-request"))
		return
	}
	changed, skipped, released, err := settle(r.Context(), body.IDs, body.Note)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"changed": changed, "skipped": skipped, "released": released,
	}))
}

type anomalySettleFunc func(ctx context.Context, ids []string, note string) (changed, skipped, released int, err error)

func (h *Handler) handleAnomalyItems(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.AnomalyItems(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if items == nil {
		items = []AnomalyItem{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"items": items}))
}

func (h *Handler) handleAnomalyItemPrefs(w http.ResponseWriter, r *http.Request) {
	var patch AnomalyPrefsPatch
	if !decodeBody(w, r, &patch) {
		return
	}
	err := h.svc.SetItemAnomalyPrefs(r.Context(), r.PathValue("targetId"), patch)
	writeJSON(w, http.StatusOK, anomalyWriteEnvelope(err))
}

func (h *Handler) handleForgetAnomalyExpectation(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = anomalyScopeItem
	}
	part := r.URL.Query().Get("part")
	if scope == anomalyScopeZFSDS && part == "" {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errors.New("name the dataset"), "bad-request"))
		return
	}
	err := h.svc.ForgetAnomalyExpectation(r.Context(), r.PathValue("targetId"), scope, part, r.PathValue("family"))
	writeJSON(w, http.StatusOK, anomalyWriteEnvelope(err))
}

// anomalyWriteEnvelope gives a refused write the code its cause deserves, so
// the page can tell an item that is gone from a value it must not send.
func anomalyWriteEnvelope(err error) map[string]any {
	switch {
	case err == nil:
		return okEnvelope(nil)
	case errors.Is(err, errNotAnAnomalyItem):
		return codedFailEnvelope(err, "not-found")
	case errors.Is(err, errUnknownSensitivity), errors.Is(err, errUnknownNotifyMin),
		errors.Is(err, errUnknownAnomalyScope), errors.Is(err, errUnknownAnomalyFamily):
		return codedFailEnvelope(err, "bad-request")
	default:
		return failEnvelope(err)
	}
}

// anomalyFilterFrom reads one listing's query. Every enumerated value is
// checked here, so an unknown one is a refusal and not an empty page.
func anomalyFilterFrom(q url.Values, now int64) (store.AnomalyFilter, error) {
	f := store.AnomalyFilter{Cursor: q.Get("cursor")}

	states, err := anomalyStateFilter(q.Get("state"))
	if err != nil {
		return store.AnomalyFilter{}, err
	}
	f.States = states
	if f.Severities, err = anomalyValues(q.Get("severity"), "severity", anomalySeverities); err != nil {
		return store.AnomalyFilter{}, err
	}
	if f.Detectors, err = anomalyValues(q.Get("detector"), "detector", anomalyDetectorNames); err != nil {
		return store.AnomalyFilter{}, err
	}
	if f.Domains, err = anomalyValues(q.Get("domain"), "domain", anomalyFilterDomains); err != nil {
		return store.AnomalyFilter{}, err
	}
	if err := anomalyScopeFilter(&f, q.Get("scope")); err != nil {
		return store.AnomalyFilter{}, err
	}

	if raw := q.Get("limit"); raw != "" {
		limit, cErr := strconv.Atoi(raw)
		if cErr != nil || limit < 1 || limit > store.MaxAnomalyLimit {
			return store.AnomalyFilter{}, fmt.Errorf("limit %q is not between 1 and %d", raw, store.MaxAnomalyLimit)
		}
		f.Limit = limit
	}

	openOnly := len(f.States) == 0 || (len(f.States) == 1 && f.States[0] == "open")
	switch raw := q.Get("since"); {
	case openOnly:
	case raw == "":
		f.Since = now - anomalyClosedWindow
	default:
		since, cErr := strconv.ParseInt(raw, 10, 64)
		if cErr != nil || since < 0 {
			return store.AnomalyFilter{}, fmt.Errorf("since %q is not a time", raw)
		}
		f.Since = since
	}
	return f, nil
}

func anomalyStateFilter(raw string) ([]string, error) {
	switch raw {
	case "", "open":
		return nil, nil
	case "all":
		return slices.Clone(anomalyStates), nil
	case "closed":
		return slices.Clone(anomalyClosed), nil
	}
	return anomalyValues(raw, "state", anomalyStates)
}

// anomalyScopeFilter narrows a listing to one series. An item is named by its
// id and matches every series of it, including its dumps and its datasets.
func anomalyScopeFilter(f *store.AnomalyFilter, raw string) error {
	if raw == "" {
		return nil
	}
	kind, id, ok := strings.Cut(raw, ":")
	if !ok || id == "" {
		return fmt.Errorf("scope %q names no series", raw)
	}
	switch kind {
	case anomalyScopeItem:
		f.TargetID = id
	case anomalyScopeDomain, anomalyScopeVolume:
		f.ScopeKind, f.ScopeID = kind, id
	default:
		return fmt.Errorf("unknown scope %q", raw)
	}
	return nil
}

func anomalyValues(raw, name string, allowed []string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	values := strings.Split(raw, ",")
	for _, v := range values {
		if !slices.Contains(allowed, v) {
			return nil, fmt.Errorf("unknown %s %q", name, v)
		}
	}
	return values, nil
}
