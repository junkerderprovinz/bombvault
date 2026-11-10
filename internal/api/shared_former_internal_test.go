package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func formerOf(t *testing.T, in *instance) []store.SharedFormer {
	t.Helper()
	sr, _, err := in.st.SharedRepoAt("shared/shared")
	if err != nil {
		t.Fatal(err)
	}
	former, err := in.st.ListSharedFormer(sr.ID)
	if err != nil {
		t.Fatal(err)
	}
	return former
}

func TestAMemberTakenOutOrLeavingIsKeptAsAFormerMember(t *testing.T) {
	for name, out := range map[string]func(nas, tower *instance) error{
		"taken out": func(nas, tower *instance) error {
			_, err := nas.svc.RemoveSharedMember(context.Background(), tower.id(t))
			return err
		},
		"leaving": func(nas, tower *instance) error { return tower.svc.LeaveSharedRepo(context.Background(), nas.id(t)) },
	} {
		t.Run(name, func(t *testing.T) {
			nas, tower, _ := sharedPair(t)
			setUpShared(t, nas)
			joined(t, nas, tower)
			if err := out(nas, tower); err != nil {
				t.Fatal(err)
			}
			if got := formerOf(t, nas); len(got) != 1 || got[0].Host != tower.svc.resticHost() || got[0].Name != "tower" || got[0].LeftAt == 0 {
				t.Fatalf("former %+v, want tower", got)
			}
		})
	}
}

func TestAFormerMemberThatJoinsAgainIsAMemberOnly(t *testing.T) {
	nas, tower, _ := sharedPair(t)
	setUpShared(t, nas)
	joined(t, nas, tower)
	if _, err := nas.svc.RemoveSharedMember(context.Background(), tower.id(t)); err != nil {
		t.Fatal(err)
	}
	waitForJoin(t, tower, store.SharedJoinOut, nas, store.SharedJoinRemoved)
	joined(t, nas, tower)
	if got := formerOf(t, nas); len(got) != 0 {
		t.Fatalf("former %+v, want none once tower is back", got)
	}
}

// formerRepo is an owner whose shared repository holds the snapshots of
// Tower, which left, and of Cube, which is still in.
func formerRepo(t *testing.T) (*Service, *upkeepEngine, store.SharedRepo) {
	t.Helper()
	eng := &upkeepEngine{snaps: []restic.Snapshot{
		snapAt("t1", towerHost, "container:plex", 1), snapAt("t2", towerHost, "folders:docs", 2), snapAt("c1", cubeHost, "container:plex", 1),
	}}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, cube)
	if err := s.store.NoteSharedFormer(sr.ID, store.SharedFormer{Host: towerHost, Name: "Tower", LeftAt: 100}); err != nil {
		t.Fatal(err)
	}
	return s, eng, sr
}

