package zfsrepl

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

func TestBringBackLandsTheWholeTreeNextToTheOriginal(t *testing.T) {
	r := newRig(t)
	r.src.add("cache/appdata/vault", "filesystem", true)
	r.src.add("cache/appdata/vm", "volume", false)
	first := r.run()
	r.ok(first)
	r.src.resetCalls()

	at := time.Unix(1760000000, 42)
	got, err := BringBack(context.Background(), r.dst, r.src, Restore{
		Replica: rootTarget, Snapshot: first.Snapshot, Dataset: "cache/appdata",
		Now: func() time.Time { return at },
	})
	if err != nil {
		t.Fatal(err)
	}
	dest := "cache/appdata-bombvault-restore-1760000000000000042"
	if got.Root != dest || got.Skipped != nil {
		t.Fatalf("restored = %+v", got)
	}
	want := []RestoredMember{
		{Dataset: dest},
		{Dataset: dest + "/plex"},
		{Dataset: dest + "/vault", Encrypted: true},
		{Dataset: dest + "/vm", Volume: true},
	}
	if !slices.Equal(got.Members, want) {
		t.Fatalf("members = %+v, want %+v", got.Members, want)
	}
	for _, m := range want {
		if snaps := r.src.snapNames(m.Dataset); !slices.Equal(snaps, []string{first.Snapshot}) {
			t.Errorf("%s holds %q", m.Dataset, snaps)
		}
	}
	if send := callFor(t, r.dst, "send", rootTarget+"/vault"); !slices.Contains(send, "-w") {
		t.Errorf("the encrypted member was not sent raw: %q", send)
	}
	for _, c := range r.src.callsOf("receive") {
		if dest := c[len(c)-1]; !strings.Contains(dest, "-bombvault-restore-") || slices.Contains(c, "-s") {
			t.Errorf("a bring back received %q", c)
		}
	}
}

func TestBringBackLeavesOutAMemberWithoutTheSnapshotAndWhatIsBelowIt(t *testing.T) {
	r := newRig(t)
	first := r.run()
	r.ok(first)
	r.dst.add(rootTarget+"/later", "filesystem", false)
	r.dst.add(rootTarget+"/later/deep", "filesystem", false)
	r.dst.snapshot(rootTarget+"/later/deep", first.Snapshot)

	got, err := BringBack(context.Background(), r.dst, r.src, Restore{
		Replica: rootTarget, Snapshot: first.Snapshot, Dataset: "cache/appdata", Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Skipped, []string{rootTarget + "/later"}) || len(got.Members) != 2 {
		t.Errorf("restored = %+v, want root and plex with later skipped", got)
	}
}

func TestBringBackOfAPoolRootLandsInsideThePool(t *testing.T) {
	at := time.Unix(1760000000, 42)
	if got := RestoreName("cache", at); got != "cache/bombvault-restore-1760000000000000042" {
		t.Errorf("RestoreName of a pool root = %q", got)
	}
	if got := RestoreName("cache/appdata", at); got != "cache/appdata-bombvault-restore-1760000000000000042" {
		t.Errorf("RestoreName = %q", got)
	}
}

func TestBringBackRefusesASnapshotTheReplicaDoesNotHold(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	_, err := BringBack(context.Background(), r.dst, r.src, Restore{
		Replica: rootTarget, Snapshot: "bombvault-replica-20200101000000", Dataset: "cache/appdata",
		Now: time.Now,
	})
	if Code(err) != "not-found" {
		t.Fatalf("err = %v, want not-found", err)
	}
	if r.src.callsOf("receive") != nil {
		t.Error("something was received")
	}
}

func TestReleaseHoldsLetsTheTreeBeDestroyed(t *testing.T) {
	r := newRig(t)
	res := r.run()
	r.ok(res)
	destroy, err := zfs.DestroyReplicaRecursiveArgs("cache/appdata", res.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.src.Run(context.Background(), destroy); !zfs.IsBusy(err) {
		t.Fatalf("destroy of a held tree = %v, want busy", err)
	}

	if err := ReleaseHolds(context.Background(), r.src, "cache/appdata"); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseHolds(context.Background(), r.src, "cache/appdata"); err != nil {
		t.Errorf("a second release failed: %v", err)
	}
	if _, err := r.src.Run(context.Background(), destroy); err != nil {
		t.Errorf("destroy after the release: %v", err)
	}
}
