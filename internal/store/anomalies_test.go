package store_test

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func anomalyRepo(t *testing.T) *store.Repo {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

// finding builds an open finding of one metric on one item, with the fields the
// lifecycle reads and nothing else.
func finding(metric, severity string, at int64) store.Anomaly {
	return store.Anomaly{
		Fingerprint: store.AnomalyFingerprint("source", "item", "tg", metric),
		Detector:    "source",
		Metric:      metric,
		Severity:    severity,
		State:       "open",
		ScopeKind:   "item",
		ScopeID:     "tg",
		TargetID:    "tg",
		Domain:      "container",
		RunID:       "run-1",
		LastRunID:   "run-1",
		LastRunAt:   at,
		Observed:    1,
		Expected:    100,
		Samples:     30,
		Sensitivity: "balanced",
		Details:     `{"collapse":true}`,
		Occurrences: 1,
		FirstSeenAt: at,
		LastSeenAt:  at,
	}
}

func openRow(t *testing.T, r *store.Repo, fingerprint string) store.Anomaly {
	t.Helper()
	state, err := r.AnomalyScopeState("item", "tg")
	if err != nil {
		t.Fatalf("AnomalyScopeState: %v", err)
	}
	row, ok := state.Open[fingerprint]
	if !ok {
		t.Fatalf("no open row for %s", fingerprint)
	}
	return row
}

func TestApplyAnomalyChangesLifecycle(t *testing.T) {
	r := anomalyRepo(t)
	shrink := finding("source_bytes_shrink", "warning", 1000)

	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Insert: []store.Anomaly{shrink}, Now: 1000}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	row := openRow(t, r, shrink.Fingerprint)
	if row.ID == "" {
		t.Fatal("the inserted row got no id")
	}
	if row.Severity != "warning" || row.Occurrences != 1 {
		t.Fatalf("inserted row is %+v", row)
	}

	res, err := r.ApplyAnomalyChanges(store.AnomalyChanges{
		Refresh: []store.AnomalyRefresh{{
			ID: row.ID, Observed: 2, Severity: "critical", LastSeenAt: 2000,
			LastRunID: "run-2", LastRunAt: 2000, OccurrencesDelta: 1, Details: `{"collapse":true}`,
		}},
		Now: 2000,
	})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(res.Stale) != 0 {
		t.Fatalf("refresh skipped %v", res.Stale)
	}
	row = openRow(t, r, shrink.Fingerprint)
	if row.Severity != "critical" || row.Occurrences != 2 || row.LastRunID != "run-2" || row.Observed != 2 {
		t.Fatalf("refreshed row is %+v", row)
	}

	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{
		Refresh: []store.AnomalyRefresh{{ID: row.ID, Observed: 3, Severity: "warning", LastSeenAt: 3000, LastRunID: "run-3", LastRunAt: 3000}},
		Now:     3000,
	}); err != nil {
		t.Fatalf("refresh down: %v", err)
	}
	row = openRow(t, r, shrink.Fingerprint)
	if row.Severity != "critical" {
		t.Fatalf("severity fell back to %q", row.Severity)
	}
	if row.Details != `{"collapse":true}` {
		t.Fatalf("a refresh without details left %q behind", row.Details)
	}

	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Recover: []string{row.ID}, Now: 4000}); err != nil {
		t.Fatalf("recover: %v", err)
	}
	row = openRow(t, r, shrink.Fingerprint)
	if row.RecoveredAt != 4000 || row.State != "open" {
		t.Fatalf("a recovered row is %+v, it stays open until it is acknowledged", row)
	}

	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Resolve: []string{row.ID}, Now: 5000}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	resolved, ok, err := r.GetAnomaly(row.ID)
	if err != nil || !ok {
		t.Fatalf("GetAnomaly: %v %v", ok, err)
	}
	if resolved.State != "resolved" || resolved.ResolvedAt != 5000 || resolved.ClearedAt != 5000 {
		t.Fatalf("resolved row is %+v", resolved)
	}
}

