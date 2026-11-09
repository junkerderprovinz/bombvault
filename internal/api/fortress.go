package api

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fortressNeedsLimit keeps a report under the contract's size when much has
// gone wrong at once.
const fortressNeedsLimit = 50

// fortressNextLimit is how many planned runs a report names.
const fortressNextLimit = 20

// registerFortress serves the status the other apps of a household read about
// this one: how its backups stand, what needs the person, what runs and what
// comes next, and the finished runs as its log.
func (h *Handler) registerFortress(mux *http.ServeMux) {
	if h.mcp == nil {
		h.mcp = newMCPState()
	}
	fortress := status.Handler(fortressSource{h}, h.authorizeFortress)
	mux.Handle(status.StatusPath, fortress)
	mux.Handle(status.LogPath, fortress)
}

// authorizeFortress lets through a browser signed in with the password and a
// request carrying an API token. An MCP key is for an assistant and opens
// nothing here.
func (h *Handler) authorizeFortress(r *http.Request) error {
	if foreignOrigin(r) {
		return status.ErrForbidden
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		return err
	}
	if hash := settings.AuthPasswordHash; hash != "" {
		if c, err := h.sessionCookie(r); err == nil && !secret.PairedSessionToken(c.Value) &&
			secret.ValidSessionToken(h.cfg.AppKey, hash, settings.SessionEpoch, c.Value) {
			return nil
		}
	}
	presented, present, conflict := presentedMCPKey(r)
	if !present {
		return status.ErrUnauthenticated
	}
	addr := h.loginClientKey(r)
	bucket := "api|" + addr
	if h.loginThrottled(bucket) {
		return fmt.Errorf("%w: too many failed attempts from %s", status.ErrUnauthenticated, addr)
	}
	tokens, err := h.store.ActiveAPITokens()
	if err != nil {
		return err
	}
	k, match := matchMCPKey(secret.HashMCPKey(h.cfg.AppKey, presented), tokens)
	if conflict || !match {
		h.recordLoginFail(bucket)
		return status.ErrUnauthenticated
	}
	h.touchMCPKey(k, addr, time.Now())
	return nil
}

// fortressSource reads the report and the log from what the overview and the
// activity log show.
type fortressSource struct{ h *Handler }

func (f fortressSource) Report(_ context.Context, lang string) (status.Report, error) {
	s := f.h.svc
	tx := fortressTextsFor(lang)
	settings, err := s.store.GetSettings()
	if err != nil {
		return status.Report{}, err
	}
	domains, err := s.domainStatusFrom(settings)
	if err != nil {
		return status.Report{}, err
	}
	now := time.Now()
	rep := status.Report{
		App:      status.App{ID: "bombvault", Name: "BombVault", Version: Version},
		Instance: status.Instance{ID: cmp.Or(s.instanceID(), "bombvault"), Name: cmp.Or(strings.TrimSpace(settings.InstanceName), "BombVault")},
	}

	states := []status.State{status.StateOff}
	for _, d := range domains {
		states = append(states, sectionState(d.Status))
		if d.Status == "overdue" {
			rep.NeedsYou = append(rep.NeedsYou, status.Need{ID: "overdue:" + d.Domain, Severity: status.SeverityWarn,
				Code: "backup.overdue", Text: tx.format("backup.overdue", tx.section(d.Domain)), Link: "/"})
		}
	}
	rep.State = status.Worst(states...)

	failed, err := f.failedBackups(tx)
	if err != nil {
		return status.Report{}, err
	}
	rep.NeedsYou = append(rep.NeedsYou, failed...)
	rep.NeedsYou = append(rep.NeedsYou, f.copies(tx, settings, now.Unix())...)
	if _, behind := s.zfsReplicaDigestLines(now.Unix(), settings); behind {
		rep.NeedsYou = append(rep.NeedsYou, status.Need{ID: "replica-behind", Severity: status.SeverityWarn,
			Code: "replica.behind", Text: tx.text("replica.behind"), Link: "/"})
	}
	anomalies, err := f.anomalies(tx)
	if err != nil {
		return status.Report{}, err
	}
	rep.NeedsYou = append(rep.NeedsYou, anomalies...)
	if len(rep.NeedsYou) > fortressNeedsLimit {
		rep.NeedsYou = rep.NeedsYou[:fortressNeedsLimit]
	}

	rep.Work = f.work(tx, now)
	rep.Next = f.next(tx)
	return rep, nil
}

