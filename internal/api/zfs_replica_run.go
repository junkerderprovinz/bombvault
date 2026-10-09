package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

func zfsReplicaProgressKey(id string) string { return "zfs-replica:" + id }

func zfsReplicaRestoreKey(id string) string { return "zfs-replica-restore:" + id }

// zfsReplicaPhase is the progress phase of a replica run and a bring back.
// The page reads every other phase as a backup or restore in flight and
// holds the backup buttons for it, which a replica must never do.
const zfsReplicaPhase = "maintenance"

// zfsReplicaCode is the reason code of anything a replica run or its setup
// failed with.
func zfsReplicaCode(err error) string {
	if code, ok := zfsRefusalCode(err); ok {
		return code
	}
	return zfsrepl.Code(err)
}

// zfsReplicaStartRefusal refuses a run before a row is written for it: an
// item that replicates nowhere or to a server that is switched off.
func (s *Service) zfsReplicaStartRefusal(d store.ZFSDataset) error {
	switch d.Replica.TargetKind {
	case store.ZFSReplicaTargetNone:
		return zfsRefuse("replica-off", d.Dataset)
	case store.ZFSReplicaTargetServer:
		srv, ok, err := s.store.GetZFSReplicaServer(d.Replica.TargetID)
		if err != nil {
			return err
		}
		if !ok {
			return zfsRefuse("replica-off", d.Dataset)
		}
		if !srv.Enabled {
			return zfsRefuse("server-disabled", srv.Name)
		}
	}
	return nil
}

// zfsReplicaRunRefusal is zfsReplicaStartRefusal for a run, which also needs
// the ZFS domain switched on. A bring back does not.
func (s *Service) zfsReplicaRunRefusal(d store.ZFSDataset) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	if !settings.ZFSEnabled {
		return zfsRefuse("domain-off", d.Dataset)
	}
	return s.zfsReplicaStartRefusal(d)
}

// StartZFSReplica starts one item's replica run in the background and
// answers with the id of its run row.
func (s *Service) StartZFSReplica(ctx context.Context, id string) (string, error) {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return "", fmt.Errorf("zfs replica: load dataset: %w", err)
	}
	if err := s.zfsReplicaRunRefusal(d); err != nil {
		return "", err
	}
	unlock, ok := s.lockZFSReplica(id)
	if !ok {
		return "", errZFSReplicaBusy
	}
	runID, err := s.startRun(ctx, id, store.ZFSReplicaRunKind)
	if err != nil {
		unlock()
		return "", err
	}
	s.replica.work.Add(1)
	go func() {
		defer s.replica.work.Done()
		defer unlock()
		defer s.recoverOperation("zfs replica: "+d.Dataset, nil, func(msg string) { s.failStuckRun(id, msg) })
		if err := s.replicateZFS(context.WithoutCancel(ctx), d, runID); err != nil {
			log.Printf("api: zfs replica: %s failed: %v", d.Dataset, err)
		}
	}()
	return runID, nil
}

// ReplicateZFSDataset runs one item's replica and waits for it, for the
// scheduler and the run after a backup. An item that is switched off, has
// nowhere to go or is already replicating is left alone.
func (s *Service) ReplicateZFSDataset(ctx context.Context, id string) error {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return fmt.Errorf("zfs replica: load dataset: %w", err)
	}
	if !d.Enabled {
		return nil
	}
	if err := s.zfsReplicaRunRefusal(d); err != nil {
		log.Printf("api: zfs replica: %s skipped: %v", d.Dataset, err)
		return nil
	}
	unlock, ok := s.lockZFSReplica(id)
	if !ok {
		log.Printf("api: zfs replica: %s skipped, it is still replicating", d.Dataset)
		return nil
	}
	defer unlock()
	runID, err := s.startRun(ctx, id, store.ZFSReplicaRunKind)
	if err != nil {
		return err
	}
	return s.replicateZFS(ctx, d, runID)
}

