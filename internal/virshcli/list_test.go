package virshcli

import (
	"context"
	"errors"
	"testing"
)

// There is no fake virsh, so these tests call vmInfoFromNames, the part of
// List that takes the state and title lookups as functions.

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

// Name stays the raw UUID because every virsh command needs it; only
// FriendlyName takes the <title>.
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

// A failed title lookup does not fail List, just as a failed state lookup
// does not.
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

func TestVMInfoFromNamesStateFailureFallsBackToUnknown(t *testing.T) {
	stateFn := func(_ context.Context, _ string) (string, error) { return "", errors.New("boom") }
	titleFn := func(_ context.Context, _ string) (string, error) { return "", nil }

	vms := vmInfoFromNames(context.Background(), []string{"Windows10"}, stateFn, titleFn)

	if len(vms) != 1 || vms[0].State != "unknown" {
		t.Fatalf("vms = %+v, want a single entry with State=unknown on a state lookup error", vms)
	}
}
