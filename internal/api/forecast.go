package api

import (
	"bytes"
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
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
	// CapacitySource names what measured FreeBytes: statfs, smb or nfs for a
	// path on this box, rclone or sftp for a remote repository.
	CapacitySource string `json:"capacitySource,omitempty"`
	// CapacityUnsupported marks a backend that cannot report its free space
	// at all, such as S3, B2 or a REST server, so only the growth is known.
	CapacityUnsupported bool `json:"capacityUnsupported,omitempty"`
}

// remoteReadingMaxAge is how old the last reading of a remote volume may be
// and still stand for its free space today. Readings are taken at most every
// six hours and only around backups.
const remoteReadingMaxAge = 7 * 24 * time.Hour

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
func (s *Service) rcloneAboutFn() func(context.Context, string) (aboutResult, error) {
	if s.rcloneAbout != nil {
		return s.rcloneAbout
	}
	config := filepath.Join(s.cfg.DataDir, "rclone.conf")
	return func(ctx context.Context, remote string) (aboutResult, error) {
		return rcloneAbout(ctx, config, remote)
	}
}

// sftpAboutFn returns the SFTP probe a test injected, or the ssh-backed one.
func (s *Service) sftpAboutFn() func(context.Context, string) (aboutResult, error) {
	if s.sftpAbout != nil {
		return s.sftpAbout
	}
	return sftpCapacity
}

// remoteVolumeSource names the probe that can measure a remote repository, or
// returns "" for a backend no probe reaches.
func remoteVolumeSource(loc string) string {
	switch {
	case isRcloneLocation(loc):
		return "rclone"
	case strings.HasPrefix(strings.TrimSpace(loc), "sftp:"):
		return "sftp"
	}
	return ""
}

// remoteVolumeKey names a remote repository's volume in the samples table.
func remoteVolumeKey(loc string) string { return "remote:" + repoLocationKey(loc) }

// StorageForecast builds the forecast for a domain and source from its size
// samples, or returns nil when nothing is known. A local repository is
// measured now; a remote one is read from the capacity rule's last reading of
// its volume, since asking a remote costs an API call or an ssh login.
func (s *Service) StorageForecast(domain, source string, stats []store.RepoStat) *StorageForecast {
	now := time.Now()
	var f StorageForecast
	if growth, ok := growthBytesPerWeek(stats, now); ok {
		f.GrowthBytesPerWeek = &growth
	}
	if _, repo, err := s.domainRepoSource(domain, source); err == nil {
		switch {
		case !restic.IsRemoteRepo(repo):
			if free, fErr := s.diskFreeFn()(repo); fErr == nil {
				freeBytes := int64(math.Min(float64(free), math.MaxInt64)) // clamp: JSON numbers are signed
				f.FreeBytes = &freeBytes
				f.CapacitySource = localVolumeSource(repo)
			}
		case remoteVolumeSource(repo) == "":
			f.CapacityUnsupported = true
		default:
			if sample, ok := s.lastVolumeReading(remoteVolumeKey(repo), now); ok {
				f.FreeBytes = &sample.FreeBytes
				f.CapacitySource = sample.Source
			}
		}
	}
	if f.GrowthBytesPerWeek != nil && *f.GrowthBytesPerWeek > 0 && f.FreeBytes != nil {
		weeks := math.Round(float64(*f.FreeBytes)/float64(*f.GrowthBytesPerWeek)*10) / 10
		f.WeeksToFull = &weeks
	}
	if f.GrowthBytesPerWeek == nil && f.FreeBytes == nil && !f.CapacityUnsupported {
		return nil
	}
	return &f
}

// lastVolumeReading returns the newest reading of volume that is recent enough
// to stand for today.
func (s *Service) lastVolumeReading(volume string, now time.Time) (store.VolumeSample, bool) {
	samples, err := s.store.ListVolumeSamples(now.Add(-remoteReadingMaxAge).Unix())
	if err != nil {
		return store.VolumeSample{}, false
	}
	for i := len(samples) - 1; i >= 0; i-- {
		if samples[i].Volume == volume {
			return samples[i], true
		}
	}
	return store.VolumeSample{}, false
}

// localVolumeSource names what statfs measured for a path on this box.
func localVolumeSource(repo string) string {
	mounts, err := os.ReadFile(mountinfoPath)
	if err != nil {
		return volumeSource("")
	}
	return volumeSource(fsTypeAt(bytes.NewReader(mounts), repo))
}
