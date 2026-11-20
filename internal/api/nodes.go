package api

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
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

// NodeView is a node as home lists it. State is "releasing" while the node
// has not heard that home let it go. Placement is where its backups go and
// PlacementTarget the receiver's address or the storage location's name
// there. Version is what the node runs, empty while it is away. The cadences
// are when home starts the backups of each section.
type NodeView struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	CreatedAt         int64  `json:"createdAt"`
	AdoptedAt         int64  `json:"adoptedAt"`
	State             string `json:"state,omitempty"`
	Placement         string `json:"placement,omitempty"`
	PlacementTarget   string `json:"placementTarget,omitempty"`
	Online            bool   `json:"online"`
	LastSeen          int64  `json:"lastSeen,omitempty"`
	Version           string `json:"version,omitempty"`
	ContainersCadence string `json:"containersCadence"`
	FilesCadence      string `json:"filesCadence"`
	// LastBackup is when the newest backup home knows of on the node
	// finished. Failures counts the entries whose backups fail and what the
	// receiver watch raised about the node's repositories, and Anomalies what
	// the node had open when home last asked.
	LastBackup int64 `json:"lastBackup,omitempty"`
	Failures   int   `json:"failures"`
	Anomalies  int   `json:"anomalies"`
}

func nodeViewOf(n store.Node) NodeView {
	return NodeView{ID: n.ID, Name: n.Name, CreatedAt: n.CreatedAt, AdoptedAt: n.AdoptedAt, State: n.State, Placement: n.Placement, PlacementTarget: n.PlacementTarget,
		ContainersCadence: n.ContainersCadence, FilesCadence: n.FilesCadence}
}

// nodeHealth fills in how the backups of node v go, as far as home learned.
func (s *Service) nodeHealth(v *NodeView) error {
	runs, err := s.store.NodeRuns(v.ID)
	if err != nil {
		return err
	}
	for _, r := range runs {
		if r.Status == "success" || r.Status == "warning" {
			v.LastBackup = max(v.LastBackup, r.FinishedAt)
		}
	}
	open, err := s.store.NodeEpisodes(v.ID)
	if err != nil {
		return err
	}
	for key := range open {
		if key != nodeAwayEpisode {
			v.Failures++
		}
	}
	watched, err := s.store.NodeWatchAlerts(v.ID)
	if err != nil {
		return err
	}
	v.Failures += watched
	v.Anomalies = s.nodes.anomalies(v.ID)
	return nil
}

// nodeHelloWait bounds the question to each node what it runs, so a slow one
// does not hold up the list.
const nodeHelloWait = 3 * time.Second

var (
	errNodeName = errors.New("a node needs a name of at most 64 characters on one line")
	// errNodeReleased and errNodeNotAdopted are a node's answers that home
	// let it go before, or has not adopted it yet.
	errNodeReleased   = errors.New("the node was let go before and needs a new APP_KEY")
	errNodeNotAdopted = errors.New("the node has not taken this instance as its home yet")
	errNodeAway       = errors.New("the node is not on the relay")
)

// homeNode is the link home holds to one node, and the adoption or release
// it may still be waiting to get through.
type homeNode struct {
	link   *nodelink.Link
	cancel context.CancelFunc
	done   chan struct{}
}

// stop ends the adoption or release still waiting and returns once it has.
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
		link, err := s.nodeLink(n)
		if err != nil {
			log.Printf("nodes: %s: %v", n.Name, err)
			continue
		}
		s.keepNode(n, link)
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

// nodeLink opens the link to node n.
func (s *Service) nodeLink(n store.Node) (*nodelink.Link, error) {
	appKey, err := s.nodeAppKey(n)
	if err != nil {
		return nil, err
	}
	seed, err := s.depositSeed()
	if err != nil {
		return nil, err
	}
	link, err := nodelink.Open(nodelink.Config{
		NodeKey: appKey, Seed: seed, Name: s.pairName(),
		Relay: group.Mode(n.RelayMode), RelayURL: n.RelayURL,
	})
	if err != nil {
		return nil, err
	}
	link.Serve(s.answersForNodes())
	return link, nil
}

