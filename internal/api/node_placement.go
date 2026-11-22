package api

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The kinds of place home sends a node's backups to: its own receiver, a
// storage location of home's that the node reaches with a sign-in of its own,
// or the node itself, which home fetches them from.
const (
	nodePlaceReceiver = "receiver"
	nodePlaceLocation = "location"
	nodePlaceNode     = "node"
)

// nodeKeyUser labels the key home adds to every repository of a node, so
// home's own recovery kit opens it.
const nodeKeyUser = "home"

var (
	errNodeAddressForm  = errors.New("enter where nodes reach the receiver as an https:// address, or as http:// with a private IP address such as one of a VPN")
	errNodeAddressPlain = errors.New("a node would send its backups over the internet unencrypted: use https://, or http:// only with a private IP address such as one of a VPN")
	errNodeCertificate  = errors.New("the certificate does not read; paste the server's certificate in PEM form, or leave it empty")
	errNoNodeAddress    = errors.New("set where nodes reach the receiver first, under Receiver, Address for nodes")
	errNodeLocation     = errors.New("a node reaches only an S3 service or a REST server of home's, over https:// or a private address")
	errNodeSignInShared = errors.New("pick a sign-in made for the node, not the one this instance writes to the location with")
	errNodePlaceKind    = errors.New("place a node's backups on the receiver, at a storage location or on the node itself")
)

// nodeLocationWarning goes with every placement at a storage location.
const nodeLocationWarning = "With its sign-in the node can usually delete what lies at this location, unlike on the receiver."

