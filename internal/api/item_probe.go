package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The restore probe reads back a sample of one snapshot, small enough to run
// after a backup without anyone waiting on it.
const (
	probeMaxFiles = 100
	probeMaxBytes = 256 << 20
	// probeDumpBytes is how much of the largest file is streamed back when it
	// is too big for the sample, as a VM disk image always is.
	probeDumpBytes = 64 << 20
	probeTimeout   = 30 * time.Minute
)

// probeLockPoll is how often a queued probe asks for its domain again.
var probeLockPoll = 15 * time.Second

var errProbeBusy = errors.New("a backup or another check is running for this item's domain; try again when it finishes")

// probeItem is the item a probe reads: its target id, its domain and the key
// its repository is looked up by (a container or VM name, a folder set or
// dataset id, nothing for flash and config).
type probeItem struct {
	targetID string
	domain   string
	key      string
	name     string
}

// resolveProbeItem finds which domain a target id belongs to.
func (s *Service) resolveProbeItem(targetID string) (probeItem, error) {
	switch targetID {
	case store.FlashTargetID:
		return probeItem{targetID: targetID, domain: "flash", name: "Unraid flash"}, nil
	case store.ConfigTargetID:
		return probeItem{targetID: targetID, domain: "config", name: "App configuration"}, nil
	}
	if tg, err := s.store.GetTargetByID(targetID); err == nil {
		return probeItem{targetID: targetID, domain: "containers", key: tg.ContainerName, name: tg.ContainerName}, nil
	}
	if vm, err := s.store.GetVMTargetByID(targetID); err == nil {
		return probeItem{targetID: targetID, domain: "vms", key: vm.Name, name: vm.Name}, nil
	}
	if set, err := s.store.GetFileSet(targetID); err == nil {
		return probeItem{targetID: targetID, domain: "files", key: set.ID, name: set.Name}, nil
	}
	if d, err := s.store.GetZFSDataset(targetID); err == nil {
		return probeItem{targetID: targetID, domain: zfsDomain, key: d.ID, name: d.Dataset}, nil
	}
	return probeItem{}, fmt.Errorf("no item with id %q", targetID)
}

// itemPrimaryRepo is the primary repository an item's backups are written to.
func (s *Service) itemPrimaryRepo(settings store.Settings, it probeItem) (string, error) {
	switch it.domain {
	case "containers":
		return s.containerRepoForName(settings, it.key, "local")
	case "vms":
		return s.vmRepoForName(settings, it.key, "local")
	case "files":
		set, err := s.store.GetFileSet(it.key)
		if err != nil {
			return "", err
		}
		return s.fileSetRepoFor(settings, set, "local")
	case zfsDomain:
		d, err := s.store.GetZFSDataset(it.key)
		if err != nil {
			return "", err
		}
		return s.zfsDatasetRepoFor(settings, d, "local")
	}
	return s.repoFor(settings, it.domain, "local")
}

// EnableFirstProbes turns on the restore probe after an item's first backup.
// main calls it; a Service built for a test leaves it off, so no test backup
// starts a probe behind the test's back.
func (s *Service) EnableFirstProbes() { s.firstProbes.Store(true) }

// queueFirstProbe asks for the probe that follows an item's first backup. It
// returns at once; one worker takes the queue in order once the domain is free.
func (s *Service) queueFirstProbe(targetID string) {
	if !s.firstProbes.Load() {
		return
	}
	s.probeMu.Lock()
	defer s.probeMu.Unlock()
	if slices.Contains(s.probeQueue, targetID) {
		return
	}
	s.probeQueue = append(s.probeQueue, targetID)
	if s.probeWorking {
		return
	}
	s.probeWorking = true
	go s.drainProbeQueue()
}

func (s *Service) drainProbeQueue() {
	defer s.recoverOperation("restore probe", nil, nil)
	for {
		s.probeMu.Lock()
		if len(s.probeQueue) == 0 {
			s.probeWorking = false
			s.probeMu.Unlock()
			return
		}
		targetID := s.probeQueue[0]
		s.probeQueue = s.probeQueue[1:]
		s.probeMu.Unlock()
		s.firstProbe(context.Background(), targetID)
	}
}

