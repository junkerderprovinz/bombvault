package api

import (
	"math"
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
	if f.GrowthBytesPerWeek != nil && *f.GrowthBytesPerWeek > 0 && f.FreeBytes != nil {
		weeks := math.Round(float64(*f.FreeBytes)/float64(*f.GrowthBytesPerWeek)*10) / 10
		f.WeeksToFull = &weeks
	}
	if f.GrowthBytesPerWeek == nil && f.FreeBytes == nil {
		return nil
	}
	return &f
}
