package api

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/hostsnap"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

const (
	// hostSnapListTimeout bounds the survey, which reads every pool.
	hostSnapListTimeout = 90 * time.Second
	// hostSnapActionTimeout bounds one change, a ZFS destroy of a large
	// snapshot included.
	hostSnapActionTimeout = 5 * time.Minute
	// hostSnapTick is how often the plans are looked at. A cadence names
	// minutes, so a finer tick would only find nothing due.
	hostSnapTick    = time.Minute
	hostSnapMaxKeep = 1000
	hostSnapMaxRefs = 500
)

// hostSnaps is the state of the server's own snapshots in the service.
type hostSnaps struct {
	host *hostsnap.Host
	// runMu keeps two runs of plans from taking and pruning snapshots of one
	// source at the same time.
	runMu sync.Mutex
}

// SetHostSnapshots wires the reach to the server's own snapshots. Without it
// every route answers that the host is out of reach.
func (s *Service) SetHostSnapshots(h *hostsnap.Host) { s.hostSnap.host = h }

var errHostSnapUnavailable = &hostsnap.Error{Code: "ssh-missing"}

func (s *Service) hostSnapHost() (*hostsnap.Host, error) {
	if s.hostSnap.host == nil {
		return nil, errHostSnapUnavailable
	}
	return s.hostSnap.host, nil
}

// HostSnapshotView is the page's whole list: the survey, the plans, and the
// container names the mounts are reported under.
type HostSnapshotView struct {
	hostsnap.Survey
	Plans []store.HostSnapshotPlan `json:"plans"`
}

// HostSnapshots lists the server's snapshots. Without the host every kind
// says so, which is how the page tells a plain Docker host from a broken one.
func (s *Service) HostSnapshots(ctx context.Context, wake bool) (HostSnapshotView, error) {
	plans, err := s.store.ListHostSnapshotPlans()
	if err != nil {
		return HostSnapshotView{}, err
	}
	h, err := s.hostSnapHost()
	if err != nil {
		empty := hostsnap.Survey{
			Kinds: map[hostsnap.Kind]hostsnap.KindState{hostsnap.KindZFS: {Code: "ssh-missing"}, hostsnap.KindBtrfs: {Code: "ssh-missing"}, hostsnap.KindVM: {Code: "ssh-missing"}},
			Pools: []hostsnap.Pool{}, Sources: []hostsnap.Source{}, Snapshots: []hostsnap.Snapshot{},
		}
		return HostSnapshotView{Survey: empty, Plans: plans}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, hostSnapListTimeout)
	defer cancel()
	return HostSnapshotView{Survey: h.Survey(ctx, wake), Plans: plans}, nil
}

// CreateHostSnapshot takes one snapshot by hand.
func (s *Service) CreateHostSnapshot(ctx context.Context, src hostsnap.Source, name string, recursive bool) error {
	h, err := s.hostSnapHost()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, hostSnapActionTimeout)
	defer cancel()
	return h.Create(ctx, src, name, recursive)
}

// RenameHostSnapshot gives a snapshot a new name.
func (s *Service) RenameHostSnapshot(ctx context.Context, ref hostsnap.Ref, name string) error {
	h, err := s.hostSnapHost()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, hostSnapActionTimeout)
	defer cancel()
	return h.Rename(ctx, ref, name)
}

// HoldHostSnapshot places or releases BombVault's hold.
func (s *Service) HoldHostSnapshot(ctx context.Context, ref hostsnap.Ref, on bool) error {
	h, err := s.hostSnapHost()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, hostSnapActionTimeout)
	defer cancel()
	return h.Hold(ctx, ref, on)
}

// EstimateHostSnapshots is what deleting the ZFS snapshots among refs frees.
func (s *Service) EstimateHostSnapshots(ctx context.Context, refs []hostsnap.Ref) (int64, error) {
	h, err := s.hostSnapHost()
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, hostSnapActionTimeout)
	defer cancel()
	return h.Estimate(ctx, refs)
}

