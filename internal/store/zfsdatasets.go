package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"
)

// ZFSDataset is one backup item of the ZFS domain: a root dataset that is
// backed up together with its descendants, from one recursive snapshot.
type ZFSDataset struct {
	ID string
	// Dataset is the root of the item's tree and is immutable after the
	// create. The member datasets' names are the restic identities
	// (zfs:<dataset>), so a different root is a different item; a tree renamed
	// on the host comes back as a new row and this one keeps its history.
	Dataset string
	// Enabled gates the item's participation in scheduled and whole-domain runs.
	Enabled bool
	// Excludes are restic --exclude patterns, anchored at the item's logical
	// tree rather than at the per-run snapshot directory.
	Excludes []string
	// ExcludedChildren are descendant datasets left out with their subtrees.
	// Only SetZFSExcludedChildren writes it.
	ExcludedChildren []string
	// ScheduleCadence is the item's optional per-item schedule. Empty follows
	// the domain, "off" takes the item out of scheduling. Only
	// SetZFSDatasetScheduleCadence writes it.
	ScheduleCadence string
	// Repo is the ID of the item's own named repository (a RoleRepo row in
	// offsite_targets), empty for the domain repository (Settings.ZFSPath).
	// Only SetZFSDatasetRepo writes it.
	Repo string
	// StopContainers are the containers stopped for the snapshot instant, so
	// the snapshot holds a consistent state. Only SetZFSDatasetStopContainers
	// writes it.
	StopContainers []string
	// RestartPending records the containers of a consistency stop before the
	// first one goes down and is cleared once they are all back. A run that
	// dies inside the window leaves the list, and the boot-time recovery reads
	// it to start the apps again.
	RestartPending []string
	// HookContainer is where PreSnapshot and PostSnapshot run. A failing
	// PreSnapshot fails the run; a failing PostSnapshot is reported.
	HookContainer string
	PreSnapshot   string
	PostSnapshot  string
	// LastCheckCode is the reason code of the newest preflight, probe or run,
	// with LastCheckDetail the scrubbed output that explains it. Empty means
	// the item has never been checked.
	LastCheckCode   string
	LastCheckDetail string
	LastCheckAt     int64
	// LastHostMountpoint is where the root was mounted at the last preflight.
	// The recovery kit and the offline view need it while the host is down.
	LastHostMountpoint string
	// LeftoverCount is how many of BombVault's own snapshot stamps the last
	// sweep still found on the tree. Anything above zero pins pool space.
	LeftoverCount     int
	LeftoverCheckedAt int64
	CreatedAt         int64
}

// ZFSMember is one dataset of an item's tree as the last preflight or run found
// it.
type ZFSMember struct {
	ItemID, Dataset, HostMountpoint, Outcome, Detail string
	FirstSeenAt, LastBackupAt, UsedByDataset         int64
}

// ZFSRun is the detail of one ZFS run that the runs table has no column for.
type ZFSRun struct {
	RunID, ItemID, SnapshotName, HookDetail string
	// WindowSeconds is how long the apps were held for the snapshot instant,
	// -1 when the item stops nothing.
	WindowSeconds int64
}

// ZFSRunMember is what one run did to one dataset of the tree.
type ZFSRunMember struct {
	RunID           string `json:"runId"`
	Dataset         string `json:"dataset"`
	Outcome         string `json:"outcome"`
	ResticSnapshot  string `json:"resticSnapshot"`
	IsNew           bool   `json:"isNew"`
	BytesAdded      int64  `json:"bytesAdded"`
	FilesNew        int64  `json:"filesNew"`
	FilesChanged    int64  `json:"filesChanged"`
	FilesUnmodified int64  `json:"filesUnmodified"`
	DurationMS      int64  `json:"durationMs"`
}

