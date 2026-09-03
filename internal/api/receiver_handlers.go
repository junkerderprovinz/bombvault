package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ---------------------------------------------------------------------------
// Receiver dashboard CRUD + read-only monitoring endpoints
// ---------------------------------------------------------------------------
//
// A box that RECEIVES immutable off-site copies registers the received repo here
// and monitors it READ-ONLY (inventory, an independent restic check, dead-mans-
// switch + integrity alerts). Every endpoint is under the existing authGate. The
// stored SENDING APP_KEY is encrypted at rest (internal/secret) and NEVER returned
// in the clear — the view only reports whether a key is stored (hasAppKey).

// receivedRepoView is the JSON wire shape of a registered received repo's
// configuration + persisted last-check status. It deliberately carries NO app key
// (only hasAppKey) so the sending secret never leaves the box.
type receivedRepoView struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Repo              string `json:"repo"`
	DeadManHours      int    `json:"deadManHours"`
	CheckCadence      string `json:"checkCadence"`
	ReadDataPercent   int    `json:"readDataPercent"`
	LastCheckAt       int64  `json:"lastCheckAt"`
	LastCheckOK       *bool  `json:"lastCheckOk"` // null = never checked
	LastCheckError    string `json:"lastCheckError"`
	LastCheckReadData bool   `json:"lastCheckReadData"`
	Enabled           bool   `json:"enabled"`
	CreatedAt         int64  `json:"createdAt"`
	SortOrder         int    `json:"sortOrder"`
	HasAppKey         bool   `json:"hasAppKey"` // a sending key is stored (the key itself is never returned)
}

// receivedRepoStatus is a received repo's configuration/last-check view PLUS the
// live read-only status the list endpoint probes: the newest received snapshot
// time, the snapshot count, and whether the repo could be opened at all. The live
// fields are best-effort — an unreachable repo lists with reachable=false and an
// empty lastReceived rather than failing the whole list.
type receivedRepoStatus struct {
	receivedRepoView
	LastReceived  string `json:"lastReceived"`
	SnapshotCount int    `json:"snapshotCount"`
	Reachable     bool   `json:"reachable"`
}

// receivedRepoInput is the create/update request body. AppKey is the SENDING
// instance's 64-hex APP_KEY; on update an empty AppKey keeps the stored key
// (so an edit need not resend the secret). Enabled is a pointer so its absence is
// distinguishable from an explicit false.
type receivedRepoInput struct {
	Name            string `json:"name"`
	Repo            string `json:"repo"`
	AppKey          string `json:"appKey"`
	DeadManHours    int    `json:"deadManHours"`
	CheckCadence    string `json:"checkCadence"`
	ReadDataPercent int    `json:"readDataPercent"`
	Enabled         *bool  `json:"enabled"`
	SortOrder       int    `json:"sortOrder"`
}

func receivedRepoToView(rr store.ReceivedRepo) receivedRepoView {
	var ok *bool
	if rr.LastCheckOK.Valid {
		v := rr.LastCheckOK.Bool
		ok = &v
	}
	return receivedRepoView{
		ID:                rr.ID,
		Name:              rr.Name,
		Repo:              rr.Repo,
		DeadManHours:      rr.DeadManHours,
		CheckCadence:      rr.CheckCadence,
		ReadDataPercent:   rr.ReadDataPercent,
		LastCheckAt:       rr.LastCheckAt,
		LastCheckOK:       ok,
		LastCheckError:    rr.LastCheckError,
		LastCheckReadData: rr.LastCheckReadData,
		Enabled:           rr.Enabled,
		CreatedAt:         rr.CreatedAt,
		SortOrder:         rr.SortOrder,
		HasAppKey:         len(rr.AppKeyEnc) > 0,
	}
}

// validateReceiverCadence normalizes a check cadence. Empty defaults to a daily
// check; "off" disables scheduled checks; any other value must parse as a schedule
// cadence. Returns (normalized, "") on success or ("", message) on an invalid one.
func validateReceiverCadence(c string) (string, string) {
	c = strings.TrimSpace(c)
	if c == "" {
		return "daily 04:00", "" // task default: a daily independent check
	}
	if strings.EqualFold(c, "off") {
		return "off", ""
	}
	if _, err := schedule.ParseCadence(c); err != nil {
		return "", "invalid check cadence: use off, daily HH:MM, weekly DOW HH:MM, everyN N HH:MM, or a 5-field cron"
	}
	return c, ""
}

