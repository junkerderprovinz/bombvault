package store_test

import "testing"

func TestAnOwnFolderIsOnUntilItIsSwitchedOff(t *testing.T) {
	r := newRepo(t)
	if off, err := r.LocalOff(); err != nil || len(off) != 0 {
		t.Fatalf("LocalOff = %v, %v, want every folder on", off, err)
	}
	if err := r.SetLocal("zfs", false); err != nil {
		t.Fatal(err)
	}
	if off, err := r.LocalOff(); err != nil || !off["zfs"] || len(off) != 1 {
		t.Fatalf("LocalOff = %v, %v, want only zfs off", off, err)
	}
	if err := r.SetLocal("zfs", true); err != nil {
		t.Fatal(err)
	}
	if off, err := r.LocalOff(); err != nil || len(off) != 0 {
		t.Fatalf("LocalOff = %v, %v, want zfs on again", off, err)
	}
}

func TestASectionWithPlacementHasNoFolderSwitch(t *testing.T) {
	r := newRepo(t)
	if err := r.SetLocal("containers", false); err == nil {
		t.Fatal("SetLocal took the containers' folder")
	}
}

func TestAPrimarySwitchedOffRemembersWhereItPointed(t *testing.T) {
	r := newRepo(t)
	if err := r.SetPrimaryOff("flash", "b2:bucket:flash", true); err != nil {
		t.Fatal(err)
	}
	if at, err := r.PrimaryOffAt("flash"); err != nil || at != "b2:bucket:flash" {
		t.Fatalf("PrimaryOffAt = %q, %v", at, err)
	}
	if at, err := r.PrimaryOffAt("config"); err != nil || at != "" {
		t.Fatalf("PrimaryOffAt config = %q, %v, want nothing", at, err)
	}
	if off, err := r.LocalOff(); err != nil || len(off) != 0 {
		t.Fatalf("LocalOff = %v, %v, want the primary's switch kept apart", off, err)
	}
	if err := r.SetPrimaryOff("flash", "", false); err != nil {
		t.Fatal(err)
	}
	if at, err := r.PrimaryOffAt("flash"); err != nil || at != "" {
		t.Fatalf("PrimaryOffAt after switching on = %q, %v", at, err)
	}
}
