package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// foreignTestKey is a valid-shaped FOREIGN APP_KEY (64 lowercase hex) — hex so
// restickey.Derive (which panics on non-hex) can derive from it.
const foreignTestKey = "4242424242424242424242424242424242424242424242424242424242424242"

// foreignRecordingEngine is a call-recording ResticEngine for the foreign-
// session guard tests. The embedded interface is nil ON PURPOSE: any engine
// method the foreign path calls that is not explicitly overridden here panics,
// so a write we forgot to record can never slip through silently. opens
// decides which probe modes RepoOpens accepts (nil = none).
type foreignRecordingEngine struct {
	ResticEngine // nil — non-overridden calls panic loudly
	opens        func(m restic.Mode) bool
	snaps        []restic.Snapshot
	snapsErr     error
	lsEntries    []restic.FileEntry // what Ls returns (the foreign subfolder picker's tree)

	// statsRestoreBytes is the size StatsRestoreSize reports (the VM restore guard's
	// "does it fit" number); 0 (the default) skips the free-space check.
	statsRestoreBytes int64

	mu            sync.Mutex
	calls         []string
	snapshotRepos []string      // repo argument of every Snapshots call (which repo was listed)
	restores      []string      // "Method|repo|snapshot|path" of every RestorePath/RestoreInclude call
	modes         []restic.Mode // mode of every read/restore call, for NoLock + cloudEnv assertions
}

func (f *foreignRecordingEngine) record(name string) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	f.mu.Unlock()
}

func (f *foreignRecordingEngine) recordMode(m restic.Mode) {
	f.mu.Lock()
	f.modes = append(f.modes, m)
	f.mu.Unlock()
}

// everyModeNoLock reports whether every recorded read/restore mode carried
// NoLock (so the foreign session never took a repository lock), and false when no
// call was recorded at all.
func (f *foreignRecordingEngine) everyModeNoLock() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.modes) == 0 {
		return false
	}
	for _, m := range f.modes {
		if !m.NoLock {
			return false
		}
	}
	return true
}

// anyModeHasEnv reports whether any recorded mode carried backend credentials
// (cloudEnv) — which a local-only foreign session must never attach.
func (f *foreignRecordingEngine) anyModeHasEnv() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.modes {
		if len(m.Env) > 0 {
			return true
		}
	}
	return false
}

// calledForbidden returns the recorded calls that are in the forbidden set.
func (f *foreignRecordingEngine) calledForbidden() []string {
	forbidden := map[string]bool{
		"Init": true, "Forget": true, "ForgetPolicy": true, "Prune": true,
		"TagAdd": true, "Backup": true, "Copy": true,
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		if forbidden[c] {
			out = append(out, c)
		}
	}
	return out
}

func (f *foreignRecordingEngine) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == name {
			n++
		}
	}
	return n
}

// Reads the foreign path may legitimately perform.
func (f *foreignRecordingEngine) RepoOpens(_ context.Context, _ string, m restic.Mode) bool {
	f.record("RepoOpens")
	f.recordMode(m)
	return f.opens != nil && f.opens(m)
}

func (f *foreignRecordingEngine) Snapshots(_ context.Context, repo string, m restic.Mode) ([]restic.Snapshot, error) {
	f.record("Snapshots")
	f.recordMode(m)
	f.mu.Lock()
	f.snapshotRepos = append(f.snapshotRepos, repo)
	f.mu.Unlock()
	return f.snaps, f.snapsErr
}

func (f *foreignRecordingEngine) Unlock(_ context.Context, _ string, _ bool, _ restic.Mode) error {
	f.record("Unlock") // stale-lock self-heal on the read path; not a repo write
	return nil
}

// The restore operations a foreign restore may legitimately run: they READ
// the foreign repo and write only to LOCAL paths, so they are allowed —
// recorded with their arguments so tests can pin WHICH repo was restored from.
func (f *foreignRecordingEngine) RestorePath(_ context.Context, repo, snapshotID, path string, m restic.Mode) error {
	f.record("RestorePath")
	f.recordMode(m)
	f.mu.Lock()
	f.restores = append(f.restores, "RestorePath|"+repo+"|"+snapshotID+"|"+path)
	f.mu.Unlock()
	return nil
}

func (f *foreignRecordingEngine) RestoreInclude(_ context.Context, repo, snapshotID, includePath, target string, m restic.Mode) error {
	f.record("RestoreInclude")
	f.recordMode(m)
	f.mu.Lock()
	f.restores = append(f.restores, "RestoreInclude|"+repo+"|"+snapshotID+"|"+includePath+"->"+target)
	f.mu.Unlock()
	return nil
}

func (f *foreignRecordingEngine) RestoreSubtreeTo(_ context.Context, repo, snapshotID, subtreePath, target string, m restic.Mode) error {
	f.record("RestoreSubtreeTo")
	f.recordMode(m)
	f.mu.Lock()
	f.restores = append(f.restores, "RestoreSubtreeTo|"+repo+"|"+snapshotID+"|"+subtreePath+"->"+target)
	f.mu.Unlock()
	return nil
}

// RestoreSubtreeInclude is the selective (pick-some-files/subfolder) restore the
// #123 foreign path uses: it reads the foreign repo and writes only to a LOCAL
// target, so it is allowed — recorded with subtree + the subtree-relative include.
func (f *foreignRecordingEngine) RestoreSubtreeInclude(_ context.Context, repo, snapshotID, subtreePath, includePath, target string, m restic.Mode) error {
	f.record("RestoreSubtreeInclude")
	f.recordMode(m)
	f.mu.Lock()
	f.restores = append(f.restores, "RestoreSubtreeInclude|"+repo+"|"+snapshotID+"|"+subtreePath+"|"+includePath+"->"+target)
	f.mu.Unlock()
	return nil
}

// Ls lists a foreign snapshot's files for the recovery subfolder picker — a READ
// of the source repo, so it is allowed. lsEntries lets a test fix the returned
// tree; the mode is recorded so tests can assert the listing stays lock-free.
func (f *foreignRecordingEngine) Ls(_ context.Context, _, _ string, m restic.Mode) ([]restic.FileEntry, error) {
	f.record("Ls")
	f.recordMode(m)
	return f.lsEntries, nil
}

// StatsRestoreSize is a READ of the source repo the VM restore's host-brick guard
// uses to size the restore. statsRestoreBytes lets a test force a specific size.
func (f *foreignRecordingEngine) StatsRestoreSize(_ context.Context, _, _ string, m restic.Mode) (int, int64, error) {
	f.record("StatsRestoreSize")
	f.recordMode(m)
	return 0, f.statsRestoreBytes, nil
}

// The forbidden writes — implemented (recording) rather than left to panic, so
// a regression yields a precise assertion failure naming the violating call.
func (f *foreignRecordingEngine) Init(_ context.Context, _ string, _ restic.Mode) error {
	f.record("Init")
	return nil
}

func (f *foreignRecordingEngine) Backup(_ context.Context, _ string, _, _ []string, _ restic.Mode, _ ...string) (restic.Summary, error) {
	f.record("Backup")
	return restic.Summary{}, nil
}

func (f *foreignRecordingEngine) Forget(_ context.Context, _ string, _ []string, _ bool, _ restic.Mode) error {
	f.record("Forget")
	return nil
}

func (f *foreignRecordingEngine) ForgetPolicy(_ context.Context, _ string, _ restic.RetentionPolicy, _ restic.Mode, _ string, _ bool) error {
	f.record("ForgetPolicy")
	return nil
}

func (f *foreignRecordingEngine) Prune(_ context.Context, _ string, _ restic.Mode) error {
	f.record("Prune")
	return nil
}

func (f *foreignRecordingEngine) TagAdd(_ context.Context, _, _ string, _ []string, _ restic.Mode) error {
	f.record("TagAdd")
	return nil
}

func (f *foreignRecordingEngine) Copy(_ context.Context, _, _ string, _ []string, _ restic.Limits, _ restic.Mode) error {
	f.record("Copy")
	return nil
}

// opensEncrypted accepts only the key-derived encrypted probe mode (the shape
// of a normal foreign BombVault repo).
func opensEncrypted(m restic.Mode) bool { return m.Encrypted }

