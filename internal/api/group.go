package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/mdns"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	kitgroup "github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/lan"
	kitmember "github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
	"github.com/junkerderprovinz/parleyport/kit/relay"
)

// relayConnectPath is where an instance serving a relay answers, on the
// address it already has.
const relayConnectPath = "/relay/connect"

func (s *Service) buildGroup() {
	s.groupOnce.Do(func() {
		s.groupMgr = group.NewManager(http.HandlerFunc(s.servePeer), s.store)
		s.relaySrv = relay.New()
		s.relaySrv.Admit = func(key string) bool { return s.relayServe.Load() && s.groupMgr.AdmitsRelayKey(key) }
		// Behind a trusted reverse proxy the relay counts failed handshakes
		// per client, as the login throttle does, not per proxy.
		proxies := trustedPrefixes(s.cfg.TrustedProxies)
		s.relaySrv.TrustedProxies = func() []netip.Prefix { return proxies }
	})
}

// trustedPrefixes is nets as the relay takes them.
func trustedPrefixes(nets []net.IPNet) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(nets))
	for _, n := range nets {
		addr, ok := netip.AddrFromSlice(n.IP)
		if !ok {
			continue
		}
		bits, _ := n.Mask.Size()
		if addr.Is4In6() && bits >= 96 {
			bits -= 96
		}
		out = append(out, netip.PrefixFrom(addr.Unmap(), bits))
	}
	return out
}

// pairing returns the Manager of this instance's pairing group.
func (s *Service) pairing() *group.Manager {
	s.buildGroup()
	return s.groupMgr
}

// relayServer returns the relay this instance serves when the switch is on.
func (s *Service) relayServer() *relay.Server {
	s.buildGroup()
	return s.relaySrv
}

// StartGroup applies the stored group and starts listening for members on the
// local network. It runs once at boot.
func (s *Service) StartGroup() {
	s.pairing().Start()
	s.applyGroup()
	go s.renewDepositsLoop()
	go s.locateReceiver()
	go s.settleAllEarlierPulls(context.Background())
}

// StopGroup closes the relay connection and the discovery service.
func (s *Service) StopGroup() { s.pairing().Close() }

// groupSecret returns the stored group secret, or nil outside a group.
func (s *Service) groupSecret(g store.GroupState) ([]byte, error) {
	if len(g.SecretEnc) == 0 {
		return nil, nil
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, g.SecretEnc)
	if err != nil {
		return nil, fmt.Errorf("the stored pairing secret could not be opened: %w", err)
	}
	if len(plain) != phrase.SecretLen {
		return nil, errors.New("the stored pairing secret has the wrong length")
	}
	return plain, nil
}

// applyGroup hands the stored group to the Manager. It runs at boot and after
// every change to the group, the relay settings or the instance name.
func (s *Service) applyGroup() {
	g, ok := s.applyGroupConfig()
	if !ok {
		return
	}
	// Connections admitted under the old key or before the switch went off
	// would otherwise stay open until they drop.
	s.relayServe.Store(g.RelayServe)
	s.relayServer().Revoke()
	s.groupApplied.Store(time.Now().UnixNano())
}

// applyGroupConfig hands the group what it runs on from the store and returns
// the state it read.
func (s *Service) applyGroupConfig() (store.GroupState, bool) {
	g, err := s.store.GetGroupState()
	if err != nil {
		log.Printf("group: %v", err)
		return g, false
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("group: %v", err)
		return g, false
	}
	sec, err := s.groupSecret(g)
	if err != nil {
		// Logged, because the instance looks paired while reaching nobody.
		log.Printf("group: %v; pairing is off until the phrase is entered again", err)
	}
	var seed []byte
	if sec != nil {
		if seed, err = s.depositSeed(); err != nil {
			log.Printf("group: the member key: %v", err)
			sec = nil
		}
	}
	s.pairing().Apply(group.Config{
		Secret:     sec,
		Seed:       seed,
		InstanceID: g.InstanceID,
		Name:       instanceDisplayName(settings),
		DirectURL:  s.directURL(g),
		Mode:       group.Mode(g.RelayMode),
		RelayURL:   g.RelayURL,
		Store:      kitStore{s},
		Former:     s.formerRoster,
		OnRemoved:  s.removedFromGroup,
		Keeper:     moveKeeper{s},
		Fortress:   s.fortressNow,
	})
	return g, true
}

// kitStore keeps the group's state as the kit writes it, sealed under the
// APP_KEY.
type kitStore struct{ s *Service }

func (k kitStore) Load() ([]byte, error) {
	enc, err := k.s.store.GroupKitState()
	if err != nil || len(enc) == 0 {
		return nil, err
	}
	return secret.Decrypt(k.s.cfg.AppKey, enc)
}

func (k kitStore) Save(raw []byte) error {
	enc, err := secret.Encrypt(k.s.cfg.AppKey, raw)
	if err != nil {
		return err
	}
	return k.s.store.SetGroupKitState(enc)
}