func TestApplyAnomalyChangesRejectsASecondOpenRowOfOneFingerprint(t *testing.T) {
	r := anomalyRepo(t)
	first := finding("source_bytes_shrink", "critical", 1000)
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Insert: []store.Anomaly{first}, Now: 1000}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	second := finding("source_bytes_shrink", "critical", 2000)
	other := finding("source_files_shrink", "warning", 2000)
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Insert: []store.Anomaly{other, second}, Now: 2000}); err == nil {
		t.Fatal("a second open row of one fingerprint was accepted")
	}

	state, err := r.AnomalyScopeState("item", "tg")
	if err != nil {
		t.Fatalf("AnomalyScopeState: %v", err)
	}
	if len(state.Open) != 1 {
		t.Fatalf("the failed change set left %d open rows behind", len(state.Open))
	}
}

func TestApplyAnomalyChangesSkipsRowsAUserClosed(t *testing.T) {
	r := anomalyRepo(t)
	shrink := finding("source_bytes_shrink", "critical", 1000)
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Insert: []store.Anomaly{shrink}, Now: 1000}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	row := openRow(t, r, shrink.Fingerprint)

	if _, err := r.AcknowledgeAnomalies([]string{row.ID}, "planned clean-up", 1500); err != nil {
		t.Fatalf("AcknowledgeAnomalies: %v", err)
	}

	res, err := r.ApplyAnomalyChanges(store.AnomalyChanges{
		Resolve: []string{row.ID},
		Refresh: []store.AnomalyRefresh{{ID: row.ID, Observed: 7, Severity: "critical", LastSeenAt: 2000}},
		Now:     2000,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Stale) != 2 {
		t.Fatalf("Stale = %v, want both writes counted", res.Stale)
	}

	got, ok, err := r.GetAnomaly(row.ID)
	if err != nil || !ok {
		t.Fatalf("GetAnomaly: %v %v", ok, err)
	}
	if got.State != "acknowledged" || got.AckNote != "planned clean-up" || got.Observed != row.Observed {
		t.Fatalf("the pass overwrote what the user had closed: %+v", got)
	}
}

func TestAnomalyScopeStateReadsTheCurrentEpisode(t *testing.T) {
	r := anomalyRepo(t)
	shrink := finding("source_bytes_shrink", "critical", 1000)
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Insert: []store.Anomaly{shrink}, Now: 1000}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	row := openRow(t, r, shrink.Fingerprint)
	if _, err := r.AcknowledgeAnomalies([]string{row.ID}, "", 1500); err != nil {
		t.Fatalf("AcknowledgeAnomalies: %v", err)
	}

	state, err := r.AnomalyScopeState("item", "tg")
	if err != nil {
		t.Fatalf("AnomalyScopeState: %v", err)
	}
	if len(state.Open) != 0 {
		t.Fatalf("the acknowledged row is still open: %+v", state.Open)
	}
	episode, ok := state.ClosedEpisode[shrink.Fingerprint]
	if !ok || episode.ID != row.ID {
		t.Fatalf("ClosedEpisode = %+v, want the acknowledged row", state.ClosedEpisode)
	}
	if state.LastRunAt[shrink.Fingerprint] != 1000 {
		t.Fatalf("LastRunAt = %d, want the run that opened the row", state.LastRunAt[shrink.Fingerprint])
	}

	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{
		TouchClosed: []store.AnomalyRefresh{{ID: row.ID, Observed: 9, LastSeenAt: 2000}},
		Now:         2000,
	}); err != nil {
		t.Fatalf("touch closed: %v", err)
	}
	got, _, err := r.GetAnomaly(row.ID)
	if err != nil {
		t.Fatalf("GetAnomaly: %v", err)
	}
	if got.Observed != 9 || got.LastSeenAt != 2000 || got.ClearedAt != 0 {
		t.Fatalf("a condition that is still present reads as %+v", got)
	}

	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Clear: []string{row.ID}, Now: 3000}); err != nil {
		t.Fatalf("clear: %v", err)
	}
	state, err = r.AnomalyScopeState("item", "tg")
	if err != nil {
		t.Fatalf("AnomalyScopeState: %v", err)
	}
	if _, ok := state.ClosedEpisode[shrink.Fingerprint]; ok {
		t.Fatal("the episode did not end when the condition went away")
	}
}

