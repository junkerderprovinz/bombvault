package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// slowBackendEngine is a received repository's backend: a repo listed in
// hang answers only when the caller gives up, and a check fails the way
// restic does once its context is done.
type slowBackendEngine struct {
	ResticEngine
	hang map[string]bool
}

func (e *slowBackendEngine) RepoOpensErr(ctx context.Context, repo string, _ restic.Mode) error {
	if e.hang[repo] {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func (e *slowBackendEngine) Snapshots(ctx context.Context, _ string, _ restic.Mode) ([]restic.Snapshot, error) {
	return []restic.Snapshot{{ID: "aaa", Time: "2026-10-20T03:00:00Z"}}, ctx.Err()
}

func (e *slowBackendEngine) Check(ctx context.Context, _ string, _ restic.Mode) error {
	return ctx.Err()
}

func receivedFixture(t *testing.T, eng ResticEngine, locations ...string) (*Handler, *store.Repo, []store.ReceivedRepo) {
	t.Helper()
	const appKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	enc, err := secret.Encrypt(appKey, []byte("their restic password"))
	if err != nil {
		t.Fatal(err)
	}
	var repos []store.ReceivedRepo
	for _, loc := range locations {
		rr, err := st.CreateReceivedRepo(store.ReceivedRepo{Name: loc, Repo: loc, MemberID: "m-tower", ResticPasswordEnc: enc, Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		repos = append(repos, rr)
	}
	cfg := config.Config{AppKey: appKey, HostMountRoot: t.TempDir()}
	svc := &Service{cfg: cfg, store: st, engine: eng}
	return &Handler{cfg: cfg, store: st, svc: svc}, st, repos
}

func TestACheckCutOffIsNoVerdict(t *testing.T) {
	h, _, repos := receivedFixture(t, &slowBackendEngine{}, "rest:http://10.0.0.9:8000/tower")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.svc.receiverCheckExclusive(ctx, repos[0], false); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancellation rather than a failed check", err)
	}
}

func TestAManualCheckStillLandsAfterThePageIsLeft(t *testing.T) {
	h, st, repos := receivedFixture(t, &slowBackendEngine{}, "rest:http://10.0.0.9:8000/tower")
	ctx, leave := context.WithCancel(context.Background())
	leave()
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/receiver/repos/"+repos[0].ID+"/check", nil)
	req.SetPathValue("id", repos[0].ID)
	h.handleReceiverCheck(httptest.NewRecorder(), req)

	deadline := time.Now().Add(5 * time.Second)
	for {
		rr, _, err := st.GetReceivedRepo(repos[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if rr.LastCheckOK.Valid {
			if !rr.LastCheckOK.Bool {
				t.Fatalf("the check was recorded as failed once the page was left: %q", rr.LastCheckError)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the check never recorded a verdict")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestOneUnreachableReceivedRepoDoesNotHoldUpTheList(t *testing.T) {
	const slow, fast = "rest:http://10.0.0.9:8000/slow", "rest:http://10.0.0.8:8000/fast"
	old := receiverProbeTimeout
	receiverProbeTimeout = 50 * time.Millisecond
	t.Cleanup(func() { receiverProbeTimeout = old })
	h, _, _ := receivedFixture(t, &slowBackendEngine{hang: map[string]bool{slow: true}}, slow, fast)

	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		h.handleListReceiverRepos(w, httptest.NewRequest(http.MethodGet, "/api/receiver/repos", nil))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the list waited on the unreachable repository")
	}
	resp := decodeResp(t, w)
	rows, _ := resp["repos"].([]any)
	if len(rows) != 2 {
		t.Fatalf("repos = %v, want both rows", resp)
	}
	for _, row := range rows {
		m := row.(map[string]any)
		if want := m["repo"] == fast; m["reachable"] != want {
			t.Errorf("%v: reachable = %v, want %v", m["repo"], m["reachable"], want)
		}
	}
}
