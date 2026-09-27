package api

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

var lifecycleItem = scopeRef{Kind: "item", ID: "t1", TargetID: "t1", Domain: "containers", Sensitivity: "balanced"}

var lifecycleVolume = scopeRef{Kind: "volume", ID: "dev:801"}

// shrinkFound is what a lost source raises: a condition, so a later pass can
// report it absent again.
func shrinkFound(severity string, observed float64) finding {
	return finding{
		Metric: metricSourceBytesShrink, Severity: severity,
		RunID: "r9", RunAt: anomalyNow,
		Observed: observed, Expected: 100 << 30, Threshold: 70 << 30, Samples: 30,
		Details: map[string]any{"ratio": 0.05},
	}
}

// spikeFound is an event: it is about one run, and nothing ever reports it
// absent again.
func spikeFound(runID string, at int64) finding {
	return finding{
		Metric: metricNewData, Severity: "warning",
		RunID: runID, RunAt: at,
		Observed: 20 << 30, Expected: 1 << 30, Samples: 30, Event: true,
	}
}

func anomalyRow(sc scopeRef, id, metric, state, severity string, lastRunAt int64) store.Anomaly {
	return store.Anomaly{
		ID:          id,
		Fingerprint: store.AnomalyFingerprint(anomalyDetectors[metric], sc.Kind, sc.ID, metric),
		Detector:    anomalyDetectors[metric],
		Metric:      metric, Severity: severity, State: state,
		ScopeKind: sc.Kind, ScopeID: sc.ID, TargetID: sc.TargetID, Domain: sc.Domain,
		LastRunID: "r8", LastRunAt: lastRunAt, Occurrences: 1,
		Expected: 100 << 30, FirstSeenAt: anomalyNow - 5*anomalyDay, LastSeenAt: anomalyNow - anomalyDay,
	}
}

// scopeState sorts rows the way AnomalyScopeState does, so the table is driven
// by the same view a pass reads from the store.
func scopeState(rows ...store.Anomaly) store.ScopeState {
	st := store.ScopeState{
		Open:          map[string]store.Anomaly{},
		ClosedEpisode: map[string]store.Anomaly{},
		LastRunAt:     map[string]int64{},
	}
	for _, row := range rows {
		if row.LastRunAt > st.LastRunAt[row.Fingerprint] {
			st.LastRunAt[row.Fingerprint] = row.LastRunAt
		}
		if row.State == "open" {
			st.Open[row.Fingerprint] = row
			continue
		}
		if row.ClearedAt == 0 {
			st.ClosedEpisode[row.Fingerprint] = row
		}
	}
	return st
}

func onlyInsert(t *testing.T, ch store.AnomalyChanges) store.Anomaly {
	t.Helper()
	if len(ch.Insert) != 1 {
		t.Fatalf("got %d inserts, want 1: %+v", len(ch.Insert), ch)
	}
	return ch.Insert[0]
}

// noWrites names every list that must be empty, so a rule that quietly writes
// a second row somewhere else fails here rather than in the store.
func noWrites(t *testing.T, ch store.AnomalyChanges, except ...string) {
	t.Helper()
	lengths := map[string]int{
		"insert": len(ch.Insert), "refresh": len(ch.Refresh), "recover": len(ch.Recover),
		"resolve": len(ch.Resolve), "clear": len(ch.Clear), "touchClosed": len(ch.TouchClosed),
	}
	for _, name := range except {
		delete(lengths, name)
	}
	for name, n := range lengths {
		if n != 0 {
			t.Fatalf("%s wrote %d rows: %+v", name, n, ch)
		}
	}
}

