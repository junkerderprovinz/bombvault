package api

import (
	"bufio"
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"log"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

// zfsSendResult is the trailer that ends a bring back stream with "ok" or
// the reason code of the send. A stream cut short by a failing send would
// otherwise end like a whole one.
const zfsSendResult = "Bombvault-Zfs-Result"

// zfsReceiveHost is the End on this instance's own host that the receive
// slots write to.
func (s *Service) zfsReceiveHost() (zfsrepl.End, error) {
	host, ok := s.zfs.(*zfs.SSHHost)
	if !ok || s.ssh == nil {
		return nil, &zfsrepl.Refusal{Code: "ssh-missing", Detail: "this instance has no SSH connection to its host"}
	}
	return zfsrepl.NewSSHEnd(host, s.ssh), nil
}

// zfsSlotHost is the host as slot may use it: only while the slot's folder is
// missing or carries the slot's source. A folder with another mark belongs to
// an instance that took the same name, and the slot reaches nothing below it.
func (s *Service) zfsSlotHost(ctx context.Context, slot store.ZFSReceiveSlot) (zfsrepl.End, error) {
	host, err := s.zfsReceiveHost()
	if err != nil {
		return nil, err
	}
	if err := zfsFolderFree(ctx, host, slot.Base(), slot.PeerID); err != nil {
		return nil, err
	}
	return host, nil
}

// zfsFolderFree refuses folder for the members of source unless it does not
// exist yet or carries source as its own.
func zfsFolderFree(ctx context.Context, host zfsrepl.End, folder, source string) error {
	_, err := zfsEndState(ctx, host, folder)
	if zfs.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	own, err := zfsOwnedBy(ctx, host, folder, source)
	if err != nil {
		return err
	}
	if !own {
		return &zfsrepl.Refusal{Code: "target-owned", Detail: folder + " belongs to another instance"}
	}
	return nil
}

// zfsSlotFor answers the request itself unless it carries the token of the
// allowed slot its path names.
func (s *Service) zfsSlotFor(w http.ResponseWriter, r *http.Request) (store.ZFSReceiveSlot, bool) {
	slot, ok, err := s.store.GetZFSReceiveSlot(r.PathValue("slot"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
		return slot, false
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such receive slot"})
		return slot, false
	}
	if slot.State != store.ZFSReceiveAllowed {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "state": slot.State, "error": "this receive slot is not allowed"})
		return slot, false
	}
	token, hasToken := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	want, err := secret.Decrypt(s.cfg.AppKey, slot.TokenEnc)
	if !hasToken || err != nil || subtle.ConstantTimeCompare(want, []byte(token)) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "wrong token for this receive slot"})
		return slot, false
	}
	on, err := s.receiverOn()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
		return slot, false
	}
	if !on {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "state": store.ZFSReceiveOff, "error": "receiving is switched off on this instance"})
		return slot, false
	}
	return slot, true
}

// zfsSlotTarget is where a member of the slot lands, refusing any dataset the
// slot was not allowed for.
func zfsSlotTarget(w http.ResponseWriter, slot store.ZFSReceiveSlot, member string) (string, bool) {
	if !slices.Contains(slot.Members, member) {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "code": "peer-waiting", "error": "that dataset is not part of this receive slot"})
		return "", false
	}
	return slot.Base() + "/" + member, true
}

func zfsSlotFail(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusOK, codedFailEnvelope(err, zfsrepl.Code(err)))
}

// zfsSlotMember is one member as the source sees it here. A placeholder this
// instance made for the source counts as missing, so the source sends a full
// stream and this side decides to land it over the placeholder.
type zfsSlotMember struct {
	Member      string         `json:"member"`
	Exists      bool           `json:"exists"`
	Type        string         `json:"type,omitempty"`
	Encryption  string         `json:"encryption,omitempty"`
	ResumeToken string         `json:"resumeToken,omitempty"`
	Points      []zfsSlotPoint `json:"points"`
}

