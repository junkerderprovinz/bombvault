package hostsnap

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

func lastArgs(sh *fakeShell) []string { return sh.calls[len(sh.calls)-1] }

func TestDeleteAnswersForEachSnapshotInOneRoundTrip(t *testing.T) {
	sh := &fakeShell{answers: []answer{{contains: "destroy", stdout: "R0\tok\n" +
		"R1\tfail\tcannot destroy snapshot tank/a@b: dataset is busy\n" +
		"R2\tfail\tcannot destroy 'tank/a@c': snapshot has dependent clones use '-R' to destroy the following datasets:\n" +
		"R3\tfail\tcould not find any snapshots to destroy; check snapshot names.\n" +
		"R4\tfail\tcannot destroy snapshot tank/a@e: it's being held. Run 'zfs holds -r tank/a@e' to see holders.\n"}}}
	refs := []Ref{
		{Kind: KindZFS, Source: "tank/a", Name: "a"},
		{Kind: KindZFS, Source: "tank/a", Name: "b"},
		{Kind: KindZFS, Source: "tank/a", Name: "c"},
		{Kind: KindZFS, Source: "tank/a", Name: "d"},
		{Kind: KindZFS, Source: "tank/a", Name: "e"},
		{Kind: KindZFS, Source: "tank/a", Name: "bombvault-replica-20261101000000"},
	}
	got := New(sh, nil).Delete(context.Background(), refs)
	if len(sh.calls) != 1 {
		t.Fatalf("%d round trips, want one", len(sh.calls))
	}
	codes := map[string]string{}
	for _, r := range got {
		codes[r.Ref.Name] = r.Code
	}
	want := map[string]string{"a": "ok", "b": CodeBusy, "c": CodeClones, "d": "ok", "e": CodeHeld, "bombvault-replica-20261101000000": CodeManaged}
	for name, code := range want {
		if codes[name] != code {
			t.Errorf("%s: code %q, want %q", name, codes[name], code)
		}
	}
	if strings.Contains(strings.Join(lastArgs(sh), " "), "replica") {
		t.Error("the replica's base was sent to destroy")
	}
}

func TestANameThatWouldReachADatasetIsRefused(t *testing.T) {
	sh := &fakeShell{}
	h := New(sh, nil)
	for _, ref := range []Ref{
		{Kind: KindZFS, Source: "tank/a", Name: ""},
		{Kind: KindZFS, Source: "tank/a@x", Name: "y"},
		{Kind: KindZFS, Source: "-r tank", Name: "y"},
		{Kind: KindZFS, Source: "tank", Name: "a,tank/b@c"},
		{Kind: KindBtrfs, FS: "/mnt/cache", Name: "../etc"},
		{Kind: KindBtrfs, FS: "/mnt/cache", Name: ""},
		{Kind: KindBtrfs, FS: "relative", Name: "x"},
	} {
		if r := h.Delete(context.Background(), []Ref{ref})[0]; r.Code != "invalid-name" {
			t.Errorf("Delete(%+v) = %q, want invalid-name", ref, r.Code)
		}
	}
	if len(sh.calls) != 0 {
		t.Errorf("the host was called: %v", sh.calls)
	}
}

func TestTheHoldIsBombVaultsOwn(t *testing.T) {
	sh := &fakeShell{}
	h := New(sh, nil)
	ref := Ref{Kind: KindZFS, Source: "tank/a", Name: "before-update"}
	if err := h.Hold(context.Background(), ref, true); err != nil {
		t.Fatal(err)
	}
	if args := lastArgs(sh); !slices.Equal(args[len(args)-4:], []string{"hold", "--", HoldTag, "tank/a@before-update"}) {
		t.Errorf("hold argv = %q", args)
	}
	if err := h.Hold(context.Background(), ref, false); err != nil {
		t.Fatal(err)
	}
	if args := lastArgs(sh); !slices.Equal(args[len(args)-4:], []string{"release", "--", HoldTag, "tank/a@before-update"}) {
		t.Errorf("release argv = %q", args)
	}
	if err := h.Hold(context.Background(), Ref{Kind: KindBtrfs, FS: "/mnt/cache", Name: "x"}, true); CodeOf(err) != CodeNoSupport {
		t.Errorf("btrfs hold = %v, want not-supported", err)
	}
}

func TestEstimateAsksZFSForExactlyTheSelection(t *testing.T) {
	sh := &fakeShell{answers: []answer{
		{contains: "tank/a@x,y", stdout: "destroy\ttank/a@x\ndestroy\ttank/a@y\nreclaim\t3000\n"},
		{contains: "tank/b@z", stdout: "destroy\ttank/b@z\nreclaim\t500\n"},
	}}
	n, err := New(sh, nil).Estimate(context.Background(), []Ref{
		{Kind: KindZFS, Source: "tank/a", Name: "x"},
		{Kind: KindZFS, Source: "tank/b", Name: "z"},
		{Kind: KindZFS, Source: "tank/a", Name: "y"},
		{Kind: KindVM, Source: "win11", Name: "s"},
	})
	if err != nil || n != 3500 {
		t.Fatalf("Estimate = %d, %v, want 3500", n, err)
	}
	if args := sh.calls[0]; !slices.Contains(args, "-n") {
		t.Errorf("the estimate was not a dry run: %q", args)
	}
}