// newForeignTestService builds a Service over an in-memory store and the given
// engine — the same bare-literal construction the other internal tests use.
func newForeignTestService(t *testing.T, eng ResticEngine) *Service {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	s := &Service{
		store:  st,
		engine: eng,
		cfg:    config.Config{HostMountRoot: t.TempDir(), AppKey: strings.Repeat("a", 64)},
	}
	// Stop the background session janitor OpenForeign starts, so its goroutine
	// never outlives the test (no-op when a test never opens a session).
	t.Cleanup(s.stopForeignJanitor)
	return s
}

// seedForeignRepoMarker writes restic's config marker under a LOCAL mounted
// subpath of the test's host mount, so a foreign session pointed at that path
// counts the repo as present (a local repo with no config marker reads as
// "missing", short-circuiting the snapshot listing). Returns the resolved repo
// path — exactly the repo string the engine records.
func seedForeignRepoMarker(t *testing.T, s *Service, sub string) string {
	t.Helper()
	repo, err := paths.Resolve(s.cfg.HostMountRoot, sub)
	if err != nil {
		t.Fatalf("resolve foreign repo %q: %v", sub, err)
	}
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("cfg"), 0o600); err != nil {
		t.Fatal(err)
	}
	return repo
}

// TestOpenForeignLeavesSettingsUntouched pins guarantee #1 of the foreign
// mode: opening a foreign repo persists NOTHING — Settings reads back deeply
// equal after the open. (The Recovery attach flow persists via UpdateSettings;
// foreign sessions must never take that path.)
func TestOpenForeignLeavesSettingsUntouched(t *testing.T) {
	eng := &foreignRecordingEngine{
		opens: opensEncrypted,
		snaps: []restic.Snapshot{{ID: "aaaaaaaa11111111", Time: "2026-07-01T10:00:00Z", Tags: []string{"container:web"}}},
	}
	s := newForeignTestService(t, eng)

	before, err := s.store.GetSettings()
	if err != nil {
		t.Fatalf("settings before: %v", err)
	}
	if _, _, err := s.OpenForeign(context.Background(), "backups/other", foreignTestKey); err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}
	after, err := s.store.GetSettings()
	if err != nil {
		t.Fatalf("settings after: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("OpenForeign must not touch settings:\nbefore: %+v\nafter:  %+v", before, after)
	}
}

// TestOpenForeignIsReadOnly pins guarantee #2: opening a foreign repo performs
// ONLY read probes and listings — never Init, Forget, ForgetPolicy, Prune,
// TagAdd, Backup, or Copy (an Init would create an empty repository on someone
// else's storage; the others would mutate their backups).
func TestOpenForeignIsReadOnly(t *testing.T) {
	eng := &foreignRecordingEngine{
		opens: opensEncrypted,
		snaps: []restic.Snapshot{{ID: "aaaaaaaa11111111", Time: "2026-07-01T10:00:00Z", Tags: []string{"container:web"}}},
	}
	s := newForeignTestService(t, eng)

	if _, _, err := s.OpenForeign(context.Background(), "backups/other", foreignTestKey); err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}
	if bad := eng.calledForbidden(); len(bad) > 0 {
		t.Fatalf("OpenForeign performed forbidden repo writes: %v (all calls: %v)", bad, eng.calls)
	}
	// Sanity: the recorder actually saw the expected reads.
	if eng.count("RepoOpens") == 0 || eng.count("Snapshots") == 0 {
		t.Fatalf("expected RepoOpens + Snapshots reads, got calls %v", eng.calls)
	}
	// Every probe/listing was lock-free (never writes a lock into the foreign
	// repo) and carried no cloud credentials (local-only session).
	if !eng.everyModeNoLock() {
		t.Fatalf("every foreign probe/listing must be lock-free (NoLock), got modes %+v", eng.modes)
	}
	if eng.anyModeHasEnv() {
		t.Fatalf("a foreign session must not attach cloud credentials, got modes %+v", eng.modes)
	}
}

// TestOpenForeignValidation pins the boundary guards: the key must be exactly
// 64 lowercase hex BEFORE anything touches the engine (restickey.Derive panics
// on non-hex input), the location must be non-empty, and a repo that opens
// with neither mode yields the clear wrong-key/not-a-repo error after exactly
// the two read probes.
func TestOpenForeignValidation(t *testing.T) {
	badKeys := []string{"", "short", strings.Repeat("AB", 32), strings.Repeat("g", 64)}
	for _, key := range badKeys {
		eng := &foreignRecordingEngine{}
		s := newForeignTestService(t, eng)
		if _, _, err := s.OpenForeign(context.Background(), "backups/other", key); err == nil || !strings.Contains(err.Error(), "64 lowercase hex") {
			t.Fatalf("key %q: want the key-shape error, got %v", key, err)
		}
		if len(eng.calls) != 0 {
			t.Fatalf("key %q: validation must precede any engine call, got %v", key, eng.calls)
		}
	}

	s := newForeignTestService(t, &foreignRecordingEngine{})
	if _, _, err := s.OpenForeign(context.Background(), "   ", foreignTestKey); err == nil || !strings.Contains(err.Error(), "location") {
		t.Fatalf("empty location: want the missing-location error, got %v", err)
	}

	eng := &foreignRecordingEngine{} // opens nothing
	s = newForeignTestService(t, eng)
	_, _, err := s.OpenForeign(context.Background(), "backups/other", foreignTestKey)
	if err == nil || !strings.Contains(err.Error(), "could not open the repository") {
		t.Fatalf("unopenable repo: want the wrong-key/not-a-repo error, got %v", err)
	}
	if got := eng.count("RepoOpens"); got != 2 { // encrypted probe, then plain
		t.Fatalf("expected exactly 2 read probes, got %d (%v)", got, eng.calls)
	}
	if eng.count("Snapshots") != 0 {
		t.Fatalf("an unopenable repo must not be listed, got %v", eng.calls)
	}
}

// TestOpenForeignModeDetection pins the probe order and result: an encrypted
// repo yields a session whose mode carries the key-DERIVED password (not the
// raw key); a plain repo falls back to the unencrypted mode.
func TestOpenForeignModeDetection(t *testing.T) {
	// Encrypted repo.
	eng := &foreignRecordingEngine{opens: opensEncrypted}
	s := newForeignTestService(t, eng)
	id, _, err := s.OpenForeign(context.Background(), "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign (encrypted): %v", err)
	}
	sess, err := s.foreignSession(id)
	if err != nil {
		t.Fatalf("foreignSession: %v", err)
	}
	if !sess.mode.Encrypted || sess.mode.Password == "" || sess.mode.Password == foreignTestKey {
		t.Fatalf("encrypted session mode must carry the DERIVED password, got %+v", sess.mode)
	}
	if sess.id != id || sess.key != foreignTestKey {
		t.Fatalf("session must carry its id + the foreign key for later def decryption, got %+v", sess)
	}
	if !strings.HasSuffix(strings.ReplaceAll(sess.repo, "\\", "/"), "/backups/other") {
		t.Fatalf("session repo must be the mount-root-resolved location, got %q", sess.repo)
	}

	// Plain repo (encrypted probe fails, plain succeeds).
	eng = &foreignRecordingEngine{opens: func(m restic.Mode) bool { return !m.Encrypted }}
	s = newForeignTestService(t, eng)
	id, _, err = s.OpenForeign(context.Background(), "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign (plain): %v", err)
	}
	if sess, err = s.foreignSession(id); err != nil {
		t.Fatalf("foreignSession: %v", err)
	}
	if sess.mode.Encrypted || sess.mode.Password != "" {
		t.Fatalf("plain session mode must be unencrypted, got %+v", sess.mode)
	}
}

