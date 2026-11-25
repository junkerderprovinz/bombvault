package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// SetPhoneAccount records that account let the member in, which a phone's
// sessions then answer to. Letting it in again starts it afresh.
func (r *Repo) SetPhoneAccount(memberID, account string) error {
	if _, err := r.db.Exec(`INSERT INTO phone_accounts (member_id, account, ended_at) VALUES (?, ?, 0)
		ON CONFLICT (member_id) DO UPDATE SET account = excluded.account, ended_at = 0`, memberID, account); err != nil {
		return fmt.Errorf("SetPhoneAccount: %w", err)
	}
	return nil
}

// PhoneAccount returns the account that let memberID in, empty for one let in
// before anybody was recorded, and whether that account's end ended it too.
func (r *Repo) PhoneAccount(memberID string) (account string, ended bool, err error) {
	var endedAt int64
	err = r.db.QueryRow(`SELECT account, ended_at FROM phone_accounts WHERE member_id = ?`, memberID).Scan(&account, &endedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("PhoneAccount: %w", err)
	}
	return account, endedAt != 0, nil
}

// EndPhonesOf ends every phone account let in.
func (r *Repo) EndPhonesOf(account string, now int64) error {
	if _, err := r.db.Exec(`UPDATE phone_accounts SET ended_at = ? WHERE account = ? AND ended_at = 0`, now, account); err != nil {
		return fmt.Errorf("EndPhonesOf: %w", err)
	}
	return nil
}