// DeleteHostSnapshots deletes and answers for each snapshot.
func (s *Service) DeleteHostSnapshots(ctx context.Context, refs []hostsnap.Ref) ([]hostsnap.Result, error) {
	h, err := s.hostSnapHost()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, hostSnapActionTimeout)
	defer cancel()
	return h.Delete(ctx, refs), nil
}

// WhereHostSnapshot names the mounts that keep a ZFS snapshot busy, with the
// container each one belongs to.
func (s *Service) WhereHostSnapshot(ctx context.Context, ref hostsnap.Ref) ([]hostsnap.Mount, error) {
	h, err := s.hostSnapHost()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, hostSnapActionTimeout)
	defer cancel()
	mounts, err := h.Where(ctx, ref)
	if err != nil {
		return nil, err
	}
	return s.nameMountContainers(ctx, mounts), nil
}

// FreeHostSnapshot unmounts a busy ZFS snapshot wherever it hangs and
// deletes it. The containers keep running.
func (s *Service) FreeHostSnapshot(ctx context.Context, ref hostsnap.Ref) ([]hostsnap.Mount, error) {
	h, err := s.hostSnapHost()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, hostSnapActionTimeout)
	defer cancel()
	mounts, err := h.FreeAndDelete(ctx, ref)
	return s.nameMountContainers(ctx, mounts), err
}

func (s *Service) nameMountContainers(ctx context.Context, mounts []hostsnap.Mount) []hostsnap.Mount {
	need := false
	for _, m := range mounts {
		need = need || m.ContainerID != ""
	}
	if !need || s.docker == nil {
		return mounts
	}
	list, err := s.docker.List(ctx)
	if err != nil {
		return mounts
	}
	for i := range mounts {
		id := mounts[i].ContainerID
		for _, c := range list {
			if id != "" && c.ID != "" && (strings.HasPrefix(id, c.ID) || strings.HasPrefix(c.ID, id)) {
				mounts[i].Container = c.Name
			}
		}
	}
	return mounts
}

// HostSnapshotPlanInput is what a person sets on a plan.
type HostSnapshotPlanInput struct {
	Kind       string `json:"kind"`
	FS         string `json:"fs"`
	Source     string `json:"source"`
	Cadence    string `json:"cadence"`
	Keep       int    `json:"keep"`
	Recursive  bool   `json:"recursive"`
	SkipAsleep bool   `json:"skipAsleep"`
	Enabled    bool   `json:"enabled"`
}

var (
	errPlanCadence = errors.New("the plan needs a schedule that runs")
	errPlanKeep    = errors.New("a plan keeps between 1 and " + strconv.Itoa(hostSnapMaxKeep) + " snapshots")
	errPlanSource  = errors.New("the plan's source is not a dataset, subvolume or VM")
)

func checkPlan(in HostSnapshotPlanInput) error {
	c, err := schedule.ParseCadence(in.Cadence)
	if err != nil || !c.Enabled {
		return errPlanCadence
	}
	if in.Keep < 1 || in.Keep > hostSnapMaxKeep {
		return errPlanKeep
	}
	switch hostsnap.Kind(in.Kind) {
	case hostsnap.KindZFS:
		if zfs.ValidateMemberName(in.Source) != nil {
			return errPlanSource
		}
	case hostsnap.KindBtrfs:
		if !strings.HasPrefix(in.FS, "/") || strings.Contains("/"+in.Source+"/", "/../") || strings.HasPrefix(in.Source, "/") {
			return errPlanSource
		}
	case hostsnap.KindVM:
		if in.Source == "" || strings.HasPrefix(in.Source, "-") {
			return errPlanSource
		}
	default:
		return errPlanSource
	}
	return nil
}