// buildReceivedRepo validates in and folds it onto existing (the zero value on
// create), returning the row to persist or a user-facing error message. The app
// key is validated (64 lowercase hex, the same shape the foreign flow enforces)
// and encrypted at rest with THIS instance's APP_KEY before it ever reaches the
// store; on update an empty AppKey preserves existing.AppKeyEnc. Identity and
// last-check columns are carried from existing so an edit never re-stamps them.
func (h *Handler) buildReceivedRepo(in receivedRepoInput, existing store.ReceivedRepo, isCreate bool) (store.ReceivedRepo, string) {
	repo := strings.TrimSpace(in.Repo)
	if repo == "" {
		return store.ReceivedRepo{}, "repo must not be empty"
	}

	rr := existing
	rr.Name = strings.TrimSpace(in.Name)
	rr.Repo = repo

	key := strings.TrimSpace(in.AppKey)
	switch {
	case key == "" && isCreate:
		return store.ReceivedRepo{}, "the sending APP_KEY is required (64 lowercase hex characters)"
	case key == "":
		// Update with no new key: keep the stored ciphertext untouched.
		rr.AppKeyEnc = existing.AppKeyEnc
	default:
		if !foreignKeyRe.MatchString(key) {
			return store.ReceivedRepo{}, "the sending APP_KEY must be exactly 64 lowercase hex characters"
		}
		enc, err := secret.Encrypt(h.cfg.AppKey, []byte(key))
		if err != nil {
			return store.ReceivedRepo{}, "could not encrypt the sending APP_KEY"
		}
		rr.AppKeyEnc = enc
	}

	cadence, msg := validateReceiverCadence(in.CheckCadence)
	if msg != "" {
		return store.ReceivedRepo{}, msg
	}
	rr.CheckCadence = cadence

	rr.DeadManHours = in.DeadManHours
	if rr.DeadManHours <= 0 {
		rr.DeadManHours = 26 // task default; matches the store column default
	}
	rr.ReadDataPercent = in.ReadDataPercent
	if rr.ReadDataPercent < 0 {
		rr.ReadDataPercent = 0
	}
	if rr.ReadDataPercent > 100 {
		rr.ReadDataPercent = 100
	}
	rr.SortOrder = in.SortOrder

	if in.Enabled != nil {
		rr.Enabled = *in.Enabled
	} else if isCreate {
		rr.Enabled = true
	}
	return rr, ""
}

// handleListReceiverRepos lists every registered received repo with its persisted
// last-check status AND a live read-only probe of the newest received snapshot.
// GET /api/receiver/repos. Never returns a decrypted app key.
func (h *Handler) handleListReceiverRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := h.store.ListReceivedRepos()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	out := make([]receivedRepoStatus, 0, len(repos))
	for _, rr := range repos {
		st := receivedRepoStatus{receivedRepoView: receivedRepoToView(rr)}
		// Best-effort live status: opening a received repo read-only can be slow or
		// fail (backend down, wrong key). A failure lists the repo as unreachable
		// rather than sinking the whole list.
		if newest, count, pErr := h.svc.receiverNewest(r.Context(), rr); pErr == nil {
			st.LastReceived = newest
			st.SnapshotCount = count
			st.Reachable = true
		}
		out = append(out, st)
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"repos": out}))
}

// handleCreateReceiverRepo registers a received repo. POST /api/receiver/repos.
// The app key shape is validated AND the repo must OPEN read-only (a `restic cat
// config` probe, never an init) before the row is saved — a repo that cannot be
// opened is rejected so a mistyped location/key never lands in the dashboard.
func (h *Handler) handleCreateReceiverRepo(w http.ResponseWriter, r *http.Request) {
	var in receivedRepoInput
	if !decodeBody(w, r, &in) {
		return
	}
	rr, msg := h.buildReceivedRepo(in, store.ReceivedRepo{}, true)
	if msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if err := h.svc.receiverProbe(r.Context(), rr); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	stored, err := h.store.CreateReceivedRepo(rr)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"repo": receivedRepoToView(stored)}))
}

