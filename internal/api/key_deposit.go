package api

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A key deposit lets an install that is gone come back through its group. The
// install seals its recovery kit for a code only the person has and gives the
// result to a member, the keeper. The keeper cannot read it. On the day, a new
// install joins the group, asks the keeper for the deposit, a person on the
// keeper allows that, and the code typed on the new install opens it there.

// keyDepositCapability is what an instance that can keep deposits names in
// its status. One that does not name it is older and is not sent any.
const keyDepositCapability = "key-deposit"

// peerCapabilities is what this instance names in its status.
var peerCapabilities = []string{keyDepositCapability}

// roleKeeper is the role a member takes when it keeps this install's deposit.
const roleKeeper = "keeper"

// The kinds the runs of store.KeyTargetID have for deposits. The first five
// are written on the install the deposit belongs to, the rest on the keeper.
// keyEventChanged is a keeper found holding something the install did not
// send.
const (
	keyEventDeposit  = "keydeposit"
	keyEventRenew    = "keyrenew"
	keyEventWithdraw = "keywithdraw"
	keyEventCode     = "keycode"
	keyEventChanged  = "keychanged"
	keyEventKeep     = "keykeep"
	keyEventGone     = "keygone"
	keyEventDrop     = "keydrop"
	keyEventAsk      = "keyask"
	keyEventAllow    = "keyallow"
	keyEventRefuse   = "keyrefuse"
	keyEventRelease  = "keyrelease"
)

// What a keeper answers a release request with besides the states of
// store.KeyRelease: it keeps no such deposit, the instance asked too often,
// or it never asked.
const (
	releaseNone    = "none"
	releaseLimited = "limited"
	releaseUnasked = "unasked"
)

// releaseLimits are what a keeper holds release requests to: three new ones an
// hour from one instance and six from all of them, a day to answer, the
// length of an emergency to collect an allowed one, and thirty days on record
// once a request is over.
var releaseLimits = store.ReleaseLimits{
	PerWindow: 3, Total: 6, Window: time.Hour,
	Answer: 24 * time.Hour, Collect: emergencyTTL, Keep: 30 * 24 * time.Hour,
}

// depositRenewEvery is how often an install looks whether what it deposited
// still says where its backups are.
const depositRenewEvery = 15 * time.Minute

// A deposit is a recovery kit with its rclone config, a few kilobytes.
// maxDepositBytes caps the sealed bytes and peerDepositMax the call they
// arrive in, where they are base64. maxKeptDeposits is how many installs one
// keeper takes a deposit from.
const (
	maxDepositBytes = 256 << 10
	peerDepositMax  = 512 << 10
	maxKeptDeposits = 32
)

// depositFull is what a keeper answers a first deposit with once it keeps
// maxKeptDeposits.
const depositFull = "full"

var (
	errKeeperTooOld = errors.New("that instance runs a version that cannot keep a deposit. Update it first")
	errNoDeposit    = errors.New("that instance keeps no deposit of the server you chose")
	errReleaseNo    = errors.New("the request for the deposit was declined on the instance that keeps it")
	errReleaseLimit = errors.New("this server asked for deposits too often. Try again in an hour")
	errDepositNewer = errors.New("the deposit was made by a newer version of BombVault than this one")
	errKeeperFull   = errors.New("that instance already keeps as many deposits as it can")
	// errDepositChanged is recorded when a keeper holds other bytes than the
	// ones this install sent, or none.
	// errDepositOtherOwner is a deposit that opened and says inside that
	// another install made it than the one the keeper held it for.
	errDepositOtherOwner = errors.New("this deposit was not made by the server it was kept for, so it is not used")
	errDepositChanged    = errors.New("the instance held something other than the deposit this server sent, so somebody else changed it there")
)

var depositCodes = []struct {
	err  error
	code string
}{
	{errKeeperTooOld, "keeper-too-old"},
	{errNoDeposit, "deposit-none"},
	{errReleaseNo, "release-declined"},
	{errReleaseLimit, "release-limit"},
	{errDepositNewer, "deposit-version"},
	{errKeeperFull, "keeper-full"},
	{errDepositChanged, "deposit-changed"},
	{errDepositOtherOwner, "deposit-owner"},
	{keydeposit.ErrNotADeposit, "deposit-unreadable"},
	{keydeposit.ErrCode, "deposit-code-shape"},
	{keydeposit.ErrWrongCode, "deposit-code"},
	{errMemberGone, "member-gone"},
	{errMemberSilent, "member-silent"},
	{errBackupKeyLocked, "backup-key-locked"},
	{store.ErrReleaseDecided, "release-decided"},
}