func TestListAnomaliesFilters(t *testing.T) {
	r := anomalyRepo(t)
	old := finding("source_bytes_shrink", "critical", 1000)
	old.LastSeenAt = 1000
	duration := finding("duration_slower", "warning", 2000)
	duration.Detector = "duration"
	duration.LastSeenAt = 2000
	dump := finding("dump_bytes_shrink", "warning", 3000)
	dump.ScopeKind = "dump"
	dump.Fingerprint = store.AnomalyFingerprint("source", "dump", "tg", "dump_bytes_shrink")
	dump.LastSeenAt = 3000
	elsewhere := finding("source_bytes_shrink", "info", 4000)
	elsewhere.ScopeID = "other"
	elsewhere.TargetID = "other"
	elsewhere.Domain = "files"
	elsewhere.Fingerprint = store.AnomalyFingerprint("source", "item", "other", "source_bytes_shrink")
	elsewhere.LastSeenAt = 4000

	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{
		Insert: []store.Anomaly{old, duration, dump, elsewhere},
		Now:    4000,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	rows, _, err := r.ListAnomalies(store.AnomalyFilter{Since: 3500})
	if err != nil {
		t.Fatalf("ListAnomalies: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d open rows, want all four whatever Since says", len(rows))
	}
	if rows[0].Severity != "critical" || rows[len(rows)-1].Severity != "info" {
		t.Fatalf("open rows are not ordered by severity: %q … %q", rows[0].Severity, rows[len(rows)-1].Severity)
	}

	rows, _, err = r.ListAnomalies(store.AnomalyFilter{Detectors: []string{"duration"}})
	if err != nil {
		t.Fatalf("ListAnomalies detector: %v", err)
	}
	if len(rows) != 1 || rows[0].Metric != "duration_slower" {
		t.Fatalf("detector filter returned %+v", rows)
	}

	rows, _, err = r.ListAnomalies(store.AnomalyFilter{TargetID: "tg"})
	if err != nil {
		t.Fatalf("ListAnomalies target: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("the item's rows are %d, want its item and dump series", len(rows))
	}

	rows, _, err = r.ListAnomalies(store.AnomalyFilter{ScopeKind: "dump", ScopeID: "tg"})
	if err != nil {
		t.Fatalf("ListAnomalies scope: %v", err)
	}
	if len(rows) != 1 || rows[0].Metric != "dump_bytes_shrink" {
		t.Fatalf("scope filter returned %+v", rows)
	}

	rows, _, err = r.ListAnomalies(store.AnomalyFilter{Domains: []string{"files"}, Severities: []string{"info"}})
	if err != nil {
		t.Fatalf("ListAnomalies domain: %v", err)
	}
	if len(rows) != 1 || rows[0].TargetID != "other" {
		t.Fatalf("domain filter returned %+v", rows)
	}

	openRows, _, err := r.ListAnomalies(store.AnomalyFilter{})
	if err != nil {
		t.Fatalf("ListAnomalies: %v", err)
	}
	if _, err := r.AcknowledgeAnomalies([]string{openRows[0].ID}, "", 5000); err != nil {
		t.Fatalf("AcknowledgeAnomalies: %v", err)
	}
	rows, _, err = r.ListAnomalies(store.AnomalyFilter{States: []string{"acknowledged"}, Since: 900})
	if err != nil {
		t.Fatalf("ListAnomalies closed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("closed listing returned %d rows", len(rows))
	}
	rows, _, err = r.ListAnomalies(store.AnomalyFilter{States: []string{"acknowledged"}, Since: 5500})
	if err != nil {
		t.Fatalf("ListAnomalies closed since: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("Since did not filter the closed listing: %+v", rows)
	}
}

func TestOpenListingCursorNeverSkipsAcrossSeverities(t *testing.T) {
	r := anomalyRepo(t)
	var rows []store.Anomaly
	for i, spec := range []struct {
		metric     string
		severity   string
		lastSeenAt int64
	}{
		{"source_bytes_shrink", "critical", 100},
		{"source_files_shrink", "critical", 90},
		{"duration_slower", "warning", 200},
		{"new_data", "warning", 150},
	} {
		a := finding(spec.metric, spec.severity, spec.lastSeenAt)
		a.Fingerprint = store.AnomalyFingerprint("source", "item", "tg", spec.metric)
		a.LastSeenAt = spec.lastSeenAt
		a.FirstSeenAt = spec.lastSeenAt
		a.RunID = string(rune('a' + i))
		rows = append(rows, a)
	}
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Insert: rows, Now: 300}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	for _, states := range [][]string{nil, {"open"}, {"open", "resolved", "acknowledged", "expected"}} {
		unpaged, _, err := r.ListAnomalies(store.AnomalyFilter{States: states})
		if err != nil {
			t.Fatalf("ListAnomalies: %v", err)
		}
		var paged []store.Anomaly
		cursor := ""
		for range len(unpaged) + 1 {
			page, next, pErr := r.ListAnomalies(store.AnomalyFilter{States: states, Limit: 2, Cursor: cursor})
			if pErr != nil {
				t.Fatalf("ListAnomalies page: %v", pErr)
			}
			paged = append(paged, page...)
			if next == "" {
				break
			}
			cursor = next
		}
		if len(paged) != len(unpaged) {
			t.Fatalf("states %v: paged over %d rows, unpaged has %d", states, len(paged), len(unpaged))
		}
		for i := range paged {
			if paged[i].ID != unpaged[i].ID {
				t.Fatalf("states %v: page %d holds %q, unpaged holds %q", states, i, paged[i].Metric, unpaged[i].Metric)
			}
		}
	}
}

func TestAcknowledgeAndExpectedOnlyChangeOpenRows(t *testing.T) {
	r := anomalyRepo(t)
	shrink := finding("source_bytes_shrink", "critical", 1000)
	growth := finding("source_bytes_growth", "warning", 1000)
	growth.Fingerprint = store.AnomalyFingerprint("source", "item", "tg", "source_bytes_growth")
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Insert: []store.Anomaly{shrink, growth}, Now: 1000}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	shrinkRow := openRow(t, r, shrink.Fingerprint)
	growthRow := openRow(t, r, growth.Fingerprint)

	note := strings.Repeat("0123456789", 60)
	acked, err := r.AcknowledgeAnomalies([]string{shrinkRow.ID}, note, 2000)
	if err != nil {
		t.Fatalf("AcknowledgeAnomalies: %v", err)
	}
	if len(acked) != 1 || acked[0].State != "acknowledged" || acked[0].AckedAt != 2000 {
		t.Fatalf("acknowledged rows are %+v", acked)
	}
	if len(acked[0].AckNote) != 500 {
		t.Fatalf("the note is %d characters long, want it cut at 500", len(acked[0].AckNote))
	}

	again, err := r.AcknowledgeAnomalies([]string{shrinkRow.ID}, "second", 3000)
	if err != nil {
		t.Fatalf("AcknowledgeAnomalies twice: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("an acknowledged row was changed again: %+v", again)
	}
	stored, _, err := r.GetAnomaly(shrinkRow.ID)
	if err != nil {
		t.Fatalf("GetAnomaly: %v", err)
	}
	if stored.AckedAt != 2000 || stored.AckNote == "second" {
		t.Fatalf("the second acknowledge overwrote the first: %+v", stored)
	}

	expected, err := r.MarkAnomaliesExpected([]string{growthRow.ID, shrinkRow.ID}, "seeded the library", 4000)
	if err != nil {
		t.Fatalf("MarkAnomaliesExpected: %v", err)
	}
	if len(expected) != 1 || expected[0].ID != growthRow.ID || expected[0].State != "expected" {
		t.Fatalf("expected rows are %+v", expected)
	}
}

func TestPendingAnomalyNotificationsAndStamp(t *testing.T) {
	r := anomalyRepo(t)
	shrink := finding("source_bytes_shrink", "warning", 1000)
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Insert: []store.Anomaly{shrink}, Now: 1000}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	row := openRow(t, r, shrink.Fingerprint)

	pending, err := r.PendingAnomalyNotifications()
	if err != nil {
		t.Fatalf("PendingAnomalyNotifications: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != row.ID {
		t.Fatalf("pending rows are %+v", pending)
	}

	if err := r.MarkAnomaliesNotified([]string{row.ID}, map[string]string{row.ID: "warning"}, 1100); err != nil {
		t.Fatalf("MarkAnomaliesNotified: %v", err)
	}
	pending, err = r.PendingAnomalyNotifications()
	if err != nil {
		t.Fatalf("PendingAnomalyNotifications: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("a stamped row is still pending: %+v", pending)
	}

	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{
		Refresh: []store.AnomalyRefresh{{ID: row.ID, Severity: "critical", LastSeenAt: 2000}},
		Now:     2000,
	}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	pending, err = r.PendingAnomalyNotifications()
	if err != nil {
		t.Fatalf("PendingAnomalyNotifications: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("the escalation to critical was not sent again: %+v", pending)
	}

	if _, err := r.AcknowledgeAnomalies([]string{row.ID}, "", 3000); err != nil {
		t.Fatalf("AcknowledgeAnomalies: %v", err)
	}
	pending, err = r.PendingAnomalyNotifications()
	if err != nil {
		t.Fatalf("PendingAnomalyNotifications: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("a closed row is pending: %+v", pending)
	}
}

func TestPruneAnomaliesKeepsOpenCurrentEpisodesAndExpectations(t *testing.T) {
	r := anomalyRepo(t)
	const day = 86400
	stale := finding("duration_slower", "warning", 10*day)
	stale.Detector = "duration"
	stale.LastSeenAt = 10 * day
	condition := finding("source_bytes_shrink", "critical", 11*day)
	condition.LastSeenAt = 11 * day
	live := finding("source_files_shrink", "critical", 12*day)
	live.Fingerprint = store.AnomalyFingerprint("source", "item", "tg", "source_files_shrink")
	live.LastSeenAt = 12 * day
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{
		Insert: []store.Anomaly{stale, condition, live}, Now: 12 * day,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	staleRow := openRow(t, r, stale.Fingerprint)
	conditionRow := openRow(t, r, condition.Fingerprint)

	if _, err := r.AcknowledgeAnomalies([]string{staleRow.ID, conditionRow.ID}, "", 13*day); err != nil {
		t.Fatalf("AcknowledgeAnomalies: %v", err)
	}
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Clear: []string{staleRow.ID}, Now: 13 * day}); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if err := r.UpsertAnomalyExpectation(store.AnomalyExpectation{
		ScopeKind: "item", ScopeID: "tg", TargetID: "tg", Family: "duration", SinceAt: 10 * day, UpdatedAt: 13 * day,
	}); err != nil {
		t.Fatalf("UpsertAnomalyExpectation: %v", err)
	}

	n, err := r.PruneAnomalies(200 * day)
	if err != nil {
		t.Fatalf("PruneAnomalies: %v", err)
	}
	if n != 1 {
		t.Fatalf("pruned %d rows, want only the ended episode", n)
	}
	if _, ok, gErr := r.GetAnomaly(staleRow.ID); gErr != nil || ok {
		t.Fatalf("the ended episode survived: %v %v", ok, gErr)
	}
	if _, ok, gErr := r.GetAnomaly(conditionRow.ID); gErr != nil || !ok {
		t.Fatalf("an acknowledged condition that is still on was pruned: %v %v", ok, gErr)
	}
	expectations, err := r.ListAnomalyExpectations("item", "tg")
	if err != nil {
		t.Fatalf("ListAnomalyExpectations: %v", err)
	}
	if len(expectations) != 1 {
		t.Fatalf("pruning took the expectations: %+v", expectations)
	}
	openRows, _, err := r.ListAnomalies(store.AnomalyFilter{})
	if err != nil {
		t.Fatalf("ListAnomalies: %v", err)
	}
	if len(openRows) != 1 {
		t.Fatalf("open rows after pruning: %+v", openRows)
	}
}

func TestAnomalyExpectationUpsertTakesMaxima(t *testing.T) {
	r := anomalyRepo(t)
	base := store.AnomalyExpectation{ScopeKind: "item", ScopeID: "tg", TargetID: "tg", Family: "new_data"}

	first := base
	first.Ceiling = 500
	first.SinceAt = 1000
	first.UpdatedAt = 1000
	if err := r.UpsertAnomalyExpectation(first); err != nil {
		t.Fatalf("UpsertAnomalyExpectation: %v", err)
	}
	lower := base
	lower.Ceiling = 100
	lower.UpdatedAt = 2000
	if err := r.UpsertAnomalyExpectation(lower); err != nil {
		t.Fatalf("UpsertAnomalyExpectation lower: %v", err)
	}

	got, err := r.ListAnomalyExpectations("item", "tg")
	if err != nil {
		t.Fatalf("ListAnomalyExpectations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expectations are %+v", got)
	}
	if got[0].Ceiling != 500 || got[0].SinceAt != 1000 || got[0].UpdatedAt != 2000 {
		t.Fatalf("a lower ceiling narrowed the expectation: %+v", got[0])
	}

	down := base
	down.Family = "source_bytes_down"
	down.SinceAt = 3000
	down.UpdatedAt = 3000
	up := base
	up.Family = "source_bytes_up"
	up.SinceAt = 3000
	up.UpdatedAt = 3000
	for _, e := range []store.AnomalyExpectation{down, up} {
		if err := r.UpsertAnomalyExpectation(e); err != nil {
			t.Fatalf("UpsertAnomalyExpectation %s: %v", e.Family, err)
		}
	}
	forTarget, err := r.ListAnomalyExpectationsForTarget("tg")
	if err != nil {
		t.Fatalf("ListAnomalyExpectationsForTarget: %v", err)
	}
	if len(forTarget) != 3 {
		t.Fatalf("the two directions did not stay apart: %+v", forTarget)
	}

	if err := r.DeleteAnomalyExpectation("item", "tg", "source_bytes_up"); err != nil {
		t.Fatalf("DeleteAnomalyExpectation: %v", err)
	}
	forTarget, err = r.ListAnomalyExpectationsForTarget("tg")
	if err != nil {
		t.Fatalf("ListAnomalyExpectationsForTarget: %v", err)
	}
	if len(forTarget) != 2 {
		t.Fatalf("after forgetting one direction: %+v", forTarget)
	}
}

func TestItemPrefsFollowTheGlobalOnceCleared(t *testing.T) {
	r := anomalyRepo(t)
	if err := r.SetItemPrefs("tg", store.ItemPrefs{Sensitivity: "strict", NotifyMin: "warning"}); err != nil {
		t.Fatalf("SetItemPrefs: %v", err)
	}
	prefs, err := r.ListItemPrefs()
	if err != nil {
		t.Fatalf("ListItemPrefs: %v", err)
	}
	if prefs["tg"].Sensitivity != "strict" || prefs["tg"].NotifyMin != "warning" {
		t.Fatalf("stored preferences are %+v", prefs)
	}

	if err := r.SetItemPrefs("tg", store.ItemPrefs{}); err != nil {
		t.Fatalf("SetItemPrefs empty: %v", err)
	}
	prefs, err = r.ListItemPrefs()
	if err != nil {
		t.Fatalf("ListItemPrefs: %v", err)
	}
	if len(prefs) != 0 {
		t.Fatalf("an item that follows the global settings kept a row: %+v", prefs)
	}
}

func TestBackfillSlotKeepsItsLastAttempt(t *testing.T) {
	r := anomalyRepo(t)
	slot := store.AnomalyBackfillSlot{Slot: "domain:containers", AttemptedAt: 1000, Error: "repository is locked"}
	if err := r.RecordAnomalyBackfill(slot); err != nil {
		t.Fatalf("RecordAnomalyBackfill: %v", err)
	}
	slot.AttemptedAt = 2000
	slot.Done = true
	slot.Filled = 12
	slot.WithoutSummary = 3
	slot.Error = ""
	if err := r.RecordAnomalyBackfill(slot); err != nil {
		t.Fatalf("RecordAnomalyBackfill again: %v", err)
	}

	slots, err := r.ListAnomalyBackfill()
	if err != nil {
		t.Fatalf("ListAnomalyBackfill: %v", err)
	}
	if len(slots) != 1 {
		t.Fatalf("slots are %+v", slots)
	}
	if !slots[0].Done || slots[0].Filled != 12 || slots[0].WithoutSummary != 3 || slots[0].Error != "" {
		t.Fatalf("the retry did not replace the failed attempt: %+v", slots[0])
	}
}

func TestOpenAnomalyCountsSplitBySeverity(t *testing.T) {
	r := anomalyRepo(t)
	shrink := finding("source_bytes_shrink", "critical", 1000)
	files := finding("source_files_shrink", "critical", 1000)
	files.Fingerprint = store.AnomalyFingerprint("source", "item", "tg", "source_files_shrink")
	slow := finding("duration_slower", "warning", 1000)
	slow.Detector = "duration"
	slow.Fingerprint = store.AnomalyFingerprint("duration", "item", "tg", "duration_slower")
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Insert: []store.Anomaly{shrink, files, slow}, Now: 1000}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	row := openRow(t, r, files.Fingerprint)
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Recover: []string{row.ID}, Now: 2000}); err != nil {
		t.Fatalf("recover: %v", err)
	}

	counts, recovered, err := r.OpenAnomalyCounts()
	if err != nil {
		t.Fatalf("OpenAnomalyCounts: %v", err)
	}
	if counts["critical"] != 2 || counts["warning"] != 1 {
		t.Fatalf("counts are %+v", counts)
	}
	if recovered != 1 {
		t.Fatalf("recovered criticals = %d", recovered)
	}
}

// seedAnomalyState gives one item an open finding on each kind of series it can
// have, plus a preference and an expectation, so a delete has state to take
// with it.
func seedAnomalyState(t *testing.T, r *store.Repo, targetID, domain string) {
	t.Helper()
	series := []struct {
		scopeKind, scopeID, metric string
	}{
		{"item", targetID, "source_bytes_shrink"},
		{"dump", targetID, "dump_bytes_shrink"},
		{"zfsds", "tank/" + targetID, "source_files_shrink"},
	}
	var rows []store.Anomaly
	for _, s := range series {
		rows = append(rows, store.Anomaly{
			Fingerprint: store.AnomalyFingerprint("source", s.scopeKind, s.scopeID, s.metric),
			Detector:    "source",
			Metric:      s.metric,
			Severity:    "critical",
			ScopeKind:   s.scopeKind,
			ScopeID:     s.scopeID,
			TargetID:    targetID,
			Domain:      domain,
			FirstSeenAt: 1000,
			LastSeenAt:  1000,
		})
	}
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Insert: rows, Now: 1000}); err != nil {
		t.Fatalf("seed anomalies: %v", err)
	}
	if err := r.SetItemPrefs(targetID, store.ItemPrefs{Sensitivity: "strict"}); err != nil {
		t.Fatalf("seed preferences: %v", err)
	}
	err := r.UpsertAnomalyExpectation(store.AnomalyExpectation{
		ScopeKind: "item", ScopeID: targetID, TargetID: targetID, Family: "source_bytes_down",
		SinceAt: 1000, UpdatedAt: 1000,
	})
	if err != nil {
		t.Fatalf("seed expectation: %v", err)
	}
}

