package api

import (
	"context"
	"log"
	"math"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A new installation would start cold: ten backups before any rule has a
// baseline. restic 0.17 writes its counters into every snapshot, so the history
// an installation already has can be read back out of the repositories once and
// the rules start with whatever the retention still keeps.

const (
	// backfillSlotTimeout bounds one repository's snapshot listing. A remote
	// repository with thousands of snapshots is slow, and the listing must not
	// outlive the pass that started it.
	backfillSlotTimeout = 5 * time.Minute
	// backfillRetryAfter is how long a repository that would not open, or a
	// finished one that has newer runs to fill, waits before it is read again.
	backfillRetryAfter = 24 * 3600
	// vmRunTagPrefix marks the snapshots of one VM run: the disk images travel
	// as their own snapshots, and only the tag says which run they belong to.
	vmRunTagPrefix = "vmrun:"
)

// backfillSlot is one repository the backfill reads, under the name its state
// is recorded by.
type backfillSlot struct {
	Slot, Repo, Domain string
}

// backfillSlots names the repositories one domain writes to: its own under the
// domain's name, each named repository under its id, so a repository several
// domains share is read once rather than once per domain.
func backfillSlots(domain string, repos []domainRepoRef) []backfillSlot {
	out := make([]backfillSlot, 0, len(repos))
	for _, ref := range repos {
		switch {
		case ref.Own:
			out = append(out, backfillSlot{Slot: "domain:" + domain, Repo: ref.Loc, Domain: domain})
		case ref.Named.ID != "":
			out = append(out, backfillSlot{Slot: "repo:" + ref.Named.ID, Repo: ref.Loc, Domain: domain})
		}
	}
	return out
}

// matchSnapshotSummaries pairs the runs that carry no figures with the
// snapshots they wrote and returns the metrics per run, plus how many matched
// snapshots are older than the counters restic writes today.
//
// A VM run owns more than one snapshot: its file disks under the run's own
// snapshot id, each zvol disk under the run's tag. Their summaries are summed,
// which also corrects the new data such a run recorded before the disks were
// counted. A dump snapshot carries the id of the backup run that triggered it,
// and that link is never followed: a dump is a series of its own.
func matchSnapshotSummaries(runs []store.UnmeasuredRun, snaps []restic.SnapshotMeta) (map[string]store.BackfillMetrics, int) {
	bySnapshot := make(map[string]string, len(runs))
	byID := make(map[string]store.UnmeasuredRun, len(runs))
	for _, run := range runs {
		byID[run.ID] = run
		if run.SnapshotID != "" {
			bySnapshot[run.SnapshotID] = run.ID
		}
	}

	type tally struct {
		sourceBytes, sourceFiles, filesNew, resticMS, bytes int64
		snapshots, withParent                               int
		summed                                              bool
	}
	tallies := map[string]*tally{}
	seen := map[string]bool{}
	withoutSummary := 0

	for _, snap := range snaps {
		runID, summed := bySnapshot[snap.ID], false
		if runID == "" {
			runID, summed = vmRunOf(snap, byID), true
		}
		if runID == "" || seen[runID+"/"+snap.ID] {
			continue
		}
		seen[runID+"/"+snap.ID] = true
		if snap.Summary == nil {
			withoutSummary++
			continue
		}
		t := tallies[runID]
		if t == nil {
			t = &tally{}
			tallies[runID] = t
		}
		t.snapshots++
		t.summed = t.summed || summed
		t.sourceBytes += clampToInt64(snap.Summary.TotalBytesProcessed)
		t.sourceFiles += clampToInt64(snap.Summary.TotalFilesProcessed)
		t.filesNew += clampToInt64(snap.Summary.FilesNew)
		t.bytes += clampToInt64(snap.Summary.DataAdded)
		t.resticMS += max(snap.Summary.BackupEnd.Sub(snap.Summary.BackupStart).Milliseconds(), 0)
		if snap.Parent != "" {
			t.withParent++
		}
	}

	out := make(map[string]store.BackfillMetrics, len(tallies))
	for runID, t := range tallies {
		// A run counts as having had a parent only when every one of its
		// snapshots did: one disk restic saw for the first time makes every file
		// in it new, which is exactly what the fresh-upload rule looks for.
		hasParent := t.withParent == t.snapshots
		m := store.BackfillMetrics{RunMetrics: store.RunMetrics{
			SourceBytes: t.sourceBytes, SourceFiles: t.sourceFiles,
			FilesNew: t.filesNew, ResticMS: t.resticMS, HasParent: &hasParent,
		}}
		if t.summed {
			bytes := t.bytes
			m.Bytes = &bytes
		}
		out[runID] = m
	}
	return out, withoutSummary
}

// clampToInt64 keeps restic's unsigned counters inside the signed column they
// are stored in.
func clampToInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

func vmRunOf(snap restic.SnapshotMeta, runs map[string]store.UnmeasuredRun) string {
	for _, tag := range snap.Tags {
		id, ok := strings.CutPrefix(tag, vmRunTagPrefix)
		if !ok {
			continue
		}
		if _, known := runs[id]; known {
			return id
		}
	}
	return ""
}

// backfillRunMetrics fills the source figures of the runs that predate the
// measurement, once per repository. A repository that would not open records
// why and is tried again the next day; a repository that is done is read again
// only when runs from an older image have appeared since.
func (e *anomalyEngine) backfillRunMetrics(ctx context.Context) error {
	if e == nil {
		return nil
	}
	pending, err := e.svc.store.UnmeasuredSnapshotRuns()
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	settings, err := e.svc.store.GetSettings()
	if err != nil {
		return err
	}
	recorded, err := e.svc.store.ListAnomalyBackfill()
	if err != nil {
		return err
	}
	state := make(map[string]store.AnomalyBackfillSlot, len(recorded))
	for _, slot := range recorded {
		state[slot.Slot] = slot
	}

	now := e.now().Unix()
	filled := false
	for _, slot := range e.backfillTargets(settings) {
		if ctx.Err() != nil {
			break
		}
		previous, known := state[slot.Slot]
		if !e.slotIsDue(previous, known, pending, now) {
			continue
		}
		if e.readSlot(ctx, settings, slot, previous, pending, now) {
			filled = true
		}
	}
	if filled {
		e.MarkAllDirty()
	}
	return nil
}

// backfillTargets is every repository this installation writes to, each named
// once even when several domains share it.
func (e *anomalyEngine) backfillTargets(settings store.Settings) []backfillSlot {
	var out []backfillSlot
	seen := map[string]bool{}
	for _, domain := range enabledDomains(settings) {
		_, repos, _, err := e.svc.domainReposForOp(domain, "local")
		if err != nil {
			log.Printf("anomaly: backfill: resolve the repositories of %s: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
			continue
		}
		for _, slot := range backfillSlots(domain, repos) {
			if seen[slot.Slot] || (!restic.IsRemoteRepo(slot.Repo) && localRepoMissing(slot.Repo)) {
				continue
			}
			seen[slot.Slot] = true
			out = append(out, slot)
		}
	}
	return out
}

// enabledDomains are the domains this installation backs up, in the order the
// rest of the service lists them.
func enabledDomains(settings store.Settings) []string {
	var out []string
	for _, d := range []struct {
		name    string
		enabled bool
	}{
		{"containers", settings.ContainersEnabled},
		{"vms", settings.VMsEnabled},
		{"flash", settings.FlashEnabled},
		{"config", settings.ConfigEnabled},
		{"files", settings.FilesEnabled},
	} {
		if d.enabled {
			out = append(out, d.name)
		}
	}
	return out
}

// slotIsDue decides whether a repository is read in this pass. A finished one
// is read again only when a run it could fill has appeared since it was last
// read, and never more than once a day.
func (e *anomalyEngine) slotIsDue(previous store.AnomalyBackfillSlot, known bool,
	pending []store.UnmeasuredRun, now int64) bool {

	if !known {
		return true
	}
	if now-previous.AttemptedAt < backfillRetryAfter {
		return false
	}
	if !previous.Done {
		return true
	}
	for _, run := range pending {
		if run.StartedAt > previous.AttemptedAt {
			return true
		}
	}
	return false
}

// readSlot lists one repository and fills what it can, and reports whether any
// run gained figures.
func (e *anomalyEngine) readSlot(ctx context.Context, settings store.Settings, slot backfillSlot,
	previous store.AnomalyBackfillSlot, pending []store.UnmeasuredRun, now int64) bool {

	record := store.AnomalyBackfillSlot{
		Slot: slot.Slot, AttemptedAt: now,
		Filled: previous.Filled, WithoutSummary: previous.WithoutSummary,
	}

	listCtx, cancel := context.WithTimeout(ctx, backfillSlotTimeout)
	snaps, err := e.svc.engine.SnapshotsMeta(listCtx, slot.Repo,
		e.svc.repoModeFor(settings, slot.Domain, "local", slot.Repo))
	cancel()
	if err != nil {
		record.Error = scrubError(err)
		e.recordBackfill(record)
		return false
	}

	metrics, withoutSummary := matchSnapshotSummaries(pending, snaps)
	set, err := e.svc.store.SetRunMetrics(metrics)
	if err != nil {
		record.Error = scrubError(err)
		e.recordBackfill(record)
		return false
	}
	record.Done = true
	record.Filled += set
	record.WithoutSummary += withoutSummary
	e.recordBackfill(record)
	return set > 0
}

func (e *anomalyEngine) recordBackfill(slot store.AnomalyBackfillSlot) {
	if err := e.svc.store.RecordAnomalyBackfill(slot); err != nil {
		log.Printf("anomaly: backfill: record %s: %v", slot.Slot, err)
	}
}
