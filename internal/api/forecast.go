package api

import (
	"math"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The storage forecast rides GET /api/stats (handleStats): alongside the raw
// repo-size samples the Storage card already plots, the response carries how
// fast the repo grows and how long the volume it lives on will last. Backend +
// API only — frontend consumption of the new fields is a separate task.

// forecastWindow is how far back the growth computation looks. Four weeks of
// samples smooth over retention prunes and irregular backup sizes while still
// reacting to a genuine trend change within a month.
const forecastWindow = 4 * 7 * 24 * time.Hour

// forecastMinSpan guards against a "trend" read off two samples taken minutes
// apart (e.g. right after enabling the feature): with less than a day between
// the oldest and newest sample in the window, no rate is claimed.
const forecastMinSpan = 24 * time.Hour

// StorageForecast extends the /api/stats payload with the repo's growth trend
// and a time-to-full projection for the volume the LOCAL repo lives on.
// Field semantics (all optional — an absent field means "unknown"):
//
//   - growthBytesPerWeek: (newest − oldest raw repo size) / weeks across the
//     samples of the last forecastWindow; negative when the repo shrank.
//     Absent with fewer than two samples in the window or a span below
//     forecastMinSpan.
//   - freeBytes: free space on the filesystem holding the repo (statfs).
//     Absent for remote repos and when the probe fails (e.g. repo path gone).
//   - weeksToFull: freeBytes / growthBytesPerWeek, rounded to one decimal.
//     Absent unless growth is known AND positive AND freeBytes is known —
//     a shrinking or flat repo never "fills" the disk.
//
// The whole object is null in the response when neither growth nor free space
// could be determined.
type StorageForecast struct {
	GrowthBytesPerWeek *int64   `json:"growthBytesPerWeek,omitempty"`
	FreeBytes          *int64   `json:"freeBytes,omitempty"`
	WeeksToFull        *float64 `json:"weeksToFull,omitempty"`
}

// growthBytesPerWeek computes the repo's growth rate from its size-sample
// history (ascending by At, as ListRepoStats returns it): the simple
// (newest − oldest) / weeks slope over the samples inside the trailing
// forecastWindow. RawSize is used — the physical (deduplicated + compressed)
// bytes are what fill the volume. Pure, so the table test needs no store.
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

// diskFreeFn returns the free-space probe: the injected test seam when set,
// else the platform statfs implementation (diskFreeBytes).
func (s *Service) diskFreeFn() func(string) (uint64, error) {
	if s.diskFree != nil {
		return s.diskFree
	}
	return diskFreeBytes
}

// StorageForecast builds the forecast for a domain + source from the (already
// fetched) sample history. Best-effort by design: every unknown simply stays
// absent, and a nil return means nothing could be determined at all. The
// free-space probe runs whenever the configured repo resolves to a LOCAL path
// (which covers the shared backup mount all local domain repos live on); a
// remote repo (rclone:/s3:/…) has no local filesystem to measure.
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
	if f.GrowthBytesPerWeek != nil && *f.GrowthBytesPerWeek > 0 && f.FreeBytes != nil {
		weeks := math.Round(float64(*f.FreeBytes)/float64(*f.GrowthBytesPerWeek)*10) / 10
		f.WeeksToFull = &weeks
	}
	if f.GrowthBytesPerWeek == nil && f.FreeBytes == nil {
		return nil
	}
	return &f
}
