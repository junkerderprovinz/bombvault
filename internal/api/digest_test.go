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

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// digestTestService builds a Service over a mem store with a webhook capture
// server wired as the (only) notify channel, so a test can read the exact
// digest message that was delivered. The generic webhook format posts JSON
// {"title","message","ok"}, so substring asserts against the body see the
// composed digest text.
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
	if err := svc.SetNotifyConfig(notify.Config{On: on, WebhookURL: srv.URL}); err != nil {
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

// TestSendDigestCarriesCountsAndFailureLine pins G6: the digest message carries
// the per-kind ok/failed counts of the seeded window (2 ok + 1 failed backup)
// and names the failed item with its reason.
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
	// 1 KiB + 2 KiB from the two successful backups must be summed in.
	if !strings.Contains(got, "New backup data: 3.0 KiB") {
		t.Fatalf("digest must sum the successful backup bytes, got %q", got)
	}
	// With a failure in the window the event is delivered as NOT-ok.
	if !strings.Contains(got, `"ok":false`) {
		t.Fatalf("a digest with failures must be delivered as ok=false, got %q", got)
	}
}

// TestSendDigestRespectsNeverPolicy pins the policy gate: with notifications
// muted ("never") the digest sends nothing and errors nothing.
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
