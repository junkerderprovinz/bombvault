package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The actions BombVault offers the other apps of its fortress.
const (
	actionBackupLast = "backup.last"
)

func refuse(status int, code string) error {
	return &fortress.Refusal{Status: status, Code: code}
}

// readAction decodes an action's body into v, refusing one that is not JSON.
func readAction(body []byte, v any) error {
	if json.Unmarshal(body, v) != nil {
		return refuse(http.StatusBadRequest, "bad_request")
	}
	return nil
}

type backupLastAnswer struct {
	Domain string `json:"domain"`
	At     string `json:"at,omitempty"`
	OK     bool   `json:"ok"`
	Run    string `json:"run,omitempty"`
}

// backupLast says when a domain was last backed up successfully, for a
// member such as FireSquire that warns about an old flash backup.
func (s *Service) backupLast(_ context.Context, _ fortress.Caller, body []byte) ([]byte, error) {
	var in struct {
		Domain string `json:"domain"`
	}
	if err := readAction(body, &in); err != nil {
		return nil, err
	}
	run, ok, err := s.store.LastSuccessfulBackupRun(in.Domain)
	if errors.Is(err, store.ErrUnknownDomain) {
		return nil, refuse(http.StatusBadRequest, "unknown_domain")
	}
	if err != nil {
		return nil, err
	}
	answer := backupLastAnswer{Domain: in.Domain}
	if ok {
		answer.At = time.Unix(*run.FinishedAt, 0).UTC().Format(time.RFC3339)
		answer.OK, answer.Run = true, run.ID
	}
	return json.Marshal(answer)
}