// zfsSlotPoint is one snapshot of a member. The guid is decimal text, since it
// is an unsigned 64-bit number.
type zfsSlotPoint struct {
	Name      string `json:"name"`
	GUID      string `json:"guid"`
	CreateTxg uint64 `json:"createtxg"`
}

// handleZFSSlotPoints lists the snapshots and resume tokens of the slot's
// members here, or of the one ?member= names.
// GET /api/zfs/receive/{slot}/points
func (s *Service) handleZFSSlotPoints(w http.ResponseWriter, r *http.Request) {
	slot, ok := s.zfsSlotFor(w, r)
	if !ok {
		return
	}
	members := slot.Members
	if m := r.URL.Query().Get("member"); m != "" {
		if _, ok := zfsSlotTarget(w, slot, m); !ok {
			return
		}
		members = []string{m}
	}
	host, err := s.zfsSlotHost(r.Context(), slot)
	if err != nil {
		zfsSlotFail(w, err)
		return
	}
	out := make([]zfsSlotMember, 0, len(members))
	for _, m := range members {
		v, err := zfsSlotMemberView(r.Context(), host, slot, m)
		if err != nil {
			zfsSlotFail(w, err)
			return
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "members": out})
}

func zfsSlotMemberView(ctx context.Context, host zfsrepl.End, slot store.ZFSReceiveSlot, member string) (zfsSlotMember, error) {
	target := slot.Base() + "/" + member
	v := zfsSlotMember{Member: member, Points: []zfsSlotPoint{}}
	st, err := zfsEndState(ctx, host, target)
	if zfs.IsNotFound(err) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	snaps, err := zfsEndSnapshots(ctx, host, target)
	if err != nil {
		return v, err
	}
	if len(snaps) == 0 && st.ResumeToken == "" {
		own, err := zfsOwnedBy(ctx, host, target, slot.PeerID)
		if err != nil || own {
			return v, err
		}
	}
	v.Exists, v.Type, v.Encryption, v.ResumeToken = true, st.Type, st.Encryption, st.ResumeToken
	for _, p := range snaps {
		v.Points = append(v.Points, zfsSlotPoint{Name: p.Name, GUID: strconv.FormatUint(p.GUID, 10), CreateTxg: p.CreateTxg})
	}
	return v, nil
}

// handleZFSSlotReceive takes one member's stream into this host. The flags
// are this side's own choice from what the stream and the target say, and the
// stream carries no more than the one snapshot they were chosen for, so a
// source can add snapshots but never roll back or replace anything that is
// not its own.
// PUT /api/zfs/receive/{slot}/members/{member}
func (s *Service) handleZFSSlotReceive(w http.ResponseWriter, r *http.Request) {
	slot, ok := s.zfsSlotFor(w, r)
	if !ok {
		return
	}
	target, ok := zfsSlotTarget(w, slot, r.PathValue("member"))
	if !ok {
		return
	}
	if _, busy := s.zfsReceiveBusy.LoadOrStore(slot.ID, true); busy {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "another stream into this slot is running"})
		return
	}
	defer s.zfsReceiveBusy.Delete(slot.ID)

	host, err := s.zfsSlotHost(r.Context(), slot)
	if err != nil {
		zfsSlotFail(w, err)
		return
	}
	ctx := r.Context()
	stream := bufio.NewReaderSize(r.Body, 64<<10)
	begin, err := zfs.ReadStreamBegin(stream)
	if err != nil {
		zfsSlotFail(w, &zfsrepl.Refusal{Code: "zfs-error", Detail: "the body is not a send stream", Err: err})
		return
	}
	_, snap, _ := strings.Cut(begin.Snapshot, "@")
	if !zfs.IsReplicaSnapshot(snap) {
		zfsSlotFail(w, &zfsrepl.Refusal{Code: "invalid-name", Detail: begin.Snapshot + " is not a replica snapshot"})
		return
	}
	room, err := zfsSlotRoom(ctx, host, slot.Pool)
	if err == nil && room <= 0 {
		err = &zfsrepl.Refusal{Code: "not-enough-space", Detail: "pool " + slot.Pool + " is down to the space this instance keeps free"}
	}
	if err != nil {
		zfsSlotFail(w, err)
		return
	}
	args, err := zfsSlotReceiveArgs(ctx, host, slot, target, begin)
	if err != nil {
		zfsSlotFail(w, err)
		return
	}
	one := zfs.NewSnapshotStream(stream)
	counted := &byteCounter{r: one, room: room}
	err = host.Receive(ctx, args, counted)
	if refused := one.Refused(); refused != nil {
		err = &zfsrepl.Refusal{Code: "zfs-error", Detail: "the stream carries more than the snapshot it starts with", Err: refused}
	}
	if counted.full.Load() {
		err = &zfsrepl.Refusal{Code: "not-enough-space", Detail: "the stream outgrew the space pool " + slot.Pool + " has above what this instance keeps free"}
	}
	if err != nil {
		log.Printf("zfs receive: %s from %s failed: %v", target, slot.PeerName, err)
		zfsSlotFail(w, err)
		return
	}
	n := counted.n.Load()
	if err := s.store.RecordZFSReceived(slot.ID, snap, n); err != nil {
		log.Printf("zfs receive: %v", err)
	}
	pruned := zfsSlotPrune(context.WithoutCancel(ctx), host, slot, target)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "bytes": n, "pruned": pruned})
}

