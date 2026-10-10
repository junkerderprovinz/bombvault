package zfs

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateDatasetName(t *testing.T) {
	good := []string{
		"tank",
		"cache/appdata",
		"cache/Media Files",
		"pool/a.b:c-d_e",
		strings.Repeat("a", MaxDatasetNameLen),
	}
	for _, name := range good {
		if err := ValidateDatasetName(name); err != nil {
			t.Errorf("ValidateDatasetName(%q) = %v, want nil", name, err)
		}
	}

	bad := []string{
		"", "/", "cache/", "/cache", "cache//x", "cache/../x", "cache/.",
		"-x", "a@b", "a#b", "a%b", "a,b", "a'b", `a"b`, `a\b`,
		"a\tb", "a\nb", "a\x00b", " cache", "cache/ x", "x ",
	}
	for _, name := range bad {
		err := ValidateDatasetName(name)
		var ne *NameError
		if !errors.As(err, &ne) {
			t.Errorf("ValidateDatasetName(%q) = %v, want a NameError", name, err)
			continue
		}
		if ne.Code != "invalid-name" {
			t.Errorf("ValidateDatasetName(%q) code = %q, want invalid-name", name, ne.Code)
		}
	}

	var ne *NameError
	err := ValidateDatasetName(strings.Repeat("a", MaxDatasetNameLen+1))
	if !errors.As(err, &ne) || ne.Code != "name-too-long" {
		t.Fatalf("a name one byte over the limit = %v, want name-too-long", err)
	}
}

func TestValidateMemberNameTakesEveryNameARunCanRead(t *testing.T) {
	longest := "cache/" + strings.Repeat("m", MaxSnapshotNameLen-len("@bombvault-")-14-len("cache/"))
	if !SnapshotNameFits(longest) {
		t.Fatalf("the fixture is %d bytes, longer than a run can snapshot", len(longest))
	}
	if len(longest) <= MaxDatasetNameLen {
		t.Fatalf("the fixture is %d bytes, too short to be longer than a root", len(longest))
	}
	if err := ValidateMemberName(longest); err != nil {
		t.Fatalf("ValidateMemberName of a %d byte member = %v, want nil", len(longest), err)
	}
	var ne *NameError
	if err := ValidateMemberName(longest + "m"); !errors.As(err, &ne) || ne.Code != "name-too-long" {
		t.Fatalf("a member one byte too long = %v, want name-too-long", err)
	}
	if err := ValidateMemberName("cache/../etc"); !errors.As(err, &ne) || ne.Code != "invalid-name" {
		t.Fatalf("a traversal = %v, want invalid-name", err)
	}
}

func TestSnapshotNameIsUTCFourteenDigits(t *testing.T) {
	cest := time.FixedZone("CEST", 2*60*60)
	at := time.Date(2026, 9, 17, 5, 15, 0, 0, cest)

	if got, want := SnapshotName(at), "bombvault-20260917031500"; got != want {
		t.Errorf("SnapshotName = %q, want %q", got, want)
	}
	if got, want := PreRestoreSnapshotName(at), "bombvault-prerestore-20260917031500"; got != want {
		t.Errorf("PreRestoreSnapshotName = %q, want %q", got, want)
	}
	if !IsBombVaultSnapshot(SnapshotName(at)) {
		t.Error("SnapshotName does not satisfy IsBombVaultSnapshot")
	}
	if !IsPreRestoreSnapshot(PreRestoreSnapshotName(at)) {
		t.Error("PreRestoreSnapshotName does not satisfy IsPreRestoreSnapshot")
	}
}

func TestIsBombVaultSnapshot(t *testing.T) {
	for _, snap := range []string{"bombvault-20260917031500", "bombvault-00000000000000"} {
		if !IsBombVaultSnapshot(snap) {
			t.Errorf("IsBombVaultSnapshot(%q) = false, want true", snap)
		}
	}
	for _, snap := range []string{
		"",
		"bombvault-",
		"bombvault-2026091703150",   // 13 digits
		"bombvault-202609170315000", // 15 digits
		"bombvault-20260917031500x", // trailing letter
		"xbombvault-20260917031500", // leading letter
		"bombvault-prerestore-20260917031500",
		"autosnap_2026-09-17_03:15:00_hourly",
		"nightly",
	} {
		if IsBombVaultSnapshot(snap) {
			t.Errorf("IsBombVaultSnapshot(%q) = true, want false", snap)
		}
	}
}

