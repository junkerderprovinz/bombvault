package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// SharedHolderNotice is a server that has still to hear that this server
// holds the shared repository RepoID now.
type SharedHolderNotice struct {
	RepoID   string
	Instance string
}

// SharedTakenOver says that the server By announced it holds the shared
// repository RepoID, which this server held until then.
type SharedTakenOver struct {
	RepoID string
	By     string
	Name   string
	At     int64
}

// QueueSharedHolderNotices notes that the servers instances have still to
// hear that this server holds the shared repository id.
func (r *Repo) QueueSharedHolderNotices(id string, instances []string) error {
	err := r.inTx(func(tx *sql.Tx) error {
		for _, in := range instances {
			if _, err := tx.Exec(`INSERT OR IGNORE INTO shared_holder_notices (shared_repo_id, instance_id) VALUES (?, ?)`, id, in); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("QueueSharedHolderNotices: %w", err)
	}
	return nil
}

// SharedHolderNotices lists the servers that have still to hear of this
// server as the holder of a shared repository.
func (r *Repo) SharedHolderNotices() ([]SharedHolderNotice, error) {
	rows, err := r.db.Query(`SELECT shared_repo_id, instance_id FROM shared_holder_notices ORDER BY shared_repo_id, instance_id`)
	if err != nil {
		return nil, fmt.Errorf("SharedHolderNotices: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []SharedHolderNotice
	for rows.Next() {
		var n SharedHolderNotice
		if err := rows.Scan(&n.RepoID, &n.Instance); err != nil {
			return nil, fmt.Errorf("SharedHolderNotices: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// DropSharedHolderNotice forgets that instance has still to hear of this
// server as the holder of the shared repository id.
func (r *Repo) DropSharedHolderNotice(id, instance string) error {
	if _, err := r.db.Exec(`DELETE FROM shared_holder_notices WHERE shared_repo_id = ? AND instance_id = ?`, id, instance); err != nil {
		return fmt.Errorf("DropSharedHolderNotice: %w", err)
	}
	return nil
}

// NoteSharedTakenOver remembers who took a shared repository over from this
// server.
func (r *Repo) NoteSharedTakenOver(t SharedTakenOver) error {
	if _, err := r.db.Exec(`INSERT INTO shared_repo_taken_over (shared_repo_id, by_instance, by_name, at) VALUES (?, ?, ?, ?)
		ON CONFLICT(shared_repo_id) DO UPDATE SET by_instance = excluded.by_instance, by_name = excluded.by_name, at = excluded.at`,
		t.RepoID, t.By, t.Name, t.At); err != nil {
		return fmt.Errorf("NoteSharedTakenOver: %w", err)
	}
	return nil
}

// SharedTakenOver returns who took the shared repository id over from this
// server, and false when nobody did.
func (r *Repo) SharedTakenOver(id string) (SharedTakenOver, bool, error) {
	t := SharedTakenOver{RepoID: id}
	err := r.db.QueryRow(`SELECT by_instance, by_name, at FROM shared_repo_taken_over WHERE shared_repo_id = ?`, id).Scan(&t.By, &t.Name, &t.At)
	if errors.Is(err, sql.ErrNoRows) {
		return SharedTakenOver{}, false, nil
	}
	if err != nil {
		return SharedTakenOver{}, false, fmt.Errorf("SharedTakenOver: %w", err)
	}
	return t, true, nil
}

// ClearSharedTakenOver forgets who took the shared repository id over.
func (r *Repo) ClearSharedTakenOver(id string) error {
	if _, err := r.db.Exec(`DELETE FROM shared_repo_taken_over WHERE shared_repo_id = ?`, id); err != nil {
		return fmt.Errorf("ClearSharedTakenOver: %w", err)
	}
	return nil
}
