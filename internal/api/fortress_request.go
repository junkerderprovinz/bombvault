package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const actionBackupRequest = "backup.request"

const (
	// requestWait is how long a request waits for another backup to end
	// before it gives up, the time ShipLog waits for the whole of it.
	requestWait = 30 * time.Minute
	// requestsKept bounds the requests a member can still ask about.
	requestsKept = 64
)

// houseRequest is a backup another app of the fortress asked for, before
// an update or a maintenance run. Its run row only exists once the backup
// starts, so the member gets the request's id.
type houseRequest struct {
	id        string
	target    string
	name      string
	asked     time.Time
	startedAt int64
	started   bool
	err       string
}

type backupRequestAnswer struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Snapshot string `json:"snapshot,omitempty"`
	Link     string `json:"link,omitempty"`
	Error    string `json:"error,omitempty"`
}

// backupRequest starts a backup of one container for a member, or, for a
// body that names a run, says how the backup it started stands.
func (s *Service) backupRequest(ctx context.Context, from fortress.Caller, body []byte) ([]byte, error) {
	var in struct {
		Container string `json:"container"`
		Reason    string `json:"reason"`
		Run       string `json:"run"`
	}
	if err := readAction(body, &in); err != nil {
		return nil, err
	}
	if in.Run != "" {
		return s.requestState(in.Run)
	}
	if in.Reason != "update" && in.Reason != "maintenance" {
		return nil, refuse(http.StatusBadRequest, "unknown_reason")
	}
	tg, err := s.store.GetTargetByContainer(in.Container)
	if errors.Is(err, sql.ErrNoRows) || in.Container == "" {
		return nil, refuse(http.StatusNotFound, "unknown_container")
	}
	if err != nil {
		return nil, err
	}
	var raw [8]byte
	_, _ = rand.Read(raw[:])
	req := &houseRequest{id: hex.EncodeToString(raw[:]), target: tg.ID, name: tg.ContainerName, asked: time.Now()}
	s.house.mu.Lock()
	if s.house.requests == nil {
		s.house.requests = map[string]*houseRequest{}
	}
	s.house.requests[req.id] = req
	s.house.order = append(s.house.order, req.id)
	if len(s.house.order) > requestsKept {
		delete(s.house.requests, s.house.order[0])
		s.house.order = s.house.order[1:]
	}
	s.house.mu.Unlock()
	log.Printf("fortress: %s on %s asks for a backup of %s before %s", from.App, from.Device, tg.ContainerName, in.Reason) //nolint:gosec // G706: names from the fortress and the store, %s on a log line
	go s.startRequested(context.WithoutCancel(ctx), req)
	return json.Marshal(map[string]string{"run": req.id})
}

// startRequested starts the backup a member asked for, waiting while another
// backup runs.
func (s *Service) startRequested(ctx context.Context, req *houseRequest) {
	start, retry := s.house.start, s.house.retry
	if start == nil {
		start = s.StartBackup
	}
	if retry == 0 {
		retry = 10 * time.Second
	}
	for {
		at := time.Now().Unix()
		started, err := start(ctx, req.name)
		var busy domainBusyError
		if err != nil && !errors.As(err, &busy) {
			s.house.mu.Lock()
			req.err = err.Error()
			s.house.mu.Unlock()
			return
		}
		if started {
			s.house.mu.Lock()
			req.started, req.startedAt = true, at
			s.house.mu.Unlock()
			return
		}
		if time.Since(req.asked) > requestWait {
			s.house.mu.Lock()
			req.err = "another backup ran for too long"
			s.house.mu.Unlock()
			return
		}
		time.Sleep(retry)
	}
}

// requestState reads how the backup behind a request stands, from the run
// it started.
func (s *Service) requestState(id string) ([]byte, error) {
	s.house.mu.Lock()
	req, ok := s.house.requests[id]
	var r houseRequest
	if ok {
		r = *req
	}
	s.house.mu.Unlock()
	if !ok {
		return nil, refuse(http.StatusNotFound, "unknown_run")
	}
	answer := backupRequestAnswer{ID: r.id, State: "queued"}
	switch {
	case r.err != "":
		answer.State, answer.Error = "failed", r.err
	case r.started:
		answer.State = "running"
		runs, err := s.store.ListRunsFiltered(store.RunFilter{TargetIDs: []string{r.target}, Kinds: []string{"backup"}, Since: r.startedAt, Limit: requestsKept})
		if err != nil {
			return nil, err
		}
		if len(runs) > 0 {
			// Newest first, and the request's own run is the first one after it started.
			run := runs[len(runs)-1]
			switch run.Status {
			case "success":
				answer.State, answer.Snapshot = "done", run.SnapshotID
				answer.Link = strings.TrimSuffix(s.selfDirectURL(), "/") + "/backups/container/" + r.name
			case "failed", "cancelled":
				answer.State, answer.Error = run.Status, run.Error
			}
		}
	}
	return json.Marshal(answer)
}
