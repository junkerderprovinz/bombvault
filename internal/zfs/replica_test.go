package zfs

import (
	"errors"
	"strings"
	"testing"
)

const (
	replicaSnap = "bombvault-replica-20261009100000"
	replicaBase = "bombvault-replica-20261008100000"
	// resumeToken is the token OpenZFS 2.4.3 left on a full receive cut
	// after 8 MiB.
	resumeToken = "1-f45f3f812-118-789c636064000310a500c4ec50360710e72765a52697303030419460caa7a515a79680646ae0f26c48f2499525a9c540fa44441d56fd25f9e9a599290c0cace973bcd2985c3f0520c97382e5f312735381e69415151725eb6796a4e6ea276764e6a43824e5e726952596e694e816a516e4642627ea1a191899191a185802311040dc2b01b507e6bfd4dca4d494fc6cb03bb891c493f3730b8a528b8b8152700000c9d12a6d"
)

// foreignSnaps are names a replica builder must never act on: a backup run's,
// a safety snapshot, someone else's, and near misses of the replica pattern.
var foreignSnaps = []string{
	"",
	"bombvault-20261009100000",
	"bombvault-prerestore-20261009100000",
	"bombvault-replica-",
	"bombvault-replica-2026100910000",
	"bombvault-replica-202610091000000",
	"bombvault-replica-20261009100000x",
	"xbombvault-replica-20261009100000",
	"autosnap_2026-10-09_10:00:00_hourly",
}

func TestReplicaSnapshotArgsTakesTheWholeTree(t *testing.T) {
	got, err := ReplicaSnapshotArgs("cache/appdata", replicaSnap)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "snapshot", "-r", "cache/appdata@" + replicaSnap}; !equalArgs(got, want) {
		t.Fatalf("ReplicaSnapshotArgs = %q, want %q", got, want)
	}
	for _, snap := range foreignSnaps {
		if args, err := ReplicaSnapshotArgs("cache/appdata", snap); err == nil || args != nil {
			t.Errorf("ReplicaSnapshotArgs took %q: %q, %v", snap, args, err)
		}
	}
	if args, err := ReplicaSnapshotArgs(strings.Repeat("a", MaxDatasetNameLen+1), replicaSnap); err == nil || args != nil {
		t.Errorf("ReplicaSnapshotArgs took a root longer than an item root may be: %q, %v", args, err)
	}
}

