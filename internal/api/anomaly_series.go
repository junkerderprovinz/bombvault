package api

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
)

// AnomalyPoint is what one backup measured. At is when the backup was taken,
// the time the backup list shows.
type AnomalyPoint struct {
	RunID string  `json:"runId"`
	At    int64   `json:"at"`
	Value float64 `json:"value"`
}

// AnomalyLimit is one edge of a usual range, in the figures a finding raised on
// it would carry.
type AnomalyLimit struct {
	Metric    string  `json:"metric"`
	Expected  float64 `json:"expected"`
	Threshold float64 `json:"threshold"`
	Samples   int     `json:"samples"`
}

// AnomalyBand is the range detection holds one quantity to. A side is null
// where no rule watches that direction.
type AnomalyBand struct {
	Low  *AnomalyLimit `json:"low"`
	High *AnomalyLimit `json:"high"`
}

// AnomalyQuantity is one measured quantity of a series over time, oldest first.
// Band is null while the rules are learning, unless an open finding already
// holds the quantity to the level it started from.
type AnomalyQuantity struct {
	Quantity string         `json:"quantity"`
	Points   []AnomalyPoint `json:"points"`
	Learning bool           `json:"learning"`
	Samples  int            `json:"samples"`
	Needed   int            `json:"needed"`
	Band     *AnomalyBand   `json:"band"`
}

// AnomalyFailedRun is a run that failed and so left no measurement.
type AnomalyFailedRun struct {
	RunID string `json:"runId"`
	At    int64  `json:"at"`
}

// AnomalySeries is the measured history of one series of an item: its own
// backups, its database dumps, or one dataset of its ZFS tree, which Part then
// names.
type AnomalySeries struct {
	ScopeKind  string             `json:"scopeKind"`
	Part       string             `json:"part"`
	Quantities []AnomalyQuantity  `json:"quantities"`
	Failed     []AnomalyFailedRun `json:"failed"`
}

// AnomalyItemSeries serves what detection measured for one item, series by
// series. It reads and judges each series the way a pass does and writes
// nothing, so a curve and the findings next to it rest on the same figures.
func (s *Service) AnomalyItemSeries(ctx context.Context, targetID string) ([]AnomalySeries, error) {
	e := s.anomalies
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	p := &anomalyPass{settings: settings, now: e.now().Unix()}
	if p.items, err = e.items(settings); err != nil {
		return nil, err
	}
	ref, known := p.items[targetID]
	if !known {
		return nil, fmt.Errorf("%s: %w", targetID, errNotAnAnomalyItem)
	}
	if p.prefs, err = s.store.ListItemPrefs(); err != nil {
		return nil, err
	}

	itemScope := anomalyScope{Kind: anomalyScopeItem, ID: targetID}
	in, _, err := e.readSeries(itemScope, ref, p)
	if err != nil {
		return nil, err
	}
	out := []AnomalySeries{seriesView(itemScope, in)}

	if ref.Domain == anomalyDomainContainer {
		dumpScope := anomalyScope{Kind: anomalyScopeDump, ID: targetID}
		dumps, _, err := e.readSeries(dumpScope, ref, p)
		if err != nil {
			return nil, err
		}
		if len(dumps.Series) > 0 {
			out = append(out, seriesView(dumpScope, dumps))
		}
	}
	if ref.Domain == zfsDomain {
		owners, err := e.datasetOwners(p)
		if err != nil {
			return nil, err
		}
		for _, dataset := range slices.Sorted(maps.Keys(owners)) {
			if owners[dataset] != targetID {
				continue
			}
			sc := anomalyScope{Kind: anomalyScopeZFSDS, ID: dataset}
			members, _, err := e.readDataset(sc, ref, p)
			if err != nil {
				return nil, err
			}
			out = append(out, seriesView(sc, members))
		}
	}
	return out, ctx.Err()
}

// seriesView judges one series and keeps what a curve needs of the verdict.
func seriesView(sc anomalyScope, in itemInput) AnomalySeries {
	view := AnomalySeries{ScopeKind: sc.Kind, Quantities: []AnomalyQuantity{}, Failed: []AnomalyFailedRun{}}
	if sc.Kind == anomalyScopeZFSDS {
		view.Part = sc.ID
	}
	for _, base := range evaluateItem(in).Baselines {
		q := AnomalyQuantity{
			Quantity: base.Quantity, Points: []AnomalyPoint{},
			Learning: base.Samples < anomalyMinSamples, Samples: base.Samples, Needed: anomalyMinSamples,
		}
		for _, m := range newestSamples(base.Points, anomalySeriesRuns) {
			q.Points = append(q.Points, AnomalyPoint{RunID: m.runID, At: m.takenAt, Value: m.value})
		}
		if base.Low != nil || base.High != nil {
			q.Band = &AnomalyBand{Low: (*AnomalyLimit)(base.Low), High: (*AnomalyLimit)(base.High)}
		}
		view.Quantities = append(view.Quantities, q)
	}
	for _, run := range oldestFirst(in.Series) {
		if ownFailure(run) {
			view.Failed = append(view.Failed, AnomalyFailedRun{
				RunID: run.ID, At: backupTakenAt(run.StartedAt, run.FinishedAt),
			})
		}
	}
	return view
}

func (h *Handler) handleAnomalyItemSeries(w http.ResponseWriter, r *http.Request) {
	series, err := h.svc.AnomalyItemSeries(r.Context(), r.PathValue("targetId"))
	switch {
	case errors.Is(err, errNotAnAnomalyItem):
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, "not-found"))
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"series": series}))
	}
}
