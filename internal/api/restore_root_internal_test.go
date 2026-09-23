package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// commonAncestor picks the node a to-folder restore extracts, which has to cover
// every recorded root of a multi-root snapshot.
func TestCommonAncestor(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  string
	}{
		{"no path at all", nil, ""},
		{
			// Not path.Clean'd: a recorded path is a restic selector and has to
			// reach the engine exactly as recorded.
			"a single root is handed back unchanged",
			[]string{"/host/olduser/data/docs/"},
			"/host/olduser/data/docs/",
		},
		{
			"two siblings share their parent",
			[]string{"/host/user/data/docs/keep-a", "/host/user/data/docs/keep-b"},
			"/host/user/data/docs",
		},
		{
			// Restoring the deeper path alone would drop the other.
			"a root and something below it resolve to the root",
			[]string{"/host/user/data/docs", "/host/user/data/docs/keep-a"},
			"/host/user/data/docs",
		},
		{
			"three roots trim to the deepest shared node",
			[]string{"/srv/a/b/one", "/srv/a/b/two/deep", "/srv/a/b/three"},
			"/srv/a/b",
		},
		{
			// Segment-aligned, like isStrictDescendant.
			"a name that merely starts with another is not below it",
			[]string{"/host/user/data", "/host/user-old/data"},
			"/host",
		},
		{
			// Sharing only "/" means a whole-tree restore; "<id>:/" is not a
			// valid selector.
			"roots on different branches have no usable ancestor",
			[]string{"/host/user/data", "/mnt/disk2/other"},
			"",
		},
		{
			// Rebuilding the path from its segments must not add a leading "/"
			// that the recorded path never had.
			"a path that does not start at / keeps its own head",
			[]string{`C:\tmp\Test001/data/docs/keep-a`, `C:\tmp\Test001/data/docs/keep-b`},
			`C:\tmp\Test001/data/docs`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := commonAncestor(tc.paths); got != tc.want {
				t.Fatalf("commonAncestor(%v) = %q, want %q", tc.paths, got, tc.want)
			}
		})
	}
}

