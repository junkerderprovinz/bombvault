package api

import (
	"context"
	"slices"
	"testing"
)

func TestTheZvolHostReadsSnapshotsWithTheirClones(t *testing.T) {
	host := sshZFSHost{ssh: &fakeHostSSH{runOut: "tank/a@bombvault-vm-20261015120000\ttank/a-bombvault-vm-20261015120000,tank/b\n" +
		"tank/a@mine\t-\n"}}

	snaps, err := host.Snapshots(context.Background(), "tank/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 || snaps[0].Name != "tank/a@bombvault-vm-20261015120000" ||
		!slices.Equal(snaps[0].Clones, []string{"tank/a-bombvault-vm-20261015120000", "tank/b"}) ||
		snaps[1].Name != "tank/a@mine" || snaps[1].Clones != nil {
		t.Fatalf("snapshots = %+v", snaps)
	}
}

func TestTheZvolHostFindsAZvolAndItsSize(t *testing.T) {
	host := sshZFSHost{ssh: &fakeHostSSH{runOut: "tank/a\t1073741824\ntank/ab\t5\n"}}

	if size, ok, err := host.VolSize(context.Background(), "tank/a"); err != nil || !ok || size != 1<<30 {
		t.Fatalf("tank/a: %d, %v, %v", size, ok, err)
	}
	if _, ok, err := host.VolSize(context.Background(), "tank/c"); err != nil || ok {
		t.Fatalf("tank/c: found %v, %v; want it missing", ok, err)
	}
}