func TestSendArgs(t *testing.T) {
	cases := []struct {
		name string
		spec SendSpec
		want []string
	}{
		{
			"full",
			SendSpec{Member: "cache/appdata", Snap: replicaSnap},
			[]string{"zfs", "send", "-c", "-L", "-e", "-p", "cache/appdata@" + replicaSnap},
		},
		{
			"full raw",
			SendSpec{Member: "cache/appdata/vault", Snap: replicaSnap, Raw: true},
			[]string{"zfs", "send", "-w", "-p", "cache/appdata/vault@" + replicaSnap},
		},
		{
			"from a snapshot, without what lies between",
			SendSpec{Member: "cache/appdata", Snap: replicaSnap, Base: replicaBase},
			[]string{"zfs", "send", "-c", "-L", "-e", "-p", "-i", "@" + replicaBase, "cache/appdata@" + replicaSnap},
		},
		{
			"from a bookmark",
			SendSpec{Member: "cache/appdata", Snap: replicaSnap, Base: replicaBase, FromBookmark: true},
			[]string{"zfs", "send", "-c", "-L", "-e", "-p", "-i", "#" + replicaBase, "cache/appdata@" + replicaSnap},
		},
		{
			"raw from a bookmark",
			SendSpec{Member: "cache/appdata/vault", Snap: replicaSnap, Base: replicaBase, FromBookmark: true, Raw: true},
			[]string{"zfs", "send", "-w", "-p", "-i", "#" + replicaBase, "cache/appdata/vault@" + replicaSnap},
		},
		{
			"a name with a space",
			SendSpec{Member: "cache/Media Files", Snap: replicaSnap},
			[]string{"zfs", "send", "-c", "-L", "-e", "-p", "cache/Media Files@" + replicaSnap},
		},
	}
	for _, c := range cases {
		got, err := SendArgs(c.spec)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !equalArgs(got, c.want) {
			t.Errorf("%s: SendArgs = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSendArgsRefusesForeignNames(t *testing.T) {
	for _, snap := range foreignSnaps {
		if args, err := SendArgs(SendSpec{Member: "cache/appdata", Snap: snap}); err == nil || args != nil {
			t.Errorf("SendArgs sent %q: %q, %v", snap, args, err)
		}
		if snap == "" {
			continue
		}
		if args, err := SendArgs(SendSpec{Member: "cache/appdata", Snap: replicaSnap, Base: snap}); err == nil || args != nil {
			t.Errorf("SendArgs started from %q: %q, %v", snap, args, err)
		}
	}
	for _, member := range []string{"", "-cache", "cache/../etc", "cache/app@x", "cache/app#x"} {
		if args, err := SendArgs(SendSpec{Member: member, Snap: replicaSnap}); err == nil || args != nil {
			t.Errorf("SendArgs took member %q: %q, %v", member, args, err)
		}
	}
}

func TestReplicaBuildersRefuseAMemberTooLongForTheReplicaName(t *testing.T) {
	// The longest member a backup can snapshot is too long for the longer
	// replica prefix.
	member := "cache/" + strings.Repeat("m", MaxSnapshotNameLen-len("@"+SnapshotPrefix)-stampLen-len("cache/"))
	if err := ValidateMemberName(member); err != nil {
		t.Fatalf("the fixture is not a member a backup reads: %v", err)
	}
	if ReplicaNameFits(member) {
		t.Fatal("the fixture fits a replica snapshot")
	}
	var ne *NameError
	_, err := SendArgs(SendSpec{Member: member, Snap: replicaSnap})
	if !errors.As(err, &ne) || ne.Code != "name-too-long" {
		t.Errorf("SendArgs = %v, want name-too-long", err)
	}
	_, err = ReceiveArgs(ReceiveSpec{Target: member})
	if !errors.As(err, &ne) || ne.Code != "name-too-long" {
		t.Errorf("ReceiveArgs = %v, want name-too-long", err)
	}
	_, err = BookmarkArgs(member, replicaSnap)
	if !errors.As(err, &ne) || ne.Code != "name-too-long" {
		t.Errorf("BookmarkArgs = %v, want name-too-long", err)
	}
}

func TestEstimateArgsAreADryRunOfTheSameStream(t *testing.T) {
	spec := SendSpec{Member: "cache/appdata", Snap: replicaSnap, Base: replicaBase, FromBookmark: true}
	got, err := EstimateArgs(spec)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zfs", "send", "-n", "-v", "-P", "-c", "-L", "-e", "-p", "-i", "#" + replicaBase, "cache/appdata@" + replicaSnap}
	if !equalArgs(got, want) {
		t.Fatalf("EstimateArgs = %q, want %q", got, want)
	}
	if args, err := EstimateArgs(SendSpec{Member: "cache/appdata", Snap: "nightly"}); err == nil || args != nil {
		t.Errorf("EstimateArgs took a foreign name: %q, %v", args, err)
	}
}

func TestResumeArgsCarryOnlyTheToken(t *testing.T) {
	got, err := ResumeSendArgs(resumeToken)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "send", "-t", resumeToken}; !equalArgs(got, want) {
		t.Fatalf("ResumeSendArgs = %q, want %q", got, want)
	}
	got, err = EstimateResumeArgs(resumeToken)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "send", "-n", "-v", "-P", "-t", resumeToken}; !equalArgs(got, want) {
		t.Fatalf("EstimateResumeArgs = %q, want %q", got, want)
	}

	for _, tok := range []string{"", "-", "1-abc", "1-abc-12-zz", "-w 1-ab-12-cd", "1-ab-12-cd;reboot", "1-AB-12-CD"} {
		if args, err := ResumeSendArgs(tok); err == nil || args != nil {
			t.Errorf("ResumeSendArgs took %q: %q, %v", tok, args, err)
		}
	}
}

func TestReceiveArgs(t *testing.T) {
	const target = "tank/bombvault-replica/bottich/cache/appdata"
	cases := []struct {
		name string
		spec ReceiveSpec
		want []string
	}{
		{
			"first filesystem stream",
			ReceiveSpec{Target: target, Full: true},
			[]string{"zfs", "receive", "-s", "-u", "-o", "readonly=on", "-o", "canmount=noauto", "-x", "mountpoint", "-x", "sharenfs", "-x", "sharesmb", "-x", "reservation", "-x", "refreservation", target},
		},
		{
			"first stream over an empty parent BombVault created",
			ReceiveSpec{Target: target, Full: true, Replace: true},
			[]string{"zfs", "receive", "-s", "-u", "-F", "-o", "readonly=on", "-o", "canmount=noauto", "-x", "mountpoint", "-x", "sharenfs", "-x", "sharesmb", "-x", "reservation", "-x", "refreservation", target},
		},
		{
			"first volume stream",
			ReceiveSpec{Target: target + "/vm", Full: true, Volume: true},
			[]string{"zfs", "receive", "-s", "-u", "-o", "readonly=on", "-x", "reservation", "-x", "refreservation", target + "/vm"},
		},
		{
			"incremental",
			ReceiveSpec{Target: target},
			[]string{"zfs", "receive", "-s", "-u", "-x", "mountpoint", "-x", "sharenfs", "-x", "sharesmb", "-x", "reservation", "-x", "refreservation", target},
		},
		{
			"incremental into a volume",
			ReceiveSpec{Target: target + "/vm", Volume: true},
			[]string{"zfs", "receive", "-s", "-u", "-x", "reservation", "-x", "refreservation", target + "/vm"},
		},
		{
			"incremental over a changed replica",
			ReceiveSpec{Target: target, Rollback: true},
			[]string{"zfs", "receive", "-s", "-u", "-F", "-x", "mountpoint", "-x", "sharenfs", "-x", "sharesmb", "-x", "reservation", "-x", "refreservation", target},
		},
	}
	for _, c := range cases {
		got, err := ReceiveArgs(c.spec)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !equalArgs(got, c.want) {
			t.Errorf("%s: ReceiveArgs = %q, want %q", c.name, got, c.want)
		}
	}

	// -F on a full stream would overwrite whatever dataset sits at the path.
	if args, err := ReceiveArgs(ReceiveSpec{Target: target, Full: true, Rollback: true}); err == nil || args != nil {
		t.Errorf("ReceiveArgs rolled back on a full stream: %q, %v", args, err)
	}
	if args, err := ReceiveArgs(ReceiveSpec{Target: target, Replace: true}); err == nil || args != nil {
		t.Errorf("ReceiveArgs replaced a dataset with an incremental: %q, %v", args, err)
	}
	for _, bad := range []string{"", "tank/../x", "-tank", "tank/x@snap"} {
		if args, err := ReceiveArgs(ReceiveSpec{Target: bad}); err == nil || args != nil {
			t.Errorf("ReceiveArgs took target %q: %q, %v", bad, args, err)
		}
	}
}

func TestRestoreReceiveArgsLandAWritableDataset(t *testing.T) {
	const target = "cache/appdata-bombvault-restore-1760000000000000000"
	got, err := RestoreReceiveArgs(target, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zfs", "receive", "-u", "-x", "readonly", "-x", "canmount", "-x", "mountpoint", "-x", "sharenfs", "-x", "sharesmb", target}
	if !equalArgs(got, want) {
		t.Fatalf("RestoreReceiveArgs = %q, want %q", got, want)
	}
	got, err = RestoreReceiveArgs(target, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "receive", "-u", "-x", "readonly", target}; !equalArgs(got, want) {
		t.Fatalf("RestoreReceiveArgs of a volume = %q, want %q", got, want)
	}
	if args, err := RestoreReceiveArgs("cache/x@y", false); err == nil || args != nil {
		t.Errorf("RestoreReceiveArgs took a snapshot: %q, %v", args, err)
	}
}

func TestCreateParentArgsNeverMountAndCarryTheOwner(t *testing.T) {
	got, err := CreateParentArgs("tank/bombvault-replica/bottich", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "create", "-p", "-u", "-o", "canmount=off", "tank/bombvault-replica/bottich"}; !equalArgs(got, want) {
		t.Fatalf("CreateParentArgs = %q, want %q", got, want)
	}
	got, err = CreateParentArgs("tank/bombvault-replica/bottich", "4f2a")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "create", "-p", "-u", "-o", "canmount=off", "-o", "bombvault:source=4f2a", "tank/bombvault-replica/bottich"}; !equalArgs(got, want) {
		t.Fatalf("CreateParentArgs with an owner = %q, want %q", got, want)
	}
	if args, err := CreateParentArgs("tank/../x", ""); err == nil || args != nil {
		t.Errorf("CreateParentArgs took a traversal: %q, %v", args, err)
	}
	for _, owner := range []string{"a b", "x=y", "-o", "a.b"} {
		if args, err := CreateParentArgs("tank/x", owner); err == nil || args != nil {
			t.Errorf("CreateParentArgs took the owner %q: %q, %v", owner, args, err)
		}
	}
}

func TestAbortReceiveArgs(t *testing.T) {
	got, err := AbortReceiveArgs("tank/r/cache/appdata")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "receive", "-A", "tank/r/cache/appdata"}; !equalArgs(got, want) {
		t.Fatalf("AbortReceiveArgs = %q, want %q", got, want)
	}
	if args, err := AbortReceiveArgs("tank/r@x"); err == nil || args != nil {
		t.Errorf("AbortReceiveArgs took a snapshot: %q, %v", args, err)
	}
}

