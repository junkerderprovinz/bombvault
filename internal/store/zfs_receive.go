package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

// The states of a receive slot. A source item with a peer target mirrors the
// state its receiving instance answered.
const (
	ZFSReceiveAsked   = "asked"
	ZFSReceiveAllowed = "allowed"
	ZFSReceiveRefused = "refused"
	ZFSReceiveRevoked = "revoked"
)

// ErrZFSReceiveMove is returned when a slot cannot take the asked-for answer
// from the state it is in.
var ErrZFSReceiveMove = errors.New("the receive slot cannot move to that state")

// zfsReceiveMoves lists, for each answer, the states a person can give it
// from. A refusal can still be turned into an allow here; the source cannot
// undo it by asking again.
var zfsReceiveMoves = map[string][]string{
	ZFSReceiveAllowed: {ZFSReceiveAsked, ZFSReceiveRefused},
	ZFSReceiveRefused: {ZFSReceiveAsked},
	ZFSReceiveRevoked: {ZFSReceiveAllowed},
}

// ZFSReceiveSlot is another instance's request to replicate one of its ZFS
// items into a pool of this one, and, once a person allowed it, the slot its
// streams arrive through.
type ZFSReceiveSlot struct {
	ID       string
	PeerID   string
	PeerName string
	// ItemID is the item's id on the source, and Dataset its root there.
	ItemID  string
	Dataset string
	// SourceServer is the folder under Root the members land in. It is fixed
	// when the source first asks, so a later rename keeps the folder.
	SourceServer string
	// Members are the source datasets the slot accepts streams for.
	Members      []string
	ProposedKeep ZFSReplicaKeep
	Pool         string
	Root         string
	Keep         ZFSReplicaKeep
	// TokenEnc is the slot's bearer token sealed with the APP_KEY, empty
	// unless the slot is allowed.
	TokenEnc  []byte
	State     string
	AskedAt   int64
	DecidedAt int64
	// LastReceived is when a stream last landed, and Bytes how many arrived
	// over the slot's life.
	LastReceived int64
	Bytes        int64
}

// Base is the dataset the slot's members land under.
func (s ZFSReceiveSlot) Base() string { return s.Root + "/" + s.SourceServer }

// ZFSReceiveDecision is a person's answer to a slot. Pool, Root, Keep and
// TokenEnc belong to an allow.
type ZFSReceiveDecision struct {
	State    string
	Pool     string
	Root     string
	Keep     ZFSReplicaKeep
	TokenEnc []byte
}

const zfsReceiveSlotCols = `id, peer_id, peer_name, item_id, dataset, source_server, members, proposed_keep,
	pool, root, keep, token_enc, state, asked_at, decided_at, last_received, bytes`

// AskZFSReceive records a source's request for one of its items and returns
// the slot as it now stands. A request for the members the slot already
// holds, or for fewer, keeps the answer it has, so a refusal stands and an
// allowed slot narrows; more members wait for a person again. A revoked slot
// stays revoked unless renew says a person on the source asked anew.
func (r *Repo) AskZFSReceive(ask ZFSReceiveSlot, renew bool) (ZFSReceiveSlot, error) {
	var out ZFSReceiveSlot
	err := r.inTx(func(tx *sql.Tx) error {
		prev, err := scanZFSReceiveSlot(tx.QueryRow(`SELECT `+zfsReceiveSlotCols+`
			FROM zfs_receive_slots WHERE peer_id = ? AND item_id = ?`, ask.PeerID, ask.ItemID))
		if errors.Is(err, sql.ErrNoRows) {
			out, err = insertZFSReceiveSlot(tx, ask)
			return err
		}
		if err != nil {
			return err
		}
		out = prev
		out.PeerName = ask.PeerName
		narrower := prev.Dataset == ask.Dataset && subset(ask.Members, prev.Members)
		switch {
		case prev.State == ZFSReceiveRevoked && !renew:
		case prev.State == ZFSReceiveAllowed && narrower,
			prev.State == ZFSReceiveRefused && narrower:
			out.Members = ask.Members
		default:
			out.Dataset, out.Members, out.ProposedKeep = ask.Dataset, ask.Members, ask.ProposedKeep
			out.State, out.TokenEnc, out.DecidedAt = ZFSReceiveAsked, nil, 0
			if prev.State != ZFSReceiveAsked {
				out.AskedAt = time.Now().Unix()
			}
		}
		return updateZFSReceiveSlot(tx, out)
	})
	if err != nil {
		return ZFSReceiveSlot{}, fmt.Errorf("AskZFSReceive: %w", err)
	}
	return out, nil
}

func subset(list, of []string) bool {
	for _, s := range list {
		if !slices.Contains(of, s) {
			return false
		}
	}
	return true
}

func insertZFSReceiveSlot(tx *sql.Tx, s ZFSReceiveSlot) (ZFSReceiveSlot, error) {
	s.ID = newID()
	s.State, s.TokenEnc, s.DecidedAt = ZFSReceiveAsked, nil, 0
	s.Keep = s.ProposedKeep
	if s.AskedAt == 0 {
		s.AskedAt = time.Now().Unix()
	}
	members, proposed, keep, err := zfsReceiveSlotJSON(s)
	if err != nil {
		return ZFSReceiveSlot{}, err
	}
	_, err = tx.Exec(`INSERT INTO zfs_receive_slots (`+zfsReceiveSlotCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, x'', ?, ?, 0, 0, 0)`,
		s.ID, s.PeerID, s.PeerName, s.ItemID, s.Dataset, s.SourceServer, members, proposed,
		s.Pool, s.Root, keep, s.State, s.AskedAt)
	return s, err
}

func updateZFSReceiveSlot(tx *sql.Tx, s ZFSReceiveSlot) error {
	members, proposed, keep, err := zfsReceiveSlotJSON(s)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE zfs_receive_slots SET peer_name = ?, dataset = ?, members = ?, proposed_keep = ?,
		pool = ?, root = ?, keep = ?, token_enc = ?, state = ?, asked_at = ?, decided_at = ?
		WHERE id = ?`,
		s.PeerName, s.Dataset, members, proposed, s.Pool, s.Root, keep, notNullBlob(s.TokenEnc),
		s.State, s.AskedAt, s.DecidedAt, s.ID)
	return err
}

