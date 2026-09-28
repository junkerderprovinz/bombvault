package store

import (
	"fmt"
	"time"
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
	// JoinedAt is when this instance created or joined its group, zero
	// outside one.
	JoinedAt time.Time
	// MemberSeenAt is when another member first showed up after JoinedAt,
	// zero until one has.
	MemberSeenAt time.Time
}

// GetGroupState reads the single group row.
func (r *Repo) GetGroupState() (GroupState, error) {
	var g GroupState
	var serve int
	var joined, seen int64
	err := r.db.QueryRow(`SELECT instance_id, secret_enc, relay_mode, relay_url, relay_serve, joined_at, member_seen_at FROM group_state WHERE id = 1`).
		Scan(&g.InstanceID, &g.SecretEnc, &g.RelayMode, &g.RelayURL, &serve, &joined, &seen)
	if err != nil {
		return GroupState{}, fmt.Errorf("GetGroupState: %w", err)
	}
	g.RelayServe = serve != 0
	g.JoinedAt = unixOrZero(joined)
	g.MemberSeenAt = unixOrZero(seen)
	return g, nil
}

func unixOrZero(sec int64) time.Time {
	if sec == 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

// SetGroupSecret stores the sealed group secret as entered at the given
// time; an empty value leaves the group. Either way no member has been seen
// in the group that follows.
func (r *Repo) SetGroupSecret(secretEnc []byte, at time.Time) error {
	joined := int64(0)
	if len(secretEnc) == 0 {
		secretEnc = []byte{}
	} else {
		joined = at.Unix()
	}
	if _, err := r.db.Exec(`UPDATE group_state SET secret_enc = ?, joined_at = ?, member_seen_at = 0 WHERE id = 1`, secretEnc, joined); err != nil {
		return fmt.Errorf("SetGroupSecret: %w", err)
	}
	return nil
}

// MarkGroupMemberSeen records the first time another member shows up in the
// current group. Later calls, and calls outside a group, change nothing.
func (r *Repo) MarkGroupMemberSeen(at time.Time) error {
	if _, err := r.db.Exec(`UPDATE group_state SET member_seen_at = ? WHERE id = 1 AND member_seen_at = 0 AND length(secret_enc) > 0`, at.Unix()); err != nil {
		return fmt.Errorf("MarkGroupMemberSeen: %w", err)
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
