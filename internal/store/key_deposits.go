package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// The states of a deposit, on the member that keeps it and on the install it
// belongs to.
const (
	// DepositHeld is a deposit the keeper holds.
	DepositHeld = "held"
	// DepositDeclined is a deposit the keeper deleted. It takes no new one
	// from that install until a person there asks again.
	DepositDeclined = "declined"
	// DepositStale is a deposit whose content changed on the install it
	// belongs to and has not reached the keeper since.
	DepositStale = "stale"
)

// The states of a request to hand a deposit out.
const (
	ReleaseAsked    = "asked"
	ReleaseAllowed  = "allowed"
	ReleaseDeclined = "declined"
	// ReleaseDone is a request whose deposit went to the instance that asked.
	ReleaseDone = "released"
	// ReleaseExpired is a request nobody answered, or one that was allowed and
	// not collected in time.
	ReleaseExpired = "expired"
)

// ErrReleaseLimit is returned when an instance asked for deposits too often.
var ErrReleaseLimit = errors.New("too many requests for a deposit from this instance, try again in an hour")

// KeptDeposit is a deposit this install keeps for a member. Deposit is sealed
// to a key only the member's recovery code gives, so nothing here reads it.
type KeptDeposit struct {
	OwnerID   string
	OwnerName string
	State     string
	Deposit   []byte
	Salt      []byte
	Version   int
	CreatedAt int64
	UpdatedAt int64
}

