package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// ErrIncompleteSharedRepo is returned for a shared repository without a
// location or an owner.
var ErrIncompleteSharedRepo = errors.New("a shared repository needs a location and an owner")

// ErrSharedHostTaken is returned when two members of a shared repository have
// the same restic host. Their snapshots could not be told apart there.
var ErrSharedHostTaken = errors.New("another member of the shared repository already writes under this host")

// SharedRepo is a restic repository several servers of a group back up into.
// The server that holds it, its owner, is the only one that forgets and prunes
// there. The others only add to it.
type SharedRepo struct {
	ID string
	// Repo is the repository as this server hands it to restic: the folder on
	// the owner, the rest-server address on a member.
	Repo string
	// Owner is the group instance id of the server that holds it.
	Owner string
	// Members are the servers that back up into it, in the order they joined.
	// A snapshot under a host none of them has belongs to nobody the owner
	// knows, so it is never forgotten.
	Members   []SharedMember
	CreatedAt int64
}

// SharedMember is one server that backs up into a shared repository.
type SharedMember struct {
	// Host is the restic host its snapshots carry (restickey.Host).
	Host string
	// Instance is its group instance id.
	Instance string
	Name     string
}

// Sharing is the part the server with group instance id self has in sr.
func (sr SharedRepo) Sharing(self string) restic.Sharing {
	if sr.Owner == self {
		return restic.SharingOwner
	}
	return restic.SharingMember
}

// PutSharedRepo stores sr with exactly its members, assigning an id and a
// creation time to a new one, and returns what it stored.
func (r *Repo) PutSharedRepo(sr SharedRepo) (SharedRepo, error) {
	if strings.TrimSpace(sr.Repo) == "" || sr.Owner == "" {
		return SharedRepo{}, ErrIncompleteSharedRepo
	}
	seen := map[string]bool{}
	for _, m := range sr.Members {
		if seen[m.Host] {
			return SharedRepo{}, fmt.Errorf("%w: %s", ErrSharedHostTaken, m.Host)
		}
		seen[m.Host] = true
	}
	if sr.ID == "" {
		sr.ID = newID()
	}
	if sr.CreatedAt == 0 {
		sr.CreatedAt = time.Now().Unix()
	}
	err := r.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO shared_repos (id, repo, owner, created_at) VALUES (?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET repo = excluded.repo, owner = excluded.owner`,
			sr.ID, sr.Repo, sr.Owner, sr.CreatedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM shared_repo_members WHERE shared_repo_id = ?`, sr.ID); err != nil {
			return err
		}
		for i, m := range sr.Members {
			if _, err := tx.Exec(`INSERT INTO shared_repo_members (shared_repo_id, host, instance_id, name, position) VALUES (?, ?, ?, ?, ?)`,
				sr.ID, m.Host, m.Instance, m.Name, i); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return SharedRepo{}, fmt.Errorf("PutSharedRepo: %w", err)
	}
	return sr, nil
}

// SharedRepoAt returns the shared repository restic reaches at repo, if there
// is one.
func (r *Repo) SharedRepoAt(repo string) (SharedRepo, bool, error) {
	all, err := r.sharedRepos(`SELECT id, repo, owner, created_at FROM shared_repos WHERE repo = ?`, repo)
	if err != nil || len(all) == 0 {
		return SharedRepo{}, false, err
	}
	return all[0], true, nil
}

// ListSharedRepos returns every shared repository this server knows, oldest
// first.
func (r *Repo) ListSharedRepos() ([]SharedRepo, error) {
	return r.sharedRepos(`SELECT id, repo, owner, created_at FROM shared_repos ORDER BY created_at, id`)
}

// DeleteSharedRepo forgets the shared repository id and its members. It
// leaves the repository itself alone.
func (r *Repo) DeleteSharedRepo(id string) error {
	if _, err := r.db.Exec(`DELETE FROM shared_repos WHERE id = ?`, id); err != nil {
		return fmt.Errorf("DeleteSharedRepo: %w", err)
	}
	return nil
}

func (r *Repo) sharedRepos(query string, args ...any) ([]SharedRepo, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list shared repositories: %w", err)
	}
	var out []SharedRepo
	for rows.Next() {
		var sr SharedRepo
		if err := rows.Scan(&sr.ID, &sr.Repo, &sr.Owner, &sr.CreatedAt); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("list shared repositories: %w", err)
		}
		out = append(out, sr)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("list shared repositories: %w", err)
	}
	for i := range out {
		if out[i].Members, err = r.sharedMembers(out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Repo) sharedMembers(id string) ([]SharedMember, error) {
	rows, err := r.db.Query(`SELECT host, instance_id, name FROM shared_repo_members WHERE shared_repo_id = ? ORDER BY position`, id)
	if err != nil {
		return nil, fmt.Errorf("list shared repository members: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []SharedMember
	for rows.Next() {
		var m SharedMember
		if err := rows.Scan(&m.Host, &m.Instance, &m.Name); err != nil {
			return nil, fmt.Errorf("list shared repository members: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