func depositCode(err error) string {
	for _, c := range depositCodes {
		if errors.Is(err, c.err) {
			return c.code
		}
	}
	return ""
}

func depositFail(w http.ResponseWriter, err error) {
	env := map[string]any{"ok": false, "error": scrubError(err)}
	if code := depositCode(err); code != "" {
		env["code"] = code
	}
	writeJSON(w, http.StatusOK, env)
}

// depositContent is what a deposit holds, this install's recovery kit, and a
// digest that changes when the kit says something else. The line with the
// time the kit was written is left out of the digest.
func (s *Service) depositContent(recipient []byte) (kit, digest string, err error) {
	kit, err = s.RecoveryKit()
	if err != nil {
		return "", "", err
	}
	sum := sha256.New()
	sum.Write(recipient)
	for line := range strings.SplitSeq(kit, "\n") {
		if !strings.HasPrefix(line, "Generated: ") {
			sum.Write([]byte(line + "\n"))
		}
	}
	return kit, hex.EncodeToString(sum.Sum(nil)), nil
}

// peerProof is what an install signs a call to its keeper with: the public
// half of its proof key, a counter that only grows, and the signature over
// what the call says.
type peerProof struct {
	Key     []byte `json:"key"`
	Counter int64  `json:"counter"`
	Proof   []byte `json:"proof"`
}

// peerDeposit is what an install sends a member to keep. Renew is set when a
// person asked, which is the only deposit a keeper that deleted the last one
// takes again.
type peerDeposit struct {
	Name    string `json:"name"`
	Deposit []byte `json:"deposit"`
	Salt    []byte `json:"salt"`
	Version int    `json:"version"`
	Renew   bool   `json:"renew"`
	peerProof
}

type peerDepositState struct {
	OK    bool   `json:"ok"`
	State string `json:"state"`
}

// depositSeed returns the seed of this install's proof key, which the first
// deposit makes.
func (s *Service) depositSeed() ([]byte, error) {
	enc, found, err := s.store.DepositSigner()
	if err != nil {
		return nil, err
	}
	if found {
		return secret.Decrypt(s.cfg.AppKey, enc)
	}
	seed, err := keydeposit.NewOwnerSeed()
	if err != nil {
		return nil, err
	}
	enc, err = secret.Encrypt(s.cfg.AppKey, seed)
	if err != nil {
		return nil, err
	}
	return seed, s.store.SetDepositSigner(enc)
}

// depositSigner is this install as it signs a call to a keeper: its id, the
// seed of its proof key and the counter of the call.
type depositSigner struct {
	id      string
	seed    []byte
	counter int64
}

func (s *Service) nextDepositSigner() (depositSigner, error) {
	seed, err := s.depositSeed()
	if err != nil {
		return depositSigner{}, err
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return depositSigner{}, err
	}
	counter, err := s.store.NextDepositCounter(time.Now().UnixMilli())
	if err != nil {
		return depositSigner{}, err
	}
	return depositSigner{id: g.InstanceID, seed: seed, counter: counter}, nil
}

// prove signs what this install says to a keeper about its deposit.
func (d depositSigner) prove(action, keeperID string, deposit, salt []byte) peerProof {
	claim := keydeposit.Claim{Action: action, OwnerID: d.id, KeeperID: keeperID, Counter: d.counter, Deposit: deposit, Salt: salt}
	return peerProof{Key: keydeposit.OwnerKey(d.seed), Counter: d.counter, Proof: keydeposit.Prove(d.seed, claim)}
}

// depositRecipient opens the public key this install's deposits are sealed
// to. One a database kept in the clear is sealed on the way.
func (s *Service) depositRecipient(identity store.DepositIdentity) ([]byte, error) {
	if len(identity.RecipientEnc) > 0 {
		return secret.Decrypt(s.cfg.AppKey, identity.RecipientEnc)
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, identity.Recipient)
	if err != nil {
		return nil, err
	}
	return identity.Recipient, s.store.SealDepositRecipient(enc)
}

