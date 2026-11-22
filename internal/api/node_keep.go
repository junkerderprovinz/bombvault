package api

import (
	"cmp"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/nodelink"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/trust"
)

// nodeKeepPath is where home asks a node to keep its backups itself, behind a
// rest-server of its own that home fetches them from.
const nodeKeepPath = "/node/keep"

// The rest-server a node keeps its backups behind: its container, the folder
// under Host Data that holds the repositories, and the one login the node and
// home both use there. A name of its own keeps it clear of a rest-server the
// person runs on the same server.
const (
	nodeRestName   = "bombvault-node-rest-server"
	nodeRestFolder = "bombvault-node"
	nodeRestUser   = "bombvault"
)

// nodeRestTLS is the folder in the rest-server's data folder that holds its
// certificate and key. rest-server serves only repositories, so it never
// hands them out.
const nodeRestTLS = ".tls"

// nodePullCadence is how often home fetches from a node that keeps its own
// backups: hourly, so a copy reaches home soon after each of the node's
// backups whenever they run.
const nodePullCadence = "15 * * * *"

// nodeRestStop bounds the stop of the rest-server before it is made anew.
const nodeRestStop = 10 * time.Second

// nodeRestReach is how long home keeps trying to reach the rest-server a node
// has just started, and nodeRepoPrepare how long it waits on each of the
// node's repositories. restic retries a server that does not answer for a
// quarter of an hour, so without them a placement looks stuck.
var (
	nodeRestReach   = 30 * time.Second
	nodeRepoPrepare = 2 * time.Minute
)

// nodePlaceWait bounds a whole placement: the reach check, each section's
// repository and the call that tells the node.
var nodePlaceWait = nodeRestReach + time.Duration(len(nodeDomains))*nodeRepoPrepare + time.Minute

// nodeRestPullWait is how long home waits for a node to download
// rest-server's image, asking every nodeRestPullPoll, and nodeRestPullLongest
// how long the node gives the download. A call over the relay ends long before
// a slow download does, so the node downloads in the background.
var (
	nodeRestPullWait    = 2 * time.Minute
	nodeRestPullPoll    = 3 * time.Second
	nodeRestPullLongest = 15 * time.Minute
)

// The codes of a placement that failed because home could not reach the
// node's rest-server, or found a server there with another certificate.
const (
	codeNodeRestUnreachable = "node-rest-unreachable"
	codeNodeRestForeign     = "node-rest-foreign"
	codeNodeRestPulling     = "node-rest-pulling"
)

var errNodeRestPulling = errors.New("the node is still downloading the rest-server image: try again in a few minutes")

var errNodeFetchPlain = errors.New("this instance would fetch the node's backups over the internet unencrypted: use https://, or http:// only with a private IP address such as one of a VPN")

// nodeKeepCall is the address home reaches the node's rest-server at.
type nodeKeepCall struct {
	Address string `json:"address"`
}

