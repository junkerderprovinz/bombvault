package zfs

import (
	"strings"
	"testing"
)

// treeFixture has the field order and value spellings of OpenZFS 2.3.4
// `zfs list -H -p -r -t filesystem,volume -o name,type,mountpoint,mounted,
// canmount,encryption,keystatus,snapdir,referenced,usedbydataset`: tab
// separated, "-" where a property does not apply to a volume, "yes"/"no" for
// mounted, raw byte counts for the sizes.
const treeFixture = "cache/appdata\tfilesystem\t/mnt/cache/appdata\tyes\ton\toff\t-\thidden\t131072\t98304\n" +
	"cache/appdata/Media Files\tfilesystem\t/mnt/cache/appdata/Media Files\tyes\ton\toff\t-\thidden\t4096\t4096\n" +
	"cache/appdata/db\tvolume\t-\t-\t-\toff\t-\t-\t2097152\t2097152\n" +
	"cache/appdata/enc\tfilesystem\t/mnt/cache/appdata/enc\tno\ton\taes-256-gcm\tunavailable\thidden\t8192\t8192\n" +
	"cache/appdata/legacy\tfilesystem\tlegacy\tno\ton\toff\t-\thidden\t4096\t4096\n" +
	"cache/appdata/noauto\tfilesystem\t/mnt/cache/appdata/noauto\tno\tnoauto\toff\t-\thidden\t4096\t4096\n" +
	"cache/appdata/none\tfilesystem\tnone\tno\ton\toff\t-\thidden\t4096\t4096\n" +
	"cache/appdata/off\tfilesystem\t/mnt/cache/appdata/off\tno\toff\toff\t-\thidden\t4096\t4096\n" +
	"cache/appdata/plex\tfilesystem\t/mnt/cache/appdata/plex\tyes\ton\toff\t-\tdisabled\t1048576\t1048576\n"

func TestParseTreeWithSpacesInNames(t *testing.T) {
	entries, err := ParseTree(treeFixture, "cache/appdata")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 9 {
		t.Fatalf("got %d entries, want 9", len(entries))
	}
	root := entries[0]
	if root.Name != "cache/appdata" || root.Type != "filesystem" || !root.Mounted {
		t.Errorf("root parsed wrong: %+v", root)
	}
	if root.Referenced != 131072 || root.UsedByDataset != 98304 {
		t.Errorf("root sizes wrong: %+v", root)
	}
	if root.Used != 0 {
		t.Errorf("Used = %d, want 0: the tree listing does not ask for it", root.Used)
	}

	share := entries[1]
	if share.Name != "cache/appdata/Media Files" || share.Mountpoint != "/mnt/cache/appdata/Media Files" {
		t.Errorf("a name with a space parsed wrong: %+v", share)
	}

	zvol := entries[2]
	if zvol.Type != "volume" || zvol.Mounted || zvol.Mountpoint != "-" {
		t.Errorf("volume parsed wrong: %+v", zvol)
	}

	enc := entries[3]
	if enc.Encryption != "aes-256-gcm" || enc.Keystatus != "unavailable" {
		t.Errorf("encrypted child parsed wrong: %+v", enc)
	}
}

func TestParseTreeRejectsForeignNamesAndWrongFieldCount(t *testing.T) {
	if _, err := ParseTree(treeFixture, "cache/other"); err == nil {
		t.Error("ParseTree accepted a listing whose first line is not the root")
	}

	foreign := "cache/appdata\tfilesystem\t/mnt/cache/appdata\tyes\ton\toff\t-\thidden\t4096\t4096\n" +
		"tank/other\tfilesystem\t/mnt/tank/other\tyes\ton\toff\t-\thidden\t4096\t4096\n"
	if _, err := ParseTree(foreign, "cache/appdata"); err == nil {
		t.Error("ParseTree accepted a dataset outside the tree")
	}

	short := "cache/appdata\tfilesystem\t/mnt/cache/appdata\tyes\ton\n"
	if _, err := ParseTree(short, "cache/appdata"); err == nil {
		t.Error("ParseTree accepted a line with too few fields")
	}

	if _, err := ParseTree("", "cache/appdata"); err == nil {
		t.Error("ParseTree accepted empty output")
	}
}