// replicateAfterZFSBackup starts the replica a successful backup is followed
// by. It runs on its own, so the domain lock the backup holds is released
// while a long send is still going.
func (s *Service) replicateAfterZFSBackup(ctx context.Context, d store.ZFSDataset) {
	if !d.Replica.AfterBackup || d.Replica.TargetKind == store.ZFSReplicaTargetNone {
		return
	}
	s.replica.work.Add(1)
	go func() {
		defer s.replica.work.Done()
		defer s.recoverOperation("zfs replica after backup: "+d.Dataset, nil, nil)
		if err := s.ReplicateZFSDataset(context.WithoutCancel(ctx), d.ID); err != nil {
			log.Printf("api: zfs replica: %s after its backup failed: %v", d.Dataset, err)
		}
	}()
}

// replicateZFS is one run of an item whose replica lock the caller holds,
// from the run row to the notification.
func (s *Service) replicateZFS(ctx context.Context, d store.ZFSDataset, runID string) error {
	key := zfsReplicaProgressKey(d.ID)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer context.AfterFunc(s.StopContext(), cancel)()
	s.registerCancel(key, cancel)
	defer s.unregisterCancel(key)

	_, startedAt := s.progBegin(ctx, key, zfsReplicaPhase)
	res, err := s.runZFSReplica(ctx, d, runID, key, startedAt)
	s.progEnd(key, zfsReplicaPhase, err == nil, startedAt)
	s.finishZFSReplicaRun(ctx, d, runID, res, err)
	s.notifyZFSReplica(d, res, err)
	return err
}

// runZFSReplica resolves both ends and runs the engine. The error is the
// run's verdict: a refusal before any member, or the members that failed.
func (s *Service) runZFSReplica(ctx context.Context, d store.ZFSDataset, runID, key string, startedAt int64) (zfsrepl.Result, error) {
	src, err := s.zfsReplicaHostEnd()
	if err != nil {
		return zfsrepl.Result{}, err
	}
	tgt, err := s.zfsReplicaTargetFor(ctx, d)
	if err != nil {
		return zfsrepl.Result{}, err
	}
	keep, ok := d.Replica.Keep.Counts()
	if !ok {
		return zfsrepl.Result{}, fmt.Errorf("unknown keep preset %q", d.Replica.Keep.Preset)
	}
	entry := zfsrepl.Entry{
		Root:       d.Dataset,
		Excluded:   d.ExcludedChildren,
		TargetBase: tgt.base,
		Keep:       keep,
		Now:        time.Now,
		Progress:   s.zfsReplicaProgress(key, startedAt),
	}
	// A paired instance creates and marks its folders itself.
	if tgt.server != nil {
		entry.Owner = s.instanceID()
		if entry.Placeholders, err = s.claimZFSReplicaFolder(ctx, tgt.end, d, tgt.base, entry.Owner); err != nil {
			return zfsrepl.Result{}, err
		}
	}
	placeholder := map[string]bool{}
	for _, p := range entry.Placeholders {
		placeholder[p] = true
	}
	last, fixed := time.Now(), d.Replica.Folder != ""
	entry.Finished = func(m zfsrepl.MemberResult) {
		now := time.Now()
		s.recordZFSReplicaMember(d, runID, m, now.Sub(last), placeholder[m.Target])
		last = now
		if m.Code == "" && !fixed {
			fixed = true
			if err := s.store.FixZFSReplicaFolder(d.ID, tgt.folder); err != nil {
				log.Printf("api: zfs replica: fixing the folder of %s failed: %v", d.Dataset, err)
			}
		}
	}

	res, err := s.zfsReplicaEngine()(ctx, src, tgt.end, entry)
	if err != nil {
		return res, err
	}
	if len(res.Created) > 0 {
		log.Printf("api: zfs replica: created %s on the target for %s", strings.Join(res.Created, ", "), d.Dataset)
	}
	s.forgetGoneZFSReplicaMembers(d, res)
	return res, zfsReplicaMembersFailed(res)
}

// claimZFSReplicaFolder checks, before an item's first full stream, that the
// folder it lands in is not another instance's, and returns the target path
// of its root when that is an empty parent this instance created, which the
// stream may replace.
func (s *Service) claimZFSReplicaFolder(ctx context.Context, end zfsrepl.End, d store.ZFSDataset, base, owner string) ([]string, error) {
	_, landed, err := s.store.GetZFSReplicaState(d.ID, d.Dataset)
	if err != nil || landed {
		return nil, err
	}
	if who, err := zfsReplicaOwner(ctx, end, base); err != nil {
		return nil, err
	} else if who != "" && who != owner {
		return nil, zfsRefuse("target-owned", base)
	}
	root := base + "/" + d.Dataset
	who, err := zfsReplicaOwner(ctx, end, root)
	if err != nil || who != owner {
		return nil, err
	}
	return []string{root}, nil
}