// handleUpdateReceiverRepo edits a received repo in place. PUT /api/receiver/
// repos/{id}. The id comes from the path; identity + last-check columns are
// preserved. As with create, the (possibly key-changed) repo must open read-only
// before the edit is saved.
func (h *Handler) handleUpdateReceiverRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, ok, err := h.store.GetReceivedRepo(id)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such received repo"})
		return
	}
	var in receivedRepoInput
	if !decodeBody(w, r, &in) {
		return
	}
	rr, msg := h.buildReceivedRepo(in, existing, false)
	if msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if err := h.svc.receiverProbe(r.Context(), rr); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.store.UpdateReceivedRepo(rr); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"repo": receivedRepoToView(rr)}))
}

// handleDeleteReceiverRepo removes a received repo's DB row and its dead-mans-
// switch episode state. DELETE /api/receiver/repos/{id}. It NEVER touches the
// received repository itself (this box only ever read it). A missing id is a
// harmless no-op.
func (h *Handler) handleDeleteReceiverRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteReceivedRepo(id); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.store.DeleteReceivedAlertStatesForRepo(id); err != nil {
		// Non-fatal: the row is gone; a leftover episode row would at worst suppress
		// one future alert for a since-deleted repo. Log and still report success.
		log.Printf("api: receiver: delete alert state for %s: %v", id, err)
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleReceiverInventory returns the read-only snapshot inventory (grouped by
// source) of one received repo. GET /api/receiver/repos/{id}/inventory.
func (h *Handler) handleReceiverInventory(w http.ResponseWriter, r *http.Request) {
	rr, ok, err := h.store.GetReceivedRepo(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such received repo"})
		return
	}
	inv, err := h.svc.receiverInventory(r.Context(), rr)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"inventory": inv}))
}

// handleReceiverCheck runs an INDEPENDENT restic check on one received repo now,
// persists the verdict, and returns it. POST /api/receiver/repos/{id}/check
// (optional ?readData=true forces the deep --read-data-subset check when a
// percent is configured). This manual check does not itself fire an integrity
// alert (mirroring a manual backup vs the watchdog): it records the verdict, and
// the scheduled receiver run drives the debounced alerting off that state.
func (h *Handler) handleReceiverCheck(w http.ResponseWriter, r *http.Request) {
	rr, ok, err := h.store.GetReceivedRepo(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such received repo"})
		return
	}
	readData := r.URL.Query().Get("readData") == "true"
	// Refused, not queued, when one is already running — a second click used to
	// start a second `restic check` on the same repository, and with
	// --read-data-subset that is a second pack-data read. Received repos are
	// outside repoMu, so nothing else was stopping it.
	res, ran := h.svc.receiverCheckExclusive(r.Context(), rr, readData)
	if !ran {
		writeJSON(w, http.StatusConflict, failEnvelope(errReceiverCheckBusy))
		return
	}
	if err := h.store.UpdateReceivedRepoCheckResult(rr.ID, res.At, nullCheckOK(res.OK), res.Error, res.RanReadData); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"result": res}))
}

// receiverProbe opens a received repo read-only (the create/update guard) and
// discards the handle: it succeeds only when the location + key open a real restic
// repository, and never initializes one.
func (s *Service) receiverProbe(ctx context.Context, rr store.ReceivedRepo) error {
	_, _, err := s.receiverOpen(ctx, rr)
	return err
}

// receiverNewest opens a received repo read-only and returns the newest snapshot
// time (raw, as restic reports it) and the total snapshot count — the live status
// the list endpoint shows. Read-only; no stats, so it stays cheap enough to run
// per repo on a list call.
func (s *Service) receiverNewest(ctx context.Context, rr store.ReceivedRepo) (string, int, error) {
	repo, mode, err := s.receiverOpen(ctx, rr)
	if err != nil {
		return "", 0, err
	}
	snaps, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return "", 0, err
	}
	var newestRaw string
	var newestParsed time.Time
	for _, sn := range snaps {
		when := parseSnapshotTime(sn.Time)
		if newestRaw == "" || when.After(newestParsed) {
			newestParsed = when
			newestRaw = sn.Time
		}
	}
	return newestRaw, len(snaps), nil
}
