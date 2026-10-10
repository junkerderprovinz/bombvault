package zfsrepl

import (
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// daily returns one replica snapshot name per day at 03:00 UTC for n days
// ending on 2026-10-09, oldest first.
func daily(n int) []string {
	end := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)
	out := make([]string, n)
	for i := range n {
		out[i] = zfs.ReplicaSnapshotName(end.AddDate(0, 0, i-n+1))
	}
	return out
}

func TestExpiredKeepsTheNewestN(t *testing.T) {
	names := daily(5)
	got := Expired(names, store.RetentionKeep{KeepLast: 2}, names[4], time.UTC)
	if !slices.Equal(got, names[:3]) {
		t.Errorf("Expired = %q, want the three oldest", got)
	}
}

func TestExpiredKeepsTheNewestSnapshotOfEachDay(t *testing.T) {
	day := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	names := []string{
		zfs.ReplicaSnapshotName(day.Add(1 * time.Hour)),
		zfs.ReplicaSnapshotName(day.Add(13 * time.Hour)),
		zfs.ReplicaSnapshotName(day.Add(25 * time.Hour)),
		zfs.ReplicaSnapshotName(day.Add(37 * time.Hour)),
	}
	got := Expired(names, store.RetentionKeep{KeepDaily: 2}, names[3], time.UTC)
	if !slices.Equal(got, []string{names[0], names[2]}) {
		t.Errorf("Expired = %q, want the earlier snapshot of each day", got)
	}
}

func TestExpiredCountsDaysInTheGivenZone(t *testing.T) {
	// 21:30 and 22:30 UTC on one day are two days in Vienna's summer time.
	names := []string{"bombvault-replica-20261008213000", "bombvault-replica-20261008223000"}
	vienna, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		t.Skip("no zone database")
	}
	if got := Expired(names, store.RetentionKeep{KeepDaily: 2}, "", vienna); got != nil {
		t.Errorf("Expired in Vienna = %q, want both kept", got)
	}
	if got := Expired(names, store.RetentionKeep{KeepDaily: 2}, "", time.UTC); !slices.Equal(got, names[:1]) {
		t.Errorf("Expired in UTC = %q, want the earlier one gone", got)
	}
}

func TestExpiredCombinesTheRules(t *testing.T) {
	// Seven daily and three weekly over sixty days keep the last seven days
	// and the newest snapshot of the three most recent ISO weeks.
	names := daily(60)
	got := Expired(names, store.RetentionKeep{KeepDaily: 7, KeepWeekly: 3}, names[59], time.UTC)
	kept := slices.DeleteFunc(slices.Clone(names), func(n string) bool { return slices.Contains(got, n) })
	// 2026-10-09 is a Friday; the weeks before end on Sunday 10-04 and 09-27.
	want := append([]string{"bombvault-replica-20260927030000"}, names[53:]...)
	if !slices.Equal(kept, want) {
		t.Errorf("kept = %q, want %q", kept, want)
	}
}

func TestExpiredKeepsMonthsAndYears(t *testing.T) {
	names := []string{
		"bombvault-replica-20240615030000",
		"bombvault-replica-20250101030000",
		"bombvault-replica-20251231030000",
		"bombvault-replica-20260801030000",
		"bombvault-replica-20260815030000",
		"bombvault-replica-20260901030000",
	}
	got := Expired(names, store.RetentionKeep{KeepMonthly: 2, KeepYearly: 2}, names[5], time.UTC)
	if want := []string{names[0], names[1], names[3]}; !slices.Equal(got, want) {
		t.Errorf("Expired = %q, want %q", got, want)
	}
}

func TestExpiredNeverTouchesForeignNamesOrTheBase(t *testing.T) {
	names := append([]string{"autosnap_2026-10-01_daily", "bombvault-20261001030000", "bombvault-prerestore-20261001030000"}, daily(3)...)
	got := Expired(names, store.RetentionKeep{KeepLast: 1}, names[3], time.UTC)
	if !slices.Equal(got, []string{names[4]}) {
		t.Errorf("Expired = %q, want only the middle replica snapshot", got)
	}
}

func TestExpiredWithoutRulesKeepsEverything(t *testing.T) {
	if got := Expired(daily(30), store.RetentionKeep{}, "", time.UTC); got != nil {
		t.Errorf("Expired = %q, want nil", got)
	}
}
