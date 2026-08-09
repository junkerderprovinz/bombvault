package api

import (
	"path"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// TestContainerAppdataRemapSinglePath: one appdata path is restored into
// <destBase>/<basename> and the bindRemap maps its HOST source path to the HOST
// dest path (toHostPath round-trip: /host/user/... -> /mnt/...).
func TestContainerAppdataRemapSinglePath(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	dirs, remap := s.containerAppdataRemap("/host/user/user/appdata", []string{"/host/user/zfs/appdata/xo"})

	if len(dirs) != 1 || dirs[0].Subtree != "/host/user/zfs/appdata/xo" || dirs[0].Target != "/host/user/user/appdata/xo" {
		t.Fatalf("dirs = %+v, want one {Subtree:/host/user/zfs/appdata/xo, Target:/host/user/user/appdata/xo}", dirs)
	}
	if got := remap["/mnt/zfs/appdata/xo"]; got != "/mnt/user/appdata/xo" {
		t.Fatalf("bindRemap[/mnt/zfs/appdata/xo] = %q, want /mnt/user/appdata/xo (remap=%v)", got, remap)
	}
}

// TestContainerAppdataRemapNoopWhenDestEqualsSource: a standard container whose
// appdata already lives under the destBase remaps to the SAME path (Target==source),
// so restoring to the default destination lands it exactly where it was.
func TestContainerAppdataRemapNoopWhenDestEqualsSource(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	dirs, _ := s.containerAppdataRemap("/host/user/user/appdata", []string{"/host/user/user/appdata/web"})
	if len(dirs) != 1 || dirs[0].Target != "/host/user/user/appdata/web" || dirs[0].Subtree != dirs[0].Target {
		t.Fatalf("dirs = %+v, want Target==Subtree==/host/user/user/appdata/web", dirs)
	}
}

// TestContainerAppdataRemapDistinctBasenames: two appdata paths with different
// leaves map to two distinct targets, no dedup suffix.
func TestContainerAppdataRemapDistinctBasenames(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	dirs, remap := s.containerAppdataRemap("/host/user/user/appdata",
		[]string{"/host/user/zfs/appdata/a", "/host/user/zfs/appdata/b"})
	want := map[string]string{
		"/host/user/zfs/appdata/a": "/host/user/user/appdata/a",
		"/host/user/zfs/appdata/b": "/host/user/user/appdata/b",
	}
	for _, d := range dirs {
		if want[d.Subtree] != d.Target {
			t.Fatalf("dir %+v not in want %v", d, want)
		}
	}
	if len(remap) != 2 {
		t.Fatalf("remap should have 2 entries, got %v", remap)
	}
}

// TestContainerAppdataRemapBasenameCollisionIsDeduped: two appdata paths that share
// a basename on DIFFERENT pools must map to DISTINCT target dirs — otherwise
// RestoreSubtreeTo would merge both into one folder (silent data loss). This is the
// safety-critical case.
func TestContainerAppdataRemapBasenameCollisionIsDeduped(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	dirs, remap := s.containerAppdataRemap("/host/user/user/appdata",
		[]string{"/host/user/zfs/appdata/config", "/host/user/cache/appdata/config"})

	if len(dirs) != 2 {
		t.Fatalf("want 2 dirs, got %+v", dirs)
	}
	if dirs[0].Target == dirs[1].Target {
		t.Fatalf("collision: both sources mapped to the same Target %q — would merge data", dirs[0].Target)
	}
	// Every Subtree preserved, every Target unique, every bindRemap value unique.
	seenTarget := map[string]bool{}
	for _, d := range dirs {
		if seenTarget[d.Target] {
			t.Fatalf("duplicate Target %q", d.Target)
		}
		seenTarget[d.Target] = true
	}
	seenDest := map[string]bool{}
	for _, v := range remap {
		if seenDest[v] {
			t.Fatalf("duplicate remap dest %q — binds would collide", v)
		}
		seenDest[v] = true
	}
	// The deduped one carries a numeric suffix.
	if _, ok := seenTarget["/host/user/user/appdata/config"]; !ok {
		t.Fatalf("expected first leaf undeduped, targets=%v", seenTarget)
	}
	if _, ok := seenTarget["/host/user/user/appdata/config-2"]; !ok {
		t.Fatalf("expected second leaf deduped to config-2, targets=%v", seenTarget)
	}
}

// TestContainerAppdataRemapSingleBindMatchesLegacyLeaf is the regression proof for
// the multi-bind fix: for a container with ONE appdata path — the common case —
// the destination must still be <destBase>/<basename>, byte-for-byte what the
// pre-fix leaf-only mapping produced. The expectation is COMPUTED with the old
// rule (base + "/" + path.Base(src)) rather than hand-written, so the two can
// never silently drift apart.
func TestContainerAppdataRemapSingleBindMatchesLegacyLeaf(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	const base = "/host/user/user/appdata"
	cases := []struct {
		name string
		src  string
	}{
		{"foreign pool", "/host/user/zfs/appdata/xo"},
		{"same pool (in-place no-op)", "/host/user/user/appdata/web"},
		{"other pool", "/host/user/cache/appdata/nexterm"},
		{"nested one level below appdata", "/host/user/cache/appdata/plex"},
		{"trailing slash is cleaned", "/host/user/zfs/appdata/sonarr/"},
		{"bind on the appdata root itself", "/host/user/zfs/appdata"},
		{"path with no appdata segment", "/host/user/zfs/other/thing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dirs, remap := s.containerAppdataRemap(base, []string{tc.src})
			clean := path.Clean(tc.src)
			legacy := base + "/" + path.Base(clean)
			if len(dirs) != 1 {
				t.Fatalf("want exactly one dir, got %+v", dirs)
			}
			if dirs[0].Subtree != clean {
				t.Fatalf("Subtree = %q, want the cleaned source %q", dirs[0].Subtree, clean)
			}
			if dirs[0].Target != legacy {
				t.Fatalf("Target = %q, want the unchanged legacy leaf mapping %q", dirs[0].Target, legacy)
			}
			if got := remap[s.toHostPath(clean)]; got != s.toHostPath(legacy) {
				t.Fatalf("bindRemap[%q] = %q, want %q (remap=%v)", s.toHostPath(clean), got, s.toHostPath(legacy), remap)
			}
		})
	}
}

