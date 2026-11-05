package api

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func localKeepLast(t *testing.T, f *placementFixture, keepLast int) store.Settings {
	t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = keepLast
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	return settings
}

func TestEveryRepositoryAgesByTheLocalPolicy(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	settings := localKeepLast(t, f, 3)
	for _, ref := range []domainRepoRef{ownRef(f.domainPath("containers")), namedRef(f.root+"/nas", nas)} {
		if got := f.svc.retentionPolicyForRef(settings, "containers", ref); got != (restic.RetentionPolicy{KeepLast: 3}) {
			t.Errorf("retentionPolicyForRef(%s) = %+v, want the local keep-last 3", ref.Loc, got)
		}
	}
}

func TestAnItemOnANamedRepositoryAgesByTheLocalPolicy(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	settings := localKeepLast(t, f, 3)
	f.svc.applyRetention(context.Background(), f.root+"/nas", settings, restic.Mode{}, tagIdentity("container:nginx"), "containers", anomalyScope{})
	want := []forgetCall{{Repo: f.root + "/nas", Tags: []string{"container:nginx"}, Policy: restic.RetentionPolicy{KeepLast: 3}, Prune: true}}
	if !reflect.DeepEqual(f.eng.forgets, want) {
		t.Fatalf("forgets = %+v, want %+v", f.eng.forgets, want)
	}
}

func TestAManualPruneAgesEveryRepositoryOfTheDomain(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	localKeepLast(t, f, 3)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:plex"))
	f.hold(f.root+"/nas", snap("b1", 100, "container:nginx"))
	if _, err := f.svc.pruneDomain(context.Background(), "containers", "local", false); err != nil {
		t.Fatalf("pruneDomain: %v", err)
	}
	var aged []string
	for _, c := range f.eng.forgets {
		if c.Policy != (restic.RetentionPolicy{KeepLast: 3}) {
			t.Errorf("%s forgot with %+v, want the local keep-last 3", c.Repo, c.Policy)
		}
		aged = append(aged, c.Repo+" "+strings.Join(c.Tags, ","))
	}
	want := []string{f.domainPath("containers") + " container:plex", f.root + "/nas container:nginx"}
	if slices.Sort(aged); !slices.Equal(aged, want) {
		t.Fatalf("aged %v, want %v", aged, want)
	}
	if len(f.eng.prunes) != 2 {
		t.Fatalf("prunes = %v, want both repositories", f.eng.prunes)
	}
}

func TestTheBatchedPruneRunsOnlyWhenARepositoryAges(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	f.svc.PruneAfterBulk(context.Background(), "containers")
	if len(f.eng.prunes) != 0 {
		t.Fatalf("without a policy the batched prune ran on %v", f.eng.prunes)
	}
	localKeepLast(t, f, 3)
	f.svc.PruneAfterBulk(context.Background(), "containers")
	if len(f.eng.prunes) != 2 {
		t.Fatalf("with a policy the batched prune reached %v, want both repositories", f.eng.prunes)
	}
}

// archiveBesideAKeptOwnFolder is a section whose own folder lies in a location
// with a card of 7 days, and one container of it on a named repository.
func archiveBesideAKeptOwnFolder(t *testing.T) (f *placementFixture, archive store.OffsiteTarget, loc string) {
	t.Helper()
	f = newPlacementFixture(t)
	f.ownFoldersInto(f.keptLocation(store.RetentionKeep{KeepDaily: 7}), "containers")
	archive = f.namedRepo("Archive", "archive")
	f.container("web", archive.ID)
	return f, archive, f.root + "/archive"
}