// firstProbe runs the probe after an item's first backup, and nothing when the
// item already has one or was first backed up before probes existed. A probe
// that never got its turn leaves no row, so the next backup asks again.
func (s *Service) firstProbe(ctx context.Context, targetID string) {
	due, snapshotID, err := s.firstProbeDue(targetID)
	if err != nil {
		log.Printf("api: restore probe %s: %v", targetID, err) //nolint:gosec // G706: targetID is a stored id
		return
	}
	if !due {
		return
	}
	it, err := s.resolveProbeItem(targetID)
	if err != nil {
		log.Printf("api: restore probe: %v", err)
		return
	}
	unlock, ok := s.waitProbeLock(it.domain)
	if !ok {
		log.Printf("api: restore probe of %q skipped: %s stayed busy", it.name, it.domain) //nolint:gosec // G706: %q-quoted
		return
	}
	defer unlock()
	rec, err := s.probeAndRecord(ctx, it, snapshotID, "first")
	if errors.Is(err, errNothingToDrill) {
		return
	}
	if !rec.OK {
		s.notifyProbeFailure(ctx, it, rec.Detail)
	}
}

// firstProbeDue reports whether an item is owed its first-backup probe, and
// the snapshot of that backup.
func (s *Service) firstProbeDue(targetID string) (bool, string, error) {
	if _, found, err := s.store.LatestItemProbe(targetID); err != nil || found {
		return false, "", err
	}
	since, err := s.store.FirstProbesSince()
	if err != nil {
		return false, "", err
	}
	first, err := s.store.FirstSuccessfulBackupAt(targetID)
	if err != nil || first == 0 || first < since {
		return false, "", err
	}
	run, err := s.store.LastSuccessfulBackup(targetID)
	if err != nil || run == nil || run.SnapshotID == "" {
		return false, "", err
	}
	return true, run.SnapshotID, nil
}

// OpenScheduledRun marks a scheduled multi-item run of domain as open until
// the returned func is called. The scheduler brackets each such run with it.
func (s *Service) OpenScheduledRun(domain string) func() {
	s.runsOpenMu.Lock()
	defer s.runsOpenMu.Unlock()
	if s.runsOpen == nil {
		s.runsOpen = map[string]int{}
	}
	s.runsOpen[domain]++
	return func() {
		s.runsOpenMu.Lock()
		defer s.runsOpenMu.Unlock()
		s.runsOpen[domain]--
	}
}

func (s *Service) scheduledRunOpen(domain string) bool {
	s.runsOpenMu.Lock()
	defer s.runsOpenMu.Unlock()
	return s.runsOpen[domain] > 0
}

// waitProbeLock takes the domain once nothing else wants it: a batch, a
// Backup Everything pass or a scheduled run of the domain keeps going first,
// since a probe between two of its items would hold the next one up.
func (s *Service) waitProbeLock(domain string) (func(), bool) {
	deadline := time.Now().Add(drillLockWait)
	for {
		if !s.batchActive.Load() && !s.everythingActive.Load() && !s.scheduledRunOpen(domain) {
			if unlock, ok := s.tryLockDomainFor(domain, "verify"); ok {
				return unlock, true
			}
		}
		if time.Now().After(deadline) {
			return nil, false
		}
		time.Sleep(probeLockPoll)
	}
}

// ProbeItem runs a restore probe of an item's newest backup now and records
// it. The result is on the screen of whoever asked, so it sends nothing.
func (s *Service) ProbeItem(ctx context.Context, targetID string) (store.ItemProbe, error) {
	it, err := s.resolveProbeItem(targetID)
	if err != nil {
		return store.ItemProbe{}, err
	}
	run, err := s.store.LastSuccessfulBackup(targetID)
	if err != nil {
		return store.ItemProbe{}, err
	}
	if run == nil || run.SnapshotID == "" {
		return store.ItemProbe{}, errors.New("this item has no backup with data to check yet")
	}
	unlock, ok := s.tryLockDomainFor(it.domain, "verify")
	if !ok {
		return store.ItemProbe{}, errProbeBusy
	}
	defer unlock()
	rec, err := s.probeAndRecord(ctx, it, run.SnapshotID, "manual")
	if errors.Is(err, errNothingToDrill) {
		return store.ItemProbe{}, errors.New("this backup holds no files, so there is nothing to check")
	}
	return rec, nil
}

// probeAndRecord runs one probe under the caller's domain lock and stores its
// outcome. A snapshot without files records nothing.
func (s *Service) probeAndRecord(ctx context.Context, it probeItem, snapshotID, trigger string) (store.ItemProbe, error) {
	key := "probe:" + it.domain + ":" + it.name
	pctx, startedAt := s.progBegin(context.WithoutCancel(ctx), key, "maintenance")
	res, err := s.runItemProbe(pctx, it, snapshotID)
	s.progEnd(key, "maintenance", err == nil, startedAt)
	if errors.Is(err, errNothingToDrill) {
		return store.ItemProbe{}, err
	}
	rec := store.ItemProbe{
		TargetID: it.targetID, Domain: it.domain, At: time.Now().Unix(),
		OK: err == nil, SnapshotID: snapshotID, Files: res.files, Bytes: res.bytes, Trigger: trigger,
	}
	if err != nil {
		rec.Detail = truncateDetail(scrubError(err))
	}
	if aErr := s.store.AddItemProbe(rec); aErr != nil {
		return rec, fmt.Errorf("record probe: %w", aErr)
	}
	return rec, nil
}