// TestContainerAppdataRemapMultiBindKeepsSharedContainerFolder is the fix for the
// multi-bind bug: resolveAppdataPaths records EVERY appdata bind as its own entry
// without merging binds that share a folder, so a container with two binds under
// one folder arrives here as two paths sharing the "SnapOtter" ancestor. Mapping
// each by BASENAME alone dropped that ancestor and dumped "conf" and "data" as
// top-level folders in the destination appdata root, colliding with any other
// container's folders of those names. Both must nest under SnapOtter/ instead.
func TestContainerAppdataRemapMultiBindKeepsSharedContainerFolder(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	dirs, remap := s.containerAppdataRemap("/host/user/cache/appdata", []string{
		"/host/user/user/appdata/SnapOtter/conf",
		"/host/user/user/appdata/SnapOtter/data",
	})

	want := map[string]string{
		"/host/user/user/appdata/SnapOtter/conf": "/host/user/cache/appdata/SnapOtter/conf",
		"/host/user/user/appdata/SnapOtter/data": "/host/user/cache/appdata/SnapOtter/data",
	}
	if len(dirs) != len(want) {
		t.Fatalf("want %d dirs, got %+v", len(want), dirs)
	}
	for _, d := range dirs {
		if want[d.Subtree] != d.Target {
			t.Fatalf("dir %+v: want Target %q — the shared SnapOtter folder must be preserved, not flattened to its leaf", d, want[d.Subtree])
		}
	}
	// The recreated container's binds must follow to the nested locations.
	if got := remap["/mnt/user/appdata/SnapOtter/conf"]; got != "/mnt/cache/appdata/SnapOtter/conf" {
		t.Fatalf("bindRemap conf = %q, want /mnt/cache/appdata/SnapOtter/conf (remap=%v)", got, remap)
	}
	if got := remap["/mnt/user/appdata/SnapOtter/data"]; got != "/mnt/cache/appdata/SnapOtter/data" {
		t.Fatalf("bindRemap data = %q, want /mnt/cache/appdata/SnapOtter/data (remap=%v)", got, remap)
	}
}

