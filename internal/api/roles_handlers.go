package api

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// roleRefreshEvery is how often an open Instances page makes this instance
// ask its members again where its own requests stand.
const roleRefreshEvery = 30 * time.Second

// registerRoleRoutes adds what the Instances page does with roles: read them
// for every member in both directions, ask a member for one, take a request
// back, and answer a member's request.
func (h *Handler) registerRoleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/group/roles", h.handleListRoles)
	mux.HandleFunc("PUT /api/group/members/{id}/roles/{role}", h.handleAskRole)
	mux.HandleFunc("DELETE /api/group/members/{id}/roles/{role}", h.handleWithdrawRole)
	mux.HandleFunc("POST /api/group/roles/requests/{id}", h.handleDecideRole)
}

// roleRequestView is a Receiver or Fetcher request as a card shows it.
type roleRequestView struct {
	ID        string   `json:"id"`
	State     string   `json:"state"`
	Sections  []string `json:"sections"`
	Store     string   `json:"store"`
	AskedAt   string   `json:"askedAt"`
	DecidedAt string   `json:"decidedAt"`
	DecidedBy string   `json:"decidedBy"`
}

func roleRequestViewOf(req store.RoleRequest) *roleRequestView {
	sections := req.Sections
	if sections == nil {
		sections = []string{}
	}
	return &roleRequestView{
		ID: req.ID, State: req.State, Sections: sections, Store: req.Store,
		AskedAt: unixRFC3339(req.AskedAt), DecidedAt: unixRFC3339(req.DecidedAt), DecidedBy: req.DecidedBy,
	}
}

// zfsRoleItemView is one ZFS item's replica request. ID is the item here for
// a request this instance sent, and the receive request for one it got.
type zfsRoleItemView struct {
	ID      string `json:"id"`
	Dataset string `json:"dataset"`
	State   string `json:"state"`
}

// zfsRoleView is the ZFS server role between two instances: every item asks
// on its own, and State sums them up for the tile.
type zfsRoleView struct {
	State string            `json:"state"`
	Items []zfsRoleItemView `json:"items"`
}

// roleSideView holds the three roles in one direction. A role nobody asked
// for is null.
type roleSideView struct {
	Receiver *roleRequestView `json:"receiver"`
	Fetcher  *roleRequestView `json:"fetcher"`
	ZFS      *zfsRoleView     `json:"zfs"`
}

// memberRolesView is what one member's card draws its tiles and its request
// badge from. ByMember is what the member does for this instance, ForMember
// what this instance does for the member.
type memberRolesView struct {
	MemberID  string `json:"memberId"`
	Name      string `json:"name"`
	Reachable bool   `json:"reachable"`
	// Answers is false for a member from before role requests: asking it
	// takes effect at once, since nobody there can answer.
	Answers   bool         `json:"answers"`
	ByMember  roleSideView `json:"byMember"`
	ForMember roleSideView `json:"forMember"`
	// Asks counts the member's requests that wait for an answer here.
	Asks int `json:"asks"`
}

type roleHolderView struct {
	MemberID string `json:"memberId"`
	Name     string `json:"name"`
}

// selfRolesView lists for whom this instance is Receiver, Fetcher and ZFS
// server.
type selfRolesView struct {
	Receiver []roleHolderView `json:"receiver"`
	Fetcher  []roleHolderView `json:"fetcher"`
	ZFS      []roleHolderView `json:"zfs"`
}

// zfsRoleStates orders the states of a ZFS role's items by which one the
// tile shows: an item still waiting keeps it on "request sent", and one
// allowed item makes it active whatever the others say.
var zfsRoleStates = []string{
	store.ZFSReceiveAsked, store.ZFSReceiveAllowed, store.ZFSReceiveOff, store.ZFSReceiveRevoked, store.ZFSReceiveRefused,
}

func (v *zfsRoleView) add(id, dataset, state string) *zfsRoleView {
	if v == nil {
		v = &zfsRoleView{}
	}
	// An item that has not heard back yet waits like one that was told so.
	if state == "" {
		state = store.ZFSReceiveAsked
	}
	v.Items = append(v.Items, zfsRoleItemView{ID: id, Dataset: dataset, State: state})
	if v.State == "" || slices.Index(zfsRoleStates, state) < slices.Index(zfsRoleStates, v.State) {
		v.State = state
	}
	return v
}

