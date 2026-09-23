package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The service API of anomaly detection. It serves the findings and the items
// behind them, and it is where a user closes an episode. The backend sends
// numbers and ids; the sentence a reader sees is built in the frontend.

// anomalyActionLimit caps one bulk action, so a filter that matched everything
// cannot turn into an unbounded transaction.
const anomalyActionLimit = 500

// newDataCeilingFactor is the headroom an accepted amount of new data keeps, so
// a series that repeats it at roughly the same size stays quiet.
const newDataCeilingFactor = 1.25

var (
	errNotAnAnomalyItem    = errors.New("not a backed-up item")
	errUnknownSensitivity  = errors.New("unknown sensitivity preset")
	errUnknownNotifyMin    = errors.New("unknown notification minimum")
	errUnknownAnomalyScope = errors.New("unknown scope kind")
)

// AnomalyView is one finding with everything the page needs to write a sentence
// about it.
type AnomalyView struct {
	ID          string `json:"id"`
	Detector    string `json:"detector"`
	Metric      string `json:"metric"`
	Severity    string `json:"severity"`
	State       string `json:"state"`
	ScopeKind   string `json:"scopeKind"`
	ScopeID     string `json:"scopeId"`
	TargetID    string `json:"targetId"`
	Domain      string `json:"domain"`
	Name        string `json:"name"`
	Part        string `json:"part"`
	TargetName  string `json:"targetName"`
	RunID       string `json:"runId"`
	LastRunID   string `json:"lastRunId"`
	LastRunAt   int64  `json:"lastRunAt"`
	LastGoodRun string `json:"lastGoodRunId"`

	Observed  float64 `json:"observed"`
	Expected  float64 `json:"expected"`
	Threshold float64 `json:"threshold"`

	Samples     int            `json:"samples"`
	Sensitivity string         `json:"sensitivity"`
	Details     map[string]any `json:"details"`
	Occurrences int            `json:"occurrences"`

	FirstSeenAt int64 `json:"firstSeenAt"`
	LastSeenAt  int64 `json:"lastSeenAt"`
	RecoveredAt int64 `json:"recoveredAt"`
	ResolvedAt  int64 `json:"resolvedAt"`
	AckedAt     int64 `json:"ackedAt"`
	ClearedAt   int64 `json:"clearedAt"`

	AckNote       string `json:"ackNote"`
	NotifiedAt    int64  `json:"notifiedAt"`
	Expectable    bool   `json:"expectable"`
	RetentionHeld bool   `json:"retentionHeld"`
	StillPresent  bool   `json:"stillPresent"`
}

// AnomalyPage is one page of findings and the cursor that continues it.
type AnomalyPage struct {
	Anomalies  []AnomalyView `json:"anomalies"`
	NextCursor string        `json:"nextCursor"`
}

// AnomalyOpenCounts is how many findings of each severity are open.
type AnomalyOpenCounts struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
}

// AnomalyBackfillSummary is how far the one-off read of the repositories' own
// history has come.
type AnomalyBackfillSummary struct {
	Slots          int `json:"slots"`
	Done           int `json:"done"`
	Failed         int `json:"failed"`
	Filled         int `json:"filled"`
	WithoutSummary int `json:"withoutSummary"`
}

// AnomalySummary is the small figure set the sidebar, the dashboard card and
// the settings page poll. It is served from memory.
type AnomalySummary struct {
	Enabled           bool                   `json:"enabled"`
	Ready             bool                   `json:"ready"`
	Generation        int64                  `json:"generation"`
	Open              AnomalyOpenCounts      `json:"open"`
	RecoveredCritical int                    `json:"recoveredCritical"`
	LearningItems     int                    `json:"learningItems"`
	RetentionHeld     int                    `json:"retentionHeld"`
	EvalErrors        int                    `json:"evalErrors"`
	NotifyMuted       bool                   `json:"notifyMuted"`
	Backfill          AnomalyBackfillSummary `json:"backfill"`
	UnmeasuredVolumes []string               `json:"unmeasuredVolumes"`
}

// AnomalyLearning is how much history each family of rules has behind it.
type AnomalyLearning struct {
	Samples  int  `json:"samples"`
	Needed   int  `json:"needed"`
	NewData  int  `json:"newData"`
	Source   int  `json:"source"`
	Duration int  `json:"duration"`
	NoData   bool `json:"noData"`
}

// AnomalyTypical are an item's usual figures, null per field while the rule
// behind it is still learning.
type AnomalyTypical struct {
	SourceBytes  *int64 `json:"sourceBytes"`
	NewDataBytes *int64 `json:"newDataBytes"`
	ResticMS     *int64 `json:"resticMs"`
}