// zfsSlotReceiveArgs picks the receive flags for one stream. -F lands a full
// stream only on a placeholder this instance made for the same source, and
// rolls an incremental back only when its base is the target's newest
// snapshot, so it never destroys a snapshot.
func zfsSlotReceiveArgs(ctx context.Context, host zfsrepl.End, slot store.ZFSReceiveSlot, target string, b zfs.StreamBegin) ([]string, error) {
	st, err := zfsEndState(ctx, host, target)
	if zfs.IsNotFound(err) {
		if b.FromGUID != 0 {
			return nil, &zfsrepl.Refusal{Code: "no-common-base", Detail: target + " does not exist for an incremental stream"}
		}
		if err := zfsSlotParents(ctx, host, slot, path.Dir(target)); err != nil {
			return nil, err
		}
		return zfs.ReceiveArgs(zfs.ReceiveSpec{Target: target, Full: true, Volume: b.Volume})
	}
	if err != nil {
		return nil, err
	}
	snaps, err := zfsEndSnapshots(ctx, host, target)
	if err != nil {
		return nil, err
	}
	volume := st.Type == "volume"
	switch {
	case st.ResumeToken != "":
		return zfs.ReceiveArgs(zfs.ReceiveSpec{Target: target, Full: len(snaps) == 0, Volume: volume})
	case b.FromGUID == 0:
		own, err := zfsOwnedBy(ctx, host, target, slot.PeerID)
		if err != nil {
			return nil, err
		}
		if len(snaps) > 0 || !own {
			return nil, &zfsrepl.Refusal{Code: "dataset-exists", Detail: target + " exists and is not a placeholder of this source"}
		}
		if err := zfsSlotParents(ctx, host, slot, path.Dir(target)); err != nil {
			return nil, err
		}
		return zfs.ReceiveArgs(zfs.ReceiveSpec{Target: target, Full: true, Replace: true, Volume: b.Volume})
	}
	i := slices.IndexFunc(snaps, func(p zfs.ReplicaPoint) bool { return p.GUID == b.FromGUID })
	if i < 0 {
		return nil, &zfsrepl.Refusal{Code: "no-common-base", Detail: target + " does not hold the base of the stream"}
	}
	return zfs.ReceiveArgs(zfs.ReceiveSpec{Target: target, Volume: volume, Rollback: i == len(snaps)-1})
}

