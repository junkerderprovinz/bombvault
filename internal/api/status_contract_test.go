package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/status"
	"github.com/junkerderprovinz/parleyport/kit/statustest"
)

// seedFortressRuns leaves a failed flash backup nobody marked as resolved and
// a config backup that went through, so the report has a need and the log two
// entries.
func seedFortressRuns(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, run := range []struct {
		id, target, status string
		at                 int64
	}{
		{"run-config", "config", "success", 1762650000},
		{"run-flash", "flash", "failed", 1762653600},
	} {
		if _, err := db.Exec(`INSERT INTO runs (id, target_id, kind, status, started_at, finished_at, error) VALUES (?, ?, 'backup', ?, ?, ?, '')`,
			run.id, run.target, run.status, run.at-60, run.at); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStatusContract(t *testing.T) {
	h, router, repo, db := newMCPGateHandler(t)
	seedFortressRuns(t, db)
	token, _ := seedAPIToken(t, h, repo, "Dashboard", false)
	statustest.Run(t, router, statustest.Options{Authenticate: func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+token)
	}})
}

func TestAnMCPKeyDoesNotOpenTheFortressStatus(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	key, _ := seedMCPKey(t, h, repo, "Assistant")
	r := httptest.NewRequest(http.MethodGet, status.StatusPath, nil)
	r.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("an MCP key got %d, want 401", w.Code)
	}
}

func TestAFailedBackupNobodyResolvedNeedsYou(t *testing.T) {
	h, router, repo, db := newMCPGateHandler(t)
	seedFortressRuns(t, db)
	token, _ := seedAPIToken(t, h, repo, "Dashboard", false)
	r := httptest.NewRequest(http.MethodGet, status.StatusPath+"?lang=de", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	var rep status.Report
	if err := json.Unmarshal(w.Body.Bytes(), &rep); err != nil {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if rep.State != status.StateFailed || len(rep.NeedsYou) != 1 {
		t.Fatalf("state %s with needs %+v", rep.State, rep.NeedsYou)
	}
	if n := rep.NeedsYou[0]; n.ID != "run:run-flash" || n.Code != "backup.failed" || n.Text != "Sicherung von Unraid-Flash fehlgeschlagen" {
		t.Errorf("need = %+v", n)
	}
	if rep.App.ID != "bombvault" || rep.App.Name != "BombVault" || status.CheckIcon(rep.App.Icon) != nil {
		t.Errorf("app = %.200v", rep.App)
	}
}

func TestSectionsMapOntoTheFortressStates(t *testing.T) {
	for in, want := range map[string]status.State{
		"ok":      status.StateOK,
		"never":   status.StateOK,
		"warn":    status.StateWarning,
		"overdue": status.StateOverdue,
		"off":     status.StateOff,
	} {
		if got := sectionState(in); got != want {
			t.Errorf("%s reads as %s, want %s", in, got, want)
		}
	}
}
