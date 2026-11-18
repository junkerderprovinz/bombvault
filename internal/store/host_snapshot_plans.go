package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// HostSnapshotPlan is a schedule that takes snapshots of one dataset,
// subvolume or VM on the server and keeps the newest Keep of them.
type HostSnapshotPlan struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	FS         string `json:"fs"`
	Source     string `json:"source"`
	Cadence    string `json:"cadence"`
	Keep       int    `json:"keep"`
	Recursive  bool   `json:"recursive"`
	SkipAsleep bool   `json:"skipAsleep"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  int64  `json:"createdAt"`
	LastRunAt  int64  `json:"lastRunAt"`
	LastCode   string `json:"lastCode"`
	LastDetail string `json:"lastDetail"`
}

// ErrHostSnapshotPlanNotFound is returned for an id no plan has.
var ErrHostSnapshotPlanNotFound = errors.New("host snapshot plan not found")

const hostSnapshotPlanCols = `id, kind, fs, source, cadence, keep, recursive, skip_asleep, enabled, created_at, last_run_at, last_code, last_detail`

func scanHostSnapshotPlan(row interface{ Scan(...any) error }) (HostSnapshotPlan, error) {
	var p HostSnapshotPlan
	var rec, skip, en int
	err := row.Scan(&p.ID, &p.Kind, &p.FS, &p.Source, &p.Cadence, &p.Keep, &rec, &skip, &en, &p.CreatedAt, &p.LastRunAt, &p.LastCode, &p.LastDetail)
	p.Recursive, p.SkipAsleep, p.Enabled = rec != 0, skip != 0, en != 0
	return p, err
}

// ListHostSnapshotPlans returns every plan, oldest first.
func (r *Repo) ListHostSnapshotPlans() ([]HostSnapshotPlan, error) {
	rows, err := r.db.Query(`SELECT ` + hostSnapshotPlanCols + ` FROM host_snapshot_plans ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("ListHostSnapshotPlans: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only cursor
	out := []HostSnapshotPlan{}
	for rows.Next() {
		p, err := scanHostSnapshotPlan(rows)
		if err != nil {
			return nil, fmt.Errorf("ListHostSnapshotPlans: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetHostSnapshotPlan returns one plan.
func (r *Repo) GetHostSnapshotPlan(id string) (HostSnapshotPlan, error) {
	p, err := scanHostSnapshotPlan(r.db.QueryRow(`SELECT `+hostSnapshotPlanCols+` FROM host_snapshot_plans WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrHostSnapshotPlanNotFound
	}
	if err != nil {
		return p, fmt.Errorf("GetHostSnapshotPlan: %w", err)
	}
	return p, nil
}

// AddHostSnapshotPlan stores a new plan and returns it with its id.
func (r *Repo) AddHostSnapshotPlan(p HostSnapshotPlan) (HostSnapshotPlan, error) {
	p.ID = newID()
	_, err := r.db.Exec(`INSERT INTO host_snapshot_plans (`+hostSnapshotPlanCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, '', '')`,
		p.ID, p.Kind, p.FS, p.Source, p.Cadence, p.Keep, boolInt(p.Recursive), boolInt(p.SkipAsleep), boolInt(p.Enabled), p.CreatedAt)
	if err != nil {
		return p, fmt.Errorf("AddHostSnapshotPlan: %w", err)
	}
	return p, nil
}

// UpdateHostSnapshotPlan writes what a person can change about a plan. What
// it snapshots stays, since the snapshots it already took carry its id.
func (r *Repo) UpdateHostSnapshotPlan(p HostSnapshotPlan) error {
	res, err := r.db.Exec(`UPDATE host_snapshot_plans SET cadence = ?, keep = ?, recursive = ?, skip_asleep = ?, enabled = ? WHERE id = ?`,
		p.Cadence, p.Keep, boolInt(p.Recursive), boolInt(p.SkipAsleep), boolInt(p.Enabled), p.ID)
	if err != nil {
		return fmt.Errorf("UpdateHostSnapshotPlan: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrHostSnapshotPlanNotFound
	}
	return nil
}

// RecordHostSnapshotRun notes what the plan's last run did.
func (r *Repo) RecordHostSnapshotRun(id string, at int64, code, detail string) error {
	if _, err := r.db.Exec(`UPDATE host_snapshot_plans SET last_run_at = ?, last_code = ?, last_detail = ? WHERE id = ?`, at, code, detail, id); err != nil {
		return fmt.Errorf("RecordHostSnapshotRun: %w", err)
	}
	return nil
}

// DeleteHostSnapshotPlan removes a plan. Its snapshots stay on the server.
func (r *Repo) DeleteHostSnapshotPlan(id string) error {
	res, err := r.db.Exec(`DELETE FROM host_snapshot_plans WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("DeleteHostSnapshotPlan: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrHostSnapshotPlanNotFound
	}
	return nil
}
