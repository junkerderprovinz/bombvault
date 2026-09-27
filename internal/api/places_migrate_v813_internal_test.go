package api

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// v813Scene holds the rows of a setup from before storage places that carries
// every form the migration has to keep: local domain paths, a NAS share, B2
// with a direct repository, a direct repository in another bucket, a
// rest-server with a user per domain, a mesh target, a remote domain path with
// its primary row, a named repository two domains share, and rclone:, sftp:
// and b2: targets.
type v813Scene struct {
	b2, b2VMs, wasabi, towerC, towerV, mesh, pi, drive, native store.OffsiteTarget
	b2Direct, wasabiDirect, nas, cold                          store.OffsiteTarget
}

func seedV813(t *testing.T, f *placementFixture) v813Scene {
	t.Helper()
	var s v813Scene
	save := func(row store.OffsiteTarget) store.OffsiteTarget {
		t.Helper()
		stored, err := f.st.UpsertOffsiteTarget(row)
		if err != nil {
			t.Fatal(err)
		}
		return stored
	}
	if err := f.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "rest-containers", Name: "Tower containers", CloudCreds: CloudCreds{RESTUser: "bombvault-containers", RESTPassword: "pc"}},
		{ID: "rest-vms", Name: "Tower vms", CloudCreds: CloudCreds{RESTUser: "bombvault-vms", RESTPassword: "pv"}},
		{ID: "mesh-tower2", Name: "mesh: tower2", CloudCreds: CloudCreds{RESTUser: "bombvault-flash", RESTPassword: "pm"}},
		{ID: "garage", Name: "Garage", CloudCreds: CloudCreds{S3KeyID: "GK", S3Secret: "GS"}},
	}); err != nil {
		t.Fatal(err)
	}
	settings := settingsOf(t, f.svc)
	settings.RetentionKeepLast = 5
	settings.ConfigPath = "s3:https://s3.example.com/bv-primary/config"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SetPrimaryRemoteConfig("config", store.OffsiteTarget{CredsRef: "garage", LimitUpload: 500}); err != nil {
		t.Fatal(err)
	}
	flash := f.singletonRepo("flash")

	s.b2 = keepLast(t, f, f.target("containers", "B2", "s3:https://s3.us-west-004.backblazeb2.com/bv-bucket/containers"), 3)
	s.b2VMs = keepLast(t, f, f.target("vms", "B2 VMs", "s3:https://s3.us-west-004.backblazeb2.com/bv-bucket/vms"), 3)
	s.b2Direct = f.direct(s.b2)
	s.wasabi = keepLast(t, f, f.target("files", "Wasabi", "s3:https://s3.eu-central-1.wasabisys.com/bv-files/files"), 7)
	wasabiDirect, err := f.st.CreateCompanionRepo(s.wasabi.ID, "Wasabi direct", "s3:https://s3.eu-central-1.wasabisys.com/bv-direct/files")
	if err != nil {
		t.Fatal(err)
	}
	s.wasabiDirect = wasabiDirect
	towerC := f.target("containers", "Tower", "rest:http://tower:8000/bombvault-containers/containers")
	towerC.CredsRef, towerC.Immutable = "rest-containers", true
	s.towerC = save(towerC)
	towerV := f.target("vms", "Tower VMs", "rest:http://tower:8000/bombvault-vms/vms")
	towerV.CredsRef, towerV.Immutable = "rest-vms", true
	s.towerV = save(towerV)
	mesh := f.target("flash", "mesh: tower2", "rest:http://tower2:8000/bombvault-flash/flash")
	mesh.CredsRef = "mesh-tower2"
	s.mesh = save(mesh)
	s.pi = f.target("flash", "Pi", "sftp:pi:flash")
	s.drive = f.target("config", "Drive", "rclone:r:config")
	s.native = f.target("files", "B2 native", "b2:bucket:files")
	// Rows made within one second tie on created_at, and the older of two
	// targets names a shared place, so the order they were made in is written
	// down.
	for i, id := range []string{s.b2.ID, s.b2VMs.ID, s.wasabi.ID, s.towerC.ID, s.towerV.ID, s.mesh.ID, s.pi.ID, s.drive.ID, s.native.ID} {
		if _, err := f.db.Exec(`UPDATE offsite_targets SET created_at = ? WHERE id = ?`, 1_700_000_000+i, id); err != nil {
			t.Fatal(err)
		}
	}

	s.nas = f.namedRepo("NAS", "remotes/nas/bombvault")
	cold := f.namedRepo("Cold", "s3:https://s3.example.com/bv-cold")
	f.offPremises(cold.ID)
	if s.cold, err = f.st.GetNamedRepo(cold.ID); err != nil {
		t.Fatal(err)
	}

	f.container("nginx", s.nas.ID)
	f.container("plex", "")
	f.container("immich", s.b2Direct.ID)
	f.container("paperless", s.cold.ID)
	f.vm("win11", s.cold.ID)
	f.vm("web", "")
	f.fileSet("Docs", "")
	for _, d := range places.Domains {
		f.replicated(d)
	}
	f.hold(f.domainPath("containers"), snap("p1", 100, "container:plex"), snap("p2", 200, "container:plex"))
	f.hold(f.root+"/remotes/nas/bombvault", snap("n1", 100, "container:nginx"), snap("n2", 200, "container:nginx"))
	f.hold(s.b2Direct.Repo, snap("i1", 100, "container:immich"))
	f.hold(s.cold.Repo, snap("c1", 100, "container:paperless"), snap("w1", 100, "vm:win11"))
	f.hold(f.domainPath("vms"), snap("v1", 100, "vm:web"))
	f.hold(f.domainPath("files"), snap("d1", 100, "fileset:Docs"))
	f.hold(flash, snap("f1", 100, "flash"))
	f.hold(settings.ConfigPath, snap("g1", 100, "config"))
	return s
}

