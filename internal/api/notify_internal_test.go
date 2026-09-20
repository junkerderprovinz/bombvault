package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fakeHostSSH records Run calls. runOut and runErr are what Run returns.
type fakeHostSSH struct {
	runs   [][]string
	runOut string
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
	return f.runOut, f.runErr
}
func (f *fakeHostSSH) StreamCommand(context.Context, ...string) (io.ReadCloser, func() error, error) {
	return io.NopCloser(strings.NewReader("")), func() error { return nil }, nil
}
func (f *fakeHostSSH) RunWithStdin(_ context.Context, rd io.Reader, _ ...string) error {
	_, err := io.Copy(io.Discard, rd)
	return err
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

func TestNotifyBackupUnraidHonoursPolicy(t *testing.T) {
	ssh := &fakeHostSSH{}
	s := unraidNotifyService(t, ssh)
	if err := s.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}

	s.notifyBackup(context.Background(), "container", "plex", true, backup.Summary{SnapshotID: "deadbeef"}, nil)
	if len(ssh.runs) != 0 {
		t.Fatalf("no Unraid notify expected on success (policy=failure), got %v", ssh.runs)
	}

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

func TestNotifyBackupUnraidSkippedWithoutSSH(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{On: "always", Unraid: true}); err != nil {
		t.Fatal(err)
	}
	s.notifyBackup(context.Background(), "flash", "", true, backup.Summary{}, nil)
}

// The config domain has no item name, so a generic "%s %q" label would read
// `config ""`.
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

func TestTestNotifyNothingConfigured(t *testing.T) {
	s := unraidNotifyService(t, &fakeHostSSH{})
	if err := s.TestNotify(context.Background(), notify.Config{}); err == nil {
		t.Fatal("expected an error when no channel is configured")
	}
}

// Healthchecks tracks the whole run, so /start is pinged even when the policy
// only notifies on failure.
func TestNotifyBackupStartPingsHealthchecks(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { path = r.URL.Path }))
	defer srv.Close()

	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{On: "failure", HealthchecksURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	s.notifyBackupStart(context.Background(), "container")
	if path != "/start" {
		t.Fatalf("notifyBackupStart should ping /start, got %q", path)
	}
}

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

// A domain with its own Healthchecks URL is pinged there; one without falls
// back to the global URL.
func TestNotifyBackupStartPerDomainURL(t *testing.T) {
	var flashPath string
	var globalHits int
	flash := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { flashPath = r.URL.Path }))
	defer flash.Close()
	global := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { globalHits++ }))
	defer global.Close()

	s := unraidNotifyService(t, nil)
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

	s.notifyBackupStart(context.Background(), "config")
	if globalHits != 1 {
		t.Fatalf("config domain (no per-domain entry) should ping the global URL once, hits=%d", globalHits)
	}
}

func TestNotifyBackupStartFilesPerDomainURL(t *testing.T) {
	var filesPath string
	var globalHits int
	files := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { filesPath = r.URL.Path }))
	defer files.Close()
	global := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { globalHits++ }))
	defer global.Close()

	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{
		On:                   "failure",
		HealthchecksURL:      global.URL,
		HealthchecksByDomain: map[string]string{"files": files.URL},
	}); err != nil {
		t.Fatal(err)
	}

	s.notifyBackupStart(context.Background(), "files")
	if filesPath != "/start" {
		t.Fatalf("notifyBackupStart(files) should ping the files /start, got %q", filesPath)
	}
	if globalHits != 0 {
		t.Fatalf("global URL must not be pinged for the files domain, hits=%d", globalHits)
	}
}

// runScheduledContainers plays a scheduled containers run as cmd/bombvault
// wires it: one /start for the run, each item backed up with per-item
// Healthchecks pings suppressed, then one result. fail names the items that
// fail.
func runScheduledContainers(s *Service, items []string, fail map[string]bool) {
	s.ScheduledHealthchecksStart(context.Background(), "containers")
	attempted, failed := 0, 0
	for _, name := range items {
		attempted++
		ictx := notify.WithHealthchecksSuppressed(context.Background())
		s.notifyBackupStart(ictx, "container")
		ok := !fail[name]
		var berr error
		if !ok {
			failed++
			berr = errors.New("boom")
		}
		s.notifyBackup(ictx, "container", name, ok, backup.Summary{SnapshotID: "deadbeef"}, berr)
	}
	s.ScheduledHealthchecksResult(context.Background(), "containers", attempted, failed)
}

