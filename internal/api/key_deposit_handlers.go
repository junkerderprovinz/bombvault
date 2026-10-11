package api

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// registerDepositRoutes adds what the Instances page does with deposits
// besides asking a member to keep one, which goes through the role routes,
// and the two routes a new install reaches a deposit by.
func (h *Handler) registerDepositRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/group/deposits", h.handleListDeposits)
	mux.HandleFunc("DELETE /api/group/deposits/{owner}", h.handleDropKeptDeposit)
	mux.HandleFunc("POST /api/group/deposits/releases/{id}", h.handleDecideRelease)
	mux.HandleFunc("POST /api/group/deposit/code", h.handleNewDepositCode)
	mux.HandleFunc("POST /api/group/deposit/renew", h.handleRenewDeposits)
	mux.HandleFunc("GET /api/emergency/deposits", h.handleEmergencyDeposits)
	mux.HandleFunc("POST /api/emergency/group", h.handleEmergencyGroup)
}

// keeperRoleView is the keeper role in one direction, as a tile shows it. On
// the side of what this instance keeps, Asks counts the requests for that
// deposit that wait for an answer here.
type keeperRoleView struct {
	State string `json:"state"`
	At    string `json:"at"`
	Asks  int    `json:"asks,omitempty"`
	// ChangedAt is when the member was last found holding something this
	// install did not send, on the side of what the member keeps.
	ChangedAt string `json:"changedAt,omitempty"`
}

type sentDepositView struct {
	MemberID  string `json:"memberId"`
	Name      string `json:"name"`
	State     string `json:"state"`
	SentAt    string `json:"sentAt"`
	ChangedAt string `json:"changedAt"`
}