// AnomalySeriesLearning is the learning state of a series that has one set of
// rules rather than all of them.
type AnomalySeriesLearning struct {
	Samples int `json:"samples"`
	Needed  int `json:"needed"`
}

// AnomalySeriesTypical are the usual figures of such a series.
type AnomalySeriesTypical struct {
	SourceBytes *int64 `json:"sourceBytes"`
	ResticMS    *int64 `json:"resticMs"`
}

// AnomalySeriesInfo is a series that belongs to an item without being its own
// item: the database dumps of a container, or one dataset of a ZFS tree.
type AnomalySeriesInfo struct {
	Part          string                `json:"part"`
	Learning      AnomalySeriesLearning `json:"learning"`
	Typical       AnomalySeriesTypical  `json:"typical"`
	Open          AnomalyOpenCounts     `json:"open"`
	RetentionHeld bool                  `json:"retentionHeld"`
}

// AnomalyExpectationView is one thing a user declared normal for a series.
type AnomalyExpectationView struct {
	Family    string  `json:"family"`
	ScopeKind string  `json:"scopeKind"`
	Part      string  `json:"part"`
	SinceAt   int64   `json:"sinceAt"`
	Ceiling   float64 `json:"ceiling"`
	UpdatedAt int64   `json:"updatedAt"`
}

// AnomalyItem is one backed-up item on the Items tab: what it usually does,
// what it has learned, and the settings it follows.
type AnomalyItem struct {
	TargetID           string                   `json:"targetId"`
	Domain             string                   `json:"domain"`
	Name               string                   `json:"name"`
	Scheduled          bool                     `json:"scheduled"`
	Sensitivity        string                   `json:"sensitivity"`
	Effective          string                   `json:"effective"`
	NotifyMin          string                   `json:"notifyMin"`
	EffectiveNotifyMin string                   `json:"effectiveNotifyMin"`
	Learning           AnomalyLearning          `json:"learning"`
	Typical            AnomalyTypical           `json:"typical"`
	Dump               *AnomalySeriesInfo       `json:"dump"`
	Datasets           []AnomalySeriesInfo      `json:"datasets"`
	Open               AnomalyOpenCounts        `json:"open"`
	RetentionHeld      bool                     `json:"retentionHeld"`
	SelectionSince     int64                    `json:"selectionSince"`
	Expectations       []AnomalyExpectationView `json:"expectations"`
}

// anomalyFamilies maps a metric to the expectation a user can record about it.
// A metric that is not in here cannot be marked as expected; new_data_full is,
// with no expectation to write, because a first full upload is information and
// happens once.
var anomalyFamilies = map[string]string{
	metricNewData:            familyNewData,
	metricNewDataRewrite:     familyNewData,
	metricNewDataFull:        "",
	metricSourceBytesShrink:  familySourceBytesDown,
	metricSourceBytesGrowth:  familySourceBytesUp,
	metricSourceFilesShrink:  familySourceFilesDown,
	metricDumpBytesShrink:    familyDumpBytesDown,
	metricDumpBytesGrowth:    familyDumpBytesUp,
	metricDurationSlower:     familyDuration,
	metricDumpDurationSlower: familyDumpDuration,
}

// holdingMetrics are the critical findings that pause deleting old backups of
// their series: the three ways a source can lose data, and the run that rewrote
// most of it.
var holdingMetrics = []string{
	metricSourceBytesShrink, metricSourceFilesShrink, metricDumpBytesShrink, metricNewDataRewrite,
}

// ListAnomalies serves one page of findings, each with the name and the flags
// the page needs next to the row's own figures.
func (s *Service) ListAnomalies(ctx context.Context, f store.AnomalyFilter) (AnomalyPage, error) {
	rows, cursor, err := s.store.ListAnomalies(f)
	if err != nil {
		return AnomalyPage{}, err
	}
	items, err := s.anomalyItemRefs()
	if err != nil {
		return AnomalyPage{}, err
	}
	page := AnomalyPage{Anomalies: make([]AnomalyView, 0, len(rows)), NextCursor: cursor}
	targets := s.offsiteTargetNames(rows)
	for _, row := range rows {
		page.Anomalies = append(page.Anomalies, anomalyViewOf(row, items, targets))
	}
	return page, ctx.Err()
}

// GetAnomaly serves one finding by id.
func (s *Service) GetAnomaly(ctx context.Context, id string) (AnomalyView, bool, error) {
	row, found, err := s.store.GetAnomaly(id)
	if err != nil || !found {
		return AnomalyView{}, found, err
	}
	items, err := s.anomalyItemRefs()
	if err != nil {
		return AnomalyView{}, false, err
	}
	return anomalyViewOf(row, items, s.offsiteTargetNames([]store.Anomaly{row})), true, ctx.Err()
}