// zfsSlotParents creates what is missing of dir and above, top down. Levels
// from the source's folder down are marked as the source's.
func zfsSlotParents(ctx context.Context, host zfsrepl.End, slot store.ZFSReceiveSlot, dir string) error {
	base := slot.Base()
	var levels []string
	for d := dir; strings.Contains(d, "/"); d = path.Dir(d) {
		levels = append(levels, d)
	}
	for i := len(levels) - 1; i >= 0; i-- {
		level := levels[i]
		owner := ""
		if level == base || zfs.DescendantOf(level, base) {
			owner = slot.PeerID
		}
		_, err := zfsEndState(ctx, host, level)
		if err == nil {
			continue
		}
		if !zfs.IsNotFound(err) {
			return err
		}
		args, err := zfs.CreateParentArgs(level, owner)
		if err != nil {
			return err
		}
		if _, err := host.Run(ctx, args); err != nil {
			return err
		}
	}
	return nil
}

// zfsSlotPrune applies the slot's keep rule to one member after a stream
// landed. The newest snapshot is the base of the source's next run and always
// stays.
func zfsSlotPrune(ctx context.Context, host zfsrepl.End, slot store.ZFSReceiveSlot, target string) []string {
	keep, ok := slot.Keep.Counts()
	if !ok {
		return nil
	}
	snaps, err := zfsEndSnapshots(ctx, host, target)
	if err != nil || len(snaps) == 0 {
		return nil
	}
	names := make([]string, 0, len(snaps))
	for _, p := range snaps {
		names = append(names, p.Name)
	}
	var pruned []string
	for _, name := range zfsrepl.Expired(names, keep, names[len(names)-1], time.Local) {
		args, err := zfs.DestroyReplicaArgs(target, name)
		if err == nil {
			_, err = host.Run(ctx, args)
		}
		if err != nil {
			log.Printf("zfs receive: pruning %s@%s failed: %v", target, name, err)
			continue
		}
		pruned = append(pruned, name)
	}
	return pruned
}

