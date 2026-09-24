package api

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The capacity rule is only as good as its readings. A backup that failed
// because the disk is full is exactly the moment the sample has to be taken,
// and a repository nobody can measure has to say so rather than look healthy.

func TestVolumeSamplesOnEveryAttempt(t *testing.T) {
	t.Run("an attempt is sampled whether or not it succeeded", func(t *testing.T) {
		src, err := os.ReadFile("service.go")
		if err != nil {
			t.Fatal(err)
		}
		for _, domain := range []string{"containers", "vms", "flash", "files", "config"} {
			if !strings.Contains(string(src), `defer s.sampleVolumesFor(ctx, "`+domain+`")`) {
				t.Errorf("%s must sample its volumes on the way out of every attempt, not only a good one", domain)
			}
		}
	})

	t.Run("a volume is read once an hour", func(t *testing.T) {
		f := newCapacityFixture(t)
		f.svc.sampleVolumesFor(context.Background(), "containers")
		f.svc.sampleVolumesFor(context.Background(), "containers")
		if got := f.samples(t); len(got) != 1 {
			t.Fatalf("%d samples, want one an hour: %+v", len(got), got)
		}
		f.probe.free = 10 << 30
		f.now += 3601
		f.svc.sampleVolumesFor(context.Background(), "containers")
		got := f.samples(t)
		if len(got) != 2 || got[1].FreeBytes != 10<<30 {
			t.Fatalf("the next hour was not sampled: %+v", got)
		}
	})

	t.Run("repositories on one pool share a volume", func(t *testing.T) {
		f := newCapacityFixture(t)
		f.namedRepo(t, "cold", f.dir+"/cold")
		f.probe.volume = "pool:tank"
		f.svc.sampleVolumesFor(context.Background(), "containers")
		if got := f.samples(t); len(got) != 1 || got[0].Volume != "pool:tank" {
			t.Fatalf("two datasets of one pool were sampled apart: %+v", got)
		}
	})

	t.Run("an rclone repository is read every six hours", func(t *testing.T) {
		f := newCapacityFixture(t)
		f.namedRepo(t, "box", "rclone:box:bv")
		f.svc.sampleVolumesFor(context.Background(), "containers")
		if f.abouts != 1 {
			t.Fatalf("the remote was asked %d times, want once", f.abouts)
		}
		f.now += 3601
		f.svc.sampleVolumesFor(context.Background(), "containers")
		if f.abouts != 1 {
			t.Fatalf("the remote was asked again after an hour (%d calls)", f.abouts)
		}
		f.now += 6 * 3600
		f.svc.sampleVolumesFor(context.Background(), "containers")
		if f.abouts != 2 {
			t.Fatalf("the remote was not asked again after six hours (%d calls)", f.abouts)
		}
		remote := false
		for _, s := range f.samples(t) {
			if s.Source == "rclone" {
				remote = true
				if s.TotalBytes == nil || *s.TotalBytes != 8<<40 {
					t.Fatalf("the remote's size did not reach the sample: %+v", s)
				}
			}
		}
		if !remote {
			t.Fatal("no reading from the remote at all")
		}
	})

	t.Run("a remote that could not be reached waits like one that answered", func(t *testing.T) {
		f := newCapacityFixture(t)
		f.namedRepo(t, "box", "rclone:box:bv")
		f.aboutErr = errors.New("dial tcp: network is unreachable")

		f.svc.sampleVolumesFor(context.Background(), "containers")
		f.now += 3601
		f.svc.sampleVolumesFor(context.Background(), "containers")
		if f.abouts != 1 {
			t.Fatalf("the remote was asked again an hour after it failed (%d calls)", f.abouts)
		}
		f.now += 6 * 3600
		f.svc.sampleVolumesFor(context.Background(), "containers")
		if f.abouts != 2 {
			t.Fatalf("the remote was not tried again after six hours (%d calls)", f.abouts)
		}
	})

	t.Run("a backend that reports nothing is named, not skipped", func(t *testing.T) {
		f := newCapacityFixture(t)
		f.namedRepo(t, "cold", "s3:example.com/bucket/cold")
		f.svc.sampleVolumesFor(context.Background(), "containers")
		if err := f.e.rebuildCache(); err != nil {
			t.Fatal(err)
		}
		got := f.e.summary().UnmeasuredVolumes
		if len(got) != 1 || !strings.Contains(got[0], "Cold") {
			t.Fatalf("unmeasured volumes = %v, want the named repository", got)
		}
	})

	t.Run("a backend without an about call is named too", func(t *testing.T) {
		f := newCapacityFixture(t)
		f.namedRepo(t, "box", "rclone:box:bv")
		f.aboutErr = errAboutUnsupported
		f.svc.sampleVolumesFor(context.Background(), "containers")
		if err := f.e.rebuildCache(); err != nil {
			t.Fatal(err)
		}
		if got := f.e.summary().UnmeasuredVolumes; len(got) != 1 {
			t.Fatalf("unmeasured volumes = %v, want the remote that cannot answer", got)
		}
	})
}