// TestContainerAppdataRemapMultiBindInPlaceIsNoop: restoring a multi-bind
// container back to the appdata root it already lives in must map every path to
// ITSELF (Target == Subtree). prepareRestoreForTarget skips the overwrite guard
// exactly on that equality, so without the fix the leaf-only mapping turned an
// in-place restore into "<destBase>/conf already contains data" prompts against
// unrelated folders.
func TestContainerAppdataRemapMultiBindInPlaceIsNoop(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	src := []string{
		"/host/user/user/appdata/SnapOtter/conf",
		"/host/user/user/appdata/SnapOtter/data",
	}
	dirs, _ := s.containerAppdataRemap("/host/user/user/appdata", src)
	if len(dirs) != 2 {
		t.Fatalf("want 2 dirs, got %+v", dirs)
	}
	for _, d := range dirs {
		if d.Target != d.Subtree {
			t.Fatalf("in-place remap must be a no-op, got %+v", d)
		}
	}
}

// TestContainerAppdataRemapDedupeIsPerContainerFolder pins the collision suffix at
// the destination ROOT, not the individual leaf: two containers' folders that
// share a name on DIFFERENT pools still get distinct roots ("SnapOtter" and
// "SnapOtter-2"), but every bind of one source folder stays TOGETHER under the
// same root — a multi-bind container must never be split across SnapOtter/ and
// SnapOtter-2/.
func TestContainerAppdataRemapDedupeIsPerContainerFolder(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	dirs, _ := s.containerAppdataRemap("/host/user/user/appdata", []string{
		"/host/user/zfs/appdata/SnapOtter/conf",
		"/host/user/zfs/appdata/SnapOtter/data",
		"/host/user/cache/appdata/SnapOtter/conf",
		"/host/user/cache/appdata/SnapOtter/data",
	})
	want := map[string]string{
		"/host/user/zfs/appdata/SnapOtter/conf":   "/host/user/user/appdata/SnapOtter/conf",
		"/host/user/zfs/appdata/SnapOtter/data":   "/host/user/user/appdata/SnapOtter/data",
		"/host/user/cache/appdata/SnapOtter/conf": "/host/user/user/appdata/SnapOtter-2/conf",
		"/host/user/cache/appdata/SnapOtter/data": "/host/user/user/appdata/SnapOtter-2/data",
	}
	if len(dirs) != len(want) {
		t.Fatalf("want %d dirs, got %+v", len(want), dirs)
	}
	for _, d := range dirs {
		if want[d.Subtree] != d.Target {
			t.Fatalf("dir %+v: want Target %q", d, want[d.Subtree])
		}
	}
}

