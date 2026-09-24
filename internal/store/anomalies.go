package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Anomaly is one finding over a series of runs. A row is an episode: it lives
// from the pass that first saw the condition until the pass that saw it gone,
// so acknowledging it settles this occurrence and not the next one.
type Anomaly struct {
	ID, Fingerprint, Detector, Metric, Severity, State string
	ScopeKind, ScopeID, TargetID, Domain               string
	RunID, LastRunID, LastGoodRunID                    string
	LastRunAt                                          int64
	Observed, Expected, Threshold                      float64
	Samples                                            int
	Sensitivity                                        string
	// Details is the JSON object the sentence on the page is built from. It
	// carries numbers and booleans, never a name.
	Details                                                              string
	Occurrences                                                          int
	FirstSeenAt, LastSeenAt, RecoveredAt, ResolvedAt, AckedAt, ClearedAt int64
	AckNote                                                              string
	NotifiedAt                                                           int64
	NotifiedSeverity                                                     string
}

// AnomalyExpectation is what a user marked as expected for one series and one
// family of rules: a re-base for the direction rules, a ceiling for new data.
type AnomalyExpectation struct {
	ScopeKind, ScopeID, TargetID, Family string
	SinceAt                              int64
	Ceiling                              float64
	AnomalyID                            string
	UpdatedAt                            int64
}

// AnomalyBackfillSlot is the state of the one-off read of a repository's
// snapshot summaries into the runs that predate the measurement.
type AnomalyBackfillSlot struct {
	Slot                   string
	AttemptedAt            int64
	Done                   bool
	Filled, WithoutSummary int
	Error                  string
}

// ItemPrefs are one item's overrides. An empty field follows the global
// setting.
type ItemPrefs struct{ Sensitivity, NotifyMin string }

// ScopeState is everything a pass needs to know about one scope before it
// decides: the open rows, the current closed episode per fingerprint, and the
// newest run any state of a fingerprint was raised from.
type ScopeState struct {
	Open          map[string]Anomaly
	ClosedEpisode map[string]Anomaly
	LastRunAt     map[string]int64
}

// AnomalyRefresh carries what a pass writes onto a row that already exists.
type AnomalyRefresh struct {
	ID               string
	Observed         float64
	Severity         string
	LastSeenAt       int64
	LastRunID        string
	LastRunAt        int64
	OccurrencesDelta int
	Details          string
}

// AnomalyChanges is one scope's whole outcome of one pass. Insert adds open
// rows, each starting a new episode at its first occurrence.
type AnomalyChanges struct {
	Insert      []Anomaly
	Refresh     []AnomalyRefresh
	Recover     []string
	Resolve     []string
	Clear       []string
	TouchClosed []AnomalyRefresh
	Now         int64
}

// AnomalyApplyResult names the rows a user changed between the pass's read and
// its write. They are left as the user left them.
type AnomalyApplyResult struct{ Stale []string }

// AnomalyFilter selects rows for a listing. An empty States means the open
// rows, which are never filtered by time.
type AnomalyFilter struct {
	States     []string
	Severities []string
	Detectors  []string
	Domains    []string
	ScopeKind  string
	ScopeID    string
	TargetID   string
	Since      int64
	Limit      int
	Cursor     string
}

const (
	defaultAnomalyLimit = 100
	MaxAnomalyLimit     = 500
	maxAckNoteRunes     = 500
)

const anomalyColumns = `id, fingerprint, detector, metric, severity, state,
		scope_kind, scope_id, target_id, domain,
		run_id, last_run_id, last_run_at, last_good_run_id,
		observed, expected, threshold, samples, sensitivity, details, occurrences,
		first_seen_at, last_seen_at, recovered_at, resolved_at, acked_at, cleared_at,
		ack_note, notified_at, notified_severity`

// AnomalyFingerprint identifies a condition across passes and episodes. The
// severity is not part of it, so a warning that grows into a critical stays the
// same finding. No id it is built from contains a pipe.
func AnomalyFingerprint(detector, scopeKind, scopeID, metric string) string {
	return detector + "|" + scopeKind + "|" + scopeID + "|" + metric
}

