package zfs

import (
	"errors"
	"strings"
	"testing"
)

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTreeArgs(t *testing.T) {
	got, err := TreeArgs("cache/appdata")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"zfs", "list", "-H", "-p", "-r", "-t", "filesystem,volume",
		"-o", "name,type,mountpoint,mounted,canmount,encryption,keystatus,snapdir,referenced,usedbydataset",
		"cache/appdata",
	}
	if !equalArgs(got, want) {
		t.Fatalf("TreeArgs = %q, want %q", got, want)
	}

	if args, err := TreeArgs("cache/../x"); err == nil || args != nil {
		t.Fatalf("TreeArgs of an invalid name = %q, %v", args, err)
	}
}

func TestListArgs(t *testing.T) {
	want := []string{
		"zfs", "list", "-H", "-p", "-t", "filesystem,volume",
		"-o", "name,type,mountpoint,mounted,canmount,encryption,keystatus,snapdir,used,referenced,usedbydataset",
	}
	if got := ListArgs(); !equalArgs(got, want) {
		t.Fatalf("ListArgs = %q, want %q", got, want)
	}
}

func TestSnapshotsArgs(t *testing.T) {
	got, err := SnapshotsArgs("cache/appdata")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zfs", "list", "-H", "-p", "-r", "-t", "snapshot", "-o", "name,creation,used", "cache/appdata"}
	if !equalArgs(got, want) {
		t.Fatalf("SnapshotsArgs = %q, want %q", got, want)
	}
	if _, err := SnapshotsArgs("a@b"); err == nil {
		t.Fatal("SnapshotsArgs accepted a name with @")
	}
}

func TestSnapshotRecursiveArgs(t *testing.T) {
	got, err := SnapshotRecursiveArgs("cache/appdata", "bombvault-20260917031500")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zfs", "snapshot", "-r", "cache/appdata@bombvault-20260917031500"}
	if !equalArgs(got, want) {
		t.Fatalf("SnapshotRecursiveArgs = %q, want %q", got, want)
	}

	for _, snap := range []string{"nightly", "bombvault-prerestore-20260917031500", "bombvault-2026", ""} {
		if args, err := SnapshotRecursiveArgs("cache/appdata", snap); err == nil || args != nil {
			t.Errorf("SnapshotRecursiveArgs took %q: %q, %v", snap, args, err)
		}
	}
}

func TestDestroyRecursiveArgsOnlyBombVaultNames(t *testing.T) {
	got, err := DestroyRecursiveArgs("cache/appdata", "bombvault-20260917031500")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zfs", "destroy", "-r", "cache/appdata@bombvault-20260917031500"}
	if !equalArgs(got, want) {
		t.Fatalf("DestroyRecursiveArgs = %q, want %q", got, want)
	}

	for _, snap := range []string{
		"", "nightly", "bombvault-prerestore-20260917031500",
		"bombvault-2026091703150", "bombvault-20260917031500x", "autosnap_2026-09-17",
	} {
		if args, err := DestroyRecursiveArgs("cache/appdata", snap); err == nil || args != nil {
			t.Errorf("DestroyRecursiveArgs took %q: %q, %v", snap, args, err)
		}
	}
	if args, err := DestroyRecursiveArgs("-cache", "bombvault-20260917031500"); err == nil || args != nil {
		t.Errorf("DestroyRecursiveArgs took an invalid root: %q, %v", args, err)
	}
}

