package zfs

import (
	"regexp"
	"testing"
)

func TestAllCodesAreUniqueKebabCase(t *testing.T) {
	shape := regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)
	seen := map[string]bool{}
	for _, list := range [][]string{AllCodes, MemberOutcomes} {
		for _, code := range list {
			if !shape.MatchString(code) {
				t.Errorf("%q is not a kebab-case code", code)
			}
			if seen[code] {
				t.Errorf("%q appears twice", code)
			}
			seen[code] = true
		}
	}

	want := []string{
		"ok", "ssh-missing", "host-placeholder", "host-fallback", "ssh-unreachable",
		"ssh-auth", "zfs-not-found", "zfs-permission", "uri-mismatch", "zfs-error",
		"propagation-missing",
		"invalid-name", "name-too-long", "invalid-exclude", "not-found", "not-filesystem",
		"overlaps-item", "docker-storage", "nothing-readable", "snapshot-failed",
		"containers-busy", "consistency-stop-failed", "pre-snapshot-failed",
		"container-unknown", "container-is-self", "leftover-snapshots",
		"zvol", "canmount-off", "legacy-mount", "no-mountpoint", "not-mounted",
		"key-not-loaded", "snapdir-disabled", "not-visible", "shfs-only",
		"snapshot-not-visible", "snapshot-loop", "backup-failed", "not-reached", "gone",
		"read-only-mount", "destination-not-mounted", "not-enough-space",
		"safety-snapshot-failed", "safety-name-too-long",
	}
	if len(AllCodes) != len(want) {
		t.Fatalf("AllCodes has %d entries, want %d", len(AllCodes), len(want))
	}
	for _, code := range want {
		if !contains(AllCodes, code) {
			t.Errorf("AllCodes is missing %q", code)
		}
	}

	outcomes := []string{"backed-up", "empty", "excluded"}
	if len(MemberOutcomes) != len(outcomes) {
		t.Fatalf("MemberOutcomes = %q, want %q", MemberOutcomes, outcomes)
	}
	for _, o := range outcomes {
		if !contains(MemberOutcomes, o) {
			t.Errorf("MemberOutcomes is missing %q", o)
		}
	}
}

func TestInternalClassifyCodesNeverReachAllCodes(t *testing.T) {
	// busy drives the destroy retry and exists becomes snapshot-failed, so
	// neither has a sentence and neither may be stored on a row.
	for _, code := range []string{"busy", "exists"} {
		if contains(AllCodes, code) || contains(MemberOutcomes, code) {
			t.Errorf("%q is an internal result and must not be a reason code", code)
		}
	}
}

func TestEveryMemberCodeHasASentence(t *testing.T) {
	entries := []ListEntry{
		{Name: "cache/appdata/db", Type: "volume"},
		{Name: "cache/appdata/legacy", Type: "filesystem", Mountpoint: "legacy"},
		{Name: "cache/appdata/none", Type: "filesystem", Mountpoint: "none"},
		{Name: "cache/appdata/off", Type: "filesystem", Mountpoint: "/mnt/x", Canmount: "off"},
		{Name: "cache/appdata/noauto", Type: "filesystem", Mountpoint: "/mnt/x", Canmount: "noauto"},
		{Name: "cache/appdata/enc", Type: "filesystem", Mountpoint: "/mnt/x", Canmount: "on", Keystatus: "unavailable"},
		{Name: "cache/appdata/plex", Type: "filesystem", Mountpoint: "/mnt/x", Canmount: "on", Mounted: true, Snapdir: "disabled"},
		{Name: "cache/appdata/%recv", Type: "filesystem", Mountpoint: "/mnt/x", Canmount: "on", Mounted: true},
		{Name: "cache/appdata/deep", Type: "filesystem", Mountpoint: "/mnt/x", Canmount: "on", Mounted: true},
	}
	for _, e := range entries {
		code := MemberCode(e, []string{"cache/appdata/deep"})
		if code == "" {
			continue
		}
		if !contains(AllCodes, code) && !contains(MemberOutcomes, code) {
			t.Errorf("MemberCode returned %q, which has no sentence", code)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
