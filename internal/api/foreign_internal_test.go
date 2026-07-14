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

	mu            sync.Mutex
	calls         []string
	snapshotRepos []string // repo argument of every Snapshots call (which repo was listed)
	restores      []string // "Method|repo|snapshot|path" of every RestorePath/RestoreInclude call
}

func (f *foreignRecordingEngine) record(name string) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	f.mu.Unlock()
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
	return f.opens != nil && f.opens(m)
}

func (f *foreignRecordingEngine) Snapshots(_ context.Context, repo string, _ restic.Mode) ([]restic.Snapshot, error) {
	f.record("Snapshots")
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
func (f *foreignRecordingEngine) RestorePath(_ context.Context, repo, snapshotID, path string, _ restic.Mode) error {
	f.record("RestorePath")
	f.mu.Lock()
	f.restores = append(f.restores, "RestorePath|"+repo+"|"+snapshotID+"|"+path)
	f.mu.Unlock()
	return nil
}

func (f *foreignRecordingEngine) RestoreInclude(_ context.Context, repo, snapshotID, includePath, target string, _ restic.Mode) error {
	f.record("RestoreInclude")
	f.mu.Lock()
	f.restores = append(f.restores, "RestoreInclude|"+repo+"|"+snapshotID+"|"+includePath+"->"+target)
	f.mu.Unlock()
	return nil
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

func (f *foreignRecordingEngine) ForgetPolicy(_ context.Context, _ string, _ restic.RetentionPolicy, _ restic.Mode) error {
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
	return &Service{
		store:  st,
		engine: eng,
		cfg:    config.Config{HostMountRoot: t.TempDir(), AppKey: strings.Repeat("a", 64)},
	}
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
func (f *foreignFakeDocker) Remove(context.Context, string) error            { return nil }
func (f *foreignFakeDocker) Pull(context.Context, string) error              { return nil }
func (f *foreignFakeDocker) ImageID(context.Context, string) (string, error) { return "", nil }
func (f *foreignFakeDocker) ImageRemove(context.Context, string) error       { return nil }

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

	started, err := s.StartForeignRestore(context.Background(), id, "containers", "web", "latest", true, "")
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
func TestForeignRestoreFileSetUsesSessionRepo(t *testing.T) {
	location := "rest:http://127.0.0.1:9999/other" // remote → used verbatim as the repo
	eng := &foreignRecordingEngine{
		opens: opensEncrypted,
		snaps: []restic.Snapshot{
			{ID: "eeeeeeee55555555", Time: "2026-07-05T10:00:00Z", Tags: []string{"fileset:docs"}},
			{ID: "ffffffff66666666", Time: "2026-07-06T10:00:00Z", Tags: []string{"fileset:docs"}},
		},
	}
	s := newForeignTestService(t, eng)

	id, inv, err := s.OpenForeign(context.Background(), location, foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}
	if len(inv.FileSets) != 1 || inv.FileSets[0].Name != "docs" {
		t.Fatalf("inventory fileSets = %+v, want [docs]", inv.FileSets)
	}

	started, err := s.StartForeignRestore(context.Background(), id, "files", "docs", "latest", true, "restore-here/docs")
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
	want := "RestoreInclude|" + location + "|ffffffff66666666|/->" + wantTarget
	if len(restores) != 1 || restores[0] != want {
		t.Fatalf("restore calls = %v, want exactly [%s]", restores, want)
	}
	if _, err := os.Stat(wantTarget); err != nil {
		t.Fatalf("target folder must be created under the host mount: %v", err)
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

	before, beforeRaw := settingsSnapshot(t, s)
	id, _, err := s.OpenForeign(context.Background(), "rest:http://127.0.0.1:9999/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}
	started, err := s.StartForeignRestore(context.Background(), id, "files", "docs", "latest", true, "restore-here/docs")
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
	ctx := context.Background()

	// Unconfirmed: the sentinel, before ANY engine call or session lookup.
	started, err := s.StartForeignRestore(ctx, "whatever", "containers", "web", "latest", false, "")
	if started || !errors.Is(err, backup.ErrNotConfirmed) {
		t.Fatalf("unconfirmed: want ErrNotConfirmed, got started=%v err=%v", started, err)
	}
	if len(eng.calls) != 0 {
		t.Fatalf("unconfirmed restore must not touch the engine, got %v", eng.calls)
	}

	// Unknown session (also the expired case — foreignSession sweeps first).
	started, err = s.StartForeignRestore(ctx, "nope", "containers", "web", "latest", true, "")
	if started || !errors.Is(err, errForeignSession) {
		t.Fatalf("unknown session: want errForeignSession, got started=%v err=%v", started, err)
	}

	id, _, err := s.OpenForeign(ctx, "rest:http://127.0.0.1:9999/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}

	// Unknown domain.
	if started, err = s.StartForeignRestore(ctx, id, "flash", "boot", "latest", true, ""); started || err == nil || !strings.Contains(err.Error(), "unknown domain") {
		t.Fatalf("unknown domain: want the domain error, got started=%v err=%v", started, err)
	}
	// File set without a target folder (foreign sets never restore in place).
	if started, err = s.StartForeignRestore(ctx, id, "files", "docs", "latest", true, ""); started || err == nil || !strings.Contains(err.Error(), "target folder") {
		t.Fatalf("files without target: want the target-folder error, got started=%v err=%v", started, err)
	}
	// Unsafe item name (feeds tags, def filenames and progress keys).
	if started, err = s.StartForeignRestore(ctx, id, "files", "../evil", "latest", true, "restore-here"); started || err == nil || !strings.Contains(err.Error(), "invalid item name") {
		t.Fatalf("unsafe item: want the name error, got started=%v err=%v", started, err)
	}
	// Container whose def the foreign repo does not mirror (remote location —
	// there is no local def dir to read).
	if started, err = s.StartForeignRestore(ctx, id, "containers", "ghost", "latest", true, ""); started || err == nil || !strings.Contains(err.Error(), "definition") {
		t.Fatalf("missing def: want the definition error, got started=%v err=%v", started, err)
	}

	// None of the failures may leak the single-flight guard: a valid restore
	// still starts (and finishes) afterwards.
	if s.BackupInProgress() {
		t.Fatal("a failed foreign restore must release the single-flight guard")
	}
	started, err = s.StartForeignRestore(ctx, id, "files", "docs", "latest", true, "restore-here/docs")
	if err != nil || !started {
		t.Fatalf("valid restore after failures: started=%v err=%v", started, err)
	}
	waitForeignIdle(t, s)
}
