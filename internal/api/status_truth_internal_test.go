package api

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// statusFixture is a Service on a real database, with the handle to age its rows.
func statusFixture(t *testing.T, mutate func(*store.Settings)) (*Service, *store.Repo, *sql.DB) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.ContainersEnabled, s.FilesEnabled, s.FlashEnabled = true, true, true
		s.ContainersSchedule = "daily 03:00"
		mutate(s)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &Service{cfg: config.Config{AppKey: strings.Repeat("a", 64)}, store: st}, st, db
}

// container adds an entry set up long ago and, for a status other than "", a
// finished backup run of it that old.
func container(t *testing.T, st *store.Repo, db *sql.DB, name, status string, age time.Duration) string {
	t.Helper()
	tg, err := st.UpsertTarget(store.Target{ContainerName: name, IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age).Unix()
	if _, err := db.Exec(`UPDATE targets SET created_at = ? WHERE id = ?`, time.Now().Add(-90*24*time.Hour).Unix(), tg.ID); err != nil {
		t.Fatal(err)
	}
	if status == "" {
		return tg.ID
	}
	id, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(id, status, "", 0, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE runs SET started_at = ?, finished_at = ? WHERE id = ?`, when, when, id); err != nil {
		t.Fatal(err)
	}
	return tg.ID
}

func statusOf(t *testing.T, s *Service, domain string) DomainStatusEntry {
	t.Helper()
	all, err := s.DomainStatus()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range all {
		if d.Domain == domain {
			return d
		}
	}
	t.Fatalf("no status of %s", domain)
	return DomainStatusEntry{}
}

func TestANeighbourDoesNotHideAnEntryThatStopped(t *testing.T) {
	s, st, db := statusFixture(t, func(*store.Settings) {})
	container(t, st, db, "plex", "success", time.Hour)
	container(t, st, db, "sonarr", "success", 3*24*time.Hour)

	got := statusOf(t, s, "containers")
	if got.Status != "overdue" || got.StaleEntry != "sonarr" {
		t.Fatalf("status %q, stale %q, want overdue for sonarr", got.Status, got.StaleEntry)
	}
	if age := time.Now().Unix() - got.LastSuccess; age < 2*86400 {
		t.Fatalf("the status shows a success %ds old, not the one that fell behind", age)
	}
}

func TestAnEntryNoRunReachesAnyMoreDoesNotCount(t *testing.T) {
	cases := map[string]func(*testing.T, *store.Repo, *sql.DB){
		"its container is gone": func(t *testing.T, st *store.Repo, db *sql.DB) {
			container(t, st, db, "sonarr", statusSkipped, time.Hour)
		},
		"it is off the schedule": func(t *testing.T, st *store.Repo, db *sql.DB) {
			tg := container(t, st, db, "sonarr", "success", 30*24*time.Hour)
			if _, err := db.Exec(`UPDATE targets SET include_in_schedule = 0 WHERE id = ?`, tg); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			s, st, db := statusFixture(t, func(*store.Settings) {})
			container(t, st, db, "plex", "success", time.Hour)
			setup(t, st, db)
			if got := statusOf(t, s, "containers"); got.Status != "ok" {
				t.Fatalf("status %q, stale %q, want ok", got.Status, got.StaleEntry)
			}
		})
	}
}

func TestAnEntryWithoutABackupCountsOncePastItsFirstRun(t *testing.T) {
	s, st, db := statusFixture(t, func(*store.Settings) {})
	container(t, st, db, "plex", "success", time.Hour)
	newcomer, err := st.UpsertTarget(store.Target{ContainerName: "newcomer", IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := statusOf(t, s, "containers"); got.Status != "ok" {
		t.Fatalf("an entry set up just now reads %q", got.Status)
	}
	if _, err := db.Exec(`UPDATE targets SET created_at = ? WHERE id = ?`, time.Now().Add(-48*time.Hour).Unix(), newcomer.ID); err != nil {
		t.Fatal(err)
	}
	if got := statusOf(t, s, "containers"); got.Status != "overdue" || got.StaleEntry != "newcomer" {
		t.Fatalf("an entry that never got its backup reads %q, stale %q", got.Status, got.StaleEntry)
	}
}

func TestOwnPlansAreNotHeldToTheSectionsCadence(t *testing.T) {
	s, st, db := statusFixture(t, func(*store.Settings) {})
	for _, name := range []string{"plex", "sonarr"} {
		tg := container(t, st, db, name, "success", 3*24*time.Hour)
		if _, err := db.Exec(`UPDATE targets SET schedule_cadence = 'weekly Sun 02:00' WHERE id = ?`, tg); err != nil {
			t.Fatal(err)
		}
	}
	if got := statusOf(t, s, "containers"); got.Status != "ok" || got.PeriodSeconds != 7*86400 {
		t.Fatalf("status %q, period %d, want ok on the weekly plan", got.Status, got.PeriodSeconds)
	}
}

func TestASectionRunStillJudgesTheEntriesItReaches(t *testing.T) {
	s, st, db := statusFixture(t, func(*store.Settings) {})
	own := container(t, st, db, "plex", "success", time.Hour)
	if _, err := db.Exec(`UPDATE targets SET schedule_cadence = 'weekly Sun 02:00' WHERE id = ?`, own); err != nil {
		t.Fatal(err)
	}
	container(t, st, db, "sonarr", "success", 3*24*time.Hour)
	if got := statusOf(t, s, "containers"); got.Status != "overdue" || got.PeriodSeconds != 86400 {
		t.Fatalf("status %q, period %d, want overdue on the daily section plan", got.Status, got.PeriodSeconds)
	}
}

func TestAPlanSetNowWaitsForItsFirstRun(t *testing.T) {
	s, st, db := statusFixture(t, func(*store.Settings) {})
	s.NoteSectionPlans()
	if _, err := db.Exec(`UPDATE schema_migrations SET applied_at = applied_at - ?`, 30*86400); err != nil {
		t.Fatal(err)
	}
	if _, err := st.MutateSettings(func(set *store.Settings) error {
		set.FlashSchedule = "daily 03:00"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := statusOf(t, s, "flash"); got.Status != "overdue" {
		t.Fatalf("with no record of the change the old rule reads %q", got.Status)
	}
	s.NoteSectionPlans()
	if got := statusOf(t, s, "flash"); got.Status != "never" {
		t.Fatalf("a plan set just now reads %q", got.Status)
	}
	if _, err := db.Exec(`UPDATE section_plans SET changed_at = changed_at - ?`, 2*86400); err != nil {
		t.Fatal(err)
	}
	if got := statusOf(t, s, "flash"); got.Status != "overdue" {
		t.Fatalf("a plan whose first run never came reads %q", got.Status)
	}
}

func TestARecordedPlanStaysAcrossALook(t *testing.T) {
	s, st, db := statusFixture(t, func(*store.Settings) {})
	s.NoteSectionPlans()
	if at, err := st.SectionPlanChangedAt("containers"); err != nil || at != 0 {
		t.Fatalf("the first look reads %d, %v, want no time", at, err)
	}
	if _, err := st.MutateSettings(func(set *store.Settings) error {
		set.ContainersSchedule = "daily 04:00"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}
	s.NoteSectionPlans()
	at, err := st.SectionPlanChangedAt("containers")
	if err != nil || at == 0 {
		t.Fatalf("a changed plan reads %d, %v", at, err)
	}
	if _, err := db.Exec(`UPDATE section_plans SET changed_at = 5`); err != nil {
		t.Fatal(err)
	}
	s.NoteSectionPlans()
	if at, err := st.SectionPlanChangedAt("containers"); err != nil || at != 5 {
		t.Fatalf("a look at the same plan moved its time to %d, %v", at, err)
	}
}

func TestTheOverdueAlertNamesTheEntryThatFellBehind(t *testing.T) {
	var mu sync.Mutex
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		body = string(b)
	}))
	t.Cleanup(srv.Close)
	s, st, db := statusFixture(t, func(*store.Settings) {})
	if err := s.SetNotifyConfig(notify.Config{On: "failure", WebhookEnabled: true, WebhookURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	container(t, st, db, "plex", "success", time.Hour)
	container(t, st, db, "sonarr", "success", 3*24*time.Hour)
	if err := s.runWatchdogAt(context.Background(), time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(body, "sonarr") || !strings.Contains(body, "last success 3d ago") {
		t.Fatalf("the alert does not name the entry: %s", body)
	}
}

func TestHomeAssistantReadsEveryEntryOfASection(t *testing.T) {
	s, st, db := statusFixture(t, func(*store.Settings) {})
	h := &Handler{svc: s, store: st}
	container(t, st, db, "plex", "failed", 2*time.Hour)
	container(t, st, db, "sonarr", "success", time.Hour)
	if got := h.lastBackupResult("containers"); got != "failed" {
		t.Fatalf("one failed entry beside a newer success reads %q", got)
	}
	container(t, st, db, "plex", "success", time.Minute)
	if got := h.lastBackupResult("containers"); got != "success" {
		t.Fatalf("every entry good reads %q", got)
	}
}