// heldDigest names sealed bytes the way a keeper and the install they belong
// to compare what is held.
func heldDigest(sealed []byte) string {
	sum := sha256.Sum256(sealed)
	return hex.EncodeToString(sum[:])
}

// sendDeposit seals this install's kit for its recovery code and gives it to
// a member. It needs no code: the public half the install kept is enough.
// changedAt is carried into the record of the deposit.
func (s *Service) sendDeposit(ctx context.Context, memberID, memberName string, renew bool, changedAt int64) (string, error) {
	identity, found, err := s.store.DepositIdentity()
	if err != nil {
		return "", err
	}
	if !found {
		return "", errors.New("this server has no recovery code yet")
	}
	recipient, err := s.depositRecipient(identity)
	if err != nil {
		return "", err
	}
	kit, digest, err := s.depositContent(recipient)
	if err != nil {
		return "", err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", err
	}
	signer, err := s.nextDepositSigner()
	if err != nil {
		return "", err
	}
	name := instanceDisplayName(settings)
	plain, err := keydeposit.Pack(keydeposit.Content{
		OwnerID: signer.id, OwnerName: name, Counter: signer.counter, SealedAt: time.Now().Unix(), Kit: kit,
	})
	if err != nil {
		return "", err
	}
	sealed, err := keydeposit.Seal(plain, recipient)
	if err != nil {
		return "", err
	}
	in := peerDeposit{
		Name: name, Deposit: sealed, Salt: identity.Salt,
		Version: keydeposit.Version, Renew: renew, peerProof: signer.prove(keydeposit.ProofKeep, memberID, sealed, identity.Salt),
	}
	var ans peerDepositState
	if err := s.callMember(ctx, memberID, http.MethodPut, "/api/group/peer/deposit", in, &ans); err != nil {
		return "", err
	}
	sent := store.SentDeposit{MemberID: memberID, MemberName: memberName, State: ans.State, SentAt: time.Now().Unix(), ChangedAt: changedAt}
	switch ans.State {
	case store.DepositHeld:
		sent.Digest, sent.HeldDigest = digest, heldDigest(sealed)
	case store.DepositDeclined, store.DepositChanged:
	case depositFull:
		return "", errKeeperFull
	default:
		return "", errMemberUnreadable
	}
	if err := s.store.SaveSentDeposit(sent); err != nil {
		return "", err
	}
	return ans.State, nil
}

// canKeepDeposits asks a member whether it names the capability.
func (s *Service) canKeepDeposits(ctx context.Context, memberID string) (bool, error) {
	var status fleetStatusResponse
	if err := s.callMember(ctx, memberID, http.MethodGet, "/api/group/peer/status", nil, &status); err != nil {
		return false, err
	}
	return slices.Contains(status.Capabilities, keyDepositCapability), nil
}

// MakeDeposit gives a member this install's deposit, as a person here wants
// it. The first deposit makes the recovery code, which is returned this once
// and kept nowhere.
func (s *Service) MakeDeposit(ctx context.Context, memberID string) (state, code string, err error) {
	m, ok := s.reachableMember(memberID)
	if !ok {
		return "", "", errMemberGone
	}
	if predatesRoleRequests(m.Version) {
		return "", "", errKeeperTooOld
	}
	if can, err := s.canKeepDeposits(ctx, memberID); err != nil {
		return "", "", err
	} else if !can {
		return "", "", errKeeperTooOld
	}
	if _, err := s.backupKey(); err != nil {
		return "", "", err
	}
	s.depositMu.Lock()
	defer s.depositMu.Unlock()
	if _, found, err := s.store.DepositIdentity(); err != nil {
		return "", "", err
	} else if !found {
		if code, err = s.newDepositIdentity(); err != nil {
			return "", "", err
		}
	}
	state, err = s.sendDeposit(ctx, m.ID, m.Name, true, 0)
	return state, code, err
}

// newDepositIdentity makes a recovery code and keeps the public half of the
// key pair it gives.
func (s *Service) newDepositIdentity() (string, error) {
	code, err := keydeposit.NewCode()
	if err != nil {
		return "", err
	}
	salt, err := keydeposit.NewSalt()
	if err != nil {
		return "", err
	}
	recipient, err := keydeposit.Recipient(code, salt)
	if err != nil {
		return "", err
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, recipient)
	if err != nil {
		return "", err
	}
	return code, s.store.SetDepositIdentity(enc, salt)
}