// checkNodeAddress reads where nodes reach a server: https anywhere, plain
// http only at a private address, since a node usually sends from the
// internet.
func checkNodeAddress(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errNodeAddressForm
	}
	switch u.Scheme {
	case "https":
	case "http":
		ip, err := netip.ParseAddr(u.Hostname())
		if err != nil || !privateAddr(ip.Unmap()) {
			return "", errNodeAddressPlain
		}
	default:
		return "", errNodeAddressForm
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// privateAddr reports whether ip is one no host on the internet can be
// reached at, a VPN's among them.
func privateAddr(ip netip.Addr) bool {
	return ip.IsPrivate() || ip.IsLoopback() || slices.ContainsFunc(vpnPrefixes, func(p netip.Prefix) bool { return p.Contains(ip) })
}

// SetReceiverNodeAddress stores where nodes reach the receiver and the
// certificate they trust it by, both empty to take nodes off it, and returns
// the address as stored. Every node on the receiver hears a new one, when it
// is back if it is away.
func (s *Service) SetReceiverNodeAddress(raw, caPEM string) (string, error) {
	rs, ok, err := s.store.GetReceiverServer()
	if err != nil || !ok {
		return "", cmp.Or(err, errNoReceiver)
	}
	addr := ""
	if strings.TrimSpace(raw) != "" {
		var err error
		if addr, err = checkNodeAddress(raw); err != nil {
			return "", err
		}
	}
	caPEM = strings.TrimSpace(caPEM)
	if caPEM != "" {
		if !certificatesIn(caPEM) {
			return "", errNodeCertificate
		}
		caPEM += "\n"
	}
	if err := s.store.SetReceiverNodeAddress(addr, caPEM); err != nil {
		return "", err
	}
	if addr == "" || (addr == rs.NodeURL && caPEM == rs.NodeCAPEM) {
		return addr, nil
	}
	nodes, err := s.store.ListNodes()
	if err != nil {
		return addr, err
	}
	for _, n := range nodes {
		if n.Placement != nodePlaceReceiver || n.AdoptedAt == 0 || n.State != "" {
			continue
		}
		if err := s.store.SetNodePlacementDue(n.ID); err != nil {
			return addr, err
		}
		s.renewNode(n.ID)
	}
	return addr, nil
}

// nodePlaceRequest is where a person sends a node's backups. Location and
// SignIn name the storage location and the sign-in made for the node, and
// Address is where home reaches a node that keeps its own.
type nodePlaceRequest struct {
	Kind     string `json:"kind"`
	Location string `json:"location,omitempty"`
	SignIn   string `json:"signIn,omitempty"`
	Address  string `json:"address,omitempty"`
}

// nodePlaceResult is whether the node reached each repository it was given,
// and what a person should know about the place.
type nodePlaceResult struct {
	Reach   []nodeReach `json:"reach"`
	Warning string      `json:"warning,omitempty"`
}

// nodeRepo is one repository home makes for a node: what the node is told,
// and how home reads it.
type nodeRepo struct {
	placement nodePlacement
	// local is where home reaches the repository, and env its sign-in there.
	local    string
	env      []string
	credsRef string
}

// PlaceNode makes the node's repositories at the place req names, adds home's
// own key to each, tells the node where they are and has the receiver watch
// them. Home fetches from a node that keeps them itself. It runs to the end
// under nodePlaceWait even when the browser that asked goes away, since a
// placement cut short leaves the node with some of its repositories and no
// place to send its backups.
func (s *Service) PlaceNode(ctx context.Context, id string, req nodePlaceRequest) (nodePlaceResult, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), nodePlaceWait)
	defer cancel()
	n, err := s.store.GetNode(id)
	if err != nil {
		return nodePlaceResult{}, err
	}
	if n.AdoptedAt == 0 || n.State != "" {
		return nodePlaceResult{}, errNodeNotAdopted
	}
	link, ok := s.nodeLinkOf(id)
	if !ok {
		return nodePlaceResult{}, errNodeNotAdopted
	}
	appKey, err := s.nodeAppKey(n)
	if err != nil {
		return nodePlaceResult{}, err
	}
	var repos []nodeRepo
	var target string
	result := nodePlaceResult{Reach: []nodeReach{}}
	switch req.Kind {
	case nodePlaceReceiver:
		repos, target, err = s.receiverRepos(ctx, n)
	case nodePlaceLocation:
		repos, target, err = s.locationRepos(n, req)
		result.Warning = nodeLocationWarning
	case nodePlaceNode:
		repos, target, err = s.keptRepos(ctx, n, link, req.Address)
	default:
		err = errNodePlaceKind
	}
	if err != nil {
		return nodePlaceResult{}, err
	}
	password := restickey.Derive(appKey)
	placements := make([]nodePlacement, 0, len(repos))
	for _, r := range repos {
		rctx, cancel := context.WithTimeout(ctx, nodeRepoPrepare)
		err := s.prepareNodeRepo(rctx, r.local, password, r.env)
		cancel()
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			err = fmt.Errorf("it did not answer within %d s", int(nodeRepoPrepare.Seconds()))
		}
		if err != nil {
			return nodePlaceResult{}, fmt.Errorf("the %s repository of the node: %w", r.placement.Domain, err)
		}
		placements = append(placements, r.placement)
	}
	var ans nodePlacementAnswer
	if err := s.callNode(ctx, link, keydeposit.ProofNodePlacement, id, nodePlacementPath, placements, &ans); err != nil {
		return nodePlaceResult{}, err
	}
	if err := s.watchNodeRepos(n, repos, password); err != nil {
		return nodePlaceResult{}, err
	}
	if req.Kind == nodePlaceNode {
		err = s.fetchFromNode(n, repos, password)
	} else {
		err = s.stopFetchingFrom(id)
	}
	if err != nil {
		return nodePlaceResult{}, err
	}
	signIn := ""
	if req.Kind == nodePlaceLocation {
		signIn = req.SignIn
	}
	if err := s.store.SetNodePlaced(id, req.Kind, target, signIn); err != nil {
		return nodePlaceResult{}, err
	}
	result.Reach = ans.Reach
	return result, nil
}

// receiverRepos are the node's repositories on home's receiver, one folder
// per section under the node's own login, and the address the node reaches
// them at.
func (s *Service) receiverRepos(ctx context.Context, n store.Node) ([]nodeRepo, string, error) {
	rs, ok, err := s.store.GetReceiverServer()
	if err != nil || !ok {
		return nil, "", cmp.Or(err, errNoReceiver)
	}
	if rs.NodeURL == "" {
		return nil, "", errNoNodeAddress
	}
	user, password, err := s.receiverNodeLogin(ctx, rs, n)
	if err != nil {
		return nil, "", err
	}
	var out []nodeRepo
	for _, d := range nodeDomains {
		out = append(out, nodeRepo{
			placement: nodePlacement{
				Domain: d, Repo: "rest:" + rs.NodeURL + "/" + user + "/" + d,
				Creds: CloudCreds{RESTUser: user, RESTPassword: password}, CAPEM: rs.NodeCAPEM,
			},
			local: path.Join(rs.Folder, user, d),
		})
	}
	return out, rs.NodeURL, nil
}

