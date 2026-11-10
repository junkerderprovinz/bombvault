package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// SharedUpkeep is how the owner keeps a shared repository: the rules it ages
// every member's history by, the window it prunes in, and what its last
// passes found.
type SharedUpkeep struct {
	RepoID string
	// Keep is the How long to keep card of the repository. Until it is saved,
	// each section ages by the owner's own off-site rules for that section.
	Keep    RetentionKeep
	KeepSet bool
	// PruneCadence opens the window the owner prunes in, and PruneHours is how
	// long the members hold back their copies once they agreed.
	PruneCadence string
	PruneHours   int
	// MaxUnused is the share of unused space in percent a prune leaves rather
	// than repacking it, and CheckSubset the share of the data the check after
	// it reads back.
	MaxUnused   int
	CheckSubset int
	// PrunedWindow is when the last window opened that the owner pruned in or
	// tried to. A window it gave up on for a busy member is not tried again.
	PrunedWindow int64
	Forget       *SharedForgetReport
	Prune        *SharedPruneReport
	Space        *SharedSpace
}

// SharedForgetReport is what the owner's last forget pass did.
type SharedForgetReport struct {
	At        int64 `json:"at"`
	Forgotten int   `json:"forgotten"`
	// Skipped names the members whose history was left alone because they
	// could not say which of their entries are held.
	Skipped []string `json:"skipped,omitempty"`
	// Unknown are the hosts in the repository no member writes under. Their
	// snapshots are never forgotten.
	Unknown []string `json:"unknown,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// SharedPruneReport is what the owner's last prune window brought.
type SharedPruneReport struct {
	At      int64 `json:"at"`
	Seconds int64 `json:"seconds"`
	// Postponed names the members still copying when the owner stopped
	// waiting, which moved the prune to the next window.
	Postponed  []string `json:"postponed,omitempty"`
	Error      string   `json:"error,omitempty"`
	Checked    bool     `json:"checked"`
	CheckError string   `json:"checkError,omitempty"`
}

// SharedSpace is what a shared repository holds and what each member's
// snapshots use of it. The members apart would need the sum.
type SharedSpace struct {
	At    int64             `json:"at"`
	Total int64             `json:"total"`
	Hosts []SharedHostSpace `json:"hosts"`
}

// SharedHostSpace is the data the snapshots of one member use.
type SharedHostSpace struct {
	Host  string `json:"host"`
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

func defaultSharedUpkeep(id string) SharedUpkeep {
	return SharedUpkeep{RepoID: id, PruneCadence: "weekly sun 03:00", PruneHours: 6, MaxUnused: 5, CheckSubset: 5}
}

// SharedUpkeepOf returns the upkeep of the shared repository id, the default
// one while nothing was saved for it.
func (r *Repo) SharedUpkeepOf(id string) (SharedUpkeep, error) {
	u := defaultSharedUpkeep(id)
	var keepSet int
	var forget, prune, space string
	err := r.db.QueryRow(`SELECT keep_last, keep_daily, keep_weekly, keep_monthly, keep_yearly, keep_set,
		prune_cadence, prune_hours, max_unused, check_subset, pruned_window, forget_report, prune_report, space_report
		FROM shared_repo_upkeep WHERE shared_repo_id = ?`, id).Scan(
		&u.Keep.KeepLast, &u.Keep.KeepDaily, &u.Keep.KeepWeekly, &u.Keep.KeepMonthly, &u.Keep.KeepYearly, &keepSet,
		&u.PruneCadence, &u.PruneHours, &u.MaxUnused, &u.CheckSubset, &u.PrunedWindow, &forget, &prune, &space)
	if errors.Is(err, sql.ErrNoRows) {
		return u, nil
	}
	if err != nil {
		return SharedUpkeep{}, fmt.Errorf("SharedUpkeepOf: %w", err)
	}
	u.KeepSet = keepSet != 0
	if u.Forget, err = decodeReport[SharedForgetReport](forget); err == nil {
		if u.Prune, err = decodeReport[SharedPruneReport](prune); err == nil {
			u.Space, err = decodeReport[SharedSpace](space)
		}
	}
	if err != nil {
		return SharedUpkeep{}, fmt.Errorf("SharedUpkeepOf: %w", err)
	}
	return u, nil
}

func decodeReport[T any](raw string) (*T, error) {
	if raw == "" {
		return nil, nil
	}
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// ensureSharedUpkeep gives the shared repository id its row with the default
// upkeep, so the writes below only ever update.
func (r *Repo) ensureSharedUpkeep(id string) error {
	d := defaultSharedUpkeep(id)
	_, err := r.db.Exec(`INSERT INTO shared_repo_upkeep (shared_repo_id, prune_cadence, prune_hours, max_unused, check_subset)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT(shared_repo_id) DO NOTHING`, id, d.PruneCadence, d.PruneHours, d.MaxUnused, d.CheckSubset)
	return err
}

// PutSharedUpkeepRules stores the rules of u and leaves what the passes found
// as it is.
func (r *Repo) PutSharedUpkeepRules(u SharedUpkeep) error {
	keepSet := 0
	if u.KeepSet {
		keepSet = 1
	}
	_, err := r.db.Exec(`INSERT INTO shared_repo_upkeep (shared_repo_id, keep_last, keep_daily, keep_weekly, keep_monthly, keep_yearly, keep_set,
		prune_cadence, prune_hours, max_unused, check_subset) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(shared_repo_id) DO UPDATE SET keep_last = excluded.keep_last, keep_daily = excluded.keep_daily,
		keep_weekly = excluded.keep_weekly, keep_monthly = excluded.keep_monthly, keep_yearly = excluded.keep_yearly,
		keep_set = excluded.keep_set, prune_cadence = excluded.prune_cadence, prune_hours = excluded.prune_hours,
		max_unused = excluded.max_unused, check_subset = excluded.check_subset`,
		u.RepoID, u.Keep.KeepLast, u.Keep.KeepDaily, u.Keep.KeepWeekly, u.Keep.KeepMonthly, u.Keep.KeepYearly, keepSet,
		u.PruneCadence, u.PruneHours, u.MaxUnused, u.CheckSubset)
	if err != nil {
		return fmt.Errorf("PutSharedUpkeepRules: %w", err)
	}
	return nil
}

// NoteSharedForget keeps what the last forget pass over id did.
func (r *Repo) NoteSharedForget(id string, f SharedForgetReport) error {
	return r.noteSharedReport(id, "forget_report = ?", f)
}

// NoteSharedPrune keeps what the prune window that opened at window brought.
func (r *Repo) NoteSharedPrune(id string, window int64, p SharedPruneReport) error {
	return r.noteSharedReport(id, "prune_report = ?, pruned_window = ?", p, window)
}

// NoteSharedSpace keeps the last measurement of id.
func (r *Repo) NoteSharedSpace(id string, s SharedSpace) error {
	return r.noteSharedReport(id, "space_report = ?", s)
}

func (r *Repo) noteSharedReport(id, set string, report any, more ...any) error {
	b, err := json.Marshal(report)
	if err != nil {
		return err
	}
	if err := r.ensureSharedUpkeep(id); err != nil {
		return fmt.Errorf("note shared upkeep: %w", err)
	}
	args := append([]any{string(b)}, more...)
	if _, err := r.db.Exec(`UPDATE shared_repo_upkeep SET `+set+` WHERE shared_repo_id = ?`, append(args, id)...); err != nil { //nolint:gosec // G202: set is one of the fixed column lists above
		return fmt.Errorf("note shared upkeep: %w", err)
	}
	return nil
}

// SetSharedSpaceFrom keeps on a member what the owner with group instance id
// owner last measured in the shared repository it holds.
func (r *Repo) SetSharedSpaceFrom(owner string, s SharedSpace) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if _, err := r.db.Exec(`INSERT INTO shared_repo_space (owner, space) VALUES (?, ?)
		ON CONFLICT(owner) DO UPDATE SET space = excluded.space`, owner, string(b)); err != nil {
		return fmt.Errorf("SetSharedSpaceFrom: %w", err)
	}
	return nil
}

// SharedSpaceFrom returns what the owner owner last measured, if it said.
func (r *Repo) SharedSpaceFrom(owner string) (SharedSpace, bool, error) {
	var raw string
	err := r.db.QueryRow(`SELECT space FROM shared_repo_space WHERE owner = ?`, owner).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return SharedSpace{}, false, nil
	}
	if err != nil {
		return SharedSpace{}, false, fmt.Errorf("SharedSpaceFrom: %w", err)
	}
	var s SharedSpace
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return SharedSpace{}, false, fmt.Errorf("SharedSpaceFrom: %w", err)
	}
	return s, true, nil
}
