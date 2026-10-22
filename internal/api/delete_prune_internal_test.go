package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// requestBoundEngine forgets the way restic does when its context ends: it
// stops and fails. It records whether each forget was asked to prune.
type requestBoundEngine struct {
	*placementEngine
	mu       sync.Mutex
	prunedIn []bool
}

func (e *requestBoundEngine) Forget(ctx context.Context, repo string, ids []string, prune bool, mode restic.Mode) error {
	e.mu.Lock()
	e.prunedIn = append(e.prunedIn, prune)
	e.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return e.placementEngine.Forget(ctx, repo, ids, prune, mode)
}

func deleteFixture(t *testing.T) (*placementFixture, *requestBoundEngine, string) {
	t.Helper()
	f := newPlacementFixture(t)
	repo := f.domainPath("containers")
	f.makeRepo(repo)
	f.eng.snaps[repo] = []restic.Snapshot{{ID: "aaaa1111", Tags: []string{"container:web"}}}
	if _, err := f.st.UpsertTarget(store.Target{ContainerName: "web"}); err != nil {
		t.Fatal(err)
	}
	eng := &requestBoundEngine{placementEngine: f.eng}
	f.svc.engine = eng
	f.signIn()
	return f, eng, repo
}

func TestDeletingBackupsFinishesAfterTheRequestEnds(t *testing.T) {
	f, _, _ := deleteFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := jsonReq("DELETE", "/api/containers/web/backups", nil).WithContext(ctx)
	r.AddCookie(&http.Cookie{Name: f.h.sessionCookieNameFor(), Value: f.session}) //nolint:gosec // G124: a cookie on a test request
	f.h.Router().ServeHTTP(httptest.NewRecorder(), r)
	f.svc.deletePrunes.Wait()

	if _, err := f.st.GetTargetByContainer("web"); err == nil {
		t.Fatal("the entry stayed although the request that deleted its backups had ended")
	}
}

func TestDeletingBackupsPrunesAfterItAnswers(t *testing.T) {
	f, eng, repo := deleteFixture(t)

	if res := f.do("DELETE", "/api/containers/web/backups", nil); res["ok"] != true {
		t.Fatalf("delete = %v", res)
	}
	f.svc.deletePrunes.Wait()

	if !slices.Equal(eng.prunedIn, []bool{false}) {
		t.Fatalf("forgets pruned = %v, want one forget that leaves the prune for later", eng.prunedIn)
	}
	if !slices.Contains(f.eng.prunes, repo) {
		t.Fatalf("pruned %v, want %s once the delete answered", f.eng.prunes, repo)
	}
	runs, err := f.st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(runs, func(r store.Run) bool {
		return r.TargetID == domainRunTargetID("containers") && r.Kind == "prune" && r.Status == "success"
	}) {
		t.Fatalf("runs = %+v, want a successful prune of the section", runs)
	}
}
