package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// nodeStartLag is how long after home's own containers a new node backs up, so
// the two do not reach for the same disks at once.
const nodeStartLag = 30 * time.Minute

// nodeStartTime is the time of day, as HH:MM, a new node backs up at: half an
// hour after the next run of home's containers schedule, or after 02:00, the
// time a schedule editor offers first, while home has none.
func nodeStartTime(homeCadence string, now time.Time) string {
	at := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
	if cad, err := schedule.ParseCadence(homeCadence); err == nil {
		if next, ok := cad.NextDue(time.Time{}, now); ok {
			at = next
		}
	}
	return at.Add(nodeStartLag).Format("15:04")
}

var errNodeCadence = errors.New("a schedule reads as off, daily HH:MM, weekly DOW HH:MM, everyN N HH:MM or a cron expression")

// nodeSchedule is when home starts the backups of each section of a node. A
// section left out keeps its schedule.
type nodeSchedule struct {
	Containers *string `json:"containers,omitempty"`
	Files      *string `json:"files,omitempty"`
}

// SetNodeSchedule stores when home starts the backups of each section of
// node id. A new cadence counts from now, so a fire before it starts nothing.
func (s *Service) SetNodeSchedule(id string, sched nodeSchedule) error {
	n, err := s.store.GetNode(id)
	if err != nil {
		return err
	}
	for domain, cadence := range map[string]*string{"containers": sched.Containers, "files": sched.Files} {
		if cadence == nil {
			continue
		}
		c := strings.TrimSpace(*cadence)
		if _, err := schedule.ParseCadence(c); err != nil {
			return errNodeCadence
		}
		if c == n.Cadence(domain) {
			continue
		}
		if err := s.store.SetNodeCadence(id, domain, c); err != nil {
			return err
		}
		if err := s.store.SetNodeStarted(id, domain, time.Now().Unix()); err != nil {
			return err
		}
	}
	return nil
}

// handleNodeSchedule stores when home starts the backups of a node's sections.
// PUT /api/nodes/{id}/schedule
func (h *Handler) handleNodeSchedule(w http.ResponseWriter, r *http.Request) {
	var body nodeSchedule
	if !decodeBody(w, r, &body) || !h.requireSignIn(w, r, "changing when a node backs up") {
		return
	}
	err := h.svc.SetNodeSchedule(r.PathValue("id"), body)
	switch {
	case errors.Is(err, store.ErrNodeNotFound):
		writeJSON(w, http.StatusNotFound, codedFailEnvelope(err, "not-found"))
	case errors.Is(err, errNodeCadence):
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(nil))
	}
}