// offsiteTargetNames resolves the named off-site targets the given drill rows
// point at, so a failing copy can be named rather than numbered.
func (s *Service) offsiteTargetNames(rows []store.Anomaly) map[string]string {
	if !slices.ContainsFunc(rows, func(a store.Anomaly) bool { return offsiteTargetOf(a) != "" }) {
		return nil
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		log.Printf("anomaly: list off-site targets: %v", err)
		return nil
	}
	out := make(map[string]string, len(targets))
	for _, t := range targets {
		out[t.ID] = t.Name
	}
	return out
}

// AcknowledgeAnomalies settles the episodes the user has seen and releases the
// retention holds they were carrying.
func (s *Service) AcknowledgeAnomalies(ctx context.Context, ids []string, note string) (int, int, error) {
	return s.closeAnomalyEpisodes(ctx, ids, note, false)
}

// MarkAnomaliesExpected settles the episodes and records the new level or
// amount as normal, so the same condition is not reported again.
func (s *Service) MarkAnomaliesExpected(ctx context.Context, ids []string, note string) (int, int, error) {
	return s.closeAnomalyEpisodes(ctx, ids, note, true)
}

func (s *Service) closeAnomalyEpisodes(ctx context.Context, ids []string, note string, expected bool) (int, int, error) {
	wanted := dedupedIDs(ids)
	skipped := 0
	if len(wanted) > anomalyActionLimit {
		skipped = len(wanted) - anomalyActionLimit
		wanted = wanted[:anomalyActionLimit]
	}

	var closable []store.Anomaly
	for _, id := range wanted {
		row, found, err := s.store.GetAnomaly(id)
		switch {
		case err != nil:
			return 0, 0, err
		case !found || row.State != "open":
			skipped++
		case expected && !anomalyExpectable(row.Metric):
			skipped++
		default:
			closable = append(closable, row)
		}
	}
	if len(closable) == 0 {
		return 0, skipped, ctx.Err()
	}

	release := s.anomalies.lockScopes(anomalyScopesOf(closable))
	defer release()

	now := s.anomalies.nowUnix()
	closeRows := s.store.AcknowledgeAnomalies
	state := "acknowledged"
	if expected {
		closeRows, state = s.store.MarkAnomaliesExpected, "expected"
	}
	closed, err := closeRows(anomalyIDsOf(closable), note, now)
	if err != nil {
		return 0, 0, err
	}
	if expected {
		for _, row := range closed {
			if wErr := s.writeAnomalyExpectation(row, now); wErr != nil {
				return 0, 0, wErr
			}
		}
	}
	log.Printf("anomaly: %d finding(s) %s from the UI: %v", len(closed), state, anomalyIDsOf(closed))

	s.anomalies.markScopesDirty(anomalyScopesOf(closed))
	s.anomalies.refresh()
	return len(closed), skipped + len(closable) - len(closed), ctx.Err()
}

// writeAnomalyExpectation records what the user just declared normal: a ceiling
// for an amount of new data, and for every other rule the run from which the
// new level counts.
func (s *Service) writeAnomalyExpectation(row store.Anomaly, now int64) error {
	family := anomalyFamilies[row.Metric]
	if family == "" {
		return nil
	}
	e := store.AnomalyExpectation{
		ScopeKind: row.ScopeKind, ScopeID: row.ScopeID, TargetID: row.TargetID,
		Family: family, AnomalyID: row.ID, UpdatedAt: now,
	}
	if family == familyNewData {
		e.Ceiling = newDataCeilingFactor * row.Observed
	} else {
		e.SinceAt = row.LastRunAt
	}
	return s.store.UpsertAnomalyExpectation(e)
}

// AnomalySummary serves the figures the SPA polls, from memory.
func (s *Service) AnomalySummary(context.Context) AnomalySummary {
	return s.anomalies.summary()
}

// AnomalyItems lists the items detection watches: the scheduled ones and those
// with a backup of their own in the last ninety days.
func (s *Service) AnomalyItems(context.Context) ([]AnomalyItem, error) {
	return s.anomalies.itemViews(), nil
}