func scanKeptDeposit(s scanner) (KeptDeposit, error) {
	var d KeptDeposit
	err := s.Scan(&d.OwnerID, &d.OwnerName, &d.State, &d.Deposit, &d.Salt, &d.Version, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

const keptDepositCols = `owner_id, owner_name, state, deposit, salt, version, created_at, updated_at`

// KeepDeposit stores the deposit a member sent and returns the state it is in
// afterwards and whether it is the member's first. A deposit this install
// deleted stays declined unless renew says a person there asked again.
func (r *Repo) KeepDeposit(d KeptDeposit, renew bool) (state string, first bool, err error) {
	now := time.Now().Unix()
	err = r.inTx(func(tx *sql.Tx) error {
		var held string
		switch err := tx.QueryRow(`SELECT state FROM key_deposits WHERE owner_id = ?`, d.OwnerID).Scan(&held); {
		case errors.Is(err, sql.ErrNoRows):
			first = true
		case err != nil:
			return err
		case held == DepositDeclined && !renew:
			state = DepositDeclined
			return nil
		}
		state = DepositHeld
		_, err := tx.Exec(`
			INSERT INTO key_deposits (owner_id, owner_name, state, deposit, salt, version, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (owner_id) DO UPDATE SET owner_name = excluded.owner_name, state = excluded.state,
				deposit = excluded.deposit, salt = excluded.salt, version = excluded.version, updated_at = excluded.updated_at`,
			d.OwnerID, d.OwnerName, DepositHeld, d.Deposit, d.Salt, d.Version, now, now)
		return err
	})
	if err != nil {
		return "", false, fmt.Errorf("KeepDeposit: %w", err)
	}
	return state, first, nil
}

// DeclineDeposit deletes what this install keeps for a member and remembers
// that it did. It reports whether there was a deposit.
func (r *Repo) DeclineDeposit(ownerID string) (bool, error) {
	res, err := r.db.Exec(`UPDATE key_deposits SET state = ?, deposit = x'', salt = x'', updated_at = ? WHERE owner_id = ? AND state = ?`,
		DepositDeclined, time.Now().Unix(), ownerID, DepositHeld)
	if err != nil {
		return false, fmt.Errorf("DeclineDeposit: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DropDeposit forgets a member's deposit and that there ever was one, as the
// member asks for when it takes its deposit back.
func (r *Repo) DropDeposit(ownerID string) (bool, error) {
	res, err := r.db.Exec(`DELETE FROM key_deposits WHERE owner_id = ?`, ownerID)
	if err != nil {
		return false, fmt.Errorf("DropDeposit: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// KeptDeposit returns what this install keeps for a member.
func (r *Repo) KeptDeposit(ownerID string) (KeptDeposit, bool, error) {
	d, err := scanKeptDeposit(r.db.QueryRow(`SELECT `+keptDepositCols+` FROM key_deposits WHERE owner_id = ?`, ownerID))
	if errors.Is(err, sql.ErrNoRows) {
		return KeptDeposit{}, false, nil
	}
	if err != nil {
		return KeptDeposit{}, false, fmt.Errorf("KeptDeposit: %w", err)
	}
	return d, true, nil
}

// KeptDeposits lists what this install keeps or declined to keep, by member
// name.
func (r *Repo) KeptDeposits() ([]KeptDeposit, error) {
	rows, err := r.db.Query(`SELECT ` + keptDepositCols + ` FROM key_deposits ORDER BY owner_name, owner_id`)
	if err != nil {
		return nil, fmt.Errorf("KeptDeposits: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	var out []KeptDeposit
	for rows.Next() {
		d, err := scanKeptDeposit(rows)
		if err != nil {
			return nil, fmt.Errorf("KeptDeposits: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// KeyRelease is one instance's request to be handed the deposit of another.
type KeyRelease struct {
	ID            string
	OwnerID       string
	RequesterID   string
	RequesterName string
	State         string
	AskedAt       int64
	DecidedAt     int64
	ReleasedAt    int64
}

const keyReleaseCols = `id, owner_id, requester_id, requester_name, state, asked_at, decided_at, released_at`

func scanKeyRelease(s scanner) (KeyRelease, error) {
	var k KeyRelease
	err := s.Scan(&k.ID, &k.OwnerID, &k.RequesterID, &k.RequesterName, &k.State, &k.AskedAt, &k.DecidedAt, &k.ReleasedAt)
	return k, err
}

// ReleaseLimits bounds the requests for deposits: how many new ones an
// instance may make within Window, how long one waits for an answer, and how
// long an allowed one can be collected.
type ReleaseLimits struct {
	PerWindow int
	Window    time.Duration
	Answer    time.Duration
	Collect   time.Duration
}

// expireReleasesTx closes the requests whose time is up.
func expireReleasesTx(tx *sql.Tx, now time.Time, l ReleaseLimits) error {
	_, err := tx.Exec(`
		UPDATE key_releases SET state = ?
		WHERE (state = ? AND asked_at < ?) OR (state = ? AND decided_at < ?)`,
		ReleaseExpired, ReleaseAsked, now.Add(-l.Answer).Unix(), ReleaseAllowed, now.Add(-l.Collect).Unix())
	return err
}

// AskKeyRelease returns where the request of requester for the deposit of
// owner stands. A request that waits or was allowed is returned as it is. With
// renew, which a person asking sets, anything else is replaced by a new
// request, unless the instance made PerWindow new ones within Window and gets
// ErrReleaseLimit. Without renew the newest request is returned whatever
// became of it, and one with an empty state when there never was any.
func (r *Repo) AskKeyRelease(ownerID, requesterID, requesterName string, renew bool, now time.Time, l ReleaseLimits) (out KeyRelease, created bool, err error) {
	err = r.inTx(func(tx *sql.Tx) error {
		if err := expireReleasesTx(tx, now, l); err != nil {
			return err
		}
		newest, err := scanKeyRelease(tx.QueryRow(`SELECT `+keyReleaseCols+` FROM key_releases
			WHERE owner_id = ? AND requester_id = ? ORDER BY asked_at DESC, rowid DESC LIMIT 1`, ownerID, requesterID))
		switch {
		case errors.Is(err, sql.ErrNoRows):
		case err != nil:
			return err
		case newest.State == ReleaseAsked, newest.State == ReleaseAllowed, !renew:
			out = newest
			return nil
		}
		if !renew {
			return nil
		}
		var recent int
		if err := tx.QueryRow(`SELECT count(*) FROM key_releases WHERE requester_id = ? AND asked_at > ?`,
			requesterID, now.Add(-l.Window).Unix()).Scan(&recent); err != nil {
			return err
		}
		if recent >= l.PerWindow {
			return ErrReleaseLimit
		}
		out = KeyRelease{ID: newID(), OwnerID: ownerID, RequesterID: requesterID, RequesterName: requesterName, State: ReleaseAsked, AskedAt: now.Unix()}
		created = true
		_, err = tx.Exec(`INSERT INTO key_releases (id, owner_id, requester_id, requester_name, state, asked_at) VALUES (?, ?, ?, ?, ?, ?)`,
			out.ID, out.OwnerID, out.RequesterID, out.RequesterName, out.State, out.AskedAt)
		return err
	})
	if errors.Is(err, ErrReleaseLimit) {
		return KeyRelease{}, false, err
	}
	if err != nil {
		return KeyRelease{}, false, fmt.Errorf("AskKeyRelease: %w", err)
	}
	return out, created, nil
}

// ErrReleaseDecided is returned for an answer to a request that is not
// waiting for one.
var ErrReleaseDecided = errors.New("the request is not waiting for an answer")

// DecideKeyRelease gives a waiting request a person's answer, ReleaseAllowed
// or ReleaseDeclined.
func (r *Repo) DecideKeyRelease(id, state string, now time.Time, l ReleaseLimits) (KeyRelease, error) {
	var out KeyRelease
	err := r.inTx(func(tx *sql.Tx) error {
		if err := expireReleasesTx(tx, now, l); err != nil {
			return err
		}
		res, err := tx.Exec(`UPDATE key_releases SET state = ?, decided_at = ? WHERE id = ? AND state = ?`,
			state, now.Unix(), id, ReleaseAsked)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			var exists int
			if err := tx.QueryRow(`SELECT count(*) FROM key_releases WHERE id = ?`, id).Scan(&exists); err != nil {
				return err
			}
			if exists == 0 {
				return sql.ErrNoRows
			}
			return ErrReleaseDecided
		}
		out, err = scanKeyRelease(tx.QueryRow(`SELECT `+keyReleaseCols+` FROM key_releases WHERE id = ?`, id))
		return err
	})
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrReleaseDecided) {
		return KeyRelease{}, err
	}
	if err != nil {
		return KeyRelease{}, fmt.Errorf("DecideKeyRelease: %w", err)
	}
	return out, nil
}

// CollectKeyRelease marks the allowed request of requester for the deposit of
// owner as handed out, and reports whether there was one to collect.
func (r *Repo) CollectKeyRelease(ownerID, requesterID string, now time.Time, l ReleaseLimits) (bool, error) {
	var done bool
	err := r.inTx(func(tx *sql.Tx) error {
		if err := expireReleasesTx(tx, now, l); err != nil {
			return err
		}
		res, err := tx.Exec(`UPDATE key_releases SET state = ?, released_at = ?
			WHERE owner_id = ? AND requester_id = ? AND state = ?`,
			ReleaseDone, now.Unix(), ownerID, requesterID, ReleaseAllowed)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		done = n > 0
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("CollectKeyRelease: %w", err)
	}
	return done, nil
}

// KeyReleases lists the requests for deposits, newest first, at most limit.
func (r *Repo) KeyReleases(limit int) ([]KeyRelease, error) {
	rows, err := r.db.Query(`SELECT `+keyReleaseCols+` FROM key_releases ORDER BY asked_at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("KeyReleases: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	var out []KeyRelease
	for rows.Next() {
		k, err := scanKeyRelease(rows)
		if err != nil {
			return nil, fmt.Errorf("KeyReleases: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// DepositIdentity is what an install keeps of its own recovery code: the
// public key deposits are sealed to and the salt the code is stretched with.
// Neither opens a deposit.
type DepositIdentity struct {
	Recipient []byte
	Salt      []byte
	CreatedAt int64
}

// DepositIdentity returns the install's deposit identity, found false while
// it has none.
func (r *Repo) DepositIdentity() (DepositIdentity, bool, error) {
	var d DepositIdentity
	err := r.db.QueryRow(`SELECT recipient, salt, created_at FROM key_deposit_identity WHERE id = 1`).
		Scan(&d.Recipient, &d.Salt, &d.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DepositIdentity{}, false, nil
	}
	if err != nil {
		return DepositIdentity{}, false, fmt.Errorf("DepositIdentity: %w", err)
	}
	return d, true, nil
}

// SetDepositIdentity stores the identity of a new recovery code. Every deposit
// sent before is stale from then on: it is sealed for the old code.
func (r *Repo) SetDepositIdentity(recipient, salt []byte) error {
	err := r.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			INSERT INTO key_deposit_identity (id, recipient, salt, created_at) VALUES (1, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET recipient = excluded.recipient, salt = excluded.salt, created_at = excluded.created_at`,
			recipient, salt, time.Now().Unix()); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE key_deposits_sent SET digest = '', state = ? WHERE state = ?`, DepositStale, DepositHeld)
		return err
	})
	if err != nil {
		return fmt.Errorf("SetDepositIdentity: %w", err)
	}
	return nil
}

// ClearDepositIdentity forgets the recovery code's identity, once no member
// keeps a deposit.
func (r *Repo) ClearDepositIdentity() error {
	if _, err := r.db.Exec(`DELETE FROM key_deposit_identity`); err != nil {
		return fmt.Errorf("ClearDepositIdentity: %w", err)
	}
	return nil
}

// SentDeposit is this install's deposit with one member. Digest is of the
// content that was last sealed for it.
type SentDeposit struct {
	MemberID   string
	MemberName string
	State      string
	Digest     string
	SentAt     int64
}

// SaveSentDeposit records what a member answered to a deposit.
func (r *Repo) SaveSentDeposit(d SentDeposit) error {
	_, err := r.db.Exec(`
		INSERT INTO key_deposits_sent (member_id, member_name, state, digest, sent_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (member_id) DO UPDATE SET member_name = excluded.member_name, state = excluded.state,
			digest = excluded.digest, sent_at = excluded.sent_at`,
		d.MemberID, d.MemberName, d.State, d.Digest, d.SentAt)
	if err != nil {
		return fmt.Errorf("SaveSentDeposit: %w", err)
	}
	return nil
}

// DeleteSentDeposit forgets the deposit with a member.
func (r *Repo) DeleteSentDeposit(memberID string) error {
	if _, err := r.db.Exec(`DELETE FROM key_deposits_sent WHERE member_id = ?`, memberID); err != nil {
		return fmt.Errorf("DeleteSentDeposit: %w", err)
	}
	return nil
}

// SentDeposits lists this install's deposits by member name.
func (r *Repo) SentDeposits() ([]SentDeposit, error) {
	rows, err := r.db.Query(`SELECT member_id, member_name, state, digest, sent_at FROM key_deposits_sent ORDER BY member_name, member_id`)
	if err != nil {
		return nil, fmt.Errorf("SentDeposits: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	var out []SentDeposit
	for rows.Next() {
		var d SentDeposit
		if err := rows.Scan(&d.MemberID, &d.MemberName, &d.State, &d.Digest, &d.SentAt); err != nil {
			return nil, fmt.Errorf("SentDeposits: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
