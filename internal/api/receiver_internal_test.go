package api

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// receiverTestService builds a minimal Service wired to the REAL restic engine and
// carrying appKey as this instance's secret key (used to encrypt/decrypt the stored
// sending key). No store/docker/ssh is needed: the receiver engine is read-only.
func receiverTestService(appKey string) *Service {
	return &Service{
		cfg:    config.Config{AppKey: appKey},
		engine: restic.Restic{Bin: "restic"},
	}
}

// makeReceivedRepo encrypts sendingKey under appKey and returns a ReceivedRepo
// pointing at repo — exactly what a registered off-site receiver row looks like.
func makeReceivedRepo(t *testing.T, appKey, sendingKey, repo string, readDataPct int) store.ReceivedRepo {
	t.Helper()
	enc, err := secret.Encrypt(appKey, []byte(sendingKey))
	if err != nil {
		t.Fatalf("Encrypt sending key: %v", err)
	}
	return store.ReceivedRepo{Repo: repo, AppKeyEnc: enc, ReadDataPercent: readDataPct, Enabled: true}
}

// seedReceivedRepo initializes a real encrypted restic repo (as the SENDING
// instance would) and writes snapshots with the given host/item tags. Returns the
// repo path. Each (path, tag) pair is one `restic backup` run = one snapshot.
func seedReceivedRepo(t *testing.T, sendingKey string) string {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil { //nolint:gosec // G301: test temp dir
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("bombvault receiver test payload"), 0o644); err != nil { //nolint:gosec // G306: test file
		t.Fatal(err)
	}

	r := restic.Restic{Bin: "restic"}
	m := restic.Mode{Encrypted: true, Password: restickey.Derive(sendingKey)}
	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// Two snapshots for container:web (proves per-source grouping counts), one for
	// vm:db. All share this machine's hostname, so grouping distinguishes by item.
	for _, tag := range []string{"container:web", "container:web", "vm:db"} {
		if _, err := r.Backup(ctx, repo, []string{src}, []string{tag}, m); err != nil {
			t.Fatalf("Backup %s: %v", tag, err)
		}
	}
	return repo
}

// TestReceiverInventoryGroupsBySource opens a real received repo READ-ONLY and
// verifies the inventory groups snapshots by source with the right counts, a
// non-empty lastReceived, a positive per-source size and repo totals.
func TestReceiverInventoryGroupsBySource(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	appKey := strings.Repeat("ab", 32)
	sendingKey := strings.Repeat("cd", 32)
	repo := seedReceivedRepo(t, sendingKey)

	s := receiverTestService(appKey)
	rr := makeReceivedRepo(t, appKey, sendingKey, repo, 0)

	inv, err := s.receiverInventory(context.Background(), rr)
	if err != nil {
		t.Fatalf("receiverInventory: %v", err)
	}
	if inv.SnapshotCount != 3 {
		t.Fatalf("repo snapshot total = %d, want 3", inv.SnapshotCount)
	}
	if len(inv.Sources) != 2 {
		t.Fatalf("want 2 sources (container:web, vm:db), got %d: %+v", len(inv.Sources), inv.Sources)
	}
	byItem := map[string]ReceiverSource{}
	for _, src := range inv.Sources {
		byItem[src.Item] = src
	}
	web, ok := byItem["container:web"]
	if !ok || web.SnapshotCount != 2 {
		t.Fatalf("container:web source wrong: %+v", web)
	}
	if web.LastReceived == "" {
		t.Fatal("container:web lastReceived must be set (newest snapshot time)")
	}
	if web.TotalSize <= 0 {
		t.Fatalf("container:web size must be > 0, got %d", web.TotalSize)
	}
	if db, ok := byItem["vm:db"]; !ok || db.SnapshotCount != 1 {
		t.Fatalf("vm:db source wrong: %+v", db)
	}
	if inv.LastReceived == "" {
		t.Fatal("repo-wide lastReceived must be set")
	}
}

// TestReceiverCheckGoodAndWrongKey verifies receiverCheck returns ok on a healthy
// repo opened with the right key, and a not-ok verdict (with an error, no panic)
// when the stored sending key is wrong — and that a deep --read-data check runs
// when requested.
func TestReceiverCheckGoodAndWrongKey(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	appKey := strings.Repeat("ab", 32)
	sendingKey := strings.Repeat("cd", 32)
	repo := seedReceivedRepo(t, sendingKey)
	s := receiverTestService(appKey)

	// Right key, structural check.
	good := makeReceivedRepo(t, appKey, sendingKey, repo, 0)
	if res := s.receiverCheck(context.Background(), good, false); !res.OK || res.Error != "" || res.RanReadData {
		t.Fatalf("structural check on a good repo should pass without read-data: %+v", res)
	}
	// Right key, deep read-data check (percent configured).
	deep := makeReceivedRepo(t, appKey, sendingKey, repo, 100)
	if res := s.receiverCheck(context.Background(), deep, true); !res.OK || !res.RanReadData {
		t.Fatalf("deep check on a good repo should pass and ran read-data: %+v", res)
	}

	// Wrong sending key stored -> the repo cannot be opened -> not-ok with an error.
	wrong := makeReceivedRepo(t, appKey, strings.Repeat("ef", 32), repo, 0)
	res := s.receiverCheck(context.Background(), wrong, false)
	if res.OK || res.Error == "" {
		t.Fatalf("check with a wrong stored key must fail: %+v", res)
	}
}

// TestReceiverNeverInitializesRepo is the read-only guarantee: opening a
// non-existent location must return an error and MUST NOT initialize a repo there
// (no restic 'config' object is written). Both engine entry points are checked.
func TestReceiverNeverInitializesRepo(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	appKey := strings.Repeat("ab", 32)
	sendingKey := strings.Repeat("cd", 32)
	missing := filepath.Join(t.TempDir(), "nope")
	s := receiverTestService(appKey)
	rr := makeReceivedRepo(t, appKey, sendingKey, missing, 0)

	if _, err := s.receiverInventory(context.Background(), rr); err == nil {
		t.Fatal("receiverInventory on a missing repo must error, not initialize it")
	}
	if res := s.receiverCheck(context.Background(), rr, false); res.OK {
		t.Fatalf("receiverCheck on a missing repo must not report ok: %+v", res)
	}
	// The read-only engine must never have created the repo (no config object,
	// ideally nothing at all under the location).
	if _, err := os.Stat(filepath.Join(missing, "config")); !os.IsNotExist(err) {
		t.Fatalf("engine initialized a repo (config exists): err=%v", err)
	}
}

// TestReceiverEnabledInSettingsView pins that the receiverEnabled flag surfaces in
// the settings view (GET/PUT via toView, and the portable export via
// buildSettingsView) so the SPA can gate the receiver tab on it.
func TestReceiverEnabledInSettingsView(t *testing.T) {
	on := toView(store.Settings{ReceiverEnabled: true})
	if !on.ReceiverEnabled {
		t.Fatal("toView must carry ReceiverEnabled=true")
	}
	off := toView(store.Settings{ReceiverEnabled: false})
	if off.ReceiverEnabled {
		t.Fatal("toView must carry ReceiverEnabled=false")
	}
	if !buildSettingsView(store.Settings{ReceiverEnabled: true}).ReceiverEnabled {
		t.Fatal("buildSettingsView must carry ReceiverEnabled through to the export")
	}
}
