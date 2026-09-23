package schedule_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestParseCadenceOff(t *testing.T) {
	cad, err := schedule.ParseCadence("off")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cad.Enabled {
		t.Fatal("'off' must be disabled")
	}
	if cad.Spec != "" {
		t.Fatalf("spec for 'off' must be empty, got %q", cad.Spec)
	}
}

func TestParseCadenceDaily(t *testing.T) {
	cad, err := schedule.ParseCadence("daily 02:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cad.Enabled {
		t.Fatal("daily must be enabled")
	}
	if cad.Spec != "30 2 * * *" {
		t.Fatalf("expected '30 2 * * *', got %q", cad.Spec)
	}
	if cad.IntervalDays != 0 {
		t.Fatalf("daily must have IntervalDays=0, got %d", cad.IntervalDays)
	}
}

func TestParseCadenceWeekly(t *testing.T) {
	cad, err := schedule.ParseCadence("weekly Mon 03:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cad.Enabled {
		t.Fatal("weekly must be enabled")
	}
	if cad.Spec != "0 3 * * 1" {
		t.Fatalf("expected '0 3 * * 1', got %q", cad.Spec)
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
		cad, err := schedule.ParseCadence("weekly " + c.day + " 00:00")
		if err != nil {
			t.Fatalf("day %s: unexpected error: %v", c.day, err)
		}
		if !cad.Enabled {
			t.Fatalf("day %s: must be enabled", c.day)
		}
		want := "0 0 * * " + c.want
		if cad.Spec != want {
			t.Fatalf("day %s: expected %q, got %q", c.day, want, cad.Spec)
		}
	}
}

func TestParseCadenceWeeklyMultiDOW(t *testing.T) {
	cad, err := schedule.ParseCadence("weekly Mon,Wed,Fri 02:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cad.Enabled {
		t.Fatal("must be enabled")
	}
	if cad.Spec != "30 2 * * 1,3,5" {
		t.Fatalf("expected '30 2 * * 1,3,5', got %q", cad.Spec)
	}
}

func TestParseCadenceWeeklyMultiDOWCaseInsensitive(t *testing.T) {
	cad, err := schedule.ParseCadence("weekly mon,WED,fri 00:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cad.Spec != "0 0 * * 1,3,5" {
		t.Fatalf("expected '0 0 * * 1,3,5', got %q", cad.Spec)
	}
}

func TestParseCadenceWeeklyDuplicateDOW(t *testing.T) {
	_, err := schedule.ParseCadence("weekly Mon,Mon 03:00")
	if err == nil {
		t.Fatal("expected error for duplicate day")
	}
}

func TestParseCadenceRawCron(t *testing.T) {
	raw := "15 4 * * 2"
	cad, err := schedule.ParseCadence(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cad.Enabled {
		t.Fatal("raw cron must be enabled")
	}
	if cad.Spec != raw {
		t.Fatalf("raw cron must pass through unchanged, got %q", cad.Spec)
	}
}

func TestCadencePeriodSeconds(t *testing.T) {
	cases := []struct {
		name    string
		cadence string
		want    int64
	}{
		{"daily", "daily 02:30", 86400},
		{"weekly", "weekly Mon 03:00", 604800},
		{"everyN", "everyN 3 04:00", 3 * 86400},
		{"cron daily", "0 5 * * *", 86400},
		{"cron weekly", "15 4 * * 2", 604800},
		{"off", "off", 0},
		{"empty", "", 0},
		// With several weekdays the period is the longest wait between two
		// fires. Sat and Sun fire a day apart and then six days apart, so the
		// answer is six days, not one.
		{"weekly two adjacent days", "weekly Sat,Sun 03:00", 6 * 86400},
		{"cron two adjacent days", "0 3 * * 0,6", 6 * 86400},
		// Mon/Wed/Fri: Fri to Mon is the widest gap.
		{"cron three spread days", "0 3 * * 1,3,5", 3 * 86400},
		{"cron weekdays only", "0 3 * * 1-5", 3 * 86400},
		// An even split has one gap, and the widest-gap rule must not inflate it.
		{"cron twice daily", "0 3,15 * * *", 12 * 3600},
		// The widest monthly gap is 31 days, not February's 28.
		{"cron monthly", "0 3 1 * *", 31 * 86400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cad, err := schedule.ParseCadence(c.cadence)
			if err != nil {
				t.Fatalf("ParseCadence(%q): %v", c.cadence, err)
			}
			if got := cad.PeriodSeconds(); got != c.want {
				t.Fatalf("PeriodSeconds(%q) = %d, want %d", c.cadence, got, c.want)
			}
		})
	}
}