// ZFSSafetySnapshot is a snapshot taken before an in-place restore. It is kept
// until the user deletes it, so the sweeper that removes leaked backup stamps
// has to be able to tell it apart.
type ZFSSafetySnapshot struct {
	ItemID    string `json:"itemId"`
	Dataset   string `json:"dataset"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
	UsedBytes int64  `json:"usedBytes"`
}

const zfsDatasetColumns = `id, dataset, enabled, excludes, excluded_children, schedule_cadence, repo,
	stop_containers, restart_pending, hook_container, pre_snapshot, post_snapshot,
	last_check_code, last_check_detail, last_check_at, last_host_mountpoint,
	leftover_count, leftover_checked_at, created_at`

// CreateZFSDataset inserts a new item. An empty ID is assigned via newID(); a
// dataset that is already an item fails (dataset is UNIQUE).
func (r *Repo) CreateZFSDataset(d ZFSDataset) (ZFSDataset, error) {
	if d.ID == "" {
		d.ID = newID()
	}
	if d.CreatedAt == 0 {
		d.CreatedAt = time.Now().Unix()
	}
	excludes, err := marshalList(d.Excludes)
	if err != nil {
		return ZFSDataset{}, fmt.Errorf("CreateZFSDataset marshal excludes: %w", err)
	}
	children, err := marshalList(d.ExcludedChildren)
	if err != nil {
		return ZFSDataset{}, fmt.Errorf("CreateZFSDataset marshal excluded children: %w", err)
	}
	stop, err := marshalList(d.StopContainers)
	if err != nil {
		return ZFSDataset{}, fmt.Errorf("CreateZFSDataset marshal stop list: %w", err)
	}

	// The INSERT carries the columns the setters own, unlike UpdateZFSDataset:
	// a new item has nothing to overwrite, and a separate setter call would
	// leave a window in which the item runs without its stop list or on the
	// domain repository while the caller believes otherwise.
	_, err = r.db.Exec(`
		INSERT INTO zfs_datasets (id, dataset, enabled, excludes, excluded_children, repo,
			stop_containers, hook_container, pre_snapshot, post_snapshot, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Dataset, boolInt(d.Enabled), excludes, children, d.Repo,
		stop, d.HookContainer, d.PreSnapshot, d.PostSnapshot, d.CreatedAt,
	)
	if err != nil {
		return ZFSDataset{}, fmt.Errorf("CreateZFSDataset: %w", err)
	}
	return d, nil
}

// UpdateZFSDataset writes enabled and excludes for the item with d.ID. Every
// other column belongs to its own setter, so an edit that does not know about
// the stop list, the hooks or the pending restart cannot drop them.
func (r *Repo) UpdateZFSDataset(d ZFSDataset) error {
	excludes, err := marshalList(d.Excludes)
	if err != nil {
		return fmt.Errorf("UpdateZFSDataset marshal excludes: %w", err)
	}
	return r.updateZFSDataset("UpdateZFSDataset", d.ID,
		`UPDATE zfs_datasets SET enabled = ?, excludes = ? WHERE id = ?`,
		boolInt(d.Enabled), excludes, d.ID)
}

// SetZFSExcludedChildren writes the descendants left out of the item's tree
// together with their own children.
func (r *Repo) SetZFSExcludedChildren(id string, names []string) error {
	encoded, err := marshalList(names)
	if err != nil {
		return fmt.Errorf("SetZFSExcludedChildren marshal: %w", err)
	}
	return r.updateZFSDataset("SetZFSExcludedChildren", id,
		`UPDATE zfs_datasets SET excluded_children = ? WHERE id = ?`, encoded, id)
}

// SetZFSHooks writes the container the snapshot commands run in and the two
// commands themselves.
func (r *Repo) SetZFSHooks(id, container, pre, post string) error {
	return r.updateZFSDataset("SetZFSHooks", id,
		`UPDATE zfs_datasets SET hook_container = ?, pre_snapshot = ?, post_snapshot = ? WHERE id = ?`,
		container, pre, post, id)
}

// SetZFSDatasetEnabled switches the item on or off. It touches nothing else, so
// it works while the host is unreachable.
func (r *Repo) SetZFSDatasetEnabled(id string, enabled bool) error {
	return r.updateZFSDataset("SetZFSDatasetEnabled", id,
		`UPDATE zfs_datasets SET enabled = ? WHERE id = ?`, boolInt(enabled), id)
}

// SetZFSDatasetScheduleCadence writes the item's per-item schedule override. An
// empty string puts it back on the domain schedule.
func (r *Repo) SetZFSDatasetScheduleCadence(id, cadence string) error {
	return r.updateZFSDataset("SetZFSDatasetScheduleCadence", id,
		`UPDATE zfs_datasets SET schedule_cadence = ? WHERE id = ?`, cadence, id)
}