// formerRoster is every member whose key this instance pinned before the
// group ran on the kit, approved as it was: such a member had proven it holds
// the phrase, and its key is what it signs with now.
func (s *Service) formerRoster() (kitgroup.Roster, error) {
	pinned, err := s.store.MemberKeys()
	if err != nil {
		return kitgroup.Roster{}, err
	}
	peers, err := s.store.ListFleetPeers()
	if err != nil {
		return kitgroup.Roster{}, err
	}
	var r kitgroup.Roster
	for id, key := range pinned {
		kind, name := kitmember.KindInstance, ""
		var seen time.Time
		for _, p := range peers {
			if p.MemberID != id {
				continue
			}
			name, seen = p.Name, time.Unix(lastSeen(p), 0)
			if p.Kind != "" {
				kind = kitmember.KindMobile
			}
		}
		kid := kitmember.ID(kind, key)
		if kid == "" {
			continue
		}
		r.Members = append(r.Members, kitgroup.Member{ID: kid, Kind: kind, Key: key, State: kitgroup.StateApproved, Name: name, LastSeen: seen})
	}
	return r, nil
}

// removedFromGroup leaves the group after another instance took this one out
// of it.
func (s *Service) removedFromGroup() {
	log.Print("group: another instance took this one out of the group")
	if err := s.leaveGroup(); err != nil {
		log.Printf("group: leave: %v", err)
	}
}

// instanceDisplayName is the name other members show this instance under.
func instanceDisplayName(settings store.Settings) string {
	if name := strings.TrimSpace(settings.InstanceName); name != "" {
		return scrubRepoLocation(name)
	}
	return "BombVault"
}

// directURL is where this instance answers members on the local network: the
// address stored in g, learned from a browser or set by hand, or, when
// nothing better is known, its first LAN address on the port it listens on.
// The stored address is what a host or macvlan setup needs a reverse proxy
// hostname for and what a Docker bridge network needs at all, since the
// container's own address is never reachable from outside it.
func (s *Service) directURL(g store.GroupState) string {
	if addr := strings.TrimSpace(g.DirectURL); addr != "" {
		return addr
	}
	ip := lan.LocalIPv4()
	if ip == "" {
		return ""
	}
	if s.cfg.HTTPOnly {
		return "http://" + ip + ":" + strconv.Itoa(s.cfg.Port)
	}
	return "https://" + ip + ":" + strconv.Itoa(s.cfg.HTTPSPort)
}

// learnDirectURL takes the scheme, host and port a browser used to reach
// this instance and stores it as the instance's own direct address.
// localhost, a loopback or a link-local address
// is ignored: none of those mean anything to another instance on the
// network, and storing one would silence the real address underneath it. A
// new address goes into the announce at once, or members on the network keep
// the old one.
func (s *Service) learnDirectURL(r *http.Request) {
	addr := directAddressFromRequest(s.cfg.HTTPOnly, r)
	if addr == "" {
		return
	}
	g, err := s.store.GetGroupState()
	if err != nil || g.DirectURL == addr {
		return
	}
	if err := s.store.LearnGroupDirectURL(addr); err != nil {
		log.Printf("group: learn this instance's direct address: %v", err)
		return
	}
	s.applyGroupConfig()
}

// directAddressFromRequest rebuilds scheme://host from a request the way
// originFor rebuilds an origin for WebAuthn, since both need what the
// browser's address bar actually shows rather than this container's own
// idea of its address.
func directAddressFromRequest(httpOnly bool, r *http.Request) string {
	host := strings.TrimSpace(r.Host)
	if host == "" || isLocalHost(host) {
		return ""
	}
	scheme := "https"
	if httpOnly {
		scheme = "http"
	}
	if fp := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); fp != "" {
		if i := strings.IndexByte(fp, ','); i > 0 {
			fp = strings.TrimSpace(fp[:i])
		}
		if fp == "http" || fp == "https" {
			scheme = fp
		}
	}
	return scheme + "://" + host
}

// isLocalHost reports whether host, as found in a Host header (so possibly
// with a port and IPv6 brackets), names this machine to itself rather than
// an address another instance could dial.
func isLocalHost(host string) bool {
	h := host
	if hh, _, err := net.SplitHostPort(host); err == nil {
		h = hh
	}
	h = strings.Trim(h, "[]")
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && (ip.IsLoopback() || ip.IsLinkLocalUnicast())
}

// servePeer answers one call from a group member. Only the routes in
// peerRoutes exist here, so nothing else on this instance is reachable from
// the group however the main router grows.
func (s *Service) servePeer(w http.ResponseWriter, r *http.Request) {
	s.peerMux().ServeHTTP(w, r)
}

// peerRoute is one call a group member may make.
type peerRoute struct {
	pattern string
	handle  func(s *Service, w http.ResponseWriter, r *http.Request)
}

