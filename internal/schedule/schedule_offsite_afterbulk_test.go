package schedule

import (
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestBatchedOffsiteRunsAfterAllBackups pins the #95 replication rewrite plus
// the batched-prune ordering across every multi-item domain (containers, VMs,
// files): a scheduled run backs up every included/enabled item FIRST, then
// runs the batched local prune ONCE (retention first — the per-item forgets
// ran without --prune), then the batched off-site replication ONCE — never
// per-item inline, and never prune after the copy (pruning first means fewer
// snapshots to replicate). Table-driven over the three domainSpec closures in
// ReloadWithDueChecks that share this shape; files is keyed on the file set's
// stable ID (not its Name) because RunFilesJob hands backupFn the ID. White-box
// (package schedule) so it can fire the registered cron entry synchronously and
// observe call ordering.
func TestBatchedOffsiteRunsAfterAllBackups(t *testing.T) {
	cases := []struct {
		domain string
		// wire registers the domain-specific job (SetVMJob/SetFilesJob) using the
		// shared backupFn/events recorder. containers needs no extra wiring — its
		// backup/list funcs are the ones passed to New itself.
		wire func(sc *Scheduler, backupFn BackupFunc)
		// items are the two item keys RunXJob passes to backupFn, in run order —
		// container/VM name, or file-set ID.
		items    [2]string
		settings store.Settings
	}{
		{
			domain:   "containers",
			wire:     func(sc *Scheduler, backupFn BackupFunc) {},
			items:    [2]string{"plex", "radarr"},
			settings: store.Settings{ContainersSchedule: "daily 03:00"},
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
			settings: store.Settings{VMsSchedule: "daily 03:00"},
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
			settings: store.Settings{FilesSchedule: "daily 03:00"},
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
			// The containers list/backup funcs New() takes; unused (return no
			// items ever run) for the vms/files subtests since ContainersSchedule
			// is left at its zero value ("off") there.
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

			if err := sc.ReloadWithDueChecks(tc.settings, nil, nil, nil, nil, nil); err != nil {
				t.Fatalf("ReloadWithDueChecks: %v", err)
			}

			// Fire the domain entry synchronously through its wrapped job (the same
			// path cron would run), so we observe the real fn including the
			// post-loop batched calls.
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

// TestContainersJobNoOffsiteAfterBulkWhenUnwired ensures the batched post-loop
// hooks are optional: with neither SetOffsiteAfterBulkJob nor SetPruneAfterBulkJob
// wired, the run still backs up every container and simply performs no batched
// replication or prune (both nil-guarded).
func TestContainersJobNoOffsiteAfterBulkWhenUnwired(t *testing.T) {
	var mu sync.Mutex
	var backups int

	sc := New(
		func(string) error { mu.Lock(); backups++; mu.Unlock(); return nil },
		func() ([]store.Target, error) {
			return []store.Target{{ContainerName: "plex", IncludeInSchedule: true}}, nil
		},
	)
	// Deliberately NOT calling SetOffsiteAfterBulkJob or SetPruneAfterBulkJob.

	if err := sc.ReloadWithDueChecks(store.Settings{ContainersSchedule: "daily 03:00"}, nil, nil, nil, nil, nil); err != nil {
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
