package api

import (
	"cmp"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
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

var errNodeFetchPlain = errors.New("this instance would fetch the node's backups over the internet unencrypted: use https://, or http:// only with a private IP address such as one of a VPN")

// nodeKeepCall is the address home reaches the node's rest-server at.
type nodeKeepCall struct {
	Address string `json:"address"`
}

// nodeKeepAnswer is the login on the node's rest-server and the certificate
// it serves, empty over plain http.
type nodeKeepAnswer struct {
	OK       bool   `json:"ok"`
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
	if err := s.ensureRestServerImage(ctx); err != nil {
		return nodeKeepAnswer{}, err
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
	if err := s.callNode(ctx, link, keydeposit.ProofNodeKeep, n.ID, nodeKeepPath, nodeKeepCall{Address: addr}, &kept); err != nil {
		return nil, "", err
	}
	if !kept.OK {
		return nil, "", errors.New(cmp.Or(kept.Error, "the node could not start its rest-server"))
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
			placement: nodePlacement{Domain: d, Repo: repo, Creds: creds, CAPEM: kept.CAPEM},
			local:     repo, env: cloudEnv(creds), credsRef: ref,
		})
	}
	return out, addr, nil
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
