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
