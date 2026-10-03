package api

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

const wrongKeyMsg = "restic cat failed: Fatal: config or key /repo is damaged: wrong password or no key found"

func TestEnsureRepoRefusesARepositoryItsKeyDoesNotOpen(t *testing.T) {
	f := newPlacementFixture(t)
	loc := "b2:bkt:taken"
	f.eng.opens[loc] = false
	f.eng.openErr[loc] = errors.New(wrongKeyMsg)
	err := f.svc.EnsureRepo(context.Background(), loc, restic.Mode{Encrypted: true, Password: "pw"})
	if !errors.Is(err, errRepoWrongKey) {
		t.Fatalf("EnsureRepo = %v, want the wrong-key refusal", err)
	}
	if slices.Contains(f.eng.ensured, loc) {
		t.Fatal("a repository the key does not open was initialised over")
	}
}

func TestEnsureRepoFailsWhenInitFails(t *testing.T) {
	f := newPlacementFixture(t)
	loc := "b2:bkt:half"
	f.eng.opens[loc] = false
	f.eng.initErr[loc] = errors.New("Fatal: create key in repository at b2:bkt:half failed: config file already exists")
	if err := f.svc.EnsureRepo(context.Background(), loc, restic.Mode{Encrypted: true, Password: "pw"}); err == nil {
		t.Fatal("EnsureRepo = nil after restic init failed")
	}
}

func TestCreatingADirectRepositoryRefusesALocationItsKeyDoesNotOpen(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("vms", "B2", "b2:bkt:vms")
	loc := "b2:bkt:vms-direct"
	f.eng.opens[loc] = false
	f.eng.openErr[loc] = errors.New(wrongKeyMsg)
	res := f.do("POST", "/api/repos", map[string]any{"name": "", "repo": loc, "companionOf": target.ID})
	if res["ok"] != false || res["code"] != "repo-wrong-key" {
		t.Fatalf("create = %v", res)
	}
	if rows, err := f.st.ListNamedRepos(); err != nil || len(rows) != 0 {
		t.Fatalf("a refused create wrote %v, %v", rows, err)
	}
}

func TestCreatingADirectRepositoryRefusesOneThatStillDoesNotOpen(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("vms", "B2", "b2:bkt:vms")
	loc := "b2:bkt:vms-direct"
	f.eng.opens[loc] = false
	f.eng.initStaysShut[loc] = true
	res := f.do("POST", "/api/repos", map[string]any{"name": "", "repo": loc, "companionOf": target.ID})
	if res["ok"] != false || res["code"] != "repo-unopened" {
		t.Fatalf("create = %v", res)
	}
	if rows, err := f.st.ListNamedRepos(); err != nil || len(rows) != 0 {
		t.Fatalf("a refused create wrote %v, %v", rows, err)
	}
}
