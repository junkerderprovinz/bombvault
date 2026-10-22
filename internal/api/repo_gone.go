package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// codeRepoGone is the code a refusal by errRemoteRepoGone carries, which the
// page answers with the offer to start a new repository there.
const codeRepoGone = "repo-gone"

// remoteRepoGone notes that repo was found gone, so the page of every storage
// location that holds it offers to start over, and returns errRemoteRepoGone.
func (s *Service) remoteRepoGone(repo string) error {
	if err := s.store.MarkRepoGone(repo); err != nil {
		log.Printf("api: mark repo gone: %v", err)
	}
	return errRemoteRepoGone
}

// startOverAt forgets that the remote location repo held a repository, so the
// next run creates a new one there. It is the user's answer to
// errRemoteRepoGone; a folder on this host is told apart by its mount instead.
func (s *Service) startOverAt(repo string) error {
	if !restic.IsRemoteRepo(repo) {
		return errors.New("this location is a folder on this host, which needs no new start")
	}
	if err := s.store.ClearRepoEstablished(repo); err != nil {
		return fmt.Errorf("start a new repository: %w", err)
	}
	return nil
}

// StartOverAtLocation lets the next run create a new repository in every
// remote repository of the storage location id that was found gone. A
// destination holds one per section, and each was emptied with the disk.
func (s *Service) StartOverAtLocation(id string) error {
	loc, err := s.StorageLocation(id)
	if err != nil {
		return err
	}
	for _, repo := range loc.repos {
		if !s.repoFoundGone(repo) {
			continue
		}
		if err := s.startOverAt(repo); err != nil {
			return err
		}
	}
	return nil
}

// handleStartOverAtLocation serves POST /api/storage/locations/{id}/start-over.
func (h *Handler) handleStartOverAtLocation(w http.ResponseWriter, r *http.Request) {
	err := h.svc.StartOverAtLocation(r.PathValue("id"))
	if errors.Is(err, errNoSuchLocation) {
		writeJSON(w, http.StatusNotFound, failEnvelope(err))
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
