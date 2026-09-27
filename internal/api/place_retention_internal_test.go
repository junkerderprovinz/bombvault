package api

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const b2Containers = "s3:https://s3.example.com/bv/containers"

// nasPlace is a place that is itself a repository, the shape a named
// repository on a NAS share becomes.
func nasPlace(keepLast int) store.Place {
	return store.Place{
		Name: "NAS", Provider: "synology", Kind: "local", Base: "nas",
		Folders: map[string]string{}, RetentionKeepLast: keepLast, Enabled: true,
	}
}

// b2Place is a bucket with a folder for the containers.
func b2Place(keepLast int) store.Place {
	return store.Place{
		Name: "B2", Provider: "b2", Kind: "s3", Base: "s3:https://s3.example.com/bv",
		Folders: map[string]string{"containers": "containers"}, OffPremises: true, RetentionKeepLast: keepLast, Enabled: true,
	}
}

func TestARowAtAPlaceAgesByTheRulesItsPlaceMirrors(t *testing.T) {
	f := newPlacementFixture(t)
	settings := localKeepLast(t, f, 3)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	f.linkRow(nas.ID, f.storePlace(nasPlace(5)), "", "")

	f.svc.applyRetention(context.Background(), f.root+"/nas", settings, restic.Mode{}, tagIdentity("container:nginx"), "containers", anomalyScope{})
	want := []forgetCall{{Repo: f.root + "/nas", Tags: []string{"container:nginx"}, Policy: restic.RetentionPolicy{KeepLast: 5}, Prune: true}}
	if !reflect.DeepEqual(f.eng.forgets, want) {
		t.Fatalf("forgets = %+v, want %+v", f.eng.forgets, want)
	}
}

func TestTheDomainPathAgesByItsHomePlace(t *testing.T) {
	f := newPlacementFixture(t)
	settings := localKeepLast(t, f, 3)
	f.storePlace(store.Place{
		Name: "Unraid", Provider: "unraid-folder", Kind: "local", Base: "backups",
		Folders: map[string]string{"containers": "containers", "vms": "vms", "files": "files"}, RetentionKeepLast: 7, Enabled: true,
	}, "containers")

	if got := f.svc.retentionPolicyForRef(settings, "containers", ownRef(f.domainPath("containers"))); got != (restic.RetentionPolicy{KeepLast: 7}) {
		t.Errorf("the domain path at Unraid = %+v, want the place's keep-last 7", got)
	}
	if got := f.svc.retentionPolicyForRef(settings, "vms", ownRef(f.domainPath("vms"))); got != (restic.RetentionPolicy{KeepLast: 3}) {
		t.Errorf("a domain path without a home place = %+v, want the local keep-last 3", got)
	}
	f.svc.applyRetention(context.Background(), f.domainPath("containers"), settings, restic.Mode{}, tagIdentity("container:plex"), "containers", anomalyScope{})
	want := []forgetCall{{Repo: f.domainPath("containers"), Tags: []string{"container:plex"}, Policy: restic.RetentionPolicy{KeepLast: 7}, Prune: true}}
	if !reflect.DeepEqual(f.eng.forgets, want) {
		t.Fatalf("forgets = %+v, want %+v", f.eng.forgets, want)
	}
}

func TestADomainPathWhoseHomePlaceCannotBeReadForgetsNothing(t *testing.T) {
	f := newPlacementFixture(t)
	settings := localKeepLast(t, f, 3)
	f.storePlace(store.Place{
		Name: "Unraid", Provider: "unraid-folder", Kind: "local", Base: "backups",
		Folders: map[string]string{"containers": "containers"}, RetentionKeepLast: 7, Enabled: true,
	}, "containers")
	if _, err := f.db.Exec(`UPDATE storage_places SET retention_keep_last = 'not a number'`); err != nil {
		t.Fatal(err)
	}

	if got := f.svc.retentionPolicyForRef(settings, "containers", ownRef(f.domainPath("containers"))); got.Any() {
		t.Errorf("a home place that cannot be read = %+v, want nothing forgotten rather than the local keep-last 3", got)
	}
}

