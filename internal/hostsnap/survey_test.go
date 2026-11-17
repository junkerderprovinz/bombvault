package hostsnap

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// fakeShell answers each script by what it contains and keeps every argv.
type fakeShell struct {
	answers []answer
	calls   [][]string
}

type answer struct {
	contains       string
	stdout, stderr string
	err            error
}

func (f *fakeShell) RunCapture(_ context.Context, args ...string) (string, string, error) {
	f.calls = append(f.calls, args)
	joined := strings.Join(args, "\x00")
	for _, a := range f.answers {
		if strings.Contains(joined, a.contains) {
			return a.stdout, a.stderr, a.err
		}
	}
	return "", "", nil
}

type exitErr int

func (e exitErr) Error() string { return "exit" }
func (e exitErr) ExitCode() int { return int(e) }

type fakeVMs struct {
	tables  map[string]string
	created []string
	deleted []string
	err     error
}

func (f *fakeVMs) DomainNames(context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []string
	for n := range f.tables {
		out = append(out, n)
	}
	return out, nil
}

func (f *fakeVMs) SnapshotTable(_ context.Context, d string) (string, error) {
	return f.tables[d], nil
}

func (f *fakeVMs) SnapshotCreate(_ context.Context, d, n string) error {
	f.created = append(f.created, d+"@"+n)
	return f.err
}

func (f *fakeVMs) SnapshotDelete(_ context.Context, d, n string) error {
	f.deleted = append(f.deleted, d+"@"+n)
	return f.err
}

const surveyOut = "Z\t+0100\n" +
	"T\tzfs\tok\n" +
	"D\tzfs\ttank\tsda\tawake\n" +
	"P\tzfs\ttank\t0\n" +
	"D\tzfs\tdisk1\tmd1\tasleep\n" +
	"P\tzfs\tdisk1\t1\n" +
	"F\ttank\tfilesystem\t/mnt/tank\t1000\t10\n" +
	"F\ttank/apps\tfilesystem\t/mnt/tank/apps\t800\t5\n" +
	"F\ttank/docker/0123abcd\tfilesystem\tlegacy\t50\t0\n" +
	"S\ttank/apps@bombvault-auto-1a2b3c4d-20261101000000\t1761955200\t100\t700\t0\t-\n" +
	"S\ttank/apps@before-update\t1761868800\t200\t650\t1\ttank/apps-clone\n" +
	"S\ttank/apps@bombvault-replica-20261101000000\t1761955300\t5\t700\t1\t-\n" +
	"S\ttank/docker/0123abcd@init\t1761868800\t0\t50\t0\t-\n" +
	"T\tbtrfs\tok\n" +
	"BF\t/mnt/cache\t/dev/sdc1\t1\n" +
	"BD\t/mnt/cache\tsdc\tawake\n" +
	"BP\t/mnt/cache\t0\n" +
	"BT\t/mnt/cache\t\tUUID:             \t11111111-top\n" +
	"BL\t/mnt/cache\tID 256 gen 10 parent 5 top level 5 parent_uuid - uuid aaaa path appdata\n" +
	"BL\t/mnt/cache\tID 300 gen 12 parent 5 top level 5 parent_uuid aaaa uuid bbbb path .bombvault-snapshots/appdata/before update\n" +
	"BS\t/mnt/cache\tID 300 gen 12 cgen 12 parent 5 top level 5 otime 2026-11-01 12:00:00 parent_uuid aaaa uuid bbbb path .bombvault-snapshots/appdata/before update\n" +
	"BR\t/mnt/cache\tID 300 gen 12 top level 5 uuid bbbb path .bombvault-snapshots/appdata/before update\n" +
	"BQ\t/mnt/cache\tqgroupid         rfer         excl\n" +
	"BQ\t/mnt/cache\t0/300          4096         1024\n"

