package store

import (
	"bytes"
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
	// DepositStale is a deposit the keeper holds an older form of: the kit
	// says something else since, or the recovery code is another.
	DepositStale = "stale"
	// DepositChanged is what an install records when the keeper holds,
	// under the install's id, something the install did not send. Either
	// the bytes differ and the fresh deposit has not arrived yet, or
	// another key made the deposit and the keeper takes none from the
	// install until a person there deletes that one.
	DepositChanged = "changed"
)

var (
	// ErrDepositNotOwner is returned for a deposit under an id whose kept
	// deposit was made with another proof key.
	ErrDepositNotOwner = errors.New("the deposit kept under this id was made by another key")
	// ErrDepositReplayed is returned for a deposit that is not newer than the
	// one kept.
	ErrDepositReplayed = errors.New("a deposit at least as new is kept already")
	// ErrDepositsFull is returned for a first deposit once the install keeps
	// as many as it takes.
	ErrDepositsFull = errors.New("this instance already keeps as many deposits as it can")
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
// OwnerKey is the proof key the deposit came with, which every later one and
// the call that takes it back have to be signed with, and Counter the highest
// one a signed call carried.
type KeptDeposit struct {
	OwnerID   string
	OwnerName string
	State     string
	Deposit   []byte
	Salt      []byte
	Version   int
	OwnerKey  []byte
	Counter   int64
	CreatedAt int64
	UpdatedAt int64
}

func scanKeptDeposit(s scanner) (KeptDeposit, error) {
	var d KeptDeposit
	err := s.Scan(&d.OwnerID, &d.OwnerName, &d.State, &d.Deposit, &d.Salt, &d.Version, &d.OwnerKey, &d.Counter, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

const keptDepositCols = `owner_id, owner_name, state, deposit, salt, version, owner_key, counter, created_at, updated_at`

// KeepDeposit stores the deposit a member sent and returns the state it is in
// afterwards and whether it is the member's first. A deposit this install
// deleted stays declined unless renew says a person there asked again. One
// that is kept is replaced only by a newer deposit with the same proof key,
// and a first one is taken while the install keeps fewer than most.
func (r *Repo) KeepDeposit(d KeptDeposit, renew bool, most int) (state string, first bool, err error) {
	now := time.Now().Unix()
	err = r.inTx(func(tx *sql.Tx) error {
		var held string
		var key []byte
		var counter int64
		switch err := tx.QueryRow(`SELECT state, owner_key, counter FROM key_deposits WHERE owner_id = ?`, d.OwnerID).Scan(&held, &key, &counter); {
		case errors.Is(err, sql.ErrNoRows):
			var kept int
			if err := tx.QueryRow(`SELECT count(*) FROM key_deposits`).Scan(&kept); err != nil {
				return err
			}
			if kept >= most {
				return ErrDepositsFull
			}
			first = true
		case err != nil:
			return err
		case held == DepositDeclined && !renew:
			state = DepositDeclined
			return nil
		case len(key) > 0 && !bytes.Equal(key, d.OwnerKey):
			return ErrDepositNotOwner
		case d.Counter <= counter:
			return ErrDepositReplayed
		}
		state = DepositHeld
		_, err := tx.Exec(`
			INSERT INTO key_deposits (owner_id, owner_name, state, deposit, salt, version, owner_key, counter, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (owner_id) DO UPDATE SET owner_name = excluded.owner_name, state = excluded.state,
				deposit = excluded.deposit, salt = excluded.salt, version = excluded.version,
				owner_key = excluded.owner_key, counter = excluded.counter, updated_at = excluded.updated_at`,
			d.OwnerID, d.OwnerName, DepositHeld, d.Deposit, d.Salt, d.Version, d.OwnerKey, d.Counter, now, now)
		return err
	})
	switch {
	case errors.Is(err, ErrDepositNotOwner), errors.Is(err, ErrDepositReplayed), errors.Is(err, ErrDepositsFull):
		return "", false, err
	case err != nil:
		return "", false, fmt.Errorf("KeepDeposit: %w", err)
	}
	return state, first, nil
}

// DeclineDeposit deletes what this install keeps for a member and remembers
// that it did. It reports whether there was a deposit. The proof key goes
// with it, so that the member's own key is taken when a person there asks
// again, whoever made the deposit that was deleted.
func (r *Repo) DeclineDeposit(ownerID string) (bool, error) {
	res, err := r.db.Exec(`UPDATE key_deposits SET state = ?, deposit = x'', salt = x'', owner_key = x'', updated_at = ? WHERE owner_id = ? AND state = ?`,
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
// instance may make within Window and how many all of them together, since an
// instance names itself, how long one waits for an answer, how long an
// allowed one can be collected, and how long a request that is over stays on
// record.
type ReleaseLimits struct {
	PerWindow int
	Total     int
	Window    time.Duration
	Answer    time.Duration
	Collect   time.Duration
	Keep      time.Duration
}

// expireReleasesTx closes the requests whose time is up and forgets the ones
// that have been over for longer than they are kept.
func expireReleasesTx(tx *sql.Tx, now time.Time, l ReleaseLimits) error {
	if _, err := tx.Exec(`
		UPDATE key_releases SET state = ?
		WHERE (state = ? AND asked_at < ?) OR (state = ? AND decided_at < ?)`,
		ReleaseExpired, ReleaseAsked, now.Add(-l.Answer).Unix(), ReleaseAllowed, now.Add(-l.Collect).Unix()); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM key_releases WHERE asked_at < ? AND state NOT IN (?, ?)`,
		now.Add(-l.Keep).Unix(), ReleaseAsked, ReleaseAllowed)
	return err
}

// AskKeyRelease returns where the request of requester for the deposit of
// owner stands. A request that waits or was allowed is returned as it is. With
// renew, which a person asking sets, anything else is replaced by a new
// request, unless the instance made PerWindow new ones within Window, or all
// instances Total, and gets ErrReleaseLimit. Without renew the newest request is returned whatever
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
		var recent, all int
		if err := tx.QueryRow(`SELECT count(*) FILTER (WHERE requester_id = ?), count(*) FROM key_releases WHERE asked_at > ?`,
			requesterID, now.Add(-l.Window).Unix()).Scan(&recent, &all); err != nil {
			return err
		}
		if recent >= l.PerWindow || all >= l.Total {
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
// content that was last sealed for it and HeldDigest of the sealed bytes the
// member took, which it names again when asked what it holds. ChangedAt is
// when the member was last found holding something else, zero for never.
type SentDeposit struct {
	MemberID   string
	MemberName string
	State      string
	Digest     string
	HeldDigest string
	SentAt     int64
	ChangedAt  int64
}

// SaveSentDeposit records what a member answered to a deposit.
func (r *Repo) SaveSentDeposit(d SentDeposit) error {
	_, err := r.db.Exec(`
		INSERT INTO key_deposits_sent (member_id, member_name, state, digest, held_digest, sent_at, changed_at) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (member_id) DO UPDATE SET member_name = excluded.member_name, state = excluded.state,
			digest = excluded.digest, held_digest = excluded.held_digest, sent_at = excluded.sent_at, changed_at = excluded.changed_at`,
		d.MemberID, d.MemberName, d.State, d.Digest, d.HeldDigest, d.SentAt, d.ChangedAt)
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
	rows, err := r.db.Query(`SELECT member_id, member_name, state, digest, held_digest, sent_at, changed_at FROM key_deposits_sent ORDER BY member_name, member_id`)
	if err != nil {
		return nil, fmt.Errorf("SentDeposits: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	var out []SentDeposit
	for rows.Next() {
		var d SentDeposit
		if err := rows.Scan(&d.MemberID, &d.MemberName, &d.State, &d.Digest, &d.HeldDigest, &d.SentAt, &d.ChangedAt); err != nil {
			return nil, fmt.Errorf("SentDeposits: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DepositSigner returns the sealed seed of the key this install proves its
// deposits with, found false before the first deposit made it.
func (r *Repo) DepositSigner() (seedEnc []byte, found bool, err error) {
	err = r.db.QueryRow(`SELECT seed_enc FROM key_deposit_signer WHERE id = 1`).Scan(&seedEnc)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("DepositSigner: %w", err)
	}
	return seedEnc, true, nil
}

// SetDepositSigner stores the sealed seed of the install's proof key. The key
// outlives a recovery code: the keepers hold its public half.
func (r *Repo) SetDepositSigner(seedEnc []byte) error {
	if _, err := r.db.Exec(`INSERT INTO key_deposit_signer (id, seed_enc, created_at) VALUES (1, ?, ?)`, seedEnc, time.Now().Unix()); err != nil {
		return fmt.Errorf("SetDepositSigner: %w", err)
	}
	return nil
}

// NextDepositCounter returns the counter of the next signed call: higher than
// every one before, and no lower than floor. The caller passes the time as
// floor, so that settings restored from an older backup do not count again
// from where they were.
func (r *Repo) NextDepositCounter(floor int64) (int64, error) {
	var next int64
	err := r.db.QueryRow(`UPDATE key_deposit_signer SET counter = max(counter + 1, ?) WHERE id = 1 RETURNING counter`, floor).Scan(&next)
	if err != nil {
		return 0, fmt.Errorf("NextDepositCounter: %w", err)
	}
	return next, nil
}
