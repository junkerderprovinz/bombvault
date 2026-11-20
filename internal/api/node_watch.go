package api

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
	kitgroup "github.com/junkerderprovinz/parleyport/kit/group"
)

// nodePollEvery is how often home asks a node how its runs went, and how long
// it leaves a node that was busy before it asks it to start again.
const nodePollEvery = 5 * time.Minute

// nodeAwayAlertAfter is how long a node may be out of reach before home says
// so, while none of its repositories has a dead man's switch of its own.
const nodeAwayAlertAfter = 6 * time.Hour

// nodeRunsAsked is how many of its newest runs a node is asked for.
const nodeRunsAsked = 50

// nodeRunLongest is how long a run a node reported as under way holds the
// prune of its repositories off. One older than that is taken for a run the
// node lost track of.
const nodeRunLongest = 24 * time.Hour

// The prune window home ages and prunes its nodes' repositories in when the
// place they lie at has no schedule of its own, as a shared repository's owner
// does by default. Home starts no run on a node while the window is open and
// it has not pruned there yet.
const (
	nodePruneCadence = "weekly sun 03:00"
	nodePruneHours   = 6
)

// nodeAwayEpisode is the episode of a node out of reach.
const nodeAwayEpisode = "away"

// nodeFailedEpisode is the episode of an entry of a node whose backups fail.
func nodeFailedEpisode(domain, target string) string {
	return "failed:" + domain + ":" + target
}

// nodeWatch keeps the schedules home holds for its nodes, learns how their
// runs went and alerts once per episode. call reaches a node as home's signed
// calls do, alert sends a message, prune ages and prunes a node's
// repositories, and fetch fetches a section of a node that keeps its own.
type nodeWatch struct {
	store *store.Repo
	call  func(ctx context.Context, id, action, path string, in, out any) error
	alert func(ctx context.Context, title, message string)
	prune func(ctx context.Context, n store.Node, domain string) error
	fetch func(n store.Node, domain string)

	// ticking lets a tick go by while the one before still runs, such as one
	// waiting on a slow relay.
	ticking sync.Mutex
	mu      sync.Mutex
	// asked is when home last asked each node how its runs went, held when a
	// busy node may be asked to start a section again, open the open
	// anomalies each node last reported, and pruning the repositories a prune
	// runs in.
	asked   map[string]time.Time
	held    map[string]time.Time
	open    map[string]int
	pruning map[string]bool
}

func newNodeWatch(st *store.Repo, call func(ctx context.Context, id, action, path string, in, out any) error,
	alert func(ctx context.Context, title, message string), prune func(ctx context.Context, n store.Node, domain string) error,
	fetch func(n store.Node, domain string)) *nodeWatch {
	return &nodeWatch{store: st, call: call, alert: alert, prune: prune, fetch: fetch,
		asked: map[string]time.Time{}, held: map[string]time.Time{}, open: map[string]int{}, pruning: map[string]bool{}}
}

// Tick runs for every adopted node what is due at now: learning how its runs
// went, the prune of its repositories and the starts of its sections.
func (w *nodeWatch) Tick(ctx context.Context, now time.Time) error {
	if !w.ticking.TryLock() {
		return nil
	}
	defer w.ticking.Unlock()
	nodes, err := w.store.ListNodes()
	if err != nil {
		return err
	}
	for _, n := range nodes {
		if n.AdoptedAt == 0 || n.State != "" {
			continue
		}
		if err := w.tickNode(ctx, n, now); err != nil {
			log.Printf("nodes: %s: %v", n.Name, err) //nolint:gosec // G706: the name is one home gave the node
		}
	}
	return nil
}

