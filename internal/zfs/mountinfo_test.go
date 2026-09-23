package zfs

import (
	"strings"
	"testing"
)

// mountinfoFixture keeps the shape of /proc/self/mountinfo as a container on
// Unraid 7 sees it: the Host Data bind of /mnt at /host/user with slave
// propagation, the two fuse.shfs user shares, and the zfs records of a pool
// below it. Field 4 is the mount root inside the source filesystem, field 5
// the mount point, field 6 the options, then the optional fields, " - ", the
// filesystem type and the source.
const mountinfoFixture = `4903 4796 0:2 /mnt /host/user rw master:1 - rootfs rootfs rw,size=16234356k,nr_inodes=4058589,inode64
5038 4903 259:3 / /host/user/cache rw,noatime master:10 - xfs /dev/nvme0n1p1 rw,nouuid,inode64
5261 4903 0:43 / /host/user/user0 rw,nosuid,nodev,noatime master:12 - fuse.shfs shfs rw,user_id=0,group_id=0,default_permissions,allow_other
5262 4903 0:44 / /host/user/user rw,nosuid,nodev,noatime master:13 - fuse.shfs shfs rw,user_id=0,group_id=0,default_permissions,allow_other
5300 4903 0:90 / /host/user/zpool rw,noatime master:14 - zfs zpool rw,xattr,noacl,casesensitive
5301 5300 0:91 / /host/user/zpool/appdata rw,noatime master:15 - zfs zpool/appdata rw,xattr,posixacl
5302 5262 0:91 / /host/user/user/appdata rw,noatime master:16 - zfs zpool/appdata rw,xattr,posixacl
5303 5300 0:92 / /host/user/zpool/Media\040Files rw,noatime master:17 - zfs zpool/Media\040Files rw,xattr,posixacl
5304 5300 0:93 /Library /host/user/zpool/bindsub rw,noatime master:18 - zfs zpool/appdata rw,xattr,posixacl
5305 5300 0:94 / /host/user/zpool/ro ro,noatime master:19 - zfs zpool/ro ro,xattr,posixacl
5306 5300 0:95 / /host/user/zpool/nested rw,noatime - zfs zpool/nested rw,xattr,posixacl
5307 5301 0:96 / /host/user/zpool/appdata/.zfs/snapshot/bombvault-20260917031500 ro,noatime master:20 - zfs zpool/appdata@bombvault-20260917031500 ro,xattr,posixacl
5400 4796 0:97 / /config/data rw,relatime - zfs zpool/config rw,xattr,posixacl
`

func fixtureRecords(t *testing.T) []MountRecord {
	t.Helper()
	recs := ParseMountinfo(strings.NewReader(mountinfoFixture))
	if len(recs) != 13 {
		t.Fatalf("parsed %d records, want 13", len(recs))
	}
	return recs
}

func TestParseMountinfoDecodesOctalAndKeepsOptions(t *testing.T) {
	recs := fixtureRecords(t)

	top := recs[0]
	if top.MountPoint != "/host/user" || top.Root != "/mnt" || top.FSType != "rootfs" {
		t.Errorf("host data record parsed wrong: %+v", top)
	}
	if len(top.Optional) != 1 || top.Optional[0] != "master:1" {
		t.Errorf("optional fields lost: %+v", top.Optional)
	}
	if len(top.Options) != 1 || top.Options[0] != "rw" {
		t.Errorf("options lost: %+v", top.Options)
	}

	var withSpace MountRecord
	for _, r := range recs {
		if r.Source == "zpool/Media Files" {
			withSpace = r
		}
	}
	if withSpace.MountPoint != "/host/user/zpool/Media Files" {
		t.Errorf("octal escape not decoded: %+v", withSpace)
	}

	nested := recs[10]
	if len(nested.Optional) != 0 {
		t.Errorf("a record without optional fields got %+v", nested.Optional)
	}
	if nested.FSType != "zfs" || nested.Source != "zpool/nested" {
		t.Errorf("fields after the separator parsed wrong: %+v", nested)
	}
}

func TestFindDatasetMountRequiresWholeDatasetRoot(t *testing.T) {
	recs := fixtureRecords(t)

	// zpool/appdata is also bound at /host/user/zpool/bindsub with root
	// /Library, which has no .zfs directory at its top.
	rec, ok := FindDatasetMount(recs, "zpool/appdata", "", "/host/user", false)
	if !ok {
		t.Fatal("no mount found for zpool/appdata")
	}
	if rec.MountPoint == "/host/user/zpool/bindsub" {
		t.Error("a bind of a subdirectory was taken for the dataset root")
	}

	if _, ok := FindDatasetMount(recs, "zpool/nothere", "", "/host/user", true); ok {
		t.Error("a dataset with no record was reported as mounted")
	}
}