func TestParseCadenceEmptyIsOff(t *testing.T) {
	cad, err := schedule.ParseCadence("")
	if err != nil {
		t.Fatalf("empty cadence must not error, got: %v", err)
	}
	if cad.Enabled {
		t.Fatal("empty cadence must be disabled")
	}
	if cad.Spec != "" {
		t.Fatalf("empty cadence spec must be empty, got %q", cad.Spec)
	}
}

func TestParseCadenceWeeklyCaseInsensitive(t *testing.T) {
	cases := []string{"mon", "MON", "Mon"}
	for _, dow := range cases {
		cad, err := schedule.ParseCadence("weekly " + dow + " 03:00")
		if err != nil {
			t.Fatalf("DOW %q: unexpected error: %v", dow, err)
		}
		if !cad.Enabled {
			t.Fatalf("DOW %q: must be enabled", dow)
		}
		if cad.Spec != "0 3 * * 1" {
			t.Fatalf("DOW %q: expected '0 3 * * 1', got %q", dow, cad.Spec)
		}
	}
}

func TestParseCadenceEveryN(t *testing.T) {
	cad, err := schedule.ParseCadence("everyN 5 03:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cad.Enabled {
		t.Fatal("everyN must be enabled")
	}
	if cad.Spec != "0 3 * * *" {
		t.Fatalf("expected '0 3 * * *', got %q", cad.Spec)
	}
	if cad.IntervalDays != 5 {
		t.Fatalf("expected IntervalDays=5, got %d", cad.IntervalDays)
	}
}

func TestParseCadenceEveryNOne(t *testing.T) {
	cad, err := schedule.ParseCadence("everyN 1 00:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cad.IntervalDays != 1 {
		t.Fatalf("expected IntervalDays=1, got %d", cad.IntervalDays)
	}
}

func TestParseCadenceInvalid(t *testing.T) {
	cases := []string{
		"daily",
		"daily 25:00",
		"daily 02:60",
		"weekly",
		"weekly Mon",
		"weekly Xyz 03:00",
		"not a cron at all extra words here",
		"everyN",
		"everyN 0 03:00",
		"everyN -1 03:00",
		"everyN abc 03:00",
		"everyN 5",
	}
	for _, s := range cases {
		_, err := schedule.ParseCadence(s)
		if err == nil {
			t.Fatalf("expected error for %q", s)
		}
	}
}

func TestEveryNDueGateSkipsWhenTooSoon(t *testing.T) {
	var ran bool

	lastRun := func() (time.Time, error) {
		return time.Now().Add(-1 * time.Hour), nil
	}

	jobFn := func() { ran = true }
	gate := buildEveryNGate(5, lastRun, jobFn)
	gate()

	if ran {
		t.Fatal("expected job to be skipped (interval not elapsed)")
	}
}

func TestEveryNDueGateFiresWhenDue(t *testing.T) {
	var ran bool

	lastRun := func() (time.Time, error) {
		return time.Now().Add(-6 * 24 * time.Hour), nil
	}

	jobFn := func() { ran = true }
	gate := buildEveryNGate(5, lastRun, jobFn)
	gate()

	if !ran {
		t.Fatal("expected job to run (interval elapsed)")
	}
}

func TestEveryNDueGateFiresWhenNeverRun(t *testing.T) {
	var ran bool

	lastRun := func() (time.Time, error) {
		return time.Time{}, nil
	}

	jobFn := func() { ran = true }
	gate := buildEveryNGate(30, lastRun, jobFn)
	gate()

	if !ran {
		t.Fatal("expected job to run (never run before)")
	}
}

func TestEveryNDueGateSkipsOnLastRunError(t *testing.T) {
	var ran bool

	lastRun := func() (time.Time, error) {
		return time.Time{}, errors.New("db unavailable")
	}

	jobFn := func() { ran = true }
	gate := buildEveryNGate(5, lastRun, jobFn)
	gate()

	if ran {
		t.Fatal("expected job to be skipped when lastRun query errors")
	}
}

