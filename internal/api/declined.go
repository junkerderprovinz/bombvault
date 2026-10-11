package api

import (
	"errors"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// declinedBackups is the entries the person said not to back up.
type declinedBackups struct {
	marks map[store.BackupEntry]bool
}

func (s *Service) declinedBackups() (declinedBackups, error) {
	marks, err := s.store.DeclinedBackups()
	return declinedBackups{marks: marks}, err
}

// holds reports whether the entry is set aside and so counts neither as
// protected nor as something that needs the person. That covers an entry no
// schedule reaches. The first backup a schedule makes drops the mark for good,
// when the run is recorded, so reading the list writes nothing.
func (d declinedBackups) holds(kind, key string, eff schedule.EffectiveSchedule) bool {
	return d.marks[store.BackupEntry{Kind: kind, Key: key}] && eff.Kind == schedule.EffectiveNone
}

var errEntryIsScheduled = errors.New("a schedule backs this entry up, take it out of the schedule first")

// handleDeclineBackup serves PUT /api/items/{kind}/{key}/declined: the person
// says not to back an entry up, or takes that back. The answer is the entry's
// row.
func (h *Handler) handleDeclineBackup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Declined bool `json:"declined"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	item, err := h.backupItem(r.Context(), settings, r.PathValue("kind"), r.PathValue("key"))
	switch {
	case errors.Is(err, errNoSuchItem), errors.Is(err, errUnknownItemKind):
		writeJSON(w, http.StatusNotFound, codedFailEnvelope(err, "not-found"))
		return
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	entry := store.BackupEntry{Kind: item.Kind, Key: item.Key}
	if body.Declined {
		if item.EffectiveSchedule.Kind != schedule.EffectiveNone {
			writeJSON(w, http.StatusOK, codedFailEnvelope(errEntryIsScheduled, "scheduled"))
			return
		}
		err = h.store.DeclineBackup(entry)
	} else {
		err = h.store.AcceptBackup(entry)
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	item.BackupDeclined = body.Declined
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"item": item}))
}
