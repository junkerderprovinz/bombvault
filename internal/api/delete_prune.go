package api

import (
	"context"
	"errors"
	"log"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// outlastRequest keeps a delete of backups going when the request that asked
// for it ends, a closed tab or a reverse proxy giving up, so the snapshots and
// the entry they belong to go together. Only BombVault stopping ends it.
func (s *Service) outlastRequest(ctx context.Context) (context.Context, context.CancelFunc) {
	held, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stop := context.AfterFunc(s.StopContext(), cancel)
	return held, func() {
		stop()
		cancel()
	}
}

// pruneAfterDelete reclaims the space a delete freed once the delete has
// answered. On a large remote repository the prune outlasts any proxy or
// relay timeout, and a delete that waited for it was reported as failed after
// every snapshot was already gone. It waits for the domain lock the delete
// holds and is recorded as a prune of the domain.
func (s *Service) pruneAfterDelete(domain, repo string, mode restic.Mode) {
	s.deletePrunes.Add(1)
	go func() {
		defer s.deletePrunes.Done()
		defer s.recoverOperation("prune after a delete", nil, nil)
		unlock := s.lockDomainFor(domain, "prune")
		defer unlock()
		ctx := s.StopContext()
		runID, err := s.startRun(ctx, domainRunTargetID(domain), "prune")
		if err != nil {
			log.Printf("api: prune after a delete in %s: the run is not recorded: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		}
		err = s.retryAfterUnlock(ctx, repo, mode, func() error { return s.engine.Prune(ctx, repo, mode) })
		if runID != "" {
			status := "success"
			switch {
			case errors.Is(err, context.Canceled):
				status = "cancelled"
			case err != nil:
				status = "failed"
			}
			if fErr := s.store.FinishRun(runID, status, "", 0, truncateRunErr(err)); fErr != nil {
				log.Printf("api: prune after a delete in %s: the run is not closed: %v", domain, fErr) //nolint:gosec // G706: domain is a fixed literal
			}
		}
		if err != nil {
			log.Printf("api: prune after a delete in %s: %v", domain, scrubError(err)) //nolint:gosec // G706: the error is scrubbed
		}
	}()
}
