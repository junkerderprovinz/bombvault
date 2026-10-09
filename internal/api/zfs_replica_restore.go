package api

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

// ZFSReplicaRestoreAck is what a bring back answers before it runs: its run,
// the root of the tree it lands in, and whether any dataset of that tree
// stays locked because it arrives encrypted and its key has to be loaded
// first.
type ZFSReplicaRestoreAck struct {
	RunID     string `json:"runId"`
	Dataset   string `json:"dataset"`
	KeyNeeded bool   `json:"keyNeeded"`
}

// StartZFSReplicaRestore sends one replica snapshot of the item's whole tree
// back into a new tree next to it, never over it, and mounts what landed.
// The checks run before it answers, the streams afterwards.
func (s *Service) StartZFSReplicaRestore(ctx context.Context, id, snapshot string) (ZFSReplicaRestoreAck, error) {
	if !zfs.IsReplicaSnapshot(snapshot) {
		return ZFSReplicaRestoreAck{}, zfsRefuse("invalid-name", snapshot)
	}
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return ZFSReplicaRestoreAck{}, fmt.Errorf("zfs replica restore: load dataset: %w", err)
	}
	if err := s.zfsReplicaStartRefusal(d); err != nil {
		return ZFSReplicaRestoreAck{}, err
	}
	unlock, ok := s.lockZFSReplica(id)
	if !ok {
		return ZFSReplicaRestoreAck{}, errZFSReplicaBusy
	}
	ack, from, to, replica, err := s.prepareZFSReplicaRestore(ctx, d, snapshot)
	if err != nil {
		unlock()
		return ZFSReplicaRestoreAck{}, err
	}
	if ack.RunID, err = s.startRun(ctx, id, "restore"); err != nil {
		unlock()
		return ZFSReplicaRestoreAck{}, err
	}
	at := time.Now()
	ack.Dataset = zfsrepl.RestoreName(d.Dataset, at)

	s.replica.work.Add(1)
	go func() {
		defer s.replica.work.Done()
		defer unlock()
		defer s.recoverOperation("zfs replica restore: "+d.Dataset, nil, nil)
		rctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		defer cancel()
		key := zfsReplicaRestoreKey(id)
		s.registerCancel(key, cancel)

		_, startedAt := s.progBegin(rctx, key, zfsReplicaPhase)
		warn, err := s.restoreZFSReplica(rctx, from, to, zfsrepl.Restore{
			Replica:  replica,
			Snapshot: snapshot,
			Dataset:  d.Dataset,
			Now:      func() time.Time { return at },
			Progress: s.zfsReplicaRestoreProgress(key, startedAt),
		})
		s.unregisterCancel(key)
		switch {
		case err != nil:
			log.Printf("api: zfs replica restore: %s@%s failed: %v", replica, snapshot, err)
			detail := zfsDetail(zfsReplicaDetail(err))
			if warn != "" {
				detail += "; " + warn
			}
			// The dataset names are the message, and the run error scrubber
			// would turn each of them into [path].
			s.finishRestoreRun(ack.RunID, "", &backup.ZFSRefusal{Detail: zfsRefusalSentence("replica restore", d.Dataset, &backup.ZFSRefusal{
				Code: zfsReplicaCode(err), Detail: detail,
			})})
		case warn != "":
			s.finishRestoreRunWarn(ack.RunID, snapshot, warn)
		default:
			s.finishRestoreRun(ack.RunID, snapshot, nil)
		}
		// As with a run, the item is free and its row closed before the bar
		// ends.
		unlock()
		s.progEnd(key, zfsReplicaPhase, err == nil, startedAt)
	}()
	return ack, nil
}

