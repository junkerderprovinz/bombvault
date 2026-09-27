package hostload

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const diskstats = `   7       0 loop0 1618539 0 4793702 100 0 0 0 0 0 4793702 0
 259       2 nvme0n1 21644405 0 1 1 0 0 0 0 0 31298112 0
 259       3 nvme0n1p1 21644320 0 1 1 0 0 0 0 0 32007417 0
   8      80 sdf 12832440 0 1 1 0 0 0 0 0 22428663 0
   8      81 sdf1 74 0 1 1 0 0 0 0 0 139 0
   8      64 sde 973399 0 1 1 0 0 0 0 0 7247460 0
   9       1 md1p1 0 0 0 0 0 0 0 0 0 0 0
`

const mdstat = `diskNumber.0=0
diskName.0=
rdevName.0=sde
diskNumber.1=1
diskName.1=md1p1
rdevName.1=sdf
diskName.5=
rdevName.5=
`

const mountinfo = `22 1 0:21 / / rw - overlay overlay rw
40 22 0:30 / /host/user rw - tmpfs tmpfs rw
50 40 9:1 / /host/user/disk1 rw,noatime shared:6 - xfs /dev/md1p1 rw
54 40 259:3 / /host/user/cache rw,noatime shared:10 - xfs /dev/nvme0n1p1 rw
57 40 0:44 / /host/user/user rw,noatime shared:13 - fuse.shfs shfs rw
`

func TestParseStatCountsIdleAndIowaitAsNotBusy(t *testing.T) {
	busy, total := parseStat([]byte("cpu  100 0 50 800 50 0 0 0 0 0\ncpu0 1 2 3 4\n"))
	if busy != 150 || total != 1000 {
		t.Fatalf("busy %d total %d", busy, total)
	}
}

func TestParseDiskstatsKeepsWholeDisksOnly(t *testing.T) {
	got := parseDiskstats([]byte(diskstats))
	for _, name := range []string{"loop0", "nvme0n1p1", "sdf1"} {
		if _, ok := got[name]; ok {
			t.Errorf("%s is kept", name)
		}
	}
	if got["sdf"] != 22428663 || got["nvme0n1"] != 31298112 {
		t.Fatalf("got %v", got)
	}
}

