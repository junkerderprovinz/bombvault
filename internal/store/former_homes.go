package store

import (
	"fmt"
	"time"
)

// itemRowIDQ finds the row id an item's runs are recorded under. A file set
// is keyed by its id already.
var itemRowIDQ = map[string]string{
	"containers": `SELECT id FROM targets WHERE container_name = ?`,
	"vms":        `SELECT id FROM vms WHERE name = ?`,
	"files":      `SELECT id FROM file_sets WHERE id = ?`,
}

// FollowSection opens an item again, so its next backup goes where its
// section sends new backups, and keeps the place it held among its former
// homes, so the backups it has there stay listed. It writes nothing and
// reports false when the row reads differently from expect.
func (r *Repo) FollowSection(item ItemRef, expect HomeState) (bool, error) {
	stmts, ok := itemHomeSQL[item.Domain]
	if !ok {
		return false, fmt.Errorf("FollowSection: unknown item domain %q", item.Domain)
	}
	tx, err := r.db.Begin()
	if err != nil {
		return false, fmt.Errorf("FollowSection: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	cur, _, err := readItemHome(tx, item)
	if err != nil {
		return false, fmt.Errorf("FollowSection read: %w", err)
	}
	if !cur.Exists || cur != expect {
		return false, nil
	}
	var id string
	if err := tx.QueryRow(itemRowIDQ[item.Domain], item.Key).Scan(&id); err != nil {
		return false, fmt.Errorf("FollowSection id: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO former_homes (domain, item_id, repo, left_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (domain, item_id, repo) DO UPDATE SET left_at = excluded.left_at`,
		item.Domain, id, cur.Repo, time.Now().Unix()); err != nil {
		return false, fmt.Errorf("FollowSection former home: %w", err)
	}
	if _, err := tx.Exec(stmts.update, "", RepoOpen, item.Key); err != nil {
		return false, fmt.Errorf("FollowSection home: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("FollowSection commit: %w", err)
	}
	return true, nil
}

// FormerHomeRepos returns every named repository id some item has as a former
// home, sorted.
func (r *Repo) FormerHomeRepos() ([]string, error) {
	rows, err := r.db.Query(`SELECT DISTINCT repo FROM former_homes WHERE repo <> '' ORDER BY repo`)
	if err != nil {
		return nil, fmt.Errorf("FormerHomeRepos: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []string{}
	for rows.Next() {
		var repo string
		if err := rows.Scan(&repo); err != nil {
			return nil, fmt.Errorf("FormerHomeRepos: %w", err)
		}
		out = append(out, repo)
	}
	return out, rows.Err()
}

// FormerHomes returns the places an item's backups were written to before it
// followed its section elsewhere: named repository ids, and "" for the
// section's own folder. They are ordered by when the item left them, the most
// recent first.
func (r *Repo) FormerHomes(domain, itemID string) ([]string, error) {
	if _, ok := itemRowIDQ[domain]; !ok {
		return nil, fmt.Errorf("FormerHomes: %q has no placement", domain)
	}
	rows, err := r.db.Query(`SELECT repo FROM former_homes WHERE domain = ? AND item_id = ? ORDER BY left_at DESC, repo`, domain, itemID)
	if err != nil {
		return nil, fmt.Errorf("FormerHomes: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []string{}
	for rows.Next() {
		var repo string
		if err := rows.Scan(&repo); err != nil {
			return nil, fmt.Errorf("FormerHomes: %w", err)
		}
		out = append(out, repo)
	}
	return out, rows.Err()
}
