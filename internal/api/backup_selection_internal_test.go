package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func quickSelectionRetry(t *testing.T) {
	t.Helper()
	before := selectionRetry
	selectionRetry = 10 * time.Millisecond
	t.Cleanup(func() { selectionRetry = before })
}

func ranBackup(t *testing.T, f *placementFixture, id string) bool {
	t.Helper()
	runs, err := f.st.RecentRunsOfKind(id, "backup", 1)
	if err != nil {
		t.Fatal(err)
	}
	return len(runs) == 1 && runs[0].FinishedAt != nil
}

func TestASelectionBacksUpTheNextKindOnceTheServerLetsGo(t *testing.T) {
	quickSelectionRetry(t)
	f, _, _, _ := flashScene(t)
	unlock := f.svc.lockDomainFor("config", "prune")

	parts := []selectionPart{{Kind: "flash", Keys: []string{"flash"}}, {Kind: "config", Keys: []string{"config"}}}
	if code, out := f.call(http.MethodPost, "/api/backup/selection", map[string]any{"parts": parts}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("POST /api/backup/selection = %d %v", code, out)
	}
	waitFor(t, "the flash backup", func() bool { return ranBackup(t, f, store.FlashTargetID) })
	time.Sleep(50 * time.Millisecond)
	if !f.svc.selectionActive.Load() || ranBackup(t, f, store.ConfigTargetID) {
		t.Fatal("the configuration did not wait for its section")
	}
	if code, out := f.call(http.MethodPost, "/api/backup/selection", map[string]any{"parts": parts}); code != http.StatusConflict || out["code"] != codeBatchBusy {
		t.Fatalf("a second selection = %d %v, want it refused while the first waits", code, out)
	}

	unlock()
	waitFor(t, "the configuration backup", func() bool { return ranBackup(t, f, store.ConfigTargetID) })
	waitFor(t, "the selection to end", func() bool { return !f.svc.selectionActive.Load() })
}

func TestASelectionWhoseFirstKindIsRefusedStartsNothing(t *testing.T) {
	f, _, _, _ := flashScene(t)
	f.svc.batchActive.Store(true)
	code, out := f.call(http.MethodPost, "/api/backup/selection", map[string]any{"parts": []selectionPart{{Kind: "flash", Keys: []string{"flash"}}, {Kind: "config", Keys: []string{"config"}}}})
	f.svc.batchActive.Store(false)
	if code != http.StatusConflict || out["code"] != codeBatchBusy {
		t.Fatalf("POST /api/backup/selection = %d %v, want 409 batch-busy", code, out)
	}
	if f.svc.selectionActive.Load() {
		t.Fatal("a refused selection still holds the server")
	}
}

func TestASelectionStopsWaitingWhenTheServiceStops(t *testing.T) {
	quickSelectionRetry(t)
	f, _, _, _ := flashScene(t)
	unlock := f.svc.lockDomainFor("config", "prune")
	defer unlock()
	_, started, err := f.svc.StartBackupSelection(context.Background(), []selectionPart{{Kind: "flash", Keys: []string{"flash"}}, {Kind: "config", Keys: []string{"config"}}})
	if err != nil || !started {
		t.Fatalf("StartBackupSelection = %v, %v", started, err)
	}
	waitFor(t, "the flash backup", func() bool { return ranBackup(t, f, store.FlashTargetID) })
	f.svc.EndDetachedWork()
	waitFor(t, "the selection to give up", func() bool { return !f.svc.selectionActive.Load() })
}

func TestASelectionWithAnUnknownKindIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	code, _ := f.call(http.MethodPost, "/api/backup/selection", map[string]any{"parts": []selectionPart{{Kind: "printer", Keys: []string{"x"}}}})
	if code != http.StatusBadRequest {
		t.Fatalf("an unknown kind = %d, want 400", code)
	}
}
