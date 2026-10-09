package api

import (
	"fmt"
	"log"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// zfsItemBackupPeriod is how often something backs the item up, 0 when
// nothing does.
func zfsItemBackupPeriod(d store.ZFSDataset, settings store.Settings) int64 {
	eff := schedule.EffectiveZFSDatasetSchedule(d, settings)
	switch eff.Kind {
	case schedule.EffectiveNone:
		return 0
	case schedule.EffectiveBoth:
		return min(cadencePeriodSeconds(eff.Spec), cadencePeriodSeconds(eff.AlsoSpec))
	}
	return cadencePeriodSeconds(eff.Spec)
}

// zfsReplicaPeriod is how often the item's replica is meant to run.
func zfsReplicaPeriod(d store.ZFSDataset, settings store.Settings) int64 {
	if d.Replica.AfterBackup {
		return zfsItemBackupPeriod(d, settings)
	}
	return cadencePeriodSeconds(d.Replica.Cadence)
}

// zfsReplicaCurrency is when the item's replica last succeeded and whether
// that is recent enough to count: within twice its period, the rule backups
// go by, or without a period at least as new as the last backup.
func (s *Service) zfsReplicaCurrency(now int64, d store.ZFSDataset, settings store.Settings) (lastOK int64, current bool) {
	if d.Replica.TargetKind == store.ZFSReplicaTargetNone {
		return 0, false
	}
	lastOK, err := s.store.LastSuccessOfKind(d.ID, store.ZFSReplicaRunKind)
	if err != nil {
		log.Printf("api: zfs replica: reading the last replica of %s failed: %v", d.Dataset, err)
		return 0, false
	}
	if lastOK == 0 {
		return 0, false
	}
	if period := zfsReplicaPeriod(d, settings); period > 0 {
		return lastOK, now-lastOK <= 2*period
	}
	lastBackup, err := s.store.LastSuccessOfKind(d.ID, "backup")
	if err != nil {
		return lastOK, false
	}
	return lastOK, lastOK >= lastBackup
}

// zfsReplicaCoverage reports whether every enabled ZFS item has a current
// replica, which makes the replicas the domain's copy off the premises, with
// the oldest of their last successes and the longest period among them.
func (s *Service) zfsReplicaCoverage(now int64, settings store.Settings) (oldest, period int64, covered bool) {
	items, err := s.store.ListZFSDatasets()
	if err != nil {
		log.Printf("api: zfs replica: listing the items for the status failed: %v", err)
		return 0, 0, false
	}
	for _, d := range items {
		if !d.Enabled {
			continue
		}
		lastOK, current := s.zfsReplicaCurrency(now, d, settings)
		if !current {
			return 0, 0, false
		}
		if !covered || lastOK < oldest {
			oldest = lastOK
		}
		period = max(period, zfsReplicaPeriod(d, settings))
		covered = true
	}
	return oldest, period, covered
}

// zfsReplicaDomainState is the replica line of the ZFS scorecard: "" with no
// item replicating, "failed" when an item's last run failed, "stale" when one
// is not current, "ok" otherwise.
func (s *Service) zfsReplicaDomainState(now int64, settings store.Settings) string {
	items, err := s.store.ListZFSDatasets()
	if err != nil {
		return ""
	}
	state := ""
	for _, d := range items {
		if !d.Enabled || d.Replica.TargetKind == store.ZFSReplicaTargetNone {
			continue
		}
		if run, ok, err := s.store.LatestZFSReplicaRun(d.ID); err == nil && ok && run.Status == "failed" {
			return zfsReplicaFailed
		}
		if _, current := s.zfsReplicaCurrency(now, d, settings); !current {
			state = "stale"
		} else if state == "" {
			state = zfsReplicaOK
		}
	}
	return state
}

// zfsItemSites is the 3-2-1 verdict of one ZFS item, scored the way the
// placement line scores an item of the other domains: the host is a site,
// a counting copy off the premises adds one, and two counting copies with one
// of them off the premises meet the rule. A current replica is such a copy,
// but no backup, so a replica without a backup is still one copy.
func (s *Service) zfsItemSites(now int64, d store.ZFSDataset, settings store.Settings, lastBackup int64) (int, string) {
	sites := map[string]bool{"host": true}
	counting, stale := 0, 0
	offSite, staleOffSite := false, false
	if lastBackup > 0 {
		counting++
		if repo, err := s.zfsDatasetRepoPath(settings, d); err == nil && restic.IsRemoteRepo(repo) {
			sites["repo"], offSite = true, true
		}
		if s.offsiteRepoFor(zfsDomain, settings) != "" {
			if at, _ := s.aggregateReplicationCurrency(zfsDomain); at > 0 {
				counting++
				sites["offsite"], offSite = true, true
			}
		}
	}
	switch lastOK, current := s.zfsReplicaCurrency(now, d, settings); {
	case current:
		counting++
		sites["replica"], offSite = true, true
	case lastOK > 0:
		stale++
		staleOffSite = true
	}
	switch {
	case counting >= 2 && offSite:
		return len(sites), "met"
	case counting+stale >= 2 && (offSite || staleOffSite):
		return len(sites), "unconfirmed"
	}
	return len(sites), "one-copy"
}

// zfsReplicaDigestLines is the replica currency of every replicating ZFS
// item for the weekly digest.
func (s *Service) zfsReplicaDigestLines(now int64, settings store.Settings) []string {
	items, err := s.store.ListZFSDatasets()
	if err != nil {
		log.Printf("api: digest: listing the ZFS items failed: %v", err)
		return nil
	}
	var lines []string
	for _, d := range items {
		if !d.Enabled || d.Replica.TargetKind == store.ZFSReplicaTargetNone {
			continue
		}
		lastOK, current := s.zfsReplicaCurrency(now, d, settings)
		switch {
		case lastOK == 0:
			lines = append(lines, fmt.Sprintf("- %s: no replica yet", d.Dataset))
		case !current:
			lines = append(lines, fmt.Sprintf("- %s: stale, last replica %s", d.Dataset, digestAge(now, lastOK)))
		default:
			lines = append(lines, fmt.Sprintf("- %s: current (last replica %s)", d.Dataset, digestAge(now, lastOK)))
		}
	}
	return lines
}