// nodeAppKey opens the APP_KEY home keeps for node n.
func (s *Service) nodeAppKey(n store.Node) (string, error) {
	appKey, err := secret.Decrypt(s.cfg.AppKey, n.AppKeyEnc)
	if err != nil {
		return "", fmt.Errorf("the node's APP_KEY does not open: %w", err)
	}
	return string(appKey), nil
}

// keepNode holds link for node n and, in the background, gets through what
// the node still has to hear: its adoption and a change to where its backups
// go, or that home let it go.
func (s *Service) keepNode(n store.Node, link *nodelink.Link) {
	ctx, cancel := context.WithCancel(context.Background())
	hn := &homeNode{link: link, cancel: cancel, done: make(chan struct{})}
	s.nodeMu.Lock()
	if s.nodeLinks == nil {
		s.nodeLinks = map[string]*homeNode{}
	}
	s.nodeLinks[n.ID] = hn
	s.nodeMu.Unlock()
	go func() {
		defer close(hn.done)
		if n.State == store.NodeReleasing {
			if untilDone(ctx, func() error { return s.releaseNode(ctx, n.ID, link) }) {
				s.dropNode(n.ID, hn)
			}
			return
		}
		if n.AdoptedAt == 0 && !untilDone(ctx, func() error { return s.adoptNode(ctx, n.ID, link) }) {
			return
		}
		if n.PlacementDue {
			untilDone(ctx, func() error {
				// Making the repositories again costs restic runs, which wait
				// until the node is back to hear where they are.
				if !link.Online() {
					return errNodeAway
				}
				_, err := s.PlaceNode(ctx, n.ID, nodePlaceRequest{Kind: n.Placement})
				return err
			})
		}
	}()
}

// untilDone runs try until it succeeds or ctx ends, and reports whether it
// succeeded. A node may be away for hours, so the wait grows after each miss.
func untilDone(ctx context.Context, try func() error) bool {
	wait := time.Second
	for {
		err := try()
		if err == nil {
			return true
		}
		if errors.Is(err, errNodeReleased) {
			log.Printf("nodes: %v", err)
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(wait):
		}
		wait = min(2*wait, time.Minute)
	}
}

// dropNode forgets a node that heard its release, unless hn is no longer the
// link home holds to it.
func (s *Service) dropNode(id string, hn *homeNode) {
	s.nodeMu.Lock()
	ours := s.nodeLinks[id] == hn
	if ours {
		delete(s.nodeLinks, id)
	}
	s.nodeMu.Unlock()
	if !ours {
		return
	}
	_ = hn.link.Close()
	if err := s.forgetNode(id); err != nil {
		log.Printf("nodes: forget %s: %v", id, err)
	}
}

// forgetNode drops a node that heard its release. Its login on the receiver
// goes, and the receiver stops watching its repositories, which stay with
// their backups. Home stops fetching from it and keeps what it fetched.
func (s *Service) forgetNode(id string) error {
	if err := s.RevokeReceiverLogin(context.Background(), id); err != nil && !errors.Is(err, errNoReceiver) {
		return err
	}
	if err := s.stopFetchingFrom(id); err != nil {
		return err
	}
	repos, err := s.store.ListReceivedRepos()
	if err != nil {
		return err
	}
	for _, rr := range repos {
		if rr.NodeID == id && rr.Enabled {
			rr.Enabled = false
			if err := s.store.UpdateReceivedRepo(rr); err != nil {
				return err
			}
		}
	}
	return s.store.DeleteNode(id)
}

// renewNode has the link to node id get through what the node has still to
// hear, as the store says it now.
func (s *Service) renewNode(id string) {
	s.nodeMu.Lock()
	hn := s.nodeLinks[id]
	delete(s.nodeLinks, id)
	s.nodeMu.Unlock()
	if hn == nil {
		return
	}
	hn.stop()
	n, err := s.store.GetNode(id)
	if err != nil {
		_ = hn.link.Close()
		return
	}
	s.keepNode(n, hn.link)
}

