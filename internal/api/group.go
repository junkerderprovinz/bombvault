package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

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
		DirectURL:  s.directURL(),
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

// directURL is where this instance answers members on the local network: its
// first LAN address on the port it listens on.
func (s *Service) directURL() string {
	ip := discovery.LocalIPv4()
	if ip == "" {
		return ""
	}
	if s.cfg.HTTPOnly {
		return "http://" + ip + ":" + strconv.Itoa(s.cfg.Port)
	}
	return "https://" + ip + ":" + strconv.Itoa(s.cfg.HTTPSPort)
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
// Fleet scorecard, a storage offer, what receiver and pull pairing need, and
// starting a check. Settings, secrets and the phrase are not among them.
var peerRoutes = []peerRoute{
	{"GET /api/group/peer/status", (*Service).handlePeerStatus},
	{"POST /api/group/peer/mesh-offer", (*Service).handlePeerMeshOffer},
	{"GET /api/group/peer/pairing", (*Service).handlePeerPairing},
	{"POST /api/group/peer/check/{domain}", (*Service).handlePeerCheck},
}

func (s *Service) peerMux() http.Handler {
	s.peerMuxOnce.Do(func() {
		mux := http.NewServeMux()
		for _, pr := range peerRoutes {
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
	return groupView{
		OK:          true,
		Active:      h.svc.pairing().Active(),
		InstanceID:  g.InstanceID,
		Name:        instanceDisplayName(settings),
		PasswordSet: settings.AuthPasswordHash != "",
		Members:     h.svc.pairing().Members(),
		Relay: relayView{
			Mode:         g.RelayMode,
			URL:          g.RelayURL,
			ProjectURL:   relay.DefaultURL,
			Connected:    h.svc.pairing().RelayConnected(),
			Serve:        g.RelayServe,
			ServeClients: clients,
		},
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
// the phrase is handed out without the password being entered again. Every
// route that hands out or takes a phrase needs a login password to be set,
// since the phrase yields the restic password of every member. An instance
// already in a group has to leave it first, since a new secret would cut off
// every member of the old one. POST /api/group/phrase
func (h *Handler) handleGroupCreate(w http.ResponseWriter, _ *http.Request) {
	if !h.requireAuthForSecrets(w, "pairing this instance") {
		return
	}
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
	if err := h.store.SetGroupSecret(enc); err != nil {
		return err
	}
	h.svc.applyGroup()
	return nil
}

// handleGroupShow shows the phrase again once the login password is entered
// again: a session may be open on an unattended screen, and the phrase lets
// any instance into the group. Wrong passwords count against the same
// throttle as the login. POST /api/group/phrase/show
func (h *Handler) handleGroupShow(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "showing the pairing phrase") {
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	hash, _, _ := h.authEnabled()
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
	if !h.requireAuthForSecrets(w, "pairing this instance") {
		return
	}
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
// disk. It runs at boot and does nothing once every row is converted.
func (s *Service) ConvertLegacyAppKeys() error {
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
	}
	return nil
}

// resticPasswordRe is the shape restickey.Derive produces.
var resticPasswordRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// resticPassword is this instance's own repository password, the one thing
// about its keys a member receives.
func (s *Service) resticPassword() string { return resticPasswordFor(s.cfg.AppKey) }

func resticPasswordFor(appKey string) string { return restickey.Derive(appKey) }