// peerRoutes is everything a group member can reach on this instance: the
// Fleet scorecard, a storage offer, the receiver this instance runs for the
// group and a login on it, what receiver and pull pairing need, the key this
// instance signs its calls with, who is in the group here and who is away,
// starting a check, a request to replicate a
// ZFS item here, asking this instance to be Receiver or Fetcher with the
// answer and the withdrawal that go with it, a way into the shared
// repository this instance holds, what runs here and how it looks
// for the Android app, a session for a phone of the group and the pages it
// opens away from home, the word that a member leaves the group, what the
// owner of a shared repository says about its upkeep, the keys a phone
// follows the group into a fortress with, and the bare hello a blind probe
// gets.
// Settings, secrets and the phrase are not among them; a member holds the key
// to every backup already, so a session for its phone adds no reach the group
// did not have. A function rather than a variable, since the session route
// leads back here through the group it syncs.
func peerRoutes() []peerRoute {
	return []peerRoute{
		{"GET /api/group/peer/status", (*Service).handlePeerStatus},
		{"POST /api/group/peer/mesh-offer", (*Service).handlePeerMeshOffer},
		{"POST /api/group/peer/receiver", (*Service).handlePeerReceiver},
		{"GET /api/group/peer/pairing", (*Service).handlePeerPairing},
		{"GET /api/group/peer/key", (*Service).handlePeerKey},
		{"GET /api/group/peer/members", (*Service).handlePeerMembers},
		{"POST /api/group/peer/check/{domain}", (*Service).handlePeerCheck},
		{"POST /api/group/peer/zfs-receive", (*Service).handlePeerZFSReceive},
		{"POST /api/group/peer/roles/ask", (*Service).handlePeerRoleAsk},
		{"POST /api/group/peer/roles/answer", (*Service).handlePeerRoleAnswer},
		{"POST /api/group/peer/roles/withdraw", (*Service).handlePeerRoleWithdraw},
		{"POST /api/group/peer/shared-repo/join", (*Service).handlePeerSharedJoin},
		{"POST /api/group/peer/shared-repo/joined", (*Service).handlePeerSharedJoined},
		{"POST /api/group/peer/shared-repo/leave", (*Service).handlePeerSharedLeave},
		{"POST /api/group/peer/shared-repo/answer", (*Service).handlePeerSharedAnswer},
		{"PUT /api/group/peer/deposit", (*Service).handlePeerDepositPut},
		{"DELETE /api/group/peer/deposit", (*Service).handlePeerDepositDelete},
		{"GET /api/group/peer/deposits", (*Service).handlePeerDeposits},
		{"POST /api/group/peer/deposit/release", (*Service).handlePeerDepositRelease},
		{"GET /api/group/peer/activity", (*Service).handlePeerActivity},
		{"GET /api/group/peer/display-prefs", (*Service).handlePeerDisplayPrefs},
		{"POST /api/group/peer/session", (*Service).handlePeerSession},
		{"POST /api/group/peer/page", (*Service).handlePeerPage},
		{"POST /api/group/peer/leave", (*Service).handlePeerLeave},
		{"POST /api/group/peer/shared-upkeep", (*Service).handlePeerSharedUpkeep},
		{"POST /api/group/peer/shared-holder", (*Service).handlePeerSharedHolder},
		{"GET /api/group/peer/moved", (*Service).handleOldMoved},
		{"GET " + group.ProbePath, (*Service).handlePeerHello},
	}
}

func (s *Service) peerMux() http.Handler {
	s.peerMuxOnce.Do(func() {
		mux := http.NewServeMux()
		for _, pr := range peerRoutes() {
			handle := pr.handle
			mux.HandleFunc(pr.pattern, func(w http.ResponseWriter, r *http.Request) { handle(s, w, r) })
		}
		s.peerMuxHandler = mux
	})
	return s.peerMuxHandler
}

// The errors callMember returns. Whatever a member or a relay says about a
// failure stays in this instance's log: their text and status codes are not
// this instance's to show, and a member could otherwise learn through the
// page what an address it announced answers.
var (
	errMemberGone       = errors.New("that instance is not reachable in the group right now")
	errMemberSilent     = errors.New("that instance did not answer")
	errMemberUnreadable = errors.New("that instance sent an answer this one cannot read")
	errMemberRefused    = errors.New("that instance could not do this; its log says why")
)

// callMember asks a member one of the peerRoutes and decodes its JSON answer
// into out.
func (s *Service) callMember(ctx context.Context, memberID, method, path string, in, out any) error {
	var body []byte
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = b
	}
	status, raw, err := s.pairing().Call(ctx, memberID, method, path, body)
	if errors.Is(err, group.ErrNotMember) || errors.Is(err, kitgroup.ErrRemoved) || errors.Is(err, kitgroup.ErrStranger) {
		return errMemberGone
	}
	if err != nil {
		log.Printf("group: %s %s to member %q: %v", method, path, memberID, err)
		return errMemberSilent
	}
	var env struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		log.Printf("group: %s %s to member %q answered HTTP %d without a readable body", method, path, memberID, status)
		return errMemberUnreadable
	}
	if !env.OK {
		log.Printf("group: %s %s to member %q answered HTTP %d: %q", method, path, memberID, status, env.Error)
		return errMemberRefused
	}
	if out != nil && json.Unmarshal(raw, out) != nil {
		return errMemberUnreadable
	}
	return nil
}

// handlePeerStatus is the protection summary a member's Fleet page shows.
// GET /api/group/peer/status
func (s *Service) handlePeerStatus(w http.ResponseWriter, _ *http.Request) {
	settings, err := s.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	domains, err := s.domainStatusFrom(settings)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, fleetStatusResponse{
		OK:           true,
		InstanceName: instanceDisplayName(settings),
		Version:      Version,
		Domains:      domains,
		DirectURL:    s.selfDirectURL(),
		Capabilities: peerCapabilities,
	})
}

