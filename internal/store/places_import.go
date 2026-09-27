package store

import (
	"database/sql"
	"fmt"
	"maps"
	"slices"
	"time"
)

// PlacesImport is the storage places a settings file carries: the places,
// the domain each one is home to and the rows that sit on them.
type PlacesImport struct {
	Places      []Place
	HomeDomains map[string]string // domain -> place id
	Links       []PlaceLink
}

// PlaceLink puts one offsite_targets row on a place.
type PlaceLink struct {
	RowID   string
	PlaceID string
	Domain  string
	Suffix  string
}

// ReplacePlaces makes in the only storage places, in one transaction, and
// marks the places migrated: they came whole from a file, and a second set
// built by the startup migration would duplicate them. The locations and
// mirrored fields the links imply are left to WritePlace.
func (r *Repo) ReplacePlaces(in PlacesImport) error {
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	now := time.Now().Unix()
	return r.inTx(func(tx *sql.Tx) error {
		if err := dropPlacesTx(tx); err != nil {
			return err
		}
		for _, p := range in.Places {
			if err := insertImportedPlaceTx(tx, p, now); err != nil {
				return err
			}
		}
		for _, domain := range slices.Sorted(maps.Keys(in.HomeDomains)) {
			if err := r.SetDomainPlaceTx(tx, domain, in.HomeDomains[domain]); err != nil {
				return fmt.Errorf("ReplacePlaces: %w", err)
			}
		}
		for _, l := range in.Links {
			if err := AttachRowTx(tx, l.RowID, l.PlaceID, l.Domain, l.Suffix); err != nil {
				return fmt.Errorf("ReplacePlaces: %w", err)
			}
		}
		for _, p := range in.Places {
			if err := markOffWithPlaceTx(tx, p); err != nil {
				return fmt.Errorf("ReplacePlaces: %w", err)
			}
		}
		if _, err := tx.Exec(`UPDATE settings SET places_migrated = ? WHERE id = 1`, now); err != nil {
			return fmt.Errorf("ReplacePlaces mark: %w", err)
		}
		return nil
	})
}

// DropPlaces removes every storage place with its home domains and row links
// and clears places_migrated, so the places migration can build them again
// from the rows as they are.
func (r *Repo) DropPlaces() error {
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	return r.inTx(func(tx *sql.Tx) error {
		if err := dropPlacesTx(tx); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE settings SET places_migrated = 0 WHERE id = 1`); err != nil {
			return fmt.Errorf("DropPlaces mark: %w", err)
		}
		return nil
	})
}

// markOffWithPlaceTx marks the rows that are off at p when p is off. Nothing
// says which switch turned such a row off, so it goes with the place, as on
// upgrade.
func markOffWithPlaceTx(tx *sql.Tx, p Place) error {
	if p.Enabled {
		return nil
	}
	if _, err := tx.Exec(`UPDATE offsite_targets SET off_with_place = 1 WHERE enabled = 0 AND place_id = ?`, p.ID); err != nil {
		return fmt.Errorf("mark the rows off with %s: %w", p.Name, err)
	}
	return nil
}

func dropPlacesTx(tx *sql.Tx) error {
	for _, q := range []string{
		`DELETE FROM storage_places`,
		`DELETE FROM storage_domain_places`,
		`UPDATE offsite_targets SET place_id = '', place_domain = '', place_suffix = '', off_with_place = 0 WHERE place_id <> ''`,
	} {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("drop places: %w", err)
		}
	}
	return nil
}

// insertImportedPlaceTx writes a place with the id, sort order and timestamps
// it has in the file, where savePlaceTx would mint and stamp its own; a zero
// timestamp is stamped now.
func insertImportedPlaceTx(tx *sql.Tx, p Place, now int64) error {
	if p.CreatedAt == 0 {
		p.CreatedAt = now
	}
	if p.UpdatedAt == 0 {
		p.UpdatedAt = now
	}
	if _, err := tx.Exec(`INSERT INTO storage_places (`+placeCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		append([]any{p.ID}, placeValues(p)...)...); err != nil {
		return fmt.Errorf("ReplacePlaces %s: %w", p.Name, err)
	}
	return nil
}
