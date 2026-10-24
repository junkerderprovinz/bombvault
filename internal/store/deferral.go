package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// What a deferred backup does once its deadline has passed.
const (
	// AtDeadlineGiveUp records the backup as failed, which puts it under
	// Needs you with its Retry.
	AtDeadlineGiveUp = "give-up"
	// AtDeadlineBackUp backs up although the app is still busy.
	AtDeadlineBackUp = "back-up"
)

// DeferPolicy is how a container's backup waits while its app is busy: the
// activity check that tells, and when a deferred backup is tried again.
type DeferPolicy struct {
	TargetID string
	// RetryMin is how many minutes pass between two tries.
	RetryMin int
	// WindowFrom and WindowTo limit the tries to a time of day, as HH:MM in
	// the server's time zone. A window may cross midnight. Both empty allow
	// any time.
	WindowFrom string
	WindowTo   string
	// MaxHours is how long after the first deferral the deadline falls.
	MaxHours int
	// AtDeadline is AtDeadlineGiveUp or AtDeadlineBackUp.
	AtDeadline string
	// Preset names the media server whose sessions are read before the
	// backup, "" for none.
	Preset string
	// APIKeyEnc is the sealed API key for Jellyfin and Emby.
	APIKeyEnc []byte
	// PausedMin lets a session paused for that many minutes stop counting. 0
	// counts every paused session.
	PausedMin int
	// FailOpen backs up when the server cannot be asked. Without it the
	// backup is deferred.
	FailOpen bool
	// NetKiB defers the backup while the container sent at least that many
	// KiB per second within the last NetMin minutes. 0 switches it off.
	NetKiB int
	NetMin int
}

// DefaultDeferPolicy is what a container that never saved one uses.
func DefaultDeferPolicy(targetID string) DeferPolicy {
	return DeferPolicy{TargetID: targetID, RetryMin: 30, MaxHours: 12, AtDeadline: AtDeadlineGiveUp, FailOpen: true, NetMin: 5}
}

const deferPolicyCols = `target_id, retry_min, window_from, window_to, max_hours, at_deadline, preset, api_key_enc, paused_min, fail_open, net_kib, net_min`

func scanDeferPolicy(s interface{ Scan(...any) error }) (DeferPolicy, error) {
	var p DeferPolicy
	var failOpen int
	err := s.Scan(&p.TargetID, &p.RetryMin, &p.WindowFrom, &p.WindowTo, &p.MaxHours, &p.AtDeadline, &p.Preset, &p.APIKeyEnc, &p.PausedMin, &failOpen, &p.NetKiB, &p.NetMin)
	p.FailOpen = failOpen != 0
	return p, err
}

// DeferPolicyOf returns the target's policy, or the default.
func (r *Repo) DeferPolicyOf(targetID string) (DeferPolicy, error) {
	p, err := scanDeferPolicy(r.db.QueryRow(`SELECT `+deferPolicyCols+` FROM defer_policies WHERE target_id = ?`, targetID))
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultDeferPolicy(targetID), nil
	}
	if err != nil {
		return DeferPolicy{}, fmt.Errorf("DeferPolicyOf: %w", err)
	}
	return p, nil
}