// selfDirectURL is this instance's own direct address, fetched fresh so it
// reflects whatever was last learned or set. It is what this instance tells
// a member calling it over the relay, the address exchange a poll or a
// pairing already makes.
func (s *Service) selfDirectURL() string {
	g, err := s.store.GetGroupState()
	if err != nil {
		return ""
	}
	return s.directURL(g)
}

// handlePeerHello says which id this instance goes by, to a member that does
// not know it yet, and where its pages are, for a phone at home.
// GET /api/group/peer/hello
func (s *Service) handlePeerHello(w http.ResponseWriter, _ *http.Request) {
	settings, err := s.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, group.Hello{
		OK:         true,
		InstanceID: g.InstanceID,
		Name:       instanceDisplayName(settings),
		Version:    Version,
		DirectURL:  s.directURL(g),
	})
}

// peerMembersAnswer is the group as this instance sees it, for a member that
// reaches only some of it, such as a phone that met the group over the relay
// while an instance was shut down.
type peerMembersAnswer struct {
	OK       bool           `json:"ok"`
	Members  []group.Member `json:"members"`
	Away     []awayMember   `json:"away"`
	Settling bool           `json:"settling"`
	// Behind are the devices left behind when the group on the old phrase
	// closed, which none of the other lists carries.
	Behind []group.LeftBehind `json:"behind"`
}

// handlePeerMembers answers with the members in reach here and the ones that
// keep their row while they are away, the same lists GET /api/group gives the
// page. GET /api/group/peer/members
func (s *Service) handlePeerMembers(w http.ResponseWriter, _ *http.Request) {
	members, away, settling, err := s.groupRoster(time.Now())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, peerMembersAnswer{OK: true, Members: members, Away: away, Settling: settling, Behind: s.behindMove()})
}

// peerPairing is what a member hands out so another member can watch or pull
// its repositories: where they are and, once the request between the two is
// allowed, the restic password that opens them. The APP_KEY never leaves the
// instance.
type peerPairing struct {
	OK             bool           `json:"ok"`
	InstanceName   string         `json:"instanceName"`
	ResticPassword string         `json:"resticPassword"`
	Repos          []pairableRepo `json:"repos"`
	DirectURL      string         `json:"directUrl,omitempty"`
	// KeyLocked says the password was granted but cannot be read there,
	// because that instance's backup key is locked.
	KeyLocked bool `json:"keyLocked,omitempty"`
}

// pairableRepo is one repository location a member offers, with any
// credential in the URL removed.
type pairableRepo struct {
	Domain   string `json:"domain"`
	Name     string `json:"name"`
	Location string `json:"location"`
}

