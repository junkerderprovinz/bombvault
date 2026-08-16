package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrEmptyFleetPeer is returned when a fleet peer has no URL. A peer with an
// empty URL addresses nowhere and could never be polled, so it is rejected at
// the store boundary (mirrors ErrEmptyReceivedRepo).
var ErrEmptyFleetPeer = errors.New("fleet peer URL must not be empty")

// FleetPeer is another BombVault instance this box polls, read-only, for its
// protection status (the Fleet view). It carries the peer's base URL plus the
// PEER's own fleet_token — the credential this instance presents when it polls
// that peer's GET /api/fleet/status — stored ENCRYPTED at rest in TokenEnc
// (internal/secret) and only ever decrypted in-engine, never by the store and
// never logged. LastPoll* caches the peer's most recent response so the Fleet
// page can render without a live round-trip on every load.
type FleetPeer struct {
	ID   string
	Name string
	URL  string
	// TokenEnc is the PEER's fleet token, AES-256-GCM encrypted at rest via
	// internal/secret. The store only ever persists/returns the ciphertext; only
	// the poller decrypts it (with this instance's APP_KEY) to authenticate the
	// outbound request. Never logged, never returned in the clear.
	TokenEnc []byte
	Enabled  bool
	// LastPollAt is the Unix time of the last poll attempt (0 = never polled).
	LastPollAt int64
	// LastPollOK is the last poll's verdict. Valid=false means never polled yet
	// (the column is nullable).
	LastPollOK sql.NullBool
	// LastPollError is the last poll's scrubbed error ('' on success/never).
	LastPollError string
	// LastPollInstanceName is the name the peer reported about itself.
	LastPollInstanceName string
	// LastPollVersion is the peer's reported BombVault version.
	LastPollVersion string
	// LastPollDomainsJSON caches the peer's DomainStatusEntry[] response verbatim
	// (already-serialized JSON) — the store treats it as an opaque string.
	LastPollDomainsJSON string
	CreatedAt           int64
	SortOrder           int
}

const fleetPeerCols = `id, name, url, token_enc, enabled, last_poll_at, last_poll_ok, last_poll_error,
	last_poll_instance_name, last_poll_version, last_poll_domains_json, created_at, sort_order`

// CreateFleetPeer inserts a new fleet peer. An empty ID is assigned via
// newID(); CreatedAt is stamped now when 0. Returns the stored row (with the
// assigned id/timestamp). The peer URL must not be empty.
func (r *Repo) CreateFleetPeer(p FleetPeer) (FleetPeer, error) {
	if strings.TrimSpace(p.URL) == "" {
		return FleetPeer{}, ErrEmptyFleetPeer
	}
	if p.ID == "" {
		p.ID = newID()
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = time.Now().Unix()
	}
	if p.TokenEnc == nil {
		p.TokenEnc = []byte{} // NOT NULL blob: bind an empty blob, never SQL NULL
	}
	_, err := r.db.Exec(`
		INSERT INTO fleet_peers (`+fleetPeerCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.URL, p.TokenEnc, boolInt(p.Enabled), p.LastPollAt, nullBool(p.LastPollOK), p.LastPollError,
		p.LastPollInstanceName, p.LastPollVersion, p.LastPollDomainsJSON, p.CreatedAt, p.SortOrder,
	)
	if err != nil {
		return FleetPeer{}, fmt.Errorf("CreateFleetPeer: %w", err)
	}
	return p, nil
}

// UpdateFleetPeer updates the fleet peer identified by p.ID in place. The peer
// URL must not be empty. Updating a missing id affects no rows and is not an
// error (mirrors the received-repo/offsite conventions).
func (r *Repo) UpdateFleetPeer(p FleetPeer) error {
	if strings.TrimSpace(p.URL) == "" {
		return ErrEmptyFleetPeer
	}
	if p.TokenEnc == nil {
		p.TokenEnc = []byte{} // NOT NULL blob: bind an empty blob, never SQL NULL
	}
	_, err := r.db.Exec(`
		UPDATE fleet_peers SET
		  name       = ?,
		  url        = ?,
		  token_enc  = ?,
		  enabled    = ?,
		  sort_order = ?
		WHERE id = ?`,
		p.Name, p.URL, p.TokenEnc, boolInt(p.Enabled), p.SortOrder, p.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateFleetPeer: %w", err)
	}
	return nil
}

// UpdateFleetPeerPollResult writes ONLY the last-poll columns for the peer with
// the given id, leaving name/url/token_enc/enabled untouched. The scheduled
// fleet poll and the manual poll-now endpoint use it to persist a result
// without a read-modify-write of the whole row. Updating a missing id affects
// no rows and is not an error.
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

// ListFleetPeers returns all fleet peers ordered by sort_order then created_at
// (a stable display order).
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
		&p.ID, &p.Name, &p.URL, &p.TokenEnc, &enabled, &p.LastPollAt, &p.LastPollOK, &p.LastPollError,
		&p.LastPollInstanceName, &p.LastPollVersion, &p.LastPollDomainsJSON, &p.CreatedAt, &p.SortOrder,
	)
	if err != nil {
		return FleetPeer{}, fmt.Errorf("scanFleetPeer: %w", err)
	}
	p.Enabled = enabled != 0
	return p, nil
}
