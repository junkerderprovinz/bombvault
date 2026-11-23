package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// nodeOnwardEpisode is the episode of a node whose section domain could not
// be copied onward.
func nodeOnwardEpisode(domain string) string {
	return "onward:" + domain
}

// SetNodeCopyOnward switches whether home copies the backups of node id to
// its own off-site targets along with its own. A node switched off has no
// failed copy left to report.
func (s *Service) SetNodeCopyOnward(id string, on bool) error {
	n, err := s.store.GetNode(id)
	if err != nil {
		return err
	}
	appKey, err := s.nodeAppKey(n)
	if err != nil {
		return err
	}
	if err := s.store.SetNodeCopyOnward(id, on, restickey.Host(appKey)); err != nil {
		return err
	}
	if on {
		return nil
	}
	for _, d := range nodeDomains {
		if err := s.store.CloseNodeEpisode(id, nodeOnwardEpisode(d)); err != nil {
			return err
		}
	}
	return nil
}

// onwardHosts are the hosts of every node home copied onward. Their
// snapshots at home's off-site targets are guests there.
func (s *Service) onwardHosts() []string {
	hosts, err := s.store.OnwardHosts()
	if err != nil {
		log.Printf("api: offsite: the hosts of the nodes copied onward could not be read: %v", err)
	}
	return hosts
}

// copyNodesOnward copies section domain of every node that follows home's
// copies to the targets home's own pass over the domain went to.
func (s *Service) copyNodesOnward(ctx context.Context, settings store.Settings, domain string) {
	if !slices.Contains(nodeDomains, domain) {
		return
	}
	nodes, err := s.store.ListNodes()
	if err != nil {
		log.Printf("nodes: copying onward: %v", err)
		return
	}
	var targets []store.OffsiteTarget
	for _, n := range nodes {
		if !n.CopyOnward || n.AdoptedAt == 0 || n.State != "" {
			continue
		}
		if targets == nil {
			// The copy rules place home's own items only, so a node goes to
			// every enabled target.
			p, _ := s.readPlacement(settings, domain)
			targets = p.enabledTargets()
		}
		s.noteNodeOnward(ctx, n, domain, s.copyNodeOnward(ctx, settings, n, domain, targets))
	}
}

// onwardSource is where home reads a node's section to copy it onward, how it
// lists it there, and what restic copy needs to read it.
type onwardSource struct {
	loc   string
	mode  restic.Mode
	creds CloudCreds
	from  restic.From
}

// onwardSourceOf is the repository home watches for section domain of n, or
// for a node that keeps its own backups, what home fetched of them, which is
// on home's own disk. ok is false while there is none.
func (s *Service) onwardSourceOf(ctx context.Context, settings store.Settings, n store.Node, domain string) (onwardSource, bool, error) {
	if n.Placement == nodePlaceNode {
		return s.fetchedSourceOf(ctx, settings, n, domain)
	}
	rr, found, err := s.store.GetReceivedRepo(nodeRepoID(n.ID, domain))
	if err != nil || !found || !rr.Enabled {
		return onwardSource{}, false, err
	}
	loc, mode, err := s.receiverOpen(ctx, rr)
	if err != nil {
		return onwardSource{}, false, err
	}
	// Read without a lock, as everything home reads on the receiver: it
	// takes no deletes, so a lock left behind would stay for good.
	from := restic.From{Encrypted: mode.Encrypted, Password: mode.Password, NoLock: true}
	return onwardSource{loc: loc, mode: mode, creds: credsInEnv(mode.Env), from: from}, true, nil
}