// receiverNodeLogin returns the node's login on the receiver, creating it the
// first time. A node needs no role a member asks for: home made it.
func (s *Service) receiverNodeLogin(ctx context.Context, rs store.ReceiverServer, n store.Node) (user, password string, err error) {
	s.receiverMu.Lock()
	defer s.receiverMu.Unlock()
	l, ok, err := s.store.GetReceiverLogin(n.ID)
	if err != nil {
		return "", "", err
	}
	if ok {
		plain, err := secret.Decrypt(s.cfg.AppKey, l.PasswordEnc)
		if err != nil {
			return "", "", errors.New("a stored receiver password could not be opened")
		}
		return l.User, string(plain), nil
	}
	logins, err := s.store.ListReceiverLogins()
	if err != nil {
		return "", "", err
	}
	taken := map[string]bool{rs.User: true}
	for _, l := range logins {
		taken[l.User] = true
	}
	user = uniqueReceiverUser(defaultReceiverUser(n.Name), taken)
	password, enc, err := s.addHtpasswdUser(rs.Folder, user)
	if err != nil {
		return "", "", err
	}
	if err := s.store.CreateReceiverLogin(store.ReceiverLogin{MemberID: n.ID, MemberName: n.Name, User: user, PasswordEnc: enc, ForNode: true}); err != nil {
		return "", "", err
	}
	s.reloadReceiver(ctx, rs)
	return user, password, nil
}

// locationRepos are the node's repositories under a storage location of
// home's, which the node reaches with the sign-in req names for it, and the
// location's id.
func (s *Service) locationRepos(n store.Node, req nodePlaceRequest) ([]nodeRepo, string, error) {
	dest, ok, err := s.store.GetDestination(req.Location)
	if err != nil || !ok {
		return nil, "", cmp.Or(err, errNodeLocation)
	}
	base := strings.TrimRight(strings.TrimSpace(dest.Repo), "/")
	if !nodeReachesLocation(base) {
		return nil, "", errNodeLocation
	}
	if req.SignIn == "" || req.SignIn == dest.CredsRef {
		return nil, "", errNodeSignInShared
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, "", err
	}
	creds, err := s.savedSignIn(settings, req.SignIn)
	if err != nil {
		return nil, "", err
	}
	var env []string
	if dest.CredsRef != "" {
		own, err := s.savedSignIn(settings, dest.CredsRef)
		if err != nil {
			return nil, "", err
		}
		env = cloudEnv(own)
	}
	var out []nodeRepo
	for _, d := range nodeDomains {
		// One folder below the location: rest-server takes repositories at
		// most two folders deep, and a location often sits in one already.
		repo := base + "/bombvault-node-" + n.ID[:12] + "-" + d
		out = append(out, nodeRepo{
			placement: nodePlacement{Domain: d, Repo: repo, Creds: creds},
			local:     repo, env: env, credsRef: dest.CredsRef,
		})
	}
	return out, dest.ID, nil
}

// nodeReachesLocation reports whether a node may be sent to the location at
// repo: an S3 service or a REST server, over https or at a private address.
func nodeReachesLocation(repo string) bool {
	scheme, rest, _ := strings.Cut(repo, ":")
	switch scheme {
	case "rest":
		_, err := checkNodeAddress(rest)
		return err == nil
	case "s3":
		if !strings.Contains(rest, "://") {
			return true
		}
		_, err := checkNodeAddress(rest)
		return err == nil
	}
	return false
}

// prepareNodeRepo makes the repository at repo with the node's password,
// unless it opens with it already, and gives it home's own key.
func (s *Service) prepareNodeRepo(ctx context.Context, repo, password string, env []string) error {
	eng, err := s.sharedEngine()
	if err != nil {
		return err
	}
	repo, err = s.resolveRepo(repo)
	if err != nil {
		return err
	}
	node := restic.Mode{Encrypted: true, Password: password, Env: env}
	if s.engine.RepoOpensErr(ctx, repo, node) != nil {
		if err := s.engine.Init(ctx, repo, node); err != nil {
			return err
		}
	}
	home := node
	home.Password = s.backupPassword()
	if s.engine.RepoOpensErr(ctx, repo, home) == nil {
		return nil
	}
	return eng.AddKey(ctx, repo, node, home.Password, s.resticHost(), nodeKeyUser)
}

