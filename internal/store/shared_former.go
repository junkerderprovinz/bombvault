package store

import "fmt"

// SharedFormer is a server that left a shared repository or was taken out.
// Its snapshots stay there until the owner forgets them on purpose, and the
// owner keeps its name so a person can confirm that by typing it.
type SharedFormer struct {
	Host string
	// Instance is its group instance id. Its host stays its own, so no
	// other server can take over what it left.
	Instance string
	Name     string
	LeftAt   int64
	// ForgetError is why forgetting its snapshots failed the last time.
	ForgetError string
}

// NoteSharedFormer keeps f as a former member of the shared repository id.
func (r *Repo) NoteSharedFormer(id string, f SharedFormer) error {
	if _, err := r.db.Exec(`INSERT OR REPLACE INTO shared_repo_former (shared_repo_id, host, instance_id, name, left_at, forget_error)
		VALUES (?, ?, ?, ?, ?, ?)`, id, f.Host, f.Instance, f.Name, f.LeftAt, f.ForgetError); err != nil {
		return fmt.Errorf("NoteSharedFormer: %w", err)
	}
	return nil
}

// ListSharedFormer returns the former members of the shared repository id,
// the latest to leave first.
func (r *Repo) ListSharedFormer(id string) ([]SharedFormer, error) {
	rows, err := r.db.Query(`SELECT host, instance_id, name, left_at, forget_error FROM shared_repo_former
		WHERE shared_repo_id = ? ORDER BY left_at DESC, host`, id)
	if err != nil {
		return nil, fmt.Errorf("ListSharedFormer: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []SharedFormer
	for rows.Next() {
		var f SharedFormer
		if err := rows.Scan(&f.Host, &f.Instance, &f.Name, &f.LeftAt, &f.ForgetError); err != nil {
			return nil, fmt.Errorf("ListSharedFormer: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// DropSharedFormer forgets that the server writing under host was a member of
// the shared repository id.
func (r *Repo) DropSharedFormer(id, host string) error {
	if _, err := r.db.Exec(`DELETE FROM shared_repo_former WHERE shared_repo_id = ? AND host = ?`, id, host); err != nil {
		return fmt.Errorf("DropSharedFormer: %w", err)
	}
	return nil
}