// seedDomainAnomaly adds a finding that belongs to no item, to show that a
// delete takes only what hangs off the item.
func seedDomainAnomaly(t *testing.T, r *store.Repo) {
	t.Helper()
	row := store.Anomaly{
		Fingerprint: store.AnomalyFingerprint("integrity", "domain", "containers:local", "drill_subset"),
		Detector:    "integrity",
		Metric:      "drill_subset",
		Severity:    "critical",
		ScopeKind:   "domain",
		ScopeID:     "containers:local",
		Domain:      "containers",
		FirstSeenAt: 1000,
		LastSeenAt:  1000,
	}
	if _, err := r.ApplyAnomalyChanges(store.AnomalyChanges{Insert: []store.Anomaly{row}, Now: 1000}); err != nil {
		t.Fatalf("seed domain anomaly: %v", err)
	}
}

func assertAnomalyStateGone(t *testing.T, r *store.Repo, targetID string) {
	t.Helper()
	rows, _, err := r.ListAnomalies(store.AnomalyFilter{TargetID: targetID})
	if err != nil {
		t.Fatalf("ListAnomalies: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("the deleted item kept %d findings: %+v", len(rows), rows)
	}
	prefs, err := r.ListItemPrefs()
	if err != nil {
		t.Fatalf("ListItemPrefs: %v", err)
	}
	if _, ok := prefs[targetID]; ok {
		t.Fatal("the deleted item kept its preferences")
	}
	expectations, err := r.ListAnomalyExpectationsForTarget(targetID)
	if err != nil {
		t.Fatalf("ListAnomalyExpectationsForTarget: %v", err)
	}
	if len(expectations) != 0 {
		t.Fatalf("the deleted item kept its expectations: %+v", expectations)
	}
}