// NewDepositCode replaces the recovery code and seals the deposit of every
// keeper for the new one. A keeper out of reach holds a deposit for the old
// code until the next renewal reaches it.
func (s *Service) NewDepositCode(ctx context.Context) (string, error) {
	s.depositMu.Lock()
	defer s.depositMu.Unlock()
	code, err := s.newDepositIdentity()
	if err != nil {
		return "", err
	}
	s.renewDepositsLocked(ctx)
	return code, nil
}

// RenewDeposits sends every keeper a fresh deposit whose content changed
// since the last one, and learns what became of the deposits it sent.
func (s *Service) RenewDeposits(ctx context.Context) {
	s.depositMu.Lock()
	defer s.depositMu.Unlock()
	s.renewDepositsLocked(ctx)
}

func (s *Service) renewDepositsLocked(ctx context.Context) {
	sent, err := s.store.SentDeposits()
	if err != nil || len(sent) == 0 {
		return
	}
	identity, found, err := s.store.DepositIdentity()
	if err != nil || !found {
		return
	}
	recipient, err := s.depositRecipient(identity)
	if err != nil {
		log.Printf("api: deposits: the recovery code's public key does not open: %v", err)
		return
	}
	_, digest, err := s.depositContent(recipient)
	if err != nil {
		log.Printf("api: deposits: %v", scrubError(err))
		return
	}
	for _, d := range sent {
		if _, ok := s.reachableMember(d.MemberID); !ok {
			continue
		}
		// The keeper says what it holds, so a deposit deleted there shows here.
		var held peerDepositList
		if err := s.callMember(ctx, d.MemberID, http.MethodGet, "/api/group/peer/deposits", nil, &held); err != nil {
			continue
		}
		switch {
		case held.Mine == store.DepositDeclined:
			d.State = store.DepositDeclined
			if err := s.store.SaveSentDeposit(d); err != nil {
				log.Printf("api: deposits: %v", err)
			}
		case d.State == store.DepositHeld && (held.Mine != store.DepositHeld || held.MineDigest != d.HeldDigest):
			// Nothing this install or the keeper's person did leaves the
			// keeper with other bytes or none, so a member did it in this
			// install's name.
			d.State, d.ChangedAt = store.DepositChanged, time.Now().Unix()
			if err := s.store.SaveSentDeposit(d); err != nil {
				log.Printf("api: deposits: %v", err)
			}
			s.keyEvent(ctx, keyEventChanged, errDepositChanged)
			s.resendDeposit(ctx, d)
		case held.Mine == store.DepositHeld && d.Digest == digest && d.State == store.DepositHeld:
		default:
			s.resendDeposit(ctx, d)
		}
	}
}

// resendDeposit sends a keeper a fresh deposit outside of a person asking. A
// keeper that still refuses it is no event: the record says so already.
func (s *Service) resendDeposit(ctx context.Context, d store.SentDeposit) {
	state, err := s.sendDeposit(ctx, d.MemberID, d.MemberName, false, d.ChangedAt)
	if err != nil || state == store.DepositHeld {
		s.keyEvent(ctx, keyEventRenew, err)
	}
}

