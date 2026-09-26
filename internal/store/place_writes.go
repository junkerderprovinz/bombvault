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