func TestSafetyArgsNeverRecursive(t *testing.T) {
	snap := "bombvault-prerestore-20260917031500"
	got, err := SnapshotSafetyArgs("cache/appdata", snap)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "snapshot", "cache/appdata@" + snap}; !equalArgs(got, want) {
		t.Fatalf("SnapshotSafetyArgs = %q, want %q", got, want)
	}
	got, err = DestroyPreRestoreArgs("cache/appdata", snap)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zfs", "destroy", "cache/appdata@" + snap}; !equalArgs(got, want) {
		t.Fatalf("DestroyPreRestoreArgs = %q, want %q", got, want)
	}

	// A backup snapshot name reaching a safety builder would let a
	// non-recursive destroy remove the root's share of a running backup.
	for _, bad := range []string{"bombvault-20260917031500", "nightly", ""} {
		if args, err := SnapshotSafetyArgs("cache/appdata", bad); err == nil || args != nil {
			t.Errorf("SnapshotSafetyArgs took %q: %q, %v", bad, args, err)
		}
		if args, err := DestroyPreRestoreArgs("cache/appdata", bad); err == nil || args != nil {
			t.Errorf("DestroyPreRestoreArgs took %q: %q, %v", bad, args, err)
		}
	}
}

func TestOnlyDestroyRecursiveArgsEmitsDashR(t *testing.T) {
	snap := "bombvault-20260917031500"
	safety := "bombvault-prerestore-20260917031500"
	tree, err := TreeArgs("cache/appdata")
	if err != nil {
		t.Fatal(err)
	}
	snaps, err := SnapshotsArgs("cache/appdata")
	if err != nil {
		t.Fatal(err)
	}
	take, err := SnapshotRecursiveArgs("cache/appdata", snap)
	if err != nil {
		t.Fatal(err)
	}
	safeTake, err := SnapshotSafetyArgs("cache/appdata", safety)
	if err != nil {
		t.Fatal(err)
	}
	safeDestroy, err := DestroyPreRestoreArgs("cache/appdata", safety)
	if err != nil {
		t.Fatal(err)
	}
	destroy, err := DestroyRecursiveArgs("cache/appdata", snap)
	if err != nil {
		t.Fatal(err)
	}
	prime, err := PrimeArgs("/mnt/cache/appdata", snap)
	if err != nil {
		t.Fatal(err)
	}

	all := [][]string{VersionArgs(), tree, ListArgs(), snaps, take, safeTake, safeDestroy, destroy, prime}
	for _, args := range all {
		for _, flag := range []string{"-R", "-f", "-d", "-n"} {
			for _, a := range args {
				if a == flag {
					t.Errorf("%q carries %q", args, flag)
				}
			}
		}
	}

	for _, args := range all {
		isDestroy := len(args) > 1 && args[1] == "destroy"
		for _, a := range args {
			if a != "-r" {
				continue
			}
			if isDestroy && !equalArgs(args, destroy) {
				t.Errorf("a destroy other than DestroyRecursiveArgs carries -r: %q", args)
			}
		}
	}

	// A destroy argv without an @ would name a dataset, and -r would then
	// take its children with it.
	for _, args := range [][]string{destroy, safeDestroy} {
		if !strings.Contains(args[len(args)-1], "@") {
			t.Errorf("destroy argv without a snapshot: %q", args)
		}
	}
}

func TestPrimeArgsRejectsRelativeOrTraversal(t *testing.T) {
	snap := "bombvault-20260917031500"
	got, err := PrimeArgs("/mnt/cache/appdata", snap)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"stat", "-c", "%d", "/mnt/cache/appdata/.zfs/snapshot/" + snap + "/."}
	if !equalArgs(got, want) {
		t.Fatalf("PrimeArgs = %q, want %q", got, want)
	}

	for _, mp := range []string{"", "mnt/cache", "/mnt/../etc", "/mnt/cache/", "legacy", "none"} {
		if args, err := PrimeArgs(mp, snap); err == nil || args != nil {
			t.Errorf("PrimeArgs took mountpoint %q: %q, %v", mp, args, err)
		}
	}
	if args, err := PrimeArgs("/mnt/cache", "nightly"); err == nil || args != nil {
		t.Errorf("PrimeArgs took a foreign snapshot name: %q, %v", args, err)
	}
}

func TestBuilderErrorsCarryANameCode(t *testing.T) {
	var ne *NameError
	_, err := TreeArgs("a@b")
	if !errors.As(err, &ne) || ne.Code != "invalid-name" {
		t.Fatalf("TreeArgs error = %v, want an invalid-name NameError", err)
	}
}