func TestIsPreRestoreSnapshot(t *testing.T) {
	if !IsPreRestoreSnapshot("bombvault-prerestore-20260917031500") {
		t.Error("a pre-restore name was not recognised")
	}
	for _, snap := range []string{
		"",
		"bombvault-20260917031500",
		"bombvault-prerestore-",
		"bombvault-prerestore-2026091703150",
		"bombvault-prerestore-202609170315000",
		"bombvault-prerestore-20260917031500x",
		"xbombvault-prerestore-20260917031500",
	} {
		if IsPreRestoreSnapshot(snap) {
			t.Errorf("IsPreRestoreSnapshot(%q) = true, want false", snap)
		}
	}
}

func TestStampFromPath(t *testing.T) {
	stamp, ok := StampFromPath("/host/user/cache/appdata/.zfs/snapshot/bombvault-20260917031500")
	if !ok || stamp != "bombvault-20260917031500" {
		t.Fatalf("StampFromPath = %q, %v", stamp, ok)
	}

	for _, p := range []string{
		"/host/user/cache/appdata/.zfs/snapshot/bombvault-20260917031500/sub",
		"/host/user/cache/appdata/.zfs/snapshot/nightly",
		"/host/user/cache/appdata",
		"",
	} {
		if stamp, ok := StampFromPath(p); ok {
			t.Errorf("StampFromPath(%q) = %q, want no stamp", p, stamp)
		}
	}
}

func TestReplicaSnapshotNameIsUTCFourteenDigits(t *testing.T) {
	cest := time.FixedZone("CEST", 2*60*60)
	got := ReplicaSnapshotName(time.Date(2026, 10, 9, 12, 0, 0, 0, cest))
	if want := "bombvault-replica-20261009100000"; got != want {
		t.Errorf("ReplicaSnapshotName = %q, want %q", got, want)
	}
	if !IsReplicaSnapshot(got) {
		t.Error("ReplicaSnapshotName does not satisfy IsReplicaSnapshot")
	}
	if at, ok := StampTime(got); !ok || !at.Equal(time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("StampTime = %v, %v", at, ok)
	}
}

func TestIsReplicaSnapshot(t *testing.T) {
	for _, snap := range []string{
		"",
		"bombvault-replica-",
		"bombvault-replica-2026100910000",
		"bombvault-replica-202610091000000",
		"bombvault-replica-20261009100000x",
		"xbombvault-replica-20261009100000",
		"bombvault-20261009100000",
		"bombvault-prerestore-20261009100000",
	} {
		if IsReplicaSnapshot(snap) {
			t.Errorf("IsReplicaSnapshot(%q) = true, want false", snap)
		}
	}
}

func TestBackupSweepNeverReachesReplicaNames(t *testing.T) {
	replica := ReplicaSnapshotName(time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC))
	if IsBombVaultSnapshot(replica) || IsPreRestoreSnapshot(replica) {
		t.Fatalf("%q passes as a backup or safety snapshot", replica)
	}

	tree := []ListEntry{{Name: "cache/appdata", Type: "filesystem"}, {Name: "cache/appdata/db", Type: "filesystem"}}
	snaps := []SnapshotEntry{
		{Dataset: "cache/appdata", Name: replica},
		{Dataset: "cache/appdata/db", Name: replica},
		{Dataset: "cache/appdata", Name: "bombvault-20261009100000"},
	}
	if got := LeakedStamps(tree, snaps); len(got) != 1 || got[0] != "bombvault-20261009100000" {
		t.Errorf("LeakedStamps = %q, want only the backup's leftover", got)
	}
	if args, err := DestroyRecursiveArgs("cache/appdata", replica); err == nil || args != nil {
		t.Errorf("the backup's recursive destroy took a replica snapshot: %q, %v", args, err)
	}
	if _, ok := StampFromPath("/host/user/cache/appdata/.zfs/snapshot/" + replica); ok {
		t.Error("StampFromPath read a replica snapshot as a backup run's")
	}
}

func TestAnItemRootFitsItsReplicaSnapshot(t *testing.T) {
	if len(ReplicaPrefix) > len(PreRestorePrefix) {
		t.Fatalf("ReplicaPrefix is longer than PreRestorePrefix, so MaxDatasetNameLen no longer covers it")
	}
	root := strings.Repeat("a", MaxDatasetNameLen)
	if !ReplicaNameFits(root) {
		t.Fatalf("an item root of %d bytes does not fit its replica snapshot", len(root))
	}
	full := root + "@" + ReplicaSnapshotName(time.Now())
	if len(full) > MaxSnapshotNameLen {
		t.Fatalf("%d bytes, over ZFS's limit", len(full))
	}

	longest := strings.Repeat("m", MaxSnapshotNameLen-1-len(ReplicaPrefix)-stampLen)
	if !ReplicaNameFits(longest) || ReplicaNameFits(longest+"m") {
		t.Errorf("ReplicaNameFits is off by one at %d bytes", len(longest))
	}
}
