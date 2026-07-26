package api

import (
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestGrowthBytesPerWeek pins the growth computation over the sample history:
// steady growth, shrink, tiny history guards, and the 4-week window cut.
func TestGrowthBytesPerWeek(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	const day = int64(86400)
	at := func(daysAgo int64) int64 { return now.Unix() - daysAgo*day }
	sample := func(daysAgo, raw int64) store.RepoStat {
		return store.RepoStat{Domain: "containers", Source: "local", At: at(daysAgo), RawSize: raw}
	}

	cases := []struct {
		name   string
		stats  []store.RepoStat
		want   int64
		wantOK bool
	}{
		{
			// 7 GiB over exactly two weeks → 3.5 GiB/week.
			name:   "steady growth",
			stats:  []store.RepoStat{sample(14, 10<<30), sample(7, 13<<30+1<<29), sample(0, 17<<30)},
			want:   (17<<30 - 10<<30) / 2,
			wantOK: true,
		},
		{
			// A prune shrank the repo: 2 GiB down over one week → negative rate.
			name:   "shrink is a negative rate",
			stats:  []store.RepoStat{sample(7, 10<<30), sample(0, 8<<30)},
			want:   -(2 << 30),
			wantOK: true,
		},
		{
			name:   "single sample claims nothing",
			stats:  []store.RepoStat{sample(0, 10<<30)},
			wantOK: false,
		},
		{
			name:   "no samples claim nothing",
			stats:  nil,
			wantOK: false,
		},
		{
			// Two samples 1 hour apart: below forecastMinSpan → no rate.
			name: "tiny span claims nothing",
			stats: []store.RepoStat{
				{At: now.Unix() - 3600, RawSize: 10 << 30},
				{At: now.Unix(), RawSize: 11 << 30},
			},
			wantOK: false,
		},
		{
			// Samples older than the 4-week window are ignored: only the two inside
			// count (1 GiB over one week), NOT the huge jump from the stale one.
			name:   "window cut ignores stale samples",
			stats:  []store.RepoStat{sample(60, 1<<30), sample(7, 10<<30), sample(0, 11<<30)},
			want:   1 << 30,
			wantOK: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := growthBytesPerWeek(c.stats, now)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if ok && got != c.want {
				t.Fatalf("growth = %d, want %d", got, c.want)
			}
		})
	}
}

// forecastTestService builds a Service over a real (temp) store whose
// containers repo resolves to a LOCAL path, with an injected free-space probe.
func forecastTestService(t *testing.T, diskFree func(string) (uint64, error)) *Service {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return &Service{
		cfg:      config.Config{HostMountRoot: t.TempDir()},
		store:    store.New(db),
		diskFree: diskFree,
	}
}

// TestStorageForecastCombinesGrowthAndFreeSpace pins the full projection: with
// a known growth rate and a mocked statfs, weeksToFull = free / rate (one
// decimal), and the probe receives the RESOLVED local repo path.
func TestStorageForecastCombinesGrowthAndFreeSpace(t *testing.T) {
	var probed string
	free := uint64(10 << 30) // 10 GiB free
	svc := forecastTestService(t, func(path string) (uint64, error) {
		probed = path
		return free, nil
	})

	now := time.Now().Unix()
	const day = int64(86400)
	stats := []store.RepoStat{
		{At: now - 7*day, RawSize: 10 << 30},
		{At: now, RawSize: 11 << 30}, // +1 GiB in one week
	}

	f := svc.StorageForecast("containers", "local", stats)
	if f == nil {
		t.Fatal("expected a forecast, got nil")
	}
	if f.GrowthBytesPerWeek == nil || *f.GrowthBytesPerWeek != 1<<30 {
		t.Fatalf("growth = %v, want 1 GiB/week", f.GrowthBytesPerWeek)
	}
	if f.FreeBytes == nil || *f.FreeBytes != int64(free) {
		t.Fatalf("freeBytes = %v, want %d", f.FreeBytes, free)
	}
	if f.WeeksToFull == nil || *f.WeeksToFull != 10.0 {
		t.Fatalf("weeksToFull = %v, want 10.0 (10 GiB free / 1 GiB per week)", f.WeeksToFull)
	}
	if probed == "" {
		t.Fatal("the free-space probe must be asked about the resolved local repo path")
	}
}