// sectionState is the fortress state of a section's status on the overview.
// A section that never had its backup is not late until its first run had its
// chance, and then it reads overdue.
func sectionState(sectionStatus string) status.State {
	switch sectionStatus {
	case "ok", "never":
		return status.StateOK
	case "warn":
		return status.StateWarning
	case "overdue":
		return status.StateOverdue
	}
	return status.StateOff
}

// failedBackups are the entries whose last backup failed and that nobody
// marked as resolved.
func (f fortressSource) failedBackups(tx fortressTexts) ([]status.Need, error) {
	runs, err := f.h.store.LastRunsOfKind("backup")
	if err != nil {
		return nil, err
	}
	names := f.h.svc.runTargetNames()
	var out []status.Need
	for _, run := range runs {
		if run.Status != "failed" || run.Acknowledged {
			continue
		}
		name := cmp.Or(names[run.TargetID], run.TargetID)
		if label, ok := tx.targetLabel(run.TargetID); ok {
			name = label
		}
		out = append(out, status.Need{ID: "run:" + run.ID, Severity: status.SeverityError, Code: "backup.failed",
			Text: tx.format("backup.failed", name), Since: time.Unix(run.StartedAt, 0).UTC(), Link: "/"})
	}
	slices.SortFunc(out, func(a, b status.Need) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

// copies are the sections whose off-site copy never succeeded or fell behind.
func (f fortressSource) copies(tx fortressTexts, settings store.Settings, now int64) []status.Need {
	var out []status.Need
	for _, line := range f.h.svc.offsiteCurrency(settings, now) {
		switch {
		case line.LastOK == 0:
			out = append(out, status.Need{ID: "copy-never:" + line.Domain, Severity: status.SeverityWarn,
				Code: "copy.never", Text: tx.format("copy.never", tx.section(line.Domain)), Link: "/storage"})
		case line.Stale:
			out = append(out, status.Need{ID: "copy-stale:" + line.Domain, Severity: status.SeverityWarn,
				Code: "copy.stale", Text: tx.format("copy.stale", tx.section(line.Domain)), Since: time.Unix(line.LastOK, 0).UTC(), Link: "/storage"})
		}
	}
	return out
}

// anomalies are the open findings that are more than a note.
func (f fortressSource) anomalies(tx fortressTexts) ([]status.Need, error) {
	rows, _, err := f.h.store.ListAnomalies(store.AnomalyFilter{Severities: []string{"critical", "warning"}, Limit: fortressNeedsLimit})
	if err != nil {
		return nil, err
	}
	names := f.h.svc.runTargetNames()
	var out []status.Need
	for _, a := range rows {
		severity := status.SeverityWarn
		if a.Severity == "critical" {
			severity = status.SeverityError
		}
		name := cmp.Or(names[a.TargetID], tx.section(a.Domain))
		out = append(out, status.Need{ID: "anomaly:" + a.ID, Severity: severity, Code: "anomaly.open",
			Text: tx.format("anomaly.open", name), Since: time.Unix(a.FirstSeenAt, 0).UTC(), Link: "/anomalies"})
	}
	return out, nil
}

// work is what the progress bars show running.
func (f fortressSource) work(tx fortressTexts, now time.Time) []status.Task {
	if f.h.svc.progress == nil {
		return nil
	}
	var out []status.Task
	for _, e := range f.h.svc.progress.Snapshot() {
		if !e.Active || e.Key == "" {
			continue
		}
		task := status.Task{ID: e.Key, Kind: workKind(e.Phase), Title: tx.workTitle(e.Key), StartedAt: now.UTC()}
		if e.StartedAt > 0 {
			task.StartedAt = time.Unix(e.StartedAt, 0).UTC()
		}
		if e.Percent > 0 {
			p := min(e.Percent/100, 1)
			task.Progress = &p
		}
		// A host restarted under a backup or a restore leaves its container
		// stopped or half written.
		task.BlocksRestart = e.Phase == "backup" || e.Phase == "restore"
		out = append(out, task)
	}
	slices.SortFunc(out, func(a, b status.Task) int { return strings.Compare(a.ID, b.ID) })
	return out
}

func workKind(phase string) status.WorkKind {
	switch phase {
	case "backup":
		return status.KindBackup
	case "restore":
		return status.KindRestore
	case "replicate":
		return status.KindSync
	case "maintenance":
		return status.KindCheck
	}
	return status.KindOther
}

// next is when the scheduler starts its backups, copies and checks.
func (f fortressSource) next(tx fortressTexts) []status.Scheduled {
	if f.h.scheduler == nil {
		return nil
	}
	var out []status.Scheduled
	for _, run := range f.h.scheduler.NextRuns() {
		var item status.Scheduled
		switch run.Job {
		case "backup":
			item = status.Scheduled{Kind: status.KindBackup, Title: tx.section(run.Domain)}
		case "offsite":
			item = status.Scheduled{Kind: status.KindSync, Title: tx.format("next.offsite", tx.section(run.Domain))}
		case "drill", "tamper":
			item = status.Scheduled{Kind: status.KindCheck, Title: tx.kind(run.Job)}
		default:
			continue
		}
		item.At = run.Next.UTC()
		out = append(out, item)
		if len(out) == fortressNextLimit {
			break
		}
	}
	return out
}

// Log pages through the finished runs. The cursor is the end time and id of
// the last run a reader got.
func (f fortressSource) Log(_ context.Context, q status.LogQuery) (status.LogPage, error) {
	tx := fortressTextsFor(q.Lang)
	at, id := parseLogCursor(q.After)
	runs, err := f.h.store.FinishedRunsAfter(at, id, q.Limit)
	if err != nil {
		return status.LogPage{}, err
	}
	page := status.LogPage{Next: q.After}
	names := f.h.svc.runTargetNames()
	for _, run := range runs {
		name := cmp.Or(names[run.TargetID], cmp.Or(run.Subject, run.TargetID))
		if label, ok := tx.targetLabel(run.TargetID); ok {
			name = label
		}
		finished := *run.FinishedAt
		page.Entries = append(page.Entries, status.LogEntry{
			ID:    run.ID,
			At:    time.Unix(finished, 0).UTC(),
			Level: runLevel(run.Status),
			Code:  "run." + logCodePart(run.Kind) + "." + logCodePart(run.Status),
			Text:  tx.format("log.run", tx.kind(run.Kind), scrubRepoLocation(name), tx.runStatus(run.Status)),
		})
		page.Next = strconv.FormatInt(finished, 10) + "." + run.ID
	}
	return page, nil
}

func parseLogCursor(cursor string) (int64, string) {
	at, id, found := strings.Cut(cursor, ".")
	n, err := strconv.ParseInt(at, 10, 64)
	if !found || err != nil {
		if cursor != "" {
			log.Printf("api: fortress log: a cursor %q that is not one this instance gave starts at the oldest run", cursor)
		}
		return 0, ""
	}
	return n, id
}

func runLevel(runStatus string) status.Severity {
	switch runStatus {
	case "success":
		return status.SeverityInfo
	case "failed":
		return status.SeverityError
	}
	return status.SeverityWarn
}

// logCodePart keeps a run's kind or status inside the characters a contract
// code allows.
func logCodePart(v string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(v) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 || b.String()[0] < 'a' {
		return "other"
	}
	return b.String()
}
