package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Passkey is a registered WebAuthn credential. Only the public half is stored,
// and none of its columns is secret, so unlike the fleet token they are kept in
// the clear.
//
// A browser offers only passkeys whose relying-party ID matches the page, so
// the same box reached through two addresses has two separate sets.
type Passkey struct {
	ID   string
	Name string
	// CredentialID is the authenticator's handle for the key. It is unique, and
	// a login answer is looked up by it.
	CredentialID []byte
	// PublicKey is the COSE-encoded public half.
	PublicKey []byte
	// AAGUID identifies the authenticator model (a YubiKey 5, iCloud Keychain,
	// Windows Hello), so the list can show what kind of key it is.
	AAGUID []byte
	// SignCount is the authenticator's counter, updated on every successful
	// login. A counter that goes backwards signals a cloned authenticator; many
	// modern ones always report 0 and are exempt.
	SignCount uint32
	// Transports is the comma-joined hint list the authenticator reported
	// ("internal", "usb", "hybrid"), passed back at login so the browser knows
	// which prompt to raise.
	Transports string
	// RPID is the domain the credential is bound to, so the login can offer
	// only the keys that work at the current address.
	RPID string
	// BackedUp reports whether the authenticator says the key is synced to a
	// cloud keychain. A key that is not backed up is lost with the device.
	BackedUp   bool
	CreatedAt  int64
	LastUsedAt int64
}

const passkeyCols = `id, name, credential_id, public_key, aaguid, sign_count, transports, rp_id, backed_up, created_at, last_used_at` //nolint:gosec // G101: a SQL column list, and none of these columns holds a secret

func scanPasskey(s scanner) (Passkey, error) {
	var p Passkey
	err := s.Scan(&p.ID, &p.Name, &p.CredentialID, &p.PublicKey, &p.AAGUID,
		&p.SignCount, &p.Transports, &p.RPID, &p.BackedUp, &p.CreatedAt, &p.LastUsedAt)
	if err != nil {
		return Passkey{}, err
	}
	return p, nil
}

// ListPasskeys returns every registered credential, newest last.
func (r *Repo) ListPasskeys() ([]Passkey, error) {
	rows, err := r.db.Query(`SELECT ` + passkeyCols + ` FROM passkeys ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("ListPasskeys: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []Passkey
	for rows.Next() {
		p, sErr := scanPasskey(rows)
		if sErr != nil {
			return nil, fmt.Errorf("ListPasskeys: %w", sErr)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PasskeysForRP returns the credentials bound to the relying-party ID rpID, the
// only ones a login at that address can use.
func (r *Repo) PasskeysForRP(rpID string) ([]Passkey, error) {
	all, err := r.ListPasskeys()
	if err != nil {
		return nil, err
	}
	out := make([]Passkey, 0, len(all))
	for _, p := range all {
		if p.RPID == rpID {
			out = append(out, p)
		}
	}
	return out, nil
}

// PasskeyByCredentialID finds the credential an authenticator's answer names,
// or reports false when it is not registered.
func (r *Repo) PasskeyByCredentialID(credID []byte) (Passkey, bool, error) {
	row := r.db.QueryRow(`SELECT `+passkeyCols+` FROM passkeys WHERE credential_id = ?`, credID)
	p, err := scanPasskey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Passkey{}, false, nil
	}
	if err != nil {
		return Passkey{}, false, fmt.Errorf("PasskeyByCredentialID: %w", err)
	}
	return p, true, nil
}

// ErrPasskeyExists is returned when the same authenticator is registered twice.
var ErrPasskeyExists = errors.New("this passkey is already registered")

// AddPasskey stores a freshly registered credential and returns it with its id
// and timestamp filled in.
func (r *Repo) AddPasskey(p Passkey) (Passkey, error) {
	if len(p.CredentialID) == 0 || len(p.PublicKey) == 0 {
		return Passkey{}, errors.New("AddPasskey: a credential needs an id and a public key")
	}
	if p.ID == "" {
		p.ID = newID()
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = time.Now().Unix()
	}
	// Some security keys report no AAGUID, and a nil slice would bind as NULL,
	// which the column rejects.
	if p.AAGUID == nil {
		p.AAGUID = []byte{}
	}
	_, err := r.db.Exec(`INSERT INTO passkeys (`+passkeyCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.CredentialID, p.PublicKey, p.AAGUID,
		p.SignCount, p.Transports, p.RPID, p.BackedUp, p.CreatedAt, p.LastUsedAt)
	if err != nil {
		// credential_id is unique, so a failed insert for a known credential
		// means the authenticator is already registered.
		if _, ok, gErr := r.PasskeyByCredentialID(p.CredentialID); gErr == nil && ok {
			return Passkey{}, ErrPasskeyExists
		}
		return Passkey{}, fmt.Errorf("AddPasskey: %w", err)
	}
	return p, nil
}

// TouchPasskey records a successful login: the authenticator's new sign counter
// and when it was last used.
func (r *Repo) TouchPasskey(id string, signCount uint32, at int64) error {
	if _, err := r.db.Exec(`UPDATE passkeys SET sign_count = ?, last_used_at = ? WHERE id = ?`, signCount, at, id); err != nil {
		return fmt.Errorf("TouchPasskey: %w", err)
	}
	return nil
}

// RenamePasskey changes a credential's label.
func (r *Repo) RenamePasskey(id, name string) error {
	res, err := r.db.Exec(`UPDATE passkeys SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("RenamePasskey: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("RenamePasskey: no passkey %q", id)
	}
	return nil
}

// DeletePasskey removes a credential. Deleting a missing one is not an error.
func (r *Repo) DeletePasskey(id string) error {
	if _, err := r.db.Exec(`DELETE FROM passkeys WHERE id = ?`, id); err != nil {
		return fmt.Errorf("DeletePasskey: %w", err)
	}
	return nil
}