type keptDepositView struct {
	OwnerID   string `json:"ownerId"`
	OwnerName string `json:"ownerName"`
	State     string `json:"state"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// releaseView is a request for a deposit as the keeper's person decides on
// it: who asks, since when this instance has had that instance in its group,
// and how often it asked.
type releaseView struct {
	ID            string `json:"id"`
	OwnerID       string `json:"ownerId"`
	OwnerName     string `json:"ownerName"`
	RequesterID   string `json:"requesterId"`
	RequesterName string `json:"requesterName"`
	State         string `json:"state"`
	AskedAt       string `json:"askedAt"`
	DecidedAt     string `json:"decidedAt"`
	ReleasedAt    string `json:"releasedAt"`
	// KnownSince is empty for an instance this one never listed as a member.
	KnownSince string `json:"knownSince"`
	// Requests counts every request of the asking instance, this one included,
	// and RequestsLastHour those of the last hour.
	Requests         int `json:"requests"`
	RequestsLastHour int `json:"requestsLastHour"`
}

// depositsView is both sides of the deposits: what this install gave its
// keepers and what it keeps for others, with the requests to hand that out.
type depositsView struct {
	// CodeSince is when this install's recovery code was made, empty while it
	// has none.
	CodeSince string            `json:"codeSince"`
	Sent      []sentDepositView `json:"sent"`
	Kept      []keptDepositView `json:"kept"`
	Releases  []releaseView     `json:"releases"`
}

// releaseState is the state of a request with its time limits applied, which
// the store writes down only when the request is touched again.
func releaseState(k store.KeyRelease, now time.Time) string {
	switch {
	case k.State == store.ReleaseAsked && k.AskedAt < now.Add(-releaseLimits.Answer).Unix(),
		k.State == store.ReleaseAllowed && k.DecidedAt < now.Add(-releaseLimits.Collect).Unix():
		return store.ReleaseExpired
	}
	return k.State
}

func (s *Service) depositsOverview() (depositsView, error) {
	view := depositsView{Sent: []sentDepositView{}, Kept: []keptDepositView{}, Releases: []releaseView{}}
	identity, found, err := s.store.DepositIdentity()
	if err != nil {
		return view, err
	}
	if found {
		view.CodeSince = unixRFC3339(identity.CreatedAt)
	}
	sent, err := s.store.SentDeposits()
	if err != nil {
		return view, err
	}
	// A keeper holds an older deposit from the moment the kit says something
	// else, which is before the next renewal finds out.
	current := ""
	if found && len(sent) > 0 {
		if recipient, err := s.depositRecipient(identity); err == nil {
			if _, digest, err := s.depositContent(recipient); err == nil {
				current = digest
			}
		}
	}
	for _, d := range sent {
		if d.State == store.DepositHeld && current != "" && d.Digest != current {
			d.State = store.DepositStale
		}
		view.Sent = append(view.Sent, sentDepositView{
			MemberID: d.MemberID, Name: d.MemberName, State: d.State,
			SentAt: unixRFC3339(d.SentAt), ChangedAt: unixRFC3339(d.ChangedAt),
		})
	}
	kept, err := s.store.KeptDeposits()
	if err != nil {
		return view, err
	}
	owners := map[string]string{}
	for _, d := range kept {
		owners[d.OwnerID] = d.OwnerName
		view.Kept = append(view.Kept, keptDepositView{
			OwnerID: d.OwnerID, OwnerName: d.OwnerName, State: d.State,
			CreatedAt: unixRFC3339(d.CreatedAt), UpdatedAt: unixRFC3339(d.UpdatedAt),
		})
	}
	releases, err := s.store.KeyReleases(200)
	if err != nil {
		return view, err
	}
	peers, err := s.store.ListFleetPeers()
	if err != nil {
		return view, err
	}
	known := map[string]int64{}
	for _, p := range peers {
		if p.MemberID != "" {
			known[p.MemberID] = p.CreatedAt
		}
	}
	now := time.Now()
	total, lastHour := map[string]int{}, map[string]int{}
	for _, k := range releases {
		total[k.RequesterID]++
		if k.AskedAt > now.Add(-time.Hour).Unix() {
			lastHour[k.RequesterID]++
		}
	}
	for _, k := range releases {
		view.Releases = append(view.Releases, releaseView{
			ID: k.ID, OwnerID: k.OwnerID, OwnerName: cmp.Or(k.OwnerName, owners[k.OwnerID]),
			RequesterID: k.RequesterID, RequesterName: k.RequesterName, State: releaseState(k, now),
			AskedAt: unixRFC3339(k.AskedAt), DecidedAt: unixRFC3339(k.DecidedAt), ReleasedAt: unixRFC3339(k.ReleasedAt),
			KnownSince: unixRFC3339(known[k.RequesterID]),
			Requests:   total[k.RequesterID], RequestsLastHour: lastHour[k.RequesterID],
		})
	}
	return view, nil
}

// addKeeperRoles puts the keeper role into the roles of every member: what
// the member keeps for this install, what this install keeps for the member,
// and the requests that wait for an answer here.
func (s *Service) addKeeperRoles(entry func(id, name string) *memberRolesView) error {
	view, err := s.depositsOverview()
	if err != nil {
		return err
	}
	for _, d := range view.Sent {
		entry(d.MemberID, d.Name).ByMember.Keeper = &keeperRoleView{State: d.State, At: d.SentAt, ChangedAt: d.ChangedAt}
	}
	for _, d := range view.Kept {
		entry(d.OwnerID, d.OwnerName).ForMember.Keeper = &keeperRoleView{State: d.State, At: d.UpdatedAt}
	}
	for _, k := range view.Releases {
		if k.State != store.ReleaseAsked {
			continue
		}
		entry(k.RequesterID, k.RequesterName).Asks++
		if keeper := entry(k.OwnerID, k.OwnerName).ForMember.Keeper; keeper != nil {
			keeper.Asks++
		}
	}
	return nil
}

// handleListDeposits answers the Instances page with both sides of the
// deposits.
// GET /api/group/deposits
func (h *Handler) handleListDeposits(w http.ResponseWriter, _ *http.Request) {
	view, err := h.svc.depositsOverview()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"deposits": view}))
}

// askKeeper serves PUT /api/group/members/{id}/roles/keeper: the member is
// given this install's deposit. The answer carries the recovery code when
// this made it, which is the one time it is shown.
func (h *Handler) askKeeper(w http.ResponseWriter, r *http.Request) {
	if !crossOriginGuard(w, r) || !h.requireAuthForSecrets(w, r, "depositing the key") {
		return
	}
	state, code, err := h.svc.MakeDeposit(r.Context(), r.PathValue("id"))
	h.svc.keyEvent(r.Context(), keyEventDeposit, err)
	if err != nil {
		depositFail(w, err)
		return
	}
	answer := map[string]any{"keeper": keeperRoleView{State: state, At: unixRFC3339(time.Now().Unix())}}
	if code != "" {
		answer["recoveryCode"] = code
	}
	writeJSON(w, http.StatusOK, okEnvelope(answer))
}

// withdrawKeeper serves DELETE /api/group/members/{id}/roles/keeper.
func (h *Handler) withdrawKeeper(w http.ResponseWriter, r *http.Request) {
	if !crossOriginGuard(w, r) || !h.requireSignIn(w, r, "taking a deposit back") {
		return
	}
	told, codeGone, err := h.svc.WithdrawDeposit(r.Context(), r.PathValue("id"))
	h.svc.keyEvent(r.Context(), keyEventWithdraw, err)
	if err != nil {
		depositFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"told": told, "codeGone": codeGone}))
}

// handleNewDepositCode replaces the recovery code and answers with the new
// one, which is shown this once.
// POST /api/group/deposit/code
func (h *Handler) handleNewDepositCode(w http.ResponseWriter, r *http.Request) {
	if !crossOriginGuard(w, r) || !h.requireAuthForSecrets(w, r, "making a new recovery code") {
		return
	}
	code, err := h.svc.NewDepositCode(r.Context())
	h.svc.keyEvent(r.Context(), keyEventCode, err)
	if err != nil {
		depositFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"recoveryCode": code}))
}

// handleRenewDeposits sends the keepers a fresh deposit where the content
// changed and answers with where the deposits stand.
// POST /api/group/deposit/renew
func (h *Handler) handleRenewDeposits(w http.ResponseWriter, r *http.Request) {
	if !crossOriginGuard(w, r) {
		return
	}
	h.svc.RenewDeposits(r.Context())
	h.handleListDeposits(w, r)
}

// handleDecideRelease allows or refuses a request for a deposit. Neither is
// for the session of a paired phone, and allowing, which hands key material
// out, is for nobody on a server without a login password.
// POST /api/group/deposits/releases/{id}
func (h *Handler) handleDecideRelease(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Decision string `json:"decision"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.Decision != "allow" && body.Decision != "refuse" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the decision must be allow or refuse"})
		return
	}
	if body.Decision == "allow" && !h.requireAuthForSecrets(w, r, "allowing a deposit to be handed out") {
		return
	}
	if !h.requireSignIn(w, r, "answering a request for a deposit") {
		return
	}
	req, err := h.svc.DecideRelease(r.Context(), r.PathValue("id"), body.Decision == "allow")
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeJSON(w, http.StatusNotFound, codedFailEnvelope(errors.New("no such request"), "not-found"))
	case err != nil:
		depositFail(w, err)
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"state": req.State}))
	}
}