// severityRankSQL and notifiedRankSQL order a severity so SQLite can compare
// two of them without a table of ranks.
const (
	severityRankSQL = `CASE severity WHEN 'critical' THEN 3 WHEN 'warning' THEN 2 WHEN 'info' THEN 1 ELSE 0 END`
	notifiedRankSQL = `CASE notified_severity WHEN 'critical' THEN 3 WHEN 'warning' THEN 2 WHEN 'info' THEN 1 ELSE 0 END`
)

func severityRank(severity string) int {
	switch severity {
	case "critical":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	}
	return 0
}

// AnomalyScopeState reads one scope's rows in one query, because a pass decides
// about every fingerprint of the scope at once.
func (r *Repo) AnomalyScopeState(scopeKind, scopeID string) (ScopeState, error) {
	state := ScopeState{
		Open:          map[string]Anomaly{},
		ClosedEpisode: map[string]Anomaly{},
		LastRunAt:     map[string]int64{},
	}
	rows, err := r.db.Query(`
		SELECT `+anomalyColumns+`
		FROM anomalies
		WHERE scope_kind = ? AND scope_id = ?
		ORDER BY last_seen_at DESC, id DESC`, scopeKind, scopeID)
	if err != nil {
		return ScopeState{}, fmt.Errorf("AnomalyScopeState: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	for rows.Next() {
		a, sErr := scanAnomaly(rows)
		if sErr != nil {
			return ScopeState{}, fmt.Errorf("AnomalyScopeState: %w", sErr)
		}
		if a.LastRunAt > state.LastRunAt[a.Fingerprint] {
			state.LastRunAt[a.Fingerprint] = a.LastRunAt
		}
		switch {
		case a.State == "open":
			state.Open[a.Fingerprint] = a
		case a.ClearedAt == 0 && (a.State == "acknowledged" || a.State == "expected"):
			if _, seen := state.ClosedEpisode[a.Fingerprint]; !seen {
				state.ClosedEpisode[a.Fingerprint] = a
			}
		}
	}
	if err := rows.Err(); err != nil {
		return ScopeState{}, fmt.Errorf("AnomalyScopeState: %w", err)
	}
	return state, nil
}

// ApplyAnomalyChanges writes one scope's whole pass in one transaction, in the
// order resolve, recover, clear, refresh, insert: a recovered critical that
// comes back is resolved before its successor is inserted, so the partial
// unique index never sees two open rows of one fingerprint. A row whose state a
// user changed in between matches no update and comes back in Stale.
func (r *Repo) ApplyAnomalyChanges(ch AnomalyChanges) (AnomalyApplyResult, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return AnomalyApplyResult{}, fmt.Errorf("ApplyAnomalyChanges begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var stale []string
	changed := func(res sql.Result, id string) {
		if n, _ := res.RowsAffected(); n == 0 {
			stale = append(stale, id)
		}
	}

	for _, id := range ch.Resolve {
		res, eErr := tx.Exec(`
			UPDATE anomalies SET state = 'resolved', resolved_at = ?, cleared_at = ?
			WHERE id = ? AND state = 'open'`, ch.Now, ch.Now, id)
		if eErr != nil {
			return AnomalyApplyResult{}, fmt.Errorf("ApplyAnomalyChanges resolve: %w", eErr)
		}
		changed(res, id)
	}
	for _, id := range ch.Recover {
		res, eErr := tx.Exec(`
			UPDATE anomalies SET recovered_at = ?
			WHERE id = ? AND state = 'open'`, ch.Now, id)
		if eErr != nil {
			return AnomalyApplyResult{}, fmt.Errorf("ApplyAnomalyChanges recover: %w", eErr)
		}
		changed(res, id)
	}
	for _, id := range ch.Clear {
		res, eErr := tx.Exec(`
			UPDATE anomalies SET cleared_at = ?
			WHERE id = ? AND cleared_at = 0`, ch.Now, id)
		if eErr != nil {
			return AnomalyApplyResult{}, fmt.Errorf("ApplyAnomalyChanges clear: %w", eErr)
		}
		changed(res, id)
	}
	for _, f := range ch.TouchClosed {
		res, eErr := tx.Exec(`
			UPDATE anomalies SET last_seen_at = ?, observed = ?
			WHERE id = ? AND cleared_at = 0`, f.LastSeenAt, f.Observed, f.ID)
		if eErr != nil {
			return AnomalyApplyResult{}, fmt.Errorf("ApplyAnomalyChanges touch: %w", eErr)
		}
		changed(res, f.ID)
	}
	for _, f := range ch.Refresh {
		res, eErr := tx.Exec(`
			UPDATE anomalies
			SET last_seen_at = ?, observed = ?, last_run_id = ?, last_run_at = ?,
			    occurrences = occurrences + ?,
			    details = CASE WHEN ? = '' THEN details ELSE ? END,
			    severity = CASE WHEN ? > `+severityRankSQL+` THEN ? ELSE severity END
			WHERE id = ? AND state = 'open'`,
			f.LastSeenAt, f.Observed, f.LastRunID, f.LastRunAt,
			f.OccurrencesDelta,
			f.Details, f.Details,
			severityRank(f.Severity), f.Severity,
			f.ID,
		)
		if eErr != nil {
			return AnomalyApplyResult{}, fmt.Errorf("ApplyAnomalyChanges refresh: %w", eErr)
		}
		changed(res, f.ID)
	}
	for _, a := range ch.Insert {
		if a.ID == "" {
			a.ID = newID()
		}
		if a.Details == "" {
			a.Details = "{}"
		}
		if a.Occurrences == 0 {
			a.Occurrences = 1
		}
		_, eErr := tx.Exec(`
			INSERT INTO anomalies (
				id, fingerprint, detector, metric, severity,
				scope_kind, scope_id, target_id, domain,
				run_id, last_run_id, last_run_at, last_good_run_id,
				observed, expected, threshold, samples, sensitivity, details, occurrences,
				first_seen_at, last_seen_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			a.ID, a.Fingerprint, a.Detector, a.Metric, a.Severity,
			a.ScopeKind, a.ScopeID, a.TargetID, a.Domain,
			a.RunID, a.LastRunID, a.LastRunAt, a.LastGoodRunID,
			a.Observed, a.Expected, a.Threshold, a.Samples, a.Sensitivity, a.Details, a.Occurrences,
			a.FirstSeenAt, a.LastSeenAt,
		)
		if eErr != nil {
			return AnomalyApplyResult{}, fmt.Errorf("ApplyAnomalyChanges insert %s: %w", a.Fingerprint, eErr)
		}
	}

	if err := tx.Commit(); err != nil {
		return AnomalyApplyResult{}, fmt.Errorf("ApplyAnomalyChanges commit: %w", err)
	}
	return AnomalyApplyResult{Stale: stale}, nil
}

// ListAnomalies returns one page of findings and the cursor for the next. A
// listing of open rows alone is ordered by severity first, and its cursor
// carries the rank, so a page boundary between two severities skips nothing.
func (r *Repo) ListAnomalies(f AnomalyFilter) ([]Anomaly, string, error) {
	openOnly := len(f.States) == 0 || (len(f.States) == 1 && f.States[0] == "open")
	limit := f.Limit
	if limit <= 0 {
		limit = defaultAnomalyLimit
	}
	if limit > MaxAnomalyLimit {
		limit = MaxAnomalyLimit
	}

	where := []string{}
	args := []any{}
	add := func(clause string, values ...any) {
		where = append(where, clause)
		args = append(args, values...)
	}
	if openOnly {
		add(`state = 'open'`)
	} else {
		clause, values := inClause("state", f.States)
		add(clause, values...)
	}
	if len(f.Severities) > 0 {
		clause, values := inClause("severity", f.Severities)
		add(clause, values...)
	}
	if len(f.Detectors) > 0 {
		clause, values := inClause("detector", f.Detectors)
		add(clause, values...)
	}
	if len(f.Domains) > 0 {
		clause, values := inClause("domain", f.Domains)
		add(clause, values...)
	}
	if f.ScopeKind != "" {
		add(`scope_kind = ?`, f.ScopeKind)
	}
	if f.ScopeID != "" {
		add(`scope_id = ?`, f.ScopeID)
	}
	if f.TargetID != "" {
		add(`target_id = ?`, f.TargetID)
	}
	if f.Since > 0 {
		add(`(state = 'open' OR last_seen_at >= ?)`, f.Since)
	}

	order := `last_seen_at DESC, id DESC`
	if openOnly {
		order = severityRankSQL + ` DESC, ` + order
	}
	if f.Cursor != "" {
		rank, lastSeenAt, id, err := parseAnomalyCursor(f.Cursor)
		if err != nil {
			return nil, "", err
		}
		if openOnly {
			add(`(`+severityRankSQL+` < ? OR (`+severityRankSQL+` = ? AND (last_seen_at < ? OR (last_seen_at = ? AND id < ?))))`,
				rank, rank, lastSeenAt, lastSeenAt, id)
		} else {
			add(`(last_seen_at < ? OR (last_seen_at = ? AND id < ?))`, lastSeenAt, lastSeenAt, id)
		}
	}

	//nolint:gosec // G202: every fragment is a literal from this function; all
	// values travel as bound parameters in args.
	query := `SELECT ` + anomalyColumns + `
		FROM anomalies
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY ` + order + `
		LIMIT ?`
	rows, err := r.db.Query(query, append(args, limit)...)
	if err != nil {
		return nil, "", fmt.Errorf("ListAnomalies: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []Anomaly
	for rows.Next() {
		a, sErr := scanAnomaly(rows)
		if sErr != nil {
			return nil, "", fmt.Errorf("ListAnomalies: %w", sErr)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("ListAnomalies: %w", err)
	}

	next := ""
	if len(out) == limit {
		last := out[len(out)-1]
		rank := 0
		if openOnly {
			rank = severityRank(last.Severity)
		}
		next = strconv.Itoa(rank) + ":" + strconv.FormatInt(last.LastSeenAt, 10) + ":" + last.ID
	}
	return out, next, nil
}

// HeldAnomalies returns every open critical finding of the given metrics that
// has not recovered, oldest episode last and with no page limit. These are the
// rows that pause deleting old backups, and a flood of unrelated criticals must
// not push one of them out of sight.
func (r *Repo) HeldAnomalies(metrics []string) ([]Anomaly, error) {
	clause, args := inClause("metric", metrics)
	//nolint:gosec // G202: the clause is built from a literal and placeholders.
	rows, err := r.db.Query(`SELECT `+anomalyColumns+`
		FROM anomalies
		WHERE state = 'open' AND severity = 'critical' AND recovered_at = 0 AND `+clause+`
		ORDER BY last_seen_at DESC, id DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("HeldAnomalies: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []Anomaly
	for rows.Next() {
		a, sErr := scanAnomaly(rows)
		if sErr != nil {
			return nil, fmt.Errorf("HeldAnomalies: %w", sErr)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("HeldAnomalies: %w", err)
	}
	return out, nil
}

// GetAnomaly reads one finding. The bool is false when the id is unknown.
func (r *Repo) GetAnomaly(id string) (Anomaly, bool, error) {
	row := r.db.QueryRow(`SELECT `+anomalyColumns+` FROM anomalies WHERE id = ?`, id)
	a, err := scanAnomaly(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Anomaly{}, false, nil
	}
	if err != nil {
		return Anomaly{}, false, fmt.Errorf("GetAnomaly: %w", err)
	}
	return a, true, nil
}

// AcknowledgeAnomalies settles the current episode of every open row named and
// returns the rows it changed. A row someone already closed is left alone.
func (r *Repo) AcknowledgeAnomalies(ids []string, note string, now int64) ([]Anomaly, error) {
	return r.closeAnomalies(ids, "acknowledged", note, now)
}

// MarkAnomaliesExpected settles the episode like AcknowledgeAnomalies and marks
// the condition as the new normal. The expectation itself is a row of its own.
func (r *Repo) MarkAnomaliesExpected(ids []string, note string, now int64) ([]Anomaly, error) {
	return r.closeAnomalies(ids, "expected", note, now)
}

func (r *Repo) closeAnomalies(ids []string, state, note string, now int64) ([]Anomaly, error) {
	if runes := []rune(note); len(runes) > maxAckNoteRunes {
		note = string(runes[:maxAckNoteRunes])
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("closeAnomalies begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var closed []string
	for _, id := range ids {
		res, eErr := tx.Exec(`
			UPDATE anomalies SET state = ?, acked_at = ?, ack_note = ?
			WHERE id = ? AND state = 'open'`, state, now, note, id)
		if eErr != nil {
			return nil, fmt.Errorf("closeAnomalies %s: %w", id, eErr)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			closed = append(closed, id)
		}
	}
	var out []Anomaly
	for _, id := range closed {
		row := tx.QueryRow(`SELECT `+anomalyColumns+` FROM anomalies WHERE id = ?`, id)
		a, sErr := scanAnomaly(row)
		if sErr != nil {
			return nil, fmt.Errorf("closeAnomalies read %s: %w", id, sErr)
		}
		out = append(out, a)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("closeAnomalies commit: %w", err)
	}
	return out, nil
}

// PendingAnomalyNotifications lists the open rows that have not been sent at
// their current severity, so an escalation is sent once more and a restart
// delivers what is still open.
func (r *Repo) PendingAnomalyNotifications() ([]Anomaly, error) {
	rows, err := r.db.Query(`
		SELECT ` + anomalyColumns + `
		FROM anomalies
		WHERE state = 'open' AND ` + notifiedRankSQL + ` < ` + severityRankSQL + `
		ORDER BY last_seen_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("PendingAnomalyNotifications: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []Anomaly
	for rows.Next() {
		a, sErr := scanAnomaly(rows)
		if sErr != nil {
			return nil, fmt.Errorf("PendingAnomalyNotifications: %w", sErr)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// MarkAnomaliesNotified stamps the severity a row was sent at.
func (r *Repo) MarkAnomaliesNotified(ids []string, severityByID map[string]string, now int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("MarkAnomaliesNotified begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	for _, id := range ids {
		if _, eErr := tx.Exec(`
			UPDATE anomalies SET notified_at = ?, notified_severity = ?
			WHERE id = ?`, now, severityByID[id], id); eErr != nil {
			return fmt.Errorf("MarkAnomaliesNotified %s: %w", id, eErr)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("MarkAnomaliesNotified commit: %w", err)
	}
	return nil
}

// PruneAnomalies deletes closed findings last touched before the cutoff. An
// open row stays whatever its age, and so does the current episode of a
// condition the user closed while it was still present: deleting that one would
// raise the condition again on the next pass.
func (r *Repo) PruneAnomalies(before int64) (int64, error) {
	res, err := r.db.Exec(`
		DELETE FROM anomalies
		WHERE state IN ('resolved', 'acknowledged', 'expected')
		  AND NOT (state IN ('acknowledged', 'expected') AND cleared_at = 0)
		  AND max(resolved_at, acked_at, last_seen_at) < ?`, before)
	if err != nil {
		return 0, fmt.Errorf("PruneAnomalies: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// OpenAnomalyCounts returns the open rows by severity and how many of the open
// criticals have recovered, which is what the card and the sidebar show.
func (r *Repo) OpenAnomalyCounts() (map[string]int, int, error) {
	rows, err := r.db.Query(`
		SELECT severity, count(*), sum(CASE WHEN recovered_at > 0 THEN 1 ELSE 0 END)
		FROM anomalies WHERE state = 'open' GROUP BY severity`)
	if err != nil {
		return nil, 0, fmt.Errorf("OpenAnomalyCounts: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	counts := map[string]int{}
	recovered := 0
	for rows.Next() {
		var severity string
		var n, rec int
		if sErr := rows.Scan(&severity, &n, &rec); sErr != nil {
			return nil, 0, fmt.Errorf("OpenAnomalyCounts: %w", sErr)
		}
		counts[severity] = n
		if severity == "critical" {
			recovered = rec
		}
	}
	return counts, recovered, rows.Err()
}

// ListItemPrefs returns the per-item overrides by target id.
func (r *Repo) ListItemPrefs() (map[string]ItemPrefs, error) {
	rows, err := r.db.Query(`SELECT target_id, sensitivity, notify_min FROM anomaly_item_prefs`)
	if err != nil {
		return nil, fmt.Errorf("ListItemPrefs: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := map[string]ItemPrefs{}
	for rows.Next() {
		var id string
		var p ItemPrefs
		if sErr := rows.Scan(&id, &p.Sensitivity, &p.NotifyMin); sErr != nil {
			return nil, fmt.Errorf("ListItemPrefs: %w", sErr)
		}
		out[id] = p
	}
	return out, rows.Err()
}

// SetItemPrefs stores one item's overrides. Both fields empty means the item
// follows the global settings again, and its row goes.
func (r *Repo) SetItemPrefs(targetID string, p ItemPrefs) error {
	if p.Sensitivity == "" && p.NotifyMin == "" {
		if _, err := r.db.Exec(`DELETE FROM anomaly_item_prefs WHERE target_id = ?`, targetID); err != nil {
			return fmt.Errorf("SetItemPrefs delete: %w", err)
		}
		return nil
	}
	_, err := r.db.Exec(`
		INSERT INTO anomaly_item_prefs (target_id, sensitivity, notify_min)
		VALUES (?, ?, ?)
		ON CONFLICT(target_id) DO UPDATE SET sensitivity = excluded.sensitivity, notify_min = excluded.notify_min`,
		targetID, p.Sensitivity, p.NotifyMin)
	if err != nil {
		return fmt.Errorf("SetItemPrefs: %w", err)
	}
	return nil
}

// ListAnomalyExpectations returns what one series is expected to do.
func (r *Repo) ListAnomalyExpectations(scopeKind, scopeID string) ([]AnomalyExpectation, error) {
	return r.anomalyExpectations(`scope_kind = ? AND scope_id = ?`, scopeKind, scopeID)
}

// ListAnomalyExpectationsForTarget returns the expectations of every series of
// one item, which is how the Items tab lists them with their Forget action.
func (r *Repo) ListAnomalyExpectationsForTarget(targetID string) ([]AnomalyExpectation, error) {
	return r.anomalyExpectations(`target_id = ?`, targetID)
}

func (r *Repo) anomalyExpectations(condition string, args ...any) ([]AnomalyExpectation, error) {
	//nolint:gosec // G202: condition is one of the two literals above; the values
	// travel as bound parameters.
	rows, err := r.db.Query(`
		SELECT scope_kind, scope_id, target_id, family, since_at, ceiling, anomaly_id, updated_at
		FROM anomaly_expectations
		WHERE `+condition+`
		ORDER BY scope_kind, scope_id, family`, args...)
	if err != nil {
		return nil, fmt.Errorf("ListAnomalyExpectations: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []AnomalyExpectation
	for rows.Next() {
		var e AnomalyExpectation
		if sErr := rows.Scan(&e.ScopeKind, &e.ScopeID, &e.TargetID, &e.Family,
			&e.SinceAt, &e.Ceiling, &e.AnomalyID, &e.UpdatedAt); sErr != nil {
			return nil, fmt.Errorf("ListAnomalyExpectations: %w", sErr)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpsertAnomalyExpectation widens what is expected and never narrows it: a
// second "mark as expected" on a smaller finding must not put the larger one
// back into the alarm.
func (r *Repo) UpsertAnomalyExpectation(e AnomalyExpectation) error {
	_, err := r.db.Exec(`
		INSERT INTO anomaly_expectations (scope_kind, scope_id, target_id, family, since_at, ceiling, anomaly_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(scope_kind, scope_id, family) DO UPDATE SET
			target_id  = excluded.target_id,
			since_at   = max(since_at, excluded.since_at),
			ceiling    = max(ceiling, excluded.ceiling),
			anomaly_id = excluded.anomaly_id,
			updated_at = excluded.updated_at`,
		e.ScopeKind, e.ScopeID, e.TargetID, e.Family, e.SinceAt, e.Ceiling, e.AnomalyID, e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("UpsertAnomalyExpectation: %w", err)
	}
	return nil
}

// DeleteAnomalyExpectation forgets one expectation, so its rule reports again.
func (r *Repo) DeleteAnomalyExpectation(scopeKind, scopeID, family string) error {
	_, err := r.db.Exec(`
		DELETE FROM anomaly_expectations
		WHERE scope_kind = ? AND scope_id = ? AND family = ?`, scopeKind, scopeID, family)
	if err != nil {
		return fmt.Errorf("DeleteAnomalyExpectation: %w", err)
	}
	return nil
}

// ListAnomalyBackfill returns the state of every backfill slot.
func (r *Repo) ListAnomalyBackfill() ([]AnomalyBackfillSlot, error) {
	rows, err := r.db.Query(`
		SELECT slot, attempted_at, done, filled, without_summary, error
		FROM anomaly_backfill ORDER BY slot`)
	if err != nil {
		return nil, fmt.Errorf("ListAnomalyBackfill: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []AnomalyBackfillSlot
	for rows.Next() {
		var s AnomalyBackfillSlot
		var done int
		if sErr := rows.Scan(&s.Slot, &s.AttemptedAt, &done, &s.Filled, &s.WithoutSummary, &s.Error); sErr != nil {
			return nil, fmt.Errorf("ListAnomalyBackfill: %w", sErr)
		}
		s.Done = done != 0
		out = append(out, s)
	}
	return out, rows.Err()
}

// RecordAnomalyBackfill stores a slot's newest attempt in place of the one
// before it.
func (r *Repo) RecordAnomalyBackfill(s AnomalyBackfillSlot) error {
	_, err := r.db.Exec(`
		INSERT INTO anomaly_backfill (slot, attempted_at, done, filled, without_summary, error)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(slot) DO UPDATE SET
			attempted_at    = excluded.attempted_at,
			done            = excluded.done,
			filled          = excluded.filled,
			without_summary = excluded.without_summary,
			error           = excluded.error`,
		s.Slot, s.AttemptedAt, boolInt(s.Done), s.Filled, s.WithoutSummary, s.Error)
	if err != nil {
		return fmt.Errorf("RecordAnomalyBackfill: %w", err)
	}
	return nil
}

// deleteAnomalyState removes the findings, preferences and expectations of the
// items whose target id the predicate matches. It runs inside the transaction
// that deletes the item and its runs, so a deleted item never leaves a finding
// behind that nothing resolves to any more.
func deleteAnomalyState(tx *sql.Tx, predicate string, args ...any) error {
	for _, table := range []string{"anomalies", "anomaly_item_prefs", "anomaly_expectations"} {
		//nolint:gosec // G202: table and predicate are literals from this package;
		// the values travel as bound parameters.
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE target_id `+predicate, args...); err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}
	}
	return nil
}

// inClause builds a "column IN (?, ?, …)" fragment and the arguments that fill
// it.
func inClause(column string, values []string) (string, []any) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")
	args := make([]any, 0, len(values))
	for _, v := range values {
		args = append(args, v)
	}
	return column + ` IN (` + placeholders + `)`, args
}

func parseAnomalyCursor(cursor string) (rank int, lastSeenAt int64, id string, err error) {
	parts := strings.SplitN(cursor, ":", 3)
	if len(parts) != 3 {
		return 0, 0, "", fmt.Errorf("ListAnomalies: malformed cursor %q", cursor)
	}
	rank, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, "", fmt.Errorf("ListAnomalies: malformed cursor %q", cursor)
	}
	lastSeenAt, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("ListAnomalies: malformed cursor %q", cursor)
	}
	return rank, lastSeenAt, parts[2], nil
}

func scanAnomaly(s scanner) (Anomaly, error) {
	var a Anomaly
	err := s.Scan(
		&a.ID, &a.Fingerprint, &a.Detector, &a.Metric, &a.Severity, &a.State,
		&a.ScopeKind, &a.ScopeID, &a.TargetID, &a.Domain,
		&a.RunID, &a.LastRunID, &a.LastRunAt, &a.LastGoodRunID,
		&a.Observed, &a.Expected, &a.Threshold, &a.Samples, &a.Sensitivity, &a.Details, &a.Occurrences,
		&a.FirstSeenAt, &a.LastSeenAt, &a.RecoveredAt, &a.ResolvedAt, &a.AckedAt, &a.ClearedAt,
		&a.AckNote, &a.NotifiedAt, &a.NotifiedSeverity,
	)
	if err != nil {
		return Anomaly{}, err
	}
	return a, nil
}