func TestApplyFindingsLifecycleTable(t *testing.T) {
	t.Run("a condition nobody has seen yet opens an episode", func(t *testing.T) {
		ch := applyFindings(lifecycleItem, []finding{shrinkFound("critical", 5<<30)}, nil, scopeState(), anomalyNow)
		row := onlyInsert(t, ch)
		noWrites(t, ch, "insert")
		if row.Fingerprint != "source|item|t1|source_bytes_shrink" || row.Detector != detectorSource {
			t.Fatalf("row = %+v", row)
		}
		if row.TargetID != "t1" || row.Domain != "containers" || row.Sensitivity != "balanced" {
			t.Fatalf("the row does not say which item it belongs to: %+v", row)
		}
		if row.Severity != "critical" || row.Observed != float64(5<<30) || row.Expected != float64(100<<30) {
			t.Fatalf("row = %+v", row)
		}
		if row.FirstSeenAt != anomalyNow || row.LastSeenAt != anomalyNow || row.LastRunAt != anomalyNow {
			t.Fatalf("row = %+v", row)
		}
		if row.RunID != "r9" || row.LastRunID != "r9" || row.Occurrences != 1 {
			t.Fatalf("row = %+v", row)
		}
		if !strings.Contains(row.Details, `"ratio"`) {
			t.Fatalf("details = %q", row.Details)
		}
	})

	t.Run("a condition that stays refreshes its row", func(t *testing.T) {
		open := anomalyRow(lifecycleItem, "a1", metricSourceBytesShrink, "open", "warning", anomalyNow-anomalyDay)
		ch := applyFindings(lifecycleItem, []finding{shrinkFound("critical", 4<<30)}, nil, scopeState(open), anomalyNow)
		noWrites(t, ch, "refresh")
		if len(ch.Refresh) != 1 {
			t.Fatalf("changes = %+v", ch)
		}
		got := ch.Refresh[0]
		if got.ID != "a1" || got.OccurrencesDelta != 1 || got.Severity != "critical" {
			t.Fatalf("refresh = %+v", got)
		}
		if got.LastSeenAt != anomalyNow || got.LastRunAt != anomalyNow || got.LastRunID != "r9" {
			t.Fatalf("refresh = %+v", got)
		}
		if got.Observed != float64(4<<30) {
			t.Fatalf("refresh = %+v", got)
		}
	})

	t.Run("a clean run resolves a warning", func(t *testing.T) {
		open := anomalyRow(lifecycleItem, "a1", metricSourceBytesShrink, "open", "warning", anomalyNow-anomalyDay)
		absent := []absence{{Metric: metricSourceBytesShrink, Observed: 95 << 30}}
		ch := applyFindings(lifecycleItem, nil, absent, scopeState(open), anomalyNow)
		noWrites(t, ch, "resolve")
		if len(ch.Resolve) != 1 || ch.Resolve[0] != "a1" {
			t.Fatalf("changes = %+v", ch)
		}
	})

	t.Run("a lost source stays open after it came back", func(t *testing.T) {
		open := anomalyRow(lifecycleItem, "a1", metricSourceBytesShrink, "open", "critical", anomalyNow-anomalyDay)
		absent := []absence{{Metric: metricSourceBytesShrink, Observed: 95 << 30}}
		ch := applyFindings(lifecycleItem, nil, absent, scopeState(open), anomalyNow)
		noWrites(t, ch, "recover")
		if len(ch.Recover) != 1 || ch.Recover[0] != "a1" {
			t.Fatalf("changes = %+v", ch)
		}

		open.RecoveredAt = anomalyNow - 3600
		ch = applyFindings(lifecycleItem, nil, absent, scopeState(open), anomalyNow)
		noWrites(t, ch)
	})

	t.Run("a disk that filled up stays open after it was emptied", func(t *testing.T) {
		open := anomalyRow(lifecycleVolume, "v1", metricCapacityLow, "open", "critical", 0)
		absent := []absence{{Metric: metricCapacityLow, Observed: 0.4}}
		ch := applyFindings(lifecycleVolume, nil, absent, scopeState(open), anomalyNow)
		noWrites(t, ch, "recover")
		if len(ch.Recover) != 1 || ch.Recover[0] != "v1" {
			t.Fatalf("changes = %+v", ch)
		}
	})

	t.Run("a failure streak resolves itself even as a critical", func(t *testing.T) {
		open := anomalyRow(lifecycleItem, "a1", metricFailureStreak, "open", "critical", anomalyNow-anomalyDay)
		absent := []absence{{Metric: metricFailureStreak, Observed: 0}}
		ch := applyFindings(lifecycleItem, nil, absent, scopeState(open), anomalyNow)
		noWrites(t, ch, "resolve")
		if len(ch.Resolve) != 1 || ch.Resolve[0] != "a1" {
			t.Fatalf("changes = %+v", ch)
		}
	})

	t.Run("a source lost again after it recovered opens a new episode", func(t *testing.T) {
		open := anomalyRow(lifecycleItem, "a1", metricSourceBytesShrink, "open", "critical", anomalyNow-anomalyDay)
		open.RecoveredAt = anomalyNow - 3600
		ch := applyFindings(lifecycleItem, []finding{shrinkFound("critical", 3<<30)}, nil, scopeState(open), anomalyNow)
		noWrites(t, ch, "insert", "resolve")
		if len(ch.Resolve) != 1 || ch.Resolve[0] != "a1" {
			t.Fatalf("the recovered row stayed open next to its successor: %+v", ch)
		}
		if row := onlyInsert(t, ch); row.FirstSeenAt != anomalyNow || row.RecoveredAt != 0 {
			t.Fatalf("row = %+v", row)
		}
	})

	t.Run("an event is not raised again for a run a closed row already saw", func(t *testing.T) {
		acked := anomalyRow(lifecycleItem, "a1", metricNewData, "acknowledged", "warning", anomalyNow)
		ch := applyFindings(lifecycleItem, []finding{spikeFound("r9", anomalyNow)}, nil, scopeState(acked), anomalyNow)
		noWrites(t, ch)

		ch = applyFindings(lifecycleItem, []finding{spikeFound("r10", anomalyNow+anomalyDay)}, nil,
			scopeState(acked), anomalyNow+anomalyDay)
		if row := onlyInsert(t, ch); row.LastRunID != "r10" {
			t.Fatalf("row = %+v", row)
		}
	})

	t.Run("two outliers of one pass share a row", func(t *testing.T) {
		found := []finding{spikeFound("r9", anomalyNow-anomalyDay), spikeFound("r10", anomalyNow)}
		ch := applyFindings(lifecycleItem, found, nil, scopeState(), anomalyNow)
		row := onlyInsert(t, ch)
		if row.Occurrences != 2 || row.LastRunID != "r10" || row.LastRunAt != anomalyNow {
			t.Fatalf("row = %+v", row)
		}
	})

	t.Run("an old warning about a single run resolves by age", func(t *testing.T) {
		stale := anomalyRow(lifecycleItem, "a1", metricNewData, "open", "warning", anomalyNow-8*anomalyDay)
		ch := applyFindings(lifecycleItem, nil, nil, scopeState(stale), anomalyNow)
		noWrites(t, ch, "resolve")
		if len(ch.Resolve) != 1 || ch.Resolve[0] != "a1" {
			t.Fatalf("changes = %+v", ch)
		}

		recent := anomalyRow(lifecycleItem, "a1", metricNewData, "open", "warning", anomalyNow-6*anomalyDay)
		noWrites(t, applyFindings(lifecycleItem, nil, nil, scopeState(recent), anomalyNow))
	})

	t.Run("a rewritten source stays open however old the run is", func(t *testing.T) {
		old := anomalyRow(lifecycleItem, "a1", metricNewDataRewrite, "open", "critical", anomalyNow-90*anomalyDay)
		noWrites(t, applyFindings(lifecycleItem, nil, nil, scopeState(old), anomalyNow))
	})

	t.Run("a condition nobody measured stays open", func(t *testing.T) {
		open := anomalyRow(lifecycleItem, "a1", metricSourceBytesShrink, "open", "warning", anomalyNow-30*anomalyDay)
		noWrites(t, applyFindings(lifecycleItem, nil, nil, scopeState(open), anomalyNow))
	})
}

