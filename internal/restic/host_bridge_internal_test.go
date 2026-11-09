package restic

import (
	"slices"
	"testing"
)

func TestBridgeParentIsTheNewestLegacySnapshotUntilTheOwnHostHasOne(t *testing.T) {
	legacy := []Snapshot{
		{ID: "old", Hostname: LegacyHost, Time: "2026-11-01T02:00:00.5+01:00"},
		{ID: "newest", Hostname: LegacyHost, Time: "2026-11-08T02:00:00+01:00"},
		{ID: "older", Hostname: LegacyHost, Time: "2026-11-07T02:00:00+01:00"},
	}
	if parent, settled := bridgeParent(legacy, "bv-0123456789ab"); parent != "newest" || settled {
		t.Fatalf("parent %q, settled %v, want the newest legacy snapshot", parent, settled)
	}
	own := append(slices.Clone(legacy), Snapshot{ID: "own", Hostname: "bv-0123456789ab", Time: "2026-11-09T02:00:00+01:00"})
	if parent, settled := bridgeParent(own, "bv-0123456789ab"); parent != "" || !settled {
		t.Fatalf("parent %q, settled %v, want restic to find the parent itself", parent, settled)
	}
	if parent, _ := bridgeParent(nil, "bv-0123456789ab"); parent != "" {
		t.Fatalf("an item without snapshots got the parent %q", parent)
	}
}

func TestParentCandidatesAreTheItemsSnapshotsUnderEitherHost(t *testing.T) {
	got := ParentCandidatesArgs("/repo", Mode{Host: "bv-0123456789ab"}, []string{"zfs:tank", "p1"}, nil)
	want := []string{"-r", "/repo", "snapshots", "--insecure-no-password", "--no-lock", "--json",
		"--host", "bv-0123456789ab", "--host", LegacyHost, "--tag", "zfs:tank,p1"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	got = ParentCandidatesArgs("/repo", Mode{Host: "bv-0123456789ab", Encrypted: true}, nil, []string{"/a", "/b"})
	want = []string{"-r", "/repo", "snapshots", "--no-lock", "--json",
		"--host", "bv-0123456789ab", "--host", LegacyHost, "--path", "/a", "--path", "/b"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