// rowFields is everything of a row but its place and its keep columns, which
// retentionByRepo compares by what they do.
type rowFields struct {
	ID, Domain, Name, Repo, Role, CredsRef, StorageClass, Schedule, CompanionOf string
	Immutable, Enabled, CompanionLost, OffPremises                              bool
	LimitUpload, LimitDownload, GrowthBudgetGB, SortOrder                       int
	CreatedAt                                                                   int64
}

func fieldsOf(r store.OffsiteTarget) rowFields {
	return rowFields{
		ID: r.ID, Domain: r.Domain, Name: r.Name, Repo: r.Repo, Role: r.Role, CredsRef: r.CredsRef, StorageClass: r.StorageClass,
		Schedule: r.Schedule, CompanionOf: r.CompanionOf, Immutable: r.Immutable, Enabled: r.Enabled, CompanionLost: r.CompanionLost,
		OffPremises: r.OffPremises, LimitUpload: r.LimitUpload, LimitDownload: r.LimitDownload, GrowthBudgetGB: r.GrowthBudgetGB,
		SortOrder: r.SortOrder, CreatedAt: r.CreatedAt,
	}
}

// storedRows are the offsite_targets rows of every role, by id.
func storedRows(t *testing.T, f *placementFixture) map[string]store.OffsiteTarget {
	t.Helper()
	out := map[string]store.OffsiteTarget{}
	targets, err := f.st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	named, err := f.st.ListNamedRepos()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range slices.Concat(targets, named) {
		out[r.ID] = r
	}
	for _, d := range places.Domains {
		row, ok, err := f.st.PrimaryRemoteTarget(d)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			out[row.ID] = row
		}
	}
	return out
}

// retentionByRepo is the keep-policy each repository ages by, keyed by
// "<domain> path" or row id.
func retentionByRepo(t *testing.T, f *placementFixture) map[string]restic.RetentionPolicy {
	t.Helper()
	settings := settingsOf(t, f.svc)
	out := map[string]restic.RetentionPolicy{}
	for _, d := range places.Domains {
		loc, err := f.svc.repoFor(settings, d, "local")
		if err != nil {
			t.Fatal(err)
		}
		out[d+" path"] = f.svc.retentionPolicyForRef(settings, d, ownRef(loc))
	}
	for id, row := range storedRows(t, f) {
		switch row.Role {
		case store.RoleRepo:
			loc, err := f.svc.resolveRepo(row.Repo)
			if err != nil {
				t.Fatal(err)
			}
			out[id] = f.svc.retentionPolicyForRef(settings, "", namedRef(loc, row))
		case store.RoleOffsite:
			out[id] = rowRetentionPolicy(row)
		}
	}
	return out
}

// firstRun replicates and prunes every domain once and returns what restic was
// asked to do, with the fixture's root and row ids spelled so two fixtures
// compare.
func firstRun(t *testing.T, f *placementFixture) []string {
	t.Helper()
	ctx := context.Background()
	var out []string
	for _, d := range places.Domains {
		if err := f.svc.ReplicateOffsite(ctx, d); err != nil {
			out = append(out, fmt.Sprintf("replicate %s: %v", d, err))
		}
		if _, err := f.svc.pruneDomain(ctx, d, "local", true); err != nil {
			out = append(out, fmt.Sprintf("prune %s: %v", d, err))
		}
	}
	f.eng.mu.Lock()
	for _, c := range f.eng.copies {
		out = append(out, fmt.Sprintf("copy %s <- %s ids=%v whole=%v", c.Dest, c.Src, c.IDs, c.IDs == nil))
	}
	for _, c := range f.eng.forgets {
		out = append(out, fmt.Sprintf("forget %s tags=%v %+v prune=%v", c.Repo, c.Tags, c.Policy, c.Prune))
	}
	for _, repo := range f.eng.prunes {
		out = append(out, "pruned "+repo)
	}
	f.eng.mu.Unlock()
	pairs := []string{f.root, "<root>"}
	for id, row := range storedRows(t, f) {
		pairs = append(pairs, id, "<"+row.Name+">")
	}
	spell := strings.NewReplacer(pairs...)
	for i := range out {
		out[i] = spell.Replace(out[i])
	}
	slices.Sort(out)
	return out
}

