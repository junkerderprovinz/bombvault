package api

import (
	"context"
	"errors"
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
