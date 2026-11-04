package api

import (
	"context"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// selfBackupEngine is the placement engine with a backup that always succeeds.
type selfBackupEngine struct {
	*placementEngine
}

func (e selfBackupEngine) Backup(context.Context, string, []string, []string, restic.Mode, ...string) (restic.Summary, error) {
	return restic.Summary{SnapshotID: "c0ffee00"}, nil
}

// keptLocation is a mounted folder whose How long to keep card is set to keep,
// saved the way the card saves it.
func (f *placementFixture) keptLocation(keep store.RetentionKeep) store.OffsiteTarget {
	f.t.Helper()
	d, err := f.svc.SaveNewDestination(context.Background(), draftRequest{Provider: "path", Settings: map[string]string{"path": "zfs/bombvault/dxp480t"}})
	if err != nil {
		f.t.Fatal(err)
	}
	if res := f.do("PUT", "/api/offsite/destinations/"+d.ID, map[string]any{"name": d.Name, "retention": keep}); res["ok"] != true {
		f.t.Fatalf("keep card save = %v", res)
	}
	return d
}

func (f *placementFixture) ownFoldersInto(d store.OffsiteTarget, domains ...string) {
	f.t.Helper()
	ownSteps(f.t, f.do("POST", "/api/storage/own-folders", map[string]any{"domains": domains, "location": "destination:" + d.ID}))
}

var fortnight = store.RetentionKeep{KeepDaily: 14}

func TestAFreshInstallPrunesAnOwnFolderByItsLocationsKeepCard(t *testing.T) {
	f := newPlacementFixture(t)
	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.ConfigEnabled, s.ConfigPath = true, "backups/config"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	local := f.keptLocation(fortnight)
	f.ownFoldersInto(local, "config")
	f.svc.engine = selfBackupEngine{f.eng}

	if _, err := f.svc.BackupConfig(context.Background()); err != nil {
		t.Fatalf("BackupConfig: %v", err)
	}

	want := forgetCall{Repo: f.root + "/zfs/bombvault/dxp480t/selfbackup", Tags: []string{"config"}, Policy: restic.RetentionPolicy{KeepDaily: 14}, Prune: true}
	if !slices.ContainsFunc(f.eng.forgets, func(c forgetCall) bool { return c.Repo == want.Repo && c.Policy == want.Policy && c.Prune }) {
		t.Fatalf("forgets = %+v, want %+v", f.eng.forgets, want)
	}
}

func TestTheRoundAfterAScheduledRunPrunesOwnFoldersInALocation(t *testing.T) {
	f := newPlacementFixture(t)
	local := f.keptLocation(fortnight)
	f.ownFoldersInto(local, "containers")
	there := f.root + "/zfs/bombvault/dxp480t/containers"
	f.makeRepo(there)
	f.hold(there, snap("a1", 100, "container:plex"))

	f.svc.PruneAfterBulk(context.Background(), "containers")

	if want := []string{there}; !slices.Equal(f.eng.prunes, want) {
		t.Fatalf("prunes = %v, want %v", f.eng.prunes, want)
	}
}

func TestASectionsOwnKeepRuleWinsOverItsLocationsCard(t *testing.T) {
	f := newPlacementFixture(t)
	local := f.keptLocation(fortnight)
	f.ownFoldersInto(local, "containers", "vms")
	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.SetOwnRetention(map[string]store.RetentionKeep{"vms": {KeepLast: 5}})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	if got := f.svc.retentionPolicy(settings, "containers"); got != (restic.RetentionPolicy{KeepDaily: 14}) {
		t.Errorf("containers ages by %+v, want the location's 14 days", got)
	}
	if got := f.svc.retentionPolicy(settings, "vms"); got != (restic.RetentionPolicy{KeepLast: 5}) {
		t.Errorf("vms ages by %+v, want its own rule", got)
	}
	loc := locationByID(t, f.svc, "destination:"+local.ID)
	if got := sectionFor(t, loc, "containers", useHome); got.Retention != fortnight || len(got.Own) != 0 {
		t.Errorf("containers = %+v, want it to follow the card", got)
	}
	if got := sectionFor(t, loc, "vms", useHome); got.Retention != (store.RetentionKeep{KeepLast: 5}) || !slices.Equal(got.Own, []string{"retention"}) {
		t.Errorf("vms = %+v, want its own rule shown as its own", got)
	}
}

func TestAnOwnFolderOutsideALocationKeepsTheSharedRule(t *testing.T) {
	f := newPlacementFixture(t)
	f.keptLocation(fortnight)
	settings := localKeepLast(t, f, 3)

	if got := f.svc.retentionPolicy(settings, "containers"); got != (restic.RetentionPolicy{KeepLast: 3}) {
		t.Fatalf("containers ages by %+v, want the shared keep-last 3", got)
	}
}

func TestTheMovePreviewNamesTheRuleBeforeAndAfter(t *testing.T) {
	f := newPlacementFixture(t)
	local := f.keptLocation(fortnight)
	localKeepLast(t, f, 30)

	res := f.do("POST", "/api/storage/own-folders/preview", map[string]any{"domains": []string{"containers"}, "location": "destination:" + local.ID})

	step := res["steps"].([]any)[0].(map[string]any)
	if got := step["keep"].(map[string]any)["keepLast"]; got != float64(30) {
		t.Errorf("keep = %v, want the shared keep-last 30", step["keep"])
	}
	if got := step["keepThere"].(map[string]any)["keepDaily"]; got != float64(14) {
		t.Errorf("keepThere = %v, want the card's 14 days", step["keepThere"])
	}
}

func TestAMoveKeepsTheCurrentRuleOfTheSectionsItNames(t *testing.T) {
	f := newPlacementFixture(t)
	local := f.keptLocation(fortnight)
	localKeepLast(t, f, 30)

	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers", "vms"}, "location": "destination:" + local.ID, "keepRules": []string{"containers"},
	}))

	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got := f.svc.retentionPolicy(settings, "containers"); got != (restic.RetentionPolicy{KeepLast: 30}) {
		t.Errorf("containers ages by %+v, want the keep-last 30 it asked to keep", got)
	}
	if got := f.svc.retentionPolicy(settings, "vms"); got != (restic.RetentionPolicy{KeepDaily: 14}) {
		t.Errorf("vms ages by %+v, want the card it did not opt out of", got)
	}
	if got := sectionFor(t, locationByID(t, f.svc, "destination:"+local.ID), "containers", useHome); !slices.Equal(got.Own, []string{"retention"}) {
		t.Errorf("containers = %+v, want the kept rule shown as its own", got)
	}
}
