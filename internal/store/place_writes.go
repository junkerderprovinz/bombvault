package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

// ErrPlaceDomainUnavailable refuses a row of a domain the place has no folder for.
var ErrPlaceDomainUnavailable = errors.New("this place has no folder for that domain")

// PlaceAddress is where a row of the domain with the address ending lies at
// the place. A row without a domain is the place's own repository at its
// base; false means the place has no folder for the domain.
func PlaceAddress(p Place, domain, suffix string) (string, bool) {
	return places.Address(p.Base, p.Folders, domain, suffix)
}

// placeAddressTx reads the place inside tx and builds the address of the
// domain and ending there.
func placeAddressTx(tx *sql.Tx, placeID, domain, suffix string) (string, error) {
	p, found, err := placeQ(tx, placeID)
	if err != nil {
		return "", fmt.Errorf("read place %s: %w", placeID, err)
	}
	if !found {
		return "", fmt.Errorf("place %s: %w", placeID, ErrPlaceNotFound)
	}
	addr, ok := PlaceAddress(p, domain, suffix)
	if !ok {
		return "", ErrPlaceDomainUnavailable
	}
	return addr, nil
}

// CreatePlaceRepo writes a named repository at the place's address for the
// domain, carrying the place's credentials, retention, limits and switches.
func (r *Repo) CreatePlaceRepo(placeID, domain, suffix string) (OffsiteTarget, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("CreatePlaceRepo: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	addr, err := placeAddressTx(tx, placeID, domain, suffix)
	if err != nil {
		return OffsiteTarget{}, err
	}
	id := newID()
	if _, err := tx.Exec(`
		INSERT INTO offsite_targets (id, domain, name, repo, role, creds_ref, storage_class, immutable, schedule,
		  retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
		  limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order, off_premises,
		  place_id, place_domain, place_suffix)
		SELECT ?, '', p.name, ?, ?, p.creds_ref, p.storage_class, p.immutable, '',
		  p.retention_keep_last, p.retention_keep_daily, p.retention_keep_weekly, p.retention_keep_monthly,
		  p.limit_upload, p.limit_download, p.growth_budget_gb, p.enabled, ?,
		  (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM offsite_targets WHERE role = ?), p.off_premises,
		  p.id, ?, ?
		FROM storage_places p WHERE p.id = ?`,
		id, addr, RoleRepo, time.Now().Unix(), RoleRepo, domain, suffix, placeID); err != nil {
		return OffsiteTarget{}, fmt.Errorf("CreatePlaceRepo: %w", err)
	}
	return commitStoredTargetTx(tx, id)
}

// AdoptRow moves a row onto the place's address for the domain and ending and
// puts it at the place, in one transaction. A domain's primary row follows its
// home place and is never adopted.
func (r *Repo) AdoptRow(rowID, placeID, domain, suffix string) (OffsiteTarget, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("AdoptRow: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	addr, err := placeAddressTx(tx, placeID, domain, suffix)
	if err != nil {
		return OffsiteTarget{}, err
	}
	res, err := tx.Exec(`UPDATE offsite_targets SET repo = ? WHERE id = ? AND role <> ?`, addr, rowID, RolePrimary)
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("AdoptRow: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("AdoptRow: %w", err)
	}
	if n == 0 {
		return OffsiteTarget{}, fmt.Errorf("AdoptRow %s: %w", rowID, sql.ErrNoRows)
	}
	if err := AttachRowTx(tx, rowID, placeID, domain, suffix); err != nil {
		return OffsiteTarget{}, err
	}
	return commitStoredTargetTx(tx, rowID)
}

// DomainCopiesWrite is one chip of a domain row: the target it creates or
// switches, and the default's skip, written together.
type DomainCopiesWrite struct {
	Domain   string
	PlaceID  string
	TargetID string    // the target to switch; "" creates one at the place
	Suffix   string    // the new target's address ending
	Enabled  *bool     // the target's switch: the chip of flash and config, or a stopped target a chip starts
	Skip     *[]string // the default's skip, for containers, VMs and folder sets
}

// WriteDomainCopies writes a chip in one transaction and returns its target.
// A target on sort_order 0 is written back into the domain's off-site field
// as well, which the dashboard reads.
func (r *Repo) WriteDomainCopies(w DomainCopiesWrite) (OffsiteTarget, error) {
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	tx, err := r.db.Begin()
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("WriteDomainCopies: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	id := w.TargetID
	if id == "" {
		if id, err = createPlaceTargetTx(tx, w.PlaceID, w.Domain, w.Suffix); err != nil {
			return OffsiteTarget{}, err
		}
	}
	if w.Enabled != nil {
		if _, err := tx.Exec(`UPDATE offsite_targets SET enabled = ? WHERE id = ? AND role = ? AND domain = ?`,
			boolInt(*w.Enabled), id, RoleOffsite, w.Domain); err != nil {
			return OffsiteTarget{}, fmt.Errorf("WriteDomainCopies: %w", err)
		}
	}
	if w.Skip != nil {
		if err := putDefaultSkipTx(tx, w.Domain, *w.Skip); err != nil {
			return OffsiteTarget{}, fmt.Errorf("WriteDomainCopies: %w", err)
		}
	}
	t, err := offsiteTargetTx(tx, id)
	if err != nil {
		return OffsiteTarget{}, err
	}
	if t.SortOrder == 0 {
		if err := mirrorFieldTx(tx, t); err != nil {
			return OffsiteTarget{}, fmt.Errorf("WriteDomainCopies: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return OffsiteTarget{}, fmt.Errorf("WriteDomainCopies commit: %w", err)
	}
	return t, nil
}

// createPlaceTargetTx writes a target of the domain at the place's address,
// carrying the place's settings. The domain's first target takes the field
// slot, sort_order 0; any other queues behind the last.
func createPlaceTargetTx(tx *sql.Tx, placeID, domain, suffix string) (string, error) {
	addr, err := placeAddressTx(tx, placeID, domain, suffix)
	if err != nil {
		return "", err
	}
	id := newID()
	_, err = tx.Exec(`
		INSERT INTO offsite_targets (id, domain, name, repo, role, creds_ref, storage_class, immutable, schedule,
		  retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
		  limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order,
		  place_id, place_domain, place_suffix)
		SELECT ?, ?, p.name, ?, ?, p.creds_ref, p.storage_class, p.immutable, '',
		  p.retention_keep_last, p.retention_keep_daily, p.retention_keep_weekly, p.retention_keep_monthly,
		  p.limit_upload, p.limit_download, p.growth_budget_gb, p.enabled, ?,
		  (SELECT CASE WHEN COUNT(*) = 0 THEN 0 ELSE MAX(sort_order) + 1 END FROM offsite_targets WHERE role = ? AND domain = ?),
		  p.id, ?, ?
		FROM storage_places p WHERE p.id = ?`,
		id, domain, addr, RoleOffsite, time.Now().Unix(), RoleOffsite, domain, domain, suffix, placeID)
	if err != nil {
		return "", fmt.Errorf("create a target at place %s: %w", placeID, err)
	}
	return id, nil
}

// putDefaultSkipTx writes the default's skip and keeps its home and pause. A
// domain without a default gets one on its own path, as PutPlacementDefault
// makes it.
func putDefaultSkipTx(tx *sql.Tx, domain string, skip []string) error {
	if err := checkPlacementDomain(domain); err != nil {
		return err
	}
	raw, err := encodeSkip(skip)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	_, err = tx.Exec(`INSERT INTO placement_defaults (domain, home, skip, confirmed_at, updated_at) VALUES (?, '', ?, ?, ?)
		ON CONFLICT(domain) DO UPDATE SET skip = excluded.skip, updated_at = excluded.updated_at`, domain, raw, now, now)
	return err
}

// mirrorFieldTx writes a domain's sort_order-0 target into its off-site field:
// the address while the target is on, empty while it is off, as WritePlace
// fills the field from that row.
func mirrorFieldTx(tx *sql.Tx, t OffsiteTarget) error {
	before, err := getSettings(tx)
	if err != nil {
		return err
	}
	after := before
	_, offsite, immutable := domainColumns(&after, t.Domain)
	*offsite, *immutable = "", false
	if t.Enabled {
		*offsite, *immutable = t.Repo, t.Immutable
	}
	if after == before {
		return nil
	}
	return updateSettings(tx, after)
}