// handlePeerPairing answers a member that pairs a receiver or a pull source
// with this instance. It names the repositories of the sections this instance
// asked the member to keep or to fetch, and adds the restic password once the
// member has allowed that request. GET /api/group/peer/pairing
func (s *Service) handlePeerPairing(w http.ResponseWriter, r *http.Request) {
	// A call without a body carries no proof and is refused below.
	var proof peerProof
	if err := json.NewDecoder(io.LimitReader(r.Body, peerRoleBodyMax)).Decode(&proof); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "malformed pairing request"})
		return
	}
	caller, ok := s.roleAsker(r)
	if ok {
		if err := s.verifyMember(r.Context(), caller, keydeposit.ProofPairing, nil, proof); err != nil {
			writeJSON(w, http.StatusForbidden, failEnvelope(err))
			return
		}
	}
	var grant pairingGrant
	if ok {
		var err error
		if grant, err = s.pairingGrantFor(caller); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	if len(grant.sections) == 0 {
		writeJSON(w, http.StatusOK, failEnvelope(errPeerNotAsked))
		return
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	repos, err := s.pairableRepos(settings)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	repos = slices.DeleteFunc(repos, func(p pairableRepo) bool { return !slices.Contains(grant.sections, p.Domain) })
	ans := peerPairing{OK: true, InstanceName: instanceDisplayName(settings), Repos: repos, DirectURL: s.selfDirectURL()}
	if grant.password {
		ans.ResticPassword = s.resticPassword()
		ans.KeyLocked = ans.ResticPassword == ""
	}
	writeJSON(w, http.StatusOK, ans)
}

// pairableRepos lists the locations another instance could reach: every
// off-site target and every domain whose own path is a remote repository.
// rclone locations are left out, since they only resolve with this
// instance's rclone configuration.
func (s *Service) pairableRepos(settings store.Settings) ([]pairableRepo, error) {
	out := []pairableRepo{}
	for _, domain := range []string{"containers", "vms", "files", "zfs", "flash", "config"} {
		if loc := strings.TrimSpace(domainPathRaw(domain, settings)); isRemoteLocation(loc) {
			out = append(out, pairableRepo{Domain: domain, Location: scrubRepoLocation(loc)})
		}
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		return nil, err
	}
	for _, t := range targets {
		if loc := strings.TrimSpace(t.Repo); loc != "" && !isRcloneLocation(loc) {
			out = append(out, pairableRepo{Domain: t.Domain, Name: scrubRepoLocation(t.Name), Location: scrubRepoLocation(loc)})
		}
	}
	return out, nil
}

func isRemoteLocation(loc string) bool {
	return loc != "" && !isRcloneLocation(loc) && strings.Contains(loc, ":")
}

// handlePeerCheck starts an integrity check of one domain for a member's
// Fleet page and answers at once, since a check outlasts any call timeout.
// POST /api/group/peer/check/{domain}
func (s *Service) handlePeerCheck(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if !checkableDomain(domain) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	go func() {
		if err := s.CheckDomain(s.StopContext(), domain, "local"); err != nil {
			log.Printf("group: check of %s asked for by a member: %v", domain, scrubError(err)) //nolint:gosec // G706: domain comes from a fixed set
		}
	}()
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}

// groupView is what GET /api/group answers with. It never carries the
// phrase, which has its own route behind the password.
type groupView struct {
	OK          bool           `json:"ok"`
	Active      bool           `json:"active"`
	InstanceID  string         `json:"instanceId"`
	Name        string         `json:"name"`
	PasswordSet bool           `json:"passwordSet"`
	Members     []group.Member `json:"members"`
	// Away is every member with a row here that is not in reach now, and
	// Settling whether the group may still be coming back after a restart,
	// in which case one that is away is not known to be gone.
	Away     []awayMember `json:"away"`
	Settling bool         `json:"settling"`
	Relay    relayView    `json:"relay"`
	// JoinedAgo is how many seconds ago this instance created or joined its
	// group, and MemberSeen whether another member has shown up since. The
	// page tells a group waiting for its second instance from one that never
	// forms by them, on the server's clock rather than the browser's.
	JoinedAgo  int64 `json:"joinedAgo"`
	MemberSeen bool  `json:"memberSeen"`
	// Waiting are the members that wait for a person here to let them in,
	// and the ones this instance waits for there.
	Waiting []group.Waiting `json:"waiting"`
	// Older names the devices that hold the phrase but still run a version
	// from before the group ran on the kit, which no member reaches.
	Older []string `json:"older"`
	// Fortress says the group moved into the fortress and runs on its
	// phrase, and OldPhrase names the members that have not followed yet.
	Fortress  bool     `json:"fortress"`
	OldPhrase []string `json:"oldPhrase"`
	// Behind are the devices that had not followed when the group on the
	// old phrase closed, 30 days after the move at the latest. They are not
	// among the ones Away lists.
	Behind []group.LeftBehind `json:"behind"`
	// Fingerprint is this instance's key as another instance shows it while
	// this one waits there, empty outside a group.
	Fingerprint string `json:"fingerprint,omitempty"`
	// SelfAddress is this instance's own address on the local network, as
	// announced to other members: learned from a browser or, failing that,
	// this host's own LAN address.
	SelfAddress string `json:"selfAddress"`
	// OwnAddresses are the addresses this instance found for itself: its
	// IPv4 addresses on the local network, then the host name it announces
	// over mDNS while it does.
	OwnAddresses []string `json:"ownAddresses"`
}

type relayView struct {
	Mode       string `json:"mode"`
	URL        string `json:"url"`
	ProjectURL string `json:"projectUrl"`
	Connected  bool   `json:"connected"`
	Serve      bool   `json:"serve"`
	// ServeClients is how many instances use this one as their relay now.
	ServeClients int `json:"serveClients"`
}

func (h *Handler) groupViewNow() (groupView, error) {
	g, err := h.store.GetGroupState()
	if err != nil {
		return groupView{}, err
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		return groupView{}, err
	}
	clients := 0
	if g.RelayServe {
		clients = h.svc.relayServer().Len()
	}
	now := time.Now()
	members, away, settling, err := h.svc.groupRoster(now)
	if err != nil {
		return groupView{}, err
	}
	if len(members) > 0 && g.MemberSeenAt.IsZero() {
		if err := h.store.MarkGroupMemberSeen(now); err != nil {
			return groupView{}, err
		}
		g.MemberSeenAt = now
	}
	var joinedAgo int64
	if !g.JoinedAt.IsZero() {
		joinedAgo = max(int64(now.Sub(g.JoinedAt).Seconds()), 0)
	}
	fingerprint := ""
	if h.svc.pairing().Active() {
		if seed, err := h.svc.depositSeed(); err == nil {
			fingerprint = kitmember.Fingerprint(keydeposit.OwnerKey(seed))
		}
	}
	return groupView{
		OK:          true,
		Fingerprint: fingerprint,
		Active:      h.svc.pairing().Active(),
		InstanceID:  g.InstanceID,
		Name:        instanceDisplayName(settings),
		PasswordSet: settings.AuthPasswordHash != "",
		Members:     members,
		Away:        away,
		Settling:    settling,
		Waiting:     h.svc.pairing().Waiting(),
		Older:       h.svc.pairing().Older(),
		Relay: relayView{
			Mode:         g.RelayMode,
			URL:          g.RelayURL,
			ProjectURL:   relay.DefaultURL,
			Connected:    h.svc.pairing().RelayConnected(),
			Serve:        g.RelayServe,
			ServeClients: clients,
		},
		JoinedAgo:    joinedAgo,
		MemberSeen:   !g.MemberSeenAt.IsZero(),
		Fortress:     h.svc.pairing().Following(),
		OldPhrase:    h.svc.onOldPhrase(),
		Behind:       h.svc.behindMove(),
		SelfAddress:  h.svc.directURL(g),
		OwnAddresses: h.ownAddresses(),
	}, nil
}

func (h *Handler) ownAddresses() []string {
	out := append([]string{}, mdns.Addresses()...)
	if h.announcedOnHostOnly() {
		return out
	}
	h.mdns.mu.Lock()
	defer h.mdns.mu.Unlock()
	if h.mdns.responder != nil {
		out = append(out, h.mdns.responder.Service().Host+".local")
	}
	return out
}

func (h *Handler) writeGroupView(w http.ResponseWriter) {
	v, err := h.groupViewNow()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// handleGroup reports whether this instance is paired, its members and its
// relay settings. GET /api/group
func (h *Handler) handleGroup(w http.ResponseWriter, _ *http.Request) {
	h.writeGroupView(w)
}

// handleGroupCreate starts a group and answers with its phrase, the only time
// the phrase is handed out without the password being entered again. Pairing
// works without a login password too; the page then warns that anyone who can
// open it can read the words and, through the group, ask every member for its
// restic password. An instance already in a group has to leave it first, since
// a new secret would cut off every member of the old one.
// POST /api/group/phrase
func (h *Handler) handleGroupCreate(w http.ResponseWriter, _ *http.Request) {
	if h.svc.pairing().Active() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "groupExists", "error": "this instance is already paired; leave the group first to start a new one"})
		return
	}
	sec, words, err := phrase.New()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.storeGroupSecret(sec); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	v, err := h.groupViewNow()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"phrase": words, "group": v}))
}