// AddHostSnapshotPlan stores a new plan.
func (s *Service) AddHostSnapshotPlan(in HostSnapshotPlanInput) (store.HostSnapshotPlan, error) {
	if err := checkPlan(in); err != nil {
		return store.HostSnapshotPlan{}, err
	}
	return s.store.AddHostSnapshotPlan(store.HostSnapshotPlan{
		Kind: in.Kind, FS: in.FS, Source: in.Source, Cadence: strings.TrimSpace(in.Cadence), Keep: in.Keep,
		Recursive: in.Recursive && in.Kind == string(hostsnap.KindZFS), SkipAsleep: in.SkipAsleep, Enabled: in.Enabled,
		CreatedAt: time.Now().Unix(),
	})
}

// UpdateHostSnapshotPlan changes a plan's schedule, keep and switches. What
// it snapshots stays.
func (s *Service) UpdateHostSnapshotPlan(id string, in HostSnapshotPlanInput) (store.HostSnapshotPlan, error) {
	p, err := s.store.GetHostSnapshotPlan(id)
	if err != nil {
		return p, err
	}
	in.Kind, in.FS, in.Source = p.Kind, p.FS, p.Source
	if err := checkPlan(in); err != nil {
		return p, err
	}
	p.Cadence, p.Keep, p.SkipAsleep, p.Enabled = strings.TrimSpace(in.Cadence), in.Keep, in.SkipAsleep, in.Enabled
	p.Recursive = in.Recursive && p.Kind == string(hostsnap.KindZFS)
	return p, s.store.UpdateHostSnapshotPlan(p)
}

// DeleteHostSnapshotPlan removes a plan and leaves its snapshots.
func (s *Service) DeleteHostSnapshotPlan(id string) error {
	return s.store.DeleteHostSnapshotPlan(id)
}

// RunHostSnapshotPlan runs one plan now and records what it did.
func (s *Service) RunHostSnapshotPlan(ctx context.Context, id string) (hostsnap.RunResult, error) {
	p, err := s.store.GetHostSnapshotPlan(id)
	if err != nil {
		return hostsnap.RunResult{}, err
	}
	h, err := s.hostSnapHost()
	if err != nil {
		return hostsnap.RunResult{}, err
	}
	return s.runHostSnapshotPlan(ctx, h, p, time.Now()), nil
}

func (s *Service) runHostSnapshotPlan(ctx context.Context, h *hostsnap.Host, p store.HostSnapshotPlan, now time.Time) hostsnap.RunResult {
	s.hostSnap.runMu.Lock()
	defer s.hostSnap.runMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, hostSnapActionTimeout)
	defer cancel()
	res := h.RunPlan(ctx, hostsnap.Plan{
		ID: p.ID, Kind: hostsnap.Kind(p.Kind), FS: p.FS, Source: p.Source,
		Recursive: p.Recursive, Keep: p.Keep, SkipAsleep: p.SkipAsleep,
	}, now)
	detail := res.Detail
	if res.Code == "ok" {
		detail = strconv.Itoa(len(res.Removed))
	}
	if err := s.store.RecordHostSnapshotRun(p.ID, now.Unix(), res.Code, detail); err != nil {
		log.Printf("api: snapshots: recording the run of plan %s: %v", p.ID, err)
	}
	s.notifyHostSnapshotPlan(ctx, p, res)
	if res.Code != "ok" && res.Code != hostsnap.CodeSkippedAsleep {
		log.Printf("api: snapshots: plan %s on %s: %s %s", p.ID, p.Source, res.Code, scrubError(errors.New(res.Detail))) //nolint:gosec // G706: ids and codes only, the detail is scrubbed
	}
	return res
}

// dueHostSnapshotPlans picks the plans whose cadence fired since they last
// ran. A plan that never ran counts from when it was made, so a new plan
// waits for its first slot instead of firing at once.
func dueHostSnapshotPlans(plans []store.HostSnapshotPlan, now time.Time) []store.HostSnapshotPlan {
	var due []store.HostSnapshotPlan
	for _, p := range plans {
		if !p.Enabled {
			continue
		}
		c, err := schedule.ParseCadence(p.Cadence)
		if err != nil {
			continue
		}
		since := p.LastRunAt
		if since == 0 {
			since = p.CreatedAt
		}
		if c.FireDue(time.Unix(since, 0), now) {
			due = append(due, p)
		}
	}
	return due
}

