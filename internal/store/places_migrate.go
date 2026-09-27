package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

// ErrPlacesMigrated reports that the database was moved onto places already.
var ErrPlacesMigrated = errors.New("storage places are already migrated")

// PlaceRowRef is one offsite_targets row the migration puts on a place, with
// the address the row must still hold when the write happens.
type PlaceRowRef struct {
	RowID  string
	Domain string // empty for a place that is one repository for every domain
	Suffix string
	Repo   string
}

// MigratedPlace is one place the migration creates, the rows it takes over and
// the domains whose path lies there.
type MigratedPlace struct {
	Place       Place
	Rows        []PlaceRowRef
	HomeDomains []string
}

// ApplyPlacesMigration writes the places in the order given, puts every row
// and domain path on its place, copies each place's keep-policy onto its rows
// and marks the settings row migrated, all in one transaction. The plan comes
// from an earlier read, so an address that moved in between fails the whole
// write and the next run plans again.
func (r *Repo) ApplyPlacesMigration(migrated []MigratedPlace) error {
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	return r.inTx(func(tx *sql.Tx) error {
		settings, err := getSettings(tx)
		if err != nil {
			return err
		}
		if settings.PlacesMigrated != 0 {
			return ErrPlacesMigrated
		}
		paths := map[string]string{
			"containers": settings.ContainersPath, "vms": settings.VMsPath, "flash": settings.FlashPath,
			"config": settings.ConfigPath, "files": settings.FilesPath,
		}
		for _, m := range migrated {
			p, err := savePlaceTx(tx, m.Place, Place{}, false)
			if err != nil {
				return fmt.Errorf("place %s: %w", m.Place.Name, err)
			}
			for _, ref := range m.Rows {
				if err := attachMigratedRowTx(tx, p, ref); err != nil {
					return err
				}
			}
			if err := markOffWithPlaceTx(tx, p); err != nil {
				return err
			}
			for _, d := range m.HomeDomains {
				if addr, offered := places.Address(p.Base, p.Folders, d, ""); !offered || addr != paths[d] {
					return fmt.Errorf("the %s path changed while the places were planned", d)
				}
				if err := r.SetDomainPlaceTx(tx, d, p.ID); err != nil {
					return err
				}
			}
			if err := mirrorRetentionTx(tx, p); err != nil {
				return err
			}
		}
		_, err = tx.Exec(`UPDATE settings SET places_migrated = ? WHERE id = 1`, time.Now().Unix())
		return err
	})
}

// attachMigratedRowTx puts one row on p once it is sure the row still holds
// the address p spells for it and sits on no place yet. The error names the
// row id only: an address can carry credentials, and this ends up in the log.
func attachMigratedRowTx(tx *sql.Tx, p Place, ref PlaceRowRef) error {
	var repo, placeID string
	if err := tx.QueryRow(`SELECT repo, place_id FROM offsite_targets WHERE id = ?`, ref.RowID).Scan(&repo, &placeID); err != nil {
		return fmt.Errorf("read row %s: %w", ref.RowID, err)
	}
	want, offered := places.Address(p.Base, p.Folders, ref.Domain, ref.Suffix)
	if !offered || want != repo || repo != ref.Repo || placeID != "" {
		return fmt.Errorf("row %s changed while the places were planned", ref.RowID)
	}
	return AttachRowTx(tx, ref.RowID, p.ID, ref.Domain, ref.Suffix)
}

// mirrorRetentionTx copies p's keep-policy onto its rows where it differs.
// The plan puts a row only on a place whose other settings it already has;
// the keep-policy is the exception, because a named repository ages by the
// global local rule and not by the zeros in its own columns.
func mirrorRetentionTx(tx *sql.Tx, p Place) error {
	_, err := tx.Exec(`UPDATE offsite_targets
		   SET retention_keep_last = ?, retention_keep_daily = ?, retention_keep_weekly = ?, retention_keep_monthly = ?
		 WHERE place_id = ?
		   AND (retention_keep_last <> ? OR retention_keep_daily <> ? OR retention_keep_weekly <> ? OR retention_keep_monthly <> ?)`,
		p.RetentionKeepLast, p.RetentionKeepDaily, p.RetentionKeepWeekly, p.RetentionKeepMonthly, p.ID,
		p.RetentionKeepLast, p.RetentionKeepDaily, p.RetentionKeepWeekly, p.RetentionKeepMonthly)
	if err != nil {
		return fmt.Errorf("copy the keep-policy of place %s onto its rows: %w", p.Name, err)
	}
	return nil
}
