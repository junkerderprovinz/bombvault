package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// VMBlockBackup is a VM's changed-block backup switch and how its last
// backup read the disks: Mode is "changed", "full" or "classic" and Reason
// says why a run did not read changed blocks only.
type VMBlockBackup struct {
	TargetID   string
	Enabled    bool
	LastMode   string
	LastReason string
	LastAt     int64
}

// GetVMBlockBackup returns the row for targetID, a disabled one when there is
// none.
func (r *Repo) GetVMBlockBackup(targetID string) (VMBlockBackup, error) {
	b := VMBlockBackup{TargetID: targetID}
	var enabled int
	err := r.db.QueryRow(`SELECT enabled, last_mode, last_reason, last_at FROM vm_block_backup WHERE target_id = ?`, targetID).
		Scan(&enabled, &b.LastMode, &b.LastReason, &b.LastAt)
	if errors.Is(err, sql.ErrNoRows) {
		return b, nil
	}
	if err != nil {
		return VMBlockBackup{}, fmt.Errorf("GetVMBlockBackup: %w", err)
	}
	b.Enabled = enabled != 0
	return b, nil
}

// ListVMBlockBackups returns every row by target id.
func (r *Repo) ListVMBlockBackups() (map[string]VMBlockBackup, error) {
	rows, err := r.db.Query(`SELECT target_id, enabled, last_mode, last_reason, last_at FROM vm_block_backup`)
	if err != nil {
		return nil, fmt.Errorf("ListVMBlockBackups: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only
	out := map[string]VMBlockBackup{}
	for rows.Next() {
		var b VMBlockBackup
		var enabled int
		if err := rows.Scan(&b.TargetID, &enabled, &b.LastMode, &b.LastReason, &b.LastAt); err != nil {
			return nil, fmt.Errorf("ListVMBlockBackups: %w", err)
		}
		b.Enabled = enabled != 0
		out[b.TargetID] = b
	}
	return out, rows.Err()
}

// SetVMBlockBackupEnabled turns changed-block backups on or off for a VM.
func (r *Repo) SetVMBlockBackupEnabled(targetID string, enabled bool) error {
	_, err := r.db.Exec(`INSERT INTO vm_block_backup (target_id, enabled) VALUES (?, ?)
		ON CONFLICT(target_id) DO UPDATE SET enabled = excluded.enabled`, targetID, boolInt(enabled))
	if err != nil {
		return fmt.Errorf("SetVMBlockBackupEnabled: %w", err)
	}
	return nil
}

// RecordVMBlockBackupRun stores how the VM's last backup read its disks.
func (r *Repo) RecordVMBlockBackupRun(targetID, mode, reason string, at int64) error {
	_, err := r.db.Exec(`INSERT INTO vm_block_backup (target_id, last_mode, last_reason, last_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(target_id) DO UPDATE SET last_mode = excluded.last_mode, last_reason = excluded.last_reason, last_at = excluded.last_at`,
		targetID, mode, reason, at)
	if err != nil {
		return fmt.Errorf("RecordVMBlockBackupRun: %w", err)
	}
	return nil
}
