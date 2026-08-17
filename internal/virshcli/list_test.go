package virshcli

import (
	"context"
	"errors"
	"testing"
)

// vmInfoFromNames is List's wiring logic split out into a pure function that
// takes plain fake func values for the state/title lookups — this package
// has no fake-exec test harness for the real virsh binary (see
// virshcli_uri_test.go/nvram_test.go/zvol_test.go for its established
// convention of testing pure/injectable logic directly instead of shelling
// out in tests), so these tests exercise the friendly-name resolution
// wiring at that level rather than through Client.List itself.

// TestVMInfoFromNamesUnraidStylePassesThrough pins the no-op case: a plain
// Unraid-style name never triggers the extra title lookup, and FriendlyName
// equals Name unchanged.
func TestVMInfoFromNamesUnraidStylePassesThrough(t *testing.T) {
	titleCalls := 0
	titleFn := func(_ context.Context, _ string) (string, error) {
		titleCalls++
		return "should not be called", nil
	}
	stateFn := func(_ context.Context, _ string) (string, error) {
		return "running", nil
	}

	vms := vmInfoFromNames(context.Background(), []string{"Windows10"}, stateFn, titleFn)

	if len(vms) != 1 {
		t.Fatalf("vms = %v, want exactly one entry", vms)
	}
	if vms[0].Name != "Windows10" || vms[0].FriendlyName != "Windows10" {
		t.Fatalf("vms[0] = %+v, want Name=FriendlyName=Windows10", vms[0])
	}
	if vms[0].State != "running" {
		t.Fatalf("vms[0].State = %q, want running", vms[0].State)
	}
	if titleCalls != 0 {
		t.Fatalf("titleFn was called %d times, want 0 for a non-UUID name", titleCalls)
	}
}

// TestVMInfoFromNamesTrueNAS2510StyleNeedsNoExtraCall pins that the 25.10
// "{id}_{name}" style is fully resolved from the raw name alone — no extra
// virsh call (titleFn) is needed or made.
func TestVMInfoFromNamesTrueNAS2510StyleNeedsNoExtraCall(t *testing.T) {
	titleCalls := 0
	titleFn := func(_ context.Context, _ string) (string, error) {
		titleCalls++
		return "should not be called", nil
	}
	stateFn := func(_ context.Context, _ string) (string, error) { return "shut off", nil }

	vms := vmInfoFromNames(context.Background(), []string{"1_debian"}, stateFn, titleFn)

	if len(vms) != 1 {
		t.Fatalf("vms = %v, want exactly one entry", vms)
	}
	if vms[0].Name != "1_debian" {
		t.Fatalf("vms[0].Name = %q, want the RAW libvirt name 1_debian unchanged", vms[0].Name)
	}
	if vms[0].FriendlyName != "debian" {
		t.Fatalf("vms[0].FriendlyName = %q, want debian", vms[0].FriendlyName)
	}
	if titleCalls != 0 {
		t.Fatalf("titleFn was called %d times, want 0 for the 25.10 id_name style", titleCalls)
	}
}

// TestVMInfoFromNamesUUIDStyleResolvesTitle pins the TrueNAS 26 happy path:
// a UUID-shaped name triggers exactly one titleFn call, and a non-empty
// <title> result becomes FriendlyName — while Name (the identifier every
// real virsh command must keep using) stays the raw UUID.
func TestVMInfoFromNamesUUIDStyleResolvesTitle(t *testing.T) {
	const uuid = "550e8400-e29b-41d4-a716-446655440000"
	titleCalls := 0
	titleFn := func(_ context.Context, name string) (string, error) {
		titleCalls++
		if name != uuid {
			t.Fatalf("titleFn called with %q, want the raw UUID %q", name, uuid)
		}
		return "my-debian-vm", nil
	}
	stateFn := func(_ context.Context, _ string) (string, error) { return "running", nil }

	vms := vmInfoFromNames(context.Background(), []string{uuid}, stateFn, titleFn)

	if len(vms) != 1 {
		t.Fatalf("vms = %v, want exactly one entry", vms)
	}
	if vms[0].Name != uuid {
		t.Fatalf("vms[0].Name = %q, want the raw UUID unchanged", vms[0].Name)
	}
	if vms[0].FriendlyName != "my-debian-vm" {
		t.Fatalf("vms[0].FriendlyName = %q, want the resolved <title> value", vms[0].FriendlyName)
	}
	if titleCalls != 1 {
		t.Fatalf("titleFn was called %d times, want exactly 1", titleCalls)
	}
}

// TestVMInfoFromNamesUUIDStyleFallsBackOnTitleError proves a failed title
// lookup (e.g. the extra DumpXML call errors) does not fail the whole List —
// it falls back to the UUID itself as FriendlyName, mirroring how a failed
// State lookup already falls back to "unknown" rather than erroring out.
func TestVMInfoFromNamesUUIDStyleFallsBackOnTitleError(t *testing.T) {
	const uuid = "550e8400-e29b-41d4-a716-446655440000"
	titleFn := func(_ context.Context, _ string) (string, error) {
		return "", errors.New("dumpxml failed")
	}
	stateFn := func(_ context.Context, _ string) (string, error) { return "running", nil }

	vms := vmInfoFromNames(context.Background(), []string{uuid}, stateFn, titleFn)

	if len(vms) != 1 {
		t.Fatalf("vms = %v, want exactly one entry", vms)
	}
	if vms[0].FriendlyName != uuid {
		t.Fatalf("vms[0].FriendlyName = %q, want fallback to the UUID %q on a title-lookup error", vms[0].FriendlyName, uuid)
	}
}

// TestVMInfoFromNamesUUIDStyleFallsBackOnEmptyTitle mirrors the error case
// but for a successful call that simply found no <title> element (empty
// string, not an error) — same fallback to the UUID.
func TestVMInfoFromNamesUUIDStyleFallsBackOnEmptyTitle(t *testing.T) {
	const uuid = "550e8400-e29b-41d4-a716-446655440000"
	titleFn := func(_ context.Context, _ string) (string, error) { return "", nil }
	stateFn := func(_ context.Context, _ string) (string, error) { return "running", nil }

	vms := vmInfoFromNames(context.Background(), []string{uuid}, stateFn, titleFn)

	if len(vms) != 1 {
		t.Fatalf("vms = %v, want exactly one entry", vms)
	}
	if vms[0].FriendlyName != uuid {
		t.Fatalf("vms[0].FriendlyName = %q, want fallback to the UUID %q when <title> is empty", vms[0].FriendlyName, uuid)
	}
}

// TestVMInfoFromNamesStateFailureFallsBackToUnknown is the pre-existing
// State-failure-tolerance regression pin (List already behaved this way
// before FriendlyName existed) — must be unaffected by this task's changes.
func TestVMInfoFromNamesStateFailureFallsBackToUnknown(t *testing.T) {
	stateFn := func(_ context.Context, _ string) (string, error) { return "", errors.New("boom") }
	titleFn := func(_ context.Context, _ string) (string, error) { return "", nil }

	vms := vmInfoFromNames(context.Background(), []string{"Windows10"}, stateFn, titleFn)

	if len(vms) != 1 || vms[0].State != "unknown" {
		t.Fatalf("vms = %+v, want a single entry with State=unknown on a state lookup error", vms)
	}
}