// renewDepositsLoop keeps the deposits current while the server runs.
func (s *Service) renewDepositsLoop() {
	ctx := s.StopContext()
	t := time.NewTicker(depositRenewEvery)
	defer t.Stop()
	for {
		s.RenewDeposits(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// WithdrawDeposit takes this install's deposit back from a member and reports
// whether the member heard. The last keeper gone, the recovery code is
// forgotten too.
func (s *Service) WithdrawDeposit(ctx context.Context, memberID string) (bool, error) {
	s.depositMu.Lock()
	defer s.depositMu.Unlock()
	if err := s.store.DeleteSentDeposit(memberID); err != nil {
		return false, err
	}
	told := false
	if _, ok := s.reachableMember(memberID); ok {
		signer, err := s.nextDepositSigner()
		if err != nil {
			return false, err
		}
		told = s.callMember(ctx, memberID, http.MethodDelete, "/api/group/peer/deposit", signer.prove(keydeposit.ProofDrop, memberID, nil, nil), nil) == nil
	}
	left, err := s.store.SentDeposits()
	if err != nil {
		return told, err
	}
	if len(left) == 0 {
		err = s.store.ClearDepositIdentity()
	}
	return told, err
}

// handlePeerDepositPut takes a member's deposit to keep.
// PUT /api/group/peer/deposit
func (s *Service) handlePeerDepositPut(w http.ResponseWriter, r *http.Request) {
	var in peerDeposit
	if err := json.NewDecoder(io.LimitReader(r.Body, peerDepositMax)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "malformed deposit"})
		return
	}
	owner, ok := s.depositAsker(w, r)
	if !ok {
		return
	}
	if len(in.Deposit) == 0 || len(in.Deposit) > maxDepositBytes || len(in.Salt) == 0 || in.Version != keydeposit.Version {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "this is not a deposit this version keeps"})
		return
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	claim := keydeposit.Claim{Action: keydeposit.ProofKeep, OwnerID: owner, KeeperID: g.InstanceID, Counter: in.Counter, Deposit: in.Deposit, Salt: in.Salt}
	if !keydeposit.Proven(in.Key, in.Proof, claim) {
		refuseUnprovenDeposit(w)
		return
	}
	state, first, err := s.store.KeepDeposit(store.KeptDeposit{
		OwnerID: owner, OwnerName: clipRoleName(in.Name), Deposit: in.Deposit, Salt: in.Salt, Version: in.Version,
		OwnerKey: in.Key, Counter: in.Counter,
	}, in.Renew, maxKeptDeposits)
	switch {
	case errors.Is(err, store.ErrDepositNotOwner):
		writeJSON(w, http.StatusOK, peerDepositState{OK: true, State: store.DepositChanged})
		return
	case errors.Is(err, store.ErrDepositsFull):
		writeJSON(w, http.StatusOK, peerDepositState{OK: true, State: depositFull})
		return
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if state == store.DepositHeld {
		kind := keyEventRenew
		if first || in.Renew {
			kind = keyEventKeep
		}
		s.keyEvent(r.Context(), kind, nil)
	}
	writeJSON(w, http.StatusOK, peerDepositState{OK: true, State: state})
}

// depositAsker names the member behind a call about a deposit, which has to
// be a member the group has in reach. It answers the refusal itself.
func (s *Service) depositAsker(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := s.roleAsker(r)
	if !ok {
		refuseUnnamedPeer(w)
		return "", false
	}
	if s.memberPresent(id) != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "no member of this group by that id"})
		return "", false
	}
	return id, true
}

func refuseUnprovenDeposit(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "the call is not signed by the install the deposit belongs to"})
}

