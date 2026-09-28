package store

import (
	"fmt"
)

// GroupState is this instance's side of the pairing group: its own id, the
// group secret and how it reaches members it cannot reach directly.
type GroupState struct {
	// InstanceID addresses this instance within the group. It is minted by
	// the migration and never changes.
	InstanceID string
	// SecretEnc is the group secret sealed under the APP_KEY
	// (internal/secret). Empty means this instance is in no group.
	SecretEnc []byte
	// RelayMode is "project", "own" or "off".
	RelayMode string
	// RelayURL is the own relay's address, kept when another mode is picked
	// so switching back does not lose it.
	RelayURL string
	// RelayServe makes this instance a relay for its own group under
	// /relay/connect.
	RelayServe bool
}

// GetGroupState reads the single group row.
func (r *Repo) GetGroupState() (GroupState, error) {
	var g GroupState
	var serve int
	err := r.db.QueryRow(`SELECT instance_id, secret_enc, relay_mode, relay_url, relay_serve FROM group_state WHERE id = 1`).
		Scan(&g.InstanceID, &g.SecretEnc, &g.RelayMode, &g.RelayURL, &serve)
	if err != nil {
		return GroupState{}, fmt.Errorf("GetGroupState: %w", err)
	}
	g.RelayServe = serve != 0
	return g, nil
}

// SetGroupSecret stores the sealed group secret; an empty value leaves the
// group.
func (r *Repo) SetGroupSecret(secretEnc []byte) error {
	if secretEnc == nil {
		secretEnc = []byte{}
	}
	if _, err := r.db.Exec(`UPDATE group_state SET secret_enc = ? WHERE id = 1`, secretEnc); err != nil {
		return fmt.Errorf("SetGroupSecret: %w", err)
	}
	return nil
}

// SetGroupRelay stores how this instance uses and serves a relay.
func (r *Repo) SetGroupRelay(mode, url string, serve bool) error {
	if _, err := r.db.Exec(`UPDATE group_state SET relay_mode = ?, relay_url = ?, relay_serve = ? WHERE id = 1`,
		mode, url, boolInt(serve)); err != nil {
		return fmt.Errorf("SetGroupRelay: %w", err)
	}
	return nil
}
