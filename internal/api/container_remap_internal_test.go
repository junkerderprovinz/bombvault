package api

import (
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
