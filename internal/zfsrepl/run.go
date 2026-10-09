package zfsrepl

import (
	"context"
	"log"
	"path"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// Run replicates one entry. It takes one recursive replica snapshot on the
// source and brings every member up to it on the target, one at a time. The
// error is for a run that could not start; how each member fared is in the
// result.
func Run(ctx context.Context, source, target End, e Entry) (Result, error) {
	tree, err := listTree(ctx, source, e.Root)
	if err != nil {
		return Result{}, err
	}
	// One descendant whose name does not fit fails the recursive snapshot of
	// the whole tree, excluded or not.
	for _, d := range tree {
		if !zfs.ReplicaNameFits(d.Name) {
			return Result{}, &Refusal{Code: "name-too-long", Detail: d.Name}
		}
	}
	snap := zfs.ReplicaSnapshotName(e.Now())
	args, err := zfs.ReplicaSnapshotArgs(e.Root, snap)
	if err != nil {
		return Result{}, err
	}
	if _, err := source.Run(ctx, args); err != nil {
		return Result{}, &Refusal{Code: "snapshot-failed", Detail: e.Root + "@" + snap, Err: err}
	}

	r := &run{src: source, dst: target, e: e, snap: snap, placeholders: map[string]bool{}}
	for _, p := range e.Placeholders {
		r.placeholders[p] = true
	}
	// Whatever the run leaves on the source has to go although it was
	// cancelled, or every cancelled run adds a snapshot that pins blocks.
	cleanup := context.WithoutCancel(ctx)
	res := Result{Snapshot: snap}
	for _, d := range tree {
		switch {
		// An excluded dataset may be another entry's member, so only the
		// snapshot this run took there goes.
		case excluded(d.Name, e.Excluded):
			r.destroyNew(cleanup, d.Name)
		case ctx.Err() != nil:
			r.destroyNew(cleanup, d.Name)
			res.Members = append(res.Members, MemberResult{Dataset: d.Name, Target: r.target(d.Name), Code: "not-reached", Err: ctx.Err()})
		default:
			res.Members = append(res.Members, r.member(ctx, d))
		}
	}
	res.Created = r.created
	return res, nil
}

type run struct {
	src, dst     End
	e            Entry
	snap         string
	placeholders map[string]bool
	created      []string
}

func (r *run) target(dataset string) string { return r.e.TargetBase + "/" + dataset }

func (r *run) member(ctx context.Context, d zfs.ListEntry) MemberResult {
	m := MemberResult{
		Dataset: d.Name,
		Target:  r.target(d.Name),
		Volume:  d.Type == "volume",
		Raw:     d.Encryption != "off",
	}
	src, tgt, err := r.replicate(ctx, &m)
	if err != nil {
		m.Code, m.Err = Code(err), err
		if ctx.Err() != nil {
			m.Code = "not-reached"
		}
		log.Printf("zfs replica: %s to %s failed: %v", m.Dataset, m.Target, err)
		r.keepOrDrop(context.WithoutCancel(ctx), m)
		return m
	}
	r.clear(context.WithoutCancel(ctx), m.Dataset, src)
	m.Pruned = r.prune(ctx, m.Target, tgt)
	return m
}

// replicate brings one member up to the run's snapshot and returns the replica
// points both sides had for the housekeeping after it.
func (r *run) replicate(ctx context.Context, m *MemberResult) (src, tgt []zfs.ReplicaPoint, err error) {
	if src, err = points(ctx, r.src, m.Dataset); err != nil {
		return nil, nil, err
	}
	fresh, ok := find(src, r.snap)
	if !ok {
		return nil, nil, &Refusal{Code: "snapshot-failed", Detail: m.Dataset + "@" + r.snap + " is not on the source"}
	}
	m.GUID = fresh.GUID

	view, err := r.view(ctx, m.Target)
	if err != nil {
		return nil, nil, err
	}
	if view.token != "" {
		if view, err = r.resume(ctx, m, view, src); err != nil {
			return nil, nil, err
		}
	}

	send := zfs.SendSpec{Member: m.Dataset, Snap: r.snap, Raw: m.Raw}
	recv := zfs.ReceiveSpec{Target: m.Target, Volume: m.Volume}
	switch base, newer := commonBase(src, view.snaps, r.snap); {
	case !view.exists:
		recv.Full = true
		if m.Dataset == r.e.Root {
			if err := r.ensureParents(ctx, path.Dir(m.Target)); err != nil {
				return nil, nil, err
			}
		}
	case base != nil:
		send.Base, send.FromBookmark = base.Name, base.Bookmark
		// -F rolls back writes on the replica, but with a newer snapshot on
		// the target it would destroy that snapshot too.
		recv.Rollback = !newer
	case len(view.snaps) == 0 && r.placeholders[m.Target]:
		recv.Full, recv.Replace = true, true
	default:
		return nil, nil, &Refusal{Code: "no-common-base", Detail: m.Target + " shares no replica snapshot with " + m.Dataset}
	}
	m.Base, m.FromBookmark = send.Base, send.FromBookmark

	sendArgs, err := zfs.SendArgs(send)
	if err != nil {
		return nil, nil, err
	}
	estArgs, err := zfs.EstimateArgs(send)
	if err != nil {
		return nil, nil, err
	}
	recvArgs, err := zfs.ReceiveArgs(recv)
	if err != nil {
		return nil, nil, err
	}
	n, err := r.stream(ctx, m.Dataset, sendArgs, recvArgs, estArgs)
	m.Bytes += n
	if err != nil {
		return nil, nil, err
	}

	if tgt, err = r.landed(ctx, m.Target, fresh); err != nil {
		return nil, nil, err
	}
	m.Snapshot = r.snap
	if err := r.anchor(ctx, m.Dataset, r.snap); err != nil {
		return nil, nil, err
	}
	return src, tgt, nil
}

// resume finishes the stream an earlier run left on the target. A token whose
// snapshot is gone from the source is dropped with receive -A, and the
// member carries on from what the target holds then.
func (r *run) resume(ctx context.Context, m *MemberResult, view targetView, src []zfs.ReplicaPoint) (targetView, error) {
	sendArgs, err := zfs.ResumeSendArgs(view.token)
	if err != nil {
		return view, err
	}
	estArgs, err := zfs.EstimateResumeArgs(view.token)
	if err != nil {
		return view, err
	}
	recvArgs, err := zfs.ReceiveArgs(zfs.ReceiveSpec{Target: m.Target, Full: len(view.snaps) == 0, Volume: m.Volume})
	if err != nil {
		return view, err
	}
	n, err := r.stream(ctx, m.Dataset, sendArgs, recvArgs, estArgs)
	m.Bytes += n
	switch {
	case err == nil:
		m.Resumed = true
	case Code(err) == "resume-token-stale":
		log.Printf("zfs replica: the interrupted stream into %s cannot be resumed, dropping it: %v", m.Target, err)
		abort, aerr := zfs.AbortReceiveArgs(m.Target)
		if aerr != nil {
			return view, aerr
		}
		if _, aerr := r.dst.Run(ctx, abort); aerr != nil {
			return view, aerr
		}
	default:
		return view, err
	}
	if view, err = r.view(ctx, m.Target); err != nil {
		return view, err
	}
	// The resumed snapshot is the base of what follows. Its bookmark keeps it
	// one if the snapshot goes before the next run.
	if m.Resumed {
		if last, ok := newestReplica(view.snaps); ok {
			if p, ok := find(src, last.Name); ok && p.GUID == last.GUID {
				if err := r.bookmark(ctx, m.Dataset, last.Name); err != nil {
					log.Printf("zfs replica: bookmarking %s@%s failed: %v", m.Dataset, last.Name, err)
				}
			}
		}
	}
	return view, nil
}

type targetView struct {
	exists bool
	token  string
	snaps  []zfs.ReplicaPoint
}

func (r *run) view(ctx context.Context, target string) (targetView, error) {
	st, err := state(ctx, r.dst, target)
	if zfs.IsNotFound(err) {
		return targetView{}, nil
	}
	if err != nil {
		return targetView{}, err
	}
	pts, err := points(ctx, r.dst, target)
	if err != nil {
		return targetView{}, err
	}
	return targetView{exists: true, token: st.ResumeToken, snaps: snapshots(pts)}, nil
}

// ensureParents creates the missing levels above the entry's root on the
// target, top down, so each one is known to be BombVault's own.
func (r *run) ensureParents(ctx context.Context, dataset string) error {
	var missing []string
	for d := dataset; ; d = path.Dir(d) {
		_, err := state(ctx, r.dst, d)
		if err == nil {
			break
		}
		if !zfs.IsNotFound(err) || !strings.Contains(d, "/") {
			return err
		}
		missing = append(missing, d)
	}
	for i := len(missing) - 1; i >= 0; i-- {
		args, err := zfs.CreateParentArgs(missing[i])
		if err != nil {
			return err
		}
		if _, err := r.dst.Run(ctx, args); err != nil {
			return err
		}
		r.created = append(r.created, missing[i])
		r.placeholders[missing[i]] = true
	}
	return nil
}

// landed checks that the target holds the snapshot the source sent, by guid,
// because only that proves the stream arrived whole.
func (r *run) landed(ctx context.Context, target string, sent zfs.ReplicaPoint) ([]zfs.ReplicaPoint, error) {
	pts, err := points(ctx, r.dst, target)
	if err != nil {
		return nil, err
	}
	if p, ok := find(pts, sent.Name); !ok || p.Bookmark || p.GUID != sent.GUID {
		return nil, &Refusal{Code: "zfs-error", Detail: target + "@" + sent.Name + " is not on the target after the receive"}
	}
	return snapshots(pts), nil
}

// anchor makes the member's new snapshot the base of its next run: a bookmark
// that survives the snapshot, and a hold so nobody destroys it by accident.
func (r *run) anchor(ctx context.Context, dataset, snap string) error {
	if err := r.bookmark(ctx, dataset, snap); err != nil {
		return err
	}
	args, err := zfs.HoldArgs(dataset, snap)
	if err != nil {
		return err
	}
	_, err = r.src.Run(ctx, args)
	return ignore(err, zfs.IsExists)
}

func (r *run) bookmark(ctx context.Context, dataset, snap string) error {
	args, err := zfs.BookmarkArgs(dataset, snap)
	if err != nil {
		return err
	}
	_, err = r.src.Run(ctx, args)
	return ignore(err, zfs.IsExists)
}

// keepOrDrop decides about a failed member's new snapshot on the source. It
// stays while the target holds it or may still resume towards it, and goes
// otherwise, so a member that keeps failing does not pile up snapshots.
func (r *run) keepOrDrop(ctx context.Context, m MemberResult) {
	view, err := r.view(ctx, m.Target)
	if err != nil {
		log.Printf("zfs replica: keeping %s@%s, the target could not be read: %v", m.Dataset, r.snap, err)
		return
	}
	if view.token != "" {
		return
	}
	if _, ok := find(view.snaps, r.snap); ok {
		return
	}
	r.destroyNew(ctx, m.Dataset)
}

func (r *run) destroyNew(ctx context.Context, dataset string) {
	args, err := zfs.DestroyReplicaArgs(dataset, r.snap)
	if err == nil {
		_, err = r.src.Run(ctx, args)
	}
	if err = ignore(err, zfs.IsNotFound); err != nil {
		log.Printf("zfs replica: removing %s@%s failed: %v", dataset, r.snap, err)
	}
}

// clear releases and destroys every replica snapshot of a member on the source
// but the run's own, which the target holds too. Bookmarks stay: they cost
// nothing and are the base once a snapshot is gone.
func (r *run) clear(ctx context.Context, dataset string, pts []zfs.ReplicaPoint) {
	for _, p := range snapshots(pts) {
		if p.Name == r.snap || !zfs.IsReplicaSnapshot(p.Name) {
			continue
		}
		if err := release(ctx, r.src, dataset, p.Name); err != nil {
			log.Printf("zfs replica: releasing %s@%s failed: %v", dataset, p.Name, err)
			continue
		}
		args, err := zfs.DestroyReplicaArgs(dataset, p.Name)
		if err == nil {
			_, err = r.src.Run(ctx, args)
		}
		if err = ignore(err, zfs.IsNotFound); err != nil {
			log.Printf("zfs replica: removing %s@%s failed: %v", dataset, p.Name, err)
		}
	}
}

// prune applies the entry's retention to the target's replica snapshots of one
// member, which end with the run's own.
func (r *run) prune(ctx context.Context, target string, snaps []zfs.ReplicaPoint) []string {
	names := make([]string, 0, len(snaps))
	for _, p := range snaps {
		names = append(names, p.Name)
	}
	var pruned []string
	for _, name := range Expired(names, r.e.Keep, r.snap, time.Local) {
		args, err := zfs.DestroyReplicaArgs(target, name)
		if err == nil {
			_, err = r.dst.Run(ctx, args)
		}
		if err = ignore(err, zfs.IsNotFound); err != nil {
			log.Printf("zfs replica: pruning %s@%s failed: %v", target, name, err)
			continue
		}
		pruned = append(pruned, name)
	}
	return pruned
}

// commonBase picks the newest replica point of the source whose guid the
// target holds as a snapshot, the snapshot before its bookmark. newer reports
// whether the target has a snapshot after that one.
func commonBase(src, tgt []zfs.ReplicaPoint, skip string) (base *zfs.ReplicaPoint, newer bool) {
	onTarget := make(map[uint64]uint64, len(tgt))
	var newest uint64
	for _, p := range tgt {
		onTarget[p.GUID] = p.CreateTxg
		newest = max(newest, p.CreateTxg)
	}
	for i := range src {
		p := &src[i]
		if p.Name == skip || !zfs.IsReplicaSnapshot(p.Name) {
			continue
		}
		if _, ok := onTarget[p.GUID]; !ok {
			continue
		}
		if base == nil || p.CreateTxg > base.CreateTxg || (p.CreateTxg == base.CreateTxg && base.Bookmark && !p.Bookmark) {
			base = p
		}
	}
	if base == nil {
		return nil, false
	}
	return base, onTarget[base.GUID] < newest
}

func newestReplica(snaps []zfs.ReplicaPoint) (zfs.ReplicaPoint, bool) {
	var last zfs.ReplicaPoint
	found := false
	for _, p := range snaps {
		if zfs.IsReplicaSnapshot(p.Name) && (!found || p.CreateTxg >= last.CreateTxg) {
			last, found = p, true
		}
	}
	return last, found
}

// find returns the snapshot called name, or its bookmark when the snapshot is
// gone.
func find(pts []zfs.ReplicaPoint, name string) (zfs.ReplicaPoint, bool) {
	var mark zfs.ReplicaPoint
	found := false
	for _, p := range pts {
		if p.Name != name {
			continue
		}
		if !p.Bookmark {
			return p, true
		}
		mark, found = p, true
	}
	return mark, found
}

func snapshots(pts []zfs.ReplicaPoint) []zfs.ReplicaPoint {
	var out []zfs.ReplicaPoint
	for _, p := range pts {
		if !p.Bookmark {
			out = append(out, p)
		}
	}
	return out
}

func excluded(name string, excludes []string) bool {
	for _, ex := range excludes {
		if name == ex || zfs.DescendantOf(name, ex) {
			return true
		}
	}
	return false
}

// ignore drops an error that only says the step was already done.
func ignore(err error, done func(error) bool) error {
	if err != nil && done(err) {
		return nil
	}
	return err
}