func (w *nodeWatch) tickNode(ctx context.Context, n store.Node, now time.Time) error {
	if w.pollDue(n.ID, now) {
		reached := w.poll(ctx, n, now)
		if reached {
			n.SeenAt = now.Unix()
		}
		if err := w.raise(ctx, n, reached, now); err != nil {
			return err
		}
	}
	if window, open := w.pruneWindow(n, now); open {
		runs, err := w.store.NodeRuns(n.ID)
		if err != nil {
			return err
		}
		if !nodeRunning(runs, now) {
			w.pruneInBackground(ctx, n, window)
		}
		return nil
	}
	return w.start(ctx, n, now)
}

// pruneInBackground prunes each repository of n unless a prune runs there
// already, and records the window once all are done. The node starts no run
// until then.
func (w *nodeWatch) pruneInBackground(ctx context.Context, n store.Node, window time.Time) {
	repos := make([]string, len(nodeDomains))
	for i, d := range nodeDomains {
		repos[i] = n.ID + "/" + d
	}
	w.mu.Lock()
	if slices.ContainsFunc(repos, func(r string) bool { return w.pruning[r] }) {
		w.mu.Unlock()
		return
	}
	for _, r := range repos {
		w.pruning[r] = true
	}
	w.mu.Unlock()
	ctx = context.WithoutCancel(ctx)
	go func() {
		for i, d := range nodeDomains {
			if err := w.prune(ctx, n, d); err != nil {
				log.Printf("nodes: %s: the prune of its %s failed and waits for the next window: %v", n.Name, d, err) //nolint:gosec // G706: the name is one home gave the node
			}
			w.mu.Lock()
			delete(w.pruning, repos[i])
			w.mu.Unlock()
		}
		if err := w.store.SetNodePruned(n.ID, window.Unix()); err != nil {
			log.Printf("nodes: %s: %v", n.Name, err) //nolint:gosec // G706: the name is one home gave the node
		}
	}()
}

// Next is when the soonest section of a node comes up next.
func (w *nodeWatch) Next(now time.Time) (time.Time, bool) {
	nodes, err := w.store.ListNodes()
	if err != nil {
		return time.Time{}, false
	}
	var soonest time.Time
	for _, n := range nodes {
		if n.AdoptedAt == 0 || n.State != "" {
			continue
		}
		for _, d := range nodeDomains {
			cad, err := schedule.ParseCadence(n.Cadence(d))
			if err != nil {
				continue
			}
			if next, ok := cad.NextDue(nodeStartedSince(n, d), now); ok && (soonest.IsZero() || next.Before(soonest)) {
				soonest = next
			}
		}
	}
	return soonest, !soonest.IsZero()
}

func nodeStartedSince(n store.Node, domain string) time.Time {
	return time.Unix(max(n.StartedAt(domain), n.AdoptedAt), 0)
}

