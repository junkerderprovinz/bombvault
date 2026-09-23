package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrEmptyMeshOffer is returned for a mesh offer without a repo location,
// which could never become a working off-site target.
var ErrEmptyMeshOffer = errors.New("mesh offer repo must not be empty")

// MeshOffer is a fleet peer's offer of its own off-site storage. The peer runs
// a rest-server on its own infrastructure and sends the connection details
// over the fleet channel; the admin here reviews the offer and, on accept,
// turns it into a named CloudCredSet and OffsiteTarget. BombVault does not host
// storage itself.
type MeshOffer struct {
	ID   string
	From string
	// SuggestedDomain is the peer's guess at which domain this storage suits.
	// The admin picks the actual domain when accepting.
	SuggestedDomain string
	Repo            string
	RESTUser        string
	// RESTPasswordEnc is the peer-generated one-time rest-server password,
	// encrypted with this instance's APP_KEY (internal/secret). It is never
	// logged or returned in the clear.
	RESTPasswordEnc []byte
	// Status is "pending" | "accepted" | "declined".
	Status     string
	ReceivedAt int64
	SortOrder  int
}

const meshOfferCols = `id, from_name, suggested_domain, repo, rest_user, rest_password_enc, status, received_at, sort_order`

// CreateMeshOffer inserts a new mesh offer and returns the stored row. An
// empty ID is assigned via newID(), a zero ReceivedAt is set to now and an
// empty Status becomes "pending". The repo must not be empty.
func (r *Repo) CreateMeshOffer(o MeshOffer) (MeshOffer, error) {
	if strings.TrimSpace(o.Repo) == "" {
		return MeshOffer{}, ErrEmptyMeshOffer
	}
	if o.ID == "" {
		o.ID = newID()
	}
	if o.ReceivedAt == 0 {
		o.ReceivedAt = time.Now().Unix()
	}
	if o.Status == "" {
		o.Status = "pending"
	}
	if o.RESTPasswordEnc == nil {
		o.RESTPasswordEnc = []byte{} // rest_password_enc is not nullable, and a nil slice binds as NULL
	}
	_, err := r.db.Exec(`
		INSERT INTO mesh_offers (`+meshOfferCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.ID, o.From, o.SuggestedDomain, o.Repo, o.RESTUser, o.RESTPasswordEnc, o.Status, o.ReceivedAt, o.SortOrder,
	)
	if err != nil {
		return MeshOffer{}, fmt.Errorf("CreateMeshOffer: %w", err)
	}
	return o, nil
}

// UpdateMeshOfferStatus writes only the status column of the offer with the
// given id. Updating a missing id affects no rows and is not an error.
func (r *Repo) UpdateMeshOfferStatus(id, status string) error {
	if _, err := r.db.Exec(`UPDATE mesh_offers SET status = ? WHERE id = ?`, status, id); err != nil {
		return fmt.Errorf("UpdateMeshOfferStatus: %w", err)
	}
	return nil
}

// ListMeshOffers returns all mesh offers ordered by sort_order, then
// received_at.
func (r *Repo) ListMeshOffers() ([]MeshOffer, error) {
	rows, err := r.db.Query(`SELECT ` + meshOfferCols + ` FROM mesh_offers ORDER BY sort_order, received_at`)
	if err != nil {
		return nil, fmt.Errorf("ListMeshOffers: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []MeshOffer
	for rows.Next() {
		o, err := scanMeshOffer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// GetMeshOffer returns the mesh offer with the given id. The bool is false
// (with a zero MeshOffer) when no such row exists.
func (r *Repo) GetMeshOffer(id string) (MeshOffer, bool, error) {
	row := r.db.QueryRow(`SELECT `+meshOfferCols+` FROM mesh_offers WHERE id = ?`, id)
	o, err := scanMeshOffer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return MeshOffer{}, false, nil
	}
	if err != nil {
		return MeshOffer{}, false, err
	}
	return o, true, nil
}

// DeleteMeshOffer removes the mesh offer with the given id. It is a no-op (no
// error) if the row does not exist.
func (r *Repo) DeleteMeshOffer(id string) error {
	if _, err := r.db.Exec(`DELETE FROM mesh_offers WHERE id = ?`, id); err != nil {
		return fmt.Errorf("DeleteMeshOffer: %w", err)
	}
	return nil
}

func scanMeshOffer(s scanner) (MeshOffer, error) {
	var o MeshOffer
	err := s.Scan(
		&o.ID, &o.From, &o.SuggestedDomain, &o.Repo, &o.RESTUser, &o.RESTPasswordEnc, &o.Status, &o.ReceivedAt, &o.SortOrder,
	)
	if err != nil {
		return MeshOffer{}, fmt.Errorf("scanMeshOffer: %w", err)
	}
	return o, nil
}
