package api

import (
	"context"
	"math"
	"path/filepath"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// forecastWindow is how far back the growth rate looks: long enough to smooth
// over prunes and uneven backups, short enough to follow a real change within
// a month.
const forecastWindow = 4 * 7 * 24 * time.Hour

// forecastMinSpan is the shortest span between samples that a growth rate is
// computed from, so two samples taken minutes apart do not make a trend.
const forecastMinSpan = 24 * time.Hour

// StorageForecast is the growth trend and time-to-full projection that GET
// /api/stats returns next to the repo size samples. A nil field is unknown.
type StorageForecast struct {
	// GrowthBytesPerWeek is the raw repo size change per week over
	// forecastWindow, negative when the repo shrank.
	GrowthBytesPerWeek *int64 `json:"growthBytesPerWeek,omitempty"`
	// FreeBytes is the free space on the filesystem of a local repo.
	FreeBytes *int64 `json:"freeBytes,omitempty"`
	// WeeksToFull is FreeBytes / GrowthBytesPerWeek to one decimal, set only
	// while the repo grows.
	WeeksToFull *float64 `json:"weeksToFull,omitempty"`
}

// growthBytesPerWeek returns the raw repo size change per week between the
// oldest and newest sample inside forecastWindow. stats is sorted by At, as
// ListRepoStats returns it. Raw size is used because the deduplicated,
// compressed bytes are what fill the disk.
func growthBytesPerWeek(stats []store.RepoStat, now time.Time) (int64, bool) {
	cutoff := now.Add(-forecastWindow).Unix()
	var window []store.RepoStat
	for _, sample := range stats {
		if sample.At >= cutoff {
			window = append(window, sample)
		}
	}
	if len(window) < 2 {
		return 0, false
	}
	oldest, newest := window[0], window[len(window)-1]
	span := newest.At - oldest.At
	if span < int64(forecastMinSpan/time.Second) {
		return 0, false
	}
	weeks := float64(span) / (7 * 86400)
	return int64(float64(newest.RawSize-oldest.RawSize) / weeks), true
}

// diskFreeFn returns the free-space probe a test injected, or diskFreeBytes.
func (s *Service) diskFreeFn() func(string) (uint64, error) {
	if s.diskFree != nil {
		return s.diskFree
	}
	return diskFreeBytes
}

// diskStatFn returns the volume probe a test injected, or the platform statfs.
func (s *Service) diskStatFn() func(string) (diskStatResult, error) {
	if s.diskStat != nil {
		return s.diskStat
	}
	return diskStat
}

// rcloneAboutFn returns the remote-capacity probe a test injected, or the
// rclone binary reading this instance's own configuration.
func (s *Service) rcloneAboutFn() func(context.Context, string, []string) (aboutResult, error) {
	if s.rcloneAbout != nil {
		return s.rcloneAbout
	}
	config := filepath.Join(s.cfg.DataDir, "rclone.conf")
	return func(ctx context.Context, remote string, env []string) (aboutResult, error) {
		return rcloneAbout(ctx, config, remote, env)
	}
}

// StorageForecast builds the forecast for a domain and source from its size
// samples, or returns nil when nothing is known. Free space is measured only
// for a local repo.
func (s *Service) StorageForecast(domain, source string, stats []store.RepoStat) *StorageForecast {
	var f StorageForecast
	if growth, ok := growthBytesPerWeek(stats, time.Now()); ok {
		f.GrowthBytesPerWeek = &growth
	}
	if _, repo, err := s.domainRepoSource(domain, source); err == nil && !restic.IsRemoteRepo(repo) {
		if free, fErr := s.diskFreeFn()(repo); fErr == nil {
			freeBytes := int64(math.Min(float64(free), math.MaxInt64)) // clamp: JSON numbers are signed
			f.FreeBytes = &freeBytes
		}
	}
	if f.GrowthBytesPerWeek != nil && f.FreeBytes != nil {
		if weeks, ok := weeksToFull(*f.FreeBytes, *f.GrowthBytesPerWeek); ok {
			f.WeeksToFull = &weeks
		}
	}
	if f.GrowthBytesPerWeek == nil && f.FreeBytes == nil {
		return nil
	}
	return &f
}

// weeksToFull is how long the free space lasts at the given growth, to one
// decimal. It is known only while the repository grows.
func weeksToFull(freeBytes, growthPerWeek int64) (float64, bool) {
	if growthPerWeek <= 0 {
		return 0, false
	}
	return math.Round(float64(freeBytes)/float64(growthPerWeek)*10) / 10, true
}

// repoCapacity is the room on the disk or remote one repository sits on. A
// nil figure is unknown, and At is when it was read.
type repoCapacity struct {
	Name    string
	Primary bool
	Offsite bool
	Remote  bool
	At      *int64
	Free    *int64
	Used    *int64
	Total   *int64
}

// repoCapacities reads the room around every repository a domain writes to,
// its items' own ones first and then the off-site targets it copies to. A
// local disk is asked on the spot. A remote comes from the capacity rule's last
// stored reading, since asking it again is an API call against somebody else's
// service.
func (s *Service) repoCapacities(domain string) []repoCapacity {
	settings, repos, _, err := s.domainReposForOp(domain, "local")
	if err != nil {
		return nil
	}
	targets := s.offsiteReplicationTargets(domain, settings)
	var readings map[string]store.VolumeSample
	out := make([]repoCapacity, 0, len(repos)+len(targets))
	measure := func(c repoCapacity, loc string) repoCapacity {
		c.Remote = restic.IsRemoteRepo(loc)
		switch {
		case !c.Remote:
			// A folder with no repository in it yet may not be on the disk the
			// repository will be on.
			if localRepoMissing(loc) {
				break
			}
			if res, sErr := s.diskStatFn()(loc); sErr == nil {
				now := s.anomalies.nowUnix()
				free, used, total := clampToInt64(res.Free), clampToInt64(res.Used), clampToInt64(res.Total)
				c.At, c.Free, c.Used, c.Total = &now, &free, &used, &total
			}
		case isRcloneLocation(loc):
			if readings == nil {
				readings = s.newestVolumeReadings()
			}
			if v, ok := readings["remote:"+repoLocationKey(loc)]; ok {
				c.At, c.Free, c.Total = &v.At, &v.FreeBytes, v.TotalBytes
				if v.TotalBytes != nil {
					used := *v.TotalBytes - v.FreeBytes
					c.Used = &used
				}
			}
		}
		return c
	}
	for _, ref := range repos {
		out = append(out, measure(repoCapacity{Name: s.refName(ref), Primary: ref.Own}, ref.Loc))
	}
	for _, t := range targets {
		c := repoCapacity{Name: scrubSafeName(t.Name), Offsite: true}
		loc, rErr := s.resolveRepo(t.Repo)
		if rErr != nil {
			c.Remote = restic.IsRemoteRepo(t.Repo)
			out = append(out, c)
			continue
		}
		if c.Name == "" {
			c.Name = shortRepoName(loc)
		}
		out = append(out, measure(c, loc))
	}
	return out
}

// newestVolumeReadings is the last stored reading of every volume inside the
// capacity rule's window.
func (s *Service) newestVolumeReadings() map[string]store.VolumeSample {
	newest := map[string]store.VolumeSample{}
	samples, err := s.store.ListVolumeSamples(s.anomalies.nowUnix() - capacityWindowDays*86400)
	if err != nil {
		return newest
	}
	for _, v := range samples {
		newest[v.Volume] = v // oldest first, so the last one stays
	}
	return newest
}