func TestParseNetDevSumsWhatWasSentButLoopback(t *testing.T) {
	dev := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 999 1 0 0 0 0 0 0 999 1 0 0 0 0 0 0
  eth0: 100 1 0 0 0 0 0 0 5000 1 0 0 0 0 0 0
  eth1: 100 1 0 0 0 0 0 0 700 1 0 0 0 0 0 0
`
	if got := parseNetDev([]byte(dev)); got != 5700 {
		t.Fatalf("tx = %d", got)
	}
}

func TestSamplerAveragesTheIntervalsOfARun(t *testing.T) {
	s := NewSampler(t.TempDir(), time.Hour)
	s.windows["run"] = &window{disks: map[string]float64{}, startedAt: time.Now()}
	a := Counters{CPUBusy: 0, CPUTotal: 100, DiskTicks: map[string]uint64{"sdb": 0, "sdc": 0}, TxBytes: 0, HasNet: true}
	b := Counters{CPUBusy: 50, CPUTotal: 200, DiskTicks: map[string]uint64{"sdb": 5000, "sdc": 1000}, TxBytes: 10_000, HasNet: true}
	c := Counters{CPUBusy: 60, CPUTotal: 300, DiskTicks: map[string]uint64{"sdb": 9000, "sdc": 1000}, TxBytes: 20_000, HasNet: true}
	s.addInterval(a, b, 5*time.Second)
	s.addInterval(b, c, 5*time.Second)
	sum, ok := s.End("run")
	if !ok {
		t.Fatal("no summary")
	}
	if sum.Samples != 2 || *sum.CPU != 0.3 || *sum.UploadBps != 2000 {
		t.Fatalf("summary %+v cpu %v up %v", sum, *sum.CPU, *sum.UploadBps)
	}
	if sum.Disks[0].Name != "sdb" || sum.Disks[0].Busy != 0.9 || sum.Disks[1].Busy != 0.1 {
		t.Fatalf("disks %+v", sum.Disks)
	}
}

func TestSamplerSkipsACounterThatWentBackwards(t *testing.T) {
	s := NewSampler(t.TempDir(), time.Hour)
	s.windows["run"] = &window{disks: map[string]float64{}, startedAt: time.Now()}
	s.addInterval(Counters{DiskTicks: map[string]uint64{"sdb": 9000}}, Counters{DiskTicks: map[string]uint64{"sdb": 10}}, 5*time.Second)
	sum, _ := s.End("run")
	if len(sum.Disks) != 0 || sum.CPU != nil || sum.UploadBps != nil {
		t.Fatalf("summary %+v", sum)
	}
}

func TestARunShorterThanOneSampleHasNoSummary(t *testing.T) {
	s := NewSampler(t.TempDir(), time.Hour)
	s.Begin("run")
	if _, ok := s.End("run"); ok {
		t.Fatal("a run without a sample got a summary")
	}
}

func TestSamplerReadsTheCountersWhileARunIsOpen(t *testing.T) {
	proc := t.TempDir()
	write(t, proc, "stat", "cpu 10 0 10 80 0 0 0 0\n")
	s := NewSampler(proc, 10*time.Millisecond)
	s.Begin("run")
	write(t, proc, "stat", "cpu 100 0 10 90 0 0 0 0\n")
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		n := s.windows["run"].samples
		s.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no sample was taken")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, ok := s.End("run"); !ok {
		t.Fatal("no summary")
	}
}

func TestCauseOfNamesOneSaturatedDisk(t *testing.T) {
	cpu := 0.4
	s := Summary{CPU: &cpu, Disks: []DiskLoad{{Name: "sdf", Label: "disk1", Role: RoleTarget, Busy: 0.98}, {Name: "sde", Busy: 0.5}}}
	c := CauseOf(s, 0)
	if c == nil || c.Kind != CauseDisk || c.Name != "disk1" || c.Role != RoleTarget {
		t.Fatalf("cause %+v", c)
	}
}

func TestCauseOfSaysNothingWhenTwoThingsWereFull(t *testing.T) {
	cpu := 0.95
	s := Summary{CPU: &cpu, Disks: []DiskLoad{{Name: "sdf", Busy: 0.98}}}
	if c := CauseOf(s, 0); c != nil {
		t.Fatalf("cause %+v", c)
	}
	if c := CauseOf(Summary{Disks: []DiskLoad{{Name: "sdf", Busy: 0.5}}}, 0); c != nil {
		t.Fatalf("cause without anything full %+v", c)
	}
}

func TestCauseOfSeesTheUploadLimit(t *testing.T) {
	up := 9.5e6
	c := CauseOf(Summary{UploadBps: &up}, 10e6)
	if c == nil || c.Kind != CauseUpload || c.Share != 0.95 {
		t.Fatalf("cause %+v", c)
	}
	if c := CauseOf(Summary{UploadBps: &up}, 0); c != nil {
		t.Fatalf("an upload without a limit is a cause: %+v", c)
	}
}

func testResolver(t *testing.T) *Resolver {
	t.Helper()
	proc := t.TempDir()
	write(t, proc, "diskstats", diskstats)
	write(t, proc, "mdstat", mdstat)
	write(t, proc, "self/mountinfo", mountinfo)
	return NewResolver(proc, "/host/user")
}

func TestResolverFollowsTheArrayDeviceToItsDisk(t *testing.T) {
	r := testResolver(t)
	d, ok := r.DeviceOf("/host/user/disk1/backups/containers")
	if !ok || d != "sdf" {
		t.Fatalf("device %q %v", d, ok)
	}
	if got := r.Label("sdf"); got != "disk1" {
		t.Fatalf("label %q", got)
	}
	if got := r.Label("sde"); got != "parity" {
		t.Fatalf("parity label %q", got)
	}
	if d, _ := r.DeviceOf("/host/user/cache/appdata/plex"); d != "nvme0n1" {
		t.Fatalf("cache device %q", d)
	}
	if got := r.Label("nvme0n1"); got != "cache" {
		t.Fatalf("cache label %q", got)
	}
}

func TestResolverFindsTheDiskBehindAUserSharePath(t *testing.T) {
	r := testResolver(t)
	r.exists = func(p string) bool { return p == "/host/user/cache/appdata/plex" }
	if d, ok := r.DeviceOf("/host/user/user/appdata/plex"); !ok || d != "nvme0n1" {
		t.Fatalf("device %q %v", d, ok)
	}
	r.exists = func(p string) bool { return true }
	if d, ok := r.DeviceOf("/host/user/user/appdata/plex"); ok {
		t.Fatalf("a share on two disks resolved to %q", d)
	}
}

func TestAnnotateMarksSourceAndTarget(t *testing.T) {
	r := testResolver(t)
	r.exists = func(p string) bool { return p == "/host/user/cache/appdata/plex" }
	s := Summary{Disks: []DiskLoad{{Name: "sdf"}, {Name: "nvme0n1"}, {Name: "sde"}}}
	r.Annotate(&s, []string{"/host/user/user/appdata/plex"}, "/host/user/disk1/backups")
	if s.Disks[0].Role != RoleTarget || s.Disks[1].Role != RoleSource || s.Disks[2].Role != "" {
		t.Fatalf("roles %+v", s.Disks)
	}
	if s.Disks[2].Label != "parity" {
		t.Fatalf("labels %+v", s.Disks)
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
