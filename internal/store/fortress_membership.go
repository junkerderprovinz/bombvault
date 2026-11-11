package store

import "fmt"

// FortressMembership is this install's place in a fortress. The secret behind
// the phrase, the member key and what the kit keeps are sealed under the
// APP_KEY.
type FortressMembership struct {
	SecretEnc []byte
	// PreviousEnc is the secret before the phrase last changed, which carries
	// the members and permissions over to the new one.
	PreviousEnc []byte
	SeedEnc     []byte
	StateEnc    []byte
	// Device is the name the other members know this server by, empty for
	// the one BombVault finds itself. It stays when the install leaves.
	Device string
}

// Joined reports whether the install is in a fortress.
func (m FortressMembership) Joined() bool { return len(m.SecretEnc) > 0 }

// FortressMembership reads the single membership row.
func (r *Repo) FortressMembership() (FortressMembership, error) {
	var m FortressMembership
	err := r.db.QueryRow(`SELECT secret_enc, previous_enc, seed_enc, state_enc, device FROM fortress_membership WHERE id = 1`).
		Scan(&m.SecretEnc, &m.PreviousEnc, &m.SeedEnc, &m.StateEnc, &m.Device)
	if err != nil {
		return FortressMembership{}, fmt.Errorf("FortressMembership: %w", err)
	}
	return m, nil
}

// JoinFortress keeps a new membership with its sealed secret and member key.
// Whatever the kit kept of an earlier one goes.
func (r *Repo) JoinFortress(secretEnc, seedEnc []byte) error {
	if _, err := r.db.Exec(`UPDATE fortress_membership SET secret_enc = ?, previous_enc = x'', seed_enc = ?, state_enc = x'' WHERE id = 1`, secretEnc, seedEnc); err != nil {
		return fmt.Errorf("JoinFortress: %w", err)
	}
	return nil
}

// SetFortressSecret keeps the secret of a new phrase and the one it replaces.
func (r *Repo) SetFortressSecret(secretEnc, previousEnc []byte) error {
	if _, err := r.db.Exec(`UPDATE fortress_membership SET secret_enc = ?, previous_enc = ? WHERE id = 1`, secretEnc, previousEnc); err != nil {
		return fmt.Errorf("SetFortressSecret: %w", err)
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
	if _, err := r.db.Exec(`UPDATE fortress_membership SET secret_enc = x'', previous_enc = x'', seed_enc = x'', state_enc = x'' WHERE id = 1`); err != nil {
		return fmt.Errorf("LeaveFortress: %w", err)
	}
	return nil
}