func TestCapacityFindingsComeFromTheSamples(t *testing.T) {
	f := newCapacityFixture(t)
	total := int64(1) << 40
	for i := range 20 {
		if err := f.st.AddVolumeSample(store.VolumeSample{
			Volume: "dev:801", At: f.now - int64(20-i)*86400,
			FreeBytes: 200<<30 - int64(i)*10<<30, TotalBytes: &total,
			Domains: []string{"containers"}, Source: "statfs",
		}); err != nil {
			t.Fatal(err)
		}
	}
	f.e.MarkVolumeDirty()
	if err := f.e.passOnce(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}

	if len(f.volumeRows(t)) == 0 {
		t.Fatal("a volume losing ten gibibytes a day raised nothing")
	}
	if f.e.summary().Open.Critical+f.e.summary().Open.Warning == 0 {
		t.Fatalf("the summary does not carry the capacity finding: %+v", f.e.summary())
	}
}

// A repository that moved to another disk leaves its old volume without
// readings. The findings about it are about hardware nobody can look at any
// more, so the next pass ends them instead of leaving them standing.
func TestCapacityFindingsEndWhenTheVolumeStopsReporting(t *testing.T) {
	f := newCapacityFixture(t)
	total := int64(1) << 40
	for i := range 20 {
		if err := f.st.AddVolumeSample(store.VolumeSample{
			Volume: "dev:801", At: f.now - int64(20-i)*86400,
			FreeBytes: 200<<30 - int64(i)*10<<30, TotalBytes: &total,
			Domains: []string{"containers"}, Source: "statfs",
		}); err != nil {
			t.Fatal(err)
		}
	}
	f.e.MarkVolumeDirty()
	if err := f.e.passOnce(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if len(f.volumeRows(t)) == 0 {
		t.Fatal("a volume losing ten gibibytes a day raised nothing")
	}

	f.now += (capacityWindowDays + 1) * 86400
	f.e.MarkVolumeDirty()
	if err := f.e.passOnce(context.Background()); err != nil {
		t.Fatalf("second pass: %v", err)
	}

	for _, row := range f.volumeRows(t) {
		if row.RecoveredAt == 0 {
			t.Fatalf("a finding about a disk that no longer reports still stands: %+v", row)
		}
	}
}

// Readings older than the trend window are never read again, so housekeeping
// drops them instead of letting the table grow for as long as the box runs.
func TestOldVolumeReadingsArePruned(t *testing.T) {
	f := newCapacityFixture(t)
	total := int64(1) << 40
	for _, at := range []int64{f.now - (capacityWindowDays+12)*86400, f.now - 86400} {
		if err := f.st.AddVolumeSample(store.VolumeSample{
			Volume: "dev:801", At: at, FreeBytes: 200 << 30, TotalBytes: &total,
			Domains: []string{"containers"}, Source: "statfs",
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := f.e.passOnce(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}

	got := f.samples(t)
	if len(got) != 1 || got[0].At != f.now-86400 {
		t.Fatalf("readings after housekeeping = %+v, want the one inside the window", got)
	}
}

func TestZFSPoolIsOneVolume(t *testing.T) {
	const mounts = `25 30 0:24 / /mnt rw,relatime shared:9 - tmpfs tmpfs rw
41 25 0:41 / /mnt/tank rw,noatime shared:21 - zfs tank rw,xattr
42 41 0:42 / /mnt/tank/backups rw,noatime shared:23 - zfs tank/backups rw,xattr
43 25 8:1 / /mnt/disk1 rw,noatime shared:25 - xfs /dev/sda1 rw
`
	cases := []struct {
		path string
		pool string
		zfs  bool
	}{
		{path: "/mnt/tank/backups/containers", pool: "tank", zfs: true},
		{path: "/mnt/tank", pool: "tank", zfs: true},
		{path: "/mnt/disk1/backups", zfs: false},
		{path: "/mnt", zfs: false},
	}
	for _, tc := range cases {
		pool, ok := zfsPoolAt(strings.NewReader(mounts), tc.path)
		if ok != tc.zfs || pool != tc.pool {
			t.Fatalf("%s resolved to (%q, %v), want (%q, %v)", tc.path, pool, ok, tc.pool, tc.zfs)
		}
	}
}

// capacityFixture is a service whose free-space and remote probes are seams, so
// a test can decide what every repository reports.
type capacityFixture struct {
	svc      *Service
	st       *store.Repo
	e        *anomalyEngine
	dir      string
	now      int64
	probe    *fakeDiskStat
	abouts   int
	aboutErr error
}

type fakeDiskStat struct {
	volume      string
	free, total uint64
	err         error
}

func newCapacityFixture(t *testing.T) *capacityFixture {
	t.Helper()
	svc, st := statsTestService(t, &statsFakeEngine{})
	f := &capacityFixture{
		svc: svc, st: st, dir: svc.cfg.DataDir, now: testNow,
		probe: &fakeDiskStat{volume: "dev:801", free: 200 << 30, total: 1 << 40},
	}
	f.e = newAnomalyEngine(svc, func() time.Time { return time.Unix(f.now, 0) })
	f.e.debounce = 0
	svc.anomalies = f.e
	svc.diskStat = func(string) (diskStatResult, error) {
		if f.probe.err != nil {
			return diskStatResult{}, f.probe.err
		}
		return diskStatResult{Volume: f.probe.volume, Free: f.probe.free, Total: f.probe.total}, nil
	}
	svc.rcloneAbout = func(context.Context, string) (aboutResult, error) {
		f.abouts++
		if f.aboutErr != nil {
			return aboutResult{}, f.aboutErr
		}
		total := int64(8) << 40
		return aboutResult{Free: 2 << 40, Total: &total}, nil
	}
	return f
}

// namedRepo points a container at a named repository, which is how a domain
// comes to write to more than one volume.
func (f *capacityFixture) namedRepo(t *testing.T, id, loc string) {
	t.Helper()
	if _, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{
		ID: id, Name: strings.ToUpper(id[:1]) + id[1:], Repo: loc, Role: store.RoleRepo, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.UpsertTarget(store.Target{ContainerName: "Nexterm", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetTargetRepo("Nexterm", id); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(loc, "://") && !restic.IsRemoteRepo(loc) {
		if err := os.MkdirAll(loc, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(loc+"/config", []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *capacityFixture) volumeRows(t *testing.T) []store.Anomaly {
	t.Helper()
	rows, _, err := f.st.ListAnomalies(store.AnomalyFilter{ScopeKind: anomalyScopeVolume, ScopeID: "dev:801"})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func (f *capacityFixture) samples(t *testing.T) []store.VolumeSample {
	t.Helper()
	got, err := f.st.ListVolumeSamples(0)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestRcloneAboutReportsAnUnsupportedBackend(t *testing.T) {
	if !errors.Is(errAboutUnsupported, errAboutUnsupported) {
		t.Fatal("the sentinel does not compare")
	}
	if _, err := parseRcloneAbout([]byte(`{"total":100,"used":40,"free":60}`)); err != nil {
		t.Fatalf("a full answer did not parse: %v", err)
	}
	got, err := parseRcloneAbout([]byte(`{"free":60}`))
	if err != nil {
		t.Fatalf("an answer without a total did not parse: %v", err)
	}
	if got.Total != nil || got.Free != 60 {
		t.Fatalf("about = %+v, want the free space alone", got)
	}
	if _, err := parseRcloneAbout([]byte(`{"used":40}`)); !errors.Is(err, errAboutUnsupported) {
		t.Fatalf("a backend that reports no free space gave %v", err)
	}
}

// A repository location is operator input. It travels as a positional behind
// the end-of-flags marker, and one that names no remote never reaches rclone.
func TestRcloneAboutTakesTheRemoteAsAPositional(t *testing.T) {
	args := rcloneAboutArgs("box:bv")
	if len(args) < 2 || args[len(args)-2] != "--" || args[len(args)-1] != "box:bv" {
		t.Fatalf("args = %v, want the remote behind an end-of-flags marker", args)
	}

	got, err := rcloneRemoteOf(" rclone:box:bv ")
	if err != nil || got != "box:bv" {
		t.Fatalf("rcloneRemoteOf = %q, %v", got, err)
	}
	for _, loc := range []string{
		"rclone:--config=/mnt/user/appdata/bombvault/rclone.conf",
		"rclone:-vv",
		"rclone:box",
		"rclone:",
	} {
		if _, err := rcloneRemoteOf(loc); !errors.Is(err, errNotAnRcloneRemote) {
			t.Fatalf("%q was accepted as a remote: %v", loc, err)
		}
	}
}