func TestAV813SetupRunsTheSameAfterTheMove(t *testing.T) {
	before, after := newPlacementFixture(t), newPlacementFixture(t)
	seedV813(t, before)
	seedV813(t, after)
	rows, settings, rules := storedRows(t, after), settingsOf(t, after.svc), retentionByRepo(t, after)

	if err := after.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("MigrateToPlaces: %v", err)
	}

	moved := storedRows(t, after)
	if got, want := slices.Sorted(maps.Keys(moved)), slices.Sorted(maps.Keys(rows)); !slices.Equal(got, want) {
		t.Fatalf("row ids after the move = %v, want %v", got, want)
	}
	for id, row := range rows {
		if got := fieldsOf(moved[id]); got != fieldsOf(row) {
			t.Errorf("row %s changed:\n got %+v\nwant %+v", row.Name, got, fieldsOf(row))
		}
	}
	now := settingsOf(t, after.svc)
	now.PlacesMigrated = settings.PlacesMigrated
	if now != settings {
		t.Error("the move changed the settings row beyond places_migrated")
	}
	if got := retentionByRepo(t, after); !maps.Equal(got, rules) {
		t.Errorf("keep-policies after the move = %v, want %v", got, rules)
	}
	a, b := firstRun(t, before), firstRun(t, after)
	if !slices.ContainsFunc(a, func(line string) bool { return strings.HasPrefix(line, "copy ") }) {
		t.Fatalf("the first run copied nothing, so there is nothing to compare: %q", a)
	}
	if !slices.Equal(a, b) {
		t.Fatalf("the first run after the move differs:\nwithout places %q\nwith places    %q", a, b)
	}
}

func TestAV813SetupLandsOnTheExpectedPlaces(t *testing.T) {
	f := newPlacementFixture(t)
	s := seedV813(t, f)
	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("MigrateToPlaces: %v", err)
	}
	checkV813Places(t, f, s)
}

func TestAV813SetupMovesAtTheStartAfterAFailedOne(t *testing.T) {
	f := newPlacementFixture(t)
	s := seedV813(t, f)
	rows, settings := storedRows(t, f), settingsOf(t, f.svc)
	// Stands in for a write that fails late: named repositories are placed
	// last, so the home places, the target places and their rows are written
	// by the time NAS is attached.
	if _, err := f.db.Exec(`CREATE TRIGGER fail_nas_attach BEFORE UPDATE ON offsite_targets
		WHEN NEW.name = 'NAS' AND NEW.place_id <> ''
		BEGIN SELECT RAISE(ABORT, 'disk I/O error'); END`); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MigrateToPlaces(); err == nil {
		t.Fatal("the move reported success over a failed write")
	}
	if all, err := f.st.ListPlaces(); err != nil || len(all) != 0 {
		t.Fatalf("places after the failed start = %+v, %v, want none", all, err)
	}
	if !maps.Equal(storedRows(t, f), rows) {
		t.Fatal("the failed start changed a row")
	}
	if settingsOf(t, f.svc) != settings {
		t.Fatal("the failed start changed the settings row")
	}

	if _, err := f.db.Exec(`DROP TRIGGER fail_nas_attach`); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("the next start: %v", err)
	}
	checkV813Places(t, f, s)
}