func TestHoldReleaseAndBookmarkArgs(t *testing.T) {
	type builder func(string, string) ([]string, error)
	cases := []struct {
		name string
		fn   builder
		want []string
	}{
		{"hold", HoldArgs, []string{"zfs", "hold", "bombvault-replica", "cache/appdata@" + replicaSnap}},
		{"release", ReleaseArgs, []string{"zfs", "release", "bombvault-replica", "cache/appdata@" + replicaSnap}},
		{"bookmark", BookmarkArgs, []string{"zfs", "bookmark", "cache/appdata@" + replicaSnap, "cache/appdata#" + replicaSnap}},
	}
	for _, c := range cases {
		got, err := c.fn("cache/appdata", replicaSnap)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !equalArgs(got, c.want) {
			t.Errorf("%s = %q, want %q", c.name, got, c.want)
		}
		for _, snap := range foreignSnaps {
			if args, err := c.fn("cache/appdata", snap); err == nil || args != nil {
				t.Errorf("%s took %q: %q, %v", c.name, snap, args, err)
			}
		}
		if args, err := c.fn("cache/../etc", replicaSnap); err == nil || args != nil {
			t.Errorf("%s took a traversal: %q, %v", c.name, args, err)
		}
	}
}

func TestDestroyReplicaArgsOnlyReplicaNames(t *testing.T) {
	got, err := DestroyReplicaArgs("tank/r/cache/appdata", replicaSnap)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "destroy", "tank/r/cache/appdata@" + replicaSnap}; !equalArgs(got, want) {
		t.Fatalf("DestroyReplicaArgs = %q, want %q", got, want)
	}
	got, err = DestroyReplicaRecursiveArgs("cache/appdata", replicaSnap)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "destroy", "-r", "cache/appdata@" + replicaSnap}; !equalArgs(got, want) {
		t.Fatalf("DestroyReplicaRecursiveArgs = %q, want %q", got, want)
	}

	for _, snap := range foreignSnaps {
		if args, err := DestroyReplicaArgs("tank/r/cache/appdata", snap); err == nil || args != nil {
			t.Errorf("DestroyReplicaArgs took %q: %q, %v", snap, args, err)
		}
		if args, err := DestroyReplicaRecursiveArgs("cache/appdata", snap); err == nil || args != nil {
			t.Errorf("DestroyReplicaRecursiveArgs took %q: %q, %v", snap, args, err)
		}
	}
}

