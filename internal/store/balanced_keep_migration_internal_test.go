package store

import (
	"bytes"
	"log"
	"reflect"
	"strings"
	"testing"
)

var (
	oldBalanced = RetentionKeep{KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 6, KeepYearly: 1}
	newBalanced = RetentionKeep{KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 6, KeepYearly: 2}
)

func keptDestination(t *testing.T, r *Repo, name string, keep RetentionKeep) OffsiteTarget {
	t.Helper()
	d, err := r.SaveDestination(OffsiteTarget{
		Name: name, Repo: "rclone:" + name + ":bv", Provider: "box",
		RetentionKeepLast: keep.KeepLast, RetentionKeepDaily: keep.KeepDaily, RetentionKeepWeekly: keep.KeepWeekly,
		RetentionKeepMonthly: keep.KeepMonthly, RetentionKeepYearly: keep.KeepYearly,
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestTheUpgradeBringsEveryPlaceOnBalancedAlong(t *testing.T) {
	db := OpenMem(t)
	bootForTodaysCode(t, db, balancedKeepMigration-1)
	r := New(db)
	box := keptDestination(t, r, "Box", oldBalanced)
	vault := keptDestination(t, r, "Vault", RetentionKeep{KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 6, KeepYearly: 3})
	copyAtBox, _, err := r.EnsureDestinationTarget(box.ID, "flash", box.Repo+"/flash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.MutateSettings(func(s *Settings) error {
		s.RetentionKeepDaily, s.RetentionKeepWeekly, s.RetentionKeepMonthly, s.RetentionKeepYearly = 7, 4, 6, 1
		s.OffsiteRetentionKeepDaily, s.OffsiteRetentionKeepWeekly, s.OffsiteRetentionKeepMonthly, s.OffsiteRetentionKeepYearly = 7, 4, 6, 1
		s.SetOwnRetention(map[string]RetentionKeep{"containers": oldBalanced, "vms": {KeepLast: 5}})
		s.SetOwnOffsiteRetention(map[string]RetentionKeep{"files": oldBalanced})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	out, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(out); log.SetFlags(flags) })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.RetentionKeepYearly != 2 || s.OffsiteRetentionKeepYearly != 2 {
		t.Errorf("shared yearly = %d, off-site %d, want 2 for both", s.RetentionKeepYearly, s.OffsiteRetentionKeepYearly)
	}
	if got, want := s.OwnRetention(), map[string]RetentionKeep{"containers": newBalanced, "vms": {KeepLast: 5}}; !reflect.DeepEqual(got, want) {
		t.Errorf("own rules = %+v, want %+v", got, want)
	}
	if got, want := s.OwnOffsiteRetention(), map[string]RetentionKeep{"files": newBalanced}; !reflect.DeepEqual(got, want) {
		t.Errorf("own off-site rules = %+v, want %+v", got, want)
	}
	for _, c := range []struct {
		id   string
		want int
	}{{box.ID, 2}, {copyAtBox.ID, 2}, {vault.ID, 3}} {
		var yearly int
		if err := db.QueryRow(`SELECT retention_keep_yearly FROM offsite_targets WHERE id = ?`, c.id).Scan(&yearly); err != nil {
			t.Fatal(err)
		}
		if yearly != c.want {
			t.Errorf("row %s keeps %d yearly, want %d", c.id, yearly, c.want)
		}
	}

	var lines []string
	for line := range strings.Lines(buf.String()) {
		if strings.HasPrefix(line, "retention: ") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	want := []string{
		"retention: Box stays on Balanced and keeps 2 yearly instead of 1",
		"retention: Box for flash stays on Balanced and keeps 2 yearly instead of 1",
		"retention: the off-site rule of files stays on Balanced and keeps 2 yearly instead of 1",
		"retention: the own rule of containers stays on Balanced and keeps 2 yearly instead of 1",
		"retention: the shared local rule stays on Balanced and keeps 2 yearly instead of 1",
		"retention: the shared off-site rule stays on Balanced and keeps 2 yearly instead of 1",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("logged %q, want %q", lines, want)
	}
}

func TestTheUpgradeLeavesAnInstallWithoutBalancedAlone(t *testing.T) {
	db := OpenMem(t)
	bootForTodaysCode(t, db, balancedKeepMigration-1)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	s, err := New(db).GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.RetentionKeepYearly != 0 || s.RetentionOverrides != "" || s.OffsiteRetentionOverrides != "" {
		t.Errorf("settings = yearly %d, own %q, off-site %q, want them untouched", s.RetentionKeepYearly, s.RetentionOverrides, s.OffsiteRetentionOverrides)
	}
}
