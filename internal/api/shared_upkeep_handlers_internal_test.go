package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func upkeepOf(t *testing.T, in *instance) []map[string]any {
	t.Helper()
	code, out := in.do(t, http.MethodGet, "/api/shared-repos/upkeep", nil)
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("GET upkeep: %d %v", code, out)
	}
	raw, _ := out["repos"].([]any)
	repos := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		m, _ := r.(map[string]any)
		repos = append(repos, m)
	}
	return repos
}

func TestTheOwnerSeesTheRulesAndTheLastPassesOfItsSharedRepository(t *testing.T) {
	in := newInstance(t, "nas", strings.Repeat("a1", 32))
	sr, err := in.st.PutSharedRepo(store.SharedRepo{Repo: "backups/shared", Owner: in.id(t), Members: []store.SharedMember{tower, cube}})
	if err != nil {
		t.Fatal(err)
	}
	if err := in.st.NoteSharedSpace(sr.ID, store.SharedSpace{At: 300, Total: 1200, Hosts: []store.SharedHostSpace{{Host: towerHost, Name: "Tower", Bytes: 1000}, {Host: cubeHost, Name: "Cube", Bytes: 900}}}); err != nil {
		t.Fatal(err)
	}

	repos := upkeepOf(t, in)
	if len(repos) != 1 {
		t.Fatalf("repos %v, want one", repos)
	}
	r := repos[0]
	rules, _ := r["rules"].(map[string]any)
	space, _ := r["space"].(map[string]any)
	members, _ := r["members"].([]any)
	if r["id"] != sr.ID || r["role"] != "owner" || len(members) != 2 || rules["pruneCadence"] != "weekly sun 03:00" || space["total"] != float64(1200) {
		t.Fatalf("repo %v", r)
	}
	if r["nextWindow"] == nil || r["running"] != false {
		t.Fatalf("repo %v, want the next window and no pass running", r)
	}
}

func TestAMemberSeesWhatTheOwnerMeasured(t *testing.T) {
	in := newInstance(t, "tower", strings.Repeat("b2", 32))
	if _, err := in.st.PutSharedRepo(store.SharedRepo{Repo: "rest:http://nas:8001/shared", Owner: "nas-id", Members: []store.SharedMember{tower}}); err != nil {
		t.Fatal(err)
	}
	if err := in.st.SetSharedSpaceFrom("nas-id", store.SharedSpace{At: 300, Total: 1200}); err != nil {
		t.Fatal(err)
	}

	repos := upkeepOf(t, in)
	space, _ := repos[0]["space"].(map[string]any)
	if len(repos) != 1 || repos[0]["role"] != "member" || space["total"] != float64(1200) || repos[0]["rules"] != nil {
		t.Fatalf("repos %v, want the member's view with the owner's numbers and no rules", repos)
	}
}

func TestTheOwnerSavesTheRulesOfItsSharedRepository(t *testing.T) {
	in := newInstance(t, "nas", strings.Repeat("a1", 32))
	sr, err := in.st.PutSharedRepo(store.SharedRepo{Repo: "backups/shared", Owner: in.id(t)})
	if err != nil {
		t.Fatal(err)
	}
	rules := map[string]any{
		"keepSet": true, "keep": map[string]any{"keepDaily": 7, "keepWeekly": 4},
		"pruneCadence": "weekly sat 01:30", "pruneHours": 4, "maxUnused": 10, "checkSubset": 2,
	}
	if code, out := in.do(t, http.MethodPut, "/api/shared-repos/"+sr.ID+"/upkeep", rules); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("PUT: %d %v", code, out)
	}
	u, err := in.st.SharedUpkeepOf(sr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !u.KeepSet || u.Keep.KeepDaily != 7 || u.PruneCadence != "weekly sat 01:30" || u.PruneHours != 4 || u.MaxUnused != 10 || u.CheckSubset != 2 {
		t.Fatalf("saved %+v", u)
	}

	for name, change := range map[string]map[string]any{
		"cadence":   {"pruneCadence": "sometimes"},
		"no window": {"pruneCadence": "off"},
		"hours":     {"pruneHours": 0},
		"unused":    {"maxUnused": 101},
		"subset":    {"checkSubset": 0},
		"keep":      {"keep": map[string]any{"keepLast": -1}},
	} {
		bad := map[string]any{}
		for k, v := range rules {
			bad[k] = v
		}
		for k, v := range change {
			bad[k] = v
		}
		if _, out := in.do(t, http.MethodPut, "/api/shared-repos/"+sr.ID+"/upkeep", bad); out["ok"] != false {
			t.Errorf("%s: saved %v", name, bad)
		}
	}
}