// The snapshot is matched by exact id or prefix; anything unmatched yields ""
// for a whole-tree restore rather than another snapshot's paths.
func TestSnapshotRestoreRoot(t *testing.T) {
	snaps := []restic.Snapshot{
		{ID: "aaaa1111", Paths: []string{"/host/user/data/docs/keep-a", "/host/user/data/docs/keep-b"}},
		{ID: "bbbb2222", Paths: []string{"/host/user/data/other"}},
		{ID: "cccc3333"},
	}
	for _, tc := range []struct{ id, want string }{
		{"aaaa1111", "/host/user/data/docs"},
		{"aaaa", "/host/user/data/docs"}, // prefix match, same as snapshotBelongs
		{"bbbb2222", "/host/user/data/other"},
		{"cccc3333", ""}, // recorded no path
		{"ffff9999", ""}, // no such snapshot
	} {
		if got := snapshotRestoreRoot(snaps, tc.id); got != tc.want {
			t.Fatalf("snapshotRestoreRoot(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

// A folder name becomes a restic --exclude pattern that matches exactly that
// folder. The bracket cases are failures seen with the real engine.
func TestEscapeGlobLiteral(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		// Plain names pass through unchanged, or every stored selection changes.
		{"/host/user/appdata/plex/Cache", "/host/user/appdata/plex/Cache"},
		// A character class that cannot match the name it came from.
		{"/media/Movies/Inception (2010) [1080p]", `/media/Movies/Inception (2010) \[1080p\]`},
		// A class that matches the siblings "Season 0" and "Season 1" instead.
		{"/media/Season [01]", `/media/Season \[01\]`},
		// An unmatched bracket makes restic refuse the whole backup run.
		{"/media/Movies [2024", `/media/Movies \[2024`},
		{"/media/star*name", `/media/star\*name`},
		{"/media/q?mark", `/media/q\?mark`},
		{`/media/back\slash`, `/media/back\\slash`},
		{"", ""},
	} {
		if got := escapeGlobLiteral(tc.in); got != tc.want {
			t.Errorf("escapeGlobLiteral(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The escaping happens where a stored path becomes a pattern, so the container
// and the file-set compile both get it.
func TestExcludedBranchesEscapesDerivedPatterns(t *testing.T) {
	got := excludedBranches([]string{
		"/media",
		"!/media/Season [01]",
		"!/media/plain",
	})
	want := []string{`/media/Season \[01\]`, "/media/plain"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("excludedBranches = %v, want %v", got, want)
	}
}

// An include below an exclusion that is itself below another include wins over
// the exclusion. PruneMaximal compares within one class only, so without this
// the include is dropped as redundant, the exclusion carves the folder out of
// every snapshot, and the stored form loses the fact that it was wanted.
func TestNormalizeSelectionResolvesIncludeUnderExclusion(t *testing.T) {
	cases := []struct {
		name    string
		entries []string
		want    []string
	}{
		{
			"an include below an exclusion drops the exclusion, not itself",
			[]string{"/c/plex", "/c/plex/Cache/Metadata", "!/c/plex/Cache"},
			[]string{"/c/plex"},
		},
		{
			"an exclusion with no include under it survives",
			[]string{"/c/plex", "!/c/plex/Cache"},
			[]string{"/c/plex", "!/c/plex/Cache"},
		},
		{
			// An exclusions-only list is how an explicitly deselected container
			// differs from auto-detect, so it has to stay as it is.
			"an exclusions-only selection is left alone",
			[]string{"!/c/plex/Cache", "!/c/plex/Logs"},
			[]string{"!/c/plex/Cache", "!/c/plex/Logs"},
		},
		{
			"only the contradicted exclusion is dropped",
			[]string{"/c/plex", "/c/plex/Cache/Metadata", "!/c/plex/Cache", "!/c/plex/Logs"},
			[]string{"/c/plex", "!/c/plex/Logs"},
		},
		{
			"a name that merely starts with the exclusion is not below it",
			[]string{"/c/plex", "/c/plex/CacheOld", "!/c/plex/Cache"},
			[]string{"/c/plex", "!/c/plex/Cache"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeSelection(tc.entries)
			if len(got) != len(tc.want) {
				t.Fatalf("NormalizeSelection(%v) = %v, want %v", tc.entries, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("NormalizeSelection(%v) = %v, want %v", tc.entries, got, tc.want)
				}
			}
			// The stored form relies on normalizing being idempotent.
			if again := NormalizeSelection(got); len(again) != len(got) {
				t.Fatalf("not idempotent: %v -> %v", got, again)
			}
		})
	}
}

// The restic positionals and the --exclude tail have to come from the same read
// of the target row. Two reads with a save between them would pair old
// positionals with new exclusions, a selection the user never made, and the
// snapshot would record a path whose content was filtered out. The race itself
// has no seam to inject a write, so this checks that one call returns both
// halves consistently.
func TestEffectiveBackupPathsWithSelectionIsOneRead(t *testing.T) {
	dir := t.TempDir()
	st := newTestStore(t)
	svc := NewService(config.Config{
		AppKey:        strings.Repeat("a", 64),
		DataDir:       dir,
		HostMountRoot: dir,
	}, st, nil, nil, nil)

	root := filepath.Join(dir, "appdata", "plex")
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
		t.Fatal(err)
	}
	stored := []string{root, "!" + root + "/transcoding"}
	if err := st.SetBackupPaths("plex", stored); err != nil {
		t.Fatal(err)
	}

	paths, selection := svc.effectiveBackupPathsWithSelection("plex", model.Inspect{})
	if len(paths) != 1 || paths[0] != root {
		t.Fatalf("paths = %v, want [%s]", paths, root)
	}
	// The selection comes back whole, exclusions included, because the
	// --exclude tail is derived from it.
	if len(selection) != len(stored) {
		t.Fatalf("selection = %v, want %v", selection, stored)
	}
	for i := range stored {
		if selection[i] != stored[i] {
			t.Fatalf("selection = %v, want %v", selection, stored)
		}
	}
	inc := includesOnly(selection)
	if len(inc) != len(paths) || inc[0] != paths[0] {
		t.Fatalf("positionals %v do not match the selection's includes %v", paths, inc)
	}
}

// The test above checks the helper, not that Backup uses its selection for the
// --exclude tail. The two statements sit far apart in one long function and no
// behavioural test can land a write between them, so this scans the source, as
// TestFilesCancelKeyMatchesTheProgressKey does.
func TestBackupExcludesComeFromTheSameReadAsThePositionals(t *testing.T) {
	raw, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	src := string(raw)

	if !strings.Contains(src, "effective, selection := s.effectiveBackupPathsWithSelection(name, in)") {
		t.Error("Backup no longer takes both halves of the selection from one read.")
	}
	if !strings.Contains(src, "excludedBranches(selection)") {
		t.Error("the container backup's --exclude tail is no longer derived from the same read as its positionals")
	}
	for _, forbidden := range []string{
		"excludedBranches(tg.SelectedPaths)",
		"_ = selection",
	} {
		if strings.Contains(src, forbidden) {
			t.Errorf("Backup is back to the two-read shape (%s).\n"+
				"tg comes from UpsertTarget's re-read, so a save landing between the two\n"+
				"reads pairs old positionals with new exclusions: a --exclude from one\n"+
				"selection biting a positional from another, recorded as a success.", forbidden)
		}
	}
}
