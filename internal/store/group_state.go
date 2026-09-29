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
	// DirectURL is this instance's own address on the local network: where
	// members should send it direct calls. It starts out learned from the
	// browser's request (scheme, host and port) the first time someone signs
	// in, and DirectURLManual is set once a person edits it by hand, after
	// which learning a new one from the browser stops overwriting it.
	DirectURL       string
	DirectURLManual bool
}

// GetGroupState reads the single group row.
func (r *Repo) GetGroupState() (GroupState, error) {
	var g GroupState
	var serve, manual int
	var joined, seen int64
	err := r.db.QueryRow(`SELECT instance_id, secret_enc, relay_mode, relay_url, relay_serve, joined_at, member_seen_at, direct_url, direct_url_manual FROM group_state WHERE id = 1`).
		Scan(&g.InstanceID, &g.SecretEnc, &g.RelayMode, &g.RelayURL, &serve, &joined, &seen, &g.DirectURL, &manual)
	if err != nil {
		return GroupState{}, fmt.Errorf("GetGroupState: %w", err)
	}
	g.RelayServe = serve != 0
	g.JoinedAt = unixOrZero(joined)
	g.MemberSeenAt = unixOrZero(seen)
	g.DirectURLManual = manual != 0
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

// SetGroupDirectURL stores this instance's own direct address. manual marks
// it as set by a person rather than learned from a request, which stops
// LearnGroupDirectURL from replacing it.
func (r *Repo) SetGroupDirectURL(url string, manual bool) error {
	if _, err := r.db.Exec(`UPDATE group_state SET direct_url = ?, direct_url_manual = ? WHERE id = 1`,
		url, boolInt(manual)); err != nil {
		return fmt.Errorf("SetGroupDirectURL: %w", err)
	}
	return nil
}

// LearnGroupDirectURL stores url as this instance's own direct address, the
// way SetGroupDirectURL does, but only while nobody has set one by hand: a
// person's own edit is never quietly replaced by what the next request's
// Host header happens to say.
func (r *Repo) LearnGroupDirectURL(url string) error {
	if _, err := r.db.Exec(`UPDATE group_state SET direct_url = ? WHERE id = 1 AND direct_url_manual = 0 AND direct_url != ?`,
		url, url); err != nil {
		return fmt.Errorf("LearnGroupDirectURL: %w", err)
	}
	return nil
}
