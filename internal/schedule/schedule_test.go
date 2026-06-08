package schedule_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ---------------------------------------------------------------------------
// ParseCadence tests
// ---------------------------------------------------------------------------

func TestParseCadenceOff(t *testing.T) {
	spec, enabled, err := schedule.ParseCadence("off")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enabled {
		t.Fatal("'off' must be disabled")
	}
	if spec != "" {
		t.Fatalf("spec for 'off' must be empty, got %q", spec)
	}
}

func TestParseCadenceDaily(t *testing.T) {
	spec, enabled, err := schedule.ParseCadence("daily 02:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Fatal("daily must be enabled")
	}
	if spec != "30 2 * * *" {
		t.Fatalf("expected '30 2 * * *', got %q", spec)
	}
}

func TestParseCadenceWeekly(t *testing.T) {
	spec, enabled, err := schedule.ParseCadence("weekly Mon 03:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Fatal("weekly must be enabled")
	}
	if spec != "0 3 * * 1" {
		t.Fatalf("expected '0 3 * * 1', got %q", spec)
	}
}

func TestParseCadenceWeeklyAllDays(t *testing.T) {
	cases := []struct {
		day  string
		want string
	}{
		{"Sun", "0"},
		{"Mon", "1"},
		{"Tue", "2"},
		{"Wed", "3"},
		{"Thu", "4"},
		{"Fri", "5"},
		{"Sat", "6"},
	}
	for _, c := range cases {
		spec, enabled, err := schedule.ParseCadence("weekly " + c.day + " 00:00")
		if err != nil {
			t.Fatalf("day %s: unexpected error: %v", c.day, err)
		}
		if !enabled {
			t.Fatalf("day %s: must be enabled", c.day)
		}
		want := "0 0 * * " + c.want
		if spec != want {
			t.Fatalf("day %s: expected %q, got %q", c.day, want, spec)
		}
	}
}

func TestParseCadenceRawCron(t *testing.T) {
	raw := "15 4 * * 2"
	spec, enabled, err := schedule.ParseCadence(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Fatal("raw cron must be enabled")
	}
	if spec != raw {
		t.Fatalf("raw cron must pass through unchanged, got %q", spec)
	}
}

func TestParseCadenceInvalid(t *testing.T) {
	cases := []string{
		"",
		"daily",
		"daily 25:00",
		"daily 02:60",
		"weekly",
		"weekly Mon",
		"weekly Xyz 03:00",
		"not a cron at all extra words here",
	}
	for _, s := range cases {
		_, _, err := schedule.ParseCadence(s)
		if err == nil {
			t.Fatalf("expected error for %q", s)
		}
	}
}

// ---------------------------------------------------------------------------
// Scheduler test — inject a fake BackupFunc and call the containers job directly
// ---------------------------------------------------------------------------

func TestSchedulerContainersJobCallsBackupFunc(t *testing.T) {
	var mu sync.Mutex
	var called []string

	backupFn := func(containerName string) error {
		mu.Lock()
		called = append(called, containerName)
		mu.Unlock()
		return nil
	}

	targets := []store.Target{
		{ContainerName: "plex", IncludeInSchedule: true},
		{ContainerName: "sonarr", IncludeInSchedule: false},
		{ContainerName: "radarr", IncludeInSchedule: true},
	}

	// RunContainersJob is the exported hook that lets tests trigger the job
	// synchronously without real time passing.
	schedule.RunContainersJob(targets, backupFn)

	mu.Lock()
	defer mu.Unlock()

	if len(called) != 2 {
		t.Fatalf("expected 2 containers backed up, got %d: %v", len(called), called)
	}
	if called[0] != "plex" || called[1] != "radarr" {
		t.Fatalf("expected [plex radarr], got %v", called)
	}
}

func TestSchedulerContainersJobContinuesOnError(t *testing.T) {
	var mu sync.Mutex
	var called []string

	backupFn := func(containerName string) error {
		mu.Lock()
		called = append(called, containerName)
		mu.Unlock()
		if containerName == "plex" {
			return errors.New("backup failed")
		}
		return nil
	}

	targets := []store.Target{
		{ContainerName: "plex", IncludeInSchedule: true},
		{ContainerName: "radarr", IncludeInSchedule: true},
	}

	// A single job failure must not abort subsequent containers.
	schedule.RunContainersJob(targets, backupFn)

	mu.Lock()
	defer mu.Unlock()

	if len(called) != 2 {
		t.Fatalf("expected 2 attempts, got %d: %v", len(called), called)
	}
}

func TestSchedulerReloadRegistersEnabledDomains(t *testing.T) {
	backupFn := func(_ string) error { return nil }
	listFn := func() ([]store.Target, error) { return nil, nil }

	sched := schedule.New(backupFn, listFn)

	// Reload with containers on daily schedule — must not panic or error.
	settings := store.Settings{
		ContainersSchedule: "daily 03:00",
		VMsSchedule:        "off",
		FlashSchedule:      "off",
	}
	if err := sched.Reload(settings); err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}

	// Reload again with all off — must clear entries without panic.
	settings.ContainersSchedule = "off"
	if err := sched.Reload(settings); err != nil {
		t.Fatalf("second Reload returned error: %v", err)
	}

	sched.Stop()
}