func zfsReceiveSlotJSON(s ZFSReceiveSlot) (members, proposed, keep string, err error) {
	if members, err = marshalList(s.Members); err != nil {
		return "", "", "", err
	}
	p, err := json.Marshal(s.ProposedKeep)
	if err != nil {
		return "", "", "", err
	}
	k, err := json.Marshal(s.Keep)
	if err != nil {
		return "", "", "", err
	}
	return members, string(p), string(k), nil
}

// DecideZFSReceive gives a slot a person's answer. An allow writes where the
// members land and the token, a refusal or a revoke drops the token. A move
// the slot's state does not allow gives ErrZFSReceiveMove, a missing slot
// sql.ErrNoRows.
func (r *Repo) DecideZFSReceive(id string, d ZFSReceiveDecision) (ZFSReceiveSlot, error) {
	var s ZFSReceiveSlot
	err := r.inTx(func(tx *sql.Tx) error {
		var err error
		s, err = scanZFSReceiveSlot(tx.QueryRow(`SELECT `+zfsReceiveSlotCols+` FROM zfs_receive_slots WHERE id = ?`, id))
		if err != nil {
			return err
		}
		if !slices.Contains(zfsReceiveMoves[d.State], s.State) {
			return fmt.Errorf("DecideZFSReceive %s to %s: %w", s.State, d.State, ErrZFSReceiveMove)
		}
		s.State, s.DecidedAt, s.TokenEnc = d.State, time.Now().Unix(), nil
		if d.State == ZFSReceiveAllowed {
			s.Pool, s.Root, s.Keep, s.TokenEnc = d.Pool, d.Root, d.Keep, d.TokenEnc
		}
		return updateZFSReceiveSlot(tx, s)
	})
	if err != nil {
		return ZFSReceiveSlot{}, err
	}
	return s, nil
}

// SetZFSReceiveKeep writes what the slot's members keep here.
func (r *Repo) SetZFSReceiveKeep(id string, keep ZFSReplicaKeep) error {
	raw, err := json.Marshal(keep)
	if err != nil {
		return fmt.Errorf("SetZFSReceiveKeep marshal: %w", err)
	}
	res, err := r.db.Exec(`UPDATE zfs_receive_slots SET keep = ? WHERE id = ?`, string(raw), id)
	if err != nil {
		return fmt.Errorf("SetZFSReceiveKeep: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("SetZFSReceiveKeep %q: %w", id, sql.ErrNoRows)
	}
	return nil
}

// RecordZFSReceived notes a stream of n bytes that landed through the slot.
func (r *Repo) RecordZFSReceived(id string, n int64) error {
	if _, err := r.db.Exec(`UPDATE zfs_receive_slots SET last_received = ?, bytes = bytes + ? WHERE id = ?`,
		time.Now().Unix(), n, id); err != nil {
		return fmt.Errorf("RecordZFSReceived: %w", err)
	}
	return nil
}

// GetZFSReceiveSlot returns the slot with the given id, or false when there is
// none.
func (r *Repo) GetZFSReceiveSlot(id string) (ZFSReceiveSlot, bool, error) {
	s, err := scanZFSReceiveSlot(r.db.QueryRow(`SELECT `+zfsReceiveSlotCols+` FROM zfs_receive_slots WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ZFSReceiveSlot{}, false, nil
	}
	if err != nil {
		return ZFSReceiveSlot{}, false, err
	}
	return s, true, nil
}

// ListZFSReceiveSlots returns every slot, oldest request first.
func (r *Repo) ListZFSReceiveSlots() ([]ZFSReceiveSlot, error) {
	rows, err := r.db.Query(`SELECT ` + zfsReceiveSlotCols + ` FROM zfs_receive_slots ORDER BY asked_at, id`)
	if err != nil {
		return nil, fmt.Errorf("ListZFSReceiveSlots: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []ZFSReceiveSlot
	for rows.Next() {
		s, err := scanZFSReceiveSlot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func scanZFSReceiveSlot(sc scanner) (ZFSReceiveSlot, error) {
	var s ZFSReceiveSlot
	var members, proposed, keep string
	var token []byte
	err := sc.Scan(&s.ID, &s.PeerID, &s.PeerName, &s.ItemID, &s.Dataset, &s.SourceServer, &members, &proposed,
		&s.Pool, &s.Root, &keep, &token, &s.State, &s.AskedAt, &s.DecidedAt, &s.LastReceived, &s.Bytes)
	if err != nil {
		return ZFSReceiveSlot{}, err
	}
	for _, col := range []struct {
		raw  string
		into any
	}{{members, &s.Members}, {proposed, &s.ProposedKeep}, {keep, &s.Keep}} {
		if err := json.Unmarshal([]byte(col.raw), col.into); err != nil {
			return ZFSReceiveSlot{}, fmt.Errorf("scanZFSReceiveSlot %s: %w", s.ID, err)
		}
	}
	if len(token) > 0 {
		s.TokenEnc = token
	}
	return s, nil
}