func TestAMemberSetsNoRulesForTheRepositoryItBacksUpInto(t *testing.T) {
	in := newInstance(t, "tower", strings.Repeat("b2", 32))
	sr, err := in.st.PutSharedRepo(store.SharedRepo{Repo: "rest:http://nas:8001/shared", Owner: "nas-id"})
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"pruneCadence": "weekly sat 01:30", "pruneHours": 4, "maxUnused": 10, "checkSubset": 2}
	if _, out := in.do(t, http.MethodPut, "/api/shared-repos/"+sr.ID+"/upkeep", body); out["ok"] != false {
		t.Fatal("a member saved rules for its owner's repository")
	}
	if _, out := in.do(t, http.MethodPost, "/api/shared-repos/"+sr.ID+"/upkeep/run", nil); out["ok"] != false {
		t.Fatal("a member started the upkeep of its owner's repository")
	}
}

func TestRunningTheUpkeepNowAnswersAtOnceAndPrunesInTheBackground(t *testing.T) {
	pruning, done := make(chan struct{}), make(chan struct{})
	eng := &upkeepEngine{onPrune: func() {
		close(pruning)
		<-done
	}}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower)

	if err := s.startSharedUpkeep(sr.ID, &fakeMembers{}); err != nil {
		t.Fatal(err)
	}
	<-pruning
	if !s.sharedUpkeep.running() {
		t.Fatal("no pass is said to run while it prunes")
	}
	if err := s.startSharedUpkeep(sr.ID, &fakeMembers{}); !errors.Is(err, errSharedUpkeepBusy) {
		t.Fatalf("err = %v, want errSharedUpkeepBusy", err)
	}
	close(done)
	deadline := time.Now().Add(10 * time.Second)
	for s.sharedUpkeep.running() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if s.sharedUpkeep.running() || len(eng.prunes) != 1 {
		t.Fatalf("running %v after %d prunes, want done after one", s.sharedUpkeep.running(), len(eng.prunes))
	}
	if err := s.runSharedUpkeep(context.Background(), &fakeMembers{}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

// stallingPruneEngine prunes until its context ends, as a prune of hours
// would look to a shutdown.
type stallingPruneEngine struct {
	*upkeepEngine
	entered chan struct{}
}

func (e stallingPruneEngine) Prune(ctx context.Context, _ string, _ restic.Mode) error {
	close(e.entered)
	<-ctx.Done()
	return ctx.Err()
}

func TestAnUpkeepStartedByHandEndsWithTheService(t *testing.T) {
	eng := stallingPruneEngine{upkeepEngine: &upkeepEngine{}, entered: make(chan struct{})}
	s := upkeepService(t, eng.upkeepEngine)
	s.engine = eng
	sr := holdSharedRepo(t, s, tower)

	if err := s.startSharedUpkeep(sr.ID, &fakeMembers{}); err != nil {
		t.Fatal(err)
	}
	<-eng.entered
	s.BeginShutdown()
	deadline := time.Now().Add(10 * time.Second)
	for s.sharedUpkeep.running() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if s.sharedUpkeep.running() {
		t.Fatal("the prune went on after the service began to shut down")
	}
}