// zfsReplicaOwner reads who created dataset on a target, "" for a dataset
// nobody marked or that does not exist.
func zfsReplicaOwner(ctx context.Context, end zfsrepl.End, dataset string) (string, error) {
	args, err := zfs.SourcePropertyArgs(dataset)
	if err != nil {
		return "", err
	}
	out, err := end.Run(ctx, args)
	if zfs.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return zfs.ParseSourceProperty(out), nil
}

// zfsReplicaProgress publishes how far the member being sent is. Members go
// one after another, so the bar starts again for each.
func (s *Service) zfsReplicaProgress(key string, startedAt int64) func(member string, done, total int64) {
	if s.progress == nil {
		return nil
	}
	current, last := "", -1.0
	return func(member string, done, total int64) {
		if total <= 0 {
			return
		}
		pct := min(float64(done)/float64(total)*100, 100)
		if member == current && pct < 100 && pct >= last && pct-last < 1 {
			return
		}
		current, last = member, pct
		s.progress.Publish(progress.Event{Key: key, Phase: zfsReplicaPhase, Percent: pct, Active: true, StartedAt: startedAt})
	}
}

// recordZFSReplicaMember writes what the run did to one member as it
// finishes, and where the member stands once it got there.
func (s *Service) recordZFSReplicaMember(d store.ZFSDataset, runID string, m zfsrepl.MemberResult, took time.Duration, placeholder bool) {
	row := store.ZFSReplicaRunMember{
		RunID:      runID,
		ItemID:     d.ID,
		Dataset:    m.Dataset,
		Base:       m.Base,
		Snapshot:   m.Snapshot,
		Bytes:      m.Bytes,
		Seconds:    int64(took / time.Second),
		Resumed:    m.Resumed,
		Code:       m.Code,
		FinishedAt: time.Now().Unix(),
	}
	if err := s.store.AddZFSReplicaRunMember(row); err != nil {
		log.Printf("api: zfs replica: recording %s failed: %v", m.Dataset, err)
	}
	if m.Code != "" {
		return
	}
	guid := strconv.FormatUint(m.GUID, 10)
	if err := s.store.PutZFSReplicaState(store.ZFSReplicaState{
		ItemID:        d.ID,
		Dataset:       m.Dataset,
		TargetPath:    m.Target,
		Volume:        m.Volume,
		SourceBase:    m.Snapshot,
		SourceGUID:    guid,
		TargetBase:    m.Snapshot,
		TargetGUID:    guid,
		CreatedParent: placeholder,
	}); err != nil {
		log.Printf("api: zfs replica: recording where %s stands failed: %v", m.Dataset, err)
	}
}

// forgetGoneZFSReplicaMembers drops the state of datasets the run did not
// find in the tree or that the item excludes.
func (s *Service) forgetGoneZFSReplicaMembers(d store.ZFSDataset, res zfsrepl.Result) {
	states, err := s.store.ListZFSReplicaStates(d.ID)
	if err != nil {
		log.Printf("api: zfs replica: reading the members of %s failed: %v", d.Dataset, err)
		return
	}
	present := map[string]bool{}
	for _, m := range res.Members {
		present[m.Dataset] = true
	}
	for _, st := range states {
		if present[st.Dataset] {
			continue
		}
		if err := s.store.DeleteZFSReplicaState(d.ID, st.Dataset); err != nil {
			log.Printf("api: zfs replica: forgetting %s failed: %v", st.Dataset, err)
		}
	}
}

// zfsReplicaMembersFailed turns the members that did not make it into the
// run's verdict, nil when every one did. The first failure's code is the
// run's.
func zfsReplicaMembersFailed(res zfsrepl.Result) error {
	var failed []string
	code := ""
	for _, m := range res.Members {
		if m.Code == "" {
			continue
		}
		if code == "" {
			code = m.Code
		}
		failed = append(failed, m.Dataset+" ["+m.Code+"]")
	}
	if code == "" {
		return nil
	}
	return &backup.ZFSRefusal{Code: code, Detail: fmt.Sprintf("%d of %d datasets failed: %s", len(failed), len(res.Members), strings.Join(failed, ", "))}
}

