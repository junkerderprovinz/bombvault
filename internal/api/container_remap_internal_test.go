package api

import (
	"path"
	"testing"
)

// TestContainerAppdataRemapSinglePath: one appdata path goes to
// <destBase>/<basename>, and bindRemap maps the host paths (/mnt/...) rather
// than the container-visible ones.
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

// TestContainerAppdataRemapNoopWhenDestEqualsSource: appdata already under
// destBase maps to itself, so a restore to the default destination lands where
// the data was.
func TestContainerAppdataRemapNoopWhenDestEqualsSource(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	dirs, _ := s.containerAppdataRemap("/host/user/user/appdata", []string{"/host/user/user/appdata/web"})
	if len(dirs) != 1 || dirs[0].Target != "/host/user/user/appdata/web" || dirs[0].Subtree != dirs[0].Target {
		t.Fatalf("dirs = %+v, want Target==Subtree==/host/user/user/appdata/web", dirs)
	}
}

// TestContainerAppdataRemapDistinctBasenames: different basenames need no dedup
// suffix.
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

// TestContainerAppdataRemapBasenameCollisionIsDeduped: two appdata paths with
// the same basename on different pools get distinct targets; a shared one would
// make RestoreSubtreeTo merge both into one folder.
func TestContainerAppdataRemapBasenameCollisionIsDeduped(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	dirs, remap := s.containerAppdataRemap("/host/user/user/appdata",
		[]string{"/host/user/zfs/appdata/config", "/host/user/cache/appdata/config"})

	if len(dirs) != 2 {
		t.Fatalf("want 2 dirs, got %+v", dirs)
	}
	if dirs[0].Target == dirs[1].Target {
		t.Fatalf("collision: both sources mapped to the same Target %q, which would merge data", dirs[0].Target)
	}
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
			t.Fatalf("duplicate remap dest %q; the binds would collide", v)
		}
		seenDest[v] = true
	}
	// The second one gets a numeric suffix.
	if _, ok := seenTarget["/host/user/user/appdata/config"]; !ok {
		t.Fatalf("expected first leaf undeduped, targets=%v", seenTarget)
	}
	if _, ok := seenTarget["/host/user/user/appdata/config-2"]; !ok {
		t.Fatalf("expected second leaf deduped to config-2, targets=%v", seenTarget)
	}
}

// TestContainerAppdataRemapSingleBindUsesBasename: a container with a single
// appdata path, the common case, lands at <destBase>/<basename>. The
// expectation is computed from that rule rather than written out, so the two
// cannot drift apart.
func TestContainerAppdataRemapSingleBindUsesBasename(t *testing.T) {
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

// TestContainerAppdataRemapMultiBindKeepsSharedContainerFolder:
// resolveAppdataPaths records every appdata bind separately, so a container
// with two binds under one folder arrives as two paths below SnapOtter. Mapping
// each by basename would put conf and data at the top of the destination, where
// they collide with other containers' folders.
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
			t.Fatalf("dir %+v: want Target %q; the shared SnapOtter folder must keep its path, not be flattened to its leaf", d, want[d.Subtree])
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

// TestContainerAppdataRemapMultiBindInPlaceIsNoop: an in-place restore of a
// multi-bind container maps every path to itself. prepareRestoreForTarget skips
// its overwrite guard on exactly that equality; any other mapping prompts about
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

// TestContainerAppdataRemapDedupeIsPerContainerFolder: the collision suffix
// goes on the container folder, not the leaf. Same-named folders from different
// pools become SnapOtter and SnapOtter-2, and all binds of one folder stay
// together.
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

// TestContainerAppdataRemapTargetsAlwaysUnique: RestoreSubtreeTo writes a
// subtree's contents into Target, so a shared Target would merge two
// containers' data. No input may produce a duplicate Target or bindRemap
// destination.
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
				t.Fatalf("case %d (%v): duplicate Target %q; restoring both would merge their data", i, in, d.Target)
			}
			seenTarget[d.Target] = true
		}
		seenDest := map[string]bool{}
		for _, v := range remap {
			if seenDest[v] {
				t.Fatalf("case %d (%v): duplicate bindRemap dest %q; two binds would collide", i, in, v)
			}
			seenDest[v] = true
		}
	}
}

// TestAppdataRelPathSplit: the split is at the first "appdata" segment, so a
// nested folder called appdata stays under the real share root. A path without
// that segment, or the appdata root itself, yields an empty rel and the caller
// falls back to the basename.
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

// TestForeignBindWarningsClassification: only a non-appdata bind on a pool that
// is not mounted here gets a warning. Appdata binds are remapped, and host
// files, sockets and mounted pools are fine.
func TestForeignBindWarningsClassification(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	// The user share is mounted, the zfs pool is not.
	writeMountFixture(t, "/", "/host/user", "/host/user/user")

	binds := []string{
		"/mnt/user/appdata/web:/config",             // appdata, remapped
		"/mnt/zfs/media:/media:ro",                  // pool not mounted: warned
		"/mnt/user/downloads:/downloads",            // mounted pool
		"/var/run/docker.sock:/var/run/docker.sock", // host socket
		"/etc/localtime:/etc/localtime:ro",          // host file
	}
	// Appdata is recorded as the container-visible path.
	warnings := s.foreignBindWarnings(binds, []string{"/host/user/user/appdata/web"})

	if len(warnings) != 1 || warnings[0].Host != "/mnt/zfs/media" {
		t.Fatalf("want exactly the absent zfs media bind warned, got %+v", warnings)
	}
}