func TestAPlacedRowWithoutRulesKeepsEverythingUntilItIsDetached(t *testing.T) {
	f := newPlacementFixture(t)
	settings := localKeepLast(t, f, 3)
	nas := f.namedRepo("NAS", "nas")
	f.linkRow(nas.ID, f.storePlace(nasPlace(0)), "", "")

	placed, err := f.st.GetNamedRepo(nas.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.svc.retentionPolicyForRef(settings, "containers", namedRef(f.root+"/nas", placed)); got.Any() {
		t.Errorf("a place that keeps everything = %+v, want nothing forgotten", got)
	}

	f.unlinkRow(nas.ID)
	detached, err := f.st.GetNamedRepo(nas.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detached.PlaceID != "" || rowRetentionPolicy(detached).Any() {
		t.Fatalf("detached row = %+v, want no place and no rules of its own", detached)
	}
	if got := f.svc.retentionPolicyForRef(settings, "containers", namedRef(f.root+"/nas", detached)); got != (restic.RetentionPolicy{KeepLast: 3}) {
		t.Errorf("a detached row with retention 0 = %+v, want the local keep-last 3", got)
	}
}

func TestADirectRepositoryAtAPlaceKeepsItsDirectRules(t *testing.T) {
	f := newPlacementFixture(t)
	settings := localKeepLast(t, f, 3)
	b2 := f.target("containers", "B2", b2Containers)
	direct := f.direct(b2)
	place := f.storePlace(b2Place(4))
	f.linkRow(b2.ID, place, "containers", "")
	f.linkRow(direct.ID, place, "containers", "-direct")

	row, err := f.st.GetNamedRepo(direct.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := restic.RetentionPolicy{KeepLast: 4, Direct: true}
	if got := f.svc.retentionPolicyForRef(settings, "containers", namedRef(row.Repo, row)); got != want {
		t.Fatalf("direct repository at B2 = %+v, want %+v", got, want)
	}
}

func TestTheBatchedPruneRunsWhenOnlyAPlaceAges(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	f.linkRow(nas.ID, f.storePlace(nasPlace(5)), "", "")

	f.svc.PruneAfterBulk(context.Background(), "containers")
	if len(f.eng.prunes) != 2 {
		t.Fatalf("the batched prune reached %v, want the domain path and the NAS", f.eng.prunes)
	}
}

func TestChangingAPlacesRulesReopensTheIdleGate(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", b2Containers)
	place := f.storePlace(b2Place(4))
	f.linkRow(b2.ID, place, "containers", "")
	rev := func() string {
		t.Helper()
		settings, err := f.st.GetSettings()
		if err != nil {
			t.Fatal(err)
		}
		p, err := f.svc.readPlacement(settings, "containers")
		if err != nil {
			t.Fatal(err)
		}
		for _, tg := range p.Targets {
			if tg.ID == b2.ID {
				return p.rulesRev(tg)
			}
		}
		t.Fatal("B2 is not among the domain's targets")
		return ""
	}

	before := rev()
	place.RetentionKeepLast = 6
	f.storePlace(place)
	if rev() == before {
		t.Fatal("a new keep-policy at the place left the target's fingerprint alone, so an aged target would never age by it")
	}
}

func TestAnAppendOnlyPlaceForgetsNothingWhateverItKeeps(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) {
		s.ContainersPath = b2Containers
		s.RetentionKeepLast = 3
	})
	home := b2Place(7)
	home.Immutable = true
	f.storePlace(home, "containers")
	archive := f.namedRepo("Archive", "s3:https://s3.example.com/archive")
	f.linkRow(archive.ID, f.storePlace(store.Place{
		Name: "Archive", Provider: "b2", Kind: "s3", Base: "s3:https://s3.example.com/archive",
		Folders: map[string]string{}, OffPremises: true, Immutable: true, RetentionKeepLast: 5, Enabled: true,
	}), "", "")
	f.container("nginx", archive.ID)
	f.hold(b2Containers, snap("a1", 100, "container:plex"))
	f.hold(archive.Repo, snap("b1", 100, "container:nginx"))
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	f.svc.applyRetention(ctx, b2Containers, settings, restic.Mode{}, tagIdentity("container:plex"), "containers", anomalyScope{})
	f.svc.applyRetention(ctx, archive.Repo, settings, restic.Mode{}, tagIdentity("container:nginx"), "containers", anomalyScope{})
	if _, err := f.svc.PruneDomain(ctx, "containers", "local"); !errors.Is(err, errAppendOnlyPrimaryRemote) {
		t.Fatalf("PruneDomain = %v, want the remote-primary append-only refusal", err)
	}
	if len(f.eng.forgets) != 0 || len(f.eng.prunes) != 0 {
		t.Fatalf("forgets %+v, prunes %v; want none at an append-only place", f.eng.forgets, f.eng.prunes)
	}
}

func TestADirectRepositoryAtAnAppendOnlyPlaceForgetsNothing(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", b2Containers)
	direct := f.direct(b2)
	place := b2Place(4)
	place.Immutable = true
	place = f.storePlace(place)
	f.linkRow(b2.ID, place, "containers", "")
	f.linkRow(direct.ID, place, "containers", "-direct")
	f.container("nginx", direct.ID)
	f.hold(direct.Repo, snap("d1", 100, "container:nginx"))
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	f.svc.applyRetention(ctx, direct.Repo, settings, restic.Mode{}, tagIdentity("container:nginx"), "containers", anomalyScope{})
	if _, err := f.svc.PruneDomain(ctx, "containers", "local"); err != nil {
		t.Fatalf("PruneDomain: %v", err)
	}
	if len(f.eng.forgets) != 0 || len(f.eng.prunes) != 1 || f.eng.prunes[0] != f.domainPath("containers") {
		t.Fatalf("forgets %+v, prunes %v; want only the domain path pruned and the direct repository left alone", f.eng.forgets, f.eng.prunes)
	}
}

func TestAnOffsitePruneAgesByTheTargetsOwnRules(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.OffsiteRetentionKeepDaily = 30 })
	b2 := f.target("containers", "B2", b2Containers)
	b2.RetentionKeepLast = 4
	if _, err := f.st.UpsertOffsiteTarget(b2); err != nil {
		t.Fatal(err)
	}
	f.hold(b2Containers, snap("c1", 100, "container:nginx"))

	if _, err := f.svc.PruneDomain(context.Background(), "containers", "offsite:"+b2.ID); err != nil {
		t.Fatalf("PruneDomain: %v", err)
	}
	want := []forgetCall{{Repo: b2Containers, Tags: []string{"container:nginx"}, Policy: restic.RetentionPolicy{KeepLast: 4}}}
	if !reflect.DeepEqual(f.eng.forgets, want) {
		t.Fatalf("forgets = %+v, want %+v", f.eng.forgets, want)
	}
}

