package api

import (
	"context"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// keepLastEngine previews a forget the way placementEngine's ForgetPolicy
// forgets, so what a preview lists can be held against what a pass removes.
type keepLastEngine struct {
	*placementEngine
}

func (e keepLastEngine) ForgetPreview(_ context.Context, repo string, p restic.RetentionPolicy, _ restic.Mode, tags ...string) ([]restic.ForgetGroup, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var mine []restic.Snapshot
	for _, sn := range e.snaps[filepath.ToSlash(repo)] {
		if slices.ContainsFunc(tags, func(t string) bool { return slices.Contains(sn.Tags, t) }) {
			mine = append(mine, sn)
		}
	}
	slices.SortStableFunc(mine, func(a, b restic.Snapshot) int { return strings.Compare(b.Time, a.Time) })
	n := min(p.KeepLast, len(mine))
	return []restic.ForgetGroup{{Tags: tags, Keep: mine[:n], Remove: mine[n:]}}, nil
}

// keepFixture is a mounted folder location holding the containers' own folder,
// with four backups of plex in it.
type keepFixture struct {
	*placementFixture
	location store.OffsiteTarget
	id       string
	there    string
}

func newKeepFixture(t *testing.T, keep store.RetentionKeep) keepFixture {
	t.Helper()
	f := newPlacementFixture(t)
	f.svc.engine = keepLastEngine{f.eng}
	local := f.keptLocation(keep)
	f.ownFoldersInto(local, "containers")
	there := f.root + "/zfs/bombvault/dxp480t/containers"
	f.makeRepo(there)
	f.hold(there,
		snap("a1", 100, "container:plex"),
		snap("a2", 200, "container:plex"),
		snap("a3", 300, "container:plex"),
		snap("a4", 400, "container:plex"))
	return keepFixture{placementFixture: f, location: local, id: "destination:" + local.ID, there: there}
}

func (f keepFixture) setCard(keep store.RetentionKeep) {
	f.t.Helper()
	if res := f.do(http.MethodPut, "/api/offsite/destinations/"+f.location.ID, map[string]any{"name": f.location.Name, "retention": keep}); res["ok"] != true {
		f.t.Fatalf("keep card save = %v", res)
	}
}

func (f keepFixture) idsAt(loc string) []string {
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	var ids []string
	for _, sn := range f.eng.snaps[filepath.ToSlash(loc)] {
		ids = append(ids, sn.ID)
	}
	slices.Sort(ids)
	return ids
}

func removalsOf(p LocationKeepPreview) []string {
	var ids []string
	for _, sec := range p.Sections {
		for _, repo := range sec.Preview.Repos {
			for _, item := range repo.Items {
				for _, sn := range item.Remove {
					ids = append(ids, sn.ID)
				}
			}
		}
	}
	slices.Sort(ids)
	return ids
}

func TestApplyingALocationsKeepRuleRemovesWhatThePreviewListedAndNothingElse(t *testing.T) {
	f := newKeepFixture(t, store.RetentionKeep{KeepLast: 3})
	localKeepLast(t, f.placementFixture, 1)
	cold := f.namedRepo("cold", "cold/containers")
	f.container("sonarr", cold.ID)
	coldAt := f.root + "/cold/containers"
	f.hold(coldAt, snap("c1", 100, "container:sonarr"), snap("c2", 200, "container:sonarr"))
	f.setCard(store.RetentionKeep{KeepLast: 1})

	preview, err := f.svc.PreviewLocationKeep(context.Background(), f.id)
	if err != nil {
		t.Fatalf("PreviewLocationKeep: %v", err)
	}
	listed := removalsOf(preview)
	if want := []string{"a1", "a2", "a3"}; !slices.Equal(listed, want) {
		t.Fatalf("the preview lists %v, want %v", listed, want)
	}

	if _, err := f.svc.ApplyLocationKeep(context.Background(), f.id); err != nil {
		t.Fatalf("ApplyLocationKeep: %v", err)
	}

	if got := f.idsAt(f.there); !slices.Equal(got, []string{"a4"}) {
		t.Fatalf("left at the location: %v, want only a4", got)
	}
	if got := f.idsAt(coldAt); !slices.Equal(got, []string{"c1", "c2"}) || len(forgetsAt(f.placementFixture, coldAt)) != 0 {
		t.Fatalf("a repository at another location was touched: %v, forgets %v", got, forgetsAt(f.placementFixture, coldAt))
	}
	runs, err := f.st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(runs, func(r store.Run) bool {
		return r.TargetID == "containers" && r.Kind == "prune" && r.Status == "success"
	}) {
		t.Fatalf("no prune run in the history: %+v", runs)
	}
}

func TestApplyingALocationsKeepRuleSparesAHeldEntryAndNamesIt(t *testing.T) {
	f := newKeepFixture(t, store.RetentionKeep{KeepLast: 1})
	plex := f.container("plex", "")
	f.container("radarr", "")
	f.hold(f.there,
		snap("a1", 100, "container:plex"), snap("a2", 200, "container:plex"),
		snap("r1", 100, "container:radarr"), snap("r2", 200, "container:radarr"))
	run, err := f.st.StartRun(plex.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.FinishRun(run, "success", "aa11", 1, store.NoteBackupEmpty+"; "+store.NoteOlderBackupsKept); err != nil {
		t.Fatal(err)
	}

	res, err := f.svc.ApplyLocationKeep(context.Background(), f.id)
	if err != nil {
		t.Fatalf("ApplyLocationKeep: %v", err)
	}

	if !slices.Contains(res.Paused, "container:plex") {
		t.Fatalf("paused = %v, want plex named", res.Paused)
	}
	if got := f.idsAt(f.there); !slices.Equal(got, []string{"a1", "a2", "r2"}) {
		t.Fatalf("left at the location: %v, want both of plex and the newest of radarr", got)
	}
}

func TestApplyingTheKeepRuleOfALocationThatRefusesDeletesRemovesNothing(t *testing.T) {
	f := newKeepFixture(t, store.RetentionKeep{KeepLast: 1})
	if res := f.do(http.MethodPut, "/api/offsite/destinations/"+f.location.ID, map[string]any{"name": f.location.Name, "immutable": true, "retention": store.RetentionKeep{KeepLast: 1}}); res["ok"] != true {
		t.Fatal(res)
	}

	if _, err := f.svc.ApplyLocationKeep(context.Background(), f.id); err == nil {
		t.Fatal("an append-only location was pruned without a word")
	}

	if got := forgetsAt(f.placementFixture, f.there); len(got) != 0 {
		t.Fatalf("forgets at an append-only location: %+v", got)
	}
	if got := f.idsAt(f.there); len(got) != 4 {
		t.Fatalf("left at the location: %v, want all four", got)
	}
}

func TestApplyingALocationsKeepRuleFollowsASectionsOwnRule(t *testing.T) {
	f := newKeepFixture(t, store.RetentionKeep{KeepLast: 1})
	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.SetOwnRetention(map[string]store.RetentionKeep{"containers": {KeepLast: 3}})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.ApplyLocationKeep(context.Background(), f.id); err != nil {
		t.Fatalf("ApplyLocationKeep: %v", err)
	}

	if got := f.idsAt(f.there); !slices.Equal(got, []string{"a2", "a3", "a4"}) {
		t.Fatalf("left at the location: %v, want the three the section's own rule keeps", got)
	}
}

func TestApplyingTheKeepRuleOfALocationWithoutBackupsSaysSo(t *testing.T) {
	f := newPlacementFixture(t)
	local := f.keptLocation(store.RetentionKeep{KeepLast: 1})
	f.tickDestination(local, "containers")
	id := "destination:" + local.ID

	preview, err := f.svc.PreviewLocationKeep(context.Background(), id)
	if err != nil {
		t.Fatalf("PreviewLocationKeep: %v", err)
	}
	if !preview.Empty {
		t.Fatalf("the preview of an empty location reads %+v", preview)
	}
	res, err := f.svc.ApplyLocationKeep(context.Background(), id)
	if err != nil {
		t.Fatalf("ApplyLocationKeep: %v", err)
	}
	if !res.Empty {
		t.Fatalf("applying at an empty location reads %+v", res)
	}
}

func TestApplyingALocationsKeepRuleNeedsTheOwnersOwnLogin(t *testing.T) {
	f := newKeepFixture(t, store.RetentionKeep{KeepLast: 1})
	routes := []string{
		"GET /api/storage/locations/" + f.id + "/keep/preview",
		"POST /api/storage/locations/" + f.id + "/keep/apply",
	}
	refused := func(code string) {
		t.Helper()
		for _, r := range routes {
			method, path, _ := strings.Cut(r, " ")
			got, out := f.call(method, path, nil)
			if got != http.StatusForbidden || out["code"] != code {
				t.Errorf("%s: %d %v, want 403 %s", r, got, out, code)
			}
		}
	}

	refused(codeNoPassword)
	f.withPassword("a password of the owner")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	f.session = listedPhoneSession(t, f.st, f.svc.cfg.AppKey, settings.AuthPasswordHash, settings.SessionEpoch)
	refused(codeOwnLoginNeeded)
	if got := forgetsAt(f.placementFixture, f.there); len(got) != 0 {
		t.Fatalf("forgotten without the owner's login: %+v", got)
	}

	f.signIn()
	if res := f.do(http.MethodPost, "/api/storage/locations/"+f.id+"/keep/apply", nil); res["ok"] != true {
		t.Fatalf("apply signed in = %v", res)
	}
	if got := f.idsAt(f.there); !slices.Equal(got, []string{"a4"}) {
		t.Fatalf("left at the location: %v, want only a4", got)
	}
}