// buildEveryNGate wires the everyN due-gate the way the Scheduler does, so it can
// be tested without a cron runner. The decision itself comes from
// schedule.EveryNDue, so the tests exercise the scheduler's rule rather than a
// copy of it.
func buildEveryNGate(intervalDays int, lastRunFn schedule.LastRunFunc, jobFn func()) func() {
	return func() {
		last, err := lastRunFn()
		if err != nil {
			return
		}
		if !schedule.EveryNDue(last, time.Now(), intervalDays) {
			return
		}
		jobFn()
	}
}

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

	// The failure list feeds the summary notification, which names each failed
	// container and why it failed.
	attempted, failed, failures := schedule.RunContainersJob(targets, backupFn)

	mu.Lock()
	defer mu.Unlock()

	if len(called) != 2 {
		t.Fatalf("expected 2 attempts, got %d: %v", len(called), called)
	}
	if attempted != 2 || failed != 1 {
		t.Fatalf("expected attempted=2 failed=1, got attempted=%d failed=%d", attempted, failed)
	}
	if len(failures) != 1 || failures[0].Name != "plex" || failures[0].Reason != "backup failed" {
		t.Fatalf("expected failures=[{plex backup failed}], got %+v", failures)
	}
}

func TestRunVMsJobBacksUpOnlyScheduled(t *testing.T) {
	var mu sync.Mutex
	var called []string

	backupFn := func(name string) error {
		mu.Lock()
		called = append(called, name)
		mu.Unlock()
		return nil
	}

	vms := []store.VMTarget{
		{Name: "ubuntu", IncludeInSchedule: true},
		{Name: "windows", IncludeInSchedule: false},
		{Name: "debian", IncludeInSchedule: true},
	}

	schedule.RunVMsJob(vms, backupFn)

	mu.Lock()
	defer mu.Unlock()

	if len(called) != 2 {
		t.Fatalf("expected 2 VMs backed up, got %d: %v", len(called), called)
	}
	if called[0] != "ubuntu" || called[1] != "debian" {
		t.Fatalf("expected [ubuntu debian], got %v", called)
	}
}

func TestRunVMsJobContinuesOnError(t *testing.T) {
	var mu sync.Mutex
	var called []string

	backupFn := func(name string) error {
		mu.Lock()
		called = append(called, name)
		mu.Unlock()
		if name == "ubuntu" {
			return errors.New("backup failed")
		}
		return nil
	}

	vms := []store.VMTarget{
		{Name: "ubuntu", IncludeInSchedule: true},
		{Name: "debian", IncludeInSchedule: true},
	}

	schedule.RunVMsJob(vms, backupFn)

	mu.Lock()
	defer mu.Unlock()

	if len(called) != 2 {
		t.Fatalf("expected 2 attempts, got %d: %v", len(called), called)
	}
}

// TestRunFilesJobBacksUpOnlyEnabledByID checks that the files job skips disabled
// sets and passes each set's ID rather than its name, so renaming a set does not
// orphan its run history.
func TestRunFilesJobBacksUpOnlyEnabledByID(t *testing.T) {
	var mu sync.Mutex
	var called []string

	backupFn := func(id string) error {
		mu.Lock()
		called = append(called, id)
		mu.Unlock()
		return nil
	}

	sets := []store.FileSet{
		{ID: "id-docs", Name: "docs", Enabled: true},
		{ID: "id-media", Name: "media", Enabled: false},
		{ID: "id-photos", Name: "photos", Enabled: true},
	}

	attempted, failed, failures := schedule.RunFilesJob(sets, backupFn)

	mu.Lock()
	defer mu.Unlock()

	if attempted != 2 || failed != 0 || len(failures) != 0 {
		t.Fatalf("expected attempted=2 failed=0 failures=0, got attempted=%d failed=%d failures=%v", attempted, failed, failures)
	}
	if len(called) != 2 || called[0] != "id-docs" || called[1] != "id-photos" {
		t.Fatalf("expected backupFn to receive the enabled sets' IDs [id-docs id-photos], got %v", called)
	}
}