func zfsReplicaBytes(res zfsrepl.Result) int64 {
	var n int64
	for _, m := range res.Members {
		n += m.Bytes
	}
	return n
}

// finishZFSReplicaRun closes the run row. Its error ends in the reason code,
// which the replica card reads back.
func (s *Service) finishZFSReplicaRun(ctx context.Context, d store.ZFSDataset, runID string, res zfsrepl.Result, runErr error) {
	status, msg := "success", ""
	switch {
	case runErr == nil:
	case ctx.Err() != nil && s.StopContext().Err() != nil:
		status, msg = "cancelled", zfsRefusalSentence("replica", d.Dataset, &backup.ZFSRefusal{
			Code: "interrupted", Detail: "BombVault was shut down",
		})
	case ctx.Err() != nil:
		status, msg = "cancelled", "cancelled"
	default:
		status = "failed"
		msg = truncateRunErr(errors.New(zfsRefusalSentence("replica", d.Dataset, &backup.ZFSRefusal{
			Code: zfsReplicaCode(runErr), Detail: zfsDetail(zfsReplicaDetail(runErr)),
		})))
	}
	if err := s.store.FinishRun(runID, status, res.Snapshot, zfsReplicaBytes(res), msg); err != nil {
		log.Printf("api: zfs replica: finishing the run of %s failed: %v", d.Dataset, err)
	}
}

// zfsReplicaDetail is the text of a failure without the reason code a
// refusal of this domain prints itself, which the sentence appends once.
func zfsReplicaDetail(err error) string {
	var ref *backup.ZFSRefusal
	if errors.As(err, &ref) {
		return ref.Detail
	}
	return err.Error()
}

// zfsRunCode reads the reason code back off the end of a run's error.
func zfsRunCode(msg string) string {
	msg = strings.TrimSpace(msg)
	if !strings.HasSuffix(msg, "]") {
		return ""
	}
	i := strings.LastIndexByte(msg, '[')
	if i < 0 {
		return ""
	}
	return msg[i+1 : len(msg)-1]
}

// notifyZFSReplica reports a run like a backup is reported: a failure
// whenever notifications are on, a success only to whoever hears about every
// backup. It is no backup, so it leaves the domain's Healthchecks check alone,
// and a scheduled summary does not swallow it.
func (s *Service) notifyZFSReplica(d store.ZFSDataset, res zfsrepl.Result, runErr error) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	ctx := notify.WithHealthchecksSuppressed(context.Background())
	where := s.zfsReplicaTargetName(d)
	var msg string
	if runErr == nil {
		msg = fmt.Sprintf("Replica of zfs %q to %s is current (%d datasets, %s sent).",
			d.Dataset, where, len(res.Members), humanBytes(zfsReplicaBytes(res)))
	} else {
		msg = fmt.Sprintf("Replica of zfs %q to %s FAILED: %s [%s]", d.Dataset, where, scrubError(errors.New(zfsReplicaDetail(runErr))), zfsReplicaCode(runErr))
	}
	notify.Send(ctx, c, zfsDomain, notify.Event{Title: "BombVault", Message: msg, OK: runErr == nil})
	if s.unraidGate(c.Unraid) && (c.On == "always" || runErr != nil) {
		subject, level := "BombVault: replica OK", "normal"
		if runErr != nil {
			subject, level = "BombVault: replica FAILED", "warning"
		}
		if err := s.sendUnraidNotify(ctx, subject, msg, level); err != nil {
			log.Printf("notify: unraid: %v", err)
		}
	}
}

// zfsReplicaTargetName is how a message names where the item replicates to.
func (s *Service) zfsReplicaTargetName(d store.ZFSDataset) string {
	if d.Replica.TargetKind == store.ZFSReplicaTargetServer {
		if srv, ok, err := s.store.GetZFSReplicaServer(d.Replica.TargetID); err == nil && ok {
			return srv.Name
		}
	}
	return d.Replica.TargetID
}
