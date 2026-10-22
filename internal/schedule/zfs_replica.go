package schedule

import (
	"fmt"
	"log"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// SetZFSReplicaJob wires the ZFS replicas that run on a cadence of their own
// rather than after each backup: replicaFn is called with the item's ID. The
// items come from the list SetZFSJob wired. Call before Reload.
func (s *Scheduler) SetZFSReplicaJob(replicaFn BackupFunc) {
	s.replicaZFS = replicaFn
}

// ZFSReplicaOwnSchedule reports whether an item's replica runs on a cron entry
// of its own, whatever its backup schedule says.
func ZFSReplicaOwnSchedule(d store.ZFSDataset, settings store.Settings) bool {
	rep := d.Replica
	return settings.ZFSEnabled && d.Enabled && rep.TargetKind != store.ZFSReplicaTargetNone && !rep.AfterBackup &&
		classifyItemOverride(rep.Cadence).ownEntry
}

// registerZFSReplicaEntries adds an entry for every enabled ZFS item whose
// replica has a cadence of its own. It does not depend on per-item schedules,
// which are about backups; with the ZFS domain off nothing replicates.
func (s *Scheduler) registerZFSReplicaEntries(settings store.Settings) error {
	if s.replicaZFS == nil || s.listZFSFn == nil || !settings.ZFSEnabled {
		return nil
	}
	ds, err := s.listZFSFn()
	if err != nil {
		log.Printf("schedule: zfs replicas: list datasets: %v", err)
		return nil
	}
	for _, d := range ds {
		if !ZFSReplicaOwnSchedule(d, settings) {
			continue
		}
		sched := classifyItemOverride(d.Replica.Cadence)
		id, name := d.ID, d.Dataset
		entry, err := s.c.AddFunc(sched.Spec, func() {
			log.Printf("schedule: running the zfs replica of %s", name)
			if err := s.replicaZFS(id); err != nil {
				log.Printf("schedule: zfs replica of %s failed: %v", name, err)
			}
		})
		if err != nil {
			return fmt.Errorf("schedule: zfs replica of %q: %w", name, err)
		}
		s.mu.Lock()
		s.entries = append(s.entries, scheduledEntry{id: entry, job: "replica", domain: "zfs"})
		s.mu.Unlock()
	}
	return nil
}
