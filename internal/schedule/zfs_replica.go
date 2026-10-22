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

// ZFSReplicaRunsOnItsOwn reports whether the scheduler runs an item's replica
// on a plan of its own, and on which cadence.
func ZFSReplicaRunsOnItsOwn(d store.ZFSDataset, settings store.Settings) (string, bool) {
	rep := d.Replica
	if !settings.ZFSEnabled || !d.Enabled || rep.TargetKind == store.ZFSReplicaTargetNone || rep.AfterBackup {
		return "", false
	}
	if !classifyItemOverride(rep.Cadence).ownEntry {
		return "", false
	}
	return rep.Cadence, true
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
		if _, ok := ZFSReplicaRunsOnItsOwn(d, settings); !ok {
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