// StartHostSnapshotPlans runs the plans as their cadences come due, until ctx
// ends.
func (s *Service) StartHostSnapshotPlans(ctx context.Context) {
	go func() {
		t := time.NewTicker(hostSnapTick)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				h, err := s.hostSnapHost()
				if err != nil {
					continue
				}
				plans, err := s.store.ListHostSnapshotPlans()
				if err != nil {
					log.Printf("api: snapshots: reading the plans: %v", err)
					continue
				}
				for _, p := range dueHostSnapshotPlans(plans, now) {
					s.runHostSnapshotPlan(ctx, h, p, now)
				}
			}
		}
	}()
}

// HostSnapshotPlans lists the plans without reaching the host.
func (s *Service) HostSnapshotPlans() ([]store.HostSnapshotPlan, error) {
	return s.store.ListHostSnapshotPlans()
}

// hostSnapPlanLabel names a plan's source the way a person reads it.
func hostSnapPlanLabel(p store.HostSnapshotPlan) string {
	switch hostsnap.Kind(p.Kind) {
	case hostsnap.KindBtrfs:
		if p.Source == "" {
			return p.FS
		}
		return p.FS + "/" + p.Source
	case hostsnap.KindVM:
		return "the VM " + p.Source
	}
	return p.Source
}

// hostSnapPlanEvent is what a run of p tells, and false when it tells
// nothing: a run that went through like the one before it, or one left out
// while the disks slept. A run that goes through after a failure says so,
// which closes the failure where a hub keeps track of it.
func hostSnapPlanEvent(p store.HostSnapshotPlan, res hostsnap.RunResult) (notify.Event, bool) {
	key := "snapshot-plan:" + p.ID
	label := hostSnapPlanLabel(p)
	reason := res.Detail
	if reason == "" {
		reason = res.Code
	}
	failedBefore := p.LastCode != "" && p.LastCode != "ok" && p.LastCode != hostsnap.CodeSkippedAsleep
	code := res.Code
	if code != "ok" && res.Created != "" {
		code = hostsnap.CodeKeptSome
	}
	switch code {
	case hostsnap.CodeSkippedAsleep:
		return notify.Event{}, false
	case "ok":
		if !failedBefore {
			return notify.Event{}, false
		}
		return notify.Event{
			Title:   "BombVault: snapshot schedule works again",
			Message: "The snapshot schedule for " + label + " took its snapshot again.",
			OK:      true, Key: key,
		}, true
	case hostsnap.CodeKeptSome:
		return notify.Event{
			Title:   "BombVault: snapshot schedule kept old snapshots",
			Message: "The snapshot schedule for " + label + " took its snapshot, but some old ones could not be removed: " + reason,
			OK:      true, Warning: true, Key: key,
		}, true
	}
	return notify.Event{
		Title:   "BombVault: snapshot schedule failed",
		Message: "The snapshot schedule for " + label + " took no snapshot: " + reason,
		Key:     key,
	}, true
}

// notifyHostSnapshotPlan tells what a run of p did, where it tells anything.
// Snapshots have no health check of their own, so none is pinged.
func (s *Service) notifyHostSnapshotPlan(ctx context.Context, p store.HostSnapshotPlan, res hostsnap.RunResult) {
	ev, send := hostSnapPlanEvent(p, res)
	if !send {
		return
	}
	c, err := s.NotifyConfig()
	if err != nil || !c.Active() {
		return
	}
	notify.Send(notify.WithHealthchecksSuppressed(context.WithoutCancel(ctx)), c, "snapshots", ev)
}
