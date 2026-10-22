package api

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// vmRestarts keeps the VMs a graceful backup holds down in the database.
type vmRestarts struct{ st *store.Repo }

func (r vmRestarts) Mark(name string) error { return r.st.MarkVMStopped(name, time.Now().Unix()) }

func (r vmRestarts) Clear(name string) error { return r.st.ClearVMStopped(name) }

// RecoverVMRestarts starts the VMs a graceful backup shut down when BombVault
// stopped before starting them again, as RecoverContainerRestarts does for
// containers. virsh runs over SSH, so the caller runs it in the background. A
// VM that runs or is gone is forgotten; one that is still shutting down or
// does not start stays recorded for the next boot.
func (s *Service) RecoverVMRestarts(ctx context.Context) {
	names, err := s.store.ListVMsStopped()
	if err != nil {
		log.Printf("api: could not read which VMs a backup left shut down: %v", err)
		return
	}
	if len(names) == 0 || s.virsh == nil {
		return
	}
	defer s.lockDomainFor("vms", "startup-recovery")()
	started := 0
	for _, name := range names {
		state, err := s.virsh.State(ctx, name)
		if err != nil {
			log.Printf("api: %q was left shut down by a backup and its state cannot be read: %v", name, err) //nolint:gosec // G706: %q-quoted
			continue
		}
		switch state {
		case "shut off":
			log.Printf("api: starting VM %q again, an interrupted backup left it shut down", name) //nolint:gosec // G706: %q-quoted
			if err := s.virsh.Start(ctx, name); err != nil {
				log.Printf("api: starting VM %q failed, trying again at the next start: %v", name, err) //nolint:gosec // G706: %q-quoted
				continue
			}
			started++
		case "in shutdown":
			continue
		}
		if err := s.store.ClearVMStopped(name); err != nil {
			log.Printf("api: clearing the record of VM %q failed: %v", name, err) //nolint:gosec // G706: %q-quoted
		}
	}
	if started == 0 {
		return
	}
	if c, err := s.NotifyConfig(); err == nil {
		notify.Send(notify.WithHealthchecksSuppressed(context.Background()), c, "vms", notify.Event{
			Title:   "BombVault",
			Message: strconv.Itoa(started) + " VMs were left shut down by an interrupted backup and have been started again",
			OK:      true,
		})
	}
}
