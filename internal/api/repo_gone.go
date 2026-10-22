package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// codeRepoGone is the code a refusal by errRemoteRepoGone carries, which the
// page answers with the offer to start a new repository there.
const codeRepoGone = "repo-gone"

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

// StartOverAtTarget lets the next copy to the off-site target id create a new
// repository where the old one is gone.
func (s *Service) StartOverAtTarget(id string) error {
	target, ok, err := s.store.GetOffsiteTarget(id)
	if err != nil {
		return fmt.Errorf("read off-site target: %w", err)
	}
	if !ok {
		return errors.New("no such off-site target")
	}
	repo, err := s.resolveRepo(target.Repo)
	if err != nil {
		return err
	}
	return s.startOverAt(repo)
}

// StartOverAtPrimary does the same for the remote backup path of a domain.
func (s *Service) StartOverAtPrimary(domain string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	loc := domainPathRaw(domain, settings)
	if loc == "" {
		return errors.New("no backup path configured for this domain")
	}
	repo, err := s.resolveRepo(loc)
	if err != nil {
		return err
	}
	return s.startOverAt(repo)
}

// handleStartOverAtTarget serves POST /api/offsite/targets/{id}/start-over.
func (h *Handler) handleStartOverAtTarget(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.StartOverAtTarget(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleStartOverAtPrimary serves POST /api/settings/primary-remote/{domain}/start-over.
func (h *Handler) handleStartOverAtPrimary(w http.ResponseWriter, r *http.Request) {
	domain, ok := handlePrimaryRemoteDomain(w, r)
	if !ok {
		return
	}
	if err := h.svc.StartOverAtPrimary(domain); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
