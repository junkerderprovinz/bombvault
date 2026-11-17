package hostsnap

import (
	"testing"
	"time"
)

func TestValidateNameAcceptsWhatEveryKindAccepts(t *testing.T) {
	for _, ok := range []string{"before-update", "manual-20261117120000", "v1.2:rc_3"} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "-r", "a b", "a/b", "a@b", "a,b", "..", ".", "ä", string(make([]byte, 201))} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("ValidateName(%q) = nil, want a refusal", bad)
		}
	}
}

func TestAPlanRecognisesOnlyItsOwnSnapshots(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	name := PlanSnapshotName("1A2B3C4D5E6F7A8B9C0D1E2F3A4B5C6D", now)
	if name != "bombvault-auto-1a2b3c4d-20261117120000" {
		t.Fatalf("PlanSnapshotName = %q", name)
	}
	if !OwnedByPlan(name, "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d") {
		t.Error("the plan does not recognise its own snapshot")
	}
	if OwnedByPlan(name, "ffffffff5e6f7a8b9c0d1e2f3a4b5c6d") {
		t.Error("another plan claims the snapshot")
	}
	for _, other := range []string{"bombvault-20261117120000", name + "-kept", "manual-1", "bombvault-auto-1a2b3c4d-2026"} {
		if OwnedByPlan(other, "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d") {
			t.Errorf("the plan claims %q", other)
		}
	}
}

func TestOwnerTellsTheSnapshotsOfRunsApart(t *testing.T) {
	cases := map[string]string{
		"bombvault-20261117120000":              "backup",
		"bombvault-replica-20261117120000":      "replica",
		"bombvault-prerestore-20261117120000":   "safety",
		"bombvault-auto-1a2b3c4d-20261117120000": "plan",
		"autosnap_2026-11-17_12:00:00_hourly":   "",
	}
	for name, want := range cases {
		if got := Owner(name); got != want {
			t.Errorf("Owner(%q) = %q, want %q", name, got, want)
		}
	}
}

func planSnap(name string, at int64, held bool) Snapshot {
	s := Snapshot{Ref: Ref{Kind: KindZFS, Source: "tank/apps", Name: name}, Label: name, CreatedAt: at}
	if held {
		s.Holds = []string{HoldTag}
	}
	return s
}

func TestRetentionRemovesOnlyThePlansOldestSnapshots(t *testing.T) {
	const id = "1a2b3c4d00000000000000000000000"
	snaps := []Snapshot{
		planSnap("bombvault-auto-1a2b3c4d-20261101000000", 1, false),
		planSnap("bombvault-auto-1a2b3c4d-20261102000000", 2, true),
		planSnap("bombvault-auto-1a2b3c4d-20261103000000", 3, false),
		planSnap("bombvault-auto-1a2b3c4d-20261104000000", 4, false),
		planSnap("bombvault-auto-1a2b3c4d-20261105000000", 5, false),
		planSnap("bombvault-auto-99999999-20261101000000", 0, false),
		planSnap("before-update", 0, false),
	}
	got := Expired(snaps, id, 2)
	if len(got) != 2 || got[0].Name != "bombvault-auto-1a2b3c4d-20261103000000" || got[1].Name != "bombvault-auto-1a2b3c4d-20261101000000" {
		t.Fatalf("Expired = %v, want the 3rd and the 1st of the plan", names(got))
	}
	if Expired(snaps, id, 0) == nil {
		t.Error("a keep of 0 removes nothing, want it to keep one")
	}
	if len(Expired(snaps, id, 0)) != 3 {
		t.Errorf("a keep of 0 kept %d, want 1", 4-len(Expired(snaps, id, 0)))
	}
}

func names(snaps []Snapshot) []string {
	out := make([]string, len(snaps))
	for i, s := range snaps {
		out[i] = s.Name
	}
	return out
}
