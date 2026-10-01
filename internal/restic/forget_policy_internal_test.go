package restic

import (
	"maps"
	"slices"
	"testing"
	"time"
)

func at(id, when string, tags ...string) Snapshot {
	return Snapshot{ID: id, Time: when, Tags: tags}
}

func removed(t *testing.T, p RetentionPolicy, snaps ...Snapshot) []string {
	t.Helper()
	got, ok := p.Forgets(snaps)
	if !ok {
		t.Fatalf("Forgets(%v) could not tell", snaps)
	}
	return slices.Sorted(maps.Keys(got))
}

func TestKeepLastForgetsAllButTheNewest(t *testing.T) {
	got := removed(t, RetentionPolicy{KeepLast: 2},
		at("a", "2026-09-27T18:00:00Z"), at("c", "2026-09-27T18:06:00Z"),
		at("b", "2026-09-27T18:03:00Z"), at("d", "2026-09-27T17:00:00Z"))
	if !slices.Equal(got, []string{"a", "d"}) {
		t.Fatalf("removed %v, want [a d]", got)
	}
}

func TestKeepDailyKeepsTheNewestOfEachDay(t *testing.T) {
	got := removed(t, RetentionPolicy{KeepDaily: 2},
		at("mon1", "2026-09-21T08:00:00Z"), at("mon2", "2026-09-21T20:00:00Z"),
		at("tue1", "2026-09-22T08:00:00Z"), at("tue2", "2026-09-22T20:00:00Z"),
		at("wed1", "2026-09-23T08:00:00Z"))
	if !slices.Equal(got, []string{"mon1", "mon2", "tue1"}) {
		t.Fatalf("removed %v, want [mon1 mon2 tue1]", got)
	}
}

func TestADayBucketWithCountsLeftKeepsTheOldestSnapshot(t *testing.T) {
	got := removed(t, RetentionPolicy{KeepDaily: 5},
		at("mon1", "2026-09-21T08:00:00Z"), at("mon2", "2026-09-21T20:00:00Z"),
		at("tue1", "2026-09-22T08:00:00Z"), at("tue2", "2026-09-22T20:00:00Z"))
	if !slices.Equal(got, []string{"tue1"}) {
		t.Fatalf("removed %v, want [tue1]", got)
	}
}

func TestTheDayIsTheOneOfTheSnapshotsOwnOffset(t *testing.T) {
	// In UTC both of the first two fall on the 21st; restic buckets by the offset
	// the snapshot was written with, where they are two days.
	got := removed(t, RetentionPolicy{KeepDaily: 2},
		at("early", "2026-09-22T00:30:00+02:00"), at("late", "2026-09-21T23:30:00+02:00"),
		at("older", "2026-09-20T12:00:00+02:00"))
	if !slices.Equal(got, []string{"older"}) {
		t.Fatalf("removed %v, want [older]", got)
	}
}

func TestWeeksAndMonthsCountTogetherWithTheLast(t *testing.T) {
	got := removed(t, RetentionPolicy{KeepLast: 1, KeepWeekly: 2, KeepMonthly: 2},
		at("jul", "2026-07-15T10:00:00Z"),
		at("aug", "2026-08-12T10:00:00Z"),
		at("sep-w38", "2026-09-14T10:00:00Z"),
		at("sep-w39a", "2026-09-21T10:00:00Z"),
		at("sep-w39b", "2026-09-22T10:00:00Z"),
		at("sep-w39c", "2026-09-23T10:00:00Z"))
	if !slices.Equal(got, []string{"jul", "sep-w39a", "sep-w39b"}) {
		t.Fatalf("removed %v, want [jul sep-w39a sep-w39b]", got)
	}
}

func TestKeepYearlyKeepsTheNewestOfEachYear(t *testing.T) {
	got := removed(t, RetentionPolicy{KeepYearly: 2},
		at("y24", "2024-12-30T10:00:00Z"),
		at("y25a", "2025-03-01T10:00:00Z"), at("y25b", "2025-11-01T10:00:00Z"),
		at("y26a", "2026-01-05T10:00:00Z"), at("y26b", "2026-09-22T10:00:00Z"))
	if !slices.Equal(got, []string{"y24", "y25a", "y26a"}) {
		t.Fatalf("removed %v, want [y24 y25a y26a]", got)
	}
}

func TestADirectSnapshotIsKeptWithoutUsingACount(t *testing.T) {
	got := removed(t, RetentionPolicy{KeepLast: 1},
		at("old", "2026-09-20T10:00:00Z", DirectTag),
		at("mid", "2026-09-21T10:00:00Z"),
		at("new", "2026-09-22T10:00:00Z"))
	if !slices.Equal(got, []string{"mid"}) {
		t.Fatalf("removed %v, want [mid]", got)
	}
	got = removed(t, RetentionPolicy{KeepLast: 1, Direct: true},
		at("old", "2026-09-20T10:00:00Z", DirectTag),
		at("new", "2026-09-22T10:00:00Z", DirectTag))
	if !slices.Equal(got, []string{"old"}) {
		t.Fatalf("under a direct repository's own rules removed %v, want [old]", got)
	}
}

func TestAnUnreadableTimeLeavesTheAnswerOpen(t *testing.T) {
	if _, ok := (RetentionPolicy{KeepLast: 1}).Forgets([]Snapshot{at("a", "yesterday"), at("b", time.Now().Format(time.RFC3339))}); ok {
		t.Fatal("Forgets answered for a snapshot whose time it cannot read")
	}
}

func TestTwoSnapshotsOfTheSameInstantLeaveTheAnswerOpen(t *testing.T) {
	// restic orders them by its own listing, which the caller does not know.
	if _, ok := (RetentionPolicy{KeepLast: 1}).Forgets([]Snapshot{at("a", "2026-09-22T10:00:00Z"), at("b", "2026-09-22T10:00:00Z")}); ok {
		t.Fatal("Forgets answered for two snapshots restic could order either way")
	}
}