// start asks the node to back up each section with a start due. One the node
// cannot take now, being away or busy, stays due.
func (w *nodeWatch) start(ctx context.Context, n store.Node, now time.Time) error {
	if n.Placement == "" || n.PlacementDue {
		return nil
	}
	if _, open := w.pruneWindow(n, now); open {
		return nil
	}
	for _, d := range nodeDomains {
		cad, err := schedule.ParseCadence(n.Cadence(d))
		if err != nil || !cad.FireDue(nodeStartedSince(n, d), now) || now.Before(w.heldUntil(n.ID, d)) {
			continue
		}
		var ans nodeRunAnswer
		err = w.call(ctx, n.ID, keydeposit.ProofNodeRun, nodeRunPath, nodeRunCall{Domain: d}, &ans)
		switch {
		case errors.Is(err, errNodeRunBusy):
			w.hold(n.ID, d, now.Add(nodePollEvery))
		case errors.Is(err, errNodeAway):
		case err != nil:
			log.Printf("nodes: %s: the %s did not start: %v", n.Name, d, err) //nolint:gosec // G706: the name is one home gave the node
		default:
			if err := w.store.SetNodeStarted(n.ID, d, now.Unix()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *nodeWatch) pollDue(id string, now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	last, ok := w.asked[id]
	if ok && now.Sub(last) < nodePollEvery && !now.Before(last) {
		return false
	}
	w.asked[id] = now
	return true
}

func (w *nodeWatch) heldUntil(id, domain string) time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.held[id+"/"+domain]
}

func (w *nodeWatch) hold(id, domain string, until time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.held[id+"/"+domain] = until
}

// anomalies is how many critical and warning anomalies node id had open when
// home last asked.
func (w *nodeWatch) anomalies(id string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.open[id]
}

// nodeRunReport is a run as a node's GET /api/runs lists it.
type nodeRunReport struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt *int64 `json:"finishedAt"`
	Error      string `json:"error"`
	Target     string `json:"target"`
	Domain     string `json:"domain"`
}

// nodeRunDomains maps the domain a node's run list names to its section.
var nodeRunDomains = map[string]string{"container": "containers", "files": "files"}

// poll asks the node how its backups went and what anomalies it has open,
// keeps what it said and reports whether the node answered. Each section of a
// node that keeps its own backups is fetched once its runs have ended; the
// hourly fetch catches one this misses.
func (w *nodeWatch) poll(ctx context.Context, n store.Node, now time.Time) bool {
	var listed struct {
		Runs []nodeRunReport `json:"runs"`
	}
	if err := w.api(ctx, n.ID, "/api/runs", fmt.Sprintf("limit=%d", nodeRunsAsked), &listed); err != nil {
		if !errors.Is(err, errNodeAway) {
			log.Printf("nodes: %s: its runs could not be read: %v", n.Name, err) //nolint:gosec // G706: the name is one home gave the node
		}
		return false
	}
	var runs []store.NodeRun
	for _, r := range listed.Runs {
		domain, ok := nodeRunDomains[r.Domain]
		if !ok || r.Kind != "backup" {
			continue
		}
		run := store.NodeRun{ID: r.ID, Domain: domain, Target: r.Target, Status: r.Status, StartedAt: r.StartedAt, Error: r.Error}
		if r.FinishedAt != nil {
			run.FinishedAt = *r.FinishedAt
		}
		runs = append(runs, run)
	}
	before, readErr := w.store.NodeRuns(n.ID)
	if readErr != nil {
		log.Printf("nodes: %s: %v", n.Name, readErr) //nolint:gosec // G706: the name is one home gave the node
	}
	if err := w.store.SaveNodeRuns(n.ID, runs); err != nil {
		log.Printf("nodes: %s: its runs could not be kept: %v", n.Name, err) //nolint:gosec // G706: the name is one home gave the node
	}
	if n.Placement == nodePlaceNode && readErr == nil {
		for _, d := range roundsEnded(before, runs, now) {
			w.fetch(n, d)
		}
	}
	var summary struct {
		Summary AnomalySummary `json:"summary"`
	}
	if err := w.api(ctx, n.ID, "/api/anomalies/summary", "", &summary); err == nil {
		w.mu.Lock()
		w.open[n.ID] = summary.Summary.Open.Critical + summary.Summary.Open.Warning
		w.mu.Unlock()
	}
	if err := w.store.SetNodeSeen(n.ID, now.Unix()); err != nil {
		log.Printf("nodes: %s: %v", n.Name, err) //nolint:gosec // G706: the name is one home gave the node
	}
	return true
}

// roundsEnded are the sections in which a run ended that had not ended in
// before, and no run is under way any more.
func roundsEnded(before, runs []store.NodeRun, now time.Time) []string {
	ended := map[string]bool{}
	for _, r := range before {
		if r.FinishedAt != 0 {
			ended[r.ID] = true
		}
	}
	var out []string
	for _, r := range runs {
		if r.FinishedAt == 0 || ended[r.ID] || slices.Contains(out, r.Domain) {
			continue
		}
		section := slices.DeleteFunc(slices.Clone(runs), func(o store.NodeRun) bool { return o.Domain != r.Domain })
		if !nodeRunning(section, now) {
			out = append(out, r.Domain)
		}
	}
	return out
}

// api reads the JSON a node's own route at path answers with into out.
func (w *nodeWatch) api(ctx context.Context, id, path, query string, out any) error {
	var ans nodeAPIAnswer
	if err := w.call(ctx, id, keydeposit.ProofNodeAPI, nodeAPIPath, nodeAPIRequest{Method: http.MethodGet, Path: path, Query: query}, &ans); err != nil {
		return err
	}
	if ans.Status != http.StatusOK {
		return fmt.Errorf("the node answered %s with HTTP %d", path, ans.Status)
	}
	body, err := ans.body()
	if err != nil {
		return errNodeNotJSON
	}
	return json.Unmarshal(body, out)
}

// raise sends an alert for each episode of n that began and closes the ones
// that are over. An episode is kept under a muted policy too, so turning
// alerts on later does not replay an old one.
func (w *nodeWatch) raise(ctx context.Context, n store.Node, reached bool, now time.Time) error {
	open, err := w.store.NodeEpisodes(n.ID)
	if err != nil {
		return err
	}
	runs, err := w.store.NodeRuns(n.ID)
	if err != nil {
		return err
	}
	seen := nodeSighting{now: now.Unix(), seenAt: max(n.SeenAt, n.AdoptedAt), awayAfter: int64(w.awayAfter(n).Seconds()), reached: reached, runs: runs}
	opened, closed := nodeEpisodes(open, seen)
	for _, ep := range opened {
		title, message := nodeAlertText(n.Name, ep, seen)
		w.alert(ctx, title, message)
		if err := w.store.OpenNodeEpisode(n.ID, ep.key, now.Unix()); err != nil {
			return err
		}
	}
	for _, key := range closed {
		if err := w.store.CloseNodeEpisode(n.ID, key); err != nil {
			return err
		}
	}
	return nil
}

// nodeSighting is what home learned of a node at one look: whether it
// answered, when it last did, how long it may be away, and its runs, newest
// first.
type nodeSighting struct {
	now       int64
	seenAt    int64
	awayAfter int64
	reached   bool
	runs      []store.NodeRun
}

// nodeEpisode is an episode that begins, with the failed run behind it.
type nodeEpisode struct {
	key string
	run store.NodeRun
}

// nodeEpisodes decides which episodes of a node begin and which are over,
// given the ones open and what home learned of it. An entry's newest finished
// run decides its episode: a failure opens one, a success closes it.
func nodeEpisodes(open map[string]int64, seen nodeSighting) (opened []nodeEpisode, closed []string) {
	_, away := open[nodeAwayEpisode]
	switch {
	case seen.reached && away:
		closed = append(closed, nodeAwayEpisode)
	case !seen.reached && !away && seen.now-seen.seenAt >= seen.awayAfter:
		opened = append(opened, nodeEpisode{key: nodeAwayEpisode})
	}
	decided := map[string]bool{}
	for _, r := range seen.runs {
		key := nodeFailedEpisode(r.Domain, r.Target)
		if decided[key] {
			continue
		}
		_, isOpen := open[key]
		switch r.Status {
		case "failed":
			if !isOpen {
				opened = append(opened, nodeEpisode{key: key, run: r})
			}
		case "success", "warning":
			if isOpen {
				closed = append(closed, key)
			}
		default:
			continue
		}
		decided[key] = true
	}
	slices.SortFunc(opened, func(a, b nodeEpisode) int { return cmp.Compare(a.key, b.key) })
	slices.Sort(closed)
	return opened, closed
}

// nodeAlertText is the alert of an episode that began on the node called name.
func nodeAlertText(name string, ep nodeEpisode, seen nodeSighting) (title, message string) {
	if ep.key == nodeAwayEpisode {
		return "BombVault: " + name + " is out of reach",
			fmt.Sprintf("%s: home last reached the node %s. Its scheduled backups wait until it is back.", name, digestAge(seen.now, seen.seenAt))
	}
	section := map[string]string{"containers": "container", "files": "folder set"}[ep.run.Domain]
	detail := cmp.Or(strings.TrimSpace(ep.run.Error), "no reason given")
	return "BombVault: " + name + ": backup failed",
		fmt.Sprintf("%s: the backup of the %s %s failed: %s", name, section, ep.run.Target, detail)
}

// pruneCadence opens the prune window of n. At a storage location home copies
// its own backups to, it is the off-site schedule of a section that copies
// there, since home prunes there right after each copy.
func (w *nodeWatch) pruneCadence(n store.Node) string {
	if n.Placement != nodePlaceLocation {
		return nodePruneCadence
	}
	targets, err := w.store.DestinationTargets(n.PlacementTarget)
	if err != nil {
		return nodePruneCadence
	}
	settings, err := w.store.GetSettings()
	if err != nil {
		return nodePruneCadence
	}
	for _, t := range targets {
		own := offsiteScheduleFromSettings(t.Domain, settings)
		if cad, err := schedule.ParseCadence(own); err == nil && cad.Enabled && t.Enabled {
			return own
		}
	}
	return nodePruneCadence
}

// nodeRunning reports whether a run a node reported is still under way.
func nodeRunning(runs []store.NodeRun, now time.Time) bool {
	return slices.ContainsFunc(runs, func(r store.NodeRun) bool {
		return r.Status == "running" && now.Sub(time.Unix(r.StartedAt, 0)) < nodeRunLongest
	})
}

// awayAfter is how long n may be out of reach: the shortest dead man's switch
// the receiver watch keeps on its repositories.
func (w *nodeWatch) awayAfter(n store.Node) time.Duration {
	hours := 0
	for _, d := range nodeDomains {
		rr, found, err := w.store.GetReceivedRepo(nodeRepoID(n.ID, d))
		if err == nil && found && rr.Enabled && rr.DeadManHours > 0 && (hours == 0 || rr.DeadManHours < hours) {
			hours = rr.DeadManHours
		}
	}
	if hours == 0 {
		return nodeAwayAlertAfter
	}
	return time.Duration(hours) * time.Hour
}

// pruneWindow is the prune window open at now, if home has not pruned the
// repositories of n in it yet.
func (w *nodeWatch) pruneWindow(n store.Node, now time.Time) (time.Time, bool) {
	cad, err := schedule.ParseCadence(w.pruneCadence(n))
	if err != nil {
		return time.Time{}, false
	}
	start, ok := cad.LastFire(now)
	return start, ok && now.Before(start.Add(nodePruneHours*time.Hour)) && start.Unix() > n.PrunedWindow
}

// callNodeByID signs a call for node id and sends it over home's link to it.
func (s *Service) callNodeByID(ctx context.Context, id, action, path string, in, out any) error {
	link, ok := s.nodeLinkOf(id)
	if !ok || !link.Online() {
		return errNodeAway
	}
	err := s.callNode(ctx, link, action, id, path, in, out)
	if errors.Is(err, kitgroup.ErrUnreachable) {
		return errNodeAway
	}
	return err
}

// notifyNode sends an alert about a node through the channels that take one.
func (s *Service) notifyNode(ctx context.Context, title, message string) {
	c, err := s.NotifyConfig()
	if err != nil {
		log.Printf("nodes: the notification settings could not be read, so this alert is not sent: %v", err)
		return
	}
	if c.Active() {
		notify.Send(notify.WithHealthchecksSuppressed(ctx), c, "node", notify.Event{Title: title, Message: message})
	}
}

// RunNodeSchedules is the scheduled tick of the nodes: it starts the sections
// that are due, learns how the runs went and prunes in the window.
func (s *Service) RunNodeSchedules() error {
	return s.nodes.Tick(context.Background(), time.Now())
}

// NextNodeRun is when the soonest section of a node comes up next.
func (s *Service) NextNodeRun(now time.Time) (time.Time, bool) {
	return s.nodes.Next(now)
}