// prepareZFSReplicaRestore resolves both ends and checks that the target
// holds the snapshot, so a bad request is refused while the page waits.
func (s *Service) prepareZFSReplicaRestore(ctx context.Context, d store.ZFSDataset, snapshot string) (ack ZFSReplicaRestoreAck, from, to zfsrepl.End, replica string, err error) {
	if to, err = s.zfsReplicaHostEnd(); err != nil {
		return ack, nil, nil, "", err
	}
	tgt, err := s.zfsReplicaTargetFor(ctx, d)
	if err != nil {
		return ack, nil, nil, "", err
	}
	replica = tgt.base + "/" + d.Dataset
	qctx, cancel := context.WithTimeout(ctx, zfsReplicaQuickTimeout)
	defer cancel()
	snaps, err := zfsReplicaSnapshotsOn(qctx, tgt.end, replica)
	if err != nil {
		return ack, nil, nil, "", err
	}
	found := false
	for _, p := range snaps {
		found = found || p.Name == snapshot
	}
	if !found {
		return ack, nil, nil, "", zfsRefuse("not-found", replica+"@"+snapshot)
	}
	args, err := zfs.TreeArgs(replica)
	if err != nil {
		return ack, nil, nil, "", err
	}
	out, err := tgt.end.Run(qctx, args)
	if err != nil {
		return ack, nil, nil, "", err
	}
	tree, err := zfs.ParseTree(out, replica)
	if err != nil {
		return ack, nil, nil, "", err
	}
	ack.KeyNeeded = slices.ContainsFunc(tree, func(e zfs.ListEntry) bool { return e.Encryption != "off" })
	return ack, tgt.end, to, replica, nil
}

// restoreZFSReplica brings the snapshot back and mounts what landed, parents
// first. A member that arrived encrypted stays unmounted until someone loads
// its key, a member left out had no such snapshot, and a mount that fails
// leaves the data where it is, so all three come back as a warning on a run
// that worked. A bring back that fails partway names in the warning what
// landed before, which stays there unmounted.
func (s *Service) restoreZFSReplica(ctx context.Context, from, to zfsrepl.End, r zfsrepl.Restore) (warn string, err error) {
	defer s.recoverOperation("zfs replica restore: "+r.Dataset, &err, nil)
	got, err := s.zfsReplicaBringBack()(ctx, from, to, r)
	if err != nil {
		if len(got.Members) == 0 {
			return "", err
		}
		landed := make([]string, 0, len(got.Members))
		for _, m := range got.Members {
			landed = append(landed, m.Dataset)
		}
		return "what had landed stays unmounted: " + strings.Join(landed, ", "), err
	}
	var locked, failed []string
	for _, m := range got.Members {
		switch {
		case m.Volume:
		case m.Encrypted:
			locked = append(locked, m.Dataset)
		default:
			args, err := zfs.MountArgs(m.Dataset)
			if err == nil {
				_, err = to.Run(ctx, args)
			}
			if err != nil {
				log.Printf("api: zfs replica restore: mounting %s failed: %v", m.Dataset, err)
				failed = append(failed, m.Dataset)
			}
		}
	}
	var notes []string
	if len(locked) > 0 {
		notes = append(notes, "stays unmounted until its encryption key is loaded: "+strings.Join(locked, ", "))
	}
	if len(failed) > 0 {
		notes = append(notes, "could not be mounted: "+strings.Join(failed, ", "))
	}
	if len(got.Skipped) > 0 {
		notes = append(notes, "has no snapshot "+r.Snapshot+" and was left out: "+strings.Join(got.Skipped, ", "))
	}
	if len(notes) == 0 {
		return "", nil
	}
	return "restored to " + got.Root + "; " + strings.Join(notes, "; "), nil
}

func (s *Service) zfsReplicaRestoreProgress(key string, startedAt int64) func(done, total int64) {
	report := s.zfsReplicaProgress(key, startedAt)
	if report == nil {
		return nil
	}
	return func(done, total int64) { report("", done, total) }
}

// zfsReplicaSnapshotsOn lists the replica snapshots of one dataset on a
// target, oldest first.
func zfsReplicaSnapshotsOn(ctx context.Context, end zfsrepl.End, dataset string) ([]zfs.ReplicaPoint, error) {
	args, err := zfs.ReplicaPointsArgs(dataset)
	if err != nil {
		return nil, err
	}
	out, err := end.Run(ctx, args)
	if err != nil {
		return nil, err
	}
	pts, err := zfs.ParseReplicaPoints(out, dataset)
	if err != nil {
		return nil, err
	}
	snaps := pts[:0]
	for _, p := range pts {
		if !p.Bookmark && zfs.IsReplicaSnapshot(p.Name) {
			snaps = append(snaps, p)
		}
	}
	return snaps, nil
}