func (s *Service) fetchedSourceOf(ctx context.Context, settings store.Settings, n store.Node, domain string) (onwardSource, bool, error) {
	sources, err := s.store.ListPullSources()
	if err != nil {
		return onwardSource{}, false, err
	}
	i := slices.IndexFunc(sources, func(ps store.PullSource) bool { return ps.NodeID == n.ID && ps.Domain == domain })
	if i < 0 {
		return onwardSource{}, false, nil
	}
	domainRepo, err := s.repoFor(settings, domain, "local")
	if err != nil {
		return onwardSource{}, false, err
	}
	loc, mode := s.pullLanding(ctx, sources[i], settings, domain, domainRepo)
	if localRepoMissing(loc) {
		return onwardSource{}, false, nil
	}
	from := restic.From{Encrypted: mode.Encrypted, Password: mode.Password}
	return onwardSource{loc: loc, mode: mode, creds: credsInEnv(mode.Env), from: from}, true, nil
}

// copyNodeOnward copies section domain of n to each target and ages its
// copies there by the node's own rule.
func (s *Service) copyNodeOnward(ctx context.Context, settings store.Settings, n store.Node, domain string, targets []store.OffsiteTarget) error {
	src, ok, err := s.onwardSourceOf(ctx, settings, n, domain)
	if err != nil || !ok {
		return err
	}
	appKey, err := s.nodeAppKey(n)
	if err != nil {
		return err
	}
	host := restickey.Host(appKey)
	snaps, err := s.listSnapshots(ctx, src.loc, src.mode)
	if err != nil {
		return fmt.Errorf("reading the node's backups: %w", err)
	}
	snaps = underHost(snaps, host)
	p := s.nodeKeepPolicy(settings, n, domain)
	var errs []error
	for _, t := range targets {
		if err := s.copyNodeTo(ctx, settings, domain, t, src, host, snaps, p); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", placementTargetName(t), err))
		}
	}
	return errors.Join(errs...)
}

// copyNodeTo copies to target t what of snaps it does not hold yet, under the
// node's host, and ages the node's copies there by p. It leaves out what p
// would forget right after, as home's own copies do.
func (s *Service) copyNodeTo(ctx context.Context, settings store.Settings, domain string, t store.OffsiteTarget, src onwardSource,
	host string, snaps []restic.Snapshot, p restic.RetentionPolicy) error {
	mode := s.offsiteModeForTarget(settings, t)
	if mode.Sharing != restic.SharingNone {
		// Its owner ages a shared repository by rules that know only the
		// servers of the group.
		log.Printf("nodes: %s is shared with other servers, so no node is copied onward to it", placementTargetName(t)) //nolint:gosec // G706: the name is the row's own
		return nil
	}
	dest, err := s.resolveRepo(t.Repo)
	if err != nil {
		return err
	}
	if sameRepoLocation(src.loc, dest) {
		return nil
	}
	mode.Guests, mode.Host, mode.Sharing = nil, host, restic.SharingOwner
	held, listErr, err := s.listDestination(ctx, dest, mode)
	if err != nil {
		return err
	}
	if listErr != nil {
		return listErr
	}
	defer makeLocalRepoReadable(dest, s.cfg.DataDir)
	ages := p.Any() && !t.Immutable
	send := pendingSnapshots(snaps, held)
	if ages {
		gone := forgottenPerEntry(append(slices.Clone(held), send...), p)
		send = slices.DeleteFunc(send, func(sn restic.Snapshot) bool { return gone[sn.ID] })
	}
	loc, copyMode, err := sourceAccessWith(src.creds, src.loc, dest, mode)
	if err != nil {
		return err
	}
	copyMode.From = &src.from
	landed, err := s.copyInChunks(ctx, domain, dest, loc, send, targetOffsiteLimits(t), copyMode, 0)
	if err != nil {
		return err
	}
	if !ages {
		return nil
	}
	return s.ageNodeCopies(ctx, dest, mode, p, append(held, landed...))
}

// forgottenPerEntry predicts what p forgets of snaps, one entry at a time.
// An entry whose outcome cannot be predicted loses nothing here.
func forgottenPerEntry(snaps []restic.Snapshot, p restic.RetentionPolicy) map[string]bool {
	out := map[string]bool{}
	for _, tag := range identityTags(snaps) {
		if removed, ok := p.Forgets(snapshotsTagged(snaps, tag)); ok {
			for id := range removed {
				out[id] = true
			}
		}
	}
	return out
}

