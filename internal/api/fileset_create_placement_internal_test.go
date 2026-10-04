package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func createDocs(t *testing.T, f *placementFixture, extra map[string]any) map[string]any {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(filepath.FromSlash(f.root), "docs"), 0o750); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"name": "docs", "path": "docs"}
	for k, v := range extra {
		body[k] = v
	}
	return f.do(http.MethodPost, "/api/files/sets", body)
}

func TestAFileSetCreatedWithoutARepositoryStaysOpen(t *testing.T) {
	f := newPlacementFixture(t)
	res := createDocs(t, f, nil)
	if res["ok"] != true {
		t.Fatalf("POST = %v", res)
	}
	if got := f.home(store.ItemRef{Domain: "files", Key: res["id"].(string)}); got.Repo != "" || got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want open", got)
	}
}

// The new folder set window sends the empty string for the domain repository,
// so an empty repo is a choice and the set does not follow the default away.
func TestAFileSetCreatedOnTheDomainRepositoryIsChosen(t *testing.T) {
	f := newPlacementFixture(t)
	res := createDocs(t, f, map[string]any{"repo": ""})
	if res["ok"] != true {
		t.Fatalf("POST = %v", res)
	}
	if got := f.home(store.ItemRef{Domain: "files", Key: res["id"].(string)}); got.Repo != "" || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want the domain repository chosen", got)
	}
}

func TestAFileSetCreatedWithARepositoryIsChosen(t *testing.T) {
	f := newPlacementFixture(t)
	cold := f.namedRepo("Cold", "backups/cold")
	res := createDocs(t, f, map[string]any{"repo": cold.ID})
	if res["ok"] != true {
		t.Fatalf("POST = %v", res)
	}
	if got := f.home(store.ItemRef{Domain: "files", Key: res["id"].(string)}); got.Repo != cold.ID || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want the chosen repository", got)
	}
}

func TestAFileSetCreatedWithCopiesGetsItsRule(t *testing.T) {
	f := newPlacementFixture(t)
	res := createDocs(t, f, map[string]any{"copies": map[string]any{"skip": []string{store.SkipAll}}})
	if res["ok"] != true {
		t.Fatalf("POST = %v", res)
	}
	rule, found, err := f.st.CopyRuleFor("files", "fileset:docs")
	if err != nil || !found || !skipsEverything(rule.Skip) {
		t.Fatalf("rule = %+v, %v, %v, want [*]", rule, found, err)
	}
}

func TestAFileSetOnARemoteRepositoryTakesNoCopies(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	f.target("files", "B2", "b2:bucket/files")
	res := createDocs(t, f, map[string]any{"repo": box.ID, "copies": map[string]any{"skip": []string{}}})
	if res["code"] != "copies-not-allowed" {
		t.Fatalf("POST = %v, want copies-not-allowed", res)
	}
	if _, err := f.st.GetFileSetByName("docs"); err == nil {
		t.Fatal("the set was created although its copies were refused")
	}
}

func TestAFileSetOpenUnderARemoteDefaultTakesNoCopies(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	f.target("files", "B2", "b2:bucket/files")
	f.setDefault("files", box.ID)
	res := createDocs(t, f, map[string]any{"copies": map[string]any{"skip": []string{}}})
	if res["code"] != "copies-not-allowed" {
		t.Fatalf("POST = %v, want copies-not-allowed", res)
	}
	if _, err := f.st.GetFileSetByName("docs"); err == nil {
		t.Fatal("the set was created although its copies were refused")
	}
}
