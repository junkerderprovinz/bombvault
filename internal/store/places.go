package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

// AttachRowTx puts a row at a place. Its repo must already be the place's
// address for domain plus suffix; WritePlace keeps it so from then on. An
// empty domain is the place's base itself.
func AttachRowTx(tx *sql.Tx, rowID, placeID, domain, suffix string) error {
	return setRowPlaceTx(tx, rowID, placeID, domain, suffix)
}

// DetachRowTx takes a row off its place. The row keeps its address and its
// settings and goes on working as a row without a place.
func DetachRowTx(tx *sql.Tx, rowID string) error {
	return setRowPlaceTx(tx, rowID, "", "", "")
}

func setRowPlaceTx(tx *sql.Tx, rowID, placeID, domain, suffix string) error {
	res, err := tx.Exec(`UPDATE offsite_targets SET place_id = ?, place_domain = ?, place_suffix = ? WHERE id = ?`,
		placeID, domain, suffix, rowID)
	if err != nil {
		return fmt.Errorf("place of row %s: %w", rowID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("place of row %s: %w", rowID, err)
	}
	if n == 0 {
		return fmt.Errorf("place of row %s: %w", rowID, sql.ErrNoRows)
	}
	return nil
}

// PlaceRows returns the offsite_targets rows at a place, any role.
func (r *Repo) PlaceRows(placeID string) ([]OffsiteTarget, error) {
	rows, err := placeRowsQ(r.db, placeID)
	if err != nil {
		return nil, fmt.Errorf("PlaceRows: %w", err)
	}
	return rows, nil
}

func placeRowsQ(q queryer, placeID string) ([]OffsiteTarget, error) {
	rows, err := q.Query(`SELECT `+offsiteTargetCols+` FROM offsite_targets
		WHERE place_id = ? AND place_id <> '' ORDER BY place_domain, place_suffix, role, created_at, id`, placeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []OffsiteTarget{}
	for rows.Next() {
		t, err := scanOffsiteTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Place is a connected storage location: a base address, the folder of each
// domain it offers, and the credentials, retention, protection and caps that
// every repository there shares. Folders maps a domain to its folder; a
// domain missing there is not offered, and an empty folder is the base.
type Place struct {
	ID                   string
	Name                 string
	Provider             string
	Kind                 string
	Base                 string
	Folders              map[string]string
	CredsRef             string
	OffPremises          bool
	StorageClass         string
	Immutable            bool
	RetentionKeepLast    int
	RetentionKeepDaily   int
	RetentionKeepWeekly  int
	RetentionKeepMonthly int
	LimitUpload          int
	LimitDownload        int
	GrowthBudgetGB       int
	Enabled              bool
	SortOrder            int
	CreatedAt            int64
	UpdatedAt            int64
}

var (
	ErrPlaceNotFound  = errors.New("place not found")
	ErrPlaceNameTaken = errors.New("place name taken")
	ErrPlaceInUse     = errors.New("place in use")
)

var errPlaceIncomplete = errors.New("a place needs a name and a base address")

// PlaceWrite is one atomic change: the place row, optionally the encrypted
// credential blob, and the domains whose home place changes.
type PlaceWrite struct {
	Place Place
	// CredSetsBlob replaces settings.cloud_cred_sets; nil leaves it alone.
	CredSetsBlob []byte
	// HomeDomains makes the place the home place of each domain it lists. A
	// value is the place's own id, or empty for a place this write creates.
	HomeDomains map[string]string
}

const placeCols = `id, name, provider, kind, base, folders, creds_ref, off_premises, storage_class, immutable,
	retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
	limit_upload, limit_download, growth_budget_gb, enabled, sort_order, created_at, updated_at`

// ListPlaces returns every place in the order the storage tab shows them.
func (r *Repo) ListPlaces() ([]Place, error) {
	rows, err := r.db.Query(`SELECT ` + placeCols + ` FROM storage_places ORDER BY sort_order, created_at, name`)
	if err != nil {
		return nil, fmt.Errorf("ListPlaces: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []Place{}
	for rows.Next() {
		p, err := scanPlace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPlace returns one place, or ErrPlaceNotFound.
func (r *Repo) GetPlace(id string) (Place, error) {
	p, found, err := placeQ(r.db, id)
	if err != nil {
		return Place{}, fmt.Errorf("GetPlace: %w", err)
	}
	if !found {
		return Place{}, fmt.Errorf("GetPlace %s: %w", id, ErrPlaceNotFound)
	}
	return p, nil
}

// DomainPlaces maps each domain that has a home place to that place's id.
func (r *Repo) DomainPlaces() (map[string]string, error) {
	homes, err := domainPlacesQ(r.db)
	if err != nil {
		return nil, fmt.Errorf("DomainPlaces: %w", err)
	}
	return homes, nil
}

// SetDomainPlaceTx makes placeID the home place of domain, or with an empty
// placeID leaves the domain without one. It sets the link and nothing else.
func (r *Repo) SetDomainPlaceTx(tx *sql.Tx, domain, placeID string) error {
	if !slices.Contains(places.Domains, domain) {
		return fmt.Errorf("home place of %q: not a domain", domain)
	}
	if placeID == "" {
		if _, err := tx.Exec(`DELETE FROM storage_domain_places WHERE domain = ?`, domain); err != nil {
			return fmt.Errorf("home place of %s: %w", domain, err)
		}
		return nil
	}
	_, found, err := placeQ(tx, placeID)
	if err != nil {
		return fmt.Errorf("home place of %s: %w", domain, err)
	}
	if !found {
		return fmt.Errorf("home place of %s: %w", domain, ErrPlaceNotFound)
	}
	if _, err := tx.Exec(`INSERT INTO storage_domain_places (domain, place_id) VALUES (?, ?)
		ON CONFLICT(domain) DO UPDATE SET place_id = excluded.place_id
		WHERE storage_domain_places.place_id <> excluded.place_id`, domain, placeID); err != nil {
		return fmt.Errorf("home place of %s: %w", domain, err)
	}
	return nil
}

// WritePlace inserts or updates the place, makes it the home place of the
// domains w names and writes the credential blob, all in one transaction. A
// place with an empty or unknown ID is new and goes behind the others; an
// unchanged place is not written.
func (r *Repo) WritePlace(w PlaceWrite) (Place, error) {
	p := w.Place
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Base) == "" {
		return Place{}, errPlaceIncomplete
	}
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	tx, err := r.db.Begin()
	if err != nil {
		return Place{}, fmt.Errorf("WritePlace: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	before, existed, err := placeQ(tx, p.ID)
	if err != nil {
		return Place{}, fmt.Errorf("WritePlace: %w", err)
	}
	if p, err = savePlaceTx(tx, p, before, existed); err != nil {
		return Place{}, err
	}
	for _, domain := range slices.Sorted(maps.Keys(w.HomeDomains)) {
		if id := w.HomeDomains[domain]; id != "" && id != p.ID {
			return Place{}, fmt.Errorf("WritePlace: the home place of %s can only be the place written", domain)
		}
		if err := r.SetDomainPlaceTx(tx, domain, p.ID); err != nil {
			return Place{}, err
		}
	}
	settings, err := getSettings(tx)
	if err != nil {
		return Place{}, err
	}
	next := settings
	if w.CredSetsBlob != nil {
		next.CloudCredSets = string(w.CredSetsBlob)
	}
	if next != settings {
		if err := updateSettings(tx, next); err != nil {
			return Place{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Place{}, fmt.Errorf("WritePlace commit: %w", err)
	}
	return p, nil
}

// savePlaceTx inserts p, or writes it over before when a column differs. An
// unchanged place is left as stored, updated_at included.
func savePlaceTx(tx *sql.Tx, p, before Place, existed bool) (Place, error) {
	var taken bool
	if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM storage_places WHERE name = ? AND id <> ?)`, p.Name, p.ID).Scan(&taken); err != nil {
		return Place{}, fmt.Errorf("WritePlace: %w", err)
	}
	if taken {
		return Place{}, ErrPlaceNameTaken
	}
	now := time.Now().Unix()
	if !existed {
		if p.ID == "" {
			p.ID = newID()
		}
		p.CreatedAt, p.UpdatedAt = now, now
		if err := tx.QueryRow(`SELECT COALESCE(MAX(sort_order) + 1, 0) FROM storage_places`).Scan(&p.SortOrder); err != nil {
			return Place{}, fmt.Errorf("WritePlace: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO storage_places (`+placeCols+`)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			append([]any{p.ID}, placeValues(p)...)...); err != nil {
			return Place{}, fmt.Errorf("WritePlace: %w", err)
		}
		return p, nil
	}
	p.CreatedAt, p.UpdatedAt = before.CreatedAt, before.UpdatedAt
	if slices.Equal(placeValues(p), placeValues(before)) {
		return before, nil
	}
	p.UpdatedAt = now
	if _, err := tx.Exec(`UPDATE storage_places SET name = ?, provider = ?, kind = ?, base = ?, folders = ?, creds_ref = ?,
		  off_premises = ?, storage_class = ?, immutable = ?,
		  retention_keep_last = ?, retention_keep_daily = ?, retention_keep_weekly = ?, retention_keep_monthly = ?,
		  limit_upload = ?, limit_download = ?, growth_budget_gb = ?, enabled = ?, sort_order = ?, created_at = ?, updated_at = ?
		WHERE id = ?`, append(placeValues(p), p.ID)...); err != nil {
		return Place{}, fmt.Errorf("WritePlace: %w", err)
	}
	return p, nil
}

// placeValues are the columns of placeCols after id, in that order.
func placeValues(p Place) []any {
	return []any{p.Name, p.Provider, p.Kind, p.Base, encodeFolders(p.Folders), p.CredsRef, boolInt(p.OffPremises),
		p.StorageClass, boolInt(p.Immutable),
		p.RetentionKeepLast, p.RetentionKeepDaily, p.RetentionKeepWeekly, p.RetentionKeepMonthly,
		p.LimitUpload, p.LimitDownload, p.GrowthBudgetGB, boolInt(p.Enabled), p.SortOrder, p.CreatedAt, p.UpdatedAt}
}

// encodeFolders spells folders the way every writer of the column does, JSON
// with sorted keys and {} for none, so equal folders are equal bytes.
func encodeFolders(f map[string]string) string {
	if len(f) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(f)
	return string(b)
}

func placeQ(q queryer, id string) (Place, bool, error) {
	p, err := scanPlace(q.QueryRow(`SELECT `+placeCols+` FROM storage_places WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Place{}, false, nil
	}
	if err != nil {
		return Place{}, false, err
	}
	return p, true, nil
}

func scanPlace(s scanner) (Place, error) {
	var p Place
	var folders string
	var offPremises, immutable, enabled int
	err := s.Scan(&p.ID, &p.Name, &p.Provider, &p.Kind, &p.Base, &folders, &p.CredsRef, &offPremises,
		&p.StorageClass, &immutable,
		&p.RetentionKeepLast, &p.RetentionKeepDaily, &p.RetentionKeepWeekly, &p.RetentionKeepMonthly,
		&p.LimitUpload, &p.LimitDownload, &p.GrowthBudgetGB, &enabled, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return Place{}, fmt.Errorf("scanPlace: %w", err)
	}
	p.Folders = map[string]string{}
	if err := json.Unmarshal([]byte(folders), &p.Folders); err != nil {
		return Place{}, fmt.Errorf("folders of place %s: %w", p.ID, err)
	}
	p.OffPremises, p.Immutable, p.Enabled = offPremises != 0, immutable != 0, enabled != 0
	return p, nil
}

func domainPlacesQ(q queryer) (map[string]string, error) {
	rows, err := q.Query(`SELECT domain, place_id FROM storage_domain_places`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	homes := map[string]string{}
	for rows.Next() {
		var domain, id string
		if err := rows.Scan(&domain, &id); err != nil {
			return nil, err
		}
		homes[domain] = id
	}
	return homes, rows.Err()
}