func TestRunFilesJobContinuesOnError(t *testing.T) {
	var mu sync.Mutex
	var called []string

	backupFn := func(id string) error {
		mu.Lock()
		called = append(called, id)
		mu.Unlock()
		if id == "id-docs" {
			return errors.New("backup failed")
		}
		return nil
	}

	sets := []store.FileSet{
		{ID: "id-docs", Name: "docs", Enabled: true},
		{ID: "id-photos", Name: "photos", Enabled: true},
	}

	attempted, failed, failures := schedule.RunFilesJob(sets, backupFn)

	mu.Lock()
	defer mu.Unlock()

	if len(called) != 2 {
		t.Fatalf("expected 2 attempts, got %d: %v", len(called), called)
	}
	if attempted != 2 || failed != 1 {
		t.Fatalf("expected attempted=2 failed=1, got attempted=%d failed=%d", attempted, failed)
	}
	// Failures are reported by the set's name, not its ID.
	if len(failures) != 1 || failures[0].Name != "docs" || failures[0].Reason != "backup failed" {
		t.Fatalf("expected failures=[{docs backup failed}], got %+v", failures)
	}
}

func TestSchedulerReloadRegistersEnabledDomains(t *testing.T) {
	backupFn := func(_ string) error { return nil }
	listFn := func() ([]store.Target, error) { return nil, nil }

	sched := schedule.New(backupFn, listFn)

	settings := store.Settings{
		ContainersEnabled:  true,
		VMsEnabled:         true,
		FlashEnabled:       true,
		ContainersSchedule: "daily 03:00",
		VMsSchedule:        "off",
		FlashSchedule:      "off",
	}
	if err := sched.Reload(settings); err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}

	settings.ContainersSchedule = "off"
	if err := sched.Reload(settings); err != nil {
		t.Fatalf("second Reload returned error: %v", err)
	}

	sched.Stop()
}

func TestSchedulerReloadEveryN(t *testing.T) {
	backupFn := func(_ string) error { return nil }
	listFn := func() ([]store.Target, error) { return nil, nil }

	sched := schedule.New(backupFn, listFn)

	settings := store.Settings{
		ContainersEnabled:  true,
		VMsEnabled:         true,
		FlashEnabled:       true,
		ContainersSchedule: "everyN 7 04:00",
		VMsSchedule:        "off",
		FlashSchedule:      "off",
	}
	if err := sched.Reload(settings); err != nil {
		t.Fatalf("Reload with everyN returned error: %v", err)
	}
	sched.Stop()
}

func TestSchedulerReloadWithDueChecksEveryN(t *testing.T) {
	backupFn := func(_ string) error { return nil }
	listFn := func() ([]store.Target, error) { return nil, nil }
	lastRun := func() (time.Time, error) { return time.Now().Add(-6 * 24 * time.Hour), nil }

	sched := schedule.New(backupFn, listFn)

	settings := store.Settings{
		ContainersEnabled:  true,
		VMsEnabled:         true,
		FlashEnabled:       true,
		ContainersSchedule: "everyN 5 03:00",
		VMsSchedule:        "off",
		FlashSchedule:      "off",
	}
	if err := sched.ReloadWithDueChecks(settings, lastRun, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks returned error: %v", err)
	}
	sched.Stop()
}

func TestNextRunsReturnsEnabledEntries(t *testing.T) {
	backupFn := func(_ string) error { return nil }
	listFn := func() ([]store.Target, error) { return nil, nil }

	sched := schedule.New(backupFn, listFn)

	settings := store.Settings{
		ContainersEnabled:  true,
		VMsEnabled:         true,
		FlashEnabled:       true,
		ContainersSchedule: "daily 03:00",
		VMsSchedule:        "off",
		FlashSchedule:      "off",
	}
	if err := sched.Reload(settings); err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}
	sched.Start()
	defer sched.Stop()

	runs := sched.NextRuns()

	var found *schedule.NextRun
	for i := range runs {
		if runs[i].Job == "backup" && runs[i].Domain == "containers" {
			found = &runs[i]
		}
		if runs[i].Domain == "vms" || runs[i].Domain == "flash" {
			t.Fatalf("disabled domain %q must not appear in NextRuns, got %+v", runs[i].Domain, runs[i])
		}
	}
	if found == nil {
		t.Fatalf("expected a job=backup domain=containers entry in NextRuns, got %+v", runs)
	}
	if !found.Next.After(time.Now()) {
		t.Fatalf("expected Next to be in the future, got %v", found.Next)
	}
}

