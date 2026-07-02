package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// TamperTest is one recorded off-site tamper-test verdict for a domain: an
// active probe of the far side's delete path. Protected means the delete was
// refused, i.e. append-only is actually enforced (not just configured).
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

// LatestTamperTest returns the most recent tamper test for a domain. The bool
// is false (with a zero TamperTest) when none has been recorded yet. Ties on
// `at` (two tests within the same second) are broken by insertion order.
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

// OffsiteRun is one off-site replication run (restic copy) for a domain:
// begin/end timestamps, outcome and the scrubbed error on failure. restic copy
// has no machine-readable progress, so duration + outcome is all there is.
type OffsiteRun struct {
	Domain     string `json:"domain"`
	StartedAt  int64  `json:"startedAt"`  // unix seconds the run began
	FinishedAt int64  `json:"finishedAt"` // unix seconds it ended; 0 = still running
	OK         bool   `json:"ok"`         // true when the copy succeeded
	Error      string `json:"error"`      // scrubbed error text on failure; empty otherwise
}

// RecordOffsiteRun records the start of an off-site replication run and returns
// the row's id (rowid), to be passed to FinishOffsiteRun when the run ends.
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

// FinishOffsiteRun closes a replication run recorded by RecordOffsiteRun,
// stamping the finish time and the outcome (errText is expected pre-scrubbed).
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

// LatestOffsiteRun returns the most recent replication run for a domain (by
// start time; a still-running row has FinishedAt 0). The bool is false (with a
// zero OffsiteRun) when none has been recorded yet.
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
	run.FinishedAt = finished.Int64 // 0 while the run is still open (NULL)
	run.OK = ok != 0
	return run, true, nil
}

// LatestSuccessfulOffsiteRun returns the most recent SUCCESSFUL replication run
// for a domain (ok=1, by start time). The bool is false (with a zero OffsiteRun)
// when no successful copy has ever landed. Unlike LatestOffsiteRun this ignores a
// newer failed or still-running row, so a broken replication reads as stale (the
// last real off-site copy) rather than fresh — this is the currency source the
// scorecard uses, mirroring how backups use their last SUCCESS.
func (r *Repo) LatestSuccessfulOffsiteRun(domain string) (OffsiteRun, bool, error) {
	row := r.db.QueryRow(`
		SELECT domain, started_at, finished_at, ok, error
		FROM offsite_runs
		WHERE domain = ? AND ok = 1
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
