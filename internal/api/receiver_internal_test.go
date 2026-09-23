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

// receiverTestService builds a minimal Service on the real restic engine with
// appKey as this instance's key, which encrypts the stored sending key. The
// receiver only reads, so no store, docker or ssh is needed.
func receiverTestService(appKey string) *Service {
	return &Service{
		// With the OS temp root as the mount, a t.TempDir() repo lies inside it,
		// the only place a container can reach.
		cfg:    config.Config{AppKey: appKey, HostMountRoot: os.TempDir()},
		engine: restic.Restic{Bin: "restic"},
	}
}

// makeReceivedRepo encrypts sendingKey under appKey and returns a ReceivedRepo
// pointing at repo, as a registered receiver row would.
func makeReceivedRepo(t *testing.T, appKey, sendingKey, repo string, readDataPct int) store.ReceivedRepo {
	t.Helper()
	enc, err := secret.Encrypt(appKey, []byte(sendingKey))
	if err != nil {
		t.Fatalf("Encrypt sending key: %v", err)
	}
	return store.ReceivedRepo{Repo: repo, AppKeyEnc: enc, ReadDataPercent: readDataPct, Enabled: true}
}

// seedReceivedRepo initializes a real encrypted restic repo as the sending
// instance would, backs up three tagged snapshots into it and returns its path.
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
	// Two snapshots for container:web and one for vm:db. All share this
	// machine's hostname, so only the item tells the groups apart.
	for _, tag := range []string{"container:web", "container:web", "vm:db"} {
		if _, err := r.Backup(ctx, repo, []string{src}, []string{tag}, m); err != nil {
			t.Fatalf("Backup %s: %v", tag, err)
		}
	}
	return repo
}

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

func TestReceiverCheckGoodAndWrongKey(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	appKey := strings.Repeat("ab", 32)
	sendingKey := strings.Repeat("cd", 32)
	repo := seedReceivedRepo(t, sendingKey)
	s := receiverTestService(appKey)

	good := makeReceivedRepo(t, appKey, sendingKey, repo, 0)
	if res := s.receiverCheck(context.Background(), good, false); !res.OK || res.Error != "" || res.RanReadData {
		t.Fatalf("structural check on a good repo should pass without read-data: %+v", res)
	}
	deep := makeReceivedRepo(t, appKey, sendingKey, repo, 100)
	if res := s.receiverCheck(context.Background(), deep, true); !res.OK || !res.RanReadData {
		t.Fatalf("deep check on a good repo should pass and ran read-data: %+v", res)
	}

	wrong := makeReceivedRepo(t, appKey, strings.Repeat("ef", 32), repo, 0)
	res := s.receiverCheck(context.Background(), wrong, false)
	if res.OK || res.Error == "" {
		t.Fatalf("check with a wrong stored key must fail: %+v", res)
	}
}

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
	if _, err := os.Stat(filepath.Join(missing, "config")); !os.IsNotExist(err) {
		t.Fatalf("engine initialized a repo (config exists): err=%v", err)
	}
}

// The SPA gates the receiver tab on receiverEnabled.
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

// A received repo's location goes through the same path resolution as every
// other repo field. BombVault runs in a container without /mnt/user, so a user
// who types the path Unraid shows has to be told the path is the problem and
// what to type instead, not that the APP_KEY is wrong.
func TestReceiverOpenResolvesTheHostPath(t *testing.T) {
	appKey := strings.Repeat("a", 64)
	svc := receiverTestService(appKey)
	svc.cfg.HostMountRoot = "/host/user"
	svc.cfg.HostSourceRoot = "/mnt"

	rr := makeReceivedRepo(t, appKey, strings.Repeat("b", 64), "/mnt/user/LJSNAS01_restic_repo/LJSNAS01/folders/", 0)
	_, _, err := svc.receiverOpen(context.Background(), rr)
	if err == nil {
		t.Fatal("an absolute host path must be rejected with an explanation, not opened")
	}
	msg := err.Error()
	if strings.Contains(msg, "APP_KEY") {
		t.Fatalf("the path is the problem; the message must not blame the key: %q", msg)
	}
	if !strings.Contains(msg, "absolute host path") {
		t.Fatalf("the message must name the path problem, got %q", msg)
	}
	if !strings.Contains(msg, "user/LJSNAS01_restic_repo/LJSNAS01/folders") {
		t.Fatalf("the message must suggest the relative path, got %q", msg)
	}
}

// rest:, s3: and rclone: locations are backends, not paths, and have to reach
// restic untouched. Resolving one would turn it into a subpath of the mount.
func TestReceiverOpenKeepsRemoteLocations(t *testing.T) {
	appKey := strings.Repeat("a", 64)
	svc := receiverTestService(appKey)
	svc.cfg.HostMountRoot = "/host/user"

	for _, loc := range []string{"rest:http://box:8000/repo", "s3:s3.example.com/bucket", "rclone:remote:path"} {
		rr := makeReceivedRepo(t, appKey, strings.Repeat("b", 64), loc, 0)
		_, _, err := svc.receiverOpen(context.Background(), rr)
		// The repo does not exist, so opening fails, but at the open and not at
		// path resolution.
		if err != nil && strings.Contains(err.Error(), "absolute host path") {
			t.Errorf("%q is a backend URL and must not be path-resolved: %v", loc, err)
		}
	}
}
