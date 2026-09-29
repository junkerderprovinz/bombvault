package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// FleetPeer is another BombVault instance whose protection status this one
// shows on the Fleet page. The LastPoll fields cache the peer's latest answer
// so the page renders without a live round trip.
type FleetPeer struct {
	ID string
	// MemberID is the peer's instance id in the pairing group. It is empty on
	// a row from before pairing, which has to be paired again to be polled.
	MemberID string
	Name     string
	// URL is where a peer from before pairing was polled. It is only shown.
	URL     string
	Enabled bool
	// LastPollAt is the Unix time of the last poll attempt (0 = never polled).
	LastPollAt int64
	// LastPollOK is the last poll's verdict. Valid=false means never polled.
	LastPollOK sql.NullBool
	// LastPollError is the last poll's scrubbed error ('' on success/never).
	LastPollError string
	// LastPollInstanceName is the name the peer reported about itself.
	LastPollInstanceName string
	// LastPollVersion is the peer's reported BombVault version.
	LastPollVersion string
	// LastPollDomainsJSON is the peer's DomainStatusEntry[] response as
	// received. The store treats it as an opaque string.
	LastPollDomainsJSON string
	CreatedAt           int64
	SortOrder           int
}

// NeedsPairing reports whether the row predates pairing and has no member to
// poll.
func (p FleetPeer) NeedsPairing() bool { return p.MemberID == "" }

const fleetPeerCols = `id, member_id, name, url, enabled, last_poll_at, last_poll_ok, last_poll_error,
	last_poll_instance_name, last_poll_version, last_poll_domains_json, created_at, sort_order`

// CreateFleetPeer inserts a new fleet peer and returns the stored row. An
// empty ID is assigned and a zero CreatedAt is set to now.
func (r *Repo) CreateFleetPeer(p FleetPeer) (FleetPeer, error) {
	if p.ID == "" {
		p.ID = newID()
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = time.Now().Unix()
	}
	_, err := r.db.Exec(`
		INSERT INTO fleet_peers (`+fleetPeerCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.MemberID, p.Name, p.URL, boolInt(p.Enabled), p.LastPollAt, nullBool(p.LastPollOK), p.LastPollError,
		p.LastPollInstanceName, p.LastPollVersion, p.LastPollDomainsJSON, p.CreatedAt, p.SortOrder,
	)
	if err != nil {
		return FleetPeer{}, fmt.Errorf("CreateFleetPeer: %w", err)
	}
	return p, nil
}

// UpdateFleetPeer updates the editable columns of the fleet peer p.ID.
// Updating a missing id affects no rows and is not an error.
func (r *Repo) UpdateFleetPeer(p FleetPeer) error {
	_, err := r.db.Exec(`
		UPDATE fleet_peers SET
		  member_id  = ?,
		  name       = ?,
		  url        = ?,
		  enabled    = ?,
		  sort_order = ?
		WHERE id = ?`,
		p.MemberID, p.Name, p.URL, boolInt(p.Enabled), p.SortOrder, p.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateFleetPeer: %w", err)
	}
	return nil
}

// UpdateFleetPeerPollResult writes only the last-poll columns of the peer with
// the given id, so the scheduled poll and the poll-now endpoint can store a
// result without rewriting the whole row. Updating a missing id affects no
// rows and is not an error.
func (r *Repo) UpdateFleetPeerPollResult(id string, at int64, ok sql.NullBool, pollErr, instanceName, version, domainsJSON string) error {
	_, err := r.db.Exec(`
		UPDATE fleet_peers SET
		  last_poll_at            = ?,
		  last_poll_ok            = ?,
		  last_poll_error         = ?,
		  last_poll_instance_name = ?,
		  last_poll_version       = ?,
		  last_poll_domains_json  = ?
		WHERE id = ?`,
		at, nullBool(ok), pollErr, instanceName, version, domainsJSON, id,
	)
	if err != nil {
		return fmt.Errorf("UpdateFleetPeerPollResult: %w", err)
	}
	return nil
}

// ListFleetPeers returns all fleet peers ordered by sort_order, then
// created_at.
func (r *Repo) ListFleetPeers() ([]FleetPeer, error) {
	rows, err := r.db.Query(`SELECT ` + fleetPeerCols + ` FROM fleet_peers ORDER BY sort_order, created_at`)
	if err != nil {
		return nil, fmt.Errorf("ListFleetPeers: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []FleetPeer
	for rows.Next() {
		p, err := scanFleetPeer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetFleetPeer returns the fleet peer with the given id. The bool is false
// (with a zero FleetPeer) when no such row exists.
func (r *Repo) GetFleetPeer(id string) (FleetPeer, bool, error) {
	row := r.db.QueryRow(`SELECT `+fleetPeerCols+` FROM fleet_peers WHERE id = ?`, id)
	p, err := scanFleetPeer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return FleetPeer{}, false, nil
	}
	if err != nil {
		return FleetPeer{}, false, err
	}
	return p, true, nil
}

// DeleteFleetPeer removes the fleet peer with the given id. It is a no-op (no
// error) if the row does not exist.
func (r *Repo) DeleteFleetPeer(id string) error {
	if _, err := r.db.Exec(`DELETE FROM fleet_peers WHERE id = ?`, id); err != nil {
		return fmt.Errorf("DeleteFleetPeer: %w", err)
	}
	return nil
}

func scanFleetPeer(s scanner) (FleetPeer, error) {
	var p FleetPeer
	var enabled int
	err := s.Scan(
		&p.ID, &p.MemberID, &p.Name, &p.URL, &enabled, &p.LastPollAt, &p.LastPollOK, &p.LastPollError,
		&p.LastPollInstanceName, &p.LastPollVersion, &p.LastPollDomainsJSON, &p.CreatedAt, &p.SortOrder,
	)
	if err != nil {
		return FleetPeer{}, fmt.Errorf("scanFleetPeer: %w", err)
	}
	p.Enabled = enabled != 0
	return p, nil
}
