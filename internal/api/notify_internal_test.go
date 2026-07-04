package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fakeHostSSH records Run calls so the Unraid-notification channel can be tested
// without a real host.
type fakeHostSSH struct {
	runs   [][]string
	runErr error
}

var _ HostSSH = (*fakeHostSSH)(nil)

func (f *fakeHostSSH) ReadFile(context.Context, string) ([]byte, error) { return nil, nil }
func (f *fakeHostSSH) WriteFile(context.Context, string, []byte) error  { return nil }
func (f *fakeHostSSH) PublicKey() (string, error)                       { return "", nil }
func (f *fakeHostSSH) Test(context.Context) error                       { return nil }
func (f *fakeHostSSH) EnsureKnownHost(context.Context) error            { return nil }
func (f *fakeHostSSH) Run(_ context.Context, args ...string) (string, error) {
	f.runs = append(f.runs, args)
	return "", f.runErr
}

func unraidNotifyService(t *testing.T, ssh HostSSH) *Service {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() }) // close before TempDir cleanup (Windows file lock)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return &Service{
		cfg:   config.Config{AppKey: strings.Repeat("a", 64)},
		store: store.New(db),
		ssh:   ssh,
	}
}

// TestNotifyBackupUnraidHonoursPolicy: the Unraid channel runs the host notify
// script over SSH, on failure when policy="failure", and never on success then.
func TestNotifyBackupUnraidHonoursPolicy(t *testing.T) {
	ssh := &fakeHostSSH{}
	s := unraidNotifyService(t, ssh)
	if err := s.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}

	// Success with policy=failure → no notification.
	s.notifyBackup(context.Background(), "container", "plex", true, backup.Summary{SnapshotID: "deadbeef"}, nil)
	if len(ssh.runs) != 0 {
		t.Fatalf("no Unraid notify expected on success (policy=failure), got %v", ssh.runs)
	}

	// Failure → one notification via the host notify script, level "warning".
	s.notifyBackup(context.Background(), "container", "plex", false, backup.Summary{}, errors.New("boom"))
	if len(ssh.runs) != 1 {
		t.Fatalf("expected 1 Unraid notify on failure, got %d", len(ssh.runs))
	}
	joined := strings.Join(ssh.runs[0], " ")
	if !strings.Contains(joined, "/usr/local/emhttp/webGui/scripts/notify") {
		t.Fatalf("host notify script not invoked: %v", ssh.runs[0])
	}
	if !strings.Contains(joined, "warning") {
		t.Fatalf("a failed backup should notify at level warning: %v", ssh.runs[0])
	}
}

// TestNotifyBackupUnraidSkippedWithoutSSH: with no SSH set up, the Unraid channel
// is silently skipped (never panics).
func TestNotifyBackupUnraidSkippedWithoutSSH(t *testing.T) {
	s := unraidNotifyService(t, nil) // no SSH
	if err := s.SetNotifyConfig(notify.Config{On: "always", Unraid: true}); err != nil {
		t.Fatal(err)
	}
	s.notifyBackup(context.Background(), "flash", "", true, backup.Summary{}, nil) // must not panic
}

// TestNotifyBackupConfigLabel: the singleton config domain (no per-item name) must
// render a clean human label ("BombVault configuration"), never the empty-quote
// `config ""` a generic "%s %q" format would produce.
func TestNotifyBackupConfigLabel(t *testing.T) {
	ssh := &fakeHostSSH{}
	s := unraidNotifyService(t, ssh)
	if err := s.SetNotifyConfig(notify.Config{On: "always", Unraid: true}); err != nil {
		t.Fatal(err)
	}

	s.notifyBackup(context.Background(), "config", "", true, backup.Summary{SnapshotID: "deadbeef"}, nil)
	if len(ssh.runs) != 1 {
		t.Fatalf("expected 1 Unraid notify for a config backup, got %d", len(ssh.runs))
	}
	joined := strings.Join(ssh.runs[0], " ")
	if !strings.Contains(joined, "BombVault configuration") {
		t.Fatalf("config backup should use the clean label: %v", ssh.runs[0])
	}
	if strings.Contains(joined, `config ""`) {
		t.Fatalf("config backup must not render the empty-quote label: %v", ssh.runs[0])
	}
}

// TestTestNotifyUnraid: the Test button path sends a test through the Unraid
// channel over SSH.
func TestTestNotifyUnraid(t *testing.T) {
	ssh := &fakeHostSSH{}
	s := unraidNotifyService(t, ssh)
	if err := s.TestNotify(context.Background(), notify.Config{Unraid: true}); err != nil {
		t.Fatalf("TestNotify: %v", err)
	}
	if len(ssh.runs) != 1 {
		t.Fatalf("expected one test notify over SSH, got %d", len(ssh.runs))
	}
}

// TestTestNotifyNothingConfigured: a test with no channels is a clear error.
func TestTestNotifyNothingConfigured(t *testing.T) {
	s := unraidNotifyService(t, &fakeHostSSH{})
	if err := s.TestNotify(context.Background(), notify.Config{}); err == nil {
		t.Fatal("expected an error when no channel is configured")
	}
}

// TestNotifyBackupStartPingsHealthchecks: notifyBackupStart pings the Healthchecks
// /start endpoint at the beginning of a backup when a URL is configured — even under
// On="failure", since Healthchecks tracks the whole lifecycle independent of policy.
func TestNotifyBackupStartPingsHealthchecks(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { path = r.URL.Path }))
	defer srv.Close()

	s := unraidNotifyService(t, nil) // SendStart is HTTP-only; no SSH needed
	if err := s.SetNotifyConfig(notify.Config{On: "failure", HealthchecksURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	s.notifyBackupStart(context.Background(), "container")
	if path != "/start" {
		t.Fatalf("notifyBackupStart should ping /start, got %q", path)
	}
}

// TestNotifyBackupStartSuppressedWhenNever: with On="never" the start ping is a no-op,
// so a Healthchecks server configured only for reference is never contacted.
func TestNotifyBackupStartSuppressedWhenNever(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer srv.Close()

	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{On: "never", HealthchecksURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	s.notifyBackupStart(context.Background(), "container")
	if hits != 0 {
		t.Fatalf("notifyBackupStart under On=never should not ping, hits=%d", hits)
	}
}

// TestNotifyBackupStartPerDomainURL: notifyBackupStart routes the /start ping to the
// domain's own Healthchecks URL when HealthchecksByDomain has an entry for it, while a
// domain without an entry falls back to the global URL.
func TestNotifyBackupStartPerDomainURL(t *testing.T) {
	var flashPath string
	var globalHits int
	flash := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { flashPath = r.URL.Path }))
	defer flash.Close()
	global := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { globalHits++ }))
	defer global.Close()

	s := unraidNotifyService(t, nil) // SendStart is HTTP-only; no SSH needed
	if err := s.SetNotifyConfig(notify.Config{
		On:                   "failure",
		HealthchecksURL:      global.URL,
		HealthchecksByDomain: map[string]string{"flash": flash.URL},
	}); err != nil {
		t.Fatal(err)
	}

	s.notifyBackupStart(context.Background(), "flash")
	if flashPath != "/start" {
		t.Fatalf("notifyBackupStart(flash) should ping the flash /start, got %q", flashPath)
	}
	if globalHits != 0 {
		t.Fatalf("global URL must not be pinged for the flash domain, hits=%d", globalHits)
	}

	// A domain without a per-domain entry falls back to the global URL.
	s.notifyBackupStart(context.Background(), "config")
	if globalHits != 1 {
		t.Fatalf("config domain (no per-domain entry) should ping the global URL once, hits=%d", globalHits)
	}
}
