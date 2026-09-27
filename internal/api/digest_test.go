package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// digestTestService returns a Service whose only notify channel is a webhook,
// and a func that returns the last body the webhook received.
func digestTestService(t *testing.T, on string) (*api.Service, *store.Repo, func() string) {
	t.Helper()
	var mu sync.Mutex
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		body = string(b)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: filepath.ToSlash(dir)}
	st := newMemStore(t)
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	if err := svc.SetNotifyConfig(notify.Config{On: on, WebhookEnabled: true, WebhookURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	return svc, st, func() string {
		mu.Lock()
		defer mu.Unlock()
		return body
	}
}

// seedBackupRun records one finished backup run for the target.
func seedBackupRun(t *testing.T, st *store.Repo, targetID, status, errMsg string, bytes int64) {
	t.Helper()
	id, err := st.StartRun(targetID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(id, status, "", bytes, errMsg); err != nil {
		t.Fatal(err)
	}
}

func TestSendDigestCarriesCountsAndFailureLine(t *testing.T) {
	svc, st, body := digestTestService(t, "always")

	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/host/user/appdata/plex"}})
	if err != nil {
		t.Fatal(err)
	}
	seedBackupRun(t, st, tg.ID, "success", "", 1024)
	seedBackupRun(t, st, tg.ID, "success", "", 2048)
	seedBackupRun(t, st, tg.ID, "failed", "disk full", 0)

	if err := svc.SendDigest(context.Background()); err != nil {
		t.Fatalf("SendDigest: %v", err)
	}

	got := body()
	if got == "" {
		t.Fatal("SendDigest must deliver a message through the notify fan-out, got none")
	}
	if !strings.Contains(got, "backup: 2 ok, 1 failed") {
		t.Fatalf("digest must carry the per-kind counts, got %q", got)
	}
	if !strings.Contains(got, "backup plex: disk full") {
		t.Fatalf("digest must name the failed item with its reason, got %q", got)
	}
	if !strings.Contains(got, "New backup data: 3.0 KiB") {
		t.Fatalf("digest must sum the successful backup bytes, got %q", got)
	}
	if !strings.Contains(got, `"ok":false`) {
		t.Fatalf("a digest with failures must be delivered as ok=false, got %q", got)
	}
}

// TestSendDigestRespectsNeverPolicy: a muted policy sends nothing and returns
// no error.
func TestSendDigestRespectsNeverPolicy(t *testing.T) {
	url, hits := webhookCounter(t)
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: filepath.ToSlash(dir)}
	st := newMemStore(t)
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	if err := svc.SetNotifyConfig(notify.Config{On: "never", WebhookURL: url}); err != nil {
		t.Fatal(err)
	}

	if err := svc.SendDigest(context.Background()); err != nil {
		t.Fatalf("SendDigest under a muted policy must be a silent no-op, got %v", err)
	}
	if atomic.LoadInt32(hits) != 0 {
		t.Fatal("a muted policy must send NO digest")
	}
}

// seedRunOfKind records one finished run of any kind for the target.
func seedRunOfKind(t *testing.T, st *store.Repo, targetID, kind, status string) {
	t.Helper()
	id, err := st.StartRun(targetID, kind)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(id, status, "", 0, ""); err != nil {
		t.Fatal(err)
	}
}

func TestDigestListsDBDumpKind(t *testing.T) {
	svc, st, body := digestTestService(t, "always")

	tg, err := st.UpsertTarget(store.Target{ContainerName: "pg", AppdataPaths: []string{"/host/user/appdata/pg"}})
	if err != nil {
		t.Fatal(err)
	}
	seedBackupRun(t, st, tg.ID, "success", "", 1024)
	seedRunOfKind(t, st, tg.ID, "dbdump", "success")
	seedRunOfKind(t, st, tg.ID, "dbdumpsave", "success")
	seedRunOfKind(t, st, tg.ID, "dbimport", "success")

	if err := svc.SendDigest(context.Background()); err != nil {
		t.Fatalf("SendDigest: %v", err)
	}

	got := body()
	for _, want := range []string{"dbdump: 1 ok, 0 failed", "dbdumpsave: 1 ok, 0 failed", "dbimport: 1 ok, 0 failed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("digest is missing %q:\n%s", want, got)
		}
	}
	order := []string{"- backup:", "- dbdump:", "- dbdumpsave:", "- dbimport:"}
	at := 0
	for _, line := range order {
		i := strings.Index(got[at:], line)
		if i < 0 {
			t.Fatalf("digest is missing %q:\n%s", line, got)
		}
		at += i
	}
}