func (h *Handler) storeGroupSecret(sec []byte) error {
	var enc []byte
	if sec != nil {
		sealed, err := secret.Encrypt(h.cfg.AppKey, sec)
		if err != nil {
			return err
		}
		enc = sealed
	}
	if err := h.store.SetGroupSecret(enc, time.Now()); err != nil {
		return err
	}
	h.svc.applyGroup()
	return nil
}

// handleGroupShow shows the phrase again. With a login password set it has to
// be entered again: a session may be open on an unattended screen, and the
// phrase lets any instance into the group. Wrong passwords count against the
// same throttle as the login. POST /api/group/phrase/show
func (h *Handler) handleGroupShow(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &body) || !h.passwordAgain(w, r, body.Password) {
		return
	}
	g, err := h.store.GetGroupState()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	sec, err := h.svc.groupSecret(g)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if sec == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "this instance is not paired"})
		return
	}
	words, err := phrase.Encode(sec)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"phrase": words}))
}

// passwordAgain checks the login password entered again before a phrase
// shows, and answers the request itself when it does not match. Without a
// login password there is nothing to ask for. Wrong passwords count against
// the same throttle as the login.
func (h *Handler) passwordAgain(w http.ResponseWriter, r *http.Request, password string) bool {
	hash, _, on := h.authEnabled()
	if !on {
		return true
	}
	key := h.loginClientKey(r)
	if h.loginThrottled(key) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": "too many failed attempts, wait a minute and try again"})
		return false
	}
	if !verifyPassword(h.cfg.AppKey, password, hash) {
		h.recordLoginFail(key)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "passwordWrong", "error": "the password is needed to show the phrase again"})
		return false
	}
	return true
}

