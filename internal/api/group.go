package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/discovery"
	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/seedphrase"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// relayConnectPath is where an instance serving a relay answers, on the
// address it already has.
const relayConnectPath = "/relay/connect"

func (s *Service) buildGroup() {
	s.groupOnce.Do(func() {
		s.groupMgr = group.NewManager(s.servePeer)
		s.relaySrv = relay.NewServer()
		s.relaySrv.Admit = func(key string) bool { return s.relayServe.Load() && s.groupMgr.AdmitsRelayKey(key) }
		// Behind a trusted reverse proxy the relay counts failed handshakes
		// per client, as the login throttle does, not per proxy.
		s.relaySrv.ClientAddr = func(r *http.Request) string { return clientKeyBehind(s.cfg.TrustedProxies, r) }
	})
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
	s.applyGroup()
	s.pairing().Start()
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
	if len(plain) != seedphrase.SecretLen {
		return nil, errors.New("the stored pairing secret has the wrong length")
	}
	return plain, nil
}

// applyGroup hands the stored group to the Manager. It runs at boot and after
// every change to the group, the relay settings or the instance name.
func (s *Service) applyGroup() {
	g, err := s.store.GetGroupState()
	if err != nil {
		log.Printf("group: %v", err)
		return
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("group: %v", err)
		return
	}
	sec, err := s.groupSecret(g)
	if err != nil {
		// Logged, because the instance looks paired while reaching nobody.
		log.Printf("group: %v; pairing is off until the phrase is entered again", err)
	}
	s.pairing().Apply(group.Config{
		Secret:     sec,
		InstanceID: g.InstanceID,
		Name:       instanceDisplayName(settings),
		Version:    Version,
		DirectURL:  s.directURL(g),
		Mode:       group.Mode(g.RelayMode),
		RelayURL:   g.RelayURL,
	})
	// Connections admitted under the old key or before the switch went off
	// would otherwise stay open until they drop.
	s.relayServe.Store(g.RelayServe)
	s.relayServer().Revoke()
}

// instanceDisplayName is the name other members show this instance under.
func instanceDisplayName(settings store.Settings) string {
	if name := strings.TrimSpace(settings.InstanceName); name != "" {
		return name
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
	ip := discovery.LocalIPv4()
	if ip == "" {
		return ""
	}
	if s.cfg.HTTPOnly {
		return "http://" + ip + ":" + strconv.Itoa(s.cfg.Port)
	}
	return "https://" + ip + ":" + strconv.Itoa(s.cfg.HTTPSPort)
}

// learnDirectURL takes the scheme, host and port a browser used to reach
// this instance and stores it as the instance's own direct address, unless a
// person has set one by hand. localhost, a loopback or a link-local address
// is ignored: none of those mean anything to another instance on the
// network, and storing one would silence the real address underneath it.
func (s *Service) learnDirectURL(r *http.Request) {
	addr := directAddressFromRequest(s.cfg.HTTPOnly, r)
	if addr == "" {
		return
	}
	if err := s.store.LearnGroupDirectURL(addr); err != nil {
		log.Printf("group: learn this instance's direct address: %v", err)
	}
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
func (s *Service) servePeer(ctx context.Context, call relay.ProxyCall) (status int, body []byte) {
	req, err := http.NewRequestWithContext(ctx, call.Method, call.Path, bytes.NewReader(call.Body))
	if err != nil {
		return http.StatusBadRequest, nil
	}
	req.Header.Set("Content-Type", "application/json")
	rec := &peerRecorder{header: http.Header{}, status: http.StatusOK}
	// The relay client runs this on its own goroutine, where no net/http
	// server would recover a panicking handler.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("group: handler panicked serving %s %s: %v", call.Method, call.Path, r)
			status, body = http.StatusInternalServerError, nil
		}
	}()
	s.peerMux().ServeHTTP(rec, req)
	return rec.status, rec.body.Bytes()
}

// peerRoute is one call a group member may make.
type peerRoute struct {
	pattern string
	handle  func(s *Service, w http.ResponseWriter, r *http.Request)
}

// peerRoutes is everything a group member can reach on this instance: the
// Fleet scorecard, a storage offer, the receiver this instance runs for the
// group and a login on it, what receiver and pull pairing need, starting a
// check, a request to replicate a ZFS item here, what runs here and how it
// looks for the Android app, a session for a phone of the group, and the bare
// hello a blind probe gets.
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
		{"POST /api/group/peer/check/{domain}", (*Service).handlePeerCheck},
		{"POST /api/group/peer/zfs-receive", (*Service).handlePeerZFSReceive},
		{"GET /api/group/peer/activity", (*Service).handlePeerActivity},
		{"GET /api/group/peer/display-prefs", (*Service).handlePeerDisplayPrefs},
		{"POST /api/group/peer/session", (*Service).handlePeerSession},
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

type peerRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *peerRecorder) Header() http.Header         { return r.header }
func (r *peerRecorder) Write(b []byte) (int, error) { return r.body.Write(b) }
func (r *peerRecorder) WriteHeader(status int)      { r.status = status }

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
	if errors.Is(err, group.ErrNotMember) {
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
	s.harvestDirectURL(memberID, raw)
	return nil
}

// harvestDirectURL notes memberID's self-reported direct address, when its
// answer carries one, as a hint for Call to try before falling back to the
// relay. Every peer route that answers over the relay ends up feeding this,
// since the field is the same on all of them; a member that never sends one
// leaves nothing to note.
func (s *Service) harvestDirectURL(memberID string, raw []byte) {
	var hint struct {
		DirectURL string `json:"directUrl"`
	}
	if json.Unmarshal(raw, &hint) == nil && hint.DirectURL != "" {
		s.pairing().NoteAddress(memberID, hint.DirectURL)
	}
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

// handlePeerHello answers a probe that does not yet know which member, if
// any, it is calling: the LAN sweep and a manually entered address both use
// it. It carries nothing a probe is not owed once it has proven group
// membership by reaching this route at all.
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

// peerPairing is what a member hands out so another member can watch or pull
// its repositories: where they are and the restic password that opens them.
// The APP_KEY never leaves the instance.
type peerPairing struct {
	OK             bool           `json:"ok"`
	InstanceName   string         `json:"instanceName"`
	ResticPassword string         `json:"resticPassword"`
	Repos          []pairableRepo `json:"repos"`
	DirectURL      string         `json:"directUrl,omitempty"`
}

// pairableRepo is one repository location a member offers, with any
// credential in the URL removed.
type pairableRepo struct {
	Domain   string `json:"domain"`
	Name     string `json:"name"`
	Location string `json:"location"`
}

// handlePeerPairing answers a member that pairs a receiver or a pull source
// with this instance. GET /api/group/peer/pairing
func (s *Service) handlePeerPairing(w http.ResponseWriter, _ *http.Request) {
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
	writeJSON(w, http.StatusOK, peerPairing{
		OK:             true,
		InstanceName:   instanceDisplayName(settings),
		ResticPassword: s.resticPassword(),
		Repos:          repos,
		DirectURL:      s.selfDirectURL(),
	})
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
			out = append(out, pairableRepo{Domain: t.Domain, Name: t.Name, Location: scrubRepoLocation(loc)})
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
	Relay       relayView      `json:"relay"`
	// JoinedAgo is how many seconds ago this instance created or joined its
	// group, and MemberSeen whether another member has shown up since. The
	// page tells a group waiting for its second instance from one that never
	// forms by them, on the server's clock rather than the browser's.
	JoinedAgo  int64 `json:"joinedAgo"`
	MemberSeen bool  `json:"memberSeen"`
	// SelfAddress is this instance's own address on the local network, as
	// announced to other members: learned from a browser, set by hand, or,
	// failing both, this host's own LAN address.
	SelfAddress string `json:"selfAddress"`
	// SelfAddressManual is true once a person has set SelfAddress by hand,
	// so a newly learned address stops replacing it until it is cleared.
	SelfAddressManual bool `json:"selfAddressManual"`
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
	members := h.svc.pairing().Members()
	now := time.Now()
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
	// Cheap to call on every load: NeedsSweep decides whether there is
	// anything to look for, and this is also what makes joining with a
	// phrase kick off a sweep at once, since that join answers with this
	// same view.
	h.svc.pairing().MaybeSweep()
	return groupView{
		OK:          true,
		Active:      h.svc.pairing().Active(),
		InstanceID:  g.InstanceID,
		Name:        instanceDisplayName(settings),
		PasswordSet: settings.AuthPasswordHash != "",
		Members:     members,
		Relay: relayView{
			Mode:         g.RelayMode,
			URL:          g.RelayURL,
			ProjectURL:   relay.DefaultURL,
			Connected:    h.svc.pairing().RelayConnected(),
			Serve:        g.RelayServe,
			ServeClients: clients,
		},
		JoinedAgo:         joinedAgo,
		MemberSeen:        !g.MemberSeenAt.IsZero(),
		SelfAddress:       h.svc.directURL(g),
		SelfAddressManual: g.DirectURLManual,
	}, nil
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
	sec, phrase, err := seedphrase.New()
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
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"phrase": phrase, "group": v}))
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
	if !decodeBody(w, r, &body) {
		return
	}
	if hash, _, on := h.authEnabled(); on {
		key := h.loginClientKey(r)
		if h.loginThrottled(key) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": "too many failed attempts, wait a minute and try again"})
			return
		}
		if !verifyPassword(h.cfg.AppKey, body.Password, hash) {
			h.recordLoginFail(key)
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "passwordWrong", "error": "the password is needed to show the phrase again"})
			return
		}
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
	phrase, err := seedphrase.Encode(sec)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"phrase": phrase}))
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
	sec, err := seedphrase.Decode(body.Phrase)
	if err != nil {
		var de *seedphrase.DecodeError
		if errors.As(err, &de) {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":       false,
				"error":    strings.TrimPrefix(de.Error(), "seedphrase: "),
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
	if err := h.storeGroupSecret(sec); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.writeGroupView(w)
}

// handleGroupLeave forgets the group secret. Receivers, pull sources and
// Fleet rows stay, and pair again once this instance joins the group again.
// DELETE /api/group
func (h *Handler) handleGroupLeave(w http.ResponseWriter, _ *http.Request) {
	if err := h.storeGroupSecret(nil); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.writeGroupView(w)
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
	if _, err := relay.ConnectURL(v); err != nil {
		return "", "this is not a relay address; use wss://, https:// or a host name"
	}
	u, err := url.Parse(v)
	if err != nil || u.User != nil {
		return "", "a relay address carries no user name or password"
	}
	return v, ""
}

// handleGroupAddress sets or clears this instance's own direct address by
// hand: a reverse proxy hostname, a port a browser never sees, or any other
// case a learned address gets wrong. An empty url clears the override, so
// the next signed-in request is learned again. PUT /api/group/address
func (h *Handler) handleGroupAddress(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	v := strings.TrimSpace(body.URL)
	if v == "" {
		if err := h.store.SetGroupDirectURL("", false); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		h.svc.applyGroup()
		h.writeGroupView(w)
		return
	}
	addr, msg := normalizeDirectURL(v)
	if msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if err := h.store.SetGroupDirectURL(addr, true); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.svc.applyGroup()
	h.writeGroupView(w)
}

// normalizeDirectURL trims a pasted address and requires the scheme a member
// actually dials: http or https, never ws, since this is where the direct
// call transport connects, not the relay.
func normalizeDirectURL(raw string) (string, string) {
	v := strings.TrimRight(strings.TrimSpace(raw), "/")
	if !strings.Contains(v, "://") {
		v = "https://" + v
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return "", "this is not an address; use http:// or https://"
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "an instance address needs http:// or https://"
	}
	if u.User != nil {
		return "", "an address carries no user name or password"
	}
	return v, ""
}

// handleGroupProbe makes one blind, signed call to an address a person typed
// under "Can't find it?", for a subnet or a port the LAN sweep does not try
// on its own. A successful answer is recorded exactly as the sweep records
// one. POST /api/group/probe
func (h *Handler) handleGroupProbe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	addr, msg := normalizeDirectURL(body.URL)
	if msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if _, err := h.svc.pairing().Probe(r.Context(), addr); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "nothing at that address answered as a member of this group"})
		return
	}
	h.writeGroupView(w)
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