// TestForeignSessionLifecycle pins the in-memory session store: unknown ids
// error, expired ids error AND are swept, Close drops a session immediately,
// and a fresh open expires foreignSessionTTL from now.
func TestForeignSessionLifecycle(t *testing.T) {
	eng := &foreignRecordingEngine{opens: opensEncrypted}
	s := newForeignTestService(t, eng)

	// Unknown session — including on a Service that never opened one.
	if _, err := s.foreignSession("nope"); !errors.Is(err, errForeignSession) {
		t.Fatalf("unknown session: want errForeignSession, got %v", err)
	}

	id, _, err := s.OpenForeign(context.Background(), "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}
	sess, err := s.foreignSession(id)
	if err != nil {
		t.Fatalf("fresh session must resolve: %v", err)
	}
	wantExpiry := time.Now().Add(foreignSessionTTL)
	if sess.expires.Before(wantExpiry.Add(-time.Minute)) || sess.expires.After(wantExpiry.Add(time.Minute)) {
		t.Fatalf("session expiry = %v, want ~%v (TTL %v)", sess.expires, wantExpiry, foreignSessionTTL)
	}

	// Expire it: the lookup errors AND the sweep removes the entry.
	s.foreignMu.Lock()
	sess = s.foreignSessions[id]
	sess.expires = time.Now().Add(-time.Second)
	s.foreignSessions[id] = sess
	s.foreignMu.Unlock()
	if _, err := s.foreignSession(id); !errors.Is(err, errForeignSession) {
		t.Fatalf("expired session: want errForeignSession, got %v", err)
	}
	s.foreignMu.Lock()
	_, still := s.foreignSessions[id]
	s.foreignMu.Unlock()
	if still {
		t.Fatal("expired session must be swept from the store")
	}

	// Close drops a live session immediately; closing again is a no-op.
	id, _, err = s.OpenForeign(context.Background(), "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}
	s.CloseForeign(id)
	if _, err := s.foreignSession(id); !errors.Is(err, errForeignSession) {
		t.Fatalf("closed session: want errForeignSession, got %v", err)
	}
	s.CloseForeign(id) // harmless no-op
}

// TestForeignInventoryGrouping pins the inventory shape Tasks 10/11 consume:
// snapshots grouped by the container:/vm:/fileset: tag prefixes, items sorted
// by name, snapshots kept in restic's order, untagged snapshots in no group —
// and the exact JSON keys (containers/vms/fileSets, [] when empty).
func TestForeignInventoryGrouping(t *testing.T) {
	snaps := []restic.Snapshot{
		{ID: "aaaaaaaa11111111", Time: "2026-07-01T10:00:00Z", Tags: []string{"container:web"}},
		{ID: "bbbbbbbb22222222", Time: "2026-07-02T10:00:00Z", Tags: []string{"container:web"}},
		{ID: "cccccccc33333333", Time: "2026-07-03T10:00:00Z", Tags: []string{"container:alpha"}},
		{ID: "dddddddd44444444", Time: "2026-07-04T10:00:00Z", Tags: []string{"vm:win11"}},
		{ID: "eeeeeeee55555555", Time: "2026-07-05T10:00:00Z", Tags: []string{"fileset:docs"}},
		{ID: "ffffffff66666666", Time: "2026-07-06T10:00:00Z"}, // untagged → in no group
	}
	eng := &foreignRecordingEngine{opens: opensEncrypted, snaps: snaps}
	s := newForeignTestService(t, eng)

	_, inv, err := s.OpenForeign(context.Background(), "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}
	if len(inv.Containers) != 2 || inv.Containers[0].Name != "alpha" || inv.Containers[1].Name != "web" {
		t.Fatalf("containers must be name-sorted [alpha web], got %+v", inv.Containers)
	}
	if len(inv.Containers[1].Snapshots) != 2 || inv.Containers[1].Snapshots[0].ID != "aaaaaaaa11111111" {
		t.Fatalf("web must keep both snapshots in restic order, got %+v", inv.Containers[1].Snapshots)
	}
	if len(inv.VMs) != 1 || inv.VMs[0].Name != "win11" {
		t.Fatalf("vms = %+v, want [win11]", inv.VMs)
	}
	if len(inv.FileSets) != 1 || inv.FileSets[0].Name != "docs" {
		t.Fatalf("fileSets = %+v, want [docs]", inv.FileSets)
	}

	// JSON contract for the frontend: exact keys, [] (never null) when a repo
	// holds no snapshots at all.
	sEmpty := newForeignTestService(t, &foreignRecordingEngine{opens: opensEncrypted})
	_, invEmpty, err := sEmpty.OpenForeign(context.Background(), "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign (empty repo): %v", err)
	}
	raw, err := json.Marshal(invEmpty)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `{"containers":[],"vms":[],"fileSets":[]}` {
		t.Fatalf("empty inventory JSON = %s, want exact containers/vms/fileSets keys with []", raw)
	}
}

// ---------------------------------------------------------------------------
// Foreign restore (Task 10)
// ---------------------------------------------------------------------------

// foreignFakeDocker is the minimal Docker fake the foreign container restore
// needs: every lifecycle call succeeds, InspectName reports "absent" (a fresh
// restore recreates the container), and CreateAndStart records the recreate
// recipe so the test can pin that the FOREIGN definition drove it.
type foreignFakeDocker struct {
	mu           sync.Mutex
	created      int
	createdIn    model.Inspect
	createdStart bool
}

var _ dockercli.Docker = (*foreignFakeDocker)(nil)

func (f *foreignFakeDocker) List(context.Context) ([]dockercli.ContainerInfo, error) {
	return nil, nil
}

func (f *foreignFakeDocker) Inspect(context.Context, string) (model.Inspect, error) {
	return model.Inspect{}, errors.New("no such container") // deleted — the stored def must be used
}

func (f *foreignFakeDocker) Allocations(context.Context) ([]model.Allocation, error) {
	return nil, nil
}

func (f *foreignFakeDocker) Stop(context.Context, string, time.Duration) error    { return nil }
func (f *foreignFakeDocker) Start(context.Context, string) error                  { return nil }
func (f *foreignFakeDocker) Restart(context.Context, string, time.Duration) error { return nil }
func (f *foreignFakeDocker) WaitRunning(context.Context, string, time.Duration) error {
	return nil
}
func (f *foreignFakeDocker) Health(context.Context, string) (model.Health, error) {
	return model.Health{Running: true}, nil
}
func (f *foreignFakeDocker) Remove(context.Context, string) error               { return nil }
func (f *foreignFakeDocker) Pull(context.Context, string) error                 { return nil }
func (f *foreignFakeDocker) PullWithAuth(context.Context, string, string) error { return nil }
func (f *foreignFakeDocker) ImageID(context.Context, string) (string, error)    { return "", nil }
func (f *foreignFakeDocker) ImageRemove(context.Context, string) error          { return nil }

func (f *foreignFakeDocker) CreateAndStart(_ context.Context, in model.Inspect, start bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created++
	f.createdIn, f.createdStart = in, start
	return nil
}

func (f *foreignFakeDocker) InspectName(context.Context, string) (string, error) { return "", nil }
func (f *foreignFakeDocker) Self(context.Context) (string, error)                { return "", nil }
func (f *foreignFakeDocker) Exec(context.Context, string, []string) error        { return nil }

// waitForeignIdle blocks until the detached foreign-restore goroutine has
// released the shared single-flight guard — i.e. progress, cancel and run
// bookkeeping are done — so a test never races its cleanup (closing the
// in-memory store) against the async work.
func waitForeignIdle(t *testing.T, s *Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !s.batchActive.Load() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("foreign restore goroutine did not finish in time")
}

// settingsSnapshot returns the settings both as the struct (DeepEqual) and as
// its JSON bytes, for the byte-identical guarantee the foreign mode makes.
func settingsSnapshot(t *testing.T, s *Service) (store.Settings, string) {
	t.Helper()
	st, err := s.store.GetSettings()
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	return st, string(raw)
}

// TestForeignRestoreContainerRoundTrip pins the Task 10 container path end to
// end: StartForeignRestore reads the encrypted def from the FOREIGN repo's
// def dir and decrypts it with the SESSION key (the local APP_KEY differs and
// could never decrypt it), a local target row appears carrying that
// definition, the restore preparation lists the SESSION repo — never the
// settings containers repo — and the EXISTING execute path recreates the
// container (fake docker) and records a "restore" run against the adopted
// target. Settings stay untouched and the foreign repo sees no writes.
func TestForeignRestoreContainerRoundTrip(t *testing.T) {
	// No container:web snapshots in the repo → the plan is recreate-only (pure
	// def), which keeps the round trip OS-independent (no appdata paths on disk).
	eng := &foreignRecordingEngine{opens: opensEncrypted}
	s := newForeignTestService(t, eng)
	d := &foreignFakeDocker{}
	s.docker = d

	// The foreign repo on the mounted share: restic's config marker (so the
	// local repo counts as present for snapshot listing) + the encrypted def
	// mirror for container "web", encrypted with the FOREIGN key.
	repoDir := filepath.Join(s.cfg.HostMountRoot, "backups", "other")
	if err := os.MkdirAll(filepath.Join(repoDir, "def"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "config"), []byte("cfg"), 0o600); err != nil {
		t.Fatal(err)
	}
	defJSON, err := json.Marshal(containerDefinition{
		Inspect:      model.Inspect{Name: "web", Config: model.Config{Image: "nginx:latest"}},
		AppdataPaths: []string{"/host/user/appdata/web"},
	})
	if err != nil {
		t.Fatalf("marshal def: %v", err)
	}
	enc, err := secret.Encrypt(foreignTestKey, defJSON)
	if err != nil {
		t.Fatalf("encrypt def: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "def", "web.def"), enc, 0o600); err != nil {
		t.Fatal(err)
	}

	before, beforeRaw := settingsSnapshot(t, s)
	id, _, err := s.OpenForeign(context.Background(), "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}
	sess, err := s.foreignSession(id)
	if err != nil {
		t.Fatalf("foreignSession: %v", err)
	}

	started, err := s.StartForeignRestore(context.Background(), id, "containers", "web", "latest", true, "", nil, false)
	if err != nil || !started {
		t.Fatalf("StartForeignRestore: started=%v err=%v", started, err)
	}
	waitForeignIdle(t, s)

	// The def decrypted with the SESSION key and became a normal local target.
	tg, err := s.store.GetTargetByContainer("web")
	if err != nil {
		t.Fatalf("adopted target must exist locally: %v", err)
	}
	if !strings.Contains(tg.Definition, "nginx:latest") {
		t.Fatalf("target definition must carry the FOREIGN def (decrypted with the session key), got %q", tg.Definition)
	}
	if len(tg.AppdataPaths) != 1 || tg.AppdataPaths[0] != "/host/user/appdata/web" {
		t.Fatalf("target appdata paths must come from the foreign def, got %v", tg.AppdataPaths)
	}

	// The EXISTING restore path ran: the container was recreated from the def.
	d.mu.Lock()
	created, createdImage := d.created, d.createdIn.Config.Image
	d.mu.Unlock()
	if created != 1 || createdImage != "nginx:latest" {
		t.Fatalf("expected one CreateAndStart from the foreign def, got created=%d image=%q", created, createdImage)
	}

	// The preparation listed the SESSION repo — never the settings repo.
	eng.mu.Lock()
	listed := append([]string(nil), eng.snapshotRepos...)
	eng.mu.Unlock()
	if len(listed) == 0 {
		t.Fatal("expected the restore preparation to list snapshots")
	}
	for _, repo := range listed {
		if repo != sess.repo {
			t.Fatalf("snapshot listing hit %q, want only the session repo %q", repo, sess.repo)
		}
	}

	// A "restore" run is recorded against the adopted target.
	runs, err := s.store.ListRuns(10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	found := false
	for _, r := range runs {
		if r.TargetID == tg.ID && r.Kind == "restore" && r.Status == "success" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a successful restore run for target %s, got %+v", tg.ID, runs)
	}

	// Hard guarantees: no writes to the foreign repo, settings byte-identical.
	if bad := eng.calledForbidden(); len(bad) > 0 {
		t.Fatalf("foreign restore performed forbidden repo writes: %v (all calls: %v)", bad, eng.calls)
	}
	after, afterRaw := settingsSnapshot(t, s)
	if !reflect.DeepEqual(before, after) || beforeRaw != afterRaw {
		t.Fatalf("open+restore must not touch settings:\nbefore: %s\nafter:  %s", beforeRaw, afterRaw)
	}
}

// TestForeignRestoreFileSetUsesSessionRepo pins the files path: the restore
// runs against the SESSION repo (a remote-style location that can never be
// confused with the settings files repo), extracts the whole fileset snapshot
// into the chosen folder under the host mount, adopts the name as a LOCAL
// disabled path-less set (like DiscoverFileSets) and records a "restore" run
// against it — performing ONLY reads plus the one RestoreInclude.
// containerGuardTarget builds an in-memory container target (valid Definition +
// one appdata path) for the foreign-container mount-guard tests, so
// prepareRestoreForTarget resolves a real snapshot (not recreate-only) and reaches
// the appdata destination guard.
func containerGuardTarget(t *testing.T, appdata string) store.Target {
	t.Helper()
	defJSON, err := json.Marshal(containerDefinition{
		Inspect: model.Inspect{Name: "web", Config: model.Config{Image: "nginx:latest"}},
	})
	if err != nil {
		t.Fatalf("marshal def: %v", err)
	}
	return store.Target{ID: "t-web", ContainerName: "web", Definition: string(defJSON), AppdataPaths: []string{appdata}}
}

// mustContainerDefWithBind builds a container definition JSON carrying the given
// docker binds, so a remap test can assert only appdata binds are rewritten.
func mustContainerDefWithBind(t *testing.T, binds ...string) string {
	t.Helper()
	defJSON, err := json.Marshal(containerDefinition{
		Inspect: model.Inspect{
			Name:       "web",
			Config:     model.Config{Image: "nginx:latest"},
			HostConfig: model.HostConfig{Binds: binds},
		},
	})
	if err != nil {
		t.Fatalf("marshal def: %v", err)
	}
	return string(defJSON)
}

// TestForeignContainerRestoreGuardAbortsWhenDestPoolMissing pins the #123/#125
// container fix: a cross-instance container restore whose DESTINATION pool is NOT
// mounted on this host is REFUSED during preparation, before the caller adopts the
// target or reaches executeRestore's destructive Stop/Remove — so appdata can
// never be written to an unmounted path (the RAM rootfs).
func TestForeignContainerRestoreGuardAbortsWhenDestPoolMissing(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{ID: "abcdef0123456789", Tags: []string{"container:web"}}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	// Only "/" and the broad host bind are mounted — the chosen destination pool is NOT.
	writeMountFixture(t, "/", "/host/user")

	tg := containerGuardTarget(t, "/host/user/appdata/web")
	// destBase points at a pool this host lacks -> dest /host/user/zfs/appdata/web.
	_, err := s.prepareRestoreForTarget(context.Background(), repoRef{repo: repoDir}, "web", "latest", tg, "/host/user/zfs/appdata", false)
	if err == nil || !strings.Contains(err.Error(), "not on a mounted") {
		t.Fatalf("want the destination not-mounted refusal, got %v", err)
	}
	if len(eng.restores) != 0 {
		t.Fatalf("a refused foreign container restore must not restore anything, got %v", eng.restores)
	}
}

// TestForeignContainerRestoreGuardAllowsMountedDest is the counterpart: when the
// destination sits on a genuinely mounted pool/share (the standard
// /host/user/appdata shfs case) the guard passes, a plan is built with the remap
// restoreDirs, and preparation itself performs no restic restore.
func TestForeignContainerRestoreGuardAllowsMountedDest(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{ID: "abcdef0123456789", Tags: []string{"container:web"}}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	writeMountFixture(t, "/", "/host/user", "/host/user/appdata")

	tg := containerGuardTarget(t, "/host/user/appdata/web")
	plan, err := s.prepareRestoreForTarget(context.Background(), repoRef{repo: repoDir}, "web", "latest", tg, "/host/user/appdata", false)
	if err != nil {
		t.Fatalf("mounted destination must pass the guard, got %v", err)
	}
	// The remap restoreDirs drive the restore; here dest == source (no-op remap).
	if len(plan.restoreDirs) != 1 || plan.restoreDirs[0].Target != "/host/user/appdata/web" || plan.restoreDirs[0].Subtree != "/host/user/appdata/web" {
		t.Fatalf("unexpected restoreDirs: %+v", plan.restoreDirs)
	}
	if len(eng.restores) != 0 {
		t.Fatalf("preparation must not restore, got %v", eng.restores)
	}
}

// TestForeignContainerRestoreRemapsCrossPool pins the actual #125 remap: a
// container backed up on a pool this host lacks (/host/user/zfs/appdata/web) is
// remapped to the chosen destination (/host/user/appdata), so restoreDirs restore
// the source subtree's CONTENTS into the dest and the recreated binds point there.
func TestForeignContainerRestoreRemapsCrossPool(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{ID: "abcdef0123456789", Tags: []string{"container:web"}}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	// Destination = the conventional local appdata share (container path
	// /host/user/user/appdata maps to host /mnt/user/appdata).
	writeMountFixture(t, "/", "/host/user", "/host/user/user/appdata")

	tg := containerGuardTarget(t, "/host/user/zfs/appdata/web") // source pool = zfs (absent here)
	tg.Definition = mustContainerDefWithBind(t, "/mnt/zfs/appdata/web:/config", "/var/run/docker.sock:/var/run/docker.sock")
	plan, err := s.prepareRestoreForTarget(context.Background(), repoRef{repo: repoDir}, "web", "latest", tg, "/host/user/user/appdata", false)
	if err != nil {
		t.Fatalf("cross-pool remap must succeed to a mounted dest, got %v", err)
	}
	if len(plan.restoreDirs) != 1 ||
		plan.restoreDirs[0].Subtree != "/host/user/zfs/appdata/web" ||
		plan.restoreDirs[0].Target != "/host/user/user/appdata/web" {
		t.Fatalf("remap should restore zfs subtree INTO the appdata dest, got %+v", plan.restoreDirs)
	}
	// The appdata bind is repointed to the dest host path; docker.sock is untouched.
	joined := strings.Join(plan.inspect.HostConfig.Binds, " ")
	if !strings.Contains(joined, "/mnt/user/appdata/web:/config") {
		t.Fatalf("appdata bind must be remapped to the dest host path, got %v", plan.inspect.HostConfig.Binds)
	}
	if !strings.Contains(joined, "/var/run/docker.sock:/var/run/docker.sock") {
		t.Fatalf("docker.sock bind must be left verbatim, got %v", plan.inspect.HostConfig.Binds)
	}
}

// TestForeignContainerRestoreRewritesNonCanonicalBind pins the review fix: a bind
// host with a trailing slash (as a user may enter it, differing from the
// normalized appdata path) is still canonicalized and remapped to the dest — no
// silent half-remap where the data moves but the bind keeps pointing at the
// absent source pool.
func TestForeignContainerRestoreRewritesNonCanonicalBind(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{ID: "abcdef0123456789", Tags: []string{"container:web"}}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	writeMountFixture(t, "/", "/host/user", "/host/user/user/appdata")

	tg := containerGuardTarget(t, "/host/user/zfs/appdata/web")
	tg.Definition = mustContainerDefWithBind(t, "/mnt/zfs/appdata/web/:/config") // trailing slash
	plan, err := s.prepareRestoreForTarget(context.Background(), repoRef{repo: repoDir}, "web", "latest", tg, "/host/user/user/appdata", false)
	if err != nil {
		t.Fatalf("remap: %v", err)
	}
	if !strings.Contains(strings.Join(plan.inspect.HostConfig.Binds, " "), "/mnt/user/appdata/web:/config") {
		t.Fatalf("non-canonical bind host must still be remapped, got %v", plan.inspect.HostConfig.Binds)
	}
}

// TestForeignContainerRestoreOverwriteGuard: a destination that is NOT this
// container's own source path and already holds data is refused unless overwrite
// is confirmed.
func TestForeignContainerRestoreOverwriteGuard(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{ID: "abcdef0123456789", Tags: []string{"container:web"}}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	writeMountFixture(t, "/", "/host/user", "/host/user/user/appdata")
	// Injected probe: the destination dir already holds data (a different
	// container's appdata) — OS-independent, no real filesystem needed.
	dest := "/host/user/user/appdata/web"
	s.dirNonEmptyProbe = func(p string) bool { return p == dest }

	tg := containerGuardTarget(t, "/host/user/zfs/appdata/web") // source != dest
	_, err := s.prepareRestoreForTarget(context.Background(), repoRef{repo: repoDir}, "web", "latest", tg, "/host/user/user/appdata", false)
	if err == nil || !strings.Contains(err.Error(), "already contains data") {
		t.Fatalf("non-empty foreign dest must be refused without overwrite, got %v", err)
	}
	// With overwrite confirmed it proceeds (guard + remap already validated).
	if _, err := s.prepareRestoreForTarget(context.Background(), repoRef{repo: repoDir}, "web", "latest", tg, "/host/user/user/appdata", true); err != nil {
		t.Fatalf("overwrite=true must proceed, got %v", err)
	}
}

// TestLocalContainerRestoreSkipsRemap proves the remap is foreign-only: a
// same-instance restore (destBase="") keeps its historical in-place behaviour —
// no restoreDirs, appdataPaths intact — even when a pool is unmounted.
func TestLocalContainerRestoreSkipsRemap(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{ID: "abcdef0123456789", Tags: []string{"container:web"}}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	writeMountFixture(t, "/", "/host/user") // appdata NOT mounted

	tg := containerGuardTarget(t, "/host/user/appdata/web")
	plan, err := s.prepareRestoreForTarget(context.Background(), repoRef{repo: repoDir}, "web", "latest", tg, "", false)
	if err != nil {
		t.Fatalf("local restore must not gain the remap/guard, got %v", err)
	}
	if len(plan.restoreDirs) != 0 || len(plan.appdataPaths) != 1 {
		t.Fatalf("local plan must be in-place (no restoreDirs), got %+v", plan)
	}
}

func TestForeignRestoreFileSetUsesSessionRepo(t *testing.T) {
	location := "backups/other" // a LOCAL mounted share — remote backends are rejected
	eng := &foreignRecordingEngine{
		opens: opensEncrypted,
		snaps: []restic.Snapshot{
			{ID: "eeeeeeee55555555", Time: "2026-07-05T10:00:00Z", Tags: []string{"fileset:docs"}},
			{ID: "ffffffff66666666", Time: "2026-07-06T10:00:00Z", Tags: []string{"fileset:docs"}},
		},
	}
	s := newForeignTestService(t, eng)
	sessionRepo := seedForeignRepoMarker(t, s, location)

	id, inv, err := s.OpenForeign(context.Background(), location, foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}
	if len(inv.FileSets) != 1 || inv.FileSets[0].Name != "docs" {
		t.Fatalf("inventory fileSets = %+v, want [docs]", inv.FileSets)
	}

	started, err := s.StartForeignRestore(context.Background(), id, "files", "docs", "latest", true, "restore-here/docs", nil, false)
	if err != nil || !started {
		t.Fatalf("StartForeignRestore: started=%v err=%v", started, err)
	}
	waitForeignIdle(t, s)

	// Exactly one whole-tree RestoreInclude, from the SESSION repo, of the
	// NEWEST snapshot ("latest"), into the resolved folder under the mount.
	wantTarget, err := paths.Resolve(s.cfg.HostMountRoot, "restore-here/docs")
	if err != nil {
		t.Fatalf("resolve want target: %v", err)
	}
	eng.mu.Lock()
	restores := append([]string(nil), eng.restores...)
	eng.mu.Unlock()
	want := "RestoreInclude|" + sessionRepo + "|ffffffff66666666|/->" + wantTarget
	if len(restores) != 1 || restores[0] != want {
		t.Fatalf("restore calls = %v, want exactly [%s]", restores, want)
	}
	if _, err := os.Stat(wantTarget); err != nil {
		t.Fatalf("target folder must be created under the host mount: %v", err)
	}
	// Read-only guarantee (#61): the foreign session took no repository lock.
	if !eng.everyModeNoLock() {
		t.Fatalf("every foreign read/restore must be lock-free (NoLock), got modes %+v", eng.modes)
	}
	if eng.anyModeHasEnv() {
		t.Fatalf("a local-only foreign session must not attach cloud credentials, got modes %+v", eng.modes)
	}

	// The name was adopted as a LOCAL disabled, path-less set and the run is
	// attributable to it.
	set, err := s.store.GetFileSetByName("docs")
	if err != nil {
		t.Fatalf("adopted file set must exist locally: %v", err)
	}
	if set.Enabled || set.Path != "" {
		t.Fatalf("adopted set must be disabled and path-less, got %+v", set)
	}
	runs, err := s.store.ListRuns(10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	found := false
	for _, r := range runs {
		if r.TargetID == set.ID && r.Kind == "restore" && r.Status == "success" && r.SnapshotID == "ffffffff66666666" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a successful restore run for set %s, got %+v", set.ID, runs)
	}

	// Read-only guarantee: only reads + the one RestoreInclude — never a write.
	if bad := eng.calledForbidden(); len(bad) > 0 {
		t.Fatalf("foreign restore performed forbidden repo writes: %v (all calls: %v)", bad, eng.calls)
	}
	if eng.count("RestorePath") != 0 {
		t.Fatalf("a foreign file-set restore must use RestoreInclude only, got calls %v", eng.calls)
	}
}

// TestForeignRestoreFileSetSelectiveUsesSubtreeInclude pins the #123 selective
// path: when StartForeignRestore is given a non-empty selection, only the chosen
// subfolders are restored — each via RestoreSubtreeInclude rooted at the
// snapshot's own backed-up path with the selection RELATIVE to that subtree, so
// its contents land at <target>/<rel> (no /host/user/… nesting, issue #62), read
// from the SESSION repo, lock-free, and never a repo write.
func TestForeignRestoreFileSetSelectiveUsesSubtreeInclude(t *testing.T) {
	location := "backups/other"
	eng := &foreignRecordingEngine{
		opens: opensEncrypted,
		snaps: []restic.Snapshot{
			{ID: "aaaaaaaa11111111", Time: "2026-07-05T10:00:00Z", Paths: []string{"/host/user/appdata"}, Tags: []string{"fileset:appdata"}},
			{ID: "bbbbbbbb22222222", Time: "2026-07-06T10:00:00Z", Paths: []string{"/host/user/appdata"}, Tags: []string{"fileset:appdata"}},
		},
	}
	s := newForeignTestService(t, eng)
	sessionRepo := seedForeignRepoMarker(t, s, location)

	id, _, err := s.OpenForeign(context.Background(), location, foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}

	sel := []string{"/host/user/appdata/vaultwarden", "/host/user/appdata/immich"}
	started, err := s.StartForeignRestore(context.Background(), id, "files", "appdata", "latest", true, "restore-here/subset", sel, false)
	if err != nil || !started {
		t.Fatalf("StartForeignRestore (selective): started=%v err=%v", started, err)
	}
	waitForeignIdle(t, s)

	wantTarget, err := paths.Resolve(s.cfg.HostMountRoot, "restore-here/subset")
	if err != nil {
		t.Fatalf("resolve want target: %v", err)
	}
	eng.mu.Lock()
	restores := append([]string(nil), eng.restores...)
	eng.mu.Unlock()
	// One RestoreSubtreeInclude per selected path, in selection order, from the
	// SESSION repo, of the NEWEST snapshot, subtree = the snapshot's backed-up
	// root, include = the selection RELATIVE to that subtree.
	want := []string{
		"RestoreSubtreeInclude|" + sessionRepo + "|bbbbbbbb22222222|/host/user/appdata|/vaultwarden->" + wantTarget,
		"RestoreSubtreeInclude|" + sessionRepo + "|bbbbbbbb22222222|/host/user/appdata|/immich->" + wantTarget,
	}
	if !reflect.DeepEqual(restores, want) {
		t.Fatalf("selective restore calls = %v, want %v", restores, want)
	}
	if _, err := os.Stat(wantTarget); err != nil {
		t.Fatalf("target folder must be created under the host mount: %v", err)
	}
	// A selective restore uses RestoreSubtreeInclude only — never the whole-set
	// RestoreInclude/RestoreSubtreeTo, and never the in-place RestorePath.
	if eng.count("RestoreInclude") != 0 || eng.count("RestoreSubtreeTo") != 0 || eng.count("RestorePath") != 0 {
		t.Fatalf("selective restore must use RestoreSubtreeInclude only, got calls %v", eng.calls)
	}
	// Read-only guarantee (#61): the foreign session took no repository lock and
	// attached no cloud credentials, and performed no forbidden repo writes.
	if !eng.everyModeNoLock() {
		t.Fatalf("every foreign read/restore must be lock-free (NoLock), got modes %+v", eng.modes)
	}
	if eng.anyModeHasEnv() {
		t.Fatalf("a local-only foreign session must not attach cloud credentials, got modes %+v", eng.modes)
	}
	if bad := eng.calledForbidden(); len(bad) > 0 {
		t.Fatalf("selective foreign restore performed forbidden repo writes: %v (all calls: %v)", bad, eng.calls)
	}

	// The set was adopted locally (disabled, path-less) and the run is attributable.
	set, err := s.store.GetFileSetByName("appdata")
	if err != nil {
		t.Fatalf("adopted file set must exist locally: %v", err)
	}
	if set.Enabled || set.Path != "" {
		t.Fatalf("adopted set must be disabled and path-less, got %+v", set)
	}
	runs, err := s.store.ListRuns(10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	found := false
	for _, r := range runs {
		if r.TargetID == set.ID && r.Kind == "restore" && r.Status == "success" && r.SnapshotID == "bbbbbbbb22222222" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a successful selective restore run for set %s, got %+v", set.ID, runs)
	}
}

// TestForeignRestoreFileSetSelectiveGuards pins the selective path's synchronous
// guards: a selection outside the snapshot's backed-up subtree is a traversal
// attempt and is rejected before any engine restore; a selection with no target
// folder is rejected (foreign restores are never in place); and neither leaks the
// single-flight guard.
func TestForeignRestoreFileSetSelectiveGuards(t *testing.T) {
	location := "backups/other"
	eng := &foreignRecordingEngine{
		opens: opensEncrypted,
		snaps: []restic.Snapshot{
			{ID: "aaaaaaaa11111111", Time: "2026-07-05T10:00:00Z", Paths: []string{"/host/user/appdata"}, Tags: []string{"fileset:appdata"}},
		},
	}
	s := newForeignTestService(t, eng)
	seedForeignRepoMarker(t, s, location)
	ctx := context.Background()

	id, _, err := s.OpenForeign(ctx, location, foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}

	// A selected path outside the snapshot's subtree is a traversal attempt.
	started, err := s.StartForeignRestore(ctx, id, "files", "appdata", "latest", true, "restore-here/x", []string{"/host/other/secret"}, false)
	if started || err == nil || !strings.Contains(err.Error(), "outside the file set snapshot") {
		t.Fatalf("outside-subtree selection: want the containment error, got started=%v err=%v", started, err)
	}
	// A selection with no target folder: foreign restores never go in place.
	started, err = s.StartForeignRestore(ctx, id, "files", "appdata", "latest", true, "", []string{"/host/user/appdata/vaultwarden"}, false)
	if started || err == nil || !strings.Contains(err.Error(), "target folder") {
		t.Fatalf("selection without target: want the target-folder error, got started=%v err=%v", started, err)
	}
	// No restore write was performed and the single-flight guard is free.
	if len(eng.restores) != 0 {
		t.Fatalf("a rejected selective restore must not touch the engine restores, got %v", eng.restores)
	}
	if s.BackupInProgress() {
		t.Fatal("a rejected selective restore must release the single-flight guard")
	}
}

// TestForeignRestoreFileSetSelectiveDeepSelectionNoTree pins the #123 fix
// (manilx, 2026-08-07): a DEEPLY nested selection must land directly as
// <target>/<name>, not recreate the whole path between the file-set root and the
// selection. The restore roots at the selection's IMMEDIATE PARENT (not the set
// root), so restic drops just the picked folder into the target.
func TestForeignRestoreFileSetSelectiveDeepSelectionNoTree(t *testing.T) {
	location := "backups/other"
	eng := &foreignRecordingEngine{
		opens: opensEncrypted,
		snaps: []restic.Snapshot{
			// File-set root is /host/user/appdata; the picked stack is nested deep.
			{ID: "aaaaaaaa11111111", Time: "2026-07-05T10:00:00Z", Paths: []string{"/host/user/appdata"}, Tags: []string{"fileset:appdata"}},
		},
	}
	s := newForeignTestService(t, eng)
	sessionRepo := seedForeignRepoMarker(t, s, location)
	id, _, err := s.OpenForeign(context.Background(), location, foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}

	deep := "/host/user/appdata/stacks/DXP480T/xo"
	started, err := s.StartForeignRestore(context.Background(), id, "files", "appdata", "latest", true, "restore-here", []string{deep}, false)
	if err != nil || !started {
		t.Fatalf("StartForeignRestore (deep selective): started=%v err=%v", started, err)
	}
	waitForeignIdle(t, s)

	wantTarget, err := paths.Resolve(s.cfg.HostMountRoot, "restore-here")
	if err != nil {
		t.Fatalf("resolve want target: %v", err)
	}
	eng.mu.Lock()
	restores := append([]string(nil), eng.restores...)
	eng.mu.Unlock()
	// Root = the selection's IMMEDIATE PARENT (/host/user/appdata/stacks/DXP480T),
	// include = /xo -> the xo folder lands at <target>/xo, NOT <target>/stacks/DXP480T/xo.
	want := "RestoreSubtreeInclude|" + sessionRepo + "|aaaaaaaa11111111|/host/user/appdata/stacks/DXP480T|/xo->" + wantTarget
	if len(restores) != 1 || restores[0] != want {
		t.Fatalf("deep selection must root at the parent (no intermediate tree):\n got %v\nwant [%s]", restores, want)
	}
}

// TestListForeignFiles pins the foreign subfolder picker's read: ListForeignFiles
// returns the session snapshot's file tree via a lock-free Ls on the SESSION repo,
// and rejects a non-files domain, an unsafe item name and an unknown session.
func TestListForeignFiles(t *testing.T) {
	location := "backups/other"
	entries := []restic.FileEntry{
		{Path: "/host/user/appdata", Type: "dir"},
		{Path: "/host/user/appdata/vaultwarden", Type: "dir"},
		{Path: "/host/user/appdata/vaultwarden/db.sqlite3", Type: "file", Size: 4096},
	}
	eng := &foreignRecordingEngine{
		opens:     opensEncrypted,
		lsEntries: entries,
		snaps: []restic.Snapshot{
			{ID: "aaaaaaaa11111111", Time: "2026-07-05T10:00:00Z", Paths: []string{"/host/user/appdata"}, Tags: []string{"fileset:appdata"}},
		},
	}
	s := newForeignTestService(t, eng)
	seedForeignRepoMarker(t, s, location)
	ctx := context.Background()

	id, _, err := s.OpenForeign(ctx, location, foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}

	files, err := s.ListForeignFiles(ctx, id, "files", "appdata", "latest")
	if err != nil {
		t.Fatalf("ListForeignFiles: %v", err)
	}
	if !reflect.DeepEqual(files, entries) {
		t.Fatalf("ListForeignFiles returned %+v, want %+v", files, entries)
	}
	if eng.count("Ls") != 1 {
		t.Fatalf("expected exactly one Ls, got calls %v", eng.calls)
	}
	// The listing is read-only (#61): every recorded mode carried NoLock.
	if !eng.everyModeNoLock() {
		t.Fatalf("foreign file listing must be lock-free (NoLock), got modes %+v", eng.modes)
	}
	if bad := eng.calledForbidden(); len(bad) > 0 {
		t.Fatalf("foreign file listing performed forbidden repo writes: %v", bad)
	}

	// A non-files domain is rejected (containers/vms have no file tree here).
	if _, err := s.ListForeignFiles(ctx, id, "containers", "appdata", "latest"); err == nil || !strings.Contains(err.Error(), "files domain") {
		t.Fatalf("non-files domain: want the domain error, got %v", err)
	}
	// An unsafe item name is rejected before any engine call.
	if _, err := s.ListForeignFiles(ctx, id, "files", "../evil", "latest"); err == nil || !strings.Contains(err.Error(), "invalid item name") {
		t.Fatalf("unsafe item: want the name error, got %v", err)
	}
	// An unknown session is rejected.
	if _, err := s.ListForeignFiles(ctx, "nope", "files", "appdata", "latest"); !errors.Is(err, errForeignSession) {
		t.Fatalf("unknown session: want errForeignSession, got %v", err)
	}
}

// TestForeignRestoreLeavesSettingsUntouched pins guarantee #1 across the FULL
// flow: after open + restore the settings read back deeply equal AND
// byte-identical — the foreign path must never take the attach flow's
// UpdateSettings route.
func TestForeignRestoreLeavesSettingsUntouched(t *testing.T) {
	eng := &foreignRecordingEngine{
		opens: opensEncrypted,
		snaps: []restic.Snapshot{{ID: "eeeeeeee55555555", Time: "2026-07-05T10:00:00Z", Tags: []string{"fileset:docs"}}},
	}
	s := newForeignTestService(t, eng)
	seedForeignRepoMarker(t, s, "backups/other")

	before, beforeRaw := settingsSnapshot(t, s)
	id, _, err := s.OpenForeign(context.Background(), "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}
	started, err := s.StartForeignRestore(context.Background(), id, "files", "docs", "latest", true, "restore-here/docs", nil, false)
	if err != nil || !started {
		t.Fatalf("StartForeignRestore: started=%v err=%v", started, err)
	}
	waitForeignIdle(t, s)

	after, afterRaw := settingsSnapshot(t, s)
	if !reflect.DeepEqual(before, after) || beforeRaw != afterRaw {
		t.Fatalf("open+restore must leave settings byte-identical:\nbefore: %s\nafter:  %s", beforeRaw, afterRaw)
	}
}

// TestForeignRestoreValidation pins the synchronous guards: an unconfirmed
// restore fails with the familiar sentinel BEFORE anything runs, an unknown
// (or expired) session errors, an unknown domain / a file set without a
// target folder / an unsafe item name / a container without a readable def
// all fail cleanly — and none of them leak the shared single-flight guard,
// proven by a valid restore starting afterwards.
func TestForeignRestoreValidation(t *testing.T) {
	eng := &foreignRecordingEngine{
		opens: opensEncrypted,
		snaps: []restic.Snapshot{{ID: "eeeeeeee55555555", Time: "2026-07-05T10:00:00Z", Tags: []string{"fileset:docs"}}},
	}
	s := newForeignTestService(t, eng)
	seedForeignRepoMarker(t, s, "backups/other")
	ctx := context.Background()

	// Unconfirmed: the sentinel, before ANY engine call or session lookup.
	started, err := s.StartForeignRestore(ctx, "whatever", "containers", "web", "latest", false, "", nil, false)
	if started || !errors.Is(err, backup.ErrNotConfirmed) {
		t.Fatalf("unconfirmed: want ErrNotConfirmed, got started=%v err=%v", started, err)
	}
	if len(eng.calls) != 0 {
		t.Fatalf("unconfirmed restore must not touch the engine, got %v", eng.calls)
	}

	// Unknown session (also the expired case — foreignSession sweeps first).
	started, err = s.StartForeignRestore(ctx, "nope", "containers", "web", "latest", true, "", nil, false)
	if started || !errors.Is(err, errForeignSession) {
		t.Fatalf("unknown session: want errForeignSession, got started=%v err=%v", started, err)
	}

	id, _, err := s.OpenForeign(ctx, "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}

	// Unknown domain.
	if started, err = s.StartForeignRestore(ctx, id, "flash", "boot", "latest", true, "", nil, false); started || err == nil || !strings.Contains(err.Error(), "unknown domain") {
		t.Fatalf("unknown domain: want the domain error, got started=%v err=%v", started, err)
	}
	// File set without a target folder (foreign sets never restore in place).
	if started, err = s.StartForeignRestore(ctx, id, "files", "docs", "latest", true, "", nil, false); started || err == nil || !strings.Contains(err.Error(), "target folder") {
		t.Fatalf("files without target: want the target-folder error, got started=%v err=%v", started, err)
	}
	// Unsafe item name (feeds tags, def filenames and progress keys).
	if started, err = s.StartForeignRestore(ctx, id, "files", "../evil", "latest", true, "restore-here", nil, false); started || err == nil || !strings.Contains(err.Error(), "invalid item name") {
		t.Fatalf("unsafe item: want the name error, got started=%v err=%v", started, err)
	}
	// Container whose def the foreign repo does not mirror (the seeded repo has a
	// config marker but no def/ghost.def to read).
	if started, err = s.StartForeignRestore(ctx, id, "containers", "ghost", "latest", true, "", nil, false); started || err == nil || !strings.Contains(err.Error(), "definition") {
		t.Fatalf("missing def: want the definition error, got started=%v err=%v", started, err)
	}

	// None of the failures may leak the single-flight guard: a valid restore
	// still starts (and finishes) afterwards.
	if s.BackupInProgress() {
		t.Fatal("a failed foreign restore must release the single-flight guard")
	}
	started, err = s.StartForeignRestore(ctx, id, "files", "docs", "latest", true, "restore-here/docs", nil, false)
	if err != nil || !started {
		t.Fatalf("valid restore after failures: started=%v err=%v", started, err)
	}
	waitForeignIdle(t, s)
}

// TestForeignRestoreVMNameIsLibvirtAware pins #122: a foreign VM restore
// validates the item name with the libvirt-aware validVMName, so a VM whose
// name contains a space ("Windows Server 2022") is NOT rejected as "invalid
// item name" — it proceeds past the name check (and only then fails because the
// seeded repo mirrors no def for it). An unsafe VM name is still rejected, and
// containers / file sets keep the strict validResourceName (spaces rejected).
func TestForeignRestoreVMNameIsLibvirtAware(t *testing.T) {
	eng := &foreignRecordingEngine{
		opens: opensEncrypted,
		snaps: []restic.Snapshot{{ID: "dddddddd44444444", Time: "2026-07-04T10:00:00Z", Tags: []string{"vm:win"}}},
	}
	s := newForeignTestService(t, eng)
	seedForeignRepoMarker(t, s, "backups/other")
	ctx := context.Background()

	id, _, err := s.OpenForeign(ctx, "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}

	// A VM name with a space passes the name check: the failure is the later
	// missing-definition error, never "invalid item name".
	started, err := s.StartForeignRestore(ctx, id, "vms", "Windows Server 2022", "latest", true, "", nil, false)
	if started {
		t.Fatalf("no def seeded, restore must not start; got started=%v", started)
	}
	if err == nil || strings.Contains(err.Error(), "invalid item name") {
		t.Fatalf("spaced VM name must pass the name check (fail later on the def), got %v", err)
	}
	if !strings.Contains(err.Error(), "definition") {
		t.Fatalf("expected the missing-definition error, got %v", err)
	}

	// Unsafe VM names are still rejected by the name check itself.
	for _, unsafe := range []string{"../evil", "win/../etc", `win\evil`, "-oProxyCommand"} {
		started, err = s.StartForeignRestore(ctx, id, "vms", unsafe, "latest", true, "", nil, false)
		if started || err == nil || !strings.Contains(err.Error(), "invalid item name") {
			t.Fatalf("unsafe VM name %q: want the name rejection, got started=%v err=%v", unsafe, started, err)
		}
	}

	// Containers and file sets keep the strict validResourceName: a space is not
	// a valid Docker container / file-set name, so it is rejected on the name.
	for _, domain := range []string{"containers", "files"} {
		started, err = s.StartForeignRestore(ctx, id, domain, "has space", "latest", true, "restore-here", nil, false)
		if started || err == nil || !strings.Contains(err.Error(), "invalid item name") {
			t.Fatalf("%s spaced name: want the strict name rejection, got started=%v err=%v", domain, started, err)
		}
	}
}

// TestOpenForeignRejectsRemoteLocation pins the confused-deputy fix (#61): a
// remote-backend location (or an unprefixed rclone remote name) is rejected
// BEFORE any engine call, so restic never contacts a third-party server carrying
// THIS instance's off-site credentials. Only a locally mounted path is allowed.
func TestOpenForeignRejectsRemoteLocation(t *testing.T) {
	remote := []string{
		"rest:http://10.0.0.9:8000/repo",
		"s3:s3.amazonaws.com/bucket",
		"sftp:user@host:/srv/repo",
		"rclone:remote:path",
		"b2:bucket/path",
		"gs:bucket/path",
		"azure:container/path",
		"BackBlaze:bucket", // unprefixed rclone remote name (the common typo)
	}
	for _, loc := range remote {
		eng := &foreignRecordingEngine{opens: opensEncrypted}
		s := newForeignTestService(t, eng)
		_, _, err := s.OpenForeign(context.Background(), loc, foreignTestKey)
		if err == nil || !strings.Contains(err.Error(), "locally mounted") {
			t.Fatalf("location %q: want the locally-mounted rejection, got %v", loc, err)
		}
		if len(eng.calls) != 0 {
			t.Fatalf("location %q: rejection must precede any engine call (cloudEnv never used), got %v", loc, eng.calls)
		}
	}
}

// TestForeignJanitorSweepsExpiredSessions pins #61's background janitor: an
// abandoned (expired) session — and the foreign APP_KEY it holds — is swept
// WITHOUT any intervening foreign API call. The sweep interval is injected fast;
// a direct guarded map read observes ONLY the janitor's effect (foreignSession()
// would itself sweep on access and mask it).
func TestForeignJanitorSweepsExpiredSessions(t *testing.T) {
	eng := &foreignRecordingEngine{opens: opensEncrypted}
	s := newForeignTestService(t, eng)
	s.foreignSweepEvery = 5 * time.Millisecond // fast tick for the test

	id, _, err := s.OpenForeign(context.Background(), "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}

	s.foreignMu.Lock()
	sess := s.foreignSessions[id]
	sess.expires = time.Now().Add(-time.Second)
	s.foreignSessions[id] = sess
	s.foreignMu.Unlock()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.foreignMu.Lock()
		_, still := s.foreignSessions[id]
		s.foreignMu.Unlock()
		if !still {
			return // the janitor swept it with no API call in between
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("the background janitor did not sweep the expired session")
}

// TestForeignRestoreValidationFailureLeavesLocalTargetIntact pins #61's
// validate-before-adopt fix: a foreign container restore that fails validation
// (here: an explicit snapshot id that does not belong to the item) must NOT have
// adopted the foreign recipe — an existing same-named local target keeps its own
// definition + appdata_paths BYTE-IDENTICAL.
func TestForeignRestoreValidationFailureLeavesLocalTargetIntact(t *testing.T) {
	// The foreign repo lists NO container:web snapshots, so any explicit id is
	// "not owned" and the restore fails during validation.
	eng := &foreignRecordingEngine{opens: opensEncrypted}
	s := newForeignTestService(t, eng)
	repoDir := seedForeignRepoMarker(t, s, "backups/other")

	// A pre-existing LOCAL target "web" with its own recipe.
	localDef, err := json.Marshal(containerDefinition{
		Inspect:      model.Inspect{Name: "web", Config: model.Config{Image: "local-image:1"}},
		AppdataPaths: []string{"/host/user/appdata/web-LOCAL"},
	})
	if err != nil {
		t.Fatalf("marshal local def: %v", err)
	}
	if _, err := s.store.UpsertTarget(store.Target{
		ContainerName: "web",
		AppdataPaths:  []string{"/host/user/appdata/web-LOCAL"},
		Definition:    string(localDef),
	}); err != nil {
		t.Fatalf("seed local target: %v", err)
	}
	before, err := s.store.GetTargetByContainer("web")
	if err != nil {
		t.Fatalf("read local target: %v", err)
	}

	// The FOREIGN repo's def mirror for "web" carries a DIFFERENT recipe — the one
	// that would clobber the local row if adoption happened before validation.
	foreignDef, err := json.Marshal(containerDefinition{
		Inspect:      model.Inspect{Name: "web", Config: model.Config{Image: "foreign-image:9"}},
		AppdataPaths: []string{"/host/user/appdata/web-FOREIGN"},
	})
	if err != nil {
		t.Fatalf("marshal foreign def: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "def"), 0o750); err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(foreignTestKey, foreignDef)
	if err != nil {
		t.Fatalf("encrypt foreign def: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "def", "web.def"), enc, 0o600); err != nil {
		t.Fatal(err)
	}

	id, _, err := s.OpenForeign(context.Background(), "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}

	// A valid-shaped but not-owned snapshot id → validation fails synchronously,
	// before any adoption.
	notOwned := strings.Repeat("ab", 8) // 16 lowercase hex = a well-formed short id
	started, err := s.StartForeignRestore(context.Background(), id, "containers", "web", notOwned, true, "", nil, false)
	if started || err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("want a not-owned-snapshot failure that starts nothing, got started=%v err=%v", started, err)
	}

	// The local target is byte-identical: the foreign recipe was never adopted.
	after, err := s.store.GetTargetByContainer("web")
	if err != nil {
		t.Fatalf("read local target after: %v", err)
	}
	if after.Definition != before.Definition {
		t.Fatalf("definition must be byte-identical:\nbefore: %s\nafter:  %s", before.Definition, after.Definition)
	}
	if !reflect.DeepEqual(after.AppdataPaths, before.AppdataPaths) {
		t.Fatalf("appdata paths must be byte-identical: before=%v after=%v", before.AppdataPaths, after.AppdataPaths)
	}
	if strings.Contains(after.Definition, "foreign-image") {
		t.Fatalf("local target must NOT hold the foreign recipe, got %s", after.Definition)
	}
}