// handleGroupJoin joins the group a phrase belongs to, leaving any group this
// instance was in. A phrase that does not decode is answered with the reason
// and the word at fault, so the page can point at it in the reader's
// language. POST /api/group/join
func (h *Handler) handleGroupJoin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Phrase string `json:"phrase"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	sec, err := phrase.Decode(body.Phrase)
	if err != nil {
		var de *phrase.DecodeError
		if errors.As(err, &de) {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":       false,
				"error":    strings.TrimPrefix(de.Error(), "phrase: "),
				"reason":   de.Reason,
				"word":     de.Word,
				"position": de.Position,
				"count":    de.Count,
			})
			return
		}
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// Another group's phrase leaves this one first. Its members would
	// otherwise stay away here for good, and keep this instance away there.
	if h.inAnotherGroup(sec) {
		err = h.switchGroup(r.Context(), sec)
	} else {
		err = h.storeGroupSecret(sec)
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.writeGroupView(w)
}

// handleGroupLeave tells the members in reach that this instance leaves,
// forgets the group secret and drops the members' Fleet rows. Receivers and
// pull sources stay, and pair again once this instance joins the group again.
// DELETE /api/group
func (h *Handler) handleGroupLeave(w http.ResponseWriter, r *http.Request) {
	if err := h.switchGroup(r.Context(), nil); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.writeGroupView(w)
}

// inAnotherGroup reports whether this instance is in a group other than the
// one sec opens. A phrase this instance cannot open names no group to leave.
func (h *Handler) inAnotherGroup(sec []byte) bool {
	g, err := h.store.GetGroupState()
	if err != nil {
		return false
	}
	cur, err := h.svc.groupSecret(g)
	return err == nil && cur != nil && !bytes.Equal(cur, sec)
}

// switchGroup leaves the group this instance is in for the one sec opens, or
// for none when sec is nil: the members in reach hear it, and their keys and
// Fleet rows go.
func (h *Handler) switchGroup(ctx context.Context, sec []byte) error {
	h.svc.tellMembersLeaving(ctx)
	if err := h.svc.forgetGroup(); err != nil {
		return err
	}
	return h.storeGroupSecret(sec)
}

// leaveGroup leaves the group without telling anybody.
func (s *Service) leaveGroup() error {
	if err := s.forgetGroup(); err != nil {
		return err
	}
	if err := s.store.SetGroupSecret(nil, time.Now()); err != nil {
		return err
	}
	s.applyGroup()
	return nil
}

// forgetGroup drops what this instance keeps of its group's members.
func (s *Service) forgetGroup() error {
	if err := s.store.ForgetMemberKeys(); err != nil {
		return err
	}
	if err := s.store.SetGroupKitState(nil); err != nil {
		return err
	}
	return s.store.DropMemberFleetPeers("")
}

// leaveNoticeTimeout bounds telling the members about a leave, so a member out
// of reach holds the page up only briefly.
const leaveNoticeTimeout = 5 * time.Second

// tellMembersLeaving lets every instance in reach know that this one leaves
// the group, so each takes it off its list at once.
func (s *Service) tellMembersLeaving(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, leaveNoticeTimeout)
	defer cancel()
	var wg sync.WaitGroup
	for _, m := range s.pairing().Members() {
		if m.Kind != "" {
			continue
		}
		proof, err := s.signCall(keydeposit.ProofLeave, m.ID, nil)
		if err != nil {
			log.Printf("group: sign the leave for member %q: %v", m.ID, err)
			continue
		}
		wg.Go(func() {
			_ = s.callMember(ctx, m.ID, http.MethodPost, "/api/group/peer/leave", proof, nil) //nolint:errcheck // callMember logs what went wrong, and the leave goes ahead anyway
		})
	}
	wg.Wait()
}

// handlePeerLeave takes a member that leaves the group off this instance's
// lists: its Fleet row goes, and the group keeps it out of the members until
// its relay connection and announces are gone. Its key stays approved, so it
// comes back as it was when it joins again.
// POST /api/group/peer/leave
func (s *Service) handlePeerLeave(w http.ResponseWriter, r *http.Request) {
	var proof peerProof
	if err := json.NewDecoder(io.LimitReader(r.Body, peerRoleBodyMax)).Decode(&proof); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "malformed leave"})
		return
	}
	sender, ok := s.roleAsker(r)
	if !ok {
		refuseUnnamedPeer(w)
		return
	}
	if err := s.verifyMember(r.Context(), sender, keydeposit.ProofLeave, nil, proof); err != nil {
		writeJSON(w, http.StatusForbidden, failEnvelope(err))
		return
	}
	s.pairing().Forget(sender)
	if err := s.store.DropMemberFleetPeers(sender); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleGroupRelay changes how this instance reaches members elsewhere and
// whether it serves a relay itself. Each field is optional, so a control
// saves only what it changed. PUT /api/group/relay
func (h *Handler) handleGroupRelay(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode  *string `json:"mode"`
		URL   *string `json:"url"`
		Serve *bool   `json:"serve"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	g, err := h.store.GetGroupState()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if body.Mode != nil {
		if !group.ValidMode(group.Mode(*body.Mode)) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the relay mode must be project, own or off"})
			return
		}
		g.RelayMode = *body.Mode
	}
	if body.URL != nil {
		u, msg := normalizeRelayURL(*body.URL)
		if msg != "" {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
			return
		}
		g.RelayURL = u
	}
	if body.Serve != nil {
		g.RelayServe = *body.Serve
	}
	if err := h.store.SetGroupRelay(g.RelayMode, g.RelayURL, g.RelayServe); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.svc.applyGroup()
	// The fortress reaches its members through the same relay.
	if h.svc.fortressNow() != nil {
		h.svc.reopenFortress()
	}
	h.writeGroupView(w)
}

// normalizeRelayURL trims a pasted relay address and puts https:// in front
// of a bare host, since the domain given to a reverse proxy is what people
// type. https rather than http, because the insecure guess is the one that
// costs something.
func normalizeRelayURL(raw string) (string, string) {
	v := strings.TrimRight(strings.TrimSpace(raw), "/")
	if v == "" {
		return "", ""
	}
	if !strings.Contains(v, "://") {
		v = "https://" + v
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" || !slices.Contains([]string{"http", "https", "ws", "wss"}, u.Scheme) {
		return "", "this is not a relay address; use wss://, https:// or a host name"
	}
	if u.User != nil {
		return "", "a relay address carries no user name or password"
	}
	return v, ""
}

// handleRelayConnect is the relay socket while this instance serves one. It
// admits this group's key only, and with the switch off it answers like a
// version without the feature. GET /relay/connect
func (h *Handler) handleRelayConnect(w http.ResponseWriter, r *http.Request) {
	g, err := h.store.GetGroupState()
	if err != nil || !g.RelayServe || !h.svc.pairing().Active() {
		http.NotFound(w, r)
		return
	}
	h.svc.relayServer().ServeHTTP(w, r)
}

// handleGroupCall answers a member's direct call on the local network, of the
// group or of the fortress. It is public like the relay socket: the call is
// signed and sealed with keys only members hold. POST /api/group/call
func (h *Handler) handleGroupCall(w http.ResponseWriter, r *http.Request) {
	if f := h.svc.fortressNow(); f != nil {
		h.svc.fortressDirect(f).ServeHTTP(w, r)
		return
	}
	h.svc.pairing().ServeDirect(w, r)
}

