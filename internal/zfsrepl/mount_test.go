package zfsrepl

import (
	"slices"
	"testing"
)

const mountState = "mountpoint,keystatus,mounted,canmount,readonly"

// mountCalls are the argv a run sent the target to read and change how a copy
// is mounted, in order.
func mountCalls(h *fakeHost, dataset string) [][]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out [][]string
	for _, c := range h.calls {
		if c[len(c)-1] != dataset || len(c) < 2 {
			continue
		}
		switch {
		case c[1] == "mount", c[1] == "set", c[1] == "get" && slices.Contains(c, mountState):
			out = append(out, c)
		}
	}
	return out
}

func TestACopyStaysUnmountedWhileTheServerIsNotAskedToMount(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	r.ok(r.run())

	for _, ds := range []string{"cache/appdata", "cache/appdata/plex"} {
		if calls := mountCalls(r.dst, base+"/"+ds); len(calls) != 0 {
			t.Errorf("%s: mount commands without the option: %q", ds, calls)
		}
		d := r.dst.get(base + "/" + ds)
		if d.mounted || d.props["canmount"] != "noauto" || d.props["readonly"] != "on" {
			t.Errorf("%s: mounted %v, canmount %q, readonly %q", ds, d.mounted, d.props["canmount"], d.props["readonly"])
		}
	}
}

func TestAFirstRunMountsEveryCopyReadOnlyAndMountableAtBoot(t *testing.T) {
	r := newRig(t)
	r.entry.Mount = true
	res := r.run()
	r.ok(res)

	for _, ds := range []string{"cache/appdata", "cache/appdata/plex"} {
		target := base + "/" + ds
		want := [][]string{
			{"zfs", "get", "-H", "-p", "-o", "property,value", mountState, target},
			{"zfs", "mount", target},
			{"zfs", "set", "canmount=on", target},
		}
		if got := mountCalls(r.dst, target); !slices.EqualFunc(got, want, slices.Equal) {
			t.Errorf("%s: commands %q, want %q", ds, got, want)
		}
		d := r.dst.get(target)
		if !d.mounted || d.props["canmount"] != "on" || d.props["readonly"] != "on" {
			t.Errorf("%s: mounted %v, canmount %q, readonly %q", ds, d.mounted, d.props["canmount"], d.props["readonly"])
		}
		if m := member(t, res, ds); m.Mount != MountDone {
			t.Errorf("%s: mount %q, want %q", ds, m.Mount, MountDone)
		}
	}
}

func TestAnIncrementIntoAMountedCopyLeavesItMountedWithoutTouchingIt(t *testing.T) {
	r := newRig(t)
	r.entry.Mount = true
	r.ok(r.run())
	r.dst.resetCalls()
	res := r.run()
	r.ok(res)

	target := base + "/cache/appdata"
	recv := callFor(t, r.dst, "receive", target)
	if !hasSeq(recv, "-s", "-u") || slices.Contains(recv, "-o") {
		t.Errorf("increment into the mounted copy = %q", recv)
	}
	want := [][]string{{"zfs", "get", "-H", "-p", "-o", "property,value", mountState, target}}
	if got := mountCalls(r.dst, target); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("commands after an increment %q, want only the read %q", got, want)
	}
	d := r.dst.get(target)
	if !d.mounted || len(d.snaps) != 2 {
		t.Errorf("mounted %v with snapshots %v, want mounted at the new one", d.mounted, r.dst.snapNames(target))
	}
	if m := member(t, res, "cache/appdata"); m.Mount != MountDone || m.Base == "" {
		t.Errorf("%+v, want an incremental run that is mounted", m)
	}
}

func TestACopyThatIsNotMountedIsMountedAgain(t *testing.T) {
	r := newRig(t)
	r.entry.Mount = true
	r.ok(r.run())
	target := base + "/cache/appdata"
	r.dst.get(target).mounted = false
	r.dst.resetCalls()
	r.ok(r.run())

	want := [][]string{
		{"zfs", "get", "-H", "-p", "-o", "property,value", mountState, target},
		{"zfs", "mount", target},
	}
	if got := mountCalls(r.dst, target); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("commands %q, want %q", got, want)
	}
}