func TestWhereNamesTheContainerThatHoldsTheMount(t *testing.T) {
	id := strings.Repeat("ab", 32)
	sh := &fakeShell{answers: []answer{{contains: "found()", stdout: "M\t1\tmnt:[4026531841]\t/mnt/tank/apps/.zfs/snapshot/before\\040update\t1\t0::/init.scope \n" +
		"M\t4242\tmnt:[4026532999]\t/data/snap\t0\t0::/docker/" + id + " \n"}}}
	mounts, err := New(sh, nil).Where(context.Background(), Ref{Kind: KindZFS, Source: "tank/apps", Name: "before-update"})
	if err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 2 || !mounts[0].Host || mounts[0].Path != "/mnt/tank/apps/.zfs/snapshot/before update" || mounts[1].ContainerID != id || mounts[1].PID != 4242 {
		t.Fatalf("mounts = %+v", mounts)
	}
}

func TestFreeAndDeleteReportsEachUnmountAndTheDestroy(t *testing.T) {
	sh := &fakeShell{answers: []answer{{contains: "nsenter", stdout: "U\t1\tmnt:[1]\t/mnt/tank/apps/.zfs/snapshot/x\t1\t0\t\t\n" +
		"U\t77\tmnt:[2]\t/snap\t0\t32\tumount: /snap: target is busy.\t0::/docker/" + strings.Repeat("cd", 32) + "\n" +
		"R\tfail\tcannot destroy snapshot tank/apps@x: dataset is busy\n"}}}
	mounts, err := New(sh, nil).FreeAndDelete(context.Background(), Ref{Kind: KindZFS, Source: "tank/apps", Name: "x"})
	if CodeOf(err) != CodeBusy {
		t.Errorf("err = %v, want busy", err)
	}
	if len(mounts) != 2 || mounts[0].Code != "ok" || mounts[1].Code != CodeFailed || mounts[1].ContainerID == "" {
		t.Fatalf("mounts = %+v", mounts)
	}
}

func TestAPlanRunTakesItsSnapshotAndLetsTheOldestGo(t *testing.T) {
	id := "1a2b3c4d" + strings.Repeat("0", 24)
	list := "tank/apps@bombvault-auto-1a2b3c4d-20261101000000\t100\t1\t1\t0\t-\n" +
		"tank/apps@bombvault-auto-1a2b3c4d-20261102000000\t200\t1\t1\t0\t-\n" +
		"tank/apps@bombvault-auto-1a2b3c4d-20261117120000\t300\t1\t1\t0\t-\n" +
		"tank/apps@keep-me\t50\t1\t1\t0\t-\n"
	sh := &fakeShell{answers: []answer{
		{contains: "\x00list\x00", stdout: list},
		{contains: "destroy", stdout: "R0\tok\n"},
	}}
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	res := New(sh, nil).RunPlan(context.Background(), Plan{ID: id, Kind: KindZFS, Source: "tank/apps", Keep: 2}, now)
	if res.Code != "ok" || res.Created != "bombvault-auto-1a2b3c4d-20261117120000" {
		t.Fatalf("run = %+v", res)
	}
	if !slices.Equal(res.Removed, []string{"bombvault-auto-1a2b3c4d-20261101000000"}) {
		t.Errorf("removed %v, want only the oldest of the plan", res.Removed)
	}
	if !slices.Contains(sh.calls[0], "tank/apps@bombvault-auto-1a2b3c4d-20261117120000") {
		t.Errorf("first call %q, want the snapshot", sh.calls[0])
	}
}

func TestAPlanLeavesASleepingPoolAlone(t *testing.T) {
	sh := &fakeShell{answers: []answer{{contains: "states", stdout: "T\tzfs\tok\nD\tzfs\tdisk1\tsdb\tasleep\nP\tzfs\tdisk1\t1\n"}}}
	res := New(sh, nil).RunPlan(context.Background(), Plan{ID: "1a2b3c4d", Kind: KindZFS, Source: "disk1/media", Keep: 3, SkipAsleep: true}, time.Now())
	if res.Code != CodeSkippedAsleep || len(sh.calls) != 1 {
		t.Fatalf("run = %+v after %d calls, want a skip and nothing else", res, len(sh.calls))
	}
}

func TestAVMSnapshotGoesThroughVirsh(t *testing.T) {
	vms := &fakeVMs{tables: map[string]string{}}
	h := New(&fakeShell{}, vms)
	if err := h.Create(context.Background(), Source{Kind: KindVM, Name: "win11"}, "before-update", false); err != nil {
		t.Fatal(err)
	}
	if r := h.Delete(context.Background(), []Ref{{Kind: KindVM, Source: "win11", Name: "clean install"}})[0]; r.Code != "ok" {
		t.Errorf("delete = %+v", r)
	}
	if !slices.Equal(vms.created, []string{"win11@before-update"}) || !slices.Equal(vms.deleted, []string{"win11@clean install"}) {
		t.Errorf("virsh saw %v and %v", vms.created, vms.deleted)
	}
	if err := h.Rename(context.Background(), Ref{Kind: KindVM, Source: "win11", Name: "a"}, "b"); CodeOf(err) != CodeNoSupport {
		t.Errorf("vm rename = %v, want not-supported", err)
	}
}

func TestABtrfsSnapshotLandsInBombVaultsFolder(t *testing.T) {
	sh := &fakeShell{}
	if err := New(sh, nil).Create(context.Background(), Source{Kind: KindBtrfs, FS: "/mnt/cache", Name: "appdata/plex"}, "before-update", false); err != nil {
		t.Fatal(err)
	}
	args := lastArgs(sh)
	tail := args[len(args)-4:]
	want := []string{"/mnt/cache", "/mnt/cache/appdata/plex", "/mnt/cache/" + BtrfsFolder + "/appdata_plex", "before-update"}
	if !slices.Equal(tail, want) {
		t.Errorf("args = %q, want %q", tail, want)
	}
}
