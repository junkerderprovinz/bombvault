package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

// The roles one instance asks another to take for it. A Receiver keeps the
// asking instance's off-site copies, a Fetcher fetches its backups.
const (
	RoleReceiver = "receiver"
	RoleFetcher  = "fetcher"
)

// The directions of a role request: a member asked this instance, or this
// instance asked a member.
const (
	RoleRequestIn  = "in"
	RoleRequestOut = "out"
)

// The states of a role request, the same four a ZFS receive slot has.
const (
	RoleAsked   = "asked"
	RoleAllowed = "allowed"
	RoleRefused = "refused"
	RoleRevoked = "revoked"
)

// Who settled a role request: a person signed in here, the other instance,
// the upgrade that carried over a login or pairing from before requests
// existed, or this instance by itself for a member too old to answer.
const (
	RoleDecidedHere      = "person"
	RoleDecidedMember    = "member"
	RoleDecidedUpgrade   = "upgrade"
	RoleDecidedAutomatic = "automatic"
)

// Where a Receiver keeps the copies: its append-only rest-server or a share.
const (
	RoleStoreRest  = "rest"
	RoleStoreShare = "share"
)

// ErrRoleRequestMove is returned when a request cannot take the asked-for
// answer from the state it is in.
var ErrRoleRequestMove = errors.New("the role request cannot move to that state")

// ErrRoleRequestChanged is returned when an allow names other sections than
// the request asks for: the member asked again while the person looked.
var ErrRoleRequestChanged = errors.New("the role request changed since it was shown")

// roleRequestMoves lists, for each answer, the states a person can give it
// from. A refusal can still be turned into an allow.
var roleRequestMoves = map[string][]string{
	RoleAllowed: {RoleAsked, RoleRefused},
	RoleRefused: {RoleAsked},
	RoleRevoked: {RoleAllowed},
}

// RoleRequest is one instance asking another to be its Receiver or Fetcher.
// Both sides keep a row: the asked one the request and its answer, the asking
// one what it sent and the answer it was told.
type RoleRequest struct {
	ID         string
	Direction  string
	MemberID   string
	MemberName string
	Role       string
	// Sections are the backup domains the role covers: what the asking
	// instance sends to a Receiver, or lets a Fetcher fetch.
	Sections []string
	// Store is one of the RoleStore values for a Receiver, empty for a Fetcher
	// and for a row the upgrade made from a watched repository.
	Store     string
	State     string
	AskedAt   int64
	DecidedAt int64
	// DecidedBy is one of the RoleDecided values, empty while the request
	// waits.
	DecidedBy string
}

const roleRequestCols = `id, direction, member_id, member_name, role, sections, store, state, asked_at, decided_at, decided_by`

// AskRoleRequest records a member's request and returns it as it now stands.
// Asking again keeps the answer the request has: an allowed one takes the new
// sections, a refused one stays refused unless it asks for a section it did
// not name before, and a revoked one stays revoked unless renew says a person
// on the asking instance asked anew.
func (r *Repo) AskRoleRequest(ask RoleRequest, renew bool) (RoleRequest, error) {
	ask.Direction = RoleRequestIn
	var out RoleRequest
	err := r.inTx(func(tx *sql.Tx) error {
		prev, err := findRoleRequest(tx, RoleRequestIn, ask.MemberID, ask.Role)
		if errors.Is(err, sql.ErrNoRows) {
			ask.State, ask.DecidedAt, ask.DecidedBy = RoleAsked, 0, ""
			out, err = insertRoleRequest(tx, ask)
			return err
		}
		if err != nil {
			return err
		}
		out = prev
		out.MemberName = ask.MemberName
		switch {
		case prev.State == RoleRevoked && !renew:
		case prev.State == RoleAllowed,
			prev.State == RoleRefused && subset(ask.Sections, prev.Sections):
			out.Sections, out.Store = ask.Sections, ask.Store
		default:
			out.Sections, out.Store = ask.Sections, ask.Store
			out.State, out.DecidedAt, out.DecidedBy = RoleAsked, 0, ""
			if prev.State != RoleAsked {
				out.AskedAt = time.Now().Unix()
			}
		}
		return updateRoleRequest(tx, out)
	})
	if err != nil {
		return RoleRequest{}, fmt.Errorf("AskRoleRequest: %w", err)
	}
	return out, nil
}

// RoleDecision is a person's answer to a member's request. Sections, when
// set, are the request as the person was shown it, and an allow only goes
// through while the request still asks for exactly that.
type RoleDecision struct {
	State    string
	Sections []string
}