// rolesOverview reads the three roles for every member in both directions:
// the members in reach, and those a request, a receive slot or a replica
// target still names.
func (s *Service) rolesOverview() ([]memberRolesView, selfRolesView, error) {
	byID := map[string]*memberRolesView{}
	entry := func(id, name string) *memberRolesView {
		e := byID[id]
		if e == nil {
			e = &memberRolesView{MemberID: id, Answers: true}
			byID[id] = e
		}
		if e.Name == "" {
			e.Name = name
		}
		return e
	}
	for _, m := range s.pairing().Members() {
		if m.Kind != "" {
			continue
		}
		e := entry(m.ID, m.Name)
		e.Reachable, e.Answers = true, !predatesRoleRequests(m.Version)
	}

	reqs, err := s.store.ListRoleRequests()
	if err != nil {
		return nil, selfRolesView{}, err
	}
	for _, req := range reqs {
		e := entry(req.MemberID, req.MemberName)
		side := &e.ByMember
		if req.Direction == store.RoleRequestIn {
			side = &e.ForMember
			if req.State == store.RoleAsked {
				e.Asks++
			}
		}
		if req.Role == store.RoleReceiver {
			side.Receiver = roleRequestViewOf(req)
		} else {
			side.Fetcher = roleRequestViewOf(req)
		}
	}

	slots, err := s.store.ListZFSReceiveSlots()
	if err != nil {
		return nil, selfRolesView{}, err
	}
	for _, slot := range slots {
		e := entry(slot.PeerID, slot.PeerName)
		e.ForMember.ZFS = e.ForMember.ZFS.add(slot.ID, slot.Dataset, slot.State)
		if slot.State == store.ZFSReceiveAsked {
			e.Asks++
		}
	}
	items, err := s.store.ListZFSDatasets()
	if err != nil {
		return nil, selfRolesView{}, err
	}
	for _, item := range items {
		if item.Replica.TargetKind == store.ZFSReplicaTargetPeer {
			e := entry(item.Replica.TargetID, "")
			e.ByMember.ZFS = e.ByMember.ZFS.add(item.ID, item.Dataset, item.Replica.Peer.State)
		}
	}

	peers, err := s.store.ListFleetPeers()
	if err != nil {
		return nil, selfRolesView{}, err
	}
	for _, p := range peers {
		if e := byID[p.MemberID]; e != nil && e.Name == "" {
			e.Name = p.Name
		}
	}

	members := make([]memberRolesView, 0, len(byID))
	for _, e := range byID {
		members = append(members, *e)
	}
	slices.SortFunc(members, func(a, b memberRolesView) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.MemberID, b.MemberID))
	})
	self := selfRolesView{Receiver: []roleHolderView{}, Fetcher: []roleHolderView{}, ZFS: []roleHolderView{}}
	for _, m := range members {
		holder := roleHolderView{MemberID: m.MemberID, Name: m.Name}
		if r := m.ForMember.Receiver; r != nil && r.State == store.RoleAllowed {
			self.Receiver = append(self.Receiver, holder)
		}
		if f := m.ForMember.Fetcher; f != nil && f.State == store.RoleAllowed {
			self.Fetcher = append(self.Fetcher, holder)
		}
		if z := m.ForMember.ZFS; z != nil && z.State == store.ZFSReceiveAllowed {
			self.ZFS = append(self.ZFS, holder)
		}
	}
	return members, self, nil
}

// refreshSentRolesSoon starts asking the members again in the background,
// unless that ran a moment ago.
func (s *Service) refreshSentRolesSoon() {
	now := time.Now().UnixNano()
	last := s.roleRefreshedAt.Load()
	if now-last < int64(roleRefreshEvery) || !s.roleRefreshedAt.CompareAndSwap(last, now) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(s.StopContext(), time.Minute)
		defer cancel()
		s.refreshSentRoles(ctx)
	}()
}

// handleListRoles answers the Instances page with every member's roles.
// GET /api/group/roles
func (h *Handler) handleListRoles(w http.ResponseWriter, _ *http.Request) {
	members, self, err := h.svc.rolesOverview()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.svc.refreshSentRolesSoon()
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"members": members, "self": self}))
}

// handleAskRole asks a member to be this instance's Receiver or Fetcher, or
// changes the sections of a request already sent.
// PUT /api/group/members/{id}/roles/{role}
func (h *Handler) handleAskRole(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Sections []string `json:"sections"`
		Store    string   `json:"store"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	role := r.PathValue("role")
	sections, where, msg := checkRoleAsk(role, body.Sections, body.Store)
	if msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	m, ok := h.svc.reachableMember(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusOK, failEnvelope(errMemberGone))
		return
	}
	req, err := h.svc.askRole(r.Context(), m, role, sections, where)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"request": roleRequestViewOf(req)}))
}

// handleWithdrawRole takes back a request this instance sent. told says
// whether the member heard; the request is gone here either way.
// DELETE /api/group/members/{id}/roles/{role}
func (h *Handler) handleWithdrawRole(w http.ResponseWriter, r *http.Request) {
	role := r.PathValue("role")
	if role != store.RoleReceiver && role != store.RoleFetcher {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the role must be receiver or fetcher"})
		return
	}
	told, err := h.svc.withdrawRole(r.Context(), r.PathValue("id"), role)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"told": told}))
}

// handleDecideRole allows or refuses a member's request, or revokes one that
// was allowed. told says whether the member heard the answer.
// POST /api/group/roles/requests/{id}
func (h *Handler) handleDecideRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validRunID(id) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid request id"})
		return
	}
	// An allow names the sections the page showed, since the member can ask
	// for others while a person looks.
	var body struct {
		Decision string   `json:"decision"`
		Sections []string `json:"sections"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	state, ok := map[string]string{"allow": store.RoleAllowed, "refuse": store.RoleRefused, "revoke": store.RoleRevoked}[body.Decision]
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the decision must be allow, refuse or revoke"})
		return
	}
	if state == store.RoleAllowed && len(body.Sections) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "an allow names the sections it was shown"})
		return
	}
	req, told, err := h.svc.decideRole(r.Context(), id, store.RoleDecision{State: state, Sections: body.Sections})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such role request"})
	case errors.Is(err, store.ErrRoleRequestMove):
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the request cannot take that answer from where it stands"})
	case errors.Is(err, store.ErrRoleRequestChanged):
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "request-changed", "error": "the other instance asked for other sections since the request was shown"})
	case errors.Is(err, errRoleNeedsReceiver):
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, "no-receiver"))
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"request": roleRequestViewOf(req), "told": told}))
	}
}