func waitForUpkeepTurn(t *testing.T, s *Service) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for s.sharedUpkeep.running() {
		if time.Now().After(deadline) {
			t.Fatal("the forget never finished")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestTheOwnerForgetsWhatAFormerMemberLeftOnceItsNameIsTyped(t *testing.T) {
	s, eng, sr := formerRepo(t)

	if err := s.startForgetFormer(sr.ID, towerHost, "Tower"); err != nil {
		t.Fatal(err)
	}
	waitForUpkeepTurn(t, s)
	if got := eng.left(); !slices.Equal(got, []string{"c1"}) {
		t.Fatalf("left %v, want only Cube's snapshot", got)
	}
	if len(eng.forgets) != 1 || len(eng.prunes) != 0 {
		t.Fatalf("forgets %v, prunes %d; want one forget and the prune left to the window", eng.forgets, len(eng.prunes))
	}
	if former, err := s.store.ListSharedFormer(sr.ID); err != nil || len(former) != 0 {
		t.Fatalf("former %+v, %v; want Tower gone", former, err)
	}
}

func TestForgettingAFormerMemberNeedsItsNameAndAHostThatLeft(t *testing.T) {
	s, eng, sr := formerRepo(t)
	for name, c := range map[string]struct {
		host, typed string
		want        error
	}{
		"another name":    {towerHost, "tower", errFormerName},
		"no name":         {towerHost, "", errFormerName},
		"a member's host": {cubeHost, "Cube", errNoSuchFormer},
		"an unknown host": {"bombvault", "bombvault", errNoSuchFormer},
	} {
		if err := s.startForgetFormer(sr.ID, c.host, c.typed); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	if err := s.startForgetFormer("elsewhere", towerHost, "Tower"); !errors.Is(err, errNotSharedOwner) {
		t.Errorf("a repository this server does not hold: err = %v", err)
	}
	if len(eng.forgets) != 0 {
		t.Fatalf("forgot %v", eng.forgets)
	}
}

func TestAFormerMemberThatCameBackKeepsItsBackups(t *testing.T) {
	s, eng, sr := formerRepo(t)
	sr.Members = append(sr.Members, tower)
	if _, err := s.store.PutSharedRepo(sr); err != nil {
		t.Fatal(err)
	}
	if err := s.startForgetFormer(sr.ID, towerHost, "Tower"); !errors.Is(err, errNoSuchFormer) {
		t.Fatalf("err = %v, want errNoSuchFormer", err)
	}
	if len(eng.forgets) != 0 {
		t.Fatalf("forgot %v", eng.forgets)
	}
}

func TestAForgetThatFailsSaysWhyOnTheFormerMember(t *testing.T) {
	s, eng, sr := formerRepo(t)
	s.engine = failingForgetEngine{eng}
	if err := s.startForgetFormer(sr.ID, towerHost, "Tower"); err != nil {
		t.Fatal(err)
	}
	waitForUpkeepTurn(t, s)
	former, err := s.store.ListSharedFormer(sr.ID)
	if err != nil || len(former) != 1 || former[0].ForgetError == "" {
		t.Fatalf("former %+v, %v; want Tower with the reason", former, err)
	}
}

type failingForgetEngine struct{ *upkeepEngine }

func (failingForgetEngine) Forget(context.Context, string, []string, bool, restic.Mode) error {
	return errors.New("Fatal: unable to save snapshot: disk full")
}

func TestTheOwnersCardOffersToForgetAFormerMember(t *testing.T) {
	nas, tower, _ := sharedPair(t)
	setUpShared(t, nas)
	joined(t, nas, tower)
	if _, err := nas.svc.RemoveSharedMember(context.Background(), tower.id(t)); err != nil {
		t.Fatal(err)
	}
	v, err := nas.svc.sharedServerView(context.Background())
	if err != nil || v.ID == "" || len(v.Former) != 1 || v.Former[0].Name != "tower" || v.Former[0].Host != tower.svc.resticHost() {
		t.Fatalf("view %+v, %v; want tower among the former members", v, err)
	}
	code, out := nas.do(t, http.MethodPost, "/api/shared-repos/"+v.ID+"/former/"+v.Former[0].Host+"/forget", map[string]any{"name": "nobody"})
	if code != http.StatusOK || out["ok"] != false {
		t.Fatalf("a wrong name: %d %v", code, out)
	}
}

func TestTheBackupsOfAFormerMemberAgeByTheOwnersRules(t *testing.T) {
	eng := &upkeepEngine{snaps: []restic.Snapshot{snapAt("t1", towerHost, "container:plex", 1), snapAt("t2", towerHost, "container:plex", 2)}}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s)
	if err := s.store.NoteSharedFormer(sr.ID, store.SharedFormer{Host: towerHost, Name: "Tower", LeftAt: 100}); err != nil {
		t.Fatal(err)
	}

	r := forgetPass(t, s, sr, &fakeMembers{unreachable: map[string]bool{towerHost: true}})
	if got := eng.left(); !slices.Equal(got, []string{"t2"}) || len(r.Unknown) != 0 || r.Error != "" {
		t.Fatalf("left %v, report %+v; want Tower's last backup kept and Tower not unknown", got, r)
	}
}