// nodeKeepAnswer is the login on the node's rest-server and the certificate
// it serves, empty over plain http. Pulling says the node is still downloading
// rest-server's image and home should ask again.
type nodeKeepAnswer struct {
	OK       bool   `json:"ok"`
	Pulling  bool   `json:"pulling,omitempty"`
	Error    string `json:"error,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	CAPEM    string `json:"caPem,omitempty"`
}

// keep starts the rest-server this node keeps its backups behind and answers
// how home signs in there.
func (n *nodeSide) keep(w http.ResponseWriter, r *http.Request) {
	call, ok := n.signed(w, r, keydeposit.ProofNodeKeep)
	if !ok || !n.adopted(w) {
		return
	}
	var req nodeKeepCall
	if json.Unmarshal(call.Body, &req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "the call could not be read"})
		return
	}
	ans, err := n.s.keepBackupsHere(r.Context(), req.Address)
	switch {
	case errors.Is(err, errNodeAddressForm) || errors.Is(err, errNodeAddressPlain):
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusOK, nodeKeepAnswer{Error: scrubError(err)})
	default:
		writeJSON(w, http.StatusOK, ans)
	}
}

// keepBackupsHere starts an append-only rest-server for this node's
// repositories on the port of addr, over TLS with a certificate made for its
// host when addr is https, and gives it a new login.
func (s *Service) keepBackupsHere(ctx context.Context, addr string) (nodeKeepAnswer, error) {
	u, err := nodeRestAddress(addr)
	if err != nil {
		return nodeKeepAnswer{}, err
	}
	port := 443
	if u.Scheme == "http" {
		port = 80
	}
	if p := u.Port(); p != "" {
		if port, err = strconv.Atoi(p); err != nil {
			return nodeKeepAnswer{}, errNodeAddressForm
		}
	}
	allocs, err := s.docker.Allocations(ctx)
	if err != nil {
		return nodeKeepAnswer{}, err
	}
	want := strconv.Itoa(port) + "/tcp"
	for _, a := range allocs {
		if a.Name != nodeRestName && slices.Contains(a.HostPorts, want) {
			return nodeKeepAnswer{}, fmt.Errorf("port %d is taken by %s on the node; choose another one", port, a.Name)
		}
	}
	ready, err := s.restServerImageReady(ctx)
	if err != nil {
		return nodeKeepAnswer{}, err
	}
	if !ready {
		return nodeKeepAnswer{Pulling: true}, nil
	}
	password, _, err := s.addHtpasswdUser(nodeRestFolder, nodeRestUser)
	if err != nil {
		return nodeKeepAnswer{}, err
	}
	options, certPEM := restServerOptions, ""
	if u.Scheme == "https" {
		if certPEM, err = s.nodeRestCertificate(u.Hostname()); err != nil {
			return nodeKeepAnswer{}, err
		}
		tls := "/data/" + nodeRestTLS
		options += " --tls --tls-cert " + tls + "/cert.pem --tls-key " + tls + "/key.pem"
	}
	if live, err := s.docker.InspectName(ctx, nodeRestName); err != nil {
		return nodeKeepAnswer{}, err
	} else if live != "" {
		if err := s.docker.Stop(ctx, nodeRestName, nodeRestStop); err != nil {
			return nodeKeepAnswer{}, err
		}
		if err := s.docker.Remove(ctx, nodeRestName); err != nil {
			return nodeKeepAnswer{}, err
		}
	}
	spec := receiverContainerSpec(path.Join(s.cfg.HostSourceRoot, nodeRestFolder), port, s.platformFn().Kind() == platform.KindUnraid)
	spec.Name = nodeRestName
	spec.Config.Env = []string{"OPTIONS=" + options}
	if err := s.docker.CreateAndStart(ctx, spec, true); err != nil {
		return nodeKeepAnswer{}, fmt.Errorf("create the %s container: %w", nodeRestName, err)
	}
	return nodeKeepAnswer{OK: true, User: nodeRestUser, Password: password, CAPEM: certPEM}, nil
}

// imagePull is a download of rest-server's image, and how it ended once done
// is closed.
type imagePull struct {
	done chan struct{}
	err  error
}

// restServerImageReady reports whether Docker has rest-server's image. When it
// does not, it starts the download in the background, unless one runs, and a
// download that failed is reported once before the next one starts.
func (s *Service) restServerImageReady(ctx context.Context) (bool, error) {
	id, err := s.docker.ImageID(ctx, restServerImage)
	if err != nil || id != "" {
		return id != "", err
	}
	s.nodeMu.Lock()
	defer s.nodeMu.Unlock()
	if p := s.restPull; p != nil {
		select {
		case <-p.done:
			s.restPull = nil
			if p.err != nil {
				return false, fmt.Errorf("download %s: %w", restServerImage, p.err)
			}
			// It ended between the look at Docker and this one.
			return true, nil
		default:
			return false, nil
		}
	}
	p := &imagePull{done: make(chan struct{})}
	s.restPull = p
	go func() {
		ctx, cancel := context.WithTimeout(s.StopContext(), nodeRestPullLongest)
		defer cancel()
		if p.err = s.docker.Pull(ctx, restServerImage); p.err != nil {
			log.Printf("node: download %s: %v", restServerImage, p.err)
		}
		close(p.done)
	}()
	return false, nil
}

// nodeRestAddress reads where home reaches a node's rest-server, which serves
// the repositories at its root, so the address names no folder.
func nodeRestAddress(raw string) (*url.URL, error) {
	addr, err := checkNodeAddress(raw)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(addr)
	if err != nil || u.Path != "" {
		return nil, errNodeAddressForm
	}
	return u, nil
}

// nodeRestCertificate returns the certificate the node's rest-server serves
// for host. It makes a new self-signed one when the one there does not cover
// host, which home then pins.
func (s *Service) nodeRestCertificate(host string) (string, error) {
	root, err := os.OpenRoot(s.cfg.HostMountRoot)
	if err != nil {
		return "", err
	}
	defer root.Close() //nolint:errcheck // a failed close changes nothing written
	dir := path.Join(nodeRestFolder, nodeRestTLS)
	certName, keyName := path.Join(dir, "cert.pem"), path.Join(dir, "key.pem")
	if held, err := root.ReadFile(certName); err == nil {
		_, keyErr := root.Stat(keyName)
		if leaf, err := parseCertificatePEM(held); err == nil && keyErr == nil && leaf.VerifyHostname(host) == nil && time.Now().Before(leaf.NotAfter) {
			return string(held), nil
		}
	}
	certPEM, keyPEM, err := selfSignedFor(host)
	if err != nil {
		return "", err
	}
	if err := root.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := root.WriteFile(keyName, keyPEM, 0o600); err != nil {
		return "", err
	}
	if err := root.WriteFile(certName, certPEM, 0o644); err != nil { //nolint:gosec // G306: a server certificate is public, and home is handed it
		return "", err
	}
	return string(certPEM), nil
}

// selfSignedFor makes a P-256 key and a self-signed certificate for host.
func selfSignedFor(host string) (certPEM, keyPEM []byte, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: host, Organization: []string{"BombVault"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), nil
}

// keptRepos are the node's repositories behind its own rest-server at the
// address raw, which home reaches from outside, and that address. Home pins
// the certificate the node made and signs in with a saved sign-in of the
// node's. A placement made again without an address keeps the one before.
func (s *Service) keptRepos(ctx context.Context, n store.Node, link *nodelink.Link, raw string) ([]nodeRepo, string, error) {
	if raw == "" && n.Placement == nodePlaceNode {
		raw = n.PlacementTarget
	}
	u, err := nodeRestAddress(raw)
	if errors.Is(err, errNodeAddressPlain) {
		err = errNodeFetchPlain
	}
	if err != nil {
		return nil, "", err
	}
	addr := u.String()
	var kept nodeKeepAnswer
	until := time.Now().Add(nodeRestPullWait)
	for {
		kept = nodeKeepAnswer{}
		if err := s.callNode(ctx, link, keydeposit.ProofNodeKeep, n.ID, nodeKeepPath, nodeKeepCall{Address: addr}, &kept); err != nil {
			return nil, "", err
		}
		if !kept.Pulling {
			break
		}
		if time.Now().After(until) {
			return nil, "", errNodeRestPulling
		}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(nodeRestPullPoll):
		}
	}
	if !kept.OK {
		return nil, "", errors.New(cmp.Or(kept.Error, "the node could not start its rest-server"))
	}
	if err := s.reachNodeRest(ctx, u, kept.CAPEM); err != nil {
		return nil, "", err
	}
	if err := writeTrusted(s.nodeCertFile(n.ID), kept.CAPEM); err != nil {
		return nil, "", err
	}
	creds := CloudCreds{RESTUser: kept.User, RESTPassword: kept.Password}
	ref, err := s.keepNodeSignIn(n, creds)
	if err != nil {
		return nil, "", err
	}
	var out []nodeRepo
	for _, d := range nodeDomains {
		repo := "rest:" + addr + "/" + kept.User + "/" + d
		out = append(out, nodeRepo{
			placement: nodePlacement{Domain: d, Repo: repo, Creds: creds, CAPEM: kept.CAPEM, Kept: true},
			local:     repo, env: cloudEnv(creds), credsRef: ref,
		})
	}
	return out, addr, nil
}

// nodeRestUnreached is a node's rest-server home could not reach at url, or
// found another server at when foreign is set.
type nodeRestUnreached struct {
	url     *url.URL
	foreign bool
	err     error
}

func (e *nodeRestUnreached) Error() string {
	if e.foreign {
		return fmt.Sprintf("the server at %s has another certificate than the one the node made: check that the address leads to the node (%v)", e.url, e.err)
	}
	return fmt.Sprintf("this instance could not reach the node's rest-server at %s within %d s: check that port %s is open in the node's firewall and in its provider's firewall (%v)",
		e.url, int(nodeRestReach.Seconds()), e.port(), e.err)
}

func (e *nodeRestUnreached) code() string {
	if e.foreign {
		return codeNodeRestForeign
	}
	return codeNodeRestUnreachable
}

func (e *nodeRestUnreached) port() string {
	if p := e.url.Port(); p != "" {
		return p
	}
	if e.url.Scheme == "http" {
		return "80"
	}
	return "443"
}

// reachNodeRest waits until the rest-server at u answers, over the
// certificate caPEM when it is https. The server may take a moment to listen
// after the node started it, so home asks again until nodeRestReach is up.
func (s *Service) reachNodeRest(ctx context.Context, u *url.URL, caPEM string) error {
	ctx, cancel := context.WithTimeout(ctx, nodeRestReach)
	defer cancel()
	tr := &http.Transport{DialContext: s.nodeRestDial, DisableKeepAlives: true}
	if caPEM != "" {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM([]byte(caPEM))
		tr.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}
	client := http.Client{Transport: tr}
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.String()+"/", nil)
		if err != nil {
			return err
		}
		res, err := client.Do(req)
		if err == nil {
			_ = res.Body.Close()
			return nil
		}
		var cert *tls.CertificateVerificationError
		if errors.As(err, &cert) {
			return &nodeRestUnreached{url: u, foreign: true, err: cert.Err}
		}
		// The error around it names the address already.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		select {
		case <-ctx.Done():
			return &nodeRestUnreached{url: u, err: err}
		case <-time.After(time.Second):
		}
	}
}

// nodeCertFile is where home keeps the certificate of node id's rest-server.
func (s *Service) nodeCertFile(id string) string {
	return filepath.Join(trust.Dir(s.cfg.DataDir), "node-"+id[:12]+".pem")
}

// keepNodeSignIn stores the login on node n's rest-server as a saved sign-in
// and returns its id, which stays the same on every placement.
func (s *Service) keepNodeSignIn(n store.Node, c CloudCreds) (string, error) {
	id := "node-" + n.ID[:12]
	set := CloudCredSet{ID: id, Name: n.Name + " rest-server", CloudCreds: c}
	err := s.editCloudCredSets(func(sets []CloudCredSet) []CloudCredSet {
		i := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == id })
		if i < 0 {
			return append(sets, set)
		}
		set.Default = sets[i].Default
		sets[i] = set
		return sets
	})
	return id, err
}

// fetchFromNode has home pull each of the node's repositories into its own, as
// a pull source of the node.
func (s *Service) fetchFromNode(n store.Node, repos []nodeRepo, password string) error {
	enc, err := secret.Encrypt(s.cfg.AppKey, []byte(password))
	if err != nil {
		return err
	}
	sources, err := s.store.ListPullSources()
	if err != nil {
		return err
	}
	for _, r := range repos {
		d := r.placement.Domain
		i := slices.IndexFunc(sources, func(ps store.PullSource) bool { return ps.NodeID == n.ID && ps.Domain == d })
		if i < 0 {
			_, err = s.store.CreatePullSource(store.PullSource{
				Name: n.Name + " " + d, Repo: r.local, ResticPasswordEnc: enc, CredsRef: r.credsRef,
				Domain: d, Cadence: nodePullCadence, Enabled: true, NodeID: n.ID,
			})
		} else {
			ps := sources[i]
			ps.Repo, ps.ResticPasswordEnc, ps.CredsRef, ps.Enabled = r.local, enc, r.credsRef, true
			err = s.store.UpdatePullSource(ps)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// stopFetchingFrom switches off home's pulls from node id. What was fetched
// stays, and so do the sources that lead to it.
func (s *Service) stopFetchingFrom(id string) error {
	sources, err := s.store.ListPullSources()
	if err != nil {
		return err
	}
	for _, ps := range sources {
		if ps.NodeID != id || !ps.Enabled {
			continue
		}
		ps.Enabled = false
		if err := s.store.UpdatePullSource(ps); err != nil {
			return err
		}
	}
	return nil
}

// nodePrunePath is where home has a node age and prune a repository it keeps
// itself.
const nodePrunePath = "/node/prune"

// The states of such a prune.
const (
	nodePruneRunning = "running"
	nodePruneDone    = "done"
	nodePruneFailed  = "failed"
)

// nodePruneWait is how long home waits before it asks a node again how the
// prune of a repository it keeps itself goes. A variable so a test does not
// wait it out.
var nodePruneWait = 30 * time.Second

// nodePruneCall is home's keep rule for one section a node keeps itself. Run
// names the prune, so the same call made again asks how it goes instead of
// starting another.
type nodePruneCall struct {
	Domain string                 `json:"domain"`
	Run    string                 `json:"run"`
	Keep   restic.RetentionPolicy `json:"keep"`
}

// nodePruneAnswer is how a node's prune goes.
type nodePruneAnswer struct {
	OK    bool   `json:"ok"`
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

// nodePrune is the prune a node runs or ran last on a section.
type nodePrune struct {
	run, state, err string
}

// prune starts the prune of a section this node keeps itself, unless it runs
// or ran already, and answers how it goes.
func (n *nodeSide) prune(w http.ResponseWriter, r *http.Request) {
	call, ok := n.signed(w, r, keydeposit.ProofNodePrune)
	if !ok || !n.adopted(w) {
		return
	}
	var req nodePruneCall
	if json.Unmarshal(call.Body, &req) != nil || !slices.Contains(nodeDomains, req.Domain) || req.Run == "" || !req.Keep.Any() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "a prune names a section a node backs up, a run and what to keep"})
		return
	}
	writeJSON(w, http.StatusOK, n.s.pruneKept(req))
}

// pruneKept starts the prune req names in the background and answers how it
// goes. While another prune of the section runs, it answers that one.
func (s *Service) pruneKept(req nodePruneCall) nodePruneAnswer {
	s.nodeMu.Lock()
	defer s.nodeMu.Unlock()
	if cur, ok := s.keptPrunes[req.Domain]; ok && (cur.run == req.Run || cur.state == nodePruneRunning) {
		return nodePruneAnswer{OK: true, State: cur.state, Error: cur.err}
	}
	if s.keptPrunes == nil {
		s.keptPrunes = map[string]nodePrune{}
	}
	s.keptPrunes[req.Domain] = nodePrune{run: req.Run, state: nodePruneRunning}
	go func() {
		done := nodePrune{run: req.Run, state: nodePruneDone}
		if err := s.pruneKeptRepo(s.StopContext(), req.Domain, req.Keep); err != nil {
			done.state, done.err = nodePruneFailed, scrubError(err)
			log.Printf("node: the prune of the %s failed: %v", req.Domain, err) //nolint:gosec // G706: the domain is one of nodeDomains
		}
		s.nodeMu.Lock()
		s.keptPrunes[req.Domain] = done
		s.nodeMu.Unlock()
	}()
	return nodePruneAnswer{OK: true, State: nodePruneRunning}
}

// keptRepoDir is the folder that holds the repository of domain behind this
// node's rest-server.
func (s *Service) keptRepoDir(domain string) string {
	return path.Join(s.cfg.HostMountRoot, nodeRestFolder, nodeRestUser, domain)
}

// pruneKeptRepo ages the repository of domain this node keeps by keep, under
// its own host alone, and prunes it. It works on the folder itself, since the
// rest-server in front of it takes no deletes.
func (s *Service) pruneKeptRepo(ctx context.Context, domain string, keep restic.RetentionPolicy) error {
	p, placed, err := s.placedSection(domain)
	if err != nil {
		return err
	}
	if !placed || !p.Kept {
		return fmt.Errorf("home has not placed the %s on this node itself", domain)
	}
	repo, mode := s.placedRepo(p), s.placedMode(p)
	if err := s.engine.RepoOpensErr(ctx, repo, mode); err != nil {
		return err
	}
	return s.ageAndPrune(ctx, repo, mode, s.resticHost(), keep)
}

// pruneOnNode has n age and prune the repository of domain it keeps itself by
// p, and waits until it is done. A node that is away is asked again once it is
// back, for as long as a run may take.
func (s *Service) pruneOnNode(ctx context.Context, n store.Node, domain string, p restic.RetentionPolicy) error {
	run, err := randomHex(8)
	if err != nil {
		return err
	}
	call := nodePruneCall{Domain: domain, Run: run, Keep: p}
	until := time.Now().Add(nodeRunLongest)
	for {
		var ans nodePruneAnswer
		err := s.callNodeByID(ctx, n.ID, keydeposit.ProofNodePrune, nodePrunePath, call, &ans)
		switch {
		case err != nil && !errors.Is(err, errNodeAway):
			return err
		case err == nil && ans.State == nodePruneDone:
			return nil
		case err == nil && ans.State == nodePruneFailed:
			return errors.New(cmp.Or(ans.Error, "the node could not prune"))
		case time.Now().After(until):
			return errors.New("the node did not finish its prune in time")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(nodePruneWait):
		}
	}
}

// fetchNodeSection fetches in the background what the runs of section domain
// on node n wrote, unless home does not fetch from n or a fetch runs already.
func (s *Service) fetchNodeSection(n store.Node, domain string) {
	sources, err := s.store.ListPullSources()
	if err != nil {
		log.Printf("nodes: %s: %v", n.Name, err) //nolint:gosec // G706: the name is one home gave the node
		return
	}
	for _, ps := range sources {
		if ps.NodeID != n.ID || ps.Domain != domain || !ps.Enabled {
			continue
		}
		go func() {
			if _, err := s.PullFromSource(s.StopContext(), ps); err != nil && !errors.Is(err, errPullBusy) {
				log.Printf("nodes: %s: the fetch of its %s failed: %v", n.Name, domain, err) //nolint:gosec // G706: the name is one home gave the node
			}
		}()
	}
}