// handleGroupCall answers a member's direct call on the local network. It is
// public like the relay socket: the call is signed and sealed with keys only
// members hold. POST /api/group/call
func (h *Handler) handleGroupCall(w http.ResponseWriter, r *http.Request) {
	h.svc.pairing().ServeDirect(w, r)
}

// handleMemberRepos lists the repository locations a member offers, for the
// receiver and pull forms. The restic password stays on the server.
// GET /api/group/members/{id}/repos
func (h *Handler) handleMemberRepos(w http.ResponseWriter, r *http.Request) {
	var p peerPairing
	if err := h.svc.callMember(r.Context(), r.PathValue("id"), http.MethodGet, "/api/group/peer/pairing", nil, &p); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"instanceName": p.InstanceName, "repos": p.Repos}))
}

// pairedPassword asks a member for its restic password and returns it sealed
// under this instance's key, ready to store.
func (s *Service) pairedPassword(ctx context.Context, memberID string) ([]byte, error) {
	if strings.TrimSpace(memberID) == "" {
		return nil, errors.New("choose the instance this belongs to")
	}
	var p peerPairing
	if err := s.callMember(ctx, memberID, http.MethodGet, "/api/group/peer/pairing", nil, &p); err != nil {
		return nil, err
	}
	if !resticPasswordRe.MatchString(p.ResticPassword) {
		return nil, errors.New("the other instance sent no usable restic password")
	}
	return secret.Encrypt(s.cfg.AppKey, []byte(p.ResticPassword))
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
func (s *Service) resticPassword() string { return resticPasswordFor(s.cfg.AppKey) }

func resticPasswordFor(appKey string) string { return restickey.Derive(appKey) }