// TestSwitchedOffDomainIsNotRegistered checks that a configured cadence alone
// does not register a schedule; its domain has to be switched on too. That
// includes the off-site schedule, which for a switched-off domain would
// replicate snapshots nobody is making.
func TestSwitchedOffDomainIsNotRegistered(t *testing.T) {
	sched := schedule.New(func(string) error { return nil }, func() ([]store.Target, error) { return nil, nil })

	// Every domain carries a real cadence; only containers is switched on.
	settings := store.Settings{
		ContainersEnabled:         true,
		ContainersSchedule:        "daily 03:00",
		VMsEnabled:                false,
		VMsSchedule:               "daily 04:00",
		FlashEnabled:              false,
		FlashSchedule:             "daily 05:00",
		ConfigEnabled:             false,
		ConfigSchedule:            "daily 06:00",
		FilesEnabled:              false,
		FilesSchedule:             "daily 07:00",
		VMsOffsiteSchedule:        "daily 08:00",
		ContainersOffsiteSchedule: "daily 09:00",
	}
	if err := sched.Reload(settings); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	sched.Start()
	defer sched.Stop()

	var domains []string
	for _, r := range sched.NextRuns() {
		domains = append(domains, r.Domain)
	}
	for _, r := range sched.NextRuns() {
		switch r.Domain {
		case "vms", "flash", "config", "files":
			t.Fatalf("domain %q is switched off and must not be registered, got %+v (all: %v)", r.Domain, r, domains)
		}
	}
	var haveContainers, haveContainersOffsite bool
	for _, r := range sched.NextRuns() {
		if r.Domain == "containers" && r.Job == "backup" {
			haveContainers = true
		}
		if r.Domain == "containers" && r.Job == "offsite" {
			haveContainersOffsite = true
		}
	}
	if !haveContainers {
		t.Fatalf("containers is switched on and must still be registered, got %v", domains)
	}
	if !haveContainersOffsite {
		t.Fatalf("the off-site schedule of a switched-on domain must still be registered, got %v", domains)
	}
}

func TestNextRunsSortedSoonestFirst(t *testing.T) {
	backupFn := func(_ string) error { return nil }
	listFn := func() ([]store.Target, error) { return nil, nil }

	sched := schedule.New(backupFn, listFn)

	settings := store.Settings{
		ContainersEnabled:  true,
		VMsEnabled:         true,
		FlashEnabled:       true,
		ContainersSchedule: "daily 23:59",
		VMsSchedule:        "daily 00:01",
		FlashSchedule:      "off",
	}
	if err := sched.Reload(settings); err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}
	sched.Start()
	defer sched.Stop()

	runs := sched.NextRuns()
	if len(runs) < 2 {
		t.Fatalf("expected at least 2 entries, got %d: %+v", len(runs), runs)
	}
	for i := 1; i < len(runs); i++ {
		if runs[i].Next.Before(runs[i-1].Next) {
			t.Fatalf("NextRuns not sorted ascending by Next: %+v", runs)
		}
	}
}

func TestSchedulerStopReturnsWhileAJobRunsOutsideCron(t *testing.T) {
	const jobDuration = 80 * time.Millisecond

	started := make(chan struct{})
	finished := make(chan struct{})

	backupFn := func(_ string) error {
		close(started)
		time.Sleep(jobDuration)
		close(finished)
		return nil
	}

	targets := []store.Target{
		{ContainerName: "plex", IncludeInSchedule: true},
	}

	sched := schedule.New(backupFn, func() ([]store.Target, error) { return targets, nil })
	sched.Start()

	go schedule.RunContainersJob(targets, backupFn)

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for job to start")
	}

	sched.Stop()

	// Stop has no cron job of its own to wait for, so the job may still be
	// sleeping here.
	select {
	case <-finished:
	default:
	}
}

// A manual "back up this domain now" takes everything the operator protects and
// has not paused, which is a different question from the one the scheduler's
// domain pass asks.
func TestPausedByOverride(t *testing.T) {
	cases := []struct {
		override string
		perItem  bool
		want     bool
	}{
		{"off", true, true},
		{"", true, false},
		{"daily 02:00", true, false},
		{"everyN 3 02:00", true, false},
		{"nonsense", true, false},
		{"off", false, false},
		{"daily 02:00", false, false},
	}
	for _, c := range cases {
		if got := schedule.PausedByOverride(c.override, c.perItem); got != c.want {
			t.Errorf("PausedByOverride(%q, %v) = %v, want %v", c.override, c.perItem, got, c.want)
		}
	}
}
