package store

import "fmt"

// BackupEntry names an entry of the backup list: its kind and the key the routes
// of that kind take.
type BackupEntry struct{ Kind, Key string }

// DeclineBackup records that the person does not want ref backed up.
func (r *Repo) DeclineBackup(ref BackupEntry) error {
	if _, err := r.db.Exec(`INSERT OR IGNORE INTO declined_backups (kind, key) VALUES (?, ?)`, ref.Kind, ref.Key); err != nil {
		return fmt.Errorf("DeclineBackup: %w", err)
	}
	return nil
}

// AcceptBackup takes back what DeclineBackup recorded for ref.
func (r *Repo) AcceptBackup(ref BackupEntry) error {
	if _, err := r.db.Exec(`DELETE FROM declined_backups WHERE kind = ? AND key = ?`, ref.Kind, ref.Key); err != nil {
		return fmt.Errorf("AcceptBackup: %w", err)
	}
	return nil
}

// DeclinedBackups returns every entry the person does not want backed up.
func (r *Repo) DeclinedBackups() (map[BackupEntry]bool, error) {
	rows, err := r.db.Query(`SELECT kind, key FROM declined_backups`)
	if err != nil {
		return nil, fmt.Errorf("DeclinedBackups: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := map[BackupEntry]bool{}
	for rows.Next() {
		var ref BackupEntry
		if err := rows.Scan(&ref.Kind, &ref.Key); err != nil {
			return nil, fmt.Errorf("DeclinedBackups: %w", err)
		}
		out[ref] = true
	}
	return out, rows.Err()
}

// acceptBackupOfRun takes the entry of a backup or import run off the declined
// list: somebody who backs an entry up wants it backed up. A container and a VM
// are declined by name, everything else by the id its runs carry.
const acceptBackupOfRun = `
	DELETE FROM declined_backups
	WHERE EXISTS (
		SELECT 1 FROM runs
		WHERE runs.id = ?1 AND runs.kind IN ('backup', 'import') AND (
			(declined_backups.kind IN ('files', 'zfs', 'flash', 'config') AND declined_backups.key = runs.target_id)
			OR (declined_backups.kind = 'container'
				AND declined_backups.key = (SELECT container_name FROM targets WHERE id = runs.target_id))
			OR (declined_backups.kind = 'vm'
				AND declined_backups.key = (SELECT name FROM vms WHERE id = runs.target_id))))`