// handleDropKeptDeposit deletes the deposit this instance keeps for a member.
// DELETE /api/group/deposits/{owner}
func (h *Handler) handleDropKeptDeposit(w http.ResponseWriter, r *http.Request) {
	if !crossOriginGuard(w, r) {
		return
	}
	owner := r.PathValue("owner")
	if !zfs.ValidSourceID(owner) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid instance id"})
		return
	}
	if err := h.svc.DropKeptDeposit(r.Context(), owner); err != nil {
		depositFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// offeredDeposit is a deposit a member keeps, as a new install chooses from.
// Newest is false for one that another member lists a newer deposit of the
// same install than.
type offeredDeposit struct {
	MemberID   string `json:"memberId"`
	MemberName string `json:"memberName"`
	OwnerID    string `json:"ownerId"`
	OwnerName  string `json:"ownerName"`
	UpdatedAt  string `json:"updatedAt"`
	Newest     bool   `json:"newest"`
	counter    int64
}

// OfferedDeposits asks every member in reach which deposits it keeps. silent
// names the members that did not answer, older ones among them.
func (s *Service) OfferedDeposits(ctx context.Context) (offered []offeredDeposit, silent []string) {
	offered, silent = []offeredDeposit{}, []string{}
	for _, m := range s.pairing().Members() {
		if m.Kind != "" {
			continue
		}
		var list peerDepositList
		if predatesRoleRequests(m.Version) || s.callMember(ctx, m.ID, http.MethodGet, "/api/group/peer/deposits", nil, &list) != nil {
			silent = append(silent, m.Name)
			continue
		}
		for _, d := range list.Deposits {
			offered = append(offered, offeredDeposit{
				MemberID: m.ID, MemberName: m.Name, OwnerID: d.OwnerID, OwnerName: d.OwnerName, UpdatedAt: unixRFC3339(d.UpdatedAt),
				counter: d.Counter,
			})
		}
	}
	newest := map[string]int64{}
	for _, d := range offered {
		newest[d.OwnerID] = max(newest[d.OwnerID], d.counter)
	}
	for i, d := range offered {
		offered[i].Newest = d.counter == newest[d.OwnerID]
	}
	slices.SortFunc(offered, func(a, b offeredDeposit) int {
		return cmp.Or(cmp.Compare(a.OwnerName, b.OwnerName), cmp.Compare(a.MemberName, b.MemberName))
	})
	return offered, silent
}

// handleEmergencyDeposits lists the deposits the group keeps.
// GET /api/emergency/deposits
func (h *Handler) handleEmergencyDeposits(w http.ResponseWriter, r *http.Request) {
	offered, silent := h.svc.OfferedDeposits(r.Context())
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"deposits": offered, "silent": silent}))
}

