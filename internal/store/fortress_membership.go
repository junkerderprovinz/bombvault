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
	// Device is the name the other members know this server by, empty for
	// the one BombVault finds itself. It stays when the install leaves.
	Device string
}

// Joined reports whether the install is in a fortress.
func (m FortressMembership) Joined() bool { return len(m.SecretEnc) > 0 }

// FortressMembership reads the single membership row.
func (r *Repo) FortressMembership() (FortressMembership, error) {
	var m FortressMembership
	var changed int64
	err := r.db.QueryRow(`SELECT secret_enc, previous_enc, changed_at, seed_enc, state_enc, pairing_enc, device FROM fortress_membership WHERE id = 1`).
		Scan(&m.SecretEnc, &m.PreviousEnc, &changed, &m.SeedEnc, &m.StateEnc, &m.PairingEnc, &m.Device)
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
	if _, err := r.db.Exec(`UPDATE fortress_membership SET secret_enc = ?, previous_enc = x'', changed_at = 0, seed_enc = ?, state_enc = x'', pairing_enc = x'' WHERE id = 1`, secretEnc, seedEnc); err != nil {
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

// SetFortressDevice names this server for the other members; "" goes back to
// the name BombVault finds itself.
func (r *Repo) SetFortressDevice(device string) error {
	if _, err := r.db.Exec(`UPDATE fortress_membership SET device = ? WHERE id = 1`, device); err != nil {
		return fmt.Errorf("SetFortressDevice: %w", err)
	}
	return nil
}

// LeaveFortress forgets the membership and keeps the name of the server.
func (r *Repo) LeaveFortress() error {
	if _, err := r.db.Exec(`UPDATE fortress_membership SET secret_enc = x'', previous_enc = x'', changed_at = 0, seed_enc = x'', state_enc = x'', pairing_enc = x'' WHERE id = 1`); err != nil {
		return fmt.Errorf("LeaveFortress: %w", err)
	}
	return nil
}
