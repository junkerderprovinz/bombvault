package api

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// vmRestarts keeps the VMs a graceful backup shuts down in the database.
type vmRestarts struct{ st *store.Repo }

func (r vmRestarts) Mark(names []string) error {
	return r.st.MarkVMsStopped(names, time.Now().Unix())
}

func (r vmRestarts) Clear(names []string) error { return r.st.ClearVMsStopped(names) }

// vmRecoveryPolls and vmRecoveryInterval bound the wait for a guest that was
// still shutting down when BombVault stopped, the same 90 seconds a backup
// gives it. vmRecoveryBudget leaves room for slow virsh calls over SSH.
const (
	vmRecoveryPolls    = 18
	vmRecoveryInterval = 5 * time.Second
	vmRecoveryBudget   = 5 * time.Minute
)

// RecoverVMRestarts starts the VMs a graceful backup left shut off when
// BombVault stopped in the middle of it. virsh goes over SSH and a guest may
// still be shutting down, so the work runs in the background; the VM domain
// lock is taken before it returns, which keeps a scheduled VM backup behind it.
func (s *Service) RecoverVMRestarts(ctx context.Context) {
	names, err := s.store.ListVMsStopped()
	if err != nil {
		log.Printf("api: could not read which VMs a backup left shut off: %v", err)
		return
	}
	if len(names) == 0 {
		return
	}
	unlock := s.lockDomainFor("vms", "startup-sweep")
	go func() {
		defer unlock()
		rctx, cancel := context.WithTimeout(ctx, vmRecoveryBudget)
		defer cancel()
		s.restartLeftVMs(rctx, names, vmRecoveryPolls, vmRecoveryInterval)
	}()
}

// restartLeftVMs starts each recorded VM once it is shut off, waiting up to
// polls rounds for guests still on their way down. It never forces a VM off: one
// still up after the wait was never left off, or someone started it, and is
// forgotten untouched. One whose state cannot be read stays recorded for the
// next boot.
func (s *Service) restartLeftVMs(ctx context.Context, names []string, polls int, interval time.Duration) {
	if s.ssh != nil {
		if err := s.ssh.EnsureKnownHost(ctx); err != nil {
			log.Printf("api: VM restart after an interrupted backup: ssh: %v", err)
		}
	}
	started := 0
	var done []string
	pending := names
	for round := 0; len(pending) > 0; round++ {
		var still []string
		for _, name := range pending {
			state, err := s.virsh.State(ctx, name)
			switch {
			case err != nil:
				log.Printf("api: %q was left shut off by a backup and its state cannot be read: %v", name, err) //nolint:gosec // G706: %q-quoted
			case state == "":
				log.Printf("api: %q was left shut off by a backup and is no longer defined, forgetting it", name) //nolint:gosec // G706: %q-quoted
				done = append(done, name)
			case state == "shut off":
				log.Printf("api: starting %q again, an interrupted backup left it shut off", name) //nolint:gosec // G706: %q-quoted
				if err := s.virsh.Start(ctx, name); err != nil {
					log.Printf("api: starting %q again failed: %v", name, err) //nolint:gosec // G706: %q-quoted
					continue
				}
				done = append(done, name)
				started++
			default:
				still = append(still, name)
			}
		}
		pending = still
		if len(pending) == 0 {
			break
		}
		if round >= polls-1 {
			done = append(done, pending...)
			break
		}
		select {
		case <-ctx.Done():
			pending = nil
		case <-time.After(interval):
		}
	}
	if err := s.store.ClearVMsStopped(done); err != nil {
		log.Printf("api: clearing the record of the restarted VMs failed: %v", err)
	}
	if started == 0 {
		return
	}
	s.notifyZFSUnsuppressed(notify.Event{
		Title:   "BombVault",
		Message: strconv.Itoa(started) + " VMs were left shut off by an interrupted backup and have been started again",
		OK:      true,
	})
}
