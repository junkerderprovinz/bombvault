package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// SignInTable keeps the documents of the kit's sign-in.
const SignInTable = "signin_docs"

// signInDocsTable is the table the kit's sqlstore expects.
const signInDocsTable = `CREATE TABLE IF NOT EXISTS ` + SignInTable + ` (
	kind TEXT NOT NULL,
	id   TEXT NOT NULL,
	data BLOB NOT NULL,
	PRIMARY KEY (kind, id)
);`

// DB is the database the repo works on, for the sign-in's store.
func (r *Repo) DB() *sql.DB { return r.db }

// PublicAddress is where people reach this instance, such as
// https://backup.example, or empty while nobody set one.
func (r *Repo) PublicAddress() (string, error) {
	var url string
	err := r.db.QueryRow(`SELECT url FROM public_address WHERE id = 1`).Scan(&url)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("PublicAddress: %w", err)
	}
	return url, nil
}

// SetPublicAddress stores where people reach this instance. Empty forgets it.
func (r *Repo) SetPublicAddress(url string) error {
	_, err := r.db.Exec(`INSERT INTO public_address (id, url) VALUES (1, ?)
		ON CONFLICT (id) DO UPDATE SET url = excluded.url`, url)
	if err != nil {
		return fmt.Errorf("SetPublicAddress: %w", err)
	}
	return nil
}

// OldLoginMarker stands in the password hash column once the sign-in keeps
// the password. It verifies no password, so the column still says that one
// is set: a settings restore and a sign-in that has to start over after
// APP_KEY changed keep the instance closed until the reset through the log.
const OldLoginMarker = "signin"

// DropOldLogin clears the login of the version before the kit: the second
// factor, the passkeys table and the password hash, which becomes
// OldLoginMarker while a password is set and empty while none is. The
// session epoch stays, since the sessions of paired phones are signed with
// it.
func (r *Repo) DropOldLogin(password bool) error {
	hash := ""
	if password {
		hash = OldLoginMarker
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("DropOldLogin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit
	if _, err := tx.Exec(`UPDATE settings SET auth_password_hash = ?, totp_enabled = 0, totp_secret = '', totp_recovery = '' WHERE id = 1`, hash); err != nil {
		return fmt.Errorf("DropOldLogin: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM passkeys`); err != nil {
		return fmt.Errorf("DropOldLogin: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("DropOldLogin: %w", err)
	}
	return nil
}

// MarkOldLogin keeps the password hash column in step with whether the
// sign-in has a password: OldLoginMarker in place of an empty column, and
// empty once the password went. A hash the version before kept stays until
// DropOldLogin.
func (r *Repo) MarkOldLogin(password bool) error {
	q := `UPDATE settings SET auth_password_hash = ? WHERE id = 1 AND auth_password_hash = ''`
	arg := OldLoginMarker
	if !password {
		q, arg = `UPDATE settings SET auth_password_hash = '' WHERE id = 1 AND auth_password_hash = ?`, OldLoginMarker
	}
	if _, err := r.db.Exec(q, arg); err != nil {
		return fmt.Errorf("MarkOldLogin: %w", err)
	}
	return nil
}

// ForgetSignIn deletes every document of the sign-in, so its next start
// imports the login anew.
func (r *Repo) ForgetSignIn() error {
	if _, err := r.db.Exec(`DELETE FROM ` + SignInTable); err != nil {
		return fmt.Errorf("ForgetSignIn: %w", err)
	}
	return nil
}

// EndSignIns ends every session and removes every passkey and API token of
// the sign-in, for a configuration restored from a backup that may bring
// back what was ended after it was saved. The accounts and their passwords
// stay.
func (r *Repo) EndSignIns() (passkeys int64, err error) {
	if err := r.db.QueryRow(`SELECT count(*) FROM ` + SignInTable + ` WHERE kind = 'passkey'`).Scan(&passkeys); err != nil {
		return 0, fmt.Errorf("EndSignIns: %w", err)
	}
	if _, err := r.db.Exec(`DELETE FROM ` + SignInTable + ` WHERE kind IN ('session', 'swapped', 'passkey', 'token')`); err != nil {
		return 0, fmt.Errorf("EndSignIns: %w", err)
	}
	return passkeys, nil
}
