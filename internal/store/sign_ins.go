package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SignInsNote is what became of the shared cloud sign-in when it turned into
// a saved one, kept until somebody has read it.
type SignInsNote struct {
	// SetID is the saved sign-in that took the shared one's place and is the
	// default. Empty when the install had no shared sign-in.
	SetID string
	// Merged says the shared sign-in held the same keys as SetID, which kept
	// its name.
	Merged bool
	// Duplicates counts the saved sign-ins that went away because another one
	// held the same keys.
	Duplicates int
	// Linked are the sections whose typed own folder turned out to lie in a
	// storage location and follows it now.
	Linked []string
}

// Changed reports whether the conversion did anything worth telling.
func (n SignInsNote) Changed() bool {
	return n.SetID != "" || n.Duplicates > 0 || len(n.Linked) > 0
}

// SignInsPending reports whether the shared sign-in still waits to be turned
// into a saved one.
func (r *Repo) SignInsPending() (bool, error) {
	var pending int
	err := r.db.QueryRow(`SELECT pending FROM sign_ins_note WHERE id = 1`).Scan(&pending)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("SignInsPending: %w", err)
	}
	return pending != 0, nil
}

// FinishSignIns records what the conversion did, and that it ran.
func (r *Repo) FinishSignIns(n SignInsNote) error {
	_, err := r.db.Exec(`UPDATE sign_ins_note SET pending = 0, set_id = ?, merged = ?, duplicates = ?, linked = ? WHERE id = 1`,
		n.SetID, boolInt(n.Merged), n.Duplicates, strings.Join(n.Linked, ","))
	if err != nil {
		return fmt.Errorf("FinishSignIns: %w", err)
	}
	return nil
}

// SignInsNoteToShow returns the note while it says something and nobody has
// dismissed it.
func (r *Repo) SignInsNoteToShow() (SignInsNote, bool, error) {
	var n SignInsNote
	var pending, dismissed, merged int
	var linked string
	err := r.db.QueryRow(`SELECT pending, dismissed, set_id, merged, duplicates, linked FROM sign_ins_note WHERE id = 1`).
		Scan(&pending, &dismissed, &n.SetID, &merged, &n.Duplicates, &linked)
	if errors.Is(err, sql.ErrNoRows) {
		return SignInsNote{}, false, nil
	}
	if err != nil {
		return SignInsNote{}, false, fmt.Errorf("SignInsNoteToShow: %w", err)
	}
	n.Merged = merged != 0
	if linked != "" {
		n.Linked = strings.Split(linked, ",")
	}
	return n, pending == 0 && dismissed == 0 && n.Changed(), nil
}

// DismissSignInsNote stops showing the note.
func (r *Repo) DismissSignInsNote() error {
	if _, err := r.db.Exec(`UPDATE sign_ins_note SET dismissed = 1 WHERE id = 1`); err != nil {
		return fmt.Errorf("DismissSignInsNote: %w", err)
	}
	return nil
}

// ReplaceCredsRef points every row that signs in with the saved sign-in from
// at the one called to.
func (r *Repo) ReplaceCredsRef(from, to string) error {
	return r.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE offsite_targets SET creds_ref = ? WHERE creds_ref = ?`, to, from); err != nil {
			return fmt.Errorf("ReplaceCredsRef: %w", err)
		}
		if _, err := tx.Exec(`UPDATE pull_sources SET creds_ref = ? WHERE creds_ref = ?`, to, from); err != nil {
			return fmt.Errorf("ReplaceCredsRef: %w", err)
		}
		return nil
	})
}

// OwnFolder is where a section's own backups go after a move.
type OwnFolder struct {
	Domain string
	// Row is the remote-primary row the folder keeps, nil for a folder that
	// needs none, such as one on this host.
	Row *OffsiteTarget
	// EndCopy is the off-site target that lies where the own folder goes. It
	// becomes the own folder, so copying there ends.
	EndCopy string
	// SwitchOn switches the own folder back on for new backups. A section
	// whose own folder is off backs up straight to its copies, and the copy
	// that ends would otherwise stop taking them.
	SwitchOn bool
}

// MoveOwnFolder points a section's own folder somewhere else in one
// transaction: settle writes the path into the settings, the section's
// remote-primary row is replaced by f.Row, and the copy at the new place is
// removed. A run that reads the path in between sees either the old place
// with its sign-in or the new one with its own. An error from settle undoes
// it all, and so does a row linked to a storage location that is gone.
func (r *Repo) MoveOwnFolder(f OwnFolder, settle func(*Settings) error) error {
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	err := r.inTx(func(tx *sql.Tx) error {
		s, err := getSettings(tx)
		if err != nil {
			return err
		}
		if err := settle(&s); err != nil {
			return err
		}
		if err := updateSettings(tx, s); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM offsite_targets WHERE domain = ? AND role = ?`, f.Domain, RolePrimary); err != nil {
			return err
		}
		if f.Row != nil {
			if f.Row.DestinationID != "" {
				if _, err := destinationTx(tx, f.Row.DestinationID); err != nil {
					return err
				}
			}
			if err := insertPrimaryTx(tx, f.Domain, *f.Row); err != nil {
				return err
			}
		}
		if f.EndCopy != "" {
			if err := deleteTargetKeepingDirectTx(tx, f.EndCopy); err != nil {
				return err
			}
		}
		if f.SwitchOn {
			if _, err := tx.Exec(`DELETE FROM places_off WHERE domain = ? AND place = ?`, f.Domain, placeLocal); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("MoveOwnFolder: %w", err)
	}
	return nil
}

func insertPrimaryTx(tx *sql.Tx, domain string, t OffsiteTarget) error {
	if t.ID == "" {
		t.ID = newID()
	}
	if t.CreatedAt == 0 {
		t.CreatedAt = time.Now().Unix()
	}
	if t.Name == "" {
		t.Name = "Primary (remote)"
	}
	_, err := tx.Exec(`INSERT INTO offsite_targets (id, domain, name, repo, role, creds_ref, storage_class, immutable,
		  limit_upload, limit_download, growth_budget_gb, enabled, created_at, destination_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		t.ID, domain, t.Name, t.Repo, RolePrimary, t.CredsRef, t.StorageClass, boolInt(t.Immutable),
		t.LimitUpload, t.LimitDownload, t.GrowthBudgetGB, t.CreatedAt, t.DestinationID)
	return err
}
