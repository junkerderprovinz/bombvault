package store

import "fmt"

// MarkVMsStopped remembers VMs a backup is about to shut down.
func (r *Repo) MarkVMsStopped(names []string, at int64) error {
	for _, n := range names {
		if _, err := r.db.Exec(`INSERT INTO vm_restart_pending (name, added_at) VALUES (?, ?)
			ON CONFLICT (name) DO NOTHING`, n, at); err != nil {
			return fmt.Errorf("MarkVMsStopped: %w", err)
		}
	}
	return nil
}

// ClearVMsStopped forgets VMs that run again.
func (r *Repo) ClearVMsStopped(names []string) error {
	for _, n := range names {
		if _, err := r.db.Exec(`DELETE FROM vm_restart_pending WHERE name = ?`, n); err != nil {
			return fmt.Errorf("ClearVMsStopped: %w", err)
		}
	}
	return nil
}

// ListVMsStopped returns the VMs a backup left shut down.
func (r *Repo) ListVMsStopped() ([]string, error) {
	return r.listStrings(`SELECT name FROM vm_restart_pending ORDER BY added_at, name`, "ListVMsStopped")
}
