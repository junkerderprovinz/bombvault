package store

import (
	"cmp"
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
func keepLeftOwnFolders(tx *sql.Tx, before, after Settings) error {
	now := sectionPathsOf(after)
	fields := offsiteFieldsOf(after)
	for domain, old := range sectionPathsOf(before) {
		next := now[domain]
		if sameSectionPath(old, next) {
			continue
		}
		if back := strings.TrimRight(strings.TrimSpace(next), "/"); back != "" {
			if _, err := tx.Exec(`DELETE FROM former_own_folders WHERE domain = ? AND rtrim(repo, '/') = ?`, domain, back); err != nil {
				return err
			}
			if strings.TrimSpace(fields[domain]) == "" {
				if err := dropLeftoverCopyTx(tx, domain, back); err != nil {
					return err
				}
			}
		}
		if strings.TrimSpace(old) == "" {
			continue
		}
		var credsRef, destID string
		err := tx.QueryRow(`SELECT creds_ref, destination_id FROM offsite_targets WHERE domain = ? AND role = ?`, domain, RolePrimary).Scan(&credsRef, &destID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO former_own_folders (domain, repo, creds_ref, destination_id, left_at) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (domain, repo) DO UPDATE SET creds_ref = excluded.creds_ref, destination_id = excluded.destination_id, left_at = excluded.left_at`,
			domain, strings.TrimSpace(old), credsRef, destID, time.Now().Unix()); err != nil {
			return err
		}
	}
	return nil
}

// offsiteFieldsOf lists each section's off-site field in s.
func offsiteFieldsOf(s Settings) map[string]string {
	return map[string]string{
		"containers": s.ContainersOffsite, "vms": s.VMsOffsite, "flash": s.FlashOffsite,
		"config": s.ConfigOffsite, "files": s.FilesOffsite, "zfs": s.ZFSOffsite,
	}
}

// dropLeftoverCopyTx deletes the row a section's off-site field left switched
// off at path, which the section's own folder takes. No page lists that row,
// and a copy on the own folder would land on itself; the backups at the place
// stay, as the own folder's. The row may hold the only sign-in for the place,
// so the section's remote-primary row takes it unless it has one of its own.
func dropLeftoverCopyTx(tx *sql.Tx, domain, path string) error {
	rows, err := tx.Query(`SELECT id, creds_ref FROM offsite_targets WHERE role = ? AND domain = ? AND destination_id = ''
		AND enabled = 0 AND sort_order = 0 AND rtrim(repo, '/') = ?`, RoleOffsite, domain, path)
	if err != nil {
		return err
	}
	var ids []string
	var creds string
	for rows.Next() {
		var id, ref string
		if err := rows.Scan(&id, &ref); err != nil {
			rows.Close() //nolint:errcheck,gosec // the scan error is the one to report
			return err
		}
		ids = append(ids, id)
		creds = cmp.Or(creds, ref)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := dropUnusedDirectTx(tx, id); err != nil {
			return err
		}
		if err := deleteTargetKeepingDirectTx(tx, id); err != nil {
			return err
		}
	}
	if creds == "" {
		return nil
	}
	var primary, own string
	err = tx.QueryRow(`SELECT id, creds_ref FROM offsite_targets WHERE domain = ? AND role = ? ORDER BY created_at LIMIT 1`,
		domain, RolePrimary).Scan(&primary, &own)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return insertPrimaryTx(tx, domain, OffsiteTarget{Repo: path, CredsRef: creds})
	case err != nil || own != "":
		return err
	}
	_, err = tx.Exec(`UPDATE offsite_targets SET creds_ref = ? WHERE id = ?`, creds, primary)
	return err
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
// written somewhere else from at on: its section's own folder moved, or the
// item left a home. Both are counted in seconds, so a move in the second at
// counts too. itemID is the row id its runs are recorded under.
func (r *Repo) PlaceChangedSince(domain, itemID string, at int64) (bool, error) {
	var changed bool
	err := r.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM former_own_folders WHERE domain = ? AND left_at >= ?)
		OR EXISTS (SELECT 1 FROM former_homes WHERE domain = ? AND item_id = ? AND left_at >= ?)`,
		domain, at, domain, itemID, at).Scan(&changed)
	if err != nil {
		return false, fmt.Errorf("PlaceChangedSince: %w", err)
	}
	return changed, nil
}
