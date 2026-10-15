package zfs

import (
	"reflect"
	"strings"
	"testing"
)

// unraidMounts is the Host Data mapping of an Unraid box with an XFS array
// disk, a ZFS pool "cache" with an appdata dataset, and the user share.
var unraidMounts = []MountRecord{
	{MountPoint: "/host/user", Root: "/", FSType: "rootfs", Source: "rootfs"},
	{MountPoint: "/host/user/disk1", Root: "/", FSType: "xfs", Source: "/dev/md1p1"},
	{MountPoint: "/host/user/cache", Root: "/", FSType: "zfs", Source: "cache"},
	{MountPoint: "/host/user/cache/appdata", Root: "/", FSType: "zfs", Source: "cache/appdata"},
	{MountPoint: "/host/user/cache/appdata/plex", Root: "/", FSType: "zfs", Source: "cache/appdata/plex"},
	{MountPoint: "/host/user/cache/appdata/plex/db", Root: "/", FSType: "zfs", Source: "cache/appdata/plex/db"},
	{MountPoint: "/host/user/user", Root: "/", FSType: "fuse.shfs", Source: "shfs"},
	{MountPoint: "/host/user/user0", Root: "/", FSType: "fuse.shfs", Source: "shfs"},
}

// fsWith is a filesystem where exactly paths exist and nothing is a symlink.
func fsWith(paths ...string) Lookup {
	set := map[string]bool{}
	for _, p := range paths {
		set[p] = true
	}
	return Lookup{Exists: func(p string) bool { return set[p] }, Real: func(p string) string { return p }}
}

