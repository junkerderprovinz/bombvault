package store_test

import (
	"database/sql"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func migratedStore(t *testing.T) (*store.Repo, *sql.DB) {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return store.New(db), db
}

func TestNamedRepoReadsItsCompanionLink(t *testing.T) {
	r, db := migratedStore(t)
	if _, err := db.Exec(`INSERT INTO offsite_targets (id, domain, repo, role, companion_of, companion_lost)
		VALUES ('d', '', 'b2:bkt:x-direct', 'repo', 't1', 0), ('l', '', 'b2:bkt:y-direct', 'repo', '', 1)`); err != nil {
		t.Fatal(err)
	}
	d, err := r.GetNamedRepo("d")
	if err != nil || d.CompanionOf != "t1" || d.CompanionLost {
		t.Fatalf("direct row = %+v, %v", d, err)
	}
	l, err := r.GetNamedRepo("l")
	if err != nil || l.CompanionOf != "" || !l.CompanionLost {
		t.Fatalf("lost row = %+v, %v", l, err)
	}
}

func TestOneTargetHasAtMostOneDirectRepository(t *testing.T) {
	_, db := migratedStore(t)
	insert := `INSERT INTO offsite_targets (id, domain, repo, role, companion_of) VALUES (?, '', ?, 'repo', ?)`
	if _, err := db.Exec(insert, "a", "b2:bkt:a", "t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(insert, "b", "b2:bkt:b", "t1"); err == nil {
		t.Fatal("a second direct repository for the same target was stored")
	}
	for _, id := range []string{"c", "d"} {
		if _, err := db.Exec(insert, id, "b2:bkt:"+id, ""); err != nil {
			t.Fatalf("plain repositories share the empty link: %v", err)
		}
	}
}

func TestCompanionMigrationIsRecordedWhereTheColumnsExist(t *testing.T) {
	_, db := migratedStore(t)
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = 'offsite_targets_companion'`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("a database that already has the columns must record the migration: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name = 'offsite_targets_companion'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("recorded %d times, err %v", n, err)
	}
}

func richTarget(t *testing.T, r *store.Repo) store.OffsiteTarget {
	t.Helper()
	tg := store.SeedOffsiteTarget(t, r, "containers", "b2:bkt:containers")
	tg.Name = "B2"
	tg.CredsRef = "set-1"
	tg.StorageClass = "STANDARD_IA"
	tg.Immutable = true
	tg.RetentionKeepLast, tg.RetentionKeepDaily, tg.RetentionKeepWeekly, tg.RetentionKeepMonthly = 1, 2, 3, 4
	tg.LimitUpload, tg.LimitDownload, tg.GrowthBudgetGB = 10, 20, 5
	saved, err := r.UpsertOffsiteTarget(tg)
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

func TestCreateCompanionRepoTakesEveryMirroredFieldOfItsTarget(t *testing.T) {
	r, _ := migratedStore(t)
	store.SeedNamedRepo(t, r, "NAS", "backups/nas")
	target := richTarget(t, r)
	d, err := r.CreateCompanionRepo(target.ID, "B2 direct", " b2:bkt:containers-direct ")
	if err != nil {
		t.Fatal(err)
	}
	if d.Role != store.RoleRepo || d.Domain != "" || d.CompanionOf != target.ID || !d.Enabled || d.Repo != "b2:bkt:containers-direct" {
		t.Fatalf("direct row = %+v", d)
	}
	if !d.MirroredEqual(target) {
		t.Fatalf("the direct row does not carry its target's fields:\n%+v\n%+v", d, target)
	}
	got, found, err := r.CompanionFor(target.ID)
	if err != nil || !found || got.ID != d.ID {
		t.Fatalf("CompanionFor = %+v, %v, %v", got, found, err)
	}
	repos, err := r.ListNamedRepos()
	if err != nil {
		t.Fatal(err)
	}
	if repos[len(repos)-1].ID != d.ID {
		t.Fatal("a new direct repository goes behind the named repositories already there")
	}
}

func TestCreateCompanionRepoRefusesWhatCannotBeADirectRepository(t *testing.T) {
	r, _ := migratedStore(t)
	target := store.SeedOffsiteTarget(t, r, "vms", "b2:bkt:vms")
	if _, err := r.CreateCompanionRepo("missing", "x", "b2:bkt:x"); !errors.Is(err, store.ErrNotOffsiteTarget) {
		t.Fatalf("unknown target: %v", err)
	}
	named := store.SeedNamedRepo(t, r, "NAS", "backups/nas")
	if _, err := r.CreateCompanionRepo(named.ID, "x", "b2:bkt:x"); !errors.Is(err, store.ErrNotOffsiteTarget) {
		t.Fatalf("a named repository is no target: %v", err)
	}
	if _, err := r.CreateCompanionRepo(target.ID, "x", "  "); !errors.Is(err, store.ErrEmptyOffsiteRepo) {
		t.Fatalf("empty location: %v", err)
	}
	if _, err := r.CreateCompanionRepo(target.ID, "one", "b2:bkt:vms-direct"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateCompanionRepo(target.ID, "two", "b2:bkt:vms-direct-2"); !errors.Is(err, store.ErrCompanionTaken) {
		t.Fatalf("second direct repository: %v", err)
	}
	other := store.SeedOffsiteTarget(t, r, "vms", "b2:bkt:other")
	if _, found, err := r.CompanionFor(other.ID); err != nil || found {
		t.Fatalf("a target without one: found %v, err %v", found, err)
	}
}

func TestMirroredEqualSeesEachMirroredField(t *testing.T) {
	base := store.OffsiteTarget{
		CredsRef: "a", StorageClass: "STANDARD",
		RetentionKeepLast: 1, RetentionKeepDaily: 1, RetentionKeepWeekly: 1, RetentionKeepMonthly: 1,
		LimitUpload: 1, LimitDownload: 1, GrowthBudgetGB: 1,
	}
	changes := map[string]func(*store.OffsiteTarget){
		"CredsRef":             func(o *store.OffsiteTarget) { o.CredsRef = "b" },
		"StorageClass":         func(o *store.OffsiteTarget) { o.StorageClass = "GLACIER_IR" },
		"Immutable":            func(o *store.OffsiteTarget) { o.Immutable = true },
		"RetentionKeepLast":    func(o *store.OffsiteTarget) { o.RetentionKeepLast = 2 },
		"RetentionKeepDaily":   func(o *store.OffsiteTarget) { o.RetentionKeepDaily = 2 },
		"RetentionKeepWeekly":  func(o *store.OffsiteTarget) { o.RetentionKeepWeekly = 2 },
		"RetentionKeepMonthly": func(o *store.OffsiteTarget) { o.RetentionKeepMonthly = 2 },
		"LimitUpload":          func(o *store.OffsiteTarget) { o.LimitUpload = 2 },
		"LimitDownload":        func(o *store.OffsiteTarget) { o.LimitDownload = 2 },
		"GrowthBudgetGB":       func(o *store.OffsiteTarget) { o.GrowthBudgetGB = 2 },
	}
	for name, change := range changes {
		o := base
		change(&o)
		if base.MirroredEqual(o) {
			t.Errorf("%s differs and MirroredEqual did not notice", name)
		}
	}
	own := base
	own.ID, own.Name, own.Repo, own.Schedule, own.Enabled = "x", "other", "elsewhere", "daily", true
	if !base.MirroredEqual(own) {
		t.Error("id, name, location, schedule and the switch belong to the row itself")
	}
}

func TestSavingATargetMirrorsIntoItsDirectRepositoryButKeepsItsCredentials(t *testing.T) {
	r, _ := migratedStore(t)
	target := richTarget(t, r)
	direct := store.SeedCompanion(t, r, target)
	target.CredsRef = "set-2"
	target.StorageClass = "GLACIER_IR"
	target.Immutable = false
	target.RetentionKeepLast, target.RetentionKeepDaily, target.RetentionKeepWeekly, target.RetentionKeepMonthly = 9, 8, 7, 6
	target.LimitUpload, target.LimitDownload, target.GrowthBudgetGB = 30, 40, 50
	if _, err := r.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetNamedRepo(direct.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := target
	want.CredsRef = "set-1"
	if !got.MirroredEqual(want) {
		t.Fatalf("direct row after the save:\n%+v\nwant the target's fields with the old credentials:\n%+v", got, want)
	}
}

func TestSavingATargetUnchangedLeavesItsDirectRowUntouched(t *testing.T) {
	r, db := migratedStore(t)
	target := richTarget(t, r)
	store.SeedCompanion(t, r, target)
	if _, err := db.Exec(`
		CREATE TABLE direct_writes (n INTEGER);
		CREATE TRIGGER count_direct_writes AFTER UPDATE ON offsite_targets WHEN old.companion_of <> ''
		BEGIN INSERT INTO direct_writes VALUES (1); END;`); err != nil {
		t.Fatal(err)
	}
	writes := func() int {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM direct_writes`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	target.Name = "B2 renamed"
	if _, err := r.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	if n := writes(); n != 0 {
		t.Fatalf("a save that changed no mirrored field wrote the direct row %d times", n)
	}
	target.RetentionKeepLast++
	if _, err := r.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	if n := writes(); n != 1 {
		t.Fatalf("a changed retention wrote the direct row %d times, want 1", n)
	}
}

func TestUpdatingADirectRowKeepsWhatItTakesFromItsTarget(t *testing.T) {
	r, _ := migratedStore(t)
	target := richTarget(t, r)
	direct := store.SeedCompanion(t, r, target)
	edit := direct
	edit.Name = "renamed"
	edit.Enabled = false
	edit.Immutable = !direct.Immutable
	edit.RetentionKeepLast = 99
	edit.CredsRef = "other"
	edit.CompanionOf = ""
	got, err := r.UpsertOffsiteTarget(edit)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "renamed" || got.Enabled {
		t.Fatalf("name and switch were not saved: %+v", got)
	}
	if !got.MirroredEqual(direct) || got.CompanionOf != target.ID {
		t.Fatalf("an update changed mirrored fields or the link: %+v", got)
	}
}

func TestCompanionFieldsAreWrittenOnInsertOnly(t *testing.T) {
	r, _ := migratedStore(t)
	row, err := r.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "Old", Repo: "b2:bkt:old", Enabled: true, CompanionLost: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !row.CompanionLost || row.CompanionOf != "" {
		t.Fatalf("inserted row = %+v, want the lost label and no link", row)
	}
	row.CompanionOf = "t1"
	row.CompanionLost = false
	again, err := r.UpsertOffsiteTarget(row)
	if err != nil {
		t.Fatal(err)
	}
	if again.CompanionOf != "" || !again.CompanionLost {
		t.Fatalf("an update rewrote the link: %+v", again)
	}
}

// A settings file can carry one id in both of its blocks. The upsert then
// meets a stored row of the other role, and writing it would turn a target
// into a named repository or move a direct repository past its guard.
func TestAnUpsertLeavesARowOfAnotherRoleAlone(t *testing.T) {
	r, _ := migratedStore(t)
	target := richTarget(t, r)
	direct := store.SeedCompanion(t, r, target)

	asRepo := store.OffsiteTarget{ID: target.ID, Role: store.RoleRepo, Name: "stolen", Repo: "b2:bkt:elsewhere", Enabled: true}
	if _, err := r.UpsertOffsiteTarget(asRepo); err != nil {
		t.Fatal(err)
	}
	back, ok, err := r.GetOffsiteTarget(target.ID)
	if err != nil || !ok || back.Name != target.Name || back.Repo != target.Repo {
		t.Fatalf("the target was rewritten as a repository: %+v, ok %v, %v", back, ok, err)
	}

	asTarget := store.OffsiteTarget{ID: direct.ID, Domain: "containers", Name: "stolen", Repo: "b2:bkt:elsewhere", Enabled: true}
	if _, err := r.UpsertOffsiteTarget(asTarget); err != nil {
		t.Fatal(err)
	}
	row, err := r.GetNamedRepo(direct.ID)
	if err != nil || row.Repo != direct.Repo || row.Name != direct.Name || row.CompanionOf != target.ID {
		t.Fatalf("the direct repository was moved by a target upsert: %+v, %v", row, err)
	}
}

func TestMirrorCompanionCredsCopiesTheTargetsCredentials(t *testing.T) {
	r, _ := migratedStore(t)
	target := richTarget(t, r)
	direct := store.SeedCompanion(t, r, target)
	target.CredsRef = "set-2"
	if _, err := r.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	changed, err := r.MirrorCompanionCreds(target.ID)
	if err != nil || !changed {
		t.Fatalf("MirrorCompanionCreds = %v, %v", changed, err)
	}
	got, err := r.GetNamedRepo(direct.ID)
	if err != nil || got.CredsRef != "set-2" {
		t.Fatalf("direct row = %+v, %v", got, err)
	}
	if changed, err := r.MirrorCompanionCreds(target.ID); err != nil || changed {
		t.Fatalf("a second call changed something: %v, %v", changed, err)
	}
	if changed, err := r.MirrorCompanionCreds("missing"); err != nil || changed {
		t.Fatalf("an unknown target changed something: %v, %v", changed, err)
	}
}

func TestConnectCompanionLinksAndMirrorsInOneStep(t *testing.T) {
	r, _ := migratedStore(t)
	target := richTarget(t, r)
	lost, err := r.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "B2 old", Repo: "b2:bkt:containers-direct", Enabled: true, CompanionLost: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ConnectCompanion(lost.ID, target.ID); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetNamedRepo(lost.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CompanionOf != target.ID || got.CompanionLost || !got.MirroredEqual(target) {
		t.Fatalf("connected row = %+v", got)
	}
	other, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "x", Repo: "b2:bkt:x", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ConnectCompanion(other.ID, target.ID); !errors.Is(err, store.ErrCompanionTaken) {
		t.Fatalf("second repository for one target: %v", err)
	}
	if err := r.ConnectCompanion(other.ID, "missing"); !errors.Is(err, store.ErrNotOffsiteTarget) {
		t.Fatalf("unknown target: %v", err)
	}
	vms := store.SeedOffsiteTarget(t, r, "vms", "b2:bkt:vms")
	if err := r.ConnectCompanion("missing", vms.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown repository: %v", err)
	}
}

func TestDeleteOffsiteTargetIfUnusedRefusesWhileAnItemUsesItsDirectRepository(t *testing.T) {
	r, _ := migratedStore(t)
	target := store.SeedOffsiteTarget(t, r, "containers", "b2:bkt:containers")
	direct := store.SeedCompanion(t, r, target)
	item := store.ItemRef{Domain: "containers", Key: "web"}
	if _, err := r.WritePlacement(item, &store.HomeWrite{Repo: direct.ID, Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}
	use, err := r.DeleteOffsiteTargetIfUnused(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !use.InUse() || use.Items != 1 || use.CompanionID != direct.ID {
		t.Fatalf("use = %+v", use)
	}
	if _, found, _ := r.GetOffsiteTarget(target.ID); !found {
		t.Fatal("the target was deleted while its direct repository is in use")
	}
}

func TestDeleteOffsiteTargetIfUnusedRefusesWhileADefaultPointsAtItsDirectRepository(t *testing.T) {
	r, _ := migratedStore(t)
	target := store.SeedOffsiteTarget(t, r, "vms", "b2:bkt:vms")
	direct := store.SeedCompanion(t, r, target)
	store.SeedDefault(t, r, "vms", direct.ID)
	use, err := r.DeleteOffsiteTargetIfUnused(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !use.InUse() || use.Items != 0 || !slices.Equal(use.DefaultDomains, []string{"vms"}) {
		t.Fatalf("use = %+v", use)
	}
}

func TestDeleteOffsiteTargetIfUnusedTakesTheUnusedDirectRepositoryWithIt(t *testing.T) {
	r, _ := migratedStore(t)
	target := store.SeedOffsiteTarget(t, r, "files", "b2:bkt:files")
	direct := store.SeedCompanion(t, r, target)
	use, err := r.DeleteOffsiteTargetIfUnused(target.ID)
	if err != nil || use.InUse() {
		t.Fatalf("use = %+v, err %v", use, err)
	}
	if _, found, _ := r.GetOffsiteTarget(target.ID); found {
		t.Fatal("the target is still there")
	}
	if _, err := r.GetNamedRepo(direct.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("the direct repository: %v", err)
	}
	plain := store.SeedOffsiteTarget(t, r, "files", "b2:bkt:plain")
	if use, err := r.DeleteOffsiteTargetIfUnused(plain.ID); err != nil || use.InUse() {
		t.Fatalf("a target without a direct repository: %+v, %v", use, err)
	}
	if _, found, _ := r.GetOffsiteTarget(plain.ID); found {
		t.Fatal("a target without a direct repository was not deleted")
	}
}

func TestDeleteOffsiteTargetLeavesItsDirectRepositoryAsAPlainOne(t *testing.T) {
	r, _ := migratedStore(t)
	target := richTarget(t, r)
	direct := store.SeedCompanion(t, r, target)
	if err := r.DeleteOffsiteTarget(target.ID); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetNamedRepo(direct.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CompanionOf != "" || !got.CompanionLost || !got.MirroredEqual(target) {
		t.Fatalf("after the import path the row is %+v", got)
	}
}