// collectedDeposit is a deposit a keeper handed over, held in memory until
// the code opens it or the time of an emergency is up. A mistyped code can
// then be typed again without asking the keeper a second time.
type collectedDeposit struct {
	memberID, ownerID string
	deposit, salt     []byte
	expires           time.Time
}

// EmergencyFromGroup asks a member for the deposit of the install this one
// replaces and, once a person there has allowed it, opens the deposit with
// the recovery code and starts an emergency from the kit inside. The code is
// used here and goes nowhere. ask says a person pressed the button, which
// opens a request; without it the call reads where the last one stands.
// release is the state of the request.
func (s *Service) EmergencyFromGroup(ctx context.Context, memberID, ownerID, code string, ask bool) (release string, err error) {
	// Checked before anything is asked: a code of the wrong shape would use up
	// a request for nothing.
	if err := keydeposit.CheckCode(code); err != nil {
		return "", err
	}
	got, release, err := s.collectDeposit(ctx, memberID, ownerID, ask)
	if err != nil || got == nil {
		return release, err
	}
	plain, err := keydeposit.Open(got.deposit, code, got.salt)
	if err != nil {
		return store.ReleaseAllowed, err
	}
	content, err := keydeposit.Unpack(plain)
	if err != nil {
		return store.ReleaseAllowed, err
	}
	// The keeper says whose deposit it hands over. What counts is what the
	// install sealed into it.
	if content.OwnerID != ownerID {
		return store.ReleaseAllowed, errDepositOtherOwner
	}
	kit, err := readRecoveryKit(content.Kit)
	if err != nil {
		return store.ReleaseAllowed, err
	}
	kit.Server, kit.WrittenAt = content.OwnerName, content.SealedAt
	origin := kitOrigin{OwnerID: content.OwnerID, NewerWith: s.newerDepositWith(ctx, ownerID, content.Counter)}
	if err := s.beginEmergencyFromKit(ctx, kit, store.KeyFromGroup, origin); err != nil {
		return store.ReleaseAllowed, err
	}
	s.emergencyMu.Lock()
	s.collected = nil
	s.emergencyMu.Unlock()
	return store.ReleaseAllowed, nil
}

