package api

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestCheckOffsiteBudgetFiresOncePerCrossing: with a growth budget set and an
// off-site repo_stats sample over it, the first budget check fires exactly one
// notification; a second check while STILL over budget stays silent (the alarm
// is once per false→true crossing, not per replication).
func TestCheckOffsiteBudgetFiresOncePerCrossing(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() }) // close before TempDir cleanup (Windows file lock)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	// Budget of 1 GB, and an off-site sample of 2 GiB (over budget).
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.OffsiteGrowthBudgetGB = 1
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := st.AddRepoStat(store.RepoStat{
		Domain: "flash", Source: "offsite", At: 1700000000,
		RawSize: 2 * 1024 * 1024 * 1024, // 2 GiB > 1 GiB budget
	}); err != nil {
		t.Fatal(err)
	}

	// Recording notifier: the Unraid channel records over fakeHostSSH.Run.
	ssh := &fakeHostSSH{}
	svc := &Service{
		cfg:               config.Config{AppKey: strings.Repeat("a", 64)},
		store:             st,
		ssh:               ssh,
		offsiteOverBudget: map[string]bool{},
	}
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	svc.checkOffsiteBudget(context.Background(), "flash", settings)
	svc.checkOffsiteBudget(context.Background(), "flash", settings) // still over → no second alarm

	if len(ssh.runs) != 1 {
		t.Fatalf("a budget breach must alarm exactly once per crossing, got %d", len(ssh.runs))
	}
	if joined := strings.Join(ssh.runs[0], " "); !strings.Contains(joined, "over budget") {
		t.Fatalf("the notification should announce the budget breach, got %v", ssh.runs[0])
	}
}

// TestCheckOffsiteBudgetDisabledAndUnder: no alarm when the budget is off, and no
// alarm when the sample is under budget.
func TestCheckOffsiteBudgetDisabledAndUnder(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	if err := st.AddRepoStat(store.RepoStat{
		Domain: "flash", Source: "offsite", At: 1700000000,
		RawSize: 2 * 1024 * 1024 * 1024,
	}); err != nil {
		t.Fatal(err)
	}
	ssh := &fakeHostSSH{}
	svc := &Service{
		cfg:               config.Config{AppKey: strings.Repeat("a", 64)},
		store:             st,
		ssh:               ssh,
		offsiteOverBudget: map[string]bool{},
	}
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}

	// Budget disabled (0) → skipped entirely.
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	svc.checkOffsiteBudget(context.Background(), "flash", settings)

	// Budget set high (10 GB) → the 2 GiB sample is under it → no alarm.
	settings.OffsiteGrowthBudgetGB = 10
	svc.checkOffsiteBudget(context.Background(), "flash", settings)

	if len(ssh.runs) != 0 {
		t.Fatalf("no alarm expected when budget is off or under budget, got %d", len(ssh.runs))
	}
}
