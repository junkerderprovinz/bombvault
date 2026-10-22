package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const (
	pullOurKey   = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	pullTheirKey = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	pullSrcLoc   = "rest:http://192.168.1.9:8000/their-containers"
	pullDomain   = "s3:https://s3.example.com/bucket/containers"
)

// ctxPullRecorder is pullRecorder whose copy fails once its context is done,
// as restic does when exec.CommandContext kills it, and can be held open.
type ctxPullRecorder struct {
	pullRecorder
	hold chan struct{}
}

func (e *ctxPullRecorder) Copy(ctx context.Context, dest, src string, ids []string, lim restic.Limits, m restic.Mode) error {
	if e.hold != nil {
		<-e.hold
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return e.pullRecorder.Copy(ctx, dest, src, ids, lim, m)
}

func pullFixture(t *testing.T, eng ResticEngine) (*Service, *store.Repo, store.PullSource) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = pullDomain
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(pullOurKey, []byte(restickey.Derive(pullTheirKey)))
	if err != nil {
		t.Fatal(err)
	}
	ps, err := st.CreatePullSource(store.PullSource{Name: "Tower", Repo: pullSrcLoc, ResticPasswordEnc: enc, Domain: "containers", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	svc := &Service{cfg: config.Config{AppKey: pullOurKey, DataDir: dir, HostMountRoot: dir}, store: st, engine: eng}
	return svc, st, ps
}

func TestPulledSnapshotsLandInARepositoryOfTheirOwn(t *testing.T) {
	eng := &pullRecorder{snaps: map[string][]restic.Snapshot{pullSrcLoc: {{ID: "aaa"}}}}
	svc, _, ps := pullFixture(t, eng)
	if _, err := svc.PullFromSource(context.Background(), ps); err != nil {
		t.Fatal(err)
	}

	want := pullDomain + "/pulled/" + ps.ID
	copied := false
	for _, c := range eng.calls {
		for _, f := range strings.Fields(c) {
			if strings.TrimPrefix(strings.TrimPrefix(f, "dest="), "src=") == pullDomain {
				t.Errorf("the pull reached this box's own containers repository: %q", c)
			}
		}
		copied = copied || strings.HasPrefix(c, "Copy dest="+want+" ")
	}
	if !copied {
		t.Fatalf("no copy into %s: %v", want, eng.calls)
	}
}

func TestASecondPullOfTheSameSourceIsRefusedAndRecordsNothing(t *testing.T) {
	eng := &ctxPullRecorder{
		pullRecorder: pullRecorder{snaps: map[string][]restic.Snapshot{pullSrcLoc: {{ID: "aaa"}}}},
		hold:         make(chan struct{}),
	}
	svc, st, ps := pullFixture(t, eng)
	first := make(chan error, 1)
	go func() {
		_, err := svc.PullFromSource(context.Background(), ps)
		first <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for svc.claimPull(ps.ID) {
		svc.releasePull(ps.ID)
		if time.Now().After(deadline) {
			t.Fatal("the first pull never started")
		}
		time.Sleep(time.Millisecond)
	}

	if _, err := svc.PullFromSource(context.Background(), ps); !errors.Is(err, errPullBusy) {
		t.Fatalf("second pull: err = %v, want errPullBusy", err)
	}
	if got, _, _ := st.GetPullSource(ps.ID); got.LastPullOK.Valid {
		t.Fatalf("the refused pull recorded a result: %+v", got.LastPullOK)
	}
	close(eng.hold)
	if err := <-first; err != nil {
		t.Fatalf("first pull: %v", err)
	}
}

func TestPullNowFinishesAfterThePageIsLeft(t *testing.T) {
	eng := &ctxPullRecorder{pullRecorder: pullRecorder{snaps: map[string][]restic.Snapshot{pullSrcLoc: {{ID: "aaa"}}}}}
	svc, st, ps := pullFixture(t, eng)
	h := &Handler{cfg: svc.cfg, store: st, svc: svc}

	ctx, leave := context.WithCancel(context.Background())
	leave()
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/pull/sources/"+ps.ID+"/run", nil)
	req.SetPathValue("id", ps.ID)
	h.handleRunPullSource(httptest.NewRecorder(), req)

	deadline := time.Now().Add(5 * time.Second)
	for {
		got, _, err := st.GetPullSource(ps.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.LastPullOK.Valid {
			if !got.LastPullOK.Bool {
				t.Fatalf("the pull failed once the page was left: %q", got.LastPullError)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the pull never recorded a result")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

type leftoverPullRecorder struct {
	pullRecorder
	forgot map[string][]string
}

func (e *leftoverPullRecorder) Forget(_ context.Context, repo string, ids []string, _ bool, _ restic.Mode) error {
	e.forgot[repo] = append(e.forgot[repo], ids...)
	return nil
}

func TestAPullMovesTheSourcesEarlierSnapshotsOutOfTheDomainsRepository(t *testing.T) {
	eng := &leftoverPullRecorder{
		pullRecorder: pullRecorder{snaps: map[string][]restic.Snapshot{
			pullSrcLoc: {{ID: "a1"}, {ID: "a2"}, {ID: "b1", Original: "mine"}},
			pullDomain: {
				{ID: "own"},
				{ID: "moved", Original: "old"},
				{ID: "mutual", Original: "mine"},
				{ID: "landed", Original: "a1"},
				{ID: "notyet", Original: "a2"},
			},
		}},
		forgot: map[string][]string{},
	}
	svc, st, ps := pullFixture(t, eng)
	eng.snaps[pulledRepo(pullDomain, ps.ID)] = []restic.Snapshot{{ID: "c1", Original: "a1"}, {ID: "c2", Original: "mine"}}
	for _, snap := range []string{"old", "mine"} {
		id, err := st.StartRun("t", "backup")
		if err != nil {
			t.Fatal(err)
		}
		if err := st.FinishRun(id, "success", snap, 0, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetPullLeftoversMoved(ps.ID, false); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.PullFromSource(context.Background(), ps); err != nil {
		t.Fatal(err)
	}
	if got := eng.forgot[pullDomain]; len(got) != 1 || got[0] != "landed" {
		t.Fatalf("forgot %v in the domain's repository, want only the pulled snapshot whose copy landed", got)
	}
	if moved, _ := st.PullLeftoversMoved(ps.ID); !moved {
		t.Fatal("the move was not recorded")
	}

	eng.forgot = map[string][]string{}
	if _, err := svc.PullFromSource(context.Background(), ps); err != nil {
		t.Fatal(err)
	}
	if len(eng.forgot) != 0 {
		t.Fatalf("a second pull forgot %v", eng.forgot)
	}
}

func TestANewPullSourceHasNoEarlierPullsToMove(t *testing.T) {
	_, st, ps := pullFixture(t, &pullRecorder{})
	if moved, err := st.PullLeftoversMoved(ps.ID); err != nil || !moved {
		t.Fatalf("moved = %v, %v; a new source never wrote into the domain's repository", moved, err)
	}
}