func TestDigestLeavesOutWhatADatabaseToolSaid(t *testing.T) {
	svc, st, body := digestTestService(t, "always")

	tg, err := st.UpsertTarget(store.Target{ContainerName: "pg", AppdataPaths: []string{"/host/user/appdata/pg"}})
	if err != nil {
		t.Fatal(err)
	}
	quoted := "pg_dump: error: DETAIL: Key (email)=(alice@example.com) already exists"
	seedFailedRunOfKind(t, st, tg.ID, "dbdump", store.ReasonDBDumpTool+": "+quoted)
	seedFailedRunOfKind(t, st, tg.ID, "dbimport",
		store.ReasonDBImportFailed+": the previous data folder is kept at /data/pg.bombvault-before-import-20260917-021403; exit 1: "+quoted)

	if err := svc.SendDigest(context.Background()); err != nil {
		t.Fatalf("SendDigest: %v", err)
	}

	got := body()
	if strings.Contains(got, "alice@example.com") {
		t.Fatalf("the digest carries a row a database tool quoted:\n%s", got)
	}
	for _, want := range []string{"dbdump pg: " + store.ReasonDBDumpTool, "pg.bombvault-before-import-20260917-021403"} {
		if !strings.Contains(got, want) {
			t.Errorf("digest is missing %q:\n%s", want, got)
		}
	}
}

func seedFailedRunOfKind(t *testing.T, st *store.Repo, targetID, kind, reason string) {
	t.Helper()
	id, err := st.StartRun(targetID, kind)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(id, "failed", "", 0, reason); err != nil {
		t.Fatal(err)
	}
}

// The weekly reminder for a critical nobody settled and for the deleting of
// old backups that is still waiting on it.
func TestDigestMentionsOpenAnomalies(t *testing.T) {
	svc, st, body := digestTestService(t, "always")
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	seedBackupRun(t, st, tg.ID, "success", "", 1024)
	if err := svc.SendDigest(context.Background()); err != nil {
		t.Fatalf("SendDigest: %v", err)
	}
	if strings.Contains(body(), "Anomalies still open") {
		t.Fatalf("a digest with no findings must not mention them, got %q", body())
	}

	now := time.Now().Unix()
	seedAnomaly(t, st, store.Anomaly{
		ID: "shrink", Detector: "source", Metric: "source_bytes_shrink", Severity: "critical",
		ScopeKind: "item", ScopeID: tg.ID, TargetID: tg.ID, Domain: "container", LastSeenAt: now,
	})
	seedAnomaly(t, st, store.Anomaly{
		ID: "slower", Detector: "duration", Metric: "duration_slower", Severity: "warning",
		ScopeKind: "item", ScopeID: tg.ID, TargetID: tg.ID, Domain: "container", LastSeenAt: now,
	})
	startAnomalyEngine(t, svc)

	if err := svc.SendDigest(context.Background()); err != nil {
		t.Fatalf("SendDigest: %v", err)
	}
	if want := "Anomalies still open: critical 1, warning 1, retention paused for 1 item(s)"; !strings.Contains(body(), want) {
		t.Fatalf("digest must carry %q, got %q", want, body())
	}
}

func TestDigestReportsTheZFSOffsiteCopy(t *testing.T) {
	svc, st, body := digestTestService(t, "always")
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ZFSEnabled = true
	s.ZFSOffsite = "s3:offsite-zfs"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	if err := svc.SendDigest(context.Background()); err != nil {
		t.Fatalf("SendDigest: %v", err)
	}
	if got := body(); !strings.Contains(got, "- zfs: no successful copy yet") {
		t.Fatalf("the digest must say how current the ZFS off-site copy is, got %q", got)
	}
}
