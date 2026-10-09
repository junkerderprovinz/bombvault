package api

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// ZFSReplicaView is one item's replica as its card shows it.
type ZFSReplicaView struct {
	Target      zfsReplicaTargetExport   `json:"target"`
	AfterBackup bool                     `json:"afterBackup"`
	Cadence     string                   `json:"cadence"`
	Keep        store.ZFSReplicaKeep     `json:"keep"`
	State       string                   `json:"state"`
	Code        string                   `json:"code"`
	LastRun     string                   `json:"lastRun"`
	LastBytes   int64                    `json:"lastBytes"`
	LastSeconds int64                    `json:"lastSeconds"`
	Snapshots   []ZFSReplicaSnapshotView `json:"snapshots"`
	Members     []ZFSReplicaMemberView   `json:"members"`
	// PeerState is what a paired instance answered, empty for any other
	// target.
	PeerState string `json:"peerState"`
}

// ZFSReplicaSnapshotView is one replica snapshot kept on the target.
type ZFSReplicaSnapshotView struct {
	Name    string `json:"name"`
	Created string `json:"created"`
}

// ZFSReplicaMemberView is one dataset or volume of the item and where its
// copy lives.
type ZFSReplicaMemberView struct {
	Dataset    string `json:"dataset"`
	Volume     bool   `json:"volume"`
	TargetPath string `json:"targetPath"`
	State      string `json:"state"`
	Code       string `json:"code"`
	LastBytes  int64  `json:"lastBytes"`
}

// The states a replica and each of its members can be in.
const (
	zfsReplicaNever   = "never"
	zfsReplicaRunning = "running"
	zfsReplicaOK      = "ok"
	zfsReplicaFailed  = "failed"
	zfsReplicaWaiting = "waiting"
)

func rfc3339(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}

// ZFSReplicaView reads an item's replica from the store, and the snapshots
// kept on the target from the target itself. A target that does not answer
// in time shows none.
func (s *Service) ZFSReplicaView(ctx context.Context, id string) (ZFSReplicaView, error) {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return ZFSReplicaView{}, fmt.Errorf("zfs replica: load dataset: %w", err)
	}
	rep := d.Replica
	v := ZFSReplicaView{
		Target:      zfsReplicaTargetExport{Kind: rep.TargetKind, ID: rep.TargetID},
		AfterBackup: rep.AfterBackup,
		Cadence:     rep.Cadence,
		Keep:        rep.Keep,
		State:       zfsReplicaNever,
		Snapshots:   []ZFSReplicaSnapshotView{},
		Members:     []ZFSReplicaMemberView{},
	}
	run, hasRun, err := s.store.LatestZFSReplicaRun(id)
	if err != nil {
		return ZFSReplicaView{}, err
	}
	if hasRun {
		v.LastRun, v.LastBytes = rfc3339(run.StartedAt), run.Bytes
		if run.FinishedAt != nil {
			v.LastRun, v.LastSeconds = rfc3339(*run.FinishedAt), *run.FinishedAt-run.StartedAt
		}
		v.State, v.Code = zfsReplicaFailed, zfsRunCode(run.Error)
		if run.Status == "success" {
			v.State = zfsReplicaOK
		}
	}
	running := s.zfsReplicaRunning(id)
	switch {
	case running:
		v.State = zfsReplicaRunning
	case s.zfsReplicaRunRefusal(d) != nil && rep.TargetKind != store.ZFSReplicaTargetNone:
		v.State = zfsReplicaWaiting
	}
	if v.Members, err = s.zfsReplicaMemberViews(d, run, hasRun, running); err != nil {
		return ZFSReplicaView{}, err
	}
	if rep.TargetKind != store.ZFSReplicaTargetNone && !running {
		v.Snapshots = s.zfsReplicaSnapshotViews(ctx, d)
	}
	// Listing a peer target's snapshots asks that instance first, which may
	// have brought a new answer.
	if rep.TargetKind == store.ZFSReplicaTargetPeer {
		if fresh, err := s.store.GetZFSDataset(id); err == nil {
			d = fresh
		}
		v.PeerState = s.zfsReplicaPeerState(d)
	}
	return v, nil
}

// zfsReplicaMemberViews lists the members the replica knows about: the ones
// it has state for, the ones the last run touched, and before any run the
// datasets the last look at the tree found.
func (s *Service) zfsReplicaMemberViews(d store.ZFSDataset, run store.ZFSReplicaRun, hasRun, running bool) ([]ZFSReplicaMemberView, error) {
	states, err := s.store.ListZFSReplicaStates(d.ID)
	if err != nil {
		return nil, err
	}
	byName := map[string]*ZFSReplicaMemberView{}
	add := func(dataset string) *ZFSReplicaMemberView {
		if m, ok := byName[dataset]; ok {
			return m
		}
		m := &ZFSReplicaMemberView{Dataset: dataset, State: zfsReplicaNever}
		byName[dataset] = m
		return m
	}
	for _, st := range states {
		m := add(st.Dataset)
		m.Volume, m.TargetPath, m.State = st.Volume, st.TargetPath, zfsReplicaOK
	}
	if hasRun {
		for _, rm := range run.Members {
			m := add(rm.Dataset)
			m.Code, m.LastBytes = rm.Code, rm.Bytes
			m.State = zfsReplicaOK
			if rm.Code != "" {
				m.State = zfsReplicaFailed
			}
		}
	}
	if len(byName) == 0 {
		tree, err := s.store.ListZFSMembers(d.ID)
		if err != nil {
			return nil, err
		}
		for _, t := range tree {
			if t.Outcome != "excluded" {
				add(t.Dataset).Volume = t.Outcome == "zvol"
			}
		}
	}
	// A run cut short by BombVault stopping never said how the members after
	// its last finished one fared, and the one it was sending may have a
	// partial receive waiting on the target.
	if hasRun && zfsRunCode(run.Error) == "interrupted" {
		finished := map[string]bool{}
		for _, rm := range run.Members {
			finished[rm.Dataset] = true
		}
		for name, m := range byName {
			if !finished[name] {
				m.State, m.Code = zfsReplicaFailed, "interrupted"
			}
		}
	}
	base := ""
	if d.Replica.TargetKind == store.ZFSReplicaTargetServer {
		if srv, ok, err := s.store.GetZFSReplicaServer(d.Replica.TargetID); err == nil && ok {
			if folder, fErr := s.zfsReplicaFolder(d); fErr == nil {
				base = srv.Root + "/" + folder
			}
		}
	}
	out := make([]ZFSReplicaMemberView, 0, len(byName))
	for _, m := range byName {
		if m.TargetPath == "" && base != "" {
			m.TargetPath = base + "/" + m.Dataset
		}
		if running {
			m.State = zfsReplicaRunning
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dataset < out[j].Dataset })
	return out, nil
}