// watchNodeRepos has the receiver watch the node's repositories, in place of
// the ones it watched for the node before.
func (s *Service) watchNodeRepos(n store.Node, repos []nodeRepo, password string) error {
	enc, err := secret.Encrypt(s.cfg.AppKey, []byte(password))
	if err != nil {
		return err
	}
	for _, r := range repos {
		id := nodeRepoID(n.ID, r.placement.Domain)
		rr, found, err := s.store.GetReceivedRepo(id)
		if err != nil {
			return err
		}
		rr.Repo, rr.NodeID, rr.ResticPasswordEnc, rr.CredsRef = r.local, n.ID, enc, r.credsRef
		if found {
			err = s.store.UpdateReceivedRepo(rr)
		} else {
			rr.ID, rr.Name, rr.DeadManHours, rr.CheckCadence, rr.Enabled = id, n.Name+" "+r.placement.Domain, 26, "daily 04:00", true
			_, err = s.store.CreateReceivedRepo(rr)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// handleSetReceiverNodeAddress stores where nodes reach the receiver.
// PUT /api/receiver/node-address
func (h *Handler) handleSetReceiverNodeAddress(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL   string `json:"url"`
		CAPEM string `json:"caPem"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	addr, err := h.svc.SetReceiverNodeAddress(body.URL, body.CAPEM)
	switch {
	case errors.Is(err, errNodeAddressForm) || errors.Is(err, errNodeAddressPlain) || errors.Is(err, errNodeCertificate):
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"nodeUrl": addr}))
	}
}

// handlePlaceNode sends a node's backups to the receiver, a storage location
// or the node itself, and answers whether the node reaches them.
// POST /api/nodes/{id}/placement
func (h *Handler) handlePlaceNode(w http.ResponseWriter, r *http.Request) {
	var req nodePlaceRequest
	if !decodeBody(w, r, &req) || !h.requireSignIn(w, r, "placing a node's backups") {
		return
	}
	result, err := h.svc.PlaceNode(r.Context(), r.PathValue("id"), req)
	var unreached *nodeRestUnreached
	switch {
	case errors.Is(err, store.ErrNodeNotFound):
		writeJSON(w, http.StatusNotFound, codedFailEnvelope(err, "not-found"))
	case errors.Is(err, errNodeAddressForm) || errors.Is(err, errNodeFetchPlain):
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
	case errors.Is(err, errNodeRestPulling):
		log.Printf("nodes: placing the backups of node %s: %v", r.PathValue("id"), err) //nolint:gosec // G706: the id is one of home's nodes
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, codeNodeRestPulling))
	case errors.As(err, &unreached):
		log.Printf("nodes: placing the backups of node %s: %v", r.PathValue("id"), err) //nolint:gosec // G706: the id is one of home's nodes
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "code": unreached.code(), "address": unreached.url.String(), "port": unreached.port(), "seconds": int(nodeRestReach.Seconds())})
	case err != nil:
		log.Printf("nodes: placing the backups of node %s: %s", r.PathValue("id"), scrubError(err)) //nolint:gosec // G706: the id is one of home's nodes
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"reach": result.Reach, "warning": result.Warning}))
	}
}

// nodesKitSection writes the recovery kit's chapter on this server's nodes:
// each node's APP_KEY, which brings the node back on a new server, and its
// repositories, which this server's own password opens as well.
func (s *Service) nodesKitSection(w func(string, ...any)) {
	nodes, err := s.store.ListNodes()
	if err != nil {
		log.Printf("api: recovery kit: the nodes could not be read: %v", err)
		return
	}
	if len(nodes) == 0 {
		return
	}
	repos, err := s.store.ListReceivedRepos()
	if err != nil {
		log.Printf("api: recovery kit: the repositories of the nodes could not be read: %v", err)
	}
	w("## Nodes\n\n")
	w("These servers back up for this one. A node set up again with the same APP_KEY\n")
	w("and NODE_HOME is the same node. Its repositories open with the restic password\n")
	w("of its APP_KEY, and with this server's password above as well. This server keeps\n")
	w("each node's containers, folder sets and runs and gives them back to it.\n\n")
	for _, n := range nodes {
		w("- %s (id %s)\n", n.Name, n.ID)
		appKey, err := s.nodeAppKey(n)
		if err != nil {
			w("  APP_KEY: (it does not open with this server's APP_KEY)\n")
		} else {
			w("  APP_KEY: %s\n", appKey)
			w("  restic password: %s\n", restickey.Derive(appKey))
		}
		for _, rr := range repos {
			if rr.NodeID != n.ID {
				continue
			}
			loc := rr.Repo
			if resolved, err := s.resolveRepo(rr.Repo); err == nil {
				loc = resolved
			}
			w("  %s: %s\n", rr.Name, loc)
		}
	}
	w("\n")
}
