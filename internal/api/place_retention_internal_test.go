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

// unlinkRow takes a row off its place, as a changed address does.
func (f *placementFixture) unlinkRow(rowID string) {
	f.t.Helper()
	tx, err := f.db.Begin()
	if err != nil {
		f.t.Fatal(err)
	}
	if err := store.DetachRowTx(tx, rowID); err != nil {
		_ = tx.Rollback()
		f.t.Fatalf("detach %s: %v", rowID, err)
	}
	if err := tx.Commit(); err != nil {
		f.t.Fatal(err)
	}
}

func TestARowAtAPlaceAgesByTheRulesItsPlaceMirrors(t *testing.T) {
	f := newPlacementFixture(t)
	settings := localKeepLast(t, f, 3)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	f.linkRow(nas.ID, f.storePlace(nasPlace(5)), "", "")

	f.svc.applyRetention(context.Background(), f.root+"/nas", settings, restic.Mode{}, tagIdentity("container:nginx"), "containers")
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
	f.svc.applyRetention(context.Background(), f.domainPath("containers"), settings, restic.Mode{}, tagIdentity("container:plex"), "containers")
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
	if detached.PlaceID != "" || targetOffsiteRetentionPolicy(detached).Any() {
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
	f.svc.applyRetention(ctx, b2Containers, settings, restic.Mode{}, tagIdentity("container:plex"), "containers")
	f.svc.applyRetention(ctx, archive.Repo, settings, restic.Mode{}, tagIdentity("container:nginx"), "containers")
	if err := f.svc.PruneDomain(ctx, "containers", "local"); !errors.Is(err, errAppendOnlyPrimaryRemote) {
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
	f.svc.applyRetention(ctx, direct.Repo, settings, restic.Mode{}, tagIdentity("container:nginx"), "containers")
	if err := f.svc.PruneDomain(ctx, "containers", "local"); err != nil {
		t.Fatalf("PruneDomain: %v", err)
	}
	if len(f.eng.forgets) != 0 || len(f.eng.prunes) != 1 || f.eng.prunes[0] != f.domainPath("containers") {
		t.Fatalf("forgets %+v, prunes %v; want only the domain path pruned and the direct repository left alone", f.eng.forgets, f.eng.prunes)
	}
}