// ageNodeCopies ages the node's copies in dest by p, one entry at a time, and
// prunes once when anything went. mode selects the node's host alone.
func (s *Service) ageNodeCopies(ctx context.Context, dest string, mode restic.Mode, p restic.RetentionPolicy, snaps []restic.Snapshot) error {
	forgot := false
	for _, tag := range identityTags(snaps) {
		if removed, ok := p.Forgets(snapshotsTagged(snaps, tag)); ok && len(removed) == 0 {
			continue
		}
		if err := s.forgetWithLockHeal(ctx, dest, p, mode, []string{tag}, false); err != nil {
			return err
		}
		forgot = true
	}
	if !forgot {
		return nil
	}
	return s.retryAfterUnlock(ctx, dest, mode, func() error { return s.engine.Prune(ctx, dest, mode) })
}

// noteNodeOnward opens the node's episode when copying section domain onward
// failed, with one alert, and closes it once a copy went through.
func (s *Service) noteNodeOnward(ctx context.Context, n store.Node, domain string, err error) {
	if err != nil {
		log.Printf("nodes: %s: copying the %s onward: %s", n.Name, domain, scrubError(err)) //nolint:gosec // G706: the name is one home gave the node, domain a fixed literal
	}
	key := nodeOnwardEpisode(domain)
	open, oErr := s.store.NodeEpisodes(n.ID)
	if oErr != nil {
		log.Printf("nodes: %s: %v", n.Name, oErr) //nolint:gosec // G706: the name is one home gave the node
		return
	}
	_, isOpen := open[key]
	switch {
	case err == nil && isOpen:
		oErr = s.store.CloseNodeEpisode(n.ID, key)
	case err != nil && !isOpen:
		section := map[string]string{"containers": "containers", "files": "folder sets"}[domain]
		s.notifyNode(ctx, "BombVault: "+n.Name+": off-site copy failed",
			fmt.Sprintf("%s: the off-site copy of the node's %s failed: %s", n.Name, section, truncateRunErr(err)))
		oErr = s.store.OpenNodeEpisode(n.ID, key, time.Now().Unix())
	}
	if oErr != nil {
		log.Printf("nodes: %s: %v", n.Name, oErr) //nolint:gosec // G706: the name is one home gave the node
	}
}

// nodeCopiesTo names the targets of the sections a node backs up, where home
// copies a node that follows its copies, as Storage names them: by the
// storage location a target follows, or by its address. A row's own name is
// often just its section's.
func (s *Service) nodeCopiesTo(settings store.Settings) []string {
	var out []string
	for _, d := range nodeDomains {
		p, _ := s.readPlacement(settings, d)
		for _, t := range p.enabledTargets() {
			name := scrubRepoLocation(t.Repo)
			if loc, found, err := s.store.GetDestination(t.DestinationID); err == nil && found {
				name = loc.Name
			}
			if !slices.Contains(out, name) {
				out = append(out, name)
			}
		}
	}
	return out
}

// handleNodeCopyOnward switches whether home copies a node's backups onward.
// PUT /api/nodes/{id}/copy-onward
func (h *Handler) handleNodeCopyOnward(w http.ResponseWriter, r *http.Request) {
	var body struct {
		On bool `json:"on"`
	}
	if !decodeBody(w, r, &body) || !h.requireSignIn(w, r, "changing where a node's backups are copied") {
		return
	}
	err := h.svc.SetNodeCopyOnward(r.PathValue("id"), body.On)
	switch {
	case errors.Is(err, store.ErrNodeNotFound):
		writeJSON(w, http.StatusNotFound, codedFailEnvelope(err, "not-found"))
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(nil))
	}
}
