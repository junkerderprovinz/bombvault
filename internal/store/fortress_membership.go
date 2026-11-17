package store

import (
	"fmt"
	"time"
)

// FortressMembership is this install's place in a fortress. The secret behind
// the phrase, the member key and what the kit keeps are sealed under the
// APP_KEY.
type FortressMembership struct {
	SecretEnc []byte
	// PreviousEnc is the secret before the phrase last changed, which carries
	// the members and permissions over to the new one, and Changed is when
	// that was.
	PreviousEnc []byte
	Changed     time.Time
	SeedEnc     []byte
	StateEnc    []byte
	// PairingEnc is what the kit keeps of the group's pairing once it moved
	// into the fortress.
	PairingEnc []byte
	// RemovedEnc lists the members the install took out since the phrase
	// last changed, sealed like the rest.
	RemovedEnc []byte
	// RemovedHere says another member took the install out. It stays until
	// the install joins again.
	RemovedHere bool
}

// Joined reports whether the install is in a fortress.
func (m FortressMembership) Joined() bool { return len(m.SecretEnc) > 0 }

// SeenBeforeChange reports whether a member last seen at seen was out of
// reach when the phrase last changed, so it may still hold the old one.
// Changed is kept to the second, so the whole of that second counts.
func (m FortressMembership) SeenBeforeChange(seen time.Time) bool {
	return len(m.PreviousEnc) > 0 && seen.Unix() <= m.Changed.Unix()
}

// FortressMembership reads the single membership row.
func (r *Repo) FortressMembership() (FortressMembership, error) {
	var m FortressMembership
	var changed int64
	err := r.db.QueryRow(`SELECT secret_enc, previous_enc, changed_at, seed_enc, state_enc, pairing_enc, removed_enc, removed_here FROM fortress_membership WHERE id = 1`).
		Scan(&m.SecretEnc, &m.PreviousEnc, &changed, &m.SeedEnc, &m.StateEnc, &m.PairingEnc, &m.RemovedEnc, &m.RemovedHere)
	if err != nil {
		return FortressMembership{}, fmt.Errorf("FortressMembership: %w", err)
	}
	if changed != 0 {
		m.Changed = time.Unix(changed, 0)
	}
	return m, nil
}

// JoinFortress keeps a new membership with its sealed secret and member key.
// Whatever the kit kept of an earlier one goes.
func (r *Repo) JoinFortress(secretEnc, seedEnc []byte) error {
	if _, err := r.db.Exec(`UPDATE fortress_membership SET secret_enc = ?, previous_enc = x'', changed_at = 0, seed_enc = ?, state_enc = x'', pairing_enc = x'', removed_enc = x'', removed_here = 0 WHERE id = 1`, secretEnc, seedEnc); err != nil {
		return fmt.Errorf("JoinFortress: %w", err)
	}
	return nil
}

// SetFortressSecret keeps the secret of a new phrase, the one it replaces and
// when it changed.
func (r *Repo) SetFortressSecret(secretEnc, previousEnc []byte, changed time.Time) error {
	if _, err := r.db.Exec(`UPDATE fortress_membership SET secret_enc = ?, previous_enc = ?, changed_at = ? WHERE id = 1`, secretEnc, previousEnc, changed.Unix()); err != nil {
		return fmt.Errorf("SetFortressSecret: %w", err)
	}
	return nil
}

// ForgetFortressPrevious drops the secret of the phrase before the last change.
func (r *Repo) ForgetFortressPrevious() error {
	if _, err := r.db.Exec(`UPDATE fortress_membership SET previous_enc = x'', changed_at = 0 WHERE id = 1`); err != nil {
		return fmt.Errorf("ForgetFortressPrevious: %w", err)
	}
	return nil
}

// SetFortressState keeps what the kit writes.
func (r *Repo) SetFortressState(enc []byte) error {
	if enc == nil {
		enc = []byte{}
	}
	if _, err := r.db.Exec(`UPDATE fortress_membership SET state_enc = ? WHERE id = 1`, enc); err != nil {
		return fmt.Errorf("SetFortressState: %w", err)
	}
	return nil
}

// SetFortressPairing keeps what the kit writes about the pairing in the
// fortress.
func (r *Repo) SetFortressPairing(enc []byte) error {
	if enc == nil {
		enc = []byte{}
	}
	if _, err := r.db.Exec(`UPDATE fortress_membership SET pairing_enc = ? WHERE id = 1`, enc); err != nil {
		return fmt.Errorf("SetFortressPairing: %w", err)
	}
	return nil
}

// SetFortressRemoved keeps the sealed list of the members the install took
// out since the phrase last changed.
func (r *Repo) SetFortressRemoved(enc []byte) error {
	if enc == nil {
		enc = []byte{}
	}
	if _, err := r.db.Exec(`UPDATE fortress_membership SET removed_enc = ? WHERE id = 1`, enc); err != nil {
		return fmt.Errorf("SetFortressRemoved: %w", err)
	}
	return nil
}

// SetFortressRemovedHere keeps whether another member took the install out.
func (r *Repo) SetFortressRemovedHere(on bool) error {
	if _, err := r.db.Exec(`UPDATE fortress_membership SET removed_here = ? WHERE id = 1`, on); err != nil {
		return fmt.Errorf("SetFortressRemovedHere: %w", err)
	}
	return nil
}

// RestoreFortress writes back a membership read earlier, as it was then.
func (r *Repo) RestoreFortress(m FortressMembership) error {
	var changed int64
	if !m.Changed.IsZero() {
		changed = m.Changed.Unix()
	}
	blob := func(b []byte) []byte {
		if b == nil {
			return []byte{}
		}
		return b
	}
	if _, err := r.db.Exec(`UPDATE fortress_membership SET secret_enc = ?, previous_enc = ?, changed_at = ?, seed_enc = ?, state_enc = ?, pairing_enc = ?, removed_enc = ?, removed_here = ? WHERE id = 1`,
		blob(m.SecretEnc), blob(m.PreviousEnc), changed, blob(m.SeedEnc), blob(m.StateEnc), blob(m.PairingEnc), blob(m.RemovedEnc), m.RemovedHere); err != nil {
		return fmt.Errorf("RestoreFortress: %w", err)
	}
	return nil
}

// LeaveFortress forgets the membership.
func (r *Repo) LeaveFortress() error {
	if _, err := r.db.Exec(`UPDATE fortress_membership SET secret_enc = x'', previous_enc = x'', changed_at = 0, seed_enc = x'', state_enc = x'', pairing_enc = x'', removed_enc = x'' WHERE id = 1`); err != nil {
		return fmt.Errorf("LeaveFortress: %w", err)
	}
	return nil
}