// SetZFSDatasetRepo writes the item's named repository. An empty string puts it
// back on the domain repository (Settings.ZFSPath). The API tier checks that
// the ID names a repository that exists and is switched on.
func (r *Repo) SetZFSDatasetRepo(id, repo string) error {
	return r.updateZFSDataset("SetZFSDatasetRepo", id,
		`UPDATE zfs_datasets SET repo = ? WHERE id = ?`, repo, id)
}

// SetZFSDatasetStopContainers writes the containers stopped for the snapshot
// instant.
func (r *Repo) SetZFSDatasetStopContainers(id string, names []string) error {
	encoded, err := marshalList(names)
	if err != nil {
		return fmt.Errorf("SetZFSDatasetStopContainers marshal: %w", err)
	}
	return r.updateZFSDataset("SetZFSDatasetStopContainers", id,
		`UPDATE zfs_datasets SET stop_containers = ? WHERE id = ?`, encoded, id)
}

// SetZFSRestartPending records the containers a consistency stop is about to
// take down. It is written before the first one goes down, so a crash inside
// the window leaves a list the boot-time recovery can act on.
func (r *Repo) SetZFSRestartPending(id string, names []string) error {
	encoded, err := marshalList(names)
	if err != nil {
		return fmt.Errorf("SetZFSRestartPending marshal: %w", err)
	}
	return r.updateZFSDataset("SetZFSRestartPending", id,
		`UPDATE zfs_datasets SET restart_pending = ? WHERE id = ?`, encoded, id)
}

// ClearZFSRestartPending drops the marker once the containers are running again.
func (r *Repo) ClearZFSRestartPending(id string) error {
	return r.updateZFSDataset("ClearZFSRestartPending", id,
		`UPDATE zfs_datasets SET restart_pending = '[]' WHERE id = ?`, id)
}