// handlePeerDepositDelete forgets the deposit of the member that calls, which
// proves itself with the key the deposit came with.
// DELETE /api/group/peer/deposit
func (s *Service) handlePeerDepositDelete(w http.ResponseWriter, r *http.Request) {
	var in peerProof
	if !decodePeerRole(w, r, &in) {
		return
	}
	owner, ok := s.depositAsker(w, r)
	if !ok {
		return
	}
	kept, found, err := s.store.KeptDeposit(owner)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !found {
		writeJSON(w, http.StatusOK, okEnvelope(nil))
		return
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	claim := keydeposit.Claim{Action: keydeposit.ProofDrop, OwnerID: owner, KeeperID: g.InstanceID, Counter: in.Counter}
	if len(kept.OwnerKey) > 0 && (in.Counter <= kept.Counter || !keydeposit.Proven(kept.OwnerKey, in.Proof, claim)) {
		refuseUnprovenDeposit(w)
		return
	}
	had, err := s.store.DropDeposit(owner)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if had {
		s.keyEvent(r.Context(), keyEventGone, nil)
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// peerDepositEntry names a deposit a member keeps without anything of it.
// Counter is the one the deposit was signed with, by which a new install
// tells the newest of several.
type peerDepositEntry struct {
	OwnerID   string `json:"ownerId"`
	OwnerName string `json:"ownerName"`
	UpdatedAt int64  `json:"updatedAt"`
	Counter   int64  `json:"counter"`
}

// peerDepositList is what a member keeps. Mine is the state of the caller's
// own deposit there, empty for none, and MineDigest names the sealed bytes
// held, so the caller can tell them from the ones it sent.
type peerDepositList struct {
	OK         bool               `json:"ok"`
	Deposits   []peerDepositEntry `json:"deposits"`
	Mine       string             `json:"mine,omitempty"`
	MineDigest string             `json:"mineDigest,omitempty"`
}

// handlePeerDeposits lists whose deposits this instance keeps, for a new
// install that looks for the one of the server it replaces.
// GET /api/group/peer/deposits
func (s *Service) handlePeerDeposits(w http.ResponseWriter, r *http.Request) {
	caller, ok := s.roleAsker(r)
	if !ok {
		refuseUnnamedPeer(w)
		return
	}
	kept, err := s.store.KeptDeposits()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	out := peerDepositList{OK: true, Deposits: []peerDepositEntry{}}
	for _, d := range kept {
		if d.OwnerID == caller {
			out.Mine = d.State
			if d.State == store.DepositHeld {
				out.MineDigest = heldDigest(d.Deposit)
			}
		}
		if d.State == store.DepositHeld {
			out.Deposits = append(out.Deposits, peerDepositEntry{OwnerID: d.OwnerID, OwnerName: d.OwnerName, UpdatedAt: d.UpdatedAt, Counter: d.Counter})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// peerReleaseAsk is a member's request for the deposit of another. Renew is
// set when a person there asked, which opens a request. Without it the call
// only reads where the last request stands.
type peerReleaseAsk struct {
	OwnerID string `json:"ownerId"`
	Name    string `json:"name"`
	Renew   bool   `json:"renew"`
}

// peerRelease is the answer: where the request stands, and the deposit once
// a person here allowed it. The deposit is as sealed as it is kept.
type peerRelease struct {
	OK      bool   `json:"ok"`
	State   string `json:"state"`
	Deposit []byte `json:"deposit,omitempty"`
	Salt    []byte `json:"salt,omitempty"`
	Version int    `json:"version,omitempty"`
}

// handlePeerDepositRelease records a member's request for a deposit and hands
// the deposit over once a person here has allowed that request. Asking again
// is how the member finds out.
// POST /api/group/peer/deposit/release
func (s *Service) handlePeerDepositRelease(w http.ResponseWriter, r *http.Request) {
	var in peerReleaseAsk
	if !decodePeerRole(w, r, &in) {
		return
	}
	requester, ok := s.depositAsker(w, r)
	if !ok {
		return
	}
	// The install a deposit belongs to has its key. A request in its name
	// would show the keeper's person a member they have known for long.
	if requester == in.OwnerID {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "an instance cannot ask for its own deposit"})
		return
	}
	kept, found, err := s.store.KeptDeposit(in.OwnerID)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !found || kept.State != store.DepositHeld {
		writeJSON(w, http.StatusOK, peerRelease{OK: true, State: releaseNone})
		return
	}
	now := time.Now()
	req, created, err := s.store.AskKeyRelease(in.OwnerID, requester, clipRoleName(in.Name), in.Renew, now, releaseLimits)
	if errors.Is(err, store.ErrReleaseLimit) {
		s.keyEvent(r.Context(), keyEventAsk, err)
		writeJSON(w, http.StatusOK, peerRelease{OK: true, State: releaseLimited})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if created {
		s.keyEvent(r.Context(), keyEventAsk, nil)
	}
	out := peerRelease{OK: true, State: cmp.Or(req.State, releaseUnasked)}
	if req.State == store.ReleaseAllowed {
		collected, err := s.store.CollectKeyRelease(in.OwnerID, requester, now, releaseLimits)
		if err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		if collected {
			out.Deposit, out.Salt, out.Version = kept.Deposit, kept.Salt, kept.Version
			s.keyEvent(r.Context(), keyEventRelease, nil)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// DecideRelease gives a waiting release request a person's answer.
func (s *Service) DecideRelease(ctx context.Context, id string, allow bool) (store.KeyRelease, error) {
	state, kind := store.ReleaseDeclined, keyEventRefuse
	if allow {
		state, kind = store.ReleaseAllowed, keyEventAllow
	}
	req, err := s.store.DecideKeyRelease(id, state, time.Now(), releaseLimits)
	if err == nil {
		s.keyEvent(ctx, kind, nil)
	}
	return req, err
}

// DropKeptDeposit deletes the deposit this instance keeps for a member. The
// member learns of it at its next renewal and has to ask again.
func (s *Service) DropKeptDeposit(ctx context.Context, ownerID string) error {
	had, err := s.store.DeclineDeposit(ownerID)
	if err == nil && had {
		s.keyEvent(ctx, keyEventDrop, nil)
	}
	return err
}