func TestACopyThatWasWritableBecomesReadOnlyBeforeItMounts(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	target := base + "/cache/appdata"
	d := r.dst.get(target)
	d.props["readonly"], d.props["canmount"] = "off", "noauto"
	r.entry.Mount = true
	r.dst.resetCalls()
	r.ok(r.run())

	want := [][]string{
		{"zfs", "get", "-H", "-p", "-o", "property,value", mountState, target},
		{"zfs", "set", "readonly=on", target},
		{"zfs", "mount", target},
		{"zfs", "set", "canmount=on", target},
	}
	if got := mountCalls(r.dst, target); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("commands %q, want %q", got, want)
	}
}

func TestAnEncryptedCopyWithoutItsKeyIsLeftUnmounted(t *testing.T) {
	r := newRig(t)
	r.src.add("cache/appdata/vault", "filesystem", true)
	r.entry.Mount = true
	res := r.run()
	r.ok(res)

	target := base + "/cache/appdata/vault"
	want := [][]string{{"zfs", "get", "-H", "-p", "-o", "property,value", mountState, target}}
	if got := mountCalls(r.dst, target); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("commands %q, want only the read %q", got, want)
	}
	d := r.dst.get(target)
	if d.mounted || d.props["canmount"] != "noauto" {
		t.Errorf("mounted %v, canmount %q, want it left as received", d.mounted, d.props["canmount"])
	}
	if m := member(t, res, "cache/appdata/vault"); m.Mount != MountLocked {
		t.Errorf("mount %q, want %q", m.Mount, MountLocked)
	}
	if m := member(t, res, "cache/appdata/plex"); m.Mount != MountDone {
		t.Errorf("a plain sibling: mount %q, want %q", m.Mount, MountDone)
	}
}

func TestAnEncryptedCopyWhoseKeyIsLoadedMounts(t *testing.T) {
	r := newRig(t)
	r.src.add("cache/appdata/vault", "filesystem", true)
	r.ok(r.run())
	target := base + "/cache/appdata/vault"
	r.dst.get(target).keyLoaded = true
	r.entry.Mount = true
	res := r.run()
	r.ok(res)

	if !r.dst.get(target).mounted || member(t, res, "cache/appdata/vault").Mount != MountDone {
		t.Error("a copy with its key loaded is not mounted")
	}
}

func TestACopyWithoutAMountpointPathIsSkipped(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	target := base + "/cache/appdata"
	r.dst.get(target).props["mountpoint"] = "none"
	r.entry.Mount = true
	r.dst.resetCalls()
	res := r.run()
	r.ok(res)

	if got := mountCalls(r.dst, target); len(got) != 1 || got[0][1] != "get" {
		t.Errorf("commands %q, want only the read", got)
	}
	if m := member(t, res, "cache/appdata"); m.Mount != MountNoPath {
		t.Errorf("mount %q, want %q", m.Mount, MountNoPath)
	}
}

func TestAMountThatFailsDoesNotFailTheMember(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	target := base + "/cache/appdata"
	r.dst.get(target).mountFails = "cannot mount '" + target + "': mountpoint is not empty"
	r.entry.Mount = true
	res := r.run()
	r.ok(res)

	m := member(t, res, "cache/appdata")
	if m.Mount != MountFailed || m.Code != "" || m.Snapshot != res.Snapshot {
		t.Errorf("%+v, want the snapshot landed and the mount failed", m)
	}
	if got := r.dst.snapNames(target); len(got) != 2 {
		t.Errorf("snapshots on the target %v, want the new one kept", got)
	}
}

func TestAVolumeIsNeverMounted(t *testing.T) {
	r := newRig(t)
	r.src.add("cache/appdata/vm", "volume", false)
	r.entry.Mount = true
	res := r.run()
	r.ok(res)

	target := base + "/cache/appdata/vm"
	if got := mountCalls(r.dst, target); len(got) != 0 {
		t.Errorf("commands for a volume: %q", got)
	}
	if m := member(t, res, "cache/appdata/vm"); m.Mount != "" {
		t.Errorf("mount %q, want none", m.Mount)
	}
}