// checkV813Places checks where the move put every row, domain path and place
// of the v8.13.0 scene.
func checkV813Places(t *testing.T, f *placementFixture, s v813Scene) {
	t.Helper()
	all, err := f.st.ListPlaces()
	if err != nil {
		t.Fatal(err)
	}
	byID, byName := map[string]store.Place{}, map[string]store.Place{}
	for _, p := range all {
		byID[p.ID], byName[p.Name] = p, p
	}
	rows := storedRows(t, f)
	for id, want := range map[string]string{
		s.b2.ID: "B2 containers", s.b2VMs.ID: "B2 vms", s.b2Direct.ID: "B2 containers-direct",
		s.wasabi.ID: "Wasabi files", s.wasabiDirect.ID: "",
		s.towerC.ID: "Tower containers", s.towerV.ID: "Tower VMs vms", s.mesh.ID: "mesh: tower2 flash",
		s.pi.ID: "Pi flash", s.drive.ID: "Drive config", s.native.ID: "",
		s.nas.ID: "NAS ", s.cold.ID: "Cold ",
	} {
		row, got := rows[id], ""
		if row.PlaceID != "" {
			got = byID[row.PlaceID].Name + " " + row.PlaceDomain + row.PlaceSuffix
		}
		if got != want {
			t.Errorf("%s sits on %q, want %q", row.Name, got, want)
		}
	}
	homes, err := f.st.DomainPlaces()
	if err != nil {
		t.Fatal(err)
	}
	for d, want := range map[string]string{"containers": "Unraid", "vms": "Unraid", "files": "Unraid", "flash": "Unraid 2", "config": "s3.example.com"} {
		if got := byID[homes[d]].Name; got != want {
			t.Errorf("home of %s = %q, want %q", d, got, want)
		}
	}
	every := map[string]string{"containers": "", "vms": "", "flash": "", "config": "", "files": ""}
	type placeWant struct {
		provider, base string
		folders        map[string]string
		offPremises    bool
		keepLast       int
	}
	wantPlaces := map[string]placeWant{
		"Unraid":         {"unraid-folder", "backups", map[string]string{"containers": "containers", "vms": "vms", "files": "files"}, false, 5},
		"Unraid 2":       {"unraid-folder", "user/bombvault", map[string]string{"flash": "flash"}, false, 5},
		"s3.example.com": {"s3-other", "s3:https://s3.example.com/bv-primary", map[string]string{"config": "config"}, true, 5},
		"B2": {"b2", "s3:https://s3.us-west-004.backblazeb2.com/bv-bucket",
			map[string]string{"containers": "containers", "vms": "vms", "flash": "flash", "config": "config", "files": "files"}, true, 3},
		"Wasabi": {"wasabi", "s3:https://s3.eu-central-1.wasabisys.com/bv-files",
			map[string]string{"files": "files", "containers": "container", "vms": "vms", "flash": "flash", "config": "config"}, true, 7},
		"Tower":        {"rest-server", "rest:http://tower:8000/bombvault-containers", map[string]string{"containers": "containers"}, true, 0},
		"Tower VMs":    {"rest-server", "rest:http://tower:8000/bombvault-vms", map[string]string{"vms": "vms"}, true, 0},
		"mesh: tower2": {"bombvault", "rest:http://tower2:8000/bombvault-flash", map[string]string{"flash": "flash"}, true, 0},
		"Pi": {"sftp", "sftp:pi:",
			map[string]string{"flash": "flash", "containers": "container", "vms": "vms", "config": "config", "files": "files"}, true, 0},
		"Drive": {"rclone", "rclone:r:",
			map[string]string{"config": "config", "containers": "container", "vms": "vms", "flash": "flash", "files": "files"}, true, 0},
		"NAS":  {"share", "remotes/nas/bombvault", every, false, 5},
		"Cold": {"s3-other", "s3:https://s3.example.com/bv-cold", every, true, 5},
	}
	if len(all) != len(wantPlaces) {
		t.Errorf("places = %d, want %d", len(all), len(wantPlaces))
	}
	for name, w := range wantPlaces {
		p, ok := byName[name]
		if !ok || p.Provider != w.provider || p.Base != w.base || !maps.Equal(p.Folders, w.folders) ||
			p.OffPremises != w.offPremises || p.RetentionKeepLast != w.keepLast {
			t.Errorf("%s = %+v, %v, want %+v", name, p, ok, w)
		}
	}
	if p := byName["s3.example.com"]; p.LimitUpload != 500 || p.Immutable {
		t.Errorf("the remote domain path's place = %+v, want the primary row's cap", p)
	}
	for name, ref := range map[string]string{
		"s3.example.com": "garage", "Tower": "rest-containers", "Tower VMs": "rest-vms", "mesh: tower2": "mesh-tower2",
		"B2": "", "Unraid": "", "NAS": "",
	} {
		if got := byName[name].CredsRef; got != ref {
			t.Errorf("%s credentials = %q, want %q", name, got, ref)
		}
	}
	for _, p := range all {
		if found, ok := places.ProviderByID(p.Provider); !ok || string(found.Kind) != p.Kind {
			t.Errorf("%s: provider %q does not speak %s", p.Name, p.Provider, p.Kind)
		}
	}
	for _, row := range rows {
		if row.PlaceID == "" {
			continue
		}
		p := byID[row.PlaceID]
		if addr, ok := places.Address(p.Base, p.Folders, row.PlaceDomain, row.PlaceSuffix); !ok || addr != row.Repo {
			t.Errorf("%s holds %q, its place spells %q, %v", row.Name, row.Repo, addr, ok)
		}
	}
}
