package api

import (
	"errors"
	"log"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// errTargetsUncertain stops a pass of a domain with copy rules while only the
// settings field's target could be read: a rule names a target by its id, and
// that target has none.
var errTargetsUncertain = errors.New("the off-site targets could not be read by id, so nothing is copied while copy rules exist")

// failPass records a failed run at every target a pass would have visited and
// returns cause, so the history shows the replication that did not happen.
func (s *Service) failPass(domain string, targets []store.OffsiteTarget, cause error) error {
	reason := truncateRunErr(cause)
	if errors.Is(cause, errPlacementUnreadable) {
		reason = store.ReasonCopyRulesUnreadable
	}
	now := time.Now().Unix()
	for _, t := range targets {
		id, err := s.store.RecordOffsiteRunForTarget(domain, t.ID, now)
		if err == nil {
			err = s.store.FinishOffsiteRun(id, false, reason)
		}
		if err != nil {
			log.Printf("api: offsite %s: could not record the failed run: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		}
	}
	return cause
}
