package api

import (
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

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
			// 7 GiB over two weeks.
			name:   "steady growth",
			stats:  []store.RepoStat{sample(14, 10<<30), sample(7, 13<<30+1<<29), sample(0, 17<<30)},
			want:   (17<<30 - 10<<30) / 2,
			wantOK: true,
		},
		{
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
			name: "tiny span claims nothing",
			stats: []store.RepoStat{
				{At: now.Unix() - 3600, RawSize: 10 << 30},
				{At: now.Unix(), RawSize: 11 << 30},
			},
			wantOK: false,
		},
		{
			// Only the two samples inside the window count, not the jump from
			// the stale one.
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

// forecastTestService returns a Service whose containers repo is local and
// whose free-space probe is diskFree.
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

func TestStorageForecastCombinesGrowthAndFreeSpace(t *testing.T) {
	var probed string
	free := uint64(10 << 30)
	svc := forecastTestService(t, func(path string) (uint64, error) {
		probed = path
		return free, nil
	})

	now := time.Now().Unix()
	const day = int64(86400)
	stats := []store.RepoStat{
		{At: now - 7*day, RawSize: 10 << 30},
		{At: now, RawSize: 11 << 30},
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

func TestStorageForecastOmissions(t *testing.T) {
	now := time.Now().Unix()
	const day = int64(86400)

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

	svc = forecastTestService(t, func(string) (uint64, error) { return 0, errors.New("statfs unsupported") })
	f = svc.StorageForecast("containers", "local", []store.RepoStat{{At: now, RawSize: 1 << 30}})
	if f != nil {
		t.Fatalf("with no growth and no free space the forecast must be nil, got %+v", f)
	}

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

func TestDiskFreeBytesPlatform(t *testing.T) {
	free, err := diskFreeBytes(t.TempDir())
	if err != nil {
		t.Skipf("free-space probe unsupported on this platform (forecast omits freeBytes): %v", err)
	}
	if free == 0 {
		t.Fatal("a writable temp dir must report free space")
	}
}