// DecideRoleRequest gives a member's request a person's answer. Giving the
// answer it already has changes nothing. A move the state does not allow
// gives ErrRoleRequestMove, a missing request sql.ErrNoRows.
func (r *Repo) DecideRoleRequest(id string, d RoleDecision) (RoleRequest, error) {
	var req RoleRequest
	err := r.inTx(func(tx *sql.Tx) error {
		var err error
		req, err = scanRoleRequest(tx.QueryRow(`SELECT `+roleRequestCols+`
			FROM role_requests WHERE id = ? AND direction = ?`, id, RoleRequestIn))
		if err != nil {
			return err
		}
		if req.State == d.State {
			return nil
		}
		if !slices.Contains(roleRequestMoves[d.State], req.State) {
			return fmt.Errorf("DecideRoleRequest %s to %s: %w", req.State, d.State, ErrRoleRequestMove)
		}
		if d.State == RoleAllowed && d.Sections != nil && !sameMembers(d.Sections, req.Sections) {
			return fmt.Errorf("DecideRoleRequest %s: %w", id, ErrRoleRequestChanged)
		}
		req.State, req.DecidedAt, req.DecidedBy = d.State, time.Now().Unix(), RoleDecidedHere
		return updateRoleRequest(tx, req)
	})
	if err != nil {
		return RoleRequest{}, err
	}
	return req, nil
}

// WithdrawRoleRequest ends a member's request because the member took it
// back, and returns the request as it stood before. One still waiting is
// forgotten, an allowed one counts as revoked by the member, and a refusal
// stands, so withdrawing and asking again does not undo it.
func (r *Repo) WithdrawRoleRequest(memberID, role string) (RoleRequest, bool, error) {
	var prev RoleRequest
	found := false
	err := r.inTx(func(tx *sql.Tx) error {
		var err error
		prev, err = findRoleRequest(tx, RoleRequestIn, memberID, role)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		switch prev.State {
		case RoleAsked:
			_, err = tx.Exec(`DELETE FROM role_requests WHERE id = ?`, prev.ID)
		case RoleAllowed:
			ended := prev
			ended.State, ended.DecidedAt, ended.DecidedBy = RoleRevoked, time.Now().Unix(), RoleDecidedMember
			err = updateRoleRequest(tx, ended)
		}
		return err
	})
	if err != nil {
		return RoleRequest{}, false, fmt.Errorf("WithdrawRoleRequest: %w", err)
	}
	return prev, found, nil
}

// SaveSentRoleRequest records what this instance asked of a member together
// with the state the member answered, replacing an earlier request for the
// same role.
func (r *Repo) SaveSentRoleRequest(sent RoleRequest) (RoleRequest, error) {
	sent.Direction = RoleRequestOut
	var out RoleRequest
	err := r.inTx(func(tx *sql.Tx) error {
		prev, err := findRoleRequest(tx, RoleRequestOut, sent.MemberID, sent.Role)
		if errors.Is(err, sql.ErrNoRows) {
			out, err = insertRoleRequest(tx, settled(sent, RoleRequest{}))
			return err
		}
		if err != nil {
			return err
		}
		out = settled(sent, prev)
		out.ID, out.AskedAt = prev.ID, prev.AskedAt
		return updateRoleRequest(tx, out)
	})
	if err != nil {
		return RoleRequest{}, fmt.Errorf("SaveSentRoleRequest: %w", err)
	}
	return out, nil
}

// settled stamps next with when and by whom its state was decided, keeping
// the stamp of prev while the state is the same.
func settled(next, prev RoleRequest) RoleRequest {
	switch next.State {
	case RoleAsked:
		next.DecidedAt, next.DecidedBy = 0, ""
	case prev.State:
		next.DecidedAt, next.DecidedBy = prev.DecidedAt, prev.DecidedBy
	default:
		next.DecidedAt = time.Now().Unix()
		if next.DecidedBy == "" {
			next.DecidedBy = RoleDecidedMember
		}
	}
	return next
}

// SetSentRoleRequestState records the answer a member gave to a request this
// instance sent it, and reports whether there is such a request.
func (r *Repo) SetSentRoleRequestState(memberID, role, state string) (bool, error) {
	found := false
	err := r.inTx(func(tx *sql.Tx) error {
		prev, err := findRoleRequest(tx, RoleRequestOut, memberID, role)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		next := prev
		next.State, next.DecidedBy = state, ""
		return updateRoleRequest(tx, settled(next, prev))
	})
	if err != nil {
		return false, fmt.Errorf("SetSentRoleRequestState: %w", err)
	}
	return found, nil
}

