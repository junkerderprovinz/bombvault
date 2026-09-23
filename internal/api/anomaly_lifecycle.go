package api

import (
	"encoding/json"
	"maps"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// anomalyEventOpenDays is how long a warning about a single run stays up once
// nothing raises it again. A condition has an observation that ends it, an
// event has none.
const anomalyEventOpenDays = 7

// scopeRef is the series one pass decided about: the kind and id that name the
// scope, plus what every row written for it carries.
type scopeRef struct {
	Kind, ID, TargetID, Domain, Sensitivity string
}

// eventMetrics are about one run rather than about a state that lasts, so
// nothing ever reports them absent again and an open row of one ages out.
var eventMetrics = []string{metricNewData, metricNewDataRewrite, metricNewDataFull}

// recoveryAckMetrics keep their row open after the condition is gone. Losing
// data and filling a disk are the two things a user has to see even once they
// are over; everything else resolves by itself, because the failed run or the
// failed check has a surface of its own.
var recoveryAckMetrics = []string{
	metricSourceBytesShrink, metricSourceFilesShrink, metricDumpBytesShrink,
	metricNewDataRewrite, metricCapacityETA, metricCapacityLow,
}

// applyFindings turns one pass's view of a scope into the rows to write. A
// condition that appears opens an episode, one that stays refreshes it, one
// that goes away resolves it or, where it has to be seen, marks it recovered.
// An episode a user settled is not raised again while its condition is still
// there; it ends the first time the condition is gone.
func applyFindings(sc scopeRef, found []finding, absent []absence, st store.ScopeState, now int64) store.AnomalyChanges {
	ch := store.AnomalyChanges{Now: now}
	written := map[string]bool{}

	for _, group := range byMetric(found) {
		fp := fingerprintOf(sc, group[0].Metric)
		if group[0].Event {
			group = raisedSince(group, st.LastRunAt[fp])
			if len(group) == 0 {
				continue
			}
		}
		newest := group[len(group)-1]

		if open, isOpen := st.Open[fp]; isOpen {
			if open.RecoveredAt > 0 {
				ch.Resolve = append(ch.Resolve, open.ID)
				ch.Insert = append(ch.Insert, newRow(sc, fp, newest, len(group), now))
			} else {
				ch.Refresh = append(ch.Refresh, store.AnomalyRefresh{
					ID: open.ID, Observed: newest.Observed, Severity: newest.Severity,
					LastSeenAt: now, LastRunID: newest.RunID, LastRunAt: newest.RunAt,
					OccurrencesDelta: len(group), Details: detailsJSON(newest.Details),
				})
			}
			written[fp] = true
			continue
		}
		// A settled episode absorbs its own condition. An event has no moment of
		// absence that could end one, so a later run opens a new episode.
		if closed, settled := st.ClosedEpisode[fp]; settled && !newest.Event {
			ch.TouchClosed = append(ch.TouchClosed, store.AnomalyRefresh{
				ID: closed.ID, Observed: newest.Observed, LastSeenAt: now,
			})
			written[fp] = true
			continue
		}
		ch.Insert = append(ch.Insert, newRow(sc, fp, newest, len(group), now))
		written[fp] = true
	}

	for _, gone := range absent {
		fp := fingerprintOf(sc, gone.Metric)
		if written[fp] {
			continue
		}
		if open, isOpen := st.Open[fp]; isOpen {
			if open.Severity == "critical" && slices.Contains(recoveryAckMetrics, gone.Metric) {
				if open.RecoveredAt == 0 {
					ch.Recover = append(ch.Recover, open.ID)
				}
			} else {
				ch.Resolve = append(ch.Resolve, open.ID)
			}
			written[fp] = true
			continue
		}
		if closed, settled := st.ClosedEpisode[fp]; settled {
			ch.Clear = append(ch.Clear, closed.ID)
			written[fp] = true
		}
	}

	for _, fp := range slices.Sorted(maps.Keys(st.Open)) {
		open := st.Open[fp]
		if written[fp] || open.Severity == "critical" || !slices.Contains(eventMetrics, open.Metric) {
			continue
		}
		if now-open.LastRunAt > anomalyEventOpenDays*86400 {
			ch.Resolve = append(ch.Resolve, open.ID)
		}
	}
	return ch
}

func fingerprintOf(sc scopeRef, metric string) string {
	return store.AnomalyFingerprint(anomalyDetectors[metric], sc.Kind, sc.ID, metric)
}

func newRow(sc scopeRef, fingerprint string, f finding, occurrences int, now int64) store.Anomaly {
	return store.Anomaly{
		Fingerprint: fingerprint,
		Detector:    anomalyDetectors[f.Metric],
		Metric:      f.Metric,
		Severity:    f.Severity,
		ScopeKind:   sc.Kind, ScopeID: sc.ID, TargetID: sc.TargetID, Domain: sc.Domain,
		RunID: f.RunID, LastRunID: f.RunID, LastRunAt: f.RunAt, LastGoodRunID: f.LastGoodRunID,
		Observed: f.Observed, Expected: f.Expected, Threshold: f.Threshold,
		Samples: f.Samples, Sensitivity: sc.Sensitivity,
		Details:     detailsJSON(f.Details),
		Occurrences: occurrences,
		FirstSeenAt: now, LastSeenAt: now,
	}
}

// byMetric gathers the findings one pass raised for the same fingerprint. A
// pass can judge several runs at once, and a fingerprint has at most one open
// row, so they become one episode with several occurrences.
func byMetric(found []finding) [][]finding {
	var order []string
	groups := map[string][]finding{}
	for _, f := range found {
		if _, seen := groups[f.Metric]; !seen {
			order = append(order, f.Metric)
		}
		groups[f.Metric] = append(groups[f.Metric], f)
	}
	out := make([][]finding, len(order))
	for i, metric := range order {
		out[i] = groups[metric]
	}
	return out
}

// raisedSince drops the events of runs a row of this fingerprint already saw.
// After a restart the detectors judge their window again, and without this an
// acknowledged outlier would come back as a new finding.
func raisedSince(group []finding, lastRunAt int64) []finding {
	out := make([]finding, 0, len(group))
	for _, f := range group {
		if f.RunAt > lastRunAt {
			out = append(out, f)
		}
	}
	return out
}

// detailsJSON renders a finding's figures for the row. An empty map writes
// nothing, so a refresh that carries no new statistics keeps the ones the
// episode opened with.
func detailsJSON(details map[string]any) string {
	if len(details) == 0 {
		return ""
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return ""
	}
	return string(raw)
}