// handleApproveMember lets in a member that waits for approval, under its id
// in the group. POST /api/group/members/{id}/approve
func (h *Handler) handleApproveMember(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.pairing().Approve(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.writeGroupView(w)
}

// handleRefuseMember turns away a member that waits for approval, for good:
// its key stays shut out. POST /api/group/members/{id}/refuse
func (h *Handler) handleRefuseMember(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.pairing().Remove(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.writeGroupView(w)
}

// handleMemberRepos lists the repository locations a member offers, for the
// receiver and pull forms. The restic password stays on the server.
// GET /api/group/members/{id}/repos
func (h *Handler) handleMemberRepos(w http.ResponseWriter, r *http.Request) {
	var p peerPairing
	if err := h.svc.askPairing(r.Context(), r.PathValue("id"), &p); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"instanceName": p.InstanceName, "repos": p.Repos}))
}

// askPairing asks a member, in a signed call, where its repositories are and
// for the restic password that opens them.
func (s *Service) askPairing(ctx context.Context, memberID string, out *peerPairing) error {
	proof, err := s.signCall(keydeposit.ProofPairing, memberID, nil)
	if err != nil {
		return err
	}
	return s.callMember(ctx, memberID, http.MethodGet, "/api/group/peer/pairing", proof, out)
}

// pairedPassword asks a member for its restic password and returns it sealed
// under this instance's key, ready to store, with the locations the member
// offers. role is what this instance takes for the member, Receiver or
// Fetcher: the member gives its password only to an instance it asked for one
// of them.
func (s *Service) pairedPassword(ctx context.Context, memberID, role string) ([]byte, []pairableRepo, error) {
	if strings.TrimSpace(memberID) == "" {
		return nil, nil, errors.New("choose the instance this belongs to")
	}
	s.retellRoleAnswer(ctx, memberID, role)
	var p peerPairing
	if err := s.askPairing(ctx, memberID, &p); err != nil {
		// The member's own words stay in its log, but this box knows why.
		if errors.Is(err, errMemberRefused) && !s.memberAsked(memberID) {
			return nil, nil, errMemberNotAsked
		}
		return nil, nil, err
	}
	switch {
	case p.KeyLocked:
		return nil, nil, errPasswordLocked
	case p.ResticPassword == "":
		return nil, nil, errPasswordHeld
	}
	if !resticPasswordRe.MatchString(p.ResticPassword) {
		return nil, nil, errors.New("the other instance sent no usable restic password")
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, []byte(p.ResticPassword))
	return enc, p.Repos, err
}

// openResticPassword returns a stored restic password in the clear.
func (s *Service) openResticPassword(enc []byte) (string, error) {
	if len(enc) == 0 {
		return "", errors.New("pair this entry with its instance again: it has no restic password yet")
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return "", errors.New("the stored restic password could not be opened")
	}
	return string(plain), nil
}

// ConvertLegacyAppKeys replaces every APP_KEY a receiver or pull source
// stored before pairing existed with the restic password derived from it, so
// those entries keep working and no other instance's master key stays on
// disk. It runs at boot and does nothing once every row is converted. After a
// conversion it compacts the database, since SQLite keeps the old bytes in
// free pages and the write-ahead log.
func (s *Service) ConvertLegacyAppKeys() error {
	converted := 0
	convert := func(enc []byte) ([]byte, error) {
		plain, err := secret.Decrypt(s.cfg.AppKey, enc)
		if err != nil {
			return nil, err
		}
		key := string(plain)
		if !foreignKeyRe.MatchString(key) {
			return nil, errors.New("the stored APP_KEY is not 64 lowercase hex characters")
		}
		return secret.Encrypt(s.cfg.AppKey, []byte(resticPasswordFor(key)))
	}

	repos, err := s.store.ListReceivedRepos()
	if err != nil {
		return err
	}
	for _, rr := range repos {
		if len(rr.LegacyAppKeyEnc) == 0 {
			continue
		}
		enc, err := convert(rr.LegacyAppKeyEnc)
		if err != nil {
			log.Printf("group: received repository %s keeps no password and has to be paired again: %v", rr.ID, err) //nolint:gosec // G706: an opaque store id
		}
		rr.ResticPasswordEnc, rr.LegacyAppKeyEnc = enc, nil
		if err := s.store.UpdateReceivedRepo(rr); err != nil {
			return err
		}
		converted++
	}

	sources, err := s.store.ListPullSources()
	if err != nil {
		return err
	}
	for _, ps := range sources {
		if len(ps.LegacyAppKeyEnc) == 0 {
			continue
		}
		enc, err := convert(ps.LegacyAppKeyEnc)
		if err != nil {
			log.Printf("group: pull source %s keeps no password and has to be paired again: %v", ps.ID, err) //nolint:gosec // G706: an opaque store id
		}
		ps.ResticPasswordEnc, ps.LegacyAppKeyEnc = enc, nil
		if err := s.store.UpdatePullSource(ps); err != nil {
			return err
		}
		converted++
	}
	if converted == 0 {
		return nil
	}
	return s.store.Compact()
}

// resticPasswordRe is the shape restickey.Derive produces.
var resticPasswordRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// resticPassword is this instance's own repository password, the one thing
// about its keys a member receives.
func (s *Service) resticPassword() string {
	key, err := s.backupKey()
	if err != nil {
		return ""
	}
	return resticPasswordFor(key)
}

func resticPasswordFor(appKey string) string { return restickey.Derive(appKey) }
