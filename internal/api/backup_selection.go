package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// selectionKey is the progress key of a backup of entries of several kinds.
// Done and Total count its entries; each kind's batch reports on its own key
// as well.
const selectionKey = "backup-selection"

// selectionRetry is how often the next kind's batch is offered while the one
// before it still holds the server.
var selectionRetry = 500 * time.Millisecond

// selectionPart is one kind's share of a selection. The parts run in the
// order given.
type selectionPart struct {
	Kind string   `json:"kind"`
	Keys []string `json:"keys"`
}

// selectionRefused is a part a selection leaves out before it starts, and why.
type selectionRefused struct {
	Kind  string `json:"kind"`
	Code  string `json:"code,omitempty"`
	Error string `json:"error"`
}

// StartBackupSelection backs up the parts one kind after the other on the
// server, so the selection goes on when the page that started it is closed.
// The first part starts before it returns: started is false and err nil while
// another backup holds the server, and err is that part's refusal otherwise.
// A flash or config part with nowhere to go is left out and named in refused.
func (s *Service) StartBackupSelection(ctx context.Context, parts []selectionPart) (refused []selectionRefused, started bool, err error) {
	if !s.selectionActive.CompareAndSwap(false, true) {
		return nil, false, nil
	}
	queue := make([]selectionPart, 0, len(parts))
	for _, p := range parts {
		if rErr := s.selectionPartRefusal(p); rErr != nil {
			refused = append(refused, selectionRefused{Kind: p.Kind, Code: refusalCode(rErr), Error: scrubError(rErr)})
			continue
		}
		queue = append(queue, p)
	}
	if len(queue) == 0 {
		s.selectionActive.Store(false)
		return refused, true, nil
	}
	ok, err := s.startSelectionPart(ctx, queue[0])
	if err != nil || !ok {
		s.selectionActive.Store(false)
		return refused, false, err
	}
	total := 0
	for _, p := range queue {
		total += len(p.Keys)
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.selectionActive.Store(false)
		defer s.recoverOperation("backup selection", nil, nil)
		s.runSelection(bctx, queue, total)
	}()
	return refused, true, nil
}

// runSelection starts each part after the first once the server is free,
// and reports until the last one has ended.
func (s *Service) runSelection(ctx context.Context, queue []selectionPart, total int) {
	done := 0
	s.publishSelection(done, total, true)
	defer func() { s.publishSelection(total, total, false) }()
	for i := 1; i < len(queue); i++ {
		if !s.awaitSelectionPart(ctx, queue[i]) {
			return
		}
		// A part starts once the one before it has let go of the server.
		done += len(queue[i-1].Keys)
		s.publishSelection(done, total, true)
	}
	s.awaitBatchFree()
}

// awaitSelectionPart offers the part until it starts, and reports false once
// BombVault shuts down. A refusal other than busy leaves the part out.
func (s *Service) awaitSelectionPart(ctx context.Context, p selectionPart) bool {
	deadline := time.Now().Add(drillWaitCap())
	for {
		ok, err := s.startSelectionPart(ctx, p)
		switch {
		case ok:
			return true
		case err != nil && !refusedAsBusy(err):
			log.Printf("api: backup selection: %s left out: %v", p.Kind, err) //nolint:gosec // G706: kind is one of the fixed kinds
			return true
		case time.Now().After(deadline):
			log.Printf("api: backup selection: %s left out, the server stayed busy", p.Kind) //nolint:gosec // G706: see above
			return true
		}
		select {
		case <-s.StopContext().Done():
			return false
		case <-time.After(selectionRetry):
		}
	}
}

// awaitBatchFree waits for the last part's batch to end, so the selection's
// bar covers it.
func (s *Service) awaitBatchFree() {
	for s.batchActive.Load() {
		select {
		case <-s.StopContext().Done():
			return
		case <-time.After(selectionRetry):
		}
	}
}

func (s *Service) startSelectionPart(ctx context.Context, p selectionPart) (bool, error) {
	switch p.Kind {
	case "container":
		return s.StartBackupAll(ctx, p.Keys)
	case "vm":
		return s.StartBackupVMsAll(ctx, p.Keys)
	case "files":
		return s.StartBackupFilesAll(ctx, p.Keys)
	case "zfs":
		return s.StartBackupZFSAll(ctx, p.Keys)
	case "flash":
		return s.StartBackupFlash(ctx)
	case "config":
		return s.StartBackupConfig(ctx)
	}
	return false, fmt.Errorf("unknown kind %q", p.Kind)
}

// selectionPartRefusal is why a singleton part cannot start at all. The
// batches of the other kinds skip an entry with no place on their own.
func (s *Service) selectionPartRefusal(p selectionPart) error {
	switch p.Kind {
	case "flash":
		return s.refuseNoPlace("flash", "")
	case "config":
		return s.refuseNoPlace("config", "")
	}
	return nil
}

func (s *Service) publishSelection(done, total int, active bool) {
	if s.progress == nil || total == 0 {
		return
	}
	s.progress.Publish(progress.Event{
		Key: selectionKey, Phase: "backup", Percent: float64(done) / float64(total) * 100,
		Active: active, Done: int64(done), Total: int64(total),
	})
}

// refusalCode is the code startRefusal gives err, empty when it names none.
func refusalCode(err error) string {
	code, _ := startRefusal(err)["code"].(string)
	return code
}

// selectionKinds are the kinds a selection can hold, as the page names them.
var selectionKinds = map[string]bool{"container": true, "vm": true, "files": true, "zfs": true, "flash": true, "config": true}

// handleBackupSelection starts the backup of a selection that spans several
// kinds and answers once the first kind has started, with the kinds refused
// before the start. The selection reports on the backup-selection progress key.
// POST /api/backup/selection  body {parts: [{kind, keys}]}
func (h *Handler) handleBackupSelection(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Parts []selectionPart `json:"parts"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if len(body.Parts) == 0 || len(body.Parts) > len(selectionKinds) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "parts must name between one and six kinds"})
		return
	}
	seen := map[string]bool{}
	for _, p := range body.Parts {
		if !selectionKinds[p.Kind] || seen[p.Kind] || len(p.Keys) == 0 || len(p.Keys) > zfsMaxBackupAllIDs {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid part"})
			return
		}
		seen[p.Kind] = true
		valid := validResourceName
		if p.Kind == "vm" {
			valid = validVMName
		}
		for _, k := range p.Keys {
			if !valid(k) {
				writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid name"})
				return
			}
		}
	}
	refused, started, err := h.svc.StartBackupSelection(r.Context(), body.Parts)
	if err != nil {
		writeJSON(w, http.StatusConflict, startRefusal(err))
		return
	}
	if !started {
		writeJSON(w, http.StatusConflict, batchBusyRefusal())
		return
	}
	if refused == nil {
		refused = []selectionRefused{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"refused": refused}))
}