// Healthchecks sees one /start and one success for the whole run, while every
// item still sends its own message.
func TestScheduledContainersRunSendsOneStartOneSuccess(t *testing.T) {
	var mu sync.Mutex
	var hcPaths []string
	var webhookHits int
	hc := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hcPaths = append(hcPaths, r.URL.Path)
		mu.Unlock()
	}))
	defer hc.Close()
	wh := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		mu.Lock()
		webhookHits++
		mu.Unlock()
	}))
	defer wh.Close()

	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{
		On: "always", HealthchecksURL: hc.URL, WebhookEnabled: true, WebhookURL: wh.URL, WebhookFormat: "generic",
	}); err != nil {
		t.Fatal(err)
	}

	runScheduledContainers(s, []string{"a", "b", "c"}, nil)

	mu.Lock()
	defer mu.Unlock()
	if len(hcPaths) != 2 || hcPaths[0] != "/start" || hcPaths[1] != "/" {
		t.Fatalf("scheduled run should send exactly [/start /] to Healthchecks, got %v", hcPaths)
	}
	if webhookHits != 3 {
		t.Fatalf("each of the 3 items must still fire its webhook, hits=%d", webhookHits)
	}
}

func TestScheduledContainersRunFailsWhenAnyItemFails(t *testing.T) {
	var mu sync.Mutex
	var hcPaths []string
	var webhookHits int
	hc := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hcPaths = append(hcPaths, r.URL.Path)
		mu.Unlock()
	}))
	defer hc.Close()
	wh := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		mu.Lock()
		webhookHits++
		mu.Unlock()
	}))
	defer wh.Close()

	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{
		On: "always", HealthchecksURL: hc.URL, WebhookEnabled: true, WebhookURL: wh.URL, WebhookFormat: "generic",
	}); err != nil {
		t.Fatal(err)
	}

	runScheduledContainers(s, []string{"a", "b", "c"}, map[string]bool{"b": true})

	mu.Lock()
	defer mu.Unlock()
	if len(hcPaths) != 2 || hcPaths[0] != "/start" || hcPaths[1] != "/fail" {
		t.Fatalf("a run with a failed item should send exactly [/start /fail], got %v", hcPaths)
	}
	if webhookHits != 3 {
		t.Fatalf("each of the 3 items must still fire its webhook, hits=%d", webhookHits)
	}
}

// Only scheduled runs aggregate the pings; a manual backup sends its own
// /start and result.
func TestManualSingleBackupStillPingsHealthchecksOnce(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
	}))
	defer srv.Close()

	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{On: "always", HealthchecksURL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	s.notifyBackupStart(ctx, "container")
	s.notifyBackup(ctx, "container", "plex", true, backup.Summary{SnapshotID: "deadbeef"}, nil)

	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 2 || paths[0] != "/start" || paths[1] != "/" {
		t.Fatalf("manual single backup should ping /start then success, got %v", paths)
	}
}

// dumpNotifyService is unraidNotifyService with a container target to hang the
// dump runs on.
func dumpNotifyService(t *testing.T, ssh HostSSH, c notify.Config) (*Service, string) {
	t.Helper()
	s := unraidNotifyService(t, ssh)
	if err := s.SetNotifyConfig(c); err != nil {
		t.Fatal(err)
	}
	tg, err := s.store.UpsertTarget(store.Target{ContainerName: "pg", AppdataPaths: []string{"/x"}})
	if err != nil {
		t.Fatal(err)
	}
	return s, tg.ID
}