// nodeLinkOf returns the link home holds to node id.
func (s *Service) nodeLinkOf(id string) (*nodelink.Link, bool) {
	s.nodeMu.Lock()
	defer s.nodeMu.Unlock()
	hn, ok := s.nodeLinks[id]
	if !ok {
		return nil, false
	}
	return hn.link, true
}

func (s *Service) adoptNode(ctx context.Context, id string, link *nodelink.Link) error {
	var hello nodeHello
	if err := s.callNode(ctx, link, keydeposit.ProofNodeAdopt, id, nodeAdoptPath, nil, &hello); err != nil {
		return err
	}
	if hello.ID != id {
		return fmt.Errorf("the node answered as %s", hello.ID)
	}
	return s.store.SetNodeAdopted(id, time.Now().Unix())
}

// releaseNode tells the node home lets it go. A node that was let go before,
// or never took this instance as its home, has nothing more to hear.
func (s *Service) releaseNode(ctx context.Context, id string, link *nodelink.Link) error {
	err := s.callNode(ctx, link, keydeposit.ProofNodeRelease, id, nodeReleasePath, nil, nil)
	if errors.Is(err, errNodeReleased) || errors.Is(err, errNodeNotAdopted) {
		return nil
	}
	return err
}

// callNode signs a call for node id that says in, sends it over link and
// reads the answer into out unless out is nil.
func (s *Service) callNode(ctx context.Context, link *nodelink.Link, action, id, path string, in, out any) error {
	call := nodeCall{Name: s.pairName()}
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		call.Body = b
	}
	p, err := s.signCall(action, id, call.payload())
	if err != nil {
		return err
	}
	call.peerProof = p
	body, err := json.Marshal(call)
	if err != nil {
		return err
	}
	return postNode(ctx, link, path, body, out)
}

// postNode sends body to path over a node's link and reads the answer into
// out unless out is nil.
func postNode(ctx context.Context, link *nodelink.Link, path string, body []byte, out any) error {
	cctx, cancel := context.WithTimeout(ctx, nodeCallTimeout)
	defer cancel()
	status, answer, err := link.Call(cctx, path, body)
	if err != nil {
		return err
	}
	if status == http.StatusConflict {
		var refusal struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(answer, &refusal)
		switch refusal.Code {
		case codeNodeReleased:
			return errNodeReleased
		case codeNodeNotAdopted:
			return errNodeNotAdopted
		case codeNodeBusy:
			return errNodeRunBusy
		}
	}
	switch {
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
	// A node reaches home over a relay whatever home's members use, so
	// pairing without one still leaves it the project's.
	mode, url := group.Mode(g.RelayMode), strings.TrimSpace(g.RelayURL)
	if mode != group.ModeOwn {
		mode, url = group.ModeProject, ""
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, []byte(appKey))
	if err != nil {
		return NodeView{}, nil, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return NodeView{}, nil, err
	}
	start := "daily " + nodeStartTime(settings.ContainersSchedule, time.Now())
	n := store.Node{ID: id, Name: name, AppKeyEnc: enc, RelayMode: string(mode), RelayURL: url, CreatedAt: time.Now().Unix(),
		ContainersCadence: start, FilesCadence: start}
	compose, err := s.nodeCompose(n)
	if err != nil {
		return NodeView{}, nil, err
	}
	if err := s.store.AddNode(n); err != nil {
		return NodeView{}, nil, err
	}
	link, err := s.nodeLink(n)
	if err != nil {
		_ = s.store.DeleteNode(id)
		return NodeView{}, nil, err
	}
	s.keepNode(n, link)
	return nodeViewOf(n), compose, nil
}

// nodeCompose returns the values node n's compose file takes, its APP_KEY
// among them.
func (s *Service) nodeCompose(n store.Node) (map[string]string, error) {
	appKey, err := s.nodeAppKey(n)
	if err != nil {
		return nil, err
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return nil, err
	}
	seed, err := s.depositSeed()
	if err != nil {
		return nil, err
	}
	home, err := nodelink.EncodeHome(appKey, nodelink.Home{ID: g.InstanceID, Key: keydeposit.OwnerKey(seed), Relay: group.Mode(n.RelayMode), RelayURL: n.RelayURL})
	if err != nil {
		return nil, err
	}
	return map[string]string{"MODE": "node", "APP_KEY": appKey, "NODE_HOME": home, "NODE_NAME": n.Name}, nil
}

// handleNodeCompose hands out the values a node's compose file takes again,
// its APP_KEY among them, to set the node up anew on another server.
// POST /api/nodes/{id}/compose
func (h *Handler) handleNodeCompose(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, r, "showing a node's APP_KEY") {
		return
	}
	n, err := h.store.GetNode(r.PathValue("id"))
	if errors.Is(err, store.ErrNodeNotFound) {
		writeJSON(w, http.StatusNotFound, codedFailEnvelope(err, "not-found"))
		return
	}
	compose, err := h.svc.nodeCompose(n)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"compose": compose}))
}

