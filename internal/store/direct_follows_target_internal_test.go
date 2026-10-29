package store

import (
	"testing"
)

func TestAnUpgradeGivesADirectRepositoryItsTargetsSettingsAndKeepsAppendOnly(t *testing.T) {
	db := openMigrated(t)
	r := New(db)
	target, err := r.CreateOffsiteTarget(OffsiteTarget{Domain: "containers", Name: "Mani", Repo: "s3:mani/containers", Enabled: true, LimitUpload: 500})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := r.CreateCompanionRepo(target.ID, "", "s3:mani/containers-direct")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE offsite_targets SET immutable = 1, limit_upload = 9, compression = 'max' WHERE id = ?`, direct.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = ?`, directFollowsTargetMigration); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	got, err := r.GetNamedRepo(direct.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, _, err := r.GetOffsiteTarget(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Immutable || !after.Immutable {
		t.Fatalf("append-only: direct %v, target %v, want both on", got.Immutable, after.Immutable)
	}
	if got.LimitUpload != 500 || got.Compression != "" {
		t.Fatalf("direct = limit %d, compression %q, want the target's 500 and automatic", got.LimitUpload, got.Compression)
	}
}

func TestADirectRepositorySwitchesWithItsTarget(t *testing.T) {
	r := New(openMigrated(t))
	target, err := r.CreateOffsiteTarget(OffsiteTarget{Domain: "containers", Name: "Mani", Repo: "s3:mani/containers", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := r.CreateCompanionRepo(target.ID, "", "s3:mani/containers-direct")
	if err != nil {
		t.Fatal(err)
	}
	enabled := func() bool {
		t.Helper()
		d, err := r.GetNamedRepo(direct.ID)
		if err != nil {
			t.Fatal(err)
		}
		return d.Enabled
	}

	target.Enabled = false
	if target, err = r.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	if enabled() {
		t.Fatal("the direct repository stayed on after its target went off")
	}
	target.Enabled = true
	if target, err = r.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	if !enabled() {
		t.Fatal("the direct repository stayed off after its target came back on")
	}
}

func TestADirectRepositorySwitchedOffOnItsOwnStaysOffThroughOtherSaves(t *testing.T) {
	r := New(openMigrated(t))
	target, err := r.CreateOffsiteTarget(OffsiteTarget{Domain: "containers", Name: "Mani", Repo: "s3:mani/containers", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := r.CreateCompanionRepo(target.ID, "", "s3:mani/containers-direct")
	if err != nil {
		t.Fatal(err)
	}
	direct.Enabled = false
	if _, err := r.UpsertOffsiteTarget(direct); err != nil {
		t.Fatal(err)
	}
	target.LimitUpload = 100
	if _, err := r.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	if d, _ := r.GetNamedRepo(direct.ID); d.Enabled {
		t.Fatal("a save that left the target's switch alone switched its direct repository on")
	}
}
