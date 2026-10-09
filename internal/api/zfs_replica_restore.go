package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

// ZFSReplicaRestoreAck is what a bring back answers before it runs: its run,
// the dataset it lands in, and whether that one stays unmounted because it
// arrives encrypted and its key has to be loaded first.
type ZFSReplicaRestoreAck struct {
	RunID     string `json:"runId"`
	Dataset   string `json:"dataset"`
	KeyNeeded bool   `json:"keyNeeded"`
}

// StartZFSReplicaRestore sends one replica snapshot of the item's root back
// into a new dataset next to it, never over it, and mounts that dataset once
// it is there. The checks run before it answers, the stream afterwards.
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
		defer s.recoverOperation("zfs replica restore: "+d.Dataset, nil, func(msg string) { s.failStuckRun(id, msg) })
		rctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		defer cancel()
		key := zfsReplicaRestoreKey(id)
		s.registerCancel(key, cancel)
		defer s.unregisterCancel(key)

		_, startedAt := s.progBegin(rctx, key, zfsReplicaPhase)
		warn, err := s.restoreZFSReplica(rctx, from, to, zfsrepl.Restore{
			Replica:  replica,
			Snapshot: snapshot,
			Dataset:  d.Dataset,
			Now:      func() time.Time { return at },
			Progress: s.zfsReplicaRestoreProgress(key, startedAt),
		}, ack)
		s.progEnd(key, zfsReplicaPhase, err == nil, startedAt)
		switch {
		case err != nil:
			log.Printf("api: zfs replica restore: %s@%s failed: %v", replica, snapshot, err)
			s.finishRestoreRun(ack.RunID, "", errors.New(zfsRefusalSentence("replica restore", d.Dataset, &backup.ZFSRefusal{
				Code: zfsReplicaCode(err), Detail: zfsDetail(zfsReplicaDetail(err)),
			})))
		case warn != "":
			s.finishRestoreRunWarn(ack.RunID, snapshot, warn)
		default:
			s.finishRestoreRun(ack.RunID, snapshot, nil)
		}
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
	args, err := zfs.DatasetStateArgs(replica)
	if err != nil {
		return ack, nil, nil, "", err
	}
	out, err := tgt.end.Run(qctx, args)
	if err != nil {
		return ack, nil, nil, "", err
	}
	st, err := zfs.ParseDatasetState(out)
	if err != nil {
		return ack, nil, nil, "", err
	}
	ack.KeyNeeded = st.Encrypted()
	return ack, tgt.end, to, replica, nil
}

// restoreZFSReplica brings the snapshot back and mounts what landed. A
// dataset that arrived encrypted stays unmounted until someone loads its key,
// and a mount that fails leaves the data where it is, so both come back as a
// warning on a run that worked.
func (s *Service) restoreZFSReplica(ctx context.Context, from, to zfsrepl.End, r zfsrepl.Restore, ack ZFSReplicaRestoreAck) (string, error) {
	dest, err := s.zfsReplicaBringBack()(ctx, from, to, r)
	if err != nil {
		return "", err
	}
	if ack.KeyNeeded {
		return "restored to " + dest + ", which stays unmounted until its encryption key is loaded", nil
	}
	args, err := zfs.MountArgs(dest)
	if err == nil {
		_, err = to.Run(ctx, args)
	}
	if err != nil {
		log.Printf("api: zfs replica restore: mounting %s failed: %v", dest, err)
		return "restored to " + dest + ", but mounting it failed: " + zfsDetail(err.Error()), nil
	}
	return "", nil
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
