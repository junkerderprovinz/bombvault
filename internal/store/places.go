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
// empty domain is the place's base itself. A row that changes place leaves
// the mark of its old one behind, since only the place that switched a row
// off may switch it back on.
func AttachRowTx(tx *sql.Tx, rowID, placeID, domain, suffix string) error {
	res, err := tx.Exec(`UPDATE offsite_targets SET place_id = ?, place_domain = ?, place_suffix = ?,
		  off_with_place = CASE WHEN place_id = ? THEN off_with_place ELSE 0 END
		WHERE id = ?`, placeID, domain, suffix, placeID, rowID)
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

// DetachRowTx takes a row off its place. The row keeps its address and its
// settings and goes on working as a row without a place.
func DetachRowTx(tx *sql.Tx, rowID string) error {
	return AttachRowTx(tx, rowID, "", "", "")
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

	// ErrPlaceOwnedField refuses a row edit of a value the row's place writes.
	ErrPlaceOwnedField = errors.New("the storage place of this row sets this value; change it in the place's details")
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

// WritePlace inserts or updates the place and mirrors it, all in one
// transaction: onto every row at it, into the path of each domain whose home
// place it is and, at a remote place, that domain's primary row, and into
// the off-site field of each domain whose field row stands there. It also
// makes the place the home place of the domains w names and writes the
// credential blob. Only what differs is written, so saving an unchanged
// place leaves every row as it was. A place with an empty or unknown ID is
// new and goes behind the others.
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
	homes, err := domainPlacesQ(tx)
	if err != nil {
		return Place{}, fmt.Errorf("WritePlace homes: %w", err)
	}
	if err := homePrimariesTx(tx, p, homes); err != nil {
		return Place{}, err
	}
	if err := mirrorPlaceRowsTx(tx, p); err != nil {
		return Place{}, err
	}
	settings, err := getSettings(tx)
	if err != nil {
		return Place{}, err
	}
	next := settings
	if w.CredSetsBlob != nil {
		next.CloudCredSets = string(w.CredSetsBlob)
	}
	if err := mirrorPlaceSettingsTx(tx, p, homes, &next); err != nil {
		return Place{}, err
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

// ErrPlaceFolderMissing refuses a place write that would leave a row at the
// place, or a domain whose home place it is, without a folder there.
var ErrPlaceFolderMissing = errors.New("the place has no folder for a domain that uses it")

// mirrorPlaceRowsTx writes p onto every row at it, only the columns that
// differ, and then carries each target's fields on to its direct repository
// as a target save does. A direct repository takes its address and switch
// from p and the rest from its target, credentials excepted: new ones reach
// it only once they open it.
func mirrorPlaceRowsTx(tx *sql.Tx, p Place) error {
	rows, err := placeRowsQ(tx, p.ID)
	if err != nil {
		return fmt.Errorf("WritePlace rows: %w", err)
	}
	for _, row := range rows {
		addr, ok := places.Address(p.Base, p.Folders, row.PlaceDomain, row.PlaceSuffix)
		if !ok {
			return fmt.Errorf("%w: %s", ErrPlaceFolderMissing, row.PlaceDomain)
		}
		set, vals := placedChanges(p, row, addr)
		if len(set) == 0 {
			continue
		}
		//nolint:gosec // G202: set holds fixed column names from placedChanges, never user text; every value travels in vals.
		if _, err := tx.Exec(`UPDATE offsite_targets SET `+strings.Join(set, ", ")+` WHERE id = ?`, append(vals, row.ID)...); err != nil {
			return fmt.Errorf("WritePlace row %s: %w", row.ID, err)
		}
	}
	for _, row := range rows {
		if row.Role != RoleOffsite {
			continue
		}
		target, err := offsiteTargetTx(tx, row.ID)
		if err != nil {
			return err
		}
		if err := mirrorTx(tx, target, false); err != nil {
			return fmt.Errorf("WritePlace direct repository of %s: %w", row.ID, err)
		}
	}
	return nil
}

// placedChanges lists the columns of row that differ from what p says, with
// their new values. A place that is off holds every row off and marks the
// ones it switched off, and a place that is on switches the marked ones back
// on. A row switched off by itself carries no mark, so it stays off through
// any edit of the place, the place going off and on included.
func placedChanges(p Place, row OffsiteTarget, addr string) ([]string, []any) {
	var set []string
	var vals []any
	add := func(col string, differs bool, v any) {
		if differs {
			set = append(set, col+" = ?")
			vals = append(vals, v)
		}
	}
	add("repo", row.Repo != addr, addr)
	if row.CompanionOf == "" {
		add("creds_ref", row.CredsRef != p.CredsRef, p.CredsRef)
		add("storage_class", row.StorageClass != p.StorageClass, p.StorageClass)
		add("immutable", row.Immutable != p.Immutable, boolInt(p.Immutable))
		add("retention_keep_last", row.RetentionKeepLast != p.RetentionKeepLast, p.RetentionKeepLast)
		add("retention_keep_daily", row.RetentionKeepDaily != p.RetentionKeepDaily, p.RetentionKeepDaily)
		add("retention_keep_weekly", row.RetentionKeepWeekly != p.RetentionKeepWeekly, p.RetentionKeepWeekly)
		add("retention_keep_monthly", row.RetentionKeepMonthly != p.RetentionKeepMonthly, p.RetentionKeepMonthly)
		add("limit_upload", row.LimitUpload != p.LimitUpload, p.LimitUpload)
		add("limit_download", row.LimitDownload != p.LimitDownload, p.LimitDownload)
		add("growth_budget_gb", row.GrowthBudgetGB != p.GrowthBudgetGB, p.GrowthBudgetGB)
		if row.Role == RoleRepo {
			add("off_premises", row.OffPremises != p.OffPremises, boolInt(p.OffPremises))
		}
	}
	on, marked := row.Enabled, row.OffWithPlace
	switch {
	case !p.Enabled && on:
		on, marked = false, true
	case p.Enabled && marked:
		on, marked = true, false
	}
	add("enabled", row.Enabled != on, boolInt(on))
	add("off_with_place", row.OffWithPlace != marked, boolInt(marked))
	return set, vals
}

// placeOwnedChanges names the columns a place sets that t would change on
// stored, a row at that place.
func placeOwnedChanges(stored, t OffsiteTarget) []string {
	p := Place{CredsRef: t.CredsRef, StorageClass: t.StorageClass, Immutable: t.Immutable,
		RetentionKeepLast: t.RetentionKeepLast, RetentionKeepDaily: t.RetentionKeepDaily,
		RetentionKeepWeekly: t.RetentionKeepWeekly, RetentionKeepMonthly: t.RetentionKeepMonthly,
		LimitUpload: t.LimitUpload, LimitDownload: t.LimitDownload, GrowthBudgetGB: t.GrowthBudgetGB,
		OffPremises: t.OffPremises, Enabled: stored.Enabled}
	set, _ := placedChanges(p, stored, stored.Repo)
	var cols []string
	for _, s := range set {
		if col := strings.TrimSuffix(s, " = ?"); col != "enabled" && col != "off_with_place" {
			cols = append(cols, col)
		}
	}
	return cols
}

// mirrorPlaceSettingsTx writes p into the settings row s: the path of every
// domain whose home place p is, and the off-site field of every domain whose
// field row stands at p.
func mirrorPlaceSettingsTx(tx *sql.Tx, p Place, homes map[string]string, s *Settings) error {
	for _, domain := range places.Domains {
		path, _, _ := domainColumns(s, domain)
		if homes[domain] == p.ID {
			addr, ok := places.Address(p.Base, p.Folders, domain, "")
			if !ok {
				return fmt.Errorf("%w: %s", ErrPlaceFolderMissing, domain)
			}
			*path = addr
		}
		field, found, err := fieldRowQ(tx, domain)
		if err != nil {
			return fmt.Errorf("WritePlace field of %s: %w", domain, err)
		}
		if found && field.PlaceID == p.ID {
			fillField(s, field)
		}
	}
	return nil
}

// fillField writes a domain's field row into its off-site field. The field
// holds the row's address only while the row is on: left filled under a row
// that is off, it would have replication fall back to the field and copy
// there anyway.
func fillField(s *Settings, row OffsiteTarget) {
	_, offsite, immutable := domainColumns(s, row.Domain)
	*offsite, *immutable = "", false
	if row.Enabled {
		*offsite, *immutable = row.Repo, row.Immutable
	}
}

// domainColumns points at the settings fields a place writes for domain:
// its path, its off-site field and that field's append-only flag.
func domainColumns(s *Settings, domain string) (path, offsite *string, immutable *bool) {
	switch domain {
	case "containers":
		return &s.ContainersPath, &s.ContainersOffsite, &s.ContainersOffsiteImmutable
	case "vms":
		return &s.VMsPath, &s.VMsOffsite, &s.VMsOffsiteImmutable
	case "flash":
		return &s.FlashPath, &s.FlashOffsite, &s.FlashOffsiteImmutable
	case "config":
		return &s.ConfigPath, &s.ConfigOffsite, &s.ConfigOffsiteImmutable
	case "files":
		return &s.FilesPath, &s.FilesOffsite, &s.FilesOffsiteImmutable
	case "zfs":
		return &s.ZFSPath, &s.ZFSOffsite, &s.ZFSOffsiteImmutable
	}
	return nil, nil, nil
}

// homePrimariesTx keeps the primary row of every domain whose home place p
// is. At a remote place that row carries the domain path's credentials,
// caps, append-only flag and budget, so it is created when missing and moved
// onto p from wherever it was, on like the domain path it serves; the row
// mirror then writes p into it, switch included. At a local place none of
// that applies, so the row leaves its place and is switched off.
func homePrimariesTx(tx *sql.Tx, p Place, homes map[string]string) error {
	for _, domain := range places.Domains {
		if homes[domain] != p.ID {
			continue
		}
		row, found, err := primaryRowQ(tx, domain)
		if err != nil {
			return fmt.Errorf("WritePlace primary of %s: %w", domain, err)
		}
		switch {
		case p.Kind == string(places.KindLocal):
			if found && row.PlaceID != "" {
				if err := DetachRowTx(tx, row.ID); err != nil {
					return err
				}
			}
			if found && row.Enabled {
				if _, err := tx.Exec(`UPDATE offsite_targets SET enabled = 0 WHERE id = ?`, row.ID); err != nil {
					return fmt.Errorf("WritePlace primary of %s: %w", domain, err)
				}
			}
		case !found:
			addr, ok := places.Address(p.Base, p.Folders, domain, "")
			if !ok {
				return fmt.Errorf("%w: %s", ErrPlaceFolderMissing, domain)
			}
			if _, err := tx.Exec(`INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, place_id, place_domain)
				VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?)`,
				newID(), domain, primaryRowName, addr, RolePrimary, time.Now().Unix(), p.ID, domain); err != nil {
				return fmt.Errorf("WritePlace primary of %s: %w", domain, err)
			}
		case row.PlaceID != p.ID || row.PlaceDomain != domain || row.PlaceSuffix != "":
			if err := AttachRowTx(tx, row.ID, p.ID, domain, ""); err != nil {
				return err
			}
			if _, err := tx.Exec(`UPDATE offsite_targets SET enabled = 1 WHERE id = ?`, row.ID); err != nil {
				return fmt.Errorf("WritePlace primary of %s: %w", domain, err)
			}
		}
	}
	return nil
}

// PlaceHolders is what keeps a place from being removed.
type PlaceHolders struct {
	HomeDomains []string  // domains whose home place it is
	Defaults    []string  // domains whose placement default homes items on a repository there
	Items       []ItemRef // items that back up to a repository there
	DirectInUse []string  // targets there whose direct repository an item or a default uses
}

// InUse reports whether anything holds the place.
func (h PlaceHolders) InUse() bool {
	return len(h.HomeDomains)+len(h.Defaults)+len(h.Items)+len(h.DirectInUse) > 0
}

// PlaceHolders lists what holds a place.
func (r *Repo) PlaceHolders(id string) (PlaceHolders, error) {
	h, err := placeHoldersQ(r.db, id)
	if err != nil {
		return PlaceHolders{}, fmt.Errorf("PlaceHolders: %w", err)
	}
	return h, nil
}

// DeletePlaceIfUnused removes a place that nothing holds, in one transaction
// with what was derived from it: its targets, each with its direct repository
// as DeleteOffsiteTargetIfUnused removes them, its named repositories, and
// the off-site field of each domain whose field row stood there. Rows the
// place does not own outright, a domain's primary row or a direct repository
// whose target stands elsewhere, stay and leave the place. While PlaceHolders
// names anything it returns ErrPlaceInUse and writes nothing. The count is
// the number of targets removed.
func (r *Repo) DeletePlaceIfUnused(id string) (int, error) {
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("DeletePlaceIfUnused: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	_, found, err := placeQ(tx, id)
	if err != nil {
		return 0, fmt.Errorf("DeletePlaceIfUnused: %w", err)
	}
	if !found {
		return 0, fmt.Errorf("DeletePlaceIfUnused %s: %w", id, ErrPlaceNotFound)
	}
	holders, err := placeHoldersQ(tx, id)
	if err != nil {
		return 0, fmt.Errorf("DeletePlaceIfUnused: %w", err)
	}
	if holders.InUse() {
		return 0, ErrPlaceInUse
	}
	if err := emptyPlaceFieldsTx(tx, id); err != nil {
		return 0, fmt.Errorf("DeletePlaceIfUnused: %w", err)
	}
	rows, err := placeRowsQ(tx, id)
	if err != nil {
		return 0, fmt.Errorf("DeletePlaceIfUnused: %w", err)
	}
	removed := 0
	for _, row := range rows {
		switch {
		case row.Role == RoleOffsite:
			// DirectInUse came back empty, so every target here goes.
			if _, err := deleteOffsiteTargetIfUnusedTx(tx, row.ID); err != nil {
				return 0, err
			}
			removed++
		case row.Role == RoleRepo && row.CompanionOf == "":
			if _, err := tx.Exec(`DELETE FROM offsite_targets WHERE id = ? AND role = ?`, row.ID, RoleRepo); err != nil {
				return 0, fmt.Errorf("DeletePlaceIfUnused: %w", err)
			}
		}
	}
	if _, err := tx.Exec(`UPDATE offsite_targets SET place_id = '', place_domain = '', place_suffix = '', off_with_place = 0
		WHERE place_id = ?`, id); err != nil {
		return 0, fmt.Errorf("DeletePlaceIfUnused: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM storage_places WHERE id = ?`, id); err != nil {
		return 0, fmt.Errorf("DeletePlaceIfUnused: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("DeletePlaceIfUnused commit: %w", err)
	}
	return removed, nil
}

// emptyPlaceFieldsTx empties the off-site field of every domain whose field
// row stands at the place. Left filled after the row goes, the field would
// keep replication copying there and bring the row back at the next settings
// save.
func emptyPlaceFieldsTx(tx *sql.Tx, placeID string) error {
	settings, err := getSettings(tx)
	if err != nil {
		return err
	}
	next := settings
	for _, domain := range places.Domains {
		field, found, err := fieldRowQ(tx, domain)
		if err != nil {
			return fmt.Errorf("field of %s: %w", domain, err)
		}
		if found && field.PlaceID == placeID {
			_, offsite, immutable := domainColumns(&next, domain)
			*offsite, *immutable = "", false
		}
	}
	if next == settings {
		return nil
	}
	return updateSettings(tx, next)
}

// placeHoldersQ reads each list with a query of its own, closed before the
// next one starts, because the store's database has a single connection.
func placeHoldersQ(q queryer, id string) (PlaceHolders, error) {
	var h PlaceHolders
	var err error
	if h.HomeDomains, err = stringsQ(q, `SELECT domain FROM storage_domain_places WHERE place_id = ? ORDER BY domain`, id); err != nil {
		return h, err
	}
	if h.Defaults, err = stringsQ(q, `SELECT d.domain FROM placement_defaults d
		JOIN offsite_targets t ON t.id = d.home AND t.role = ?
		WHERE t.place_id = ? ORDER BY d.domain`, RoleRepo, id); err != nil {
		return h, err
	}
	if h.Items, err = itemsAtPlaceQ(q, id); err != nil {
		return h, err
	}
	h.DirectInUse, err = stringsQ(q, `SELECT t.id FROM offsite_targets t
		JOIN offsite_targets c ON c.companion_of = t.id AND c.role = ?
		WHERE t.place_id = ? AND t.role = ?
		  AND (EXISTS (SELECT 1 FROM targets WHERE repo = c.id)
		    OR EXISTS (SELECT 1 FROM vms WHERE repo = c.id)
		    OR EXISTS (SELECT 1 FROM file_sets WHERE repo = c.id)
		    OR EXISTS (SELECT 1 FROM placement_defaults WHERE home = c.id))
		ORDER BY t.id`, RoleRepo, id, RoleOffsite)
	return h, err
}

// itemsAtPlaceQ lists the containers, VMs and file sets whose repository is
// a named or direct repository at the place.
func itemsAtPlaceQ(q queryer, id string) ([]ItemRef, error) {
	rows, err := q.Query(`
		SELECT 'containers', container_name FROM targets   WHERE repo IN (SELECT id FROM offsite_targets WHERE place_id = ? AND role = ?)
		UNION ALL
		SELECT 'vms',        name           FROM vms       WHERE repo IN (SELECT id FROM offsite_targets WHERE place_id = ? AND role = ?)
		UNION ALL
		SELECT 'files',      id             FROM file_sets WHERE repo IN (SELECT id FROM offsite_targets WHERE place_id = ? AND role = ?)
		ORDER BY 1, 2`, id, RoleRepo, id, RoleRepo, id, RoleRepo)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []ItemRef{}
	for rows.Next() {
		var it ItemRef
		if err := rows.Scan(&it.Domain, &it.Key); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// stringsQ runs a query of one text column and returns its values, never nil.
func stringsQ(q queryer, query string, args ...any) ([]string, error) {
	rows, err := q.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// detachMovedRowTx takes a row off its place when a write gives it another
// address: the place does not spell the new one, and the row goes on working
// from the address it was given.
func detachMovedRowTx(tx *sql.Tx, id, role, repo string) error {
	_, err := tx.Exec(`UPDATE offsite_targets SET place_id = '', place_domain = '', place_suffix = '', off_with_place = 0
		WHERE id = ? AND role = ? AND place_id <> '' AND repo <> ?`, id, role, repo)
	return err
}

// PlaceOwned names the settings columns of a domain that its places write.
// A home place writes the domain's path, so Path reports that. Every domain
// PlaceOwnedSettings lists also has its off-site field and that field's
// append-only flag written by the place its field row stands at.
type PlaceOwned struct {
	Path bool
}

// PlaceOwnedSettings lists, for each domain with a column a place writes,
// which of its columns those are.
func (r *Repo) PlaceOwnedSettings() (map[string]PlaceOwned, error) {
	owned, err := placeOwnedQ(r.db)
	if err != nil {
		return nil, fmt.Errorf("PlaceOwnedSettings: %w", err)
	}
	return owned, nil
}

func placeOwnedQ(q queryer) (map[string]PlaceOwned, error) {
	homes, err := domainPlacesQ(q)
	if err != nil {
		return nil, err
	}
	owned := map[string]PlaceOwned{}
	for _, domain := range places.Domains {
		field, found, err := fieldRowQ(q, domain)
		if err != nil {
			return nil, err
		}
		home := homes[domain] != ""
		if home || (found && field.PlaceID != "") {
			owned[domain] = PlaceOwned{Path: home}
		}
	}
	return owned, nil
}

// MutateSettingsKeepingPlaces is MutateSettings for a writer that does not
// own what places write into the settings row: those columns keep their
// stored values whatever fn sets. The places are read in the same
// transaction, so a place written in the meantime is kept too.
func (r *Repo) MutateSettingsKeepingPlaces(fn func(*Settings) error) (Settings, error) {
	return r.mutateSettings(func(tx *sql.Tx, before Settings, after *Settings) error {
		if err := fn(after); err != nil {
			return err
		}
		owned, err := placeOwnedQ(tx)
		if err != nil {
			return fmt.Errorf("MutateSettingsKeepingPlaces: %w", err)
		}
		for domain, o := range owned {
			path, offsite, immutable := domainColumns(after, domain)
			storedPath, storedOffsite, storedImmutable := domainColumns(&before, domain)
			if o.Path {
				*path = *storedPath
			}
			*offsite, *immutable = *storedOffsite, *storedImmutable
		}
		return nil
	})
}