// SetItemAnomalyPrefs stores one item's overrides. An empty field means the
// item follows the global setting again.
func (s *Service) SetItemAnomalyPrefs(ctx context.Context, targetID string, p store.ItemPrefs) error {
	items, err := s.anomalyItemRefs()
	if err != nil {
		return err
	}
	if _, known := items[targetID]; !known {
		return fmt.Errorf("%s: %w", targetID, errNotAnAnomalyItem)
	}
	if p.Sensitivity != "" && !slices.Contains(anomalyPresets, Sensitivity(p.Sensitivity)) {
		return fmt.Errorf("%s: %w", p.Sensitivity, errUnknownSensitivity)
	}
	if p.NotifyMin != "" && !slices.Contains(anomalyNotifyLevels, p.NotifyMin) {
		return fmt.Errorf("%s: %w", p.NotifyMin, errUnknownNotifyMin)
	}

	scopes := []anomalyScope{
		{Kind: anomalyScopeItem, ID: targetID},
		{Kind: anomalyScopeDump, ID: targetID},
	}
	release := s.anomalies.lockScopes(scopes)
	if err := s.store.SetItemPrefs(targetID, p); err != nil {
		release()
		return err
	}
	release()

	s.anomalies.markScopesDirty(scopes)
	s.anomalies.refresh()
	return ctx.Err()
}

// ForgetAnomalyExpectation drops one expectation, so its rule watches the
// series again from the next pass on.
func (s *Service) ForgetAnomalyExpectation(ctx context.Context, targetID, scopeKind, part, family string) error {
	scopeID := targetID
	switch scopeKind {
	case anomalyScopeItem, anomalyScopeDump:
	case anomalyScopeZFSDS:
		scopeID = part
	default:
		return fmt.Errorf("%s: %w", scopeKind, errUnknownAnomalyScope)
	}

	scope := anomalyScope{Kind: scopeKind, ID: scopeID}
	release := s.anomalies.lockScopes([]anomalyScope{scope})
	err := s.store.DeleteAnomalyExpectation(scopeKind, scopeID, family)
	release()
	if err != nil {
		return err
	}

	s.anomalies.markScopesDirty([]anomalyScope{scope})
	s.anomalies.refresh()
	return ctx.Err()
}

// StartAnomalyEngine runs the worker that evaluates the history after every
// backup, until ctx is cancelled.
func (s *Service) StartAnomalyEngine(ctx context.Context) {
	s.anomalies.Start(ctx)
}

func anomalyViewOf(row store.Anomaly, items map[string]anomalyItemRef, targets map[string]string) AnomalyView {
	view := AnomalyView{
		ID: row.ID, Detector: row.Detector, Metric: row.Metric,
		Severity: row.Severity, State: row.State,
		ScopeKind: row.ScopeKind, ScopeID: row.ScopeID, TargetID: row.TargetID, Domain: row.Domain,
		Name:  items[row.TargetID].Name,
		RunID: row.RunID, LastRunID: row.LastRunID, LastRunAt: row.LastRunAt,
		LastGoodRun: row.LastGoodRunID,
		Observed:    row.Observed, Expected: row.Expected, Threshold: row.Threshold,
		Samples: row.Samples, Sensitivity: row.Sensitivity,
		Details: anomalyDetails(row.Details), Occurrences: row.Occurrences,
		FirstSeenAt: row.FirstSeenAt, LastSeenAt: row.LastSeenAt,
		RecoveredAt: row.RecoveredAt, ResolvedAt: row.ResolvedAt,
		AckedAt: row.AckedAt, ClearedAt: row.ClearedAt,
		AckNote: row.AckNote, NotifiedAt: row.NotifiedAt,
		Expectable:    anomalyExpectable(row.Metric),
		RetentionHeld: anomalyHolds(row),
		StillPresent:  row.ClearedAt == 0 && (row.State == "acknowledged" || row.State == "expected"),
	}
	if row.ScopeKind == anomalyScopeZFSDS {
		view.Part = row.ScopeID
	}
	view.TargetName = targets[offsiteTargetOf(row)]
	return view
}

// offsiteTargetOf reads the named off-site target out of a drill scope's id.
func offsiteTargetOf(row store.Anomaly) string {
	if row.ScopeKind != anomalyScopeDomain {
		return ""
	}
	_, id, _ := strings.Cut(row.ScopeID, ":offsite:")
	return id
}

// anomalyHolds is the one predicate behind a retention hold, a held badge and
// the count in the summary, so they can never disagree.
func anomalyHolds(row store.Anomaly) bool {
	return row.State == "open" && row.Severity == "critical" &&
		row.RecoveredAt == 0 && slices.Contains(holdingMetrics, row.Metric)
}

func anomalyExpectable(metric string) bool {
	_, ok := anomalyFamilies[metric]
	return ok
}

func anomalyDetails(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func anomalyIDsOf(rows []store.Anomaly) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.ID
	}
	return out
}

func anomalyScopesOf(rows []store.Anomaly) []anomalyScope {
	var out []anomalyScope
	for _, row := range rows {
		sc := anomalyScope{Kind: row.ScopeKind, ID: row.ScopeID}
		if !slices.Contains(out, sc) {
			out = append(out, sc)
		}
	}
	return out
}

func dedupedIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
