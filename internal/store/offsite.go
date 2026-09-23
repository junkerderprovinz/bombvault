package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// TamperTest is one off-site tamper-test verdict for a domain, from an active
// probe of the far side's delete path. A refused delete shows that append-only
// is enforced and not just configured.
type TamperTest struct {
	Domain    string `json:"domain"`
	At        int64  `json:"at"`        // unix seconds the test ran
	Protected bool   `json:"protected"` // true when the delete was refused
	Detail    string `json:"detail"`    // scrubbed status/error; empty when protected
}

// RecordTamperTest records a tamper-test verdict for a domain, stamped now.
func (r *Repo) RecordTamperTest(domain string, protected bool, detail string) error {
	_, err := r.db.Exec(`
		INSERT INTO tamper_tests (domain, at, protected, detail)
		VALUES (?, ?, ?, ?)`,
		domain, time.Now().Unix(), boolInt(protected), detail,
	)
	if err != nil {
		return fmt.Errorf("RecordTamperTest: %w", err)
	}
	return nil
}

// RecordTamperTestForTarget is RecordTamperTest attributed to one off-site
// target. An empty targetID, as for a single target synthesized from Settings,
// leaves offsite_target_id at its default.
func (r *Repo) RecordTamperTestForTarget(domain, targetID string, protected bool, detail string) error {
	if targetID == "" {
		return r.RecordTamperTest(domain, protected, detail)
	}
	_, err := r.db.Exec(`
		INSERT INTO tamper_tests (domain, at, protected, detail, offsite_target_id)
		VALUES (?, ?, ?, ?, ?)`,
		domain, time.Now().Unix(), boolInt(protected), detail, targetID,
	)
	if err != nil {
		return fmt.Errorf("RecordTamperTestForTarget: %w", err)
	}
	return nil
}

// LatestTamperTest returns the most recent tamper test for a domain, or false
// when none exists. Tests within the same second are ordered by insertion.
func (r *Repo) LatestTamperTest(domain string) (TamperTest, bool, error) {
	row := r.db.QueryRow(`
		SELECT domain, at, protected, detail
		FROM tamper_tests
		WHERE domain = ?
		ORDER BY at DESC, rowid DESC
		LIMIT 1`, domain)
	var tt TamperTest
	var protected int
	err := row.Scan(&tt.Domain, &tt.At, &protected, &tt.Detail)
	if errors.Is(err, sql.ErrNoRows) {
		return TamperTest{}, false, nil
	}
	if err != nil {
		return TamperTest{}, false, fmt.Errorf("LatestTamperTest: %w", err)
	}
	tt.Protected = protected != 0
	return tt, true, nil
}

// LatestTamperTestForTarget is LatestTamperTest scoped to one off-site target.
// An empty targetID falls back to LatestTamperTest, which suits a single-target
// install whose rows carry no target id.
func (r *Repo) LatestTamperTestForTarget(domain, targetID string) (TamperTest, bool, error) {
	if targetID == "" {
		return r.LatestTamperTest(domain)
	}
	row := r.db.QueryRow(`
		SELECT domain, at, protected, detail
		FROM tamper_tests
		WHERE domain = ? AND offsite_target_id = ?
		ORDER BY at DESC, rowid DESC
		LIMIT 1`, domain, targetID)
	var tt TamperTest
	var protected int
	err := row.Scan(&tt.Domain, &tt.At, &protected, &tt.Detail)
	if errors.Is(err, sql.ErrNoRows) {
		return TamperTest{}, false, nil
	}
	if err != nil {
		return TamperTest{}, false, fmt.Errorf("LatestTamperTestForTarget: %w", err)
	}
	tt.Protected = protected != 0
	return tt, true, nil
}

// OffsiteRun is one off-site replication run (restic copy) for a domain. Live
// progress goes out over SSE and is not stored.
type OffsiteRun struct {
	Domain     string `json:"domain"`
	StartedAt  int64  `json:"startedAt"`  // unix seconds
	FinishedAt int64  `json:"finishedAt"` // unix seconds; 0 while running
	OK         bool   `json:"ok"`
	Error      string `json:"error"` // scrubbed; empty on success
}

