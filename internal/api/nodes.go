package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/nodelink"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// nodeCallTimeout bounds one call home makes to a node.
const nodeCallTimeout = 30 * time.Second

// NodeView is a node as home lists it.
type NodeView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
	AdoptedAt int64  `json:"adoptedAt"`
}

func nodeViewOf(n store.Node) NodeView {
	return NodeView{ID: n.ID, Name: n.Name, CreatedAt: n.CreatedAt, AdoptedAt: n.AdoptedAt}
}

var (
	errNodeName = errors.New("a node needs a name of at most 64 characters on one line")
	// errNodeReleased is a node's answer that home let it go before.
	errNodeReleased = errors.New("the node was let go before and needs a new APP_KEY")
)

// homeNode is the link home holds to one node, and the adoption it may still
// be waiting for.
type homeNode struct {
	link   *nodelink.Link
	cancel context.CancelFunc
	done   chan struct{}
}

// stop ends the adoption still waiting and returns once it has.
func (hn *homeNode) stop() {
	hn.cancel()
	<-hn.done
}

// OpenNodeLinks connects this instance to every node it keeps.
func (s *Service) OpenNodeLinks() {
	nodes, err := s.store.ListNodes()
	if err != nil {
		log.Printf("nodes: %v", err)
		return
	}
	for _, n := range nodes {
		if err := s.openNode(n); err != nil {
			log.Printf("nodes: %s: %v", n.Name, err)
		}
	}
}

// CloseNodeLinks leaves the link to every node.
func (s *Service) CloseNodeLinks() {
	s.nodeMu.Lock()
	links := s.nodeLinks
	s.nodeLinks = nil
	s.nodeMu.Unlock()
	for _, hn := range links {
		hn.stop()
		_ = hn.link.Close()
	}
}

func (s *Service) openNode(n store.Node) error {
	appKey, err := secret.Decrypt(s.cfg.AppKey, n.AppKeyEnc)
	if err != nil {
		return fmt.Errorf("the node's APP_KEY does not open: %w", err)
	}
	seed, err := s.depositSeed()
	if err != nil {
		return err
	}
	link, err := nodelink.Open(nodelink.Config{
		NodeKey: string(appKey), Seed: seed, Name: s.pairName(),
		Relay: group.Mode(n.RelayMode), RelayURL: n.RelayURL,
	})
	if err != nil {
		return err
	}
	link.Serve(s.answersForNodes())
	ctx, cancel := context.WithCancel(context.Background())
	hn := &homeNode{link: link, cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(hn.done)
		if n.AdoptedAt == 0 {
			s.adoptUntilTaken(ctx, n.ID, link)
		}
	}()
	s.nodeMu.Lock()
	if s.nodeLinks == nil {
		s.nodeLinks = map[string]*homeNode{}
	}
	s.nodeLinks[n.ID] = hn
	s.nodeMu.Unlock()
	return nil
}

// adoptUntilTaken asks the node to take this instance as its home until it
// does. A node may be set up hours after home made it, so the wait grows
// after each miss.
func (s *Service) adoptUntilTaken(ctx context.Context, id string, link *nodelink.Link) {
	wait := time.Second
	for {
		err := s.adoptNode(ctx, id, link)
		if err == nil || errors.Is(err, errNodeReleased) {
			if err != nil {
				log.Printf("nodes: %s: %v", id, err)
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(2*wait, time.Minute)
	}
}

func (s *Service) adoptNode(ctx context.Context, id string, link *nodelink.Link) error {
	var hello nodeHello
	if err := s.callNode(ctx, link, keydeposit.ProofNodeAdopt, id, nodeAdoptPath, s.pairName(), &hello); err != nil {
		return err
	}
	if hello.ID != id {
		return fmt.Errorf("the node answered as %s", hello.ID)
	}
	return s.store.SetNodeAdopted(id, time.Now().Unix())
}

// callNode signs a call for node id, sends it over link and reads the answer
// into out unless out is nil.
func (s *Service) callNode(ctx context.Context, link *nodelink.Link, action, id, path, name string, out any) error {
	p, err := s.signCall(action, id, callPayload(name))
	if err != nil {
		return err
	}
	body, err := json.Marshal(nodeCall{peerProof: p, Name: name})
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, nodeCallTimeout)
	defer cancel()
	status, answer, err := link.Call(cctx, path, body)
	switch {
	case err != nil:
		return err
	case status == http.StatusConflict:
		return errNodeReleased
	case status != http.StatusOK:
		return fmt.Errorf("the node refused the call with HTTP %d", status)
	case out != nil && json.Unmarshal(answer, out) != nil:
		return errors.New("the node's answer could not be read")
	}
	return nil
}

// answersForNodes answers the calls of this instance's nodes: their hello and
// nothing else.
func (s *Service) answersForNodes() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != nodeHelloPath {
			http.NotFound(w, r)
			return
		}
		g, err := s.store.GetGroupState()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
			return
		}
		writeJSON(w, http.StatusOK, nodeHello{OK: true, ID: g.InstanceID, Name: s.pairName(), Version: Version})
	})
}