// SetZFSCheck records the newest reason code for the item and returns the code
// it replaced, so the caller can notify on a change rather than on every run.
// The detail is cut to 2000 bytes here, at a character boundary.
func (r *Repo) SetZFSCheck(id, code, detail, hostMountpoint string, at int64) (string, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return "", fmt.Errorf("SetZFSCheck begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	var prev string
	if err := tx.QueryRow(`SELECT last_check_code FROM zfs_datasets WHERE id = ?`, id).Scan(&prev); err != nil {
		return "", fmt.Errorf("SetZFSCheck read previous: %w", err)
	}
	_, err = tx.Exec(`
		UPDATE zfs_datasets
		SET last_check_code = ?, last_check_detail = ?, last_check_at = ?, last_host_mountpoint = ?
		WHERE id = ?`,
		code, cutToBytes(detail, 2000), at, hostMountpoint, id,
	)
	if err != nil {
		return "", fmt.Errorf("SetZFSCheck: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("SetZFSCheck commit: %w", err)
	}
	return prev, nil
}

// SetZFSLeftovers records how many of BombVault's snapshot stamps the last
// sweep still found on the item's tree.
func (r *Repo) SetZFSLeftovers(id string, count int, at int64) error {
	return r.updateZFSDataset("SetZFSLeftovers", id,
		`UPDATE zfs_datasets SET leftover_count = ?, leftover_checked_at = ? WHERE id = ?`, count, at, id)
}

// AddZFSLeftover counts one more snapshot stamp that could not be removed. It
// adds to the stored count rather than to a copy the caller read, because a
// sweep may have rewritten the count since.
func (r *Repo) AddZFSLeftover(id string, at int64) error {
	return r.updateZFSDataset("AddZFSLeftover", id,
		`UPDATE zfs_datasets SET leftover_count = leftover_count + 1, leftover_checked_at = ? WHERE id = ?`, at, id)
}

// ListZFSDatasets returns every item ordered by its root dataset.
func (r *Repo) ListZFSDatasets() ([]ZFSDataset, error) {
	return r.zfsDatasets("ListZFSDatasets", `SELECT `+zfsDatasetColumns+` FROM zfs_datasets ORDER BY dataset`)
}

// ListZFSRestartPending returns the items whose containers a consistency stop
// took down and never started again.
func (r *Repo) ListZFSRestartPending() ([]ZFSDataset, error) {
	return r.zfsDatasets("ListZFSRestartPending", `SELECT `+zfsDatasetColumns+`
		FROM zfs_datasets WHERE restart_pending != '[]' ORDER BY dataset`)
}

func (r *Repo) zfsDatasets(label, query string, args ...any) ([]ZFSDataset, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []ZFSDataset
	for rows.Next() {
		d, err := scanZFSDataset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetZFSDataset returns the item with the given id.
func (r *Repo) GetZFSDataset(id string) (ZFSDataset, error) {
	row := r.db.QueryRow(`SELECT `+zfsDatasetColumns+` FROM zfs_datasets WHERE id = ?`, id)
	return scanZFSDataset(row)
}

// GetZFSDatasetByName returns the item whose root is the given (unique) dataset.
func (r *Repo) GetZFSDatasetByName(dataset string) (ZFSDataset, error) {
	row := r.db.QueryRow(`SELECT `+zfsDatasetColumns+` FROM zfs_datasets WHERE dataset = ?`, dataset)
	return scanZFSDataset(row)
}

// DeleteZFSDataset removes an item with its runs and every detail row that
// belongs to it, in one transaction. The safety snapshots on the host are not
// touched. Deleting an item that does not exist is not an error.
func (r *Repo) DeleteZFSDataset(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("DeleteZFSDataset begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	del := func(what, query string, args ...any) error {
		if _, err := tx.Exec(query, args...); err != nil {
			return fmt.Errorf("DeleteZFSDataset %s: %w", what, err)
		}
		return nil
	}
	// A run interrupted before it recorded its zfs_runs row still has members,
	// so both ways of reaching them are followed.
	if err := del("run members", `DELETE FROM zfs_run_members
		WHERE run_id IN (SELECT id FROM runs WHERE target_id = ?)
		   OR run_id IN (SELECT run_id FROM zfs_runs WHERE item_id = ?)`, id, id); err != nil {
		return err
	}
	if err := del("run detail", `DELETE FROM zfs_runs WHERE item_id = ?`, id); err != nil {
		return err
	}
	if err := del("members", `DELETE FROM zfs_members WHERE item_id = ?`, id); err != nil {
		return err
	}
	if err := del("safety snapshots", `DELETE FROM zfs_safety_snapshots WHERE item_id = ?`, id); err != nil {
		return err
	}
	if err := del("runs", `DELETE FROM runs WHERE target_id = ?`, id); err != nil {
		return err
	}
	if err := deleteAnomalyState(tx, `= ?`, id); err != nil {
		return fmt.Errorf("DeleteZFSDataset anomaly state: %w", err)
	}
	if err := del("item", `DELETE FROM zfs_datasets WHERE id = ?`, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("DeleteZFSDataset commit: %w", err)
	}
	return nil
}

// ReplaceZFSMembers rewrites an item's member list from what the host reports,
// in one transaction. A dataset that was already there keeps its first sighting
// and its last backup time, so a preflight that knows neither cannot erase the
// history that makes a child picked up later visible as new.
func (r *Repo) ReplaceZFSMembers(itemID string, ms []ZFSMember) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("ReplaceZFSMembers begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	known, err := knownZFSMembers(tx, itemID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM zfs_members WHERE item_id = ?`, itemID); err != nil {
		return fmt.Errorf("ReplaceZFSMembers clear: %w", err)
	}

	now := time.Now().Unix()
	for _, m := range ms {
		firstSeen, lastBackup := m.FirstSeenAt, m.LastBackupAt
		if seen, ok := known[m.Dataset]; ok {
			if seen.FirstSeenAt != 0 {
				firstSeen = seen.FirstSeenAt
			}
			if lastBackup == 0 {
				lastBackup = seen.LastBackupAt
			}
		}
		if firstSeen == 0 {
			firstSeen = now
		}
		_, err := tx.Exec(`
			INSERT INTO zfs_members (item_id, dataset, host_mountpoint, outcome, detail,
				first_seen_at, last_backup_at, used_by_dataset)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			itemID, m.Dataset, m.HostMountpoint, m.Outcome, cutToBytes(m.Detail, 2000),
			firstSeen, lastBackup, m.UsedByDataset,
		)
		if err != nil {
			return fmt.Errorf("ReplaceZFSMembers %s: %w", m.Dataset, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ReplaceZFSMembers commit: %w", err)
	}
	return nil
}

// SetZFSMemberOutcome records what a run did to one member of the tree. A
// backedUpAt of zero keeps the member's last backup time.
func (r *Repo) SetZFSMemberOutcome(itemID, dataset, outcome string, backedUpAt int64) error {
	_, err := r.db.Exec(`
		UPDATE zfs_members
		SET outcome = ?, last_backup_at = CASE WHEN ? > 0 THEN ? ELSE last_backup_at END
		WHERE item_id = ? AND dataset = ?`,
		outcome, backedUpAt, backedUpAt, itemID, dataset)
	if err != nil {
		return fmt.Errorf("SetZFSMemberOutcome %s: %w", dataset, err)
	}
	return nil
}

// ListZFSMembers returns an item's tree as the last preflight or run found it,
// ordered by dataset.
func (r *Repo) ListZFSMembers(itemID string) ([]ZFSMember, error) {
	rows, err := r.db.Query(`
		SELECT item_id, dataset, host_mountpoint, outcome, detail, first_seen_at, last_backup_at, used_by_dataset
		FROM zfs_members WHERE item_id = ? ORDER BY dataset`, itemID)
	if err != nil {
		return nil, fmt.Errorf("ListZFSMembers: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []ZFSMember
	for rows.Next() {
		var m ZFSMember
		if err := rows.Scan(&m.ItemID, &m.Dataset, &m.HostMountpoint, &m.Outcome, &m.Detail,
			&m.FirstSeenAt, &m.LastBackupAt, &m.UsedByDataset); err != nil {
			return nil, fmt.Errorf("ListZFSMembers scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// RecordZFSRun writes what the runs table has no column for: the snapshot stamp
// that names the restore point, how long the apps were held (-1 without a stop
// list) and what a post-snapshot command complained about.
func (r *Repo) RecordZFSRun(runID, itemID, snapName string, windowSeconds int64, hookDetail string) error {
	_, err := r.db.Exec(`
		INSERT INTO zfs_runs (run_id, item_id, snapshot_name, window_seconds, hook_detail)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO UPDATE SET
			item_id = excluded.item_id,
			snapshot_name = excluded.snapshot_name,
			window_seconds = excluded.window_seconds,
			hook_detail = excluded.hook_detail`,
		runID, itemID, snapName, windowSeconds, cutToBytes(hookDetail, 2000),
	)
	if err != nil {
		return fmt.Errorf("RecordZFSRun: %w", err)
	}
	return nil
}

// GetZFSRun returns the ZFS detail of one run. A run that never got that far
// gives sql.ErrNoRows.
func (r *Repo) GetZFSRun(runID string) (ZFSRun, error) {
	var run ZFSRun
	err := r.db.QueryRow(`
		SELECT run_id, item_id, snapshot_name, window_seconds, hook_detail
		FROM zfs_runs WHERE run_id = ?`, runID,
	).Scan(&run.RunID, &run.ItemID, &run.SnapshotName, &run.WindowSeconds, &run.HookDetail)
	if err != nil {
		return ZFSRun{}, fmt.Errorf("GetZFSRun: %w", err)
	}
	return run, nil
}

// ListZFSRuns returns an item's recorded runs, newest first, so the restore
// points of its history can be matched to what each run did.
func (r *Repo) ListZFSRuns(itemID string, limit int) ([]ZFSRun, error) {
	rows, err := r.db.Query(`
		SELECT z.run_id, z.item_id, z.snapshot_name, z.window_seconds, z.hook_detail
		FROM zfs_runs z
		JOIN runs r ON r.id = z.run_id
		WHERE z.item_id = ?
		ORDER BY COALESCE(r.finished_at, r.started_at) DESC
		LIMIT ?`, itemID, limit)
	if err != nil {
		return nil, fmt.Errorf("ListZFSRuns: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []ZFSRun
	for rows.Next() {
		var run ZFSRun
		if err := rows.Scan(&run.RunID, &run.ItemID, &run.SnapshotName, &run.WindowSeconds, &run.HookDetail); err != nil {
			return nil, fmt.Errorf("ListZFSRuns scan: %w", err)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// AddZFSRunMember records what one run did to one dataset. It is written as
// each member finishes, so an interrupted run keeps what it managed.
func (r *Repo) AddZFSRunMember(m ZFSRunMember) error {
	_, err := r.db.Exec(`
		INSERT INTO zfs_run_members (run_id, dataset, outcome, is_new, restic_snapshot,
			bytes_added, files_new, files_changed, files_unmodified, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id, dataset) DO UPDATE SET
			outcome = excluded.outcome,
			is_new = excluded.is_new,
			restic_snapshot = excluded.restic_snapshot,
			bytes_added = excluded.bytes_added,
			files_new = excluded.files_new,
			files_changed = excluded.files_changed,
			files_unmodified = excluded.files_unmodified,
			duration_ms = excluded.duration_ms`,
		m.RunID, m.Dataset, m.Outcome, boolInt(m.IsNew), m.ResticSnapshot,
		m.BytesAdded, m.FilesNew, m.FilesChanged, m.FilesUnmodified, m.DurationMS,
	)
	if err != nil {
		return fmt.Errorf("AddZFSRunMember %s: %w", m.Dataset, err)
	}
	return nil
}

// ListZFSRunMembers returns one run's members ordered by dataset.
func (r *Repo) ListZFSRunMembers(runID string) ([]ZFSRunMember, error) {
	return r.zfsRunMembers(`
		SELECT run_id, dataset, outcome, is_new, restic_snapshot,
		       bytes_added, files_new, files_changed, files_unmodified, duration_ms
		FROM zfs_run_members WHERE run_id = ? ORDER BY dataset`, "ListZFSRunMembers", runID)
}

// ListZFSRunMembersByDataset returns one dataset's newest rows across runs,
// most recent first, for the baselines a change is measured against.
func (r *Repo) ListZFSRunMembersByDataset(dataset string, limit int) ([]ZFSRunMember, error) {
	return r.zfsRunMembers(`
		SELECT m.run_id, m.dataset, m.outcome, m.is_new, m.restic_snapshot,
		       m.bytes_added, m.files_new, m.files_changed, m.files_unmodified, m.duration_ms
		FROM zfs_run_members m
		JOIN runs r ON r.id = m.run_id
		WHERE m.dataset = ?
		ORDER BY COALESCE(r.finished_at, r.started_at) DESC
		LIMIT ?`, "ListZFSRunMembersByDataset", dataset, limit)
}

func (r *Repo) zfsRunMembers(query, label string, args ...any) ([]ZFSRunMember, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []ZFSRunMember
	for rows.Next() {
		var m ZFSRunMember
		var isNew int
		if err := rows.Scan(&m.RunID, &m.Dataset, &m.Outcome, &isNew, &m.ResticSnapshot,
			&m.BytesAdded, &m.FilesNew, &m.FilesChanged, &m.FilesUnmodified, &m.DurationMS); err != nil {
			return nil, fmt.Errorf("%s scan: %w", label, err)
		}
		m.IsNew = isNew != 0
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpsertZFSSafetySnapshot records a snapshot taken before an in-place restore.
func (r *Repo) UpsertZFSSafetySnapshot(s ZFSSafetySnapshot) error {
	_, err := r.db.Exec(`
		INSERT INTO zfs_safety_snapshots (item_id, dataset, name, created_at, used_bytes)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(dataset, name) DO UPDATE SET
			item_id = excluded.item_id,
			created_at = excluded.created_at,
			used_bytes = excluded.used_bytes`,
		s.ItemID, s.Dataset, s.Name, s.CreatedAt, s.UsedBytes,
	)
	if err != nil {
		return fmt.Errorf("UpsertZFSSafetySnapshot %s: %w", s.Name, err)
	}
	return nil
}

// ReplaceZFSSafetySnapshots rewrites an item's safety snapshots from a host
// listing, so one destroyed outside BombVault stops being offered.
func (r *Repo) ReplaceZFSSafetySnapshots(itemID string, ss []ZFSSafetySnapshot) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("ReplaceZFSSafetySnapshots begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	if _, err := tx.Exec(`DELETE FROM zfs_safety_snapshots WHERE item_id = ?`, itemID); err != nil {
		return fmt.Errorf("ReplaceZFSSafetySnapshots clear: %w", err)
	}
	for _, s := range ss {
		_, err := tx.Exec(`
			INSERT INTO zfs_safety_snapshots (item_id, dataset, name, created_at, used_bytes)
			VALUES (?, ?, ?, ?, ?)`,
			itemID, s.Dataset, s.Name, s.CreatedAt, s.UsedBytes,
		)
		if err != nil {
			return fmt.Errorf("ReplaceZFSSafetySnapshots %s: %w", s.Name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ReplaceZFSSafetySnapshots commit: %w", err)
	}
	return nil
}

// ListZFSSafetySnapshots returns an item's safety snapshots, newest first.
func (r *Repo) ListZFSSafetySnapshots(itemID string) ([]ZFSSafetySnapshot, error) {
	rows, err := r.db.Query(`
		SELECT item_id, dataset, name, created_at, used_bytes
		FROM zfs_safety_snapshots WHERE item_id = ? ORDER BY created_at DESC, dataset, name`, itemID)
	if err != nil {
		return nil, fmt.Errorf("ListZFSSafetySnapshots: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []ZFSSafetySnapshot
	for rows.Next() {
		var s ZFSSafetySnapshot
		if err := rows.Scan(&s.ItemID, &s.Dataset, &s.Name, &s.CreatedAt, &s.UsedBytes); err != nil {
			return nil, fmt.Errorf("ListZFSSafetySnapshots scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeleteZFSSafetySnapshot forgets one safety snapshot. Destroying it on the
// host is a separate step.
func (r *Repo) DeleteZFSSafetySnapshot(dataset, name string) error {
	if _, err := r.db.Exec(
		`DELETE FROM zfs_safety_snapshots WHERE dataset = ? AND name = ?`, dataset, name,
	); err != nil {
		return fmt.Errorf("DeleteZFSSafetySnapshot: %w", err)
	}
	return nil
}

func (r *Repo) updateZFSDataset(label, id, query string, args ...any) error {
	res, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%s: no ZFS item %q", label, id)
	}
	return nil
}

func knownZFSMembers(tx *sql.Tx, itemID string) (map[string]ZFSMember, error) {
	rows, err := tx.Query(
		`SELECT dataset, first_seen_at, last_backup_at FROM zfs_members WHERE item_id = ?`, itemID)
	if err != nil {
		return nil, fmt.Errorf("ReplaceZFSMembers read: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	known := map[string]ZFSMember{}
	for rows.Next() {
		var m ZFSMember
		if err := rows.Scan(&m.Dataset, &m.FirstSeenAt, &m.LastBackupAt); err != nil {
			return nil, fmt.Errorf("ReplaceZFSMembers scan: %w", err)
		}
		known[m.Dataset] = m
	}
	return known, rows.Err()
}

func scanZFSDataset(s scanner) (ZFSDataset, error) {
	var d ZFSDataset
	var excludes, children, stop, pending string
	var enabled int
	err := s.Scan(&d.ID, &d.Dataset, &enabled, &excludes, &children, &d.ScheduleCadence, &d.Repo,
		&stop, &pending, &d.HookContainer, &d.PreSnapshot, &d.PostSnapshot,
		&d.LastCheckCode, &d.LastCheckDetail, &d.LastCheckAt, &d.LastHostMountpoint,
		&d.LeftoverCount, &d.LeftoverCheckedAt, &d.CreatedAt)
	if err != nil {
		return ZFSDataset{}, fmt.Errorf("scanZFSDataset: %w", err)
	}
	for _, list := range []struct {
		column string
		raw    string
		into   *[]string
	}{
		{"excludes", excludes, &d.Excludes},
		{"excluded_children", children, &d.ExcludedChildren},
		{"stop_containers", stop, &d.StopContainers},
		{"restart_pending", pending, &d.RestartPending},
	} {
		if err := json.Unmarshal([]byte(list.raw), list.into); err != nil {
			return ZFSDataset{}, fmt.Errorf("scanZFSDataset unmarshal %s: %w", list.column, err)
		}
	}
	d.Enabled = enabled != 0
	return d, nil
}

// marshalList encodes a string list for a JSON column, writing an empty array
// rather than "null" for a nil slice.
func marshalList(list []string) (string, error) {
	if list == nil {
		list = []string{}
	}
	b, err := json.Marshal(list)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// cutToBytes shortens s to at most max bytes without splitting a character.
func cutToBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	s = s[:max]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
