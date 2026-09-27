package restic

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestImportDirDatesTheSnapshotBeforeThePositional(t *testing.T) {
	at := time.Date(2025, 2, 1, 3, 4, 5, 0, time.Local)
	args := ImportDirArgs("/repo", []string{"container:plex", "import:ab_20250201_030405"}, at, Mode{Encrypted: true})
	sep := slices.Index(args, "--")
	ti := slices.Index(args, "--time")
	if ti < 0 || ti > sep || args[ti+1] != "2025-02-01 03:04:05" {
		t.Fatalf("args = %v, want --time with the local date before --", args)
	}
	if args[len(args)-1] != "." || !slices.Contains(args, "import:ab_20250201_030405") {
		t.Fatalf("args = %v, want the tags and the positional .", args)
	}
}

// An imported copy is laid out below the staging folder the way the container
// paths look, and the snapshot has to hold those paths at its root, dated when
// the copy was made rather than now.
func TestImportDirKeepsTheLayoutAndTheDate(t *testing.T) {
	r, repo, m := zfsdirRepo(t)
	dir := t.TempDir()
	zfsdirTree(t, dir, "host/user/user/appdata/plex/Preferences.xml")
	at := time.Date(2024, 11, 5, 3, 0, 0, 0, time.Local)

	sum, err := r.ImportDir(context.Background(), repo, dir, []string{"container:plex", "import:ab_20241105_030000"}, at, m)
	if err != nil {
		t.Fatalf("ImportDir: %v", err)
	}
	got := zfsdirEntries(t, r, repo, sum.SnapshotID, m)
	if len(got) != 1 || got[0] != "/host/user/user/appdata/plex/Preferences.xml" {
		t.Fatalf("snapshot holds %v", got)
	}
	snaps, err := r.Snapshots(context.Background(), repo, m)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("snapshots %v %v", snaps, err)
	}
	when, err := time.Parse(time.RFC3339Nano, snaps[0].Time)
	if err != nil || !when.Equal(at) {
		t.Fatalf("snapshot time %q, want %v", snaps[0].Time, at)
	}
}