func TestTheSurveyListsWhatEachKindHolds(t *testing.T) {
	db := base64.StdEncoding.EncodeToString([]byte(`{"S1":{"name":"S1","creationtime":"1761868800"}}`))
	sh := &fakeShell{answers: []answer{
		{contains: "bombvault-snapshots\x000\x00full", stdout: surveyOut + "U\twin11\t" + db + "\n"},
		{contains: "written@", stdout: "W\ttank/apps\twritten@before-update\t300\nW\ttank/apps\twritten@bombvault-auto-1a2b3c4d-20261101000000\t40\nH\ttank/apps@before-update\tbombvault-keep\tThu Nov  1 12:00 2026\n"},
	}}
	vms := &fakeVMs{tables: map[string]string{"win11": " Name           Creation Time               State\n" +
		"----------------------------------------------------------\n" +
		" clean install  2026-11-02 08:30:00 +0100   shutoff\n"}}
	s := New(sh, vms).Survey(context.Background(), false)

	for _, k := range []Kind{KindZFS, KindBtrfs, KindVM} {
		if s.Kinds[k].Code != "ok" {
			t.Errorf("kind %s = %+v, want ok", k, s.Kinds[k])
		}
	}
	if s.HiddenSystem != 1 {
		t.Errorf("HiddenSystem = %d, want the docker layer's one", s.HiddenSystem)
	}
	var sleeping *Pool
	for i := range s.Pools {
		if s.Pools[i].Name == "disk1" {
			sleeping = &s.Pools[i]
		}
	}
	if sleeping == nil || sleeping.State != StateAsleep || sleeping.Read {
		t.Errorf("disk1 = %+v, want asleep and not read", sleeping)
	}

	by := map[string]Snapshot{}
	for _, sn := range s.Snapshots {
		by[string(sn.Kind)+":"+sn.Source+"@"+sn.Label] = sn
	}
	manual := by["zfs:tank/apps@before-update"]
	if manual.NewSince != 300 || manual.Used != 200 || len(manual.Holds) != 1 || manual.Holds[0] != HoldTag || manual.Clones[0] != "tank/apps-clone" {
		t.Errorf("before-update = %+v", manual)
	}
	if !manual.Caps.Delete || !manual.Caps.Hold {
		t.Errorf("before-update caps = %+v, want every action", manual.Caps)
	}
	if plan := by["zfs:tank/apps@bombvault-auto-1a2b3c4d-20261101000000"]; plan.Owner != "plan" || plan.PlanID != "1a2b3c4d" {
		t.Errorf("plan snapshot = %+v", plan)
	}
	if replica := by["zfs:tank/apps@bombvault-replica-20261101000000"]; replica.Caps != (Caps{}) {
		t.Errorf("a replica's base can be changed here: %+v", replica.Caps)
	}
	btr := by["btrfs:appdata@before update"]
	if btr.Name != ".bombvault-snapshots/appdata/before update" || btr.FS != "/mnt/cache" || btr.Used != 1024 || !btr.ReadOnly || btr.Caps.Hold {
		t.Errorf("btrfs snapshot = %+v", btr)
	}
	// 2026-11-01 12:00 at +0100.
	if btr.CreatedAt != 1793530800 {
		t.Errorf("btrfs created at %d, want the host's local time read with its offset", btr.CreatedAt)
	}
	if vm := by["vm:win11@clean install"]; vm.Caps.Delete != true || vm.Caps.Rename {
		t.Errorf("vm snapshot = %+v", vm)
	}
	if u := by["vm:win11@S1"]; u.ManagedBy != "unraid" || u.Caps != (Caps{}) || u.CreatedAt != 1761868800 {
		t.Errorf("Unraid's snapshot = %+v", u)
	}
	for _, src := range s.Sources {
		if strings.Contains(src.Name, "docker") || strings.HasPrefix(src.Name, BtrfsFolder) {
			t.Errorf("source %q is listed", src.Name)
		}
	}
}

func TestAReadOfSleepingPoolsSaysWhetherTheyWoke(t *testing.T) {
	out := "T\tzfs\tok\nD\tzfs\tdisk1\tsdb\tasleep\nP\tzfs\tdisk1\t1\nA\tzfs\tdisk1\tsdb\tawake\nT\tbtrfs\tmissing\n"
	sh := &fakeShell{answers: []answer{{contains: "bombvault-snapshots\x001\x00full", stdout: out}}}
	s := New(sh, nil).Survey(context.Background(), true)
	if len(s.Pools) != 1 || !s.Pools[0].Read || !s.Pools[0].Woke {
		t.Fatalf("pools = %+v, want disk1 read and woken", s.Pools)
	}
	if s.Kinds[KindBtrfs].Code != "missing" || s.Kinds[KindVM].Code != "missing" {
		t.Errorf("kinds = %+v", s.Kinds)
	}
}

func TestAHostOutOfReachIsAStateNotAFailure(t *testing.T) {
	sh := &fakeShell{answers: []answer{{contains: "full", stderr: "Permission denied (publickey).", err: exitErr(255)}}}
	vms := &fakeVMs{err: errors.New("failed to connect to the hypervisor")}
	s := New(sh, vms).Survey(context.Background(), false)
	if s.Kinds[KindZFS].Code != "ssh-auth" || s.Kinds[KindBtrfs].Code != "ssh-auth" {
		t.Errorf("kinds = %+v, want ssh-auth", s.Kinds)
	}
	if s.Kinds[KindVM].Code != "vm-unreachable" {
		t.Errorf("vm = %+v", s.Kinds[KindVM])
	}
	if s.Snapshots == nil || s.Sources == nil || s.Pools == nil {
		t.Error("a list is null, want it empty")
	}
}

func TestMountEscapesRoundTrip(t *testing.T) {
	for _, s := range []string{"tank/my share@a", `back\slash`, "plain"} {
		if got := mountUnescape(mountEscape(s)); got != s {
			t.Errorf("round trip of %q = %q", s, got)
		}
	}
	if mountEscape("tank/my share@a") != `tank/my\040share@a` {
		t.Errorf("escape = %q", mountEscape("tank/my share@a"))
	}
}