func TestPlaceOnZFSFollowsAUserShareToItsPool(t *testing.T) {
	got, u := PlaceOnZFS(unraidMounts, []string{"/host/user/user/appdata/sonarr"}, "/host/user",
		fsWith("/host/user/cache/appdata/sonarr"))
	if u != nil {
		t.Fatalf("unplaced: %v", u)
	}
	want := []Binding{{Dataset: "cache/appdata", Rel: "sonarr", Target: "/host/user/user/appdata/sonarr"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestPlaceOnZFSBindsEveryDatasetBelowThePath(t *testing.T) {
	got, u := PlaceOnZFS(unraidMounts, []string{"/host/user/user/appdata/plex"}, "/host/user",
		fsWith("/host/user/cache/appdata/plex"))
	if u != nil {
		t.Fatalf("unplaced: %v", u)
	}
	want := []Binding{
		{Dataset: "cache/appdata/plex", Target: "/host/user/user/appdata/plex"},
		{Dataset: "cache/appdata/plex/db", Target: "/host/user/user/appdata/plex/db"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestPlaceOnZFSReadsADirectPoolPath(t *testing.T) {
	got, u := PlaceOnZFS(unraidMounts, []string{"/host/user/cache/appdata/sonarr/config"}, "/host/user", fsWith())
	if u != nil {
		t.Fatalf("unplaced: %v", u)
	}
	want := []Binding{{Dataset: "cache/appdata", Rel: "sonarr/config", Target: "/host/user/cache/appdata/sonarr/config"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestPlaceOnZFSRefusesAShareOnTheArray(t *testing.T) {
	_, u := PlaceOnZFS(unraidMounts, []string{"/host/user/user/media/x"}, "/host/user",
		fsWith("/host/user/disk1/media/x"))
	if u == nil || u.Code != CodeNotOnZFS || u.Path != "/host/user/user/media/x" {
		t.Fatalf("got %+v, want %s for the share path", u, CodeNotOnZFS)
	}
}

func TestPlaceOnZFSRefusesAShareSpreadOverDisks(t *testing.T) {
	_, u := PlaceOnZFS(unraidMounts, []string{"/host/user/user/appdata/sonarr"}, "/host/user",
		fsWith("/host/user/cache/appdata/sonarr", "/host/user/disk1/appdata/sonarr"))
	if u == nil || u.Code != CodeSplitShare {
		t.Fatalf("got %+v, want %s", u, CodeSplitShare)
	}
}

func TestPlaceOnZFSRefusesAShareItCannotFind(t *testing.T) {
	_, u := PlaceOnZFS(unraidMounts, []string{"/host/user/user/appdata/gone"}, "/host/user", fsWith())
	if u == nil || u.Code != CodeNoBranch {
		t.Fatalf("got %+v, want %s", u, CodeNoBranch)
	}
}

func TestPlaceOnZFSRefusesAnotherFilesystemBelowThePath(t *testing.T) {
	recs := append(append([]MountRecord(nil), unraidMounts...),
		MountRecord{MountPoint: "/host/user/cache/appdata/sonarr/tmp", Root: "/", FSType: "tmpfs", Source: "tmpfs"})
	_, u := PlaceOnZFS(recs, []string{"/host/user/user/appdata/sonarr"}, "/host/user",
		fsWith("/host/user/cache/appdata/sonarr"))
	if u == nil || u.Code != CodeNestedMount || u.Path != "/host/user/user/appdata/sonarr/tmp" {
		t.Fatalf("got %+v, want %s at the share path of the mount", u, CodeNestedMount)
	}
}

func TestPlaceOnZFSReadsABoundSubdirectoryInsideItsDataset(t *testing.T) {
	recs := []MountRecord{
		{MountPoint: "/data", Root: "/appdata/x", FSType: "zfs", Source: "tank/apps"},
		{MountPoint: "/data/db", Root: "/", FSType: "zfs", Source: "tank/apps/db"},
	}
	got, u := PlaceOnZFS(recs, []string{"/data/config"}, "/host/user", fsWith())
	want := []Binding{{Dataset: "tank/apps", Rel: "appdata/x/config", Target: "/data/config"}}
	if u != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, u, want)
	}
	got, _ = PlaceOnZFS(recs, []string{"/data"}, "/host/user", fsWith())
	want = []Binding{
		{Dataset: "tank/apps", Rel: "appdata/x", Target: "/data"},
		{Dataset: "tank/apps/db", Target: "/data/db"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestPlaceOnZFSFollowsAnExclusiveShareLinkIntoItsPool(t *testing.T) {
	fs := fsWith()
	fs.Real = func(p string) string {
		return strings.Replace(p, "/host/user/user/appdata", "/host/user/cache/appdata", 1)
	}
	got, u := PlaceOnZFS(unraidMounts, []string{"/host/user/user/appdata/sonarr"}, "/host/user", fs)
	want := []Binding{{Dataset: "cache/appdata", Rel: "sonarr", Target: "/host/user/user/appdata/sonarr"}}
	if u != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v without looking for a branch", got, u, want)
	}
}

func TestPlaceOnZFSIgnoresSnapshotsMountedBelowThePath(t *testing.T) {
	recs := append(append([]MountRecord(nil), unraidMounts...), MountRecord{
		MountPoint: "/host/user/cache/appdata/sonarr/.zfs/snapshot/bombvault-20261015000000",
		Root:       "/", FSType: "zfs", Source: "cache/appdata@bombvault-20261015000000",
	})
	got, u := PlaceOnZFS(recs, []string{"/host/user/cache/appdata/sonarr"}, "/host/user", fsWith())
	if u != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v; want the dataset alone", got, u)
	}
}

func TestPlaceOnZFSTakesTheMountThatHidesTheOthers(t *testing.T) {
	recs := []MountRecord{
		{MountPoint: "/mnt/tank", Root: "/", FSType: "xfs", Source: "/dev/sdb1"},
		{MountPoint: "/mnt/tank", Root: "/", FSType: "zfs", Source: "tank"},
	}
	got, u := PlaceOnZFS(recs, []string{"/mnt/tank/apps"}, "/host/user", fsWith())
	if u != nil || len(got) != 1 || got[0].Dataset != "tank" {
		t.Fatalf("got %+v, %v; want tank", got, u)
	}
}