func TestFindDatasetMountPrefersTranslatedPath(t *testing.T) {
	recs := fixtureRecords(t)

	// An exclusive share gives the same dataset a second record under
	// /host/user/user, which is the shorter path of the two.
	rec, ok := FindDatasetMount(recs, "zpool/appdata", "/host/user/zpool/appdata", "/host/user", false)
	if !ok || rec.MountPoint != "/host/user/zpool/appdata" {
		t.Fatalf("FindDatasetMount = %q, %v, want the pool path", rec.MountPoint, ok)
	}

	// Without a preference the shortest path below the host mount root wins.
	rec, ok = FindDatasetMount(recs, "zpool/appdata", "", "/host/user", false)
	if !ok || rec.MountPoint != "/host/user/user/appdata" {
		t.Fatalf("FindDatasetMount = %q, %v, want the shortest path", rec.MountPoint, ok)
	}
}

func TestFindDatasetMountIdentityMappingOnlyForBackups(t *testing.T) {
	recs := fixtureRecords(t)

	rec, ok := FindDatasetMount(recs, "zpool/config", "", "/host/user", true)
	if !ok || rec.MountPoint != "/config/data" {
		t.Fatalf("a backup did not find the record outside the host mount root: %q, %v", rec.MountPoint, ok)
	}

	// A restore writes, so it must never land in an app's own volume.
	if rec, ok := FindDatasetMount(recs, "zpool/config", "", "/host/user", false); ok {
		t.Errorf("a restore accepted %q, which is outside the host mount root", rec.MountPoint)
	}
}

func TestSnapshotMountedMatchesSourceAndPoint(t *testing.T) {
	recs := fixtureRecords(t)
	snap := "bombvault-20260917031500"
	cpath := "/host/user/zpool/appdata"

	if !SnapshotMounted(recs, "zpool/appdata", snap, cpath) {
		t.Error("the automounted snapshot was not recognised")
	}
	// The control directory alone carries no data, so a record for another
	// dataset or another stamp must not count.
	if SnapshotMounted(recs, "zpool/nested", snap, cpath) {
		t.Error("a snapshot of another dataset was accepted")
	}
	if SnapshotMounted(recs, "zpool/appdata", "bombvault-20260101000000", cpath) {
		t.Error("a snapshot of another run was accepted")
	}
	if SnapshotMounted(recs, "zpool/appdata", snap, "/host/user/user/appdata") {
		t.Error("a snapshot mounted under another container path was accepted")
	}
}

func TestShfsOnlyForTheGivenPath(t *testing.T) {
	recs := fixtureRecords(t)

	if !ShfsOnly(recs, "/host/user/user/media") {
		t.Error("a share reachable only through shfs was not recognised")
	}
	if ShfsOnly(recs, "/host/user/zpool/appdata") {
		t.Error("a native zfs mount was reported as shfs only")
	}
	if ShfsOnly(recs, "/host/user/cache/appdata") {
		t.Error("an xfs pool was reported as shfs only")
	}
}

func TestWritableReadsRoOption(t *testing.T) {
	recs := fixtureRecords(t)

	rw, ok := FindDatasetMount(recs, "zpool/appdata", "/host/user/zpool/appdata", "/host/user", false)
	if !ok || !Writable(rw) {
		t.Error("a rw mapping was reported as read only")
	}
	ro, ok := FindDatasetMount(recs, "zpool/ro", "", "/host/user", false)
	if !ok || Writable(ro) {
		t.Error("a ro mapping was reported as writable")
	}
}

func TestPropagationReportsNestedRecords(t *testing.T) {
	recs := fixtureRecords(t)

	top, without := Propagation(recs, "/host/user")
	if !top {
		t.Error("the host data record carries master:1 but was reported without propagation")
	}
	if len(without) != 1 || without[0] != "/host/user/zpool/nested" {
		t.Fatalf("nested records without propagation = %q, want zpool/nested", without)
	}

	plain := ParseMountinfo(strings.NewReader("4903 4796 0:2 /mnt /host/user rw - rootfs rootfs rw\n"))
	if top, without := Propagation(plain, "/host/user"); top || len(without) != 0 {
		t.Errorf("a mapping without propagation = %v, %q", top, without)
	}
}
