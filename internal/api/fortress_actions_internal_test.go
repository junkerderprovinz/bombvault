package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func newFortressSvc(t *testing.T) (*Service, *store.Repo) {
	t.Helper()
	st := newTestStore(t)
	return NewService(config.Config{DataDir: t.TempDir()}, st, nil, nil, nil), st
}

var fireSquire = fortress.Caller{ID: "f1", App: "firesquire", Device: "tower"}

func act(t *testing.T, run func(context.Context, fortress.Caller, []byte) ([]byte, error), body string) map[string]any {
	t.Helper()
	out, err := run(context.Background(), fireSquire, []byte(body))
	if err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	var answer map[string]any
	if err := json.Unmarshal(out, &answer); err != nil {
		t.Fatalf("%s answered %s: %v", body, out, err)
	}
	return answer
}

func refusedWith(t *testing.T, err error, status int, code string) {
	t.Helper()
	var r *fortress.Refusal
	if !errors.As(err, &r) || r.Status != status || r.Code != code {
		t.Fatalf("got %v, want a refusal %d %s", err, status, code)
	}
}

func TestBackupLastNamesWhenTheFlashWasLastBackedUp(t *testing.T) {
	svc, st := newFortressSvc(t)

	if got := act(t, svc.backupLast, `{"domain":"flash"}`); got["ok"] != false || got["domain"] != "flash" || got["at"] != nil {
		t.Fatalf("before any backup: %v", got)
	}

	id, err := st.StartRun(store.FlashTargetID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(id, "success", "abc123", 1, ""); err != nil {
		t.Fatal(err)
	}
	run, err := st.GetRun(id)
	if err != nil {
		t.Fatal(err)
	}
	got := act(t, svc.backupLast, `{"domain":"flash"}`)
	want := time.Unix(*run.FinishedAt, 0).UTC().Format(time.RFC3339)
	if got["ok"] != true || got["domain"] != "flash" || got["at"] != want || got["run"] != id {
		t.Fatalf("after a backup: %v, want at %s and run %s", got, want, id)
	}
}

func TestBackupLastRefusesADomainItDoesNotKnow(t *testing.T) {
	svc, _ := newFortressSvc(t)
	_, err := svc.backupLast(context.Background(), fireSquire, []byte(`{"domain":"photos"}`))
	refusedWith(t, err, 400, "unknown_domain")
	_, err = svc.backupLast(context.Background(), fireSquire, []byte(`{"domain":`))
	refusedWith(t, err, 400, "bad_request")
}
