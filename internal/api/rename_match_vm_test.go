package api

import "testing"

// A live domain and a not-installed target sharing one non-empty UUID produce
// a suggestion keyed on the live libvirt name.
func TestMatchVMRenamesByUUIDMatch(t *testing.T) {
	live := map[string]string{"win11": "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33"}
	orphans := map[string]string{"windows-11": "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33"}

	got := matchVMRenamesByUUID(live, orphans)
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d: %+v", len(got), got)
	}
	cand, ok := got["win11"]
	if !ok {
		t.Fatalf("expected a suggestion keyed on the live libvirt name %q, got %+v", "win11", got)
	}
	if cand.OldName != "windows-11" || cand.Reason != reasonLibvirtUUID {
		t.Fatalf("candidate = %+v, want OldName=%q Reason=%q", cand, "windows-11", reasonLibvirtUUID)
	}
}

// An empty UUID matches nothing on either side. libvirt always assigns one,
// so an empty value means unknown here, not no UUID.
func TestMatchVMRenamesByUUIDNoUUIDEitherSide(t *testing.T) {
	live := map[string]string{"win11": ""}
	orphans := map[string]string{"windows-11": ""}

	got := matchVMRenamesByUUID(live, orphans)
	if len(got) != 0 {
		t.Fatalf("expected no matches when both sides carry no UUID, got %+v", got)
	}
}

// The same UUID live under two domain names is ambiguous and suggests
// neither.
func TestMatchVMRenamesByUUIDAmbiguousLive(t *testing.T) {
	const uuid = "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33"
	live := map[string]string{"win11": uuid, "win11-clone": uuid}
	orphans := map[string]string{"windows-11": uuid}

	got := matchVMRenamesByUUID(live, orphans)
	if len(got) != 0 {
		t.Fatalf("expected no matches when a UUID is live under two names, got %+v", got)
	}
}

// TestMatchVMRenamesByUUIDAmbiguousOrphan is the mirror: the same UUID
// stored on two not-installed targets is ambiguous and must not produce a
// suggestion for either.
func TestMatchVMRenamesByUUIDAmbiguousOrphan(t *testing.T) {
	const uuid = "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33"
	live := map[string]string{"win11": uuid}
	orphans := map[string]string{"windows-11": uuid, "windows-11-old": uuid}

	got := matchVMRenamesByUUID(live, orphans)
	if len(got) != 0 {
		t.Fatalf("expected no matches when a UUID is stored on two entries, got %+v", got)
	}
}

func TestMatchVMRenamesByUUIDDifferentUUID(t *testing.T) {
	live := map[string]string{"win11": "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33"}
	orphans := map[string]string{"windows-11": "ffffffff-ffff-ffff-ffff-ffffffffffff"}

	got := matchVMRenamesByUUID(live, orphans)
	if len(got) != 0 {
		t.Fatalf("expected no match for a different UUID, got %+v", got)
	}
}