func TestAcknowledgedConditionStaysClosedWhilePresent(t *testing.T) {
	cases := []struct {
		name     string
		scope    scopeRef
		state    string
		found    finding
		observed float64
	}{
		{
			name: "a lost source that was acknowledged", scope: lifecycleItem, state: "acknowledged",
			found: shrinkFound("critical", 5<<30), observed: 95 << 30,
		},
		{
			name: "a full disk that was acknowledged", scope: lifecycleVolume, state: "acknowledged",
			found: finding{
				Metric: metricCapacityLow, Severity: "critical", RunAt: anomalyNow,
				Observed: 0.04, Expected: 0.1, Samples: 20,
			},
			observed: 0.3,
		},
		{
			name: "a slower backup that was marked as expected", scope: lifecycleItem, state: "expected",
			found: finding{
				Metric: metricDurationSlower, Severity: "warning", RunID: "r9", RunAt: anomalyNow,
				Observed: 1_200_000, Expected: 60_000, Samples: 30,
			},
			observed: 61_000,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			closed := anomalyRow(tc.scope, "a1", tc.found.Metric, tc.state, tc.found.Severity, anomalyNow-anomalyDay)
			st := scopeState(closed)

			ch := applyFindings(tc.scope, []finding{tc.found}, nil, st, anomalyNow)
			noWrites(t, ch, "touchClosed")
			if len(ch.TouchClosed) != 1 || ch.TouchClosed[0].ID != "a1" {
				t.Fatalf("changes = %+v", ch)
			}
			if got := ch.TouchClosed[0]; got.Observed != tc.found.Observed || got.LastSeenAt != anomalyNow {
				t.Fatalf("touch = %+v", got)
			}

			absent := []absence{{Metric: tc.found.Metric, Observed: tc.observed}}
			ch = applyFindings(tc.scope, nil, absent, st, anomalyNow)
			noWrites(t, ch, "clear")
			if len(ch.Clear) != 1 || ch.Clear[0] != "a1" {
				t.Fatalf("changes = %+v", ch)
			}

			closed.ClearedAt = anomalyNow
			ch = applyFindings(tc.scope, []finding{tc.found}, nil, scopeState(closed), anomalyNow)
			if row := onlyInsert(t, ch); row.Severity != tc.found.Severity {
				t.Fatalf("row = %+v", row)
			}
		})
	}
}
