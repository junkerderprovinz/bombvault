package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// FormerOwnFolder is a place a section's own folder lay before it moved. The
// backups made there stay there, read with the sign-in the folder had: its
// own, or that of the storage location it lay in.
type FormerOwnFolder struct {
	Repo          string
	CredsRef      string
	DestinationID string
	LeftAt        int64
}

// sectionPathsOf lists each section's own folder in s.
func sectionPathsOf(s Settings) map[string]string {
	return map[string]string{
		"containers": s.ContainersPath, "vms": s.VMsPath, "flash": s.FlashPath,
		"config": s.ConfigPath, "files": s.FilesPath, "zfs": s.ZFSPath,
	}
}

func sameSectionPath(a, b string) bool {
	return strings.TrimRight(strings.TrimSpace(a), "/") == strings.TrimRight(strings.TrimSpace(b), "/")
}

// keepLeftOwnFolders remembers the own folder of every section whose path the
// write from before to after changes, whichever page or import changes it, and
// forgets a former one a section comes back to. It runs before the section's
// remote-primary row is replaced, so a remote folder keeps the sign-in it had.
func keepLeftOwnFolders(e settingsWriter, before, after Settings) error {
	now := sectionPathsOf(after)
	for domain, old := range sectionPathsOf(before) {
		next := now[domain]
		if sameSectionPath(old, next) {
			continue
		}
		if back := strings.TrimRight(strings.TrimSpace(next), "/"); back != "" {
			if _, err := e.Exec(`DELETE FROM former_own_folders WHERE domain = ? AND rtrim(repo, '/') = ?`, domain, back); err != nil {
				return err
			}
		}
		if strings.TrimSpace(old) == "" {
			continue
		}
		var credsRef, destID string
		err := e.QueryRow(`SELECT creds_ref, destination_id FROM offsite_targets WHERE domain = ? AND role = ?`, domain, RolePrimary).Scan(&credsRef, &destID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := e.Exec(`INSERT INTO former_own_folders (domain, repo, creds_ref, destination_id, left_at) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (domain, repo) DO UPDATE SET creds_ref = excluded.creds_ref, destination_id = excluded.destination_id, left_at = excluded.left_at`,
			domain, strings.TrimSpace(old), credsRef, destID, time.Now().Unix()); err != nil {
			return err
		}
	}
	return nil
}

// FormerOwnFolders returns the places a section's own folder lay before, the
// one it left last first.
func (r *Repo) FormerOwnFolders(domain string) ([]FormerOwnFolder, error) {
	rows, err := r.db.Query(`SELECT repo, creds_ref, destination_id, left_at FROM former_own_folders
		WHERE domain = ? ORDER BY left_at DESC, repo`, domain)
	if err != nil {
		return nil, fmt.Errorf("FormerOwnFolders: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []FormerOwnFolder{}
	for rows.Next() {
		var f FormerOwnFolder
		if err := rows.Scan(&f.Repo, &f.CredsRef, &f.DestinationID, &f.LeftAt); err != nil {
			return nil, fmt.Errorf("FormerOwnFolders: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// PlaceChangedSince reports whether the backups of an item may have been
// written somewhere else after at: its section's own folder moved, or the
// item left a home. itemID is the row id its runs are recorded under.
func (r *Repo) PlaceChangedSince(domain, itemID string, at int64) (bool, error) {
	var changed bool
	err := r.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM former_own_folders WHERE domain = ? AND left_at > ?)
		OR EXISTS (SELECT 1 FROM former_homes WHERE domain = ? AND item_id = ? AND left_at > ?)`,
		domain, at, domain, itemID, at).Scan(&changed)
	if err != nil {
		return false, fmt.Errorf("PlaceChangedSince: %w", err)
	}
	return changed, nil
}