// handleZFSSlotAbort drops the partial state an interrupted stream left on a
// member, the one thing a source may remove here.
// POST /api/zfs/receive/{slot}/members/{member}/abort
func (s *Service) handleZFSSlotAbort(w http.ResponseWriter, r *http.Request) {
	slot, ok := s.zfsSlotFor(w, r)
	if !ok {
		return
	}
	target, ok := zfsSlotTarget(w, slot, r.PathValue("member"))
	if !ok {
		return
	}
	if _, busy := s.zfsReceiveBusy.LoadOrStore(slot.ID, true); busy {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "a stream into this slot is running"})
		return
	}
	defer s.zfsReceiveBusy.Delete(slot.ID)
	host, err := s.zfsSlotHost(r.Context(), slot)
	if err != nil {
		zfsSlotFail(w, err)
		return
	}
	st, err := zfsEndState(r.Context(), host, target)
	if err == nil && st.ResumeToken == "" {
		err = &zfsrepl.Refusal{Code: "not-found", Detail: target + " has no interrupted stream"}
	}
	if err == nil {
		var args []string
		if args, err = zfs.AbortReceiveArgs(target); err == nil {
			_, err = host.Run(r.Context(), args)
		}
	}
	if err != nil {
		zfsSlotFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleZFSSlotSend streams one replica snapshot of a member back to its
// source, raw when the replica is encrypted. The result arrives as the
// zfsSendResult trailer once the stream is through.
// GET /api/zfs/receive/{slot}/members/{member}/send?snapshot=
func (s *Service) handleZFSSlotSend(w http.ResponseWriter, r *http.Request) {
	slot, ok := s.zfsSlotFor(w, r)
	if !ok {
		return
	}
	target, ok := zfsSlotTarget(w, slot, r.PathValue("member"))
	if !ok {
		return
	}
	snap := r.URL.Query().Get("snapshot")
	if !zfs.IsReplicaSnapshot(snap) {
		zfsSlotFail(w, &zfsrepl.Refusal{Code: "invalid-name", Detail: "not a replica snapshot name"})
		return
	}
	host, err := s.zfsSlotHost(r.Context(), slot)
	if err != nil {
		zfsSlotFail(w, err)
		return
	}
	ctx := r.Context()
	st, err := zfsEndState(ctx, host, target)
	if err != nil {
		zfsSlotFail(w, err)
		return
	}
	snaps, err := zfsEndSnapshots(ctx, host, target)
	if err != nil {
		zfsSlotFail(w, err)
		return
	}
	if !slices.ContainsFunc(snaps, func(p zfs.ReplicaPoint) bool { return p.Name == snap }) {
		zfsSlotFail(w, &zfsrepl.Refusal{Code: "not-found", Detail: target + "@" + snap})
		return
	}
	args, err := zfs.SendArgs(zfs.SendSpec{Member: target, Snap: snap, Raw: st.Encrypted()})
	if err != nil {
		zfsSlotFail(w, err)
		return
	}
	out, wait, err := host.Send(ctx, args)
	if err != nil {
		zfsSlotFail(w, err)
		return
	}
	w.Header().Set("Trailer", zfsSendResult)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, copyErr := io.Copy(w, out)
	err = errors.Join(wait(), copyErr)
	_ = out.Close()
	result := "ok"
	if err != nil {
		log.Printf("zfs receive: sending %s@%s back to %s failed: %v", target, snap, slot.PeerName, err) //nolint:gosec // G706: snap matched the replica pattern and target is a member the slot holds
		result = zfsrepl.Code(err)
	}
	w.Header().Set(zfsSendResult, result)
}

func zfsEndState(ctx context.Context, end zfsrepl.End, dataset string) (zfs.DatasetState, error) {
	args, err := zfs.DatasetStateArgs(dataset)
	if err != nil {
		return zfs.DatasetState{}, err
	}
	out, err := end.Run(ctx, args)
	if err != nil {
		return zfs.DatasetState{}, err
	}
	return zfs.ParseDatasetState(out)
}

// zfsEndSnapshots lists a dataset's snapshots oldest first, without its
// bookmarks.
func zfsEndSnapshots(ctx context.Context, end zfsrepl.End, dataset string) ([]zfs.ReplicaPoint, error) {
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
	return slices.DeleteFunc(pts, func(p zfs.ReplicaPoint) bool { return p.Bookmark }), nil
}

// zfsOwnedBy reports whether the dataset carries source as its own
// zfs.SourceProperty.
func zfsOwnedBy(ctx context.Context, end zfsrepl.End, dataset, source string) (bool, error) {
	who, err := zfsReplicaOwner(ctx, end, dataset)
	return who == source, err
}

// zfsReceiveHeadroom is the part of a pool, one in this many, that streams
// into a slot leave free. The host goes on writing its own data to the pool,
// and ZFS slows down badly once a pool runs nearly full.
const zfsReceiveHeadroom = 10

// zfsSlotRoom is how many bytes a stream into pool may bring before the pool
// drops below its headroom. A stream lands about as large as it travels,
// since the source sends blocks compressed or raw as they lie on its disk.
func zfsSlotRoom(ctx context.Context, host zfsrepl.End, pool string) (int64, error) {
	out, err := host.Run(ctx, zfs.PoolsArgs())
	if err != nil {
		return 0, err
	}
	pools, err := zfs.ParsePools(out)
	if err != nil {
		return 0, err
	}
	for _, p := range pools {
		if p.Name == pool {
			return p.FreeBytes - p.SizeBytes/zfsReceiveHeadroom, nil
		}
	}
	return 0, &zfsrepl.Refusal{Code: "not-found", Detail: "pool " + pool + " is not on this host"}
}

// byteCounter counts what a receive read of its stream and cuts the stream
// off once it brought more than room bytes.
type byteCounter struct {
	r    io.Reader
	room int64
	n    atomic.Int64
	full atomic.Bool
}

var errSlotFull = errors.New("the stream outgrew the room left in the pool")

func (c *byteCounter) Read(p []byte) (int, error) {
	if c.full.Load() {
		return 0, errSlotFull
	}
	if left := c.room - c.n.Load() + 1; int64(len(p)) > left {
		p = p[:left]
	}
	n, err := c.r.Read(p)
	if c.n.Add(int64(n)) > c.room {
		c.full.Store(true)
		return 0, errSlotFull
	}
	return n, err
}