// TestContainerAppdataRemapTargetsAlwaysUnique pins the safety invariant behind
// the whole function: RestoreSubtreeTo dumps a subtree's CONTENTS into Target, so
// two sources sharing a Target would silently MERGE two containers' data. No input
// shape may ever produce a duplicate Target or a duplicate bindRemap destination.
func TestContainerAppdataRemapTargetsAlwaysUnique(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	cases := [][]string{
		{"/host/user/zfs/appdata/a", "/host/user/zfs/appdata/b"},
		{"/host/user/zfs/appdata/config", "/host/user/cache/appdata/config"},
		{"/host/user/zfs/appdata/X/conf", "/host/user/cache/appdata/X/conf"},
		{"/host/user/zfs/appdata/X/conf", "/host/user/zfs/appdata/X/data", "/host/user/zfs/appdata/Y"},
		{"/host/user/zfs/appdata/X", "/host/user/zfs/appdata/X/conf"},
		{"/host/user/zfs/appdata", "/host/user/cache/appdata"},
		{"/host/user/zfs/other/a", "/host/user/cache/other/a"},
	}
	for i, in := range cases {
		dirs, remap := s.containerAppdataRemap("/host/user/user/appdata", in)
		if len(dirs) != len(in) {
			t.Fatalf("case %d: want %d dirs, got %+v", i, len(in), dirs)
		}
		seenTarget := map[string]bool{}
		for _, d := range dirs {
			if seenTarget[d.Target] {
				t.Fatalf("case %d (%v): duplicate Target %q — restoring both would merge their data", i, in, d.Target)
			}
			seenTarget[d.Target] = true
		}
		seenDest := map[string]bool{}
		for _, v := range remap {
			if seenDest[v] {
				t.Fatalf("case %d (%v): duplicate bindRemap dest %q — two binds would collide", i, in, v)
			}
			seenDest[v] = true
		}
	}
}

// TestAppdataRelPathSplit pins the anchor rule the destination path is built on:
// the split is at the FIRST "appdata" segment, so a container that owns a NESTED
// folder called "appdata" stays anchored at the real share root; a path that has
// no "appdata" segment, or that IS an appdata root, yields an empty rel and lets
// the caller fall back to the plain basename.
func TestAppdataRelPathSplit(t *testing.T) {
	cases := []struct {
		in, root, rel string
	}{
		{"/host/user/user/appdata/SnapOtter/conf", "/host/user/user/appdata", "SnapOtter/conf"},
		{"/host/user/zfs/appdata/nexterm", "/host/user/zfs/appdata", "nexterm"},
		{"/host/user/user/appdata/foo/appdata", "/host/user/user/appdata", "foo/appdata"},
		{"/host/user/user/appdata", "", ""},
		{"/host/user/zfs/other/thing", "", ""},
		{"/host/user/user/appdataX/thing", "", ""},
	}
	for _, tc := range cases {
		root, rel := appdataRelPath(tc.in)
		if root != tc.root || rel != tc.rel {
			t.Fatalf("appdataRelPath(%q) = (%q, %q), want (%q, %q)", tc.in, root, rel, tc.root, tc.rel)
		}
	}
}

// compile-time: containerAppdataRemap returns backup.RestoreDir (aliased to VMRestoreDir).
var _ = func() []backup.RestoreDir { return nil }

// TestForeignBindWarningsClassification pins the #125 (Q1) bind classifier: only a
// NON-appdata pool bind whose pool is absent on this host is warned; appdata binds
// (remapped), host devices/sockets, and binds on mounted pools are not.
func TestForeignBindWarningsClassification(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	// /host/user/user is the shfs user share (holds appdata + downloads) and IS
	// mounted; the zfs pool is NOT.
	writeMountFixture(t, "/", "/host/user", "/host/user/user")

	binds := []string{
		"/mnt/user/appdata/web:/config",             // appdata -> remapped, skip
		"/mnt/zfs/media:/media:ro",                  // non-appdata pool, absent -> WARN
		"/mnt/user/downloads:/downloads",            // non-appdata pool, mounted -> ok
		"/var/run/docker.sock:/var/run/docker.sock", // host socket -> skip
		"/etc/localtime:/etc/localtime:ro",          // host file -> skip
	}
	// appdata is recorded as the container-visible path (/host/user/user/appdata/web).
	warnings := s.foreignBindWarnings(binds, []string{"/host/user/user/appdata/web"})

	if len(warnings) != 1 || warnings[0].Host != "/mnt/zfs/media" {
		t.Fatalf("want exactly the absent zfs media bind warned, got %+v", warnings)
	}
}
