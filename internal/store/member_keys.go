package store

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrMemberKey is returned for a key that is not the one pinned for the
	// member.
	ErrMemberKey = errors.New("the call is not signed with the key this instance knows that member by")
	// ErrMemberCounter is returned for a signed call that is not newer than
	// the last one from the member.
	ErrMemberCounter = errors.New("the call is not newer than the last one from that member")
)

// MemberKey returns the key pinned for a member, found false before the first.
func (r *Repo) MemberKey(memberID string) (key []byte, found bool, err error) {
	err = r.db.QueryRow(`SELECT key FROM member_keys WHERE member_id = ?`, memberID).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("MemberKey: %w", err)
	}
	return key, true, nil
}

// LegacyMember reports whether a member may call unsigned: this install knew
// it before it first ran a release whose members sign, and it has not signed
// a call here since.
func (r *Repo) LegacyMember(memberID string) (bool, error) {
	var legacy bool
	if err := r.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM legacy_members WHERE member_id = ?)`, memberID).Scan(&legacy); err != nil {
		return false, fmt.Errorf("LegacyMember: %w", err)
	}
	return legacy, nil
}

// PinMemberKey pins key for a member that has none and returns ErrMemberKey
// when another one is pinned already. A member with a key signs from then on,
// even after the key is forgotten.
func (r *Repo) PinMemberKey(memberID string, key []byte) error {
	err := r.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO member_keys (member_id, key, pinned_at) VALUES (?, ?, ?) ON CONFLICT (member_id) DO NOTHING`,
			memberID, key, time.Now().Unix()); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM legacy_members WHERE member_id = ?`, memberID); err != nil {
			return err
		}
		var pinned []byte
		if err := tx.QueryRow(`SELECT key FROM member_keys WHERE member_id = ?`, memberID).Scan(&pinned); err != nil {
			return err
		}
		if !bytes.Equal(pinned, key) {
			return ErrMemberKey
		}
		return nil
	})
	if err != nil && !errors.Is(err, ErrMemberKey) {
		return fmt.Errorf("PinMemberKey: %w", err)
	}
	return err
}

// AdvanceMemberCounter records counter as the newest a signed call of the
// member carried, and returns ErrMemberCounter for one that is not newer.
func (r *Repo) AdvanceMemberCounter(memberID string, counter int64) error {
	res, err := r.db.Exec(`UPDATE member_keys SET counter = ? WHERE member_id = ? AND counter < ?`, counter, memberID, counter)
	if err != nil {
		return fmt.Errorf("AdvanceMemberCounter: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrMemberCounter
	}
	return nil
}

// ForgetMemberKey drops the key pinned for a member, so that the next signed
// call from it pins the key the member names itself. A member that comes back
// after this has to sign.
func (r *Repo) ForgetMemberKey(memberID string) error {
	err := r.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM member_keys WHERE member_id = ?`, memberID); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM legacy_members WHERE member_id = ?`, memberID)
		return err
	})
	if err != nil {
		return fmt.Errorf("ForgetMemberKey: %w", err)
	}
	return nil
}

// ForgetMemberKeys drops every pinned key and every old member's leave to
// call unsigned, as leaving the group does.
func (r *Repo) ForgetMemberKeys() error {
	err := r.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM member_keys`); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM legacy_members`)
		return err
	})
	if err != nil {
		return fmt.Errorf("ForgetMemberKeys: %w", err)
	}
	return nil
}