// zfsReplicaSnapshotViews lists the replica snapshots of the item's root on
// its target, newest first.
func (s *Service) zfsReplicaSnapshotViews(ctx context.Context, d store.ZFSDataset) []ZFSReplicaSnapshotView {
	out := []ZFSReplicaSnapshotView{}
	qctx, cancel := context.WithTimeout(ctx, zfsReplicaQuickTimeout)
	defer cancel()
	tgt, err := s.zfsReplicaTargetFor(qctx, d)
	if err != nil {
		return out
	}
	snaps, err := zfsReplicaSnapshotsOn(qctx, tgt.end, tgt.base+"/"+d.Dataset)
	if err != nil {
		if !zfs.IsNotFound(err) {
			log.Printf("api: zfs replica: listing the snapshots of %s on its target failed: %v", d.Dataset, err)
		}
		return out
	}
	for i := len(snaps) - 1; i >= 0; i-- {
		created := ""
		if at, ok := zfs.StampTime(snaps[i].Name); ok {
			created = at.Format(time.RFC3339)
		}
		out = append(out, ZFSReplicaSnapshotView{Name: snaps[i].Name, Created: created})
	}
	return out
}

// ZFSReplicaPatch carries a replica edit. A nil field keeps its value.
type ZFSReplicaPatch struct {
	Target      *zfsReplicaTargetExport `json:"target"`
	AfterBackup *bool                   `json:"afterBackup"`
	Cadence     *string                 `json:"cadence"`
	Keep        *store.ZFSReplicaKeep   `json:"keep"`
}

// PatchZFSReplica changes how an item replicates. Everything is checked
// before anything is written. Leaving a target behind cleans the item's tree
// of the replica snapshots and bookmarks kept for it; the copy on the target
// stays.
func (s *Service) PatchZFSReplica(ctx context.Context, id string, p ZFSReplicaPatch) error {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return fmt.Errorf("zfs replica: load dataset: %w", err)
	}
	rep := d.Replica
	target := zfsReplicaTargetExport{Kind: rep.TargetKind, ID: rep.TargetID}
	if p.Target != nil {
		target = zfsReplicaTargetExport{Kind: p.Target.Kind, ID: strings.TrimSpace(p.Target.ID)}
		if target.Kind == store.ZFSReplicaTargetNone {
			target.ID = ""
		}
	}
	cadence, keep := rep.Cadence, rep.Keep
	if p.Cadence != nil {
		cadence = strings.TrimSpace(*p.Cadence)
	}
	if p.Keep != nil {
		keep = *p.Keep
	}
	servers, err := s.store.ListZFSReplicaServers()
	if err != nil {
		return err
	}
	ids := make(map[string]bool, len(servers))
	for _, srv := range servers {
		ids[srv.ID] = true
	}
	if msg := zfsReplicaItemRefusal(target, cadence, keep, ids); msg != "" {
		return fmt.Errorf("the replica %s", msg)
	}

	moved := target.Kind != rep.TargetKind || target.ID != rep.TargetID
	if moved {
		unlock, ok := s.lockZFSReplica(id)
		if !ok {
			return errZFSReplicaBusy
		}
		defer unlock()
		if rep.TargetKind != store.ZFSReplicaTargetNone {
			s.cleanZFSReplicaSource(ctx, d)
		}
		if err := s.store.SetZFSReplicaTarget(id, target.Kind, target.ID); err != nil {
			return err
		}
	}
	if p.AfterBackup != nil {
		if err := s.store.SetZFSReplicaAfterBackup(id, *p.AfterBackup); err != nil {
			return err
		}
	}
	if p.Cadence != nil {
		if err := s.store.SetZFSReplicaCadence(id, cadence); err != nil {
			return err
		}
	}
	if p.Keep != nil {
		if err := s.store.SetZFSReplicaKeep(id, keep); err != nil {
			return err
		}
	}
	// The same peer target again is how the page asks a second time.
	if target.Kind == store.ZFSReplicaTargetPeer && (p.Target != nil || p.Keep != nil) {
		if d, err = s.store.GetZFSDataset(id); err != nil {
			return err
		}
		return s.zfsReplicaPeerRequest(ctx, d)
	}
	return nil
}
