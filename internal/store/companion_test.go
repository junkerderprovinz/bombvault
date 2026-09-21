package store_test

import (
	"database/sql"
	"errors"
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