func TestParseList(t *testing.T) {
	out := "cache\tfilesystem\t/mnt/cache\tyes\ton\toff\t-\thidden\t10737418240\t98304\t98304\n" +
		"cache/appdata\tfilesystem\t/mnt/cache/appdata\tyes\ton\toff\t-\thidden\t5368709120\t131072\t98304\n" +
		"tank/vm/win11\tvolume\t-\t-\t-\toff\t-\t-\t53687091200\t42949672960\t42949672960\n"

	entries, err := ParseList(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	if entries[0].Used != 10737418240 || entries[0].Referenced != 98304 || entries[0].UsedByDataset != 98304 {
		t.Errorf("pool root sizes wrong: %+v", entries[0])
	}
	if entries[2].Name != "tank/vm/win11" || entries[2].Type != "volume" || entries[2].Used != 53687091200 {
		t.Errorf("volume parsed wrong: %+v", entries[2])
	}

	if _, err := ParseList(treeFixture); err == nil {
		t.Error("ParseList accepted the tree listing, which has one field fewer")
	}
}

func TestParseSnapshotsSplitsDataset(t *testing.T) {
	out := "cache/appdata@bombvault-20260917031500\t1789629300\t0\n" +
		"cache/appdata/Media Files@autosnap_2026-09-17_03:00:00_hourly\t1789628400\t16384\n"

	snaps, err := ParseSnapshots(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("got %d snapshots, want 2", len(snaps))
	}
	if snaps[0].Dataset != "cache/appdata" || snaps[0].Name != "bombvault-20260917031500" {
		t.Errorf("split wrong: %+v", snaps[0])
	}
	if snaps[0].Creation != 1789629300 {
		t.Errorf("creation = %d", snaps[0].Creation)
	}
	if snaps[1].Dataset != "cache/appdata/Media Files" || snaps[1].Used != 16384 {
		t.Errorf("second snapshot parsed wrong: %+v", snaps[1])
	}

	if _, err := ParseSnapshots("cache/appdata\t1789629300\t0\n"); err == nil {
		t.Error("ParseSnapshots accepted a name without an @")
	}
}

func TestDescendantOfPrefixTraps(t *testing.T) {
	cases := []struct {
		name, root string
		want       bool
	}{
		{"cache/appdata/plex", "cache/appdata", true},
		{"cache/appdata/plex/Library", "cache/appdata", true},
		{"cache/appdata", "cache/appdata", false},
		{"cache/app", "cache/ap", false},
		{"cache/appdata", "cache/appdata/plex", false},
		{"cache", "cache/appdata", false},
		{"", "cache", false},
		{"cache/appdata", "", false},
	}
	for _, c := range cases {
		if got := DescendantOf(c.name, c.root); got != c.want {
			t.Errorf("DescendantOf(%q, %q) = %v, want %v", c.name, c.root, got, c.want)
		}
	}
}

func TestMemberCodeTable(t *testing.T) {
	entries, err := ParseTree(treeFixture, "cache/appdata")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]ListEntry{}
	for _, e := range entries {
		byName[e.Name] = e
	}

	excluded := []string{"cache/appdata/Media Files"}
	cases := []struct {
		dataset string
		want    string
	}{
		{"cache/appdata", ""},
		{"cache/appdata/Media Files", "excluded"},
		{"cache/appdata/db", "zvol"},
		{"cache/appdata/enc", "key-not-loaded"},
		{"cache/appdata/legacy", "legacy-mount"},
		{"cache/appdata/noauto", "not-mounted"},
		{"cache/appdata/none", "no-mountpoint"},
		{"cache/appdata/off", "canmount-off"},
		{"cache/appdata/plex", "snapdir-disabled"},
	}
	for _, c := range cases {
		if got := MemberCode(byName[c.dataset], excluded); got != c.want {
			t.Errorf("MemberCode(%s) = %q, want %q", c.dataset, got, c.want)
		}
	}

	// Excluding a dataset excludes everything below it.
	child := ListEntry{Name: "cache/appdata/Media Files/Films", Type: "filesystem", Mountpoint: "/mnt/x", Mounted: true, Canmount: "on", Snapdir: "hidden"}
	if got := MemberCode(child, excluded); got != "excluded" {
		t.Errorf("MemberCode of a child below an exclusion = %q, want excluded", got)
	}

	// A receive leaves datasets named pool/%recv behind; BombVault cannot put
	// that name in an argv.
	odd := ListEntry{Name: "cache/appdata/%recv", Type: "filesystem", Mountpoint: "/mnt/x", Mounted: true, Canmount: "on", Snapdir: "hidden"}
	if got := MemberCode(odd, nil); got != "invalid-name" {
		t.Errorf("MemberCode of an unusable name = %q, want invalid-name", got)
	}

	long := ListEntry{Name: "cache/" + strings.Repeat("a", 240), Type: "filesystem", Mountpoint: "/mnt/x", Mounted: true, Canmount: "on", Snapdir: "hidden"}
	if got := MemberCode(long, nil); got != "name-too-long" {
		t.Errorf("MemberCode of a name the snapshot would not fit = %q, want name-too-long", got)
	}
}

func TestLeakedStampsIgnoresZvolOnlyStamps(t *testing.T) {
	tree, err := ParseTree(treeFixture, "cache/appdata")
	if err != nil {
		t.Fatal(err)
	}
	snaps := []SnapshotEntry{
		{Dataset: "cache/appdata", Name: "bombvault-20260917031500"},
		{Dataset: "cache/appdata/plex", Name: "bombvault-20260917031500"},
		{Dataset: "cache/appdata/db", Name: "bombvault-20260101000000"},
		{Dataset: "cache/appdata", Name: "bombvault-prerestore-20260917031500"},
		{Dataset: "cache/appdata", Name: "autosnap_2026-09-17_03:00:00_hourly"},
		{Dataset: "cache/appdata/plex", Name: "bombvault-20260916031500"},
	}

	got := LeakedStamps(tree, snaps)
	want := []string{"bombvault-20260917031500", "bombvault-20260916031500"}
	if len(got) != len(want) {
		t.Fatalf("LeakedStamps = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("LeakedStamps = %q, want %q", got, want)
		}
	}
}