func TestANamedRepositoryAgesByWhatItsPageShowsAndNotByTheOwnFoldersCard(t *testing.T) {
	f, archive, loc := archiveBesideAKeptOwnFolder(t)
	aged := func(settings store.Settings) []forgetCall {
		f.eng.forgets = nil
		f.svc.applyRetention(context.Background(), loc, settings, restic.Mode{}, tagIdentity("container:web"), "containers", anomalyScope{})
		return f.eng.forgets
	}
	shown := func() store.RetentionKeep {
		t.Helper()
		return sectionFor(t, locationByID(t, f.svc, "repo:"+archive.ID), "containers", useHome).Retention
	}

	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got := aged(settings); len(got) != 0 {
		t.Errorf("forgets = %+v, want none: the shared rule keeps everything", got)
	}
	if got := shown(); got != (store.RetentionKeep{}) {
		t.Errorf("the repository's page shows %+v, want the shared rule that keeps everything", got)
	}

	settings = localKeepLast(t, f, 3)
	want := []forgetCall{{Repo: loc, Tags: []string{"container:web"}, Policy: restic.RetentionPolicy{KeepLast: 3}, Prune: true}}
	if got := aged(settings); !reflect.DeepEqual(got, want) {
		t.Errorf("forgets = %+v, want %+v", got, want)
	}
	if got := shown(); got != (store.RetentionKeep{KeepLast: 3}) {
		t.Errorf("the repository's page shows %+v, want the shared keep-last 3", got)
	}
	if got := f.svc.retentionPolicyForRef(settings, "containers", ownRef(f.domainPath("containers"))); got != (restic.RetentionPolicy{KeepDaily: 7}) {
		t.Errorf("the own folder ages by %+v, want the card's 7 days", got)
	}

	settings.SetOwnRetention(map[string]store.RetentionKeep{"containers": {KeepLast: 5}})
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	want[0].Policy = restic.RetentionPolicy{KeepLast: 5}
	if got := aged(settings); !reflect.DeepEqual(got, want) {
		t.Errorf("forgets = %+v, want the section's own keep-last 5", got)
	}
	if got := shown(); got != (store.RetentionKeep{KeepLast: 5}) {
		t.Errorf("the repository's page shows %+v, want the section's own keep-last 5", got)
	}
}

func TestThePreviewWorksOutANamedRepositoryByItsOwnRule(t *testing.T) {
	f, archive, loc := archiveBesideAKeptOwnFolder(t)
	f.hold(loc, snap("b1", 100, "container:web"))

	preview, err := f.svc.PreviewRetention(context.Background(), "containers", "local")
	if err != nil {
		t.Fatal(err)
	}
	row := slices.IndexFunc(preview.Repos, func(r RetentionPreviewRepo) bool { return strings.Contains(r.Name, archive.Name) })
	if row < 0 || len(preview.Repos[row].Items) != 0 {
		t.Errorf("repos = %+v, want %s listed with nothing to age, as its rule keeps everything", preview.Repos, archive.Name)
	}

	localKeepLast(t, f, 3)
	preview, err = f.svc.PreviewRetention(context.Background(), "containers", "local")
	if err != nil {
		t.Fatal(err)
	}
	row = slices.IndexFunc(preview.Repos, func(r RetentionPreviewRepo) bool { return strings.Contains(r.Name, archive.Name) })
	if row < 0 || len(preview.Repos[row].Items) != 1 {
		t.Errorf("repos = %+v, want %s aged by the shared keep-last 3", preview.Repos, archive.Name)
	}
}

func TestAnAssistantsWindowCountsANamedRepositorysRule(t *testing.T) {
	f, _, _ := archiveBesideAKeptOwnFolder(t)
	settings := localKeepLast(t, f, 3)

	if n, err := f.h.mcpKeepLast(settings, "containers"); err != nil || n != 3 {
		t.Fatalf("mcpKeepLast = %d, %v; want the named repository's keep-last 3", n, err)
	}
	if n, err := f.h.mcpKeepLast(settings, "vms"); err != nil || n != 3 {
		t.Fatalf("mcpKeepLast(vms) = %d, %v; want its own folder's keep-last 3", n, err)
	}
}