func TestReplicaBuildersEmitDashROnlyOnTheSourceTreeSnapshot(t *testing.T) {
	must := func(args []string, err error) []string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return args
	}
	take := must(ReplicaSnapshotArgs("cache/appdata", replicaSnap))
	destroyTree := must(DestroyReplicaRecursiveArgs("cache/appdata", replicaSnap))
	all := [][]string{
		take, destroyTree,
		must(SendArgs(SendSpec{Member: "cache/appdata", Snap: replicaSnap, Base: replicaBase})),
		must(EstimateArgs(SendSpec{Member: "cache/appdata", Snap: replicaSnap})),
		must(ResumeSendArgs(resumeToken)),
		must(ReceiveArgs(ReceiveSpec{Target: "tank/r/cache/appdata", Rollback: true})),
		must(AbortReceiveArgs("tank/r/cache/appdata")),
		must(HoldArgs("cache/appdata", replicaSnap)),
		must(ReleaseArgs("cache/appdata", replicaSnap)),
		must(BookmarkArgs("cache/appdata", replicaSnap)),
		must(DestroyReplicaArgs("tank/r/cache/appdata", replicaSnap)),
		must(ReplicaPointsArgs("cache/appdata")),
		must(DatasetStateArgs("tank/r/cache/appdata")),
		must(RestoreReceiveArgs("cache/appdata-bombvault-restore-1", false)),
		must(CreateParentArgs("tank/r/bottich", "4f2a")),
		must(DestroyReplicaBookmarkArgs("cache/appdata", replicaSnap)),
		must(SourcePropertyArgs("tank/r/bottich")),
	}
	for _, args := range all {
		for _, a := range args {
			if a == "-R" {
				t.Errorf("%q carries -R, which would send the excluded children", args)
			}
			if a == "-r" && !equalArgs(args, take) && !equalArgs(args, destroyTree) {
				t.Errorf("%q carries -r", args)
			}
		}
		if args[1] == "destroy" && !strings.ContainsAny(args[len(args)-1], "@#") {
			t.Errorf("destroy argv without a snapshot or bookmark: %q", args)
		}
	}
}