func TestAnOffsitePruneAgesByItsPlace(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.OffsiteRetentionKeepDaily = 30 })
	b2 := f.target("containers", "B2", b2Containers)
	f.linkRow(b2.ID, f.storePlace(b2Place(6)), "containers", "")
	f.hold(b2Containers, snap("c1", 100, "container:nginx"))

	if _, err := f.svc.PruneDomain(context.Background(), "containers", "offsite"); err != nil {
		t.Fatalf("PruneDomain: %v", err)
	}
	want := []forgetCall{{Repo: b2Containers, Tags: []string{"container:nginx"}, Policy: restic.RetentionPolicy{KeepLast: 6}}}
	if !reflect.DeepEqual(f.eng.forgets, want) {
		t.Fatalf("forgets = %+v, want %+v", f.eng.forgets, want)
	}
}

func TestAnAppendOnlyPlaceRefusesTheOffsitePrune(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", b2Containers)
	place := b2Place(6)
	place.Immutable = true
	f.linkRow(b2.ID, f.storePlace(place), "containers", "")
	f.hold(b2Containers, snap("c1", 100, "container:nginx"))

	if _, err := f.svc.PruneDomain(context.Background(), "containers", "offsite:"+b2.ID); !errors.Is(err, errAppendOnlyOffsiteTarget) {
		t.Fatalf("PruneDomain = %v, want the append-only refusal", err)
	}
	if len(f.eng.forgets) != 0 || len(f.eng.prunes) != 0 {
		t.Fatalf("forgets %+v, prunes %v; want none at an append-only place", f.eng.forgets, f.eng.prunes)
	}
}

// photosAtAPlace is a file set on the NAS set to Local, with three older
// copies at a B2 target whose place keeps one.
func photosAtAPlace(t *testing.T, f *placementFixture, appendOnly bool) store.OffsiteTarget {
	t.Helper()
	b2 := f.target("files", "B2", "s3:https://s3.example.com/bv/files")
	f.linkRow(b2.ID, f.storePlace(store.Place{
		Name: "B2", Provider: "b2", Kind: "s3", Base: "s3:https://s3.example.com/bv",
		Folders: map[string]string{"files": "files"}, OffPremises: true, Immutable: appendOnly, RetentionKeepLast: 1, Enabled: true,
	}), "files", "")
	nas := f.namedRepo("NAS", "nas")
	f.fileSet("Photos", nas.ID)
	f.rule("files", "fileset:Photos", store.SkipAll)
	f.replicated("files")
	f.hold(f.root+"/nas", snap("p1", 1000, "fileset:Photos"), snap("p2", 1100, "fileset:Photos"), snap("p3", 1200, "fileset:Photos"))
	f.hold(b2.Repo, copied("c1", "p1", 1000, "fileset:Photos"), copied("c2", "p2", 1100, "fileset:Photos"), copied("c3", "p3", 1200, "fileset:Photos"))
	return b2
}

func TestAReplicationAgesATargetByItsPlace(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := photosAtAPlace(t, f, false)

	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatalf("ReplicateOffsite = %v, want a green pass", err)
	}
	if n := len(heldAt(t, f, b2.Repo)); n != 1 {
		t.Fatalf("B2 holds %d, want the one its place keeps", n)
	}
}

func TestAReplicationNeverAgesATargetAtAnAppendOnlyPlace(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := photosAtAPlace(t, f, true)

	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatalf("ReplicateOffsite = %v, want a green pass", err)
	}
	if n := len(heldAt(t, f, b2.Repo)); n != 3 || len(f.eng.forgets) != 0 {
		t.Fatalf("B2 at an append-only place holds %d after %+v, want all 3 and no forget", n, f.eng.forgets)
	}
}