// DeferPolicies returns every stored policy by target id.
func (r *Repo) DeferPolicies() (map[string]DeferPolicy, error) {
	rows, err := r.db.Query(`SELECT ` + deferPolicyCols + ` FROM defer_policies`)
	if err != nil {
		return nil, fmt.Errorf("DeferPolicies: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := map[string]DeferPolicy{}
	for rows.Next() {
		p, err := scanDeferPolicy(rows)
		if err != nil {
			return nil, fmt.Errorf("DeferPolicies: %w", err)
		}
		out[p.TargetID] = p
	}
	return out, rows.Err()
}

// SetDeferPolicy stores p whole.
func (r *Repo) SetDeferPolicy(p DeferPolicy) error {
	_, err := r.db.Exec(`INSERT INTO defer_policies (`+deferPolicyCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(target_id) DO UPDATE SET retry_min = excluded.retry_min, window_from = excluded.window_from,
		  window_to = excluded.window_to, max_hours = excluded.max_hours, at_deadline = excluded.at_deadline,
		  preset = excluded.preset, api_key_enc = excluded.api_key_enc, paused_min = excluded.paused_min,
		  fail_open = excluded.fail_open, net_kib = excluded.net_kib, net_min = excluded.net_min`,
		p.TargetID, p.RetryMin, p.WindowFrom, p.WindowTo, p.MaxHours, p.AtDeadline, p.Preset, p.APIKeyEnc,
		p.PausedMin, boolInt(p.FailOpen), p.NetKiB, p.NetMin)
	if err != nil {
		return fmt.Errorf("SetDeferPolicy: %w", err)
	}
	return nil
}

// Deferral is a container backup that waits for its app, kept so a restart
// picks it up again.
type Deferral struct {
	TargetID  string
	Container string
	// Since is when the backup was deferred the first time, in Unix seconds.
	Since int64
	// NextTry is when it is tried again.
	NextTry int64
	// Tries counts the deferrals so far, the first included.
	Tries int
	// Reason is why the last try was deferred.
	Reason string
}

// SaveDeferral stores d, replacing the one of the same target.
func (r *Repo) SaveDeferral(d Deferral) error {
	_, err := r.db.Exec(`INSERT INTO deferred_backups (target_id, container, since, next_try, tries, reason)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(target_id) DO UPDATE SET container = excluded.container, since = excluded.since,
		  next_try = excluded.next_try, tries = excluded.tries, reason = excluded.reason`,
		d.TargetID, d.Container, d.Since, d.NextTry, d.Tries, d.Reason)
	if err != nil {
		return fmt.Errorf("SaveDeferral: %w", err)
	}
	return nil
}

// GetDeferral returns the waiting backup of a target. ok is false when it
// waits for nothing.
func (r *Repo) GetDeferral(targetID string) (d Deferral, ok bool, err error) {
	err = r.db.QueryRow(`SELECT target_id, container, since, next_try, tries, reason
		FROM deferred_backups WHERE target_id = ?`, targetID).
		Scan(&d.TargetID, &d.Container, &d.Since, &d.NextTry, &d.Tries, &d.Reason)
	if errors.Is(err, sql.ErrNoRows) {
		return Deferral{}, false, nil
	}
	if err != nil {
		return Deferral{}, false, fmt.Errorf("GetDeferral: %w", err)
	}
	return d, true, nil
}

// DeleteDeferral forgets the deferral of a target once its backup ran or
// gave up.
func (r *Repo) DeleteDeferral(targetID string) error {
	if _, err := r.db.Exec(`DELETE FROM deferred_backups WHERE target_id = ?`, targetID); err != nil {
		return fmt.Errorf("DeleteDeferral: %w", err)
	}
	return nil
}

// ListDeferrals returns the waiting backups, the next to try first.
func (r *Repo) ListDeferrals() ([]Deferral, error) {
	rows, err := r.db.Query(`SELECT target_id, container, since, next_try, tries, reason
		FROM deferred_backups ORDER BY next_try, container`)
	if err != nil {
		return nil, fmt.Errorf("ListDeferrals: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	var out []Deferral
	for rows.Next() {
		var d Deferral
		if err := rows.Scan(&d.TargetID, &d.Container, &d.Since, &d.NextTry, &d.Tries, &d.Reason); err != nil {
			return nil, fmt.Errorf("ListDeferrals: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SetRunRetryAt notes on a deferred run when its backup is tried again.
func (r *Repo) SetRunRetryAt(runID string, at int64) error {
	if _, err := r.db.Exec(`UPDATE runs SET retry_at = ? WHERE id = ?`, at, runID); err != nil {
		return fmt.Errorf("SetRunRetryAt: %w", err)
	}
	return nil
}
