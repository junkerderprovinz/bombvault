package api

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
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

	mu    sync.Mutex
	calls []string
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

func (f *foreignRecordingEngine) Snapshots(_ context.Context, _ string, _ restic.Mode) ([]restic.Snapshot, error) {
	f.record("Snapshots")
	return f.snaps, f.snapsErr
}

func (f *foreignRecordingEngine) Unlock(_ context.Context, _ string, _ bool, _ restic.Mode) error {
	f.record("Unlock") // stale-lock self-heal on the read path; not a repo write
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