// GrantRoleRequest makes sure the request between this instance and a member
// is allowed without anyone answering it, for a member too old to take part
// in requests. A request a person refused or revoked is left as it is.
func (r *Repo) GrantRoleRequest(grant RoleRequest) error {
	err := r.inTx(func(tx *sql.Tx) error {
		prev, err := findRoleRequest(tx, grant.Direction, grant.MemberID, grant.Role)
		if errors.Is(err, sql.ErrNoRows) {
			grant.State, grant.DecidedAt, grant.DecidedBy = RoleAllowed, time.Now().Unix(), RoleDecidedAutomatic
			_, err = insertRoleRequest(tx, grant)
			return err
		}
		if err != nil || prev.State == RoleRefused || prev.State == RoleRevoked {
			return err
		}
		prev.MemberName, prev.Sections, prev.Store = grant.MemberName, grant.Sections, grant.Store
		if prev.State != RoleAllowed {
			prev.State, prev.DecidedAt, prev.DecidedBy = RoleAllowed, time.Now().Unix(), RoleDecidedAutomatic
		}
		return updateRoleRequest(tx, prev)
	})
	if err != nil {
		return fmt.Errorf("GrantRoleRequest: %w", err)
	}
	return nil
}

// DeleteRoleRequest forgets the request for role between this instance and a
// member in one direction. A missing one is not an error.
func (r *Repo) DeleteRoleRequest(direction, memberID, role string) error {
	if _, err := r.db.Exec(`DELETE FROM role_requests WHERE direction = ? AND member_id = ? AND role = ?`,
		direction, memberID, role); err != nil {
		return fmt.Errorf("DeleteRoleRequest: %w", err)
	}
	return nil
}

// GetRoleRequest returns the request with the given id, or false when there
// is none.
func (r *Repo) GetRoleRequest(id string) (RoleRequest, bool, error) {
	return optionalRoleRequest(scanRoleRequest(r.db.QueryRow(`SELECT `+roleRequestCols+` FROM role_requests WHERE id = ?`, id)))
}

// FindRoleRequest returns the request for role between this instance and a
// member in one direction, or false when there is none.
func (r *Repo) FindRoleRequest(direction, memberID, role string) (RoleRequest, bool, error) {
	return optionalRoleRequest(findRoleRequest(r.db, direction, memberID, role))
}

func optionalRoleRequest(req RoleRequest, err error) (RoleRequest, bool, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return RoleRequest{}, false, nil
	}
	if err != nil {
		return RoleRequest{}, false, err
	}
	return req, true, nil
}

// ListRoleRequests returns every request in both directions, oldest first.
func (r *Repo) ListRoleRequests() ([]RoleRequest, error) {
	rows, err := r.db.Query(`SELECT ` + roleRequestCols + ` FROM role_requests ORDER BY asked_at, id`)
	if err != nil {
		return nil, fmt.Errorf("ListRoleRequests: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []RoleRequest
	for rows.Next() {
		req, err := scanRoleRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func findRoleRequest(q rowQuerier, direction, memberID, role string) (RoleRequest, error) {
	return scanRoleRequest(q.QueryRow(`SELECT `+roleRequestCols+`
		FROM role_requests WHERE direction = ? AND member_id = ? AND role = ?`, direction, memberID, role))
}

func insertRoleRequest(tx *sql.Tx, req RoleRequest) (RoleRequest, error) {
	req.ID = newID()
	if req.AskedAt == 0 {
		req.AskedAt = time.Now().Unix()
	}
	sections, err := marshalList(req.Sections)
	if err != nil {
		return RoleRequest{}, err
	}
	_, err = tx.Exec(`INSERT INTO role_requests (`+roleRequestCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.Direction, req.MemberID, req.MemberName, req.Role, sections, req.Store,
		req.State, req.AskedAt, req.DecidedAt, req.DecidedBy)
	return req, err
}

func updateRoleRequest(tx *sql.Tx, req RoleRequest) error {
	sections, err := marshalList(req.Sections)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE role_requests SET member_name = ?, sections = ?, store = ?, state = ?,
		asked_at = ?, decided_at = ?, decided_by = ? WHERE id = ?`,
		req.MemberName, sections, req.Store, req.State, req.AskedAt, req.DecidedAt, req.DecidedBy, req.ID)
	return err
}

func scanRoleRequest(sc scanner) (RoleRequest, error) {
	var req RoleRequest
	var sections string
	err := sc.Scan(&req.ID, &req.Direction, &req.MemberID, &req.MemberName, &req.Role, &sections, &req.Store,
		&req.State, &req.AskedAt, &req.DecidedAt, &req.DecidedBy)
	if err != nil {
		return RoleRequest{}, err
	}
	if err := json.Unmarshal([]byte(sections), &req.Sections); err != nil {
		return RoleRequest{}, fmt.Errorf("scanRoleRequest %s: %w", req.ID, err)
	}
	return req, nil
}
