package api

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/nodelink"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The calls home makes over a node's link. A node asks home for its hello
// alone.
const (
	nodeHelloPath   = "/node/hello"
	nodeAdoptPath   = "/node/adopt"
	nodeReleasePath = "/node/release"
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

// nodeCall is a call home signs for a node. Name is home's, and the signature
// covers it.
type nodeCall struct {
	peerProof
	Name string `json:"name,omitempty"`
}

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
	err := n.verify(action, callPayload(call.Name), call.peerProof)
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
