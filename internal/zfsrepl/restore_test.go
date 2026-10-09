package zfsrepl

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

func TestBringBackLandsNextToTheOriginal(t *testing.T) {
	r := newRig(t)
	r.src.add("cache/appdata/vault", "filesystem", true)
	first := r.run()
	r.ok(first)
	r.src.resetCalls()

	at := time.Unix(1760000000, 42)
	for _, c := range []struct{ replica, dataset string }{
		{rootTarget, "cache/appdata"},
		{base + "/cache/appdata/vault", "cache/appdata/vault"},
	} {
		dest, err := BringBack(context.Background(), r.dst, r.src, Restore{
			Replica: c.replica, Snapshot: first.Snapshot, Dataset: c.dataset,
			Now: func() time.Time { return at },
		})
		if err != nil {
			t.Fatalf("BringBack %s: %v", c.dataset, err)
		}
		if want := c.dataset + "-bombvault-restore-1760000000000000042"; dest != want {
			t.Errorf("dest = %q, want %q", dest, want)
		}
		if got := r.src.snapNames(dest); !slices.Equal(got, []string{first.Snapshot}) {
			t.Errorf("%s holds %q", dest, got)
		}
		send := callFor(t, r.dst, "send", c.replica)
		if raw := slices.Contains(send, "-w"); raw != strings.HasSuffix(c.dataset, "vault") {
			t.Errorf("send = %q", send)
		}
	}
	for _, c := range r.src.callsOf("receive") {
		if dest := c[len(c)-1]; !strings.Contains(dest, "-bombvault-restore-") || slices.Contains(c, "-s") {
			t.Errorf("a bring back received %q", c)
		}
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