// AddNode makes a node called name and returns it with the values its compose
// file takes. Those hold the node's APP_KEY, which is shown this once.
func (s *Service) AddNode(name string) (NodeView, map[string]string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 64 || strings.ContainsAny(name, "\r\n") {
		return NodeView{}, nil, errNodeName
	}
	appKey, err := nodelink.NewNodeKey()
	if err != nil {
		return NodeView{}, nil, err
	}
	id, err := nodelink.NodeID(appKey)
	if err != nil {
		return NodeView{}, nil, err
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return NodeView{}, nil, err
	}
	seed, err := s.depositSeed()
	if err != nil {
		return NodeView{}, nil, err
	}
	// A node reaches home over a relay whatever home's members use, so
	// pairing without one still leaves it the project's.
	mode, url := group.Mode(g.RelayMode), strings.TrimSpace(g.RelayURL)
	if mode != group.ModeOwn {
		mode, url = group.ModeProject, ""
	}
	home, err := nodelink.EncodeHome(appKey, nodelink.Home{ID: g.InstanceID, Key: keydeposit.OwnerKey(seed), Relay: mode, RelayURL: url})
	if err != nil {
		return NodeView{}, nil, err
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, []byte(appKey))
	if err != nil {
		return NodeView{}, nil, err
	}
	n := store.Node{ID: id, Name: name, AppKeyEnc: enc, RelayMode: string(mode), RelayURL: url, CreatedAt: time.Now().Unix()}
	if err := s.store.AddNode(n); err != nil {
		return NodeView{}, nil, err
	}
	if err := s.openNode(n); err != nil {
		_ = s.store.DeleteNode(id)
		return NodeView{}, nil, err
	}
	compose := map[string]string{"MODE": "node", "APP_KEY": appKey, "NODE_HOME": home, "NODE_NAME": name}
	return nodeViewOf(n), compose, nil
}

// ListNodes returns every node of this instance.
func (s *Service) ListNodes() ([]NodeView, error) {
	nodes, err := s.store.ListNodes()
	if err != nil {
		return nil, err
	}
	out := make([]NodeView, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, nodeViewOf(n))
	}
	return out, nil
}

// ReleaseNode lets the node with id go and forgets it. told says whether the
// node heard it; one that did not keeps waiting for a home that no longer
// calls.
func (s *Service) ReleaseNode(ctx context.Context, id string) (told bool, err error) {
	if _, err := s.store.GetNode(id); err != nil {
		return false, err
	}
	s.nodeMu.Lock()
	hn := s.nodeLinks[id]
	delete(s.nodeLinks, id)
	s.nodeMu.Unlock()
	if hn != nil {
		hn.stop()
		if err := s.callNode(ctx, hn.link, keydeposit.ProofNodeRelease, id, nodeReleasePath, "", nil); err != nil {
			log.Printf("nodes: %s did not hear it was let go: %v", id, err)
		} else {
			told = true
		}
		_ = hn.link.Close()
	}
	return told, s.store.DeleteNode(id)
}

// handleListNodes lists the nodes this instance runs backups on.
// GET /api/nodes
func (h *Handler) handleListNodes(w http.ResponseWriter, _ *http.Request) {
	nodes, err := h.svc.ListNodes()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"nodes": nodes}))
}

// handleAddNode makes a node and hands out, this once, the values its compose
// file takes, its APP_KEY among them.
// POST /api/nodes
func (h *Handler) handleAddNode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &body) || !h.requireAuthForSecrets(w, r, "adding a node") {
		return
	}
	node, compose, err := h.svc.AddNode(body.Name)
	switch {
	case errors.Is(err, errNodeName):
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"node": node, "compose": compose}))
	}
}

// handleReleaseNode lets a node go. told says whether the node heard it.
// DELETE /api/nodes/{id}
func (h *Handler) handleReleaseNode(w http.ResponseWriter, r *http.Request) {
	if !h.requireSignIn(w, r, "releasing a node") {
		return
	}
	told, err := h.svc.ReleaseNode(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, store.ErrNodeNotFound):
		writeJSON(w, http.StatusNotFound, codedFailEnvelope(err, "not-found"))
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"told": told}))
	}
}
