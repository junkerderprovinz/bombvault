package store

import (
	"bytes"
	"database/sql"
	"log"
	"reflect"
	"strings"
	"testing"
)

// plansBeforeTheUpgrade is a database one step before ownPlansCountMigration
// whose switch for own plans is on or off, holding one own plan per kind.
func plansBeforeTheUpgrade(t *testing.T, on bool) *sql.DB {
	t.Helper()
	db := OpenMem(t)
	bootAs(t, db, ownPlansCountMigration-1)
	if _, err := db.Exec(`UPDATE settings SET per_item_schedules = ?`, boolInt(on)); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	if _, err := r.UpsertTarget(Target{ContainerName: "plex", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetScheduleCadence("plex", "daily 06:00"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpsertVMTarget(VMTarget{Name: "win11", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetVMScheduleCadence("win11", "off"); err != nil {
		t.Fatal(err)
	}
	set, err := r.CreateFileSet(FileSet{Name: "My_Backups", Path: "/mnt/user/my_backups", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetFileSetScheduleCadence(set.ID, "weekly sun 04:00"); err != nil {
		t.Fatal(err)
	}
	ds, err := r.CreateZFSDataset(ZFSDataset{Dataset: "tank/media", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetZFSDatasetScheduleCadence(ds.ID, "weekly sat 02:00"); err != nil {
		t.Fatal(err)
	}
	return db
}

func storedPlans(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT schedule_cadence FROM targets
UNION ALL SELECT schedule_cadence FROM vms
UNION ALL SELECT schedule_cadence FROM file_sets
UNION ALL SELECT schedule_cadence FROM zfs_datasets`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // read-only
	var plans []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		plans = append(plans, p)
	}
	return plans
}

func TestTheUpgradeClearsOwnPlansTheSwitchLeftUnused(t *testing.T) {
	db := plansBeforeTheUpgrade(t, false)
	var buf bytes.Buffer
	out, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(out); log.SetFlags(flags) })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if got, want := storedPlans(t, db), []string{"", "", "", ""}; !reflect.DeepEqual(got, want) {
		t.Fatalf("plans = %q, want every one cleared", got)
	}
	var got []string
	for line := range strings.Lines(buf.String()) {
		if strings.Contains(line, "own plan") {
			got = append(got, strings.TrimSpace(line))
		}
	}
	want := []string{
		`schedule: cleared the unused own plan "daily 06:00" of container plex, so it keeps following its section`,
		`schedule: cleared the unused own plan "weekly sun 04:00" of folder set My_Backups, so it keeps following its section`,
		`schedule: cleared the unused own plan "off" of VM win11, so it keeps following its section`,
		`schedule: cleared the unused own plan "weekly sat 02:00" of ZFS item tank/media, so it keeps following its section`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("logged %q, want %q", got, want)
	}
}

func TestTheUpgradeKeepsOwnPlansThatAlreadyCounted(t *testing.T) {
	db := plansBeforeTheUpgrade(t, true)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	want := []string{"daily 06:00", "off", "weekly sun 04:00", "weekly sat 02:00"}
	if got := storedPlans(t, db); !reflect.DeepEqual(got, want) {
		t.Fatalf("plans = %q, want %q", got, want)
	}
}
