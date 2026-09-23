package schedule

import (
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestBatchedOffsiteRunsAfterAllBackups checks the order of a scheduled
// containers, VMs or files run: every item is backed up first, then one batched
// local prune (the per-item forgets ran without --prune), then one batched
// off-site copy. Pruning before the copy leaves fewer snapshots to replicate.
// Files are keyed by set ID because RunFilesJob passes the ID to backupFn.
func TestBatchedOffsiteRunsAfterAllBackups(t *testing.T) {
	cases := []struct {
		domain string
		// wire registers the domain's job with the shared backupFn. Containers
		// need none; their funcs are the ones passed to New.
		wire func(sc *Scheduler, backupFn BackupFunc)
		// items are the keys backupFn receives, in run order: container or VM
		// name, or file set ID.
		items    [2]string
		settings store.Settings
	}{
		{
			domain:   "containers",
			wire:     func(sc *Scheduler, backupFn BackupFunc) {},
			items:    [2]string{"plex", "radarr"},
			settings: store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00"},
		},
		{
			domain: "vms",
			wire: func(sc *Scheduler, backupFn BackupFunc) {
				sc.SetVMJob(backupFn, func() ([]store.VMTarget, error) {
					return []store.VMTarget{
						{Name: "vm1", IncludeInSchedule: true},
						{Name: "vm2", IncludeInSchedule: true},
					}, nil
				})
			},
			items:    [2]string{"vm1", "vm2"},
			settings: store.Settings{VMsEnabled: true, VMsSchedule: "daily 03:00"},
		},
		{
			domain: "files",
			wire: func(sc *Scheduler, backupFn BackupFunc) {
				sc.SetFilesJob(backupFn, func() ([]store.FileSet, error) {
					return []store.FileSet{
						{ID: "fs1", Name: "documents", Enabled: true},
						{ID: "fs2", Name: "photos", Enabled: true},
					}, nil
				})
			},
			items:    [2]string{"fs1", "fs2"},
			settings: store.Settings{FilesEnabled: true, FilesSchedule: "daily 03:00"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.domain, func(t *testing.T) {
			var mu sync.Mutex
			var events []string

			backupFn := func(name string) error {
				mu.Lock()
				events = append(events, "backup:"+name)
				mu.Unlock()
				return nil
			}
			// Unused in the vms and files cases, where ContainersSchedule stays
			// off.
			listFn := func() ([]store.Target, error) {
				return []store.Target{
					{ContainerName: tc.items[0], IncludeInSchedule: true},
					{ContainerName: tc.items[1], IncludeInSchedule: true},
				}, nil
			}

			sc := New(backupFn, listFn)
			tc.wire(sc, backupFn)
			sc.SetPruneAfterBulkJob(func(domain string) {
				mu.Lock()
				events = append(events, "prune:"+domain)
				mu.Unlock()
			})
			sc.SetOffsiteAfterBulkJob(func(domain string) {
				mu.Lock()
				events = append(events, "offsite:"+domain)
				mu.Unlock()
			})

			if err := sc.ReloadWithDueChecks(tc.settings, nil, nil, nil, nil, nil, nil); err != nil {
				t.Fatalf("ReloadWithDueChecks: %v", err)
			}

			// Fire the entry through its wrapped job, as cron would.
			fired := false
			for _, e := range sc.entries {
				if e.domain == tc.domain {
					sc.c.Entry(e.id).WrappedJob.Run()
					fired = true
				}
			}
			if !fired {
				t.Fatalf("no %s entry registered", tc.domain)
			}

			mu.Lock()
			defer mu.Unlock()
			want := []string{
				"backup:" + tc.items[0],
				"backup:" + tc.items[1],
				"prune:" + tc.domain,
				"offsite:" + tc.domain,
			}
			if len(events) != len(want) {
				t.Fatalf("events = %v, want %v (backups, then ONE batched prune, then ONE batched off-site copy)", events, want)
			}
			for i := range want {
				if events[i] != want[i] {
					t.Fatalf("events = %v, want %v", events, want)
				}
			}
		})
	}
}

// TestEmptyDomainRunSkipsJobAndBatchedTail checks that a domain run with no item
// left to back up does nothing: no Healthchecks ping, no prune, no off-site
// copy. With per-item schedules every included item can sit on its own entry
// or be off, and the domain's everyN gate then finds no last success at any
// fire, because an empty pass records none. Running the tail anyway would prune
// and replicate every night and report "0 of 0 items succeeded". Each case
// keeps the domain schedule on, so the entry stays registered for the UI.
func TestEmptyDomainRunSkipsJobAndBatchedTail(t *testing.T) {
	cases := []struct {
		domain string
		wire   func(sc *Scheduler, backupFn BackupFunc)
		// listFn is the containers list New() takes; only the containers case
		// needs a non-empty one.
		listFn   func() ([]store.Target, error)
		settings store.Settings
	}{
		{
			domain: "containers",
			wire:   func(sc *Scheduler, backupFn BackupFunc) {},
			listFn: func() ([]store.Target, error) {
				// Included but paused per item, so it is neither in the domain
				// run nor on an entry of its own.
				return []store.Target{{ContainerName: "plex", IncludeInSchedule: true, ScheduleCadence: "off"}}, nil
			},
			settings: store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00", PerItemSchedules: true},
		},
		{
			domain: "vms",
			wire: func(sc *Scheduler, backupFn BackupFunc) {
				sc.SetVMJob(backupFn, func() ([]store.VMTarget, error) {
					return []store.VMTarget{{Name: "vm1", IncludeInSchedule: true, ScheduleCadence: "off"}}, nil
				})
			},
			listFn:   func() ([]store.Target, error) { return nil, nil },
			settings: store.Settings{VMsEnabled: true, VMsSchedule: "daily 03:00", PerItemSchedules: true},
		},
		{
			domain: "files",
			wire: func(sc *Scheduler, backupFn BackupFunc) {
				sc.SetFilesJob(backupFn, func() ([]store.FileSet, error) {
					return []store.FileSet{{ID: "fs1", Name: "documents", Enabled: false}}, nil
				})
			},
			listFn:   func() ([]store.Target, error) { return nil, nil },
			settings: store.Settings{FilesEnabled: true, FilesSchedule: "daily 03:00"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.domain, func(t *testing.T) {
			var mu sync.Mutex
			var events []string
			record := func(s string) {
				mu.Lock()
				events = append(events, s)
				mu.Unlock()
			}

			backupFn := func(name string) error { record("backup:" + name); return nil }
			sc := New(backupFn, tc.listFn)
			tc.wire(sc, backupFn)
			sc.SetPruneAfterBulkJob(func(domain string) { record("prune:" + domain) })
			sc.SetOffsiteAfterBulkJob(func(domain string) { record("offsite:" + domain) })
			sc.SetHealthchecksAggregator(
				func(domain string) { record("hc-start:" + domain) },
				func(domain string, attempted, failed int, failures []ItemFailure) { record("hc-finish:" + domain) },
			)

			if err := sc.ReloadWithDueChecks(tc.settings, nil, nil, nil, nil, nil, nil); err != nil {
				t.Fatalf("ReloadWithDueChecks: %v", err)
			}

			fired := false
			for _, e := range sc.entries {
				if e.domain == tc.domain {
					sc.c.Entry(e.id).WrappedJob.Run()
					fired = true
				}
			}
			if !fired {
				t.Fatalf("no %s entry registered; an enabled cadence must stay on the schedule even with no items", tc.domain)
			}

			mu.Lock()
			defer mu.Unlock()
			if len(events) != 0 {
				t.Fatalf("empty %s run performed %v, want nothing at all (no ping, no prune, no off-site copy)", tc.domain, events)
			}
		})
	}
}

// TestContainersJobNoOffsiteAfterBulkWhenUnwired checks that a run without
// SetOffsiteAfterBulkJob and SetPruneAfterBulkJob still backs up every
// container.
func TestContainersJobNoOffsiteAfterBulkWhenUnwired(t *testing.T) {
	var mu sync.Mutex
	var backups int

	sc := New(
		func(string) error { mu.Lock(); backups++; mu.Unlock(); return nil },
		func() ([]store.Target, error) {
			return []store.Target{{ContainerName: "plex", IncludeInSchedule: true}}, nil
		},
	)

	if err := sc.ReloadWithDueChecks(store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00"}, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	for _, e := range sc.entries {
		if e.domain == "containers" {
			sc.c.Entry(e.id).WrappedJob.Run()
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if backups != 1 {
		t.Fatalf("expected 1 backup, got %d", backups)
	}
}