// recordDumpRun writes a finished dbdump run and returns its id.
func recordDumpRun(t *testing.T, s *Service, targetID, status, reason string) string {
	t.Helper()
	id, err := s.store.StartRun(targetID, "dbdump")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.FinishRun(id, status, "", 0, reason); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestDBDumpFailureNotifiesAfterTheBackup(t *testing.T) {
	failed := func(runID, reason string) backup.DBDumpOutcome {
		return backup.DBDumpOutcome{RunID: runID, Status: "failed", Reason: reason}
	}

	t.Run("it says how the files backup went", func(t *testing.T) {
		ssh := &fakeHostSSH{}
		s, targetID := dumpNotifyService(t, ssh, notify.Config{On: "failure", Unraid: true})

		runID := recordDumpRun(t, s, targetID, "failed", store.ReasonDBDumpAuth+": FATAL: password authentication failed for user \"immich\"")
		s.notifyDBDumpFailed(context.Background(), targetID, "pg", failed(runID, store.ReasonDBDumpAuth+": FATAL: password authentication failed for user \"immich\""), true)

		if len(ssh.runs) != 1 {
			t.Fatalf("%d messages, want one", len(ssh.runs))
		}
		sent := strings.Join(ssh.runs[0], " ")
		if !strings.Contains(sent, `Database dump of container "pg" failed: the database refused the login.`) {
			t.Errorf("message = %s", sent)
		}
		if !strings.Contains(sent, "The files backup of the container succeeded.") {
			t.Errorf("message does not say how the backup went: %s", sent)
		}
		if strings.Contains(sent, "password authentication failed") {
			t.Errorf("the tool's own message left the box: %s", sent)
		}
		if !strings.Contains(sent, "warning") {
			t.Errorf("a failed dump notifies at level warning: %s", sent)
		}
	})

	t.Run("a backup that failed as well", func(t *testing.T) {
		ssh := &fakeHostSSH{}
		s, targetID := dumpNotifyService(t, ssh, notify.Config{On: "failure", Unraid: true})

		runID := recordDumpRun(t, s, targetID, "failed", store.ReasonDBDumpTool)
		s.notifyDBDumpFailed(context.Background(), targetID, "pg", failed(runID, store.ReasonDBDumpTool), false)

		if len(ssh.runs) != 1 {
			t.Fatalf("%d messages, want one", len(ssh.runs))
		}
		if sent := strings.Join(ssh.runs[0], " "); !strings.Contains(sent, "The files backup of the container failed as well; see its own message.") {
			t.Errorf("message = %s", sent)
		}
	})

	t.Run("the same failure twice", func(t *testing.T) {
		ssh := &fakeHostSSH{}
		s, targetID := dumpNotifyService(t, ssh, notify.Config{On: "failure", Unraid: true})

		first := recordDumpRun(t, s, targetID, "failed", store.ReasonDBDumpAuth)
		s.notifyDBDumpFailed(context.Background(), targetID, "pg", failed(first, store.ReasonDBDumpAuth), true)

		second := recordDumpRun(t, s, targetID, "failed", store.ReasonDBDumpAuth+": FATAL")
		s.notifyDBDumpFailed(context.Background(), targetID, "pg", failed(second, store.ReasonDBDumpAuth+": FATAL"), true)
		if len(ssh.runs) != 1 {
			t.Fatalf("%d messages, want the repeat to stay quiet", len(ssh.runs))
		}

		third := recordDumpRun(t, s, targetID, "failed", store.ReasonDBDumpTool)
		s.notifyDBDumpFailed(context.Background(), targetID, "pg", failed(third, store.ReasonDBDumpTool), true)
		if len(ssh.runs) != 2 {
			t.Fatalf("%d messages, want another reason to speak up", len(ssh.runs))
		}

		recordDumpRun(t, s, targetID, "success", "")
		fourth := recordDumpRun(t, s, targetID, "failed", store.ReasonDBDumpTool)
		s.notifyDBDumpFailed(context.Background(), targetID, "pg", failed(fourth, store.ReasonDBDumpTool), true)
		if len(ssh.runs) != 3 {
			t.Fatalf("%d messages, want the same reason after a success to speak up", len(ssh.runs))
		}
	})

	t.Run("a cancelled dump", func(t *testing.T) {
		ssh := &fakeHostSSH{}
		s, targetID := dumpNotifyService(t, ssh, notify.Config{On: "always", Unraid: true})

		runID := recordDumpRun(t, s, targetID, "cancelled", store.ReasonCancelled)
		s.notifyDBDumpFailed(context.Background(), targetID, "pg",
			backup.DBDumpOutcome{RunID: runID, Status: "cancelled", Reason: store.ReasonCancelled}, true)
		s.notifyDBDumpFailed(context.Background(), targetID, "pg",
			backup.DBDumpOutcome{RunID: runID, Status: "failed", Reason: store.ReasonCancelled}, true)

		if len(ssh.runs) != 0 {
			t.Fatalf("a cancelled dump notified: %v", ssh.runs)
		}
	})

	t.Run("a scheduled round in summary mode", func(t *testing.T) {
		ssh := &fakeHostSSH{}
		s, targetID := dumpNotifyService(t, ssh, notify.Config{On: "failure", Unraid: true, ScheduledSummary: true})

		ctx := withDBDumpTally(notify.WithMessagesSuppressed(context.Background()))
		runID := recordDumpRun(t, s, targetID, "failed", store.ReasonDBDumpAuth)
		s.notifyDBDumpFailed(ctx, targetID, "pg", failed(runID, store.ReasonDBDumpAuth), true)
		if len(ssh.runs) != 0 {
			t.Fatalf("a per-item message went out in summary mode: %v", ssh.runs)
		}

		s.ScheduledNotifyResult(ctx, "containers", 1, 0, nil)
		if len(ssh.runs) != 1 {
			t.Fatalf("%d summaries, want one", len(ssh.runs))
		}
		if sent := strings.Join(ssh.runs[0], " "); !strings.Contains(sent, "1 database dumps failed: pg") {
			t.Fatalf("summary = %s", sent)
		}
	})
}