// ListNodes returns every node of this instance, with whether it is there
// now and what it runs, which each node that is there is asked for.
func (s *Service) ListNodes(ctx context.Context) ([]NodeView, error) {
	nodes, err := s.store.ListNodes()
	if err != nil {
		return nil, err
	}
	dests := map[string]string{}
	if list, err := s.store.ListDestinations(); err == nil {
		for _, d := range list {
			dests[d.ID] = d.Name
		}
	}
	out := make([]NodeView, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		out[i] = nodeViewOf(n)
		if n.Placement == nodePlaceLocation {
			out[i].PlacementTarget = cmp.Or(dests[n.PlacementTarget], n.PlacementTarget)
		}
		if err := s.nodeHealth(&out[i]); err != nil {
			return nil, err
		}
		out[i].LastSeen = n.SeenAt
		link, ok := s.nodeLinkOf(n.ID)
		if !ok {
			continue
		}
		if seen := link.LastSeen(); seen.Unix() > out[i].LastSeen {
			out[i].LastSeen = seen.Unix()
		}
		if out[i].Online = link.Online(); !out[i].Online {
			continue
		}
		wg.Go(func() {
			hctx, cancel := context.WithTimeout(ctx, nodeHelloWait)
			defer cancel()
			var hello nodeHello
			if postNode(hctx, link, nodeHelloPath, nil, &hello) == nil {
				out[i].Version = hello.Version
			}
		})
	}
	wg.Wait()
	return out, nil
}

// ReleaseNode lets the node with id go. told says whether the node heard it
// now; until it does, home keeps it as being released and tells it once it
// is back on the relay.
func (s *Service) ReleaseNode(ctx context.Context, id string) (told bool, err error) {
	n, err := s.store.GetNode(id)
	if err != nil {
		return false, err
	}
	if err := s.store.SetNodeState(id, store.NodeReleasing); err != nil {
		return false, err
	}
	n.State = store.NodeReleasing
	s.nodeMu.Lock()
	hn := s.nodeLinks[id]
	delete(s.nodeLinks, id)
	s.nodeMu.Unlock()
	var link *nodelink.Link
	if hn != nil {
		hn.stop()
		link = hn.link
	} else if link, err = s.nodeLink(n); err != nil {
		return false, err
	}
	if err := s.releaseNode(ctx, id, link); err != nil {
		log.Printf("nodes: %s did not hear it was let go, and hears it once it is back: %v", id, err)
		s.keepNode(n, link)
		return false, nil
	}
	_ = link.Close()
	return true, s.forgetNode(id)
}

// handleListNodes lists the nodes this instance runs backups on.
// GET /api/nodes
func (h *Handler) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.svc.ListNodes(r.Context())
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

// handleReleaseNode lets a node go. told says whether the node heard it now;
// one that did not hears it once it is back.
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
