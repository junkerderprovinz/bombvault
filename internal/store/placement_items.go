package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RepoChoice is repo_chosen: whether an item's location is settled.
type RepoChoice int

const (
	RepoOpen         RepoChoice = 0 // takes the default's home at its first backup
	RepoChosen       RepoChoice = 1
	RepoChosenUnread RepoChoice = 2 // Discover left repo empty because the domain path could not be read
)

var ErrRepoChoice = errors.New("repo and repo_chosen disagree")

// checkRepoChoice keeps the rule every writer of repo follows: a repository is
// only ever stored as a choice.
func checkRepoChoice(repo string, c RepoChoice) error {
	switch {
	case c == RepoChosen:
		return nil
	case (c == RepoOpen || c == RepoChosenUnread) && repo == "":
		return nil
	}
	return ErrRepoChoice
}

// HomeState is an item's location columns as read, the compare half of a guarded write.
type HomeState struct {
	Exists bool
	Repo   string
	Choice RepoChoice
}

// HomeWrite is the location WritePlacement stores.
type HomeWrite struct {
	Repo   string
	Choice RepoChoice
}

// CopiesWrite is the copy rule WritePlacement stores under the item's name.
type CopiesWrite struct {
	Follow bool // delete the item's rule so it follows the default
	Skip   []string
}

// itemHomeSQL holds the location statements of each item table. Each read
// returns the name the item's snapshots are tagged with.
var itemHomeSQL = map[string]struct {
	read, update, insert, tagPrefix string
}{
	"containers": {
		read:      `SELECT container_name, repo, repo_chosen FROM targets WHERE container_name = ?`,
		update:    `UPDATE targets SET repo = ?, repo_chosen = ? WHERE container_name = ?`,
		insert:    `INSERT INTO targets (id, container_name, appdata_paths, created_at, repo, repo_chosen) VALUES (?, ?, '[]', ?, ?, ?)`,
		tagPrefix: "container:",
	},
	"vms": {
		read:      `SELECT name, repo, repo_chosen FROM vms WHERE name = ?`,
		update:    `UPDATE vms SET repo = ?, repo_chosen = ? WHERE name = ?`,
		insert:    `INSERT INTO vms (id, name, created_at, repo, repo_chosen) VALUES (?, ?, ?, ?, ?)`,
		tagPrefix: "vm:",
	},
	"files": {
		read:      `SELECT name, repo, repo_chosen FROM file_sets WHERE id = ?`,
		update:    `UPDATE file_sets SET repo = ?, repo_chosen = ? WHERE id = ?`,
		tagPrefix: "fileset:",
	},
}

type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

func readItemHome(q rowQuerier, item ItemRef) (HomeState, string, error) {
	stmts, ok := itemHomeSQL[item.Domain]
	if !ok {
		return HomeState{}, "", fmt.Errorf("unknown item domain %q", item.Domain)
	}
	var h HomeState
	var name string
	err := q.QueryRow(stmts.read, item.Key).Scan(&name, &h.Repo, &h.Choice)
	if errors.Is(err, sql.ErrNoRows) {
		return HomeState{}, "", nil
	}
	if err != nil {
		return HomeState{}, "", err
	}
	h.Exists = true
	return h, name, nil
}

// ItemHome reads an item's location columns. A missing row is open, not an error.
func (r *Repo) ItemHome(item ItemRef) (HomeState, error) {
	h, _, err := readItemHome(r.db, item)
	if err != nil {
		return HomeState{}, fmt.Errorf("ItemHome: %w", err)
	}
	return h, nil
}

// WritePlacement writes an item's location and copy rule in one transaction.
// With expect it writes nothing and reports false when the row reads differently.
func (r *Repo) WritePlacement(item ItemRef, home *HomeWrite, copies *CopiesWrite, expect *HomeState) (bool, error) {
	if home == nil && copies == nil {
		return false, errors.New("WritePlacement: nothing to write")
	}
	if home != nil {
		if err := checkRepoChoice(home.Repo, home.Choice); err != nil {
			return false, fmt.Errorf("WritePlacement: %w", err)
		}
	}
	tx, err := r.db.Begin()
	if err != nil {
		return false, fmt.Errorf("WritePlacement: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	cur, name, err := readItemHome(tx, item)
	if err != nil {
		return false, fmt.Errorf("WritePlacement read: %w", err)
	}
	if expect != nil && cur != *expect {
		return false, nil
	}
	stmts := itemHomeSQL[item.Domain]
	if !cur.Exists && stmts.insert == "" {
		return false, fmt.Errorf("WritePlacement: no file set %q", item.Key)
	}
	if home != nil {
		if cur.Exists {
			_, err = tx.Exec(stmts.update, home.Repo, home.Choice, item.Key)
		} else {
			_, err = tx.Exec(stmts.insert, newID(), item.Key, time.Now().Unix(), home.Repo, home.Choice)
		}
		if err != nil {
			return false, fmt.Errorf("WritePlacement home: %w", err)
		}
	}
	if copies != nil {
		if !cur.Exists {
			name = item.Key
		}
		identity := stmts.tagPrefix + name
		if copies.Follow {
			err = deleteCopyRuleTx(tx, item.Domain, identity)
		} else {
			err = setCopyRuleTx(tx, item.Domain, identity, copies.Skip, time.Now().Unix())
		}
		if err != nil {
			return false, fmt.Errorf("WritePlacement rule: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("WritePlacement commit: %w", err)
	}
	return true, nil
}