// TestStorageForecastOmissions pins the absent-field contract: no/flat/negative
// growth never yields weeksToFull; a failing probe omits freeBytes; and with
// NOTHING known the forecast is nil.
func TestStorageForecastOmissions(t *testing.T) {
	now := time.Now().Unix()
	const day = int64(86400)

	// Shrinking repo + working probe: growth + free present, weeksToFull absent.
	svc := forecastTestService(t, func(string) (uint64, error) { return 5 << 30, nil })
	shrink := []store.RepoStat{
		{At: now - 7*day, RawSize: 10 << 30},
		{At: now, RawSize: 9 << 30},
	}
	f := svc.StorageForecast("containers", "local", shrink)
	if f == nil || f.GrowthBytesPerWeek == nil || *f.GrowthBytesPerWeek >= 0 {
		t.Fatalf("shrink must report a negative growth, got %+v", f)
	}
	if f.WeeksToFull != nil {
		t.Fatalf("a shrinking repo never fills the disk, got weeksToFull=%v", *f.WeeksToFull)
	}

	// Failing probe + single sample: nothing is known → nil forecast.
	svc = forecastTestService(t, func(string) (uint64, error) { return 0, errors.New("statfs unsupported") })
	f = svc.StorageForecast("containers", "local", []store.RepoStat{{At: now, RawSize: 1 << 30}})
	if f != nil {
		t.Fatalf("with no growth and no free space the forecast must be nil, got %+v", f)
	}

	// Failing probe but known growth: growth present, free + weeksToFull absent.
	f = svc.StorageForecast("containers", "local", []store.RepoStat{
		{At: now - 7*day, RawSize: 1 << 30},
		{At: now, RawSize: 2 << 30},
	})
	if f == nil || f.GrowthBytesPerWeek == nil {
		t.Fatalf("growth must survive a failed free-space probe, got %+v", f)
	}
	if f.FreeBytes != nil || f.WeeksToFull != nil {
		t.Fatalf("a failed probe must omit freeBytes and weeksToFull, got %+v", f)
	}
}

// TestStorageForecastRemoteRepoSkipsProbe pins the remote path: an off-site
// rclone:/s3: repo has no local filesystem, so the probe is never called and
// freeBytes stays absent (growth from the off-site samples still works).
func TestStorageForecastRemoteRepoSkipsProbe(t *testing.T) {
	probes := 0
	svc := forecastTestService(t, func(string) (uint64, error) { probes++; return 1 << 30, nil })
	settings, err := svc.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersOffsite = "rclone:remote:bucket"
	if err := svc.store.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	now := time.Now().Unix()
	const day = int64(86400)
	f := svc.StorageForecast("containers", "offsite", []store.RepoStat{
		{At: now - 7*day, RawSize: 1 << 30},
		{At: now, RawSize: 2 << 30},
	})
	if probes != 0 {
		t.Fatalf("a remote repo must never be statfs'd, got %d probes", probes)
	}
	if f == nil || f.GrowthBytesPerWeek == nil || f.FreeBytes != nil {
		t.Fatalf("remote forecast must carry growth only, got %+v", f)
	}
}

// TestDiskFreeBytesPlatform smoke-tests the real statfs implementation where it
// exists (Linux — the shipped container); elsewhere the stub's error path is
// pinned instead, which is exactly what StorageForecast handles by omission.
func TestDiskFreeBytesPlatform(t *testing.T) {
	free, err := diskFreeBytes(t.TempDir())
	if err != nil {
		t.Skipf("free-space probe unsupported on this platform (forecast omits freeBytes): %v", err)
	}
	if free == 0 {
		t.Fatal("a writable temp dir must report free space")
	}
}
