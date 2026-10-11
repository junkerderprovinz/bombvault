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

// PinMemberKey pins key for a member that has none and returns ErrMemberKey
// when another one is pinned already.
func (r *Repo) PinMemberKey(memberID string, key []byte) error {
	err := r.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO member_keys (member_id, key, pinned_at) VALUES (?, ?, ?) ON CONFLICT (member_id) DO NOTHING`,
			memberID, key, time.Now().Unix()); err != nil {
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
