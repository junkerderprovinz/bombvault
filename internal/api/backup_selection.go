package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// backupSelectionKey is the progress key of a backup of several kinds. Done
// and Total count the kinds that have started; each kind's batch reports on
// its own key as well.
const backupSelectionKey = "backup-selection"

// backupSelectionRetry is how often a kind waiting for its turn asks again.
var backupSelectionRetry = 2 * time.Second

// backupSelectionPart is the entries of one kind a selection backs up.
type backupSelectionPart struct {
	Kind string   `json:"kind"`
	Keys []string `json:"keys"`
}

// StartBackupSelection backs up the batch of each kind in turn. The first
// starts before it returns, so its refusal is the answer; the others wait in
// the background until the server lets go of the one before. started is
// false while another batch runs.
func (s *Service) StartBackupSelection(ctx context.Context, parts []backupSelectionPart) (bool, error) {
	if !s.selectionActive.CompareAndSwap(false, true) {
		return false, nil
	}
	started, err := s.startSelectionPart(ctx, parts[0])
	if err != nil || !started || len(parts) == 1 {
		s.selectionActive.Store(false)
		return started, err
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.selectionActive.Store(false)
		defer s.recoverOperation("backup selection", nil, nil)
		s.startSelectionRest(bctx, parts)
	}()
	return true, nil
}

func (s *Service) startSelectionRest(ctx context.Context, parts []backupSelectionPart) {
	total := int64(len(parts))
	for i, part := range parts[1:] {
		s.publishSelection(int64(i+1), total, true)
		if !s.awaitSelectionTurn(ctx, part) {
			break
		}
	}
	s.publishSelection(total, total, false)
}

// awaitSelectionTurn starts one kind's batch once the server takes it. A
// refusal other than a busy server is logged and the selection goes on; false
// means the service is stopping.
func (s *Service) awaitSelectionTurn(ctx context.Context, part backupSelectionPart) bool {
	for {
		started, err := s.startSelectionPart(ctx, part)
		if started {
			return true
		}
		if err != nil && !refusedAsBusy(err) {
			log.Printf("api: backup selection: the %s did not start (going on): %v", part.Kind, err)
			return true
		}
		select {
		case <-s.StopContext().Done():
			return false
		case <-time.After(backupSelectionRetry):
		}
	}
}

func (s *Service) startSelectionPart(ctx context.Context, part backupSelectionPart) (bool, error) {
	switch part.Kind {
	case "container":
		return s.StartBackupAll(ctx, part.Keys)
	case "vm":
		return s.StartBackupVMsAll(ctx, part.Keys)
	case "files":
		return s.StartBackupFilesAll(ctx, part.Keys)
	case zfsDomain:
		return s.StartBackupZFSAll(ctx, part.Keys)
	case "flash":
		return s.StartBackupFlash(ctx)
	case "config":
		return s.StartBackupConfig(ctx)
	}
	return false, fmt.Errorf("unknown kind %q", part.Kind)
}

func (s *Service) publishSelection(done, total int64, active bool) {
	if s.progress == nil {
		return
	}
	s.progress.Publish(progress.Event{
		Key: backupSelectionKey, Phase: "backup", Percent: float64(done) / float64(total) * 100,
		Active: active, Done: done, Total: total,
	})
}

// validSelectionPart checks a part's kind and keys at the boundary, as each
// kind's own batch route does.
func validSelectionPart(part backupSelectionPart) bool {
	switch part.Kind {
	case "flash", "config":
		return true
	case "container", "files", zfsDomain:
		return validKeys(part.Keys, validResourceName)
	case "vm":
		return validKeys(part.Keys, validVMName)
	}
	return false
}

func validKeys(keys []string, valid func(string) bool) bool {
	if len(keys) == 0 || len(keys) > 1000 {
		return false
	}
	for _, k := range keys {
		if !valid(k) {
			return false
		}
	}
	return true
}

// handleBackupSelection serves POST /api/backup/selection: the entries of
// several kinds, backed up one kind after the other on the server.
func (h *Handler) handleBackupSelection(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Parts []backupSelectionPart `json:"parts"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if len(body.Parts) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "nothing selected"})
		return
	}
	for _, part := range body.Parts {
		if !validSelectionPart(part) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid selection"})
			return
		}
	}
	started, err := h.svc.StartBackupSelection(r.Context(), body.Parts)
	if err != nil {
		writeJSON(w, http.StatusConflict, startRefusal(err))
		return
	}
	if !started {
		writeJSON(w, http.StatusConflict, batchBusyRefusal())
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}
