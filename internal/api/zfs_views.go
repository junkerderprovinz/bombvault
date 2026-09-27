package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ZFSMemberView is one dataset of an item's tree as the last preflight or run
// found it.
type ZFSMemberView struct {
	Dataset        string `json:"dataset"`
	RelPath        string `json:"relPath"`
	HostMountpoint string `json:"hostMountpoint"`
	Outcome        string `json:"outcome"`
	IsNew          bool   `json:"isNew"`
	UsedByDataset  int64  `json:"usedByDataset"`
	LastBackupAt   int64  `json:"lastBackupAt"`
}

// ZFSDatasetView is one item of the ZFS domain as the page shows it.
type ZFSDatasetView struct {
	ID                string                     `json:"id"`
	Dataset           string                     `json:"dataset"`
	Enabled           bool                       `json:"enabled"`
	Excludes          []string                   `json:"excludes"`
	ScheduleCadence   string                     `json:"scheduleCadence"`
	Repo              string                     `json:"repo"`
	RepoEffective     string                     `json:"repoEffective"`
	StopContainers    []string                   `json:"stopContainers"`
	RestartPending    []string                   `json:"restartPending"`
	ExcludedChildren  []string                   `json:"excludedChildren"`
	HookContainer     string                     `json:"hookContainer"`
	PreSnapshot       string                     `json:"preSnapshot"`
	PostSnapshot      string                     `json:"postSnapshot"`
	HostMountpoint    string                     `json:"hostMountpoint"`
	LastBackup        int64                      `json:"lastBackup"`
	LastRunStatus     string                     `json:"lastRunStatus"`
	LastCheckCode     string                     `json:"lastCheckCode"`
	LastCheckDetail   string                     `json:"lastCheckDetail"`
	LastCheckAt       int64                      `json:"lastCheckAt"`
	LeftoverCount     int                        `json:"leftoverCount"`
	SafetyCount       int                        `json:"safetyCount"`
	SafetyOldestAt    int64                      `json:"safetyOldestAt"`
	Members           []ZFSMemberView            `json:"members"`
	EffectiveSchedule schedule.EffectiveSchedule `json:"effectiveSchedule"`
}

// ListZFSDatasetViews serves the page from the store alone. It asks the host
// nothing, so the list still renders while the machine is off.
func (s *Service) ListZFSDatasetViews(_ context.Context) ([]ZFSDatasetView, error) {
	items, err := s.store.ListZFSDatasets()
	if err != nil {
		return nil, fmt.Errorf("list zfs datasets: %w", err)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	views := make([]ZFSDatasetView, 0, len(items))
	for _, d := range items {
		v := ZFSDatasetView{
			ID:                d.ID,
			Dataset:           d.Dataset,
			Enabled:           d.Enabled,
			Excludes:          orEmptyList(d.Excludes),
			ScheduleCadence:   d.ScheduleCadence,
			Repo:              d.Repo,
			StopContainers:    orEmptyList(d.StopContainers),
			RestartPending:    orEmptyList(d.RestartPending),
			ExcludedChildren:  orEmptyList(d.ExcludedChildren),
			HookContainer:     d.HookContainer,
			PreSnapshot:       d.PreSnapshot,
			PostSnapshot:      d.PostSnapshot,
			HostMountpoint:    d.LastHostMountpoint,
			LastCheckCode:     d.LastCheckCode,
			LastCheckDetail:   d.LastCheckDetail,
			LastCheckAt:       d.LastCheckAt,
			LeftoverCount:     d.LeftoverCount,
			Members:           []ZFSMemberView{},
			EffectiveSchedule: schedule.EffectiveZFSDatasetSchedule(d, settings),
		}
		// An override that does not resolve is served raw: a repository the
		// item cannot reach is exactly what the user has to see.
		if eff, rErr := s.zfsDatasetRepoPath(settings, d); rErr == nil {
			v.RepoEffective = eff
		} else {
			v.RepoEffective = d.Repo
		}
		if run, rErr := s.store.LastRunOfKind(d.ID, "backup"); rErr == nil && run != nil {
			v.LastRunStatus = run.Status
		}
		if run, rErr := s.store.LastSuccessfulBackup(d.ID); rErr == nil && run != nil && run.FinishedAt != nil {
			v.LastBackup = *run.FinishedAt
		}
		members, mErr := s.store.ListZFSMembers(d.ID)
		if mErr != nil {
			return nil, mErr
		}
		for _, m := range members {
			v.Members = append(v.Members, ZFSMemberView{
				Dataset:        m.Dataset,
				RelPath:        zfsRelPath(d.Dataset, m.Dataset),
				HostMountpoint: m.HostMountpoint,
				Outcome:        m.Outcome,
				UsedByDataset:  m.UsedByDataset,
				LastBackupAt:   m.LastBackupAt,
			})
		}
		safety, sErr := s.store.ListZFSSafetySnapshots(d.ID)
		if sErr != nil {
			return nil, sErr
		}
		v.SafetyCount = len(safety)
		for _, snap := range safety {
			if v.SafetyOldestAt == 0 || snap.CreatedAt < v.SafetyOldestAt {
				v.SafetyOldestAt = snap.CreatedAt
			}
		}
		views = append(views, v)
	}
	return views, nil
}

// orEmptyList keeps a null out of the JSON, which the page renders as a list.
func orEmptyList(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// zfsRelPath is a member's place in the item's logical tree: "/" for the root,
// "/plex" for cache/appdata/plex.
func zfsRelPath(root, dataset string) string {
	if dataset == root {
		return "/"
	}
	return "/" + strings.TrimPrefix(dataset, root+"/")
}

// zfsRepoPath resolves the domain repository.
func (s *Service) zfsRepoPath(settings store.Settings) (string, error) {
	return s.resolveRepo(settings.ZFSPath)
}

// zfsDatasetRepoPath resolves where one item's members are written: its own
// named repository when it has one, otherwise the domain repository.
func (s *Service) zfsDatasetRepoPath(settings store.Settings, d store.ZFSDataset) (string, error) {
	return s.itemRepoPath(d.Repo, func() (string, error) { return s.zfsRepoPath(settings) })
}
