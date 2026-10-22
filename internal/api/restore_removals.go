package api

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

type restoreRemovals struct{ st *store.Repo }

func (r restoreRemovals) Mark(name string, in model.Inspect, oldID string) error {
	def, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return r.st.MarkRestoreRemoval(store.RestoreRemoval{Name: name, Definition: string(def), OldID: oldID, At: time.Now().Unix()})
}

func (r restoreRemovals) Clear(name string) error { return r.st.ClearRestoreRemoval(name) }

// RecoverInterruptedRestores creates the containers again, stopped, that a
// restore removed when BombVault stopped in the middle of it, and moves the
// containers sharing their namespaces onto them. It runs at boot next to
// RecoverContainerRestarts. Each gets one attempt, as a restore that fails
// does, and is forgotten after it.
func (s *Service) RecoverInterruptedRestores(ctx context.Context) {
	removed, err := s.store.ListRestoreRemovals()
	if err != nil {
		log.Printf("api: could not read which containers a restore removed: %v", err)
		return
	}
	var back, failed []string
	for _, m := range removed {
		if live, err := s.docker.InspectName(ctx, m.Name); err == nil && live != "" {
			s.forgetRestoreRemoval(m.Name)
			continue
		}
		var in model.Inspect
		if err := json.Unmarshal([]byte(m.Definition), &in); err != nil {
			log.Printf("api: the definition kept for %q does not read: %v", m.Name, err) //nolint:gosec // G706: %q-quoted
			failed = append(failed, m.Name)
			s.forgetRestoreRemoval(m.Name)
			continue
		}
		if err := s.docker.CreateAndStart(ctx, in, false); err != nil {
			log.Printf("api: %q, removed by an interrupted restore, could not be created again: %v", m.Name, err) //nolint:gosec // G706: %q-quoted
			failed = append(failed, m.Name)
			s.forgetRestoreRemoval(m.Name)
			continue
		}
		if rejoiner, ok := s.docker.(backup.NamespaceRejoiner); ok && m.OldID != "" {
			if _, err := rejoiner.RejoinNamespace(ctx, m.OldID, m.Name); err != nil {
				log.Printf("api: containers sharing a namespace with %q: %v", m.Name, err) //nolint:gosec // G706: %q-quoted
			}
		}
		back = append(back, m.Name)
		s.forgetRestoreRemoval(m.Name)
	}
	if len(back) > 0 {
		s.notifyZFSUnsuppressed(notify.Event{
			Title: "BombVault",
			Message: "A restart cut off a restore after it had removed these containers. They were created again, stopped, " +
				"and their data may be only partly restored, so restore them once more: " + strings.Join(back, ", "),
		})
	}
	if len(failed) > 0 {
		s.notifyZFSUnsuppressed(notify.Event{
			Title:   "BombVault",
			Message: "A restart cut off a restore after it had removed these containers, and they could not be created again: " + strings.Join(failed, ", "),
		})
	}
}

func (s *Service) forgetRestoreRemoval(name string) {
	if err := s.store.ClearRestoreRemoval(name); err != nil {
		log.Printf("api: forget the removal of %q: %v", name, err) //nolint:gosec // G706: %q-quoted
	}
}
