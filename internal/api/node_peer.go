package api

import (
	"bytes"
	"cmp"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/nodelink"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/trust"
)

// The calls home makes over a node's link. A node asks home for its hello
// alone.
const (
	nodeHelloPath     = "/node/hello"
	nodeAdoptPath     = "/node/adopt"
	nodeReleasePath   = "/node/release"
	nodePlacementPath = "/node/placement"
)

// maxNodeCall caps what a call over a node's link may carry.
const maxNodeCall = 64 << 10

// nodeHello is what either end of a node's link says of itself.
type nodeHello struct {
	OK        bool   `json:"ok"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	AdoptedAt int64  `json:"adoptedAt,omitempty"`
}

// nodeCall is a call home signs for a node. Name is home's and Body what
// the call says, and the signature covers both.
type nodeCall struct {
	peerProof
	Name string          `json:"name,omitempty"`
	Body json.RawMessage `json:"body,omitempty"`
}

// payload is what the signature of a call covers besides its claim.
func (c nodeCall) payload() []byte { return callPayload(c.Name, string(c.Body)) }

// codeNodeReleased refuses a call to a node home let go, and
// codeNodeNotAdopted one that needs an adoption first.
const (
	codeNodeReleased   = "node-released"
	codeNodeNotAdopted = "node-not-adopted"
)

// OpenHomeLink connects this node to the home NODE_HOME names and answers its
// calls there. It refuses a home other than the one that adopted this node.
func (s *Service) OpenHomeLink(home nodelink.Home, name string) error {
	name = cmp.Or(name, s.pairName())
	routes, err := s.homeRoutes(home, name)
	if err != nil {
		return err
	}
	link, err := nodelink.Open(nodelink.Config{NodeKey: s.cfg.AppKey, Home: home.Key, Name: name, Relay: home.Relay, RelayURL: home.RelayURL})
	if err != nil {
		return err
	}
	link.Serve(routes)
	s.nodeMu.Lock()
	s.homeLink = link
	s.nodeMu.Unlock()
	return nil
}

// CloseHomeLink leaves the link to home.
func (s *Service) CloseHomeLink() {
	s.nodeMu.Lock()
	link := s.homeLink
	s.homeLink = nil
	s.nodeMu.Unlock()
	if link != nil {
		_ = link.Close()
	}
}

// homeRoutes pins home's key for its signed calls and returns what answers
// them. Before an adoption NODE_HOME may name another home or key, after one
// it may not.
func (s *Service) homeRoutes(home nodelink.Home, name string) (http.Handler, error) {
	id, err := nodelink.NodeID(s.cfg.AppKey)
	if err != nil {
		return nil, err
	}
	link, err := s.store.GetNodeLink()
	if err != nil {
		return nil, err
	}
	adopted := link.AdoptedAt != 0 && link.ReleasedAt == 0
	if adopted && link.HomeID != home.ID {
		return nil, fmt.Errorf("this node belongs to the home %s and NODE_HOME names %s; release the node there first", link.HomeID, home.ID)
	}
	err = s.store.PinMemberKey(home.ID, home.Key)
	if errors.Is(err, store.ErrMemberKey) && !adopted {
		if err = s.store.ForgetMemberKey(home.ID); err == nil {
			err = s.store.PinMemberKey(home.ID, home.Key)
		}
	}
	if errors.Is(err, store.ErrMemberKey) {
		return nil, errors.New("NODE_HOME carries another key for home than the one this node was adopted with")
	}
	if err != nil {
		return nil, err
	}
	n := &nodeSide{s: s, home: home, id: id, name: name}
	return http.HandlerFunc(n.serve), nil
}

// nodeSide is a node as its home calls it.
type nodeSide struct {
	s    *Service
	home nodelink.Home
	// id is the id this node goes by on its link.
	id   string
	name string
}

func (n *nodeSide) serve(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case nodeHelloPath:
		n.hello(w)
	case nodeAdoptPath:
		n.adopt(w, r)
	case nodeReleasePath:
		n.release(w, r)
	case nodePlacementPath:
		n.placement(w, r)
	case nodeAPIPath:
		n.api(w, r)
	case nodeRunPath:
		n.run(w, r)
	case nodeKeepPath:
		n.keep(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (n *nodeSide) hello(w http.ResponseWriter) {
	link, err := n.s.store.GetNodeLink()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, nodeHello{OK: true, ID: n.id, Name: n.name, Version: Version, AdoptedAt: link.AdoptedAt})
}

func (n *nodeSide) adopt(w http.ResponseWriter, r *http.Request) {
	call, ok := n.signed(w, r, keydeposit.ProofNodeAdopt)
	if !ok {
		return
	}
	link, err := n.s.store.GetNodeLink()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
		return
	}
	if link.ReleasedAt != 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			"ok": false, "code": codeNodeReleased,
			"error": "home let this node go, and it takes a home again only with a new APP_KEY",
		})
		return
	}
	if link.AdoptedAt == 0 {
		if err := n.s.store.SetNodeLinkAdopted(n.home.ID, time.Now().Unix()); err != nil {
			writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
			return
		}
		log.Printf("node: %q took this node on", call.Name)
	}
	n.hello(w)
}

func (n *nodeSide) release(w http.ResponseWriter, r *http.Request) {
	if _, ok := n.signed(w, r, keydeposit.ProofNodeRelease); !ok || !n.adopted(w) {
		return
	}
	if err := n.s.store.SetNodeLinkReleased(time.Now().Unix()); err != nil {
		writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
		return
	}
	log.Print("node: home let this node go")
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// adopted reports whether home adopted this node and has not let it go, and
// answers the call itself when not. Every call but the hello and the
// adoption needs it.
func (n *nodeSide) adopted(w http.ResponseWriter) bool {
	link, err := n.s.store.GetNodeLink()
	switch {
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
	case link.ReleasedAt != 0:
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "code": codeNodeReleased, "error": "home let this node go"})
	case link.AdoptedAt == 0:
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "code": codeNodeNotAdopted, "error": "home has not adopted this node yet"})
	default:
		return true
	}
	return false
}

// signed reads a call home signed for action, and answers it itself when the
// signature does not hold or the call was taken before.
func (n *nodeSide) signed(w http.ResponseWriter, r *http.Request, action string) (nodeCall, bool) {
	var call nodeCall
	if json.NewDecoder(io.LimitReader(r.Body, maxNodeCall)).Decode(&call) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "the call could not be read"})
		return call, false
	}
	err := n.verify(action, call.payload(), call.peerProof)
	switch {
	case errors.Is(err, errUnsignedCall) || errors.Is(err, store.ErrMemberCounter):
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": err.Error()})
		return call, false
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
		return call, false
	}
	return call, true
}

// verify checks that a call is signed with home's key for this node and
// carries a counter newer than every call before.
func (n *nodeSide) verify(action string, payload []byte, p peerProof) error {
	if len(p.Key) == 0 || p.Counter > time.Now().Add(maxCounterLead).UnixMilli() {
		return errUnsignedCall
	}
	key, found, err := n.s.store.MemberKey(n.home.ID)
	if err != nil {
		return err
	}
	claim := keydeposit.Claim{Action: action, OwnerID: n.home.ID, KeeperID: n.id, Counter: p.Counter, Deposit: payload}
	if !found || !bytes.Equal(key, p.Key) || !keydeposit.Proven(key, p.Proof, claim) {
		return errUnsignedCall
	}
	return n.s.store.AdvanceMemberCounter(n.home.ID, p.Counter)
}

// nodeDomains are the sections a node backs up.
var nodeDomains = []string{"containers", "files"}

// nodeReachWait bounds the test whether a node reaches a repository, which
// restic would otherwise retry for minutes.
const nodeReachWait = time.Minute

// nodePlacement is where home sends one section of a node's backups, with
// the sign-in the node uses there and the certificate it trusts the server
// by, empty for one the system trusts.
type nodePlacement struct {
	Domain string     `json:"domain"`
	Repo   string     `json:"repo"`
	Creds  CloudCreds `json:"creds"`
	CAPEM  string     `json:"caPem,omitempty"`
}

// nodeReach is whether a node opened the repository of a section.
type nodeReach struct {
	Domain  string `json:"domain"`
	Reached bool   `json:"reached"`
	Detail  string `json:"detail,omitempty"`
}

// nodePlacementAnswer is a node's answer to its placements.
type nodePlacementAnswer struct {
	OK    bool        `json:"ok"`
	Reach []nodeReach `json:"reach"`
}

var errNodePlacement = errors.New("a placement names a section a node does not back up, a location on this host or a certificate that does not read")

// placement stores where each section's backups go and answers whether this
// node opens each repository there.
func (n *nodeSide) placement(w http.ResponseWriter, r *http.Request) {
	call, ok := n.signed(w, r, keydeposit.ProofNodePlacement)
	if !ok || !n.adopted(w) {
		return
	}
	var ps []nodePlacement
	if json.Unmarshal(call.Body, &ps) != nil || checkPlacements(ps) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": errNodePlacement.Error()})
		return
	}
	ans := nodePlacementAnswer{OK: true, Reach: []nodeReach{}}
	for _, p := range ps {
		if err := n.s.keepPlacement(p); err != nil {
			writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
			return
		}
		ans.Reach = append(ans.Reach, n.s.reachPlacement(r.Context(), p))
	}
	writeJSON(w, http.StatusOK, ans)
}

func checkPlacements(ps []nodePlacement) error {
	seen := map[string]bool{}
	for _, p := range ps {
		if !slices.Contains(nodeDomains, p.Domain) || seen[p.Domain] || !restic.IsRemoteRepo(p.Repo) {
			return errNodePlacement
		}
		seen[p.Domain] = true
		if p.CAPEM != "" && !certificatesIn(p.CAPEM) {
			return errNodePlacement
		}
	}
	return nil
}

// certificatesIn reports whether pemText holds certificates and nothing else.
func certificatesIn(pemText string) bool {
	rest, found := []byte(pemText), false
	for {
		block, more := pem.Decode(rest)
		if block == nil {
			return found && len(bytes.TrimSpace(more)) == 0
		}
		if block.Type != "CERTIFICATE" {
			return false
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return false
		}
		rest, found = more, true
	}
}

// keepPlacement stores p with its sign-in sealed, and puts its certificate
// where restic trusts it.
func (s *Service) keepPlacement(p nodePlacement) error {
	creds, err := json.Marshal(p.Creds)
	if err != nil {
		return err
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, creds)
	if err != nil {
		return err
	}
	if err := writeTrusted(filepath.Join(trust.Dir(s.cfg.DataDir), "node-"+p.Domain+".pem"), p.CAPEM); err != nil {
		return err
	}
	return s.store.SetNodePlacement(store.NodePlacement{Domain: p.Domain, Repo: p.Repo, CredsEnc: enc, CAPEM: p.CAPEM, SetAt: time.Now().Unix()})
}

// writeTrusted puts pemText into file in the certs folder, where restic
// trusts it, or takes file away when pemText is empty.
func writeTrusted(file, pemText string) error {
	if pemText == "" {
		err := os.Remove(file)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(pemText), 0o600)
}

// reachPlacement opens the repository of p as this node's backups will,
// without a lock and with no sign-in but the one home sent.
func (s *Service) reachPlacement(ctx context.Context, p nodePlacement) nodeReach {
	ctx, cancel := context.WithTimeout(ctx, nodeReachWait)
	defer cancel()
	mode := restic.Mode{Encrypted: true, Password: s.backupPassword(), NoLock: true, NoAmbientCreds: true, Env: cloudEnv(p.Creds)}
	if err := s.engine.RepoOpensErr(ctx, p.Repo, mode); err != nil {
		return nodeReach{Domain: p.Domain, Detail: scrubError(err)}
	}
	return nodeReach{Domain: p.Domain, Reached: true}
}

// placementCreds opens the sign-in home sent for a placement.
func (s *Service) placementCreds(p store.NodePlacement) (CloudCreds, error) {
	raw, err := secret.Decrypt(s.cfg.AppKey, p.CredsEnc)
	if err != nil {
		return CloudCreds{}, err
	}
	var c CloudCreds
	return c, json.Unmarshal(raw, &c)
}