// newerDepositWith names a member that lists a deposit of ownerID newer than
// the one with counter, empty when there is none. A keeper that was out of
// reach at the last renewal holds an older one, and so does a keeper that
// hands back one it should have replaced.
func (s *Service) newerDepositWith(ctx context.Context, ownerID string, counter int64) string {
	offered, _ := s.OfferedDeposits(ctx)
	for _, d := range offered {
		if d.OwnerID == ownerID && d.counter > counter {
			return d.MemberName
		}
	}
	return ""
}

// collectDeposit returns the deposit once the keeper hands it over. Until
// then it returns none and the state the request is in there.
func (s *Service) collectDeposit(ctx context.Context, memberID, ownerID string, ask bool) (*collectedDeposit, string, error) {
	s.emergencyMu.Lock()
	held := s.collected
	s.emergencyMu.Unlock()
	if held != nil && held.memberID == memberID && held.ownerID == ownerID && time.Now().Before(held.expires) {
		return held, store.ReleaseAllowed, nil
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, "", err
	}
	var ans peerRelease
	in := peerReleaseAsk{OwnerID: ownerID, Name: instanceDisplayName(settings), Renew: ask}
	if in.peerProof, err = s.signCall(keydeposit.ProofRelease, memberID, callPayload(in.OwnerID, in.Name, strconv.FormatBool(in.Renew))); err != nil {
		return nil, "", err
	}
	if err := s.callMember(ctx, memberID, http.MethodPost, "/api/group/peer/deposit/release", in, &ans); err != nil {
		return nil, "", err
	}
	switch ans.State {
	case store.ReleaseDeclined:
		return nil, ans.State, errReleaseNo
	case releaseLimited:
		return nil, ans.State, errReleaseLimit
	case releaseNone:
		return nil, ans.State, errNoDeposit
	case store.ReleaseAllowed:
	case store.ReleaseAsked, store.ReleaseDone, store.ReleaseExpired, releaseUnasked:
		return nil, ans.State, nil
	default:
		return nil, "", errMemberUnreadable
	}
	if len(ans.Deposit) == 0 {
		// Another call collected it in the same moment.
		return nil, store.ReleaseDone, nil
	}
	if ans.Version != keydeposit.Version {
		return nil, ans.State, errDepositNewer
	}
	got := &collectedDeposit{memberID: memberID, ownerID: ownerID, deposit: ans.Deposit, salt: ans.Salt, expires: time.Now().Add(emergencyTTL)}
	s.emergencyMu.Lock()
	s.collected = got
	s.emergencyMu.Unlock()
	return got, ans.State, nil
}

// handleEmergencyGroup serves POST /api/emergency/group with the body
// {memberId, ownerId, code, ask}. ask is true when a person pressed the
// button and false for the calls that wait for the keeper's answer. With
// release "asked" the request waits there; with "allowed" the emergency has
// started; with "unasked", "expired" or "released" a person has to ask again.
func (h *Handler) handleEmergencyGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MemberID string `json:"memberId"`
		OwnerID  string `json:"ownerId"`
		Code     string `json:"code"`
		Ask      bool   `json:"ask"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	release, err := h.svc.EmergencyFromGroup(r.Context(), body.MemberID, body.OwnerID, body.Code, body.Ask)
	if err != nil {
		h.svc.keyEvent(r.Context(), keyEventRead, err)
		if code := depositCode(err); code != "" {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "code": code, "release": release})
			return
		}
		kitFail(w, err)
		return
	}
	if release != store.ReleaseAllowed {
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"release": release}))
		return
	}
	h.svc.keyEvent(r.Context(), keyEventRead, nil)
	view, err := h.svc.EmergencyState()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"release": release, "emergency": view}))
}