func TestReplicaListingArgs(t *testing.T) {
	got, err := ReplicaPointsArgs("tank/r/cache/Media Files")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zfs", "list", "-H", "-p", "-t", "snapshot,bookmark", "-o", "name,guid,createtxg", "-s", "createtxg", "-d", "1", "tank/r/cache/Media Files"}
	if !equalArgs(got, want) {
		t.Fatalf("ReplicaPointsArgs = %q, want %q", got, want)
	}
	got, err = DatasetStateArgs("tank/r/cache/appdata")
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"zfs", "get", "-H", "-p", "-o", "property,value", "type,encryption,receive_resume_token", "tank/r/cache/appdata"}
	if !equalArgs(got, want) {
		t.Fatalf("DatasetStateArgs = %q, want %q", got, want)
	}
	for _, bad := range []string{"", "a@b", "a/../b"} {
		if args, err := ReplicaPointsArgs(bad); err == nil || args != nil {
			t.Errorf("ReplicaPointsArgs took %q: %q, %v", bad, args, err)
		}
		if args, err := DatasetStateArgs(bad); err == nil || args != nil {
			t.Errorf("DatasetStateArgs took %q: %q, %v", bad, args, err)
		}
	}
}

func TestDestroyReplicaBookmarkArgsOnlyReplicaNames(t *testing.T) {
	got, err := DestroyReplicaBookmarkArgs("cache/appdata", replicaSnap)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "destroy", "cache/appdata#" + replicaSnap}; !equalArgs(got, want) {
		t.Fatalf("DestroyReplicaBookmarkArgs = %q, want %q", got, want)
	}
	for _, snap := range foreignSnaps {
		if args, err := DestroyReplicaBookmarkArgs("cache/appdata", snap); err == nil || args != nil {
			t.Errorf("DestroyReplicaBookmarkArgs took %q: %q, %v", snap, args, err)
		}
	}
}

func TestSourcePropertyReadsOnlyALocalValue(t *testing.T) {
	got, err := SourcePropertyArgs("tank/r/bottich")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zfs", "get", "-H", "-p", "-s", "local", "-o", "value", "bombvault:source", "tank/r/bottich"}
	if !equalArgs(got, want) {
		t.Fatalf("SourcePropertyArgs = %q, want %q", got, want)
	}
	for out, want := range map[string]string{"": "", "-\n": "", "4f2a\n": "4f2a"} {
		if got := ParseSourceProperty(out); got != want {
			t.Errorf("ParseSourceProperty(%q) = %q, want %q", out, got, want)
		}
	}
	if args, err := SourcePropertyArgs("tank/../r"); err == nil || args != nil {
		t.Errorf("SourcePropertyArgs took a traversal: %q, %v", args, err)
	}
}

func TestParsePoolsAddsUsedToFree(t *testing.T) {
	pools, err := ParsePools("tank\t1000\t3000\nbv95demo\t0\t-\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []Pool{{"tank", 4000, 3000}, {"bv95demo", 0, 0}}
	if len(pools) != len(want) || pools[0] != want[0] || pools[1] != want[1] {
		t.Fatalf("ParsePools = %+v, want %+v", pools, want)
	}
	for _, bad := range []string{"tank\t1000\n", "tank/child\t1\t2\n", "tank\tx\t2\n"} {
		if _, err := ParsePools(bad); err == nil {
			t.Errorf("ParsePools took %q", bad)
		}
	}
	if want := []string{"zfs", "list", "-H", "-p", "-d", "0", "-o", "name,used,available"}; !equalArgs(PoolsArgs(), want) {
		t.Errorf("PoolsArgs = %q, want %q", PoolsArgs(), want)
	}
}

func TestMountArgs(t *testing.T) {
	got, err := MountArgs("cache/appdata-bombvault-restore-1")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "mount", "cache/appdata-bombvault-restore-1"}; !equalArgs(got, want) {
		t.Fatalf("MountArgs = %q, want %q", got, want)
	}
	if args, err := MountArgs("-a"); err == nil || args != nil {
		t.Errorf("MountArgs took a flag: %q, %v", args, err)
	}
}