func truncateDetail(s string) string {
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

type probeResult struct {
	files int
	bytes int64
}

// runItemProbe restores a sample of a snapshot into a sandbox with restic's
// own read-back check, compares every restored size with the listing, and
// streams the start of the largest file when it was too big for the sample.
func (s *Service) runItemProbe(ctx context.Context, it probeItem, snapshotID string) (probeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	settings, err := s.store.GetSettings()
	if err != nil {
		return probeResult{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.itemPrimaryRepo(settings, it)
	if err != nil {
		return probeResult{}, err
	}
	mode := s.primaryModeFor(settings, it.domain, repo)
	// An interrupted run's lock would stop the restore below outright. A remote
	// repository is unlocked only when a lock is in the way, since every call
	// to it is billed.
	if !restic.IsRemoteRepo(repo) {
		s.unlockStale(ctx, repo, mode)
	}

	sampler := newProbeSampler(rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0))) //nolint:gosec // G404: picks which files to read back, nothing secret
	layout := blockLayout{}
	if err := s.retryAfterUnlock(ctx, repo, mode, func() error {
		return s.engine.LsStream(ctx, repo, snapshotID, mode, func(e restic.FileEntry) {
			sampler.add(e)
			if it.domain == "vms" {
				layout.add(e)
			}
		})
	}); err != nil {
		return probeResult{}, fmt.Errorf("list the backup: %w", err)
	}
	// A changed-block snapshot is a set of segments per disk. The sample reads
	// some of them back; a missing or short one would only show when the disk
	// is put back together, so the listing is held against the manifest too.
	if layout.manifest {
		m, err := (&vmBlockRestic{engine: s.engine, mode: mode}).Manifest(ctx, repo, snapshotID)
		if err != nil {
			return probeResult{}, fmt.Errorf("read the disk list: %w", err)
		}
		for _, d := range m.Disks {
			if err := backup.CheckBlocksLayout(d, layout.segments["/"+d.Dev]); err != nil {
				return probeResult{}, err
			}
		}
	}
	files, bytes, big := sampler.pick()
	if len(files) == 0 && big == nil {
		return probeResult{}, errNothingToDrill
	}

	sandbox, cleanup, err := s.newDrillSandbox(settings, "bombvault-probe")
	if err != nil {
		return probeResult{}, err
	}
	defer cleanup()

	if len(files) > 0 {
		includes := make([]string, len(files))
		for i, f := range files {
			includes[i] = f.Path
		}
		if err := s.engine.RestoreVerify(ctx, repo, snapshotID, includes, sandbox, mode); err != nil && !errors.Is(err, restic.ErrRestoreMetadataOnly) {
			return probeResult{}, fmt.Errorf("restore the sample: %w", err)
		}
		for _, f := range files {
			info, sErr := os.Stat(filepath.Join(sandbox, filepath.FromSlash(f.Path))) //nolint:gosec // G703: sandbox is resolved under the host mount root, the path came from restic's listing of that snapshot
			if sErr != nil {
				return probeResult{}, fmt.Errorf("%s was not restored", f.Path)
			}
			if info.Size() != f.Size {
				return probeResult{}, fmt.Errorf("%s came back with %d bytes, the backup has %d", f.Path, info.Size(), f.Size)
			}
		}
	}
	res := probeResult{files: len(files), bytes: bytes}
	if big != nil {
		n, err := s.dumpHead(ctx, repo, snapshotID, big.Path, mode)
		if err != nil {
			return res, fmt.Errorf("read back %s: %w", big.Path, err)
		}
		res.files++
		res.bytes += n
	}
	return res, nil
}

// errHeadRead stops a dump once enough of the file came back.
var errHeadRead = errors.New("probe read enough")

type headWriter struct {
	n, limit int64
}

func (w *headWriter) Write(p []byte) (int, error) {
	take := min(int64(len(p)), w.limit-w.n)
	w.n += take
	if w.n >= w.limit {
		return int(take), errHeadRead
	}
	return len(p), nil
}

// dumpHead streams the first probeDumpBytes of one file out of the snapshot.
// Every block restic hands over has passed its hash check, so reading it is
// the proof; the rest of the file is left alone.
func (s *Service) dumpHead(ctx context.Context, repo, snapshotID, file string, mode restic.Mode) (int64, error) {
	dctx, cancel := context.WithCancel(ctx)
	defer cancel()
	w := &headWriter{limit: probeDumpBytes}
	err := s.engine.DumpRaw(dctx, repo, snapshotID, file, w, mode)
	if w.n >= w.limit {
		// restic fails on the pipe the writer closed, which is how this
		// dump is meant to end.
		return w.n, nil
	}
	return w.n, err
}

// blockLayout collects the segment files of a changed-block VM snapshot by
// disk folder, and whether the snapshot carries the manifest that names them.
type blockLayout struct {
	manifest bool
	segments map[string]map[string]int64
}

func (l *blockLayout) add(e restic.FileEntry) {
	if e.Type != "file" {
		return
	}
	if e.Path == "/"+backup.BlocksManifestName {
		l.manifest = true
		return
	}
	dir := path.Dir(e.Path)
	if path.Dir(dir) != "/" {
		return
	}
	if l.segments == nil {
		l.segments = map[string]map[string]int64{}
	}
	if l.segments[dir] == nil {
		l.segments[dir] = map[string]int64{}
	}
	l.segments[dir][path.Base(e.Path)] = e.Size
}

// probeSampler draws the probe sample while restic lists the snapshot, so a
// listing of a million files is never held in memory.
type probeSampler struct {
	rng  *rand.Rand
	seen int
	pool []restic.FileEntry
	big  *restic.FileEntry
}

func newProbeSampler(rng *rand.Rand) *probeSampler {
	return &probeSampler{rng: rng}
}

func (p *probeSampler) add(e restic.FileEntry) {
	if e.Type != "file" {
		return
	}
	if e.Size > probeMaxBytes {
		if p.big == nil || e.Size > p.big.Size {
			big := e
			p.big = &big
		}
		return
	}
	// restic reads each include as a pattern, so a name with a glob character
	// could match something else or nothing.
	if strings.ContainsAny(e.Path, `*?[]\`) {
		return
	}
	p.seen++
	if len(p.pool) < probeMaxFiles {
		p.pool = append(p.pool, e)
		return
	}
	if j := p.rng.IntN(p.seen); j < probeMaxFiles {
		p.pool[j] = e
	}
}

// pick keeps the smallest files of the pool that fit the byte budget, and the
// largest file that was too big for it.
func (p *probeSampler) pick() ([]restic.FileEntry, int64, *restic.FileEntry) {
	pool := slices.Clone(p.pool)
	slices.SortFunc(pool, func(a, b restic.FileEntry) int {
		if a.Size != b.Size {
			if a.Size < b.Size {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Path, b.Path)
	})
	var out []restic.FileEntry
	var total int64
	for _, e := range pool {
		if total+e.Size > probeMaxBytes {
			break
		}
		total += e.Size
		out = append(out, e)
	}
	return out, total, p.big
}

// newDrillSandbox creates an empty directory under the restore folder and
// marks it as ours before anything is written into it. cleanup removes it only
// while the marker is there.
func (s *Service) newDrillSandbox(settings store.Settings, prefix string) (string, func(), error) {
	sub := path.Join(settings.RestoreFolder, fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()))
	sandbox, err := paths.Resolve(s.cfg.HostMountRoot, sub)
	if err != nil {
		return "", nil, errors.New("invalid restore folder: must be a relative subpath under the host mount")
	}
	if err := paths.EnsureDir(filepath.Dir(sandbox)); err != nil {
		return "", nil, fmt.Errorf("create sandbox parent: %w", err)
	}
	// Mkdir rather than MkdirAll: it fails on a directory that is already
	// there, so a sandbox is always one this call created.
	if err := os.Mkdir(sandbox, 0o700); err != nil { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root by paths.Resolve
		return "", nil, fmt.Errorf("create sandbox: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sandbox, drillMarkerName), sandboxMarker(s.instanceID()), 0o600); err != nil { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root by paths.Resolve
		if rmErr := os.Remove(sandbox); rmErr != nil { //nolint:gosec // G703: sandbox is resolved strictly under the host mount root and was just created empty
			log.Printf("api: sandbox: could not remove %s after the marker write failed: %v", sandbox, rmErr)
		}
		return "", nil, fmt.Errorf("write sandbox marker: %w", err)
	}
	cleanup := func() {
		if cErr := cleanupDrillSandbox(sandbox); cErr != nil {
			log.Printf("api: sandbox cleanup: %v", cErr)
		}
	}
	return sandbox, cleanup, nil
}

// notifyProbeFailure reports a failed first-backup probe the way a failed
// restore check is reported.
func (s *Service) notifyProbeFailure(ctx context.Context, it probeItem, detail string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	msg := fmt.Sprintf("The restore check after the first backup of %s failed, so this backup may not be restorable: %s", it.name, detail)
	notify.Send(ctx, c, it.domain, notify.Event{Title: "BombVault", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: restore check failed", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}