// RecordOffsiteRun records the start of an off-site replication run and returns
// its rowid for FinishOffsiteRun. It leaves out offsite_target_id so it also
// works on a schema older than that column.
func (r *Repo) RecordOffsiteRun(domain string, startedAt int64) (int64, error) {
	res, err := r.db.Exec(`
		INSERT INTO offsite_runs (domain, started_at)
		VALUES (?, ?)`,
		domain, startedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("RecordOffsiteRun: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("RecordOffsiteRun: rowid: %w", err)
	}
	return id, nil
}

// RecordOffsiteRunForTarget is RecordOffsiteRun attributed to one off-site
// target. An empty targetID, as for a single target synthesized from Settings,
// leaves offsite_target_id at its default.
func (r *Repo) RecordOffsiteRunForTarget(domain, targetID string, startedAt int64) (int64, error) {
	if targetID == "" {
		return r.RecordOffsiteRun(domain, startedAt)
	}
	res, err := r.db.Exec(`
		INSERT INTO offsite_runs (domain, started_at, offsite_target_id)
		VALUES (?, ?, ?)`,
		domain, startedAt, targetID,
	)
	if err != nil {
		return 0, fmt.Errorf("RecordOffsiteRun: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("RecordOffsiteRun: rowid: %w", err)
	}
	return id, nil
}

// FinishOffsiteRun stamps the finish time and outcome of a replication run.
// errText must already be scrubbed.
func (r *Repo) FinishOffsiteRun(id int64, ok bool, errText string) error {
	_, err := r.db.Exec(`
		UPDATE offsite_runs SET finished_at = ?, ok = ?, error = ?
		WHERE rowid = ?`,
		time.Now().Unix(), boolInt(ok), errText, id,
	)
	if err != nil {
		return fmt.Errorf("FinishOffsiteRun: %w", err)
	}
	return nil
}

// ReasonCopyRulesUnreadable is the error of a run that copied nothing because the
// placement rules could not be read. It says nothing about the target.
const ReasonCopyRulesUnreadable = "copy rules could not be read"

// MarkOffsiteRunAgingOnly marks the newest run of a domain to a target that
// started at startedAt as one that only listed and aged it. The domain is
// part of the match because the settings-synthesized N=1 target shares the
// empty target id across every domain, and two of them can start in the
// same second.
func (r *Repo) MarkOffsiteRunAgingOnly(domain, targetID string, startedAt int64) error {
	res, err := r.db.Exec(`
		UPDATE offsite_runs SET aging_only = 1
		 WHERE rowid = (SELECT rowid FROM offsite_runs
		                 WHERE domain = ? AND offsite_target_id = ? AND started_at = ?
		                 ORDER BY rowid DESC LIMIT 1)`, domain, targetID, startedAt)
	if err != nil {
		return fmt.Errorf("MarkOffsiteRunAgingOnly: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("MarkOffsiteRunAgingOnly: no run to %s in %s started at %d", targetID, domain, startedAt)
	}
	return nil
}

// LatestOffsiteRun returns the most recently started replication run for a
// domain, running or not, or false when none exists.
func (r *Repo) LatestOffsiteRun(domain string) (OffsiteRun, bool, error) {
	row := r.db.QueryRow(`
		SELECT domain, started_at, finished_at, ok, error
		FROM offsite_runs
		WHERE domain = ?
		ORDER BY started_at DESC, rowid DESC
		LIMIT 1`, domain)
	var run OffsiteRun
	var finished sql.NullInt64
	var ok int
	err := row.Scan(&run.Domain, &run.StartedAt, &finished, &ok, &run.Error)
	if errors.Is(err, sql.ErrNoRows) {
		return OffsiteRun{}, false, nil
	}
	if err != nil {
		return OffsiteRun{}, false, fmt.Errorf("LatestOffsiteRun: %w", err)
	}
	run.FinishedAt = finished.Int64
	run.OK = ok != 0
	return run, true, nil
}

// LatestSuccessfulOffsiteRun returns the most recently started successful
// replication run for a domain, or false when none has succeeded. Skipping
// newer failed runs lets the scorecard see a broken replication as stale.
func (r *Repo) LatestSuccessfulOffsiteRun(domain string) (OffsiteRun, bool, error) {
	row := r.db.QueryRow(`
		SELECT domain, started_at, finished_at, ok, error
		FROM offsite_runs
		WHERE domain = ? AND ok = 1 AND aging_only = 0
		ORDER BY started_at DESC, rowid DESC
		LIMIT 1`, domain)
	var run OffsiteRun
	var finished sql.NullInt64
	var ok int
	err := row.Scan(&run.Domain, &run.StartedAt, &finished, &ok, &run.Error)
	if errors.Is(err, sql.ErrNoRows) {
		return OffsiteRun{}, false, nil
	}
	if err != nil {
		return OffsiteRun{}, false, fmt.Errorf("LatestSuccessfulOffsiteRun: %w", err)
	}
	run.FinishedAt = finished.Int64
	run.OK = ok != 0
	return run, true, nil
}

// LatestSuccessfulOffsiteRunForTarget is LatestSuccessfulOffsiteRun scoped to
// one off-site target. An empty targetID falls back to the domain-wide query.
func (r *Repo) LatestSuccessfulOffsiteRunForTarget(domain, targetID string) (OffsiteRun, bool, error) {
	if targetID == "" {
		return r.LatestSuccessfulOffsiteRun(domain)
	}
	row := r.db.QueryRow(`
		SELECT domain, started_at, finished_at, ok, error
		FROM offsite_runs
		WHERE domain = ? AND ok = 1 AND aging_only = 0 AND offsite_target_id = ?
		ORDER BY started_at DESC, rowid DESC
		LIMIT 1`, domain, targetID)
	var run OffsiteRun
	var finished sql.NullInt64
	var ok int
	err := row.Scan(&run.Domain, &run.StartedAt, &finished, &ok, &run.Error)
	if errors.Is(err, sql.ErrNoRows) {
		return OffsiteRun{}, false, nil
	}
	if err != nil {
		return OffsiteRun{}, false, fmt.Errorf("LatestSuccessfulOffsiteRunForTarget: %w", err)
	}
	run.FinishedAt = finished.Int64
	run.OK = ok != 0
	return run, true, nil
}

// FirstOffsiteFailureAfter returns when the earliest finished, failed run to the
// target began after since, or 0 when there is none. A run that failed because the
// copy rules could not be read says nothing about the target and is left out.
func (r *Repo) FirstOffsiteFailureAfter(targetID string, since int64) (int64, error) {
	var at sql.NullInt64
	err := r.db.QueryRow(`
		SELECT MIN(started_at) FROM offsite_runs
		 WHERE offsite_target_id = ? AND started_at > ?
		   AND finished_at IS NOT NULL AND ok = 0 AND error <> ?`,
		targetID, since, ReasonCopyRulesUnreadable).Scan(&at)
	if err != nil {
		return 0, fmt.Errorf("FirstOffsiteFailureAfter: %w", err)
	}
	return at.Int64, nil
}
