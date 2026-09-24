package api_test

// GET /api/coverage: what on this server nothing backs up.
//
// The question the dashboard could not answer: it showed a traffic light per
// domain, which says whether the containers that ARE scheduled ran on time. It
// said nothing about the container nobody ever added. That is the gap an
// operator actually falls into, because a container that was never set up looks
// exactly like one that is fine: absent from every list, absent from every
// error.

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// coverageNames pulls the unprotected item names out of the response, across
// every domain, so a test can assert on "who is unprotected" without walking
// the envelope by hand.
func coverageNames(t *testing.T, m map[string]any) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	report, _ := m["coverage"].(map[string]any)
	domains, _ := report["domains"].([]any)
	for _, d := range domains {
		dm, _ := d.(map[string]any)
		items, _ := dm["unprotected"].([]any)
		for _, it := range items {
			im, _ := it.(map[string]any)
			if name, ok := im["name"].(string); ok {
				out[name] = true
			}
		}
	}
	return out
}

// TestCoverageNamesAContainerNobodyEverAdded is the core case. A container
// running on the host with no target row has no include flag, no schedule and
// no backup: there is no row to hold any of them. Reading only the stored rows
// would miss exactly the item that most needs naming.
func TestCoverageNamesAContainerNobodyEverAdded(t *testing.T) {
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{
		{Name: "plex"},
		{Name: "sonarr"},
	}}
	h, st, _ := newTestRouterSvc(t, d, &fakeResticEngine{})

	// plex is set up and scheduled; sonarr was never added at all.
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersEnabled = true
	s.ContainersSchedule = "daily 02:00"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	w, m := doJSON(t, h, http.MethodGet, "/api/coverage", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if m["ok"] != true {
		t.Fatalf("expected ok, got %v", m)
	}

	names := coverageNames(t, m)
	if !names["sonarr"] {
		t.Fatalf("a container that was never added must be reported as unprotected, got %v", names)
	}
	if names["plex"] {
		t.Fatalf("a scheduled container must not be reported as unprotected, got %v", names)
	}
}

// TestCoverageNamesAnExcludedContainer: the include toggle reads like "skipped
// by the domain schedule" and actually means "nothing backs this up", which is
// the misunderstanding the file-set helper was built for. It has to show here.
func TestCoverageNamesAnExcludedContainer(t *testing.T) {
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{{Name: "plex"}}}
	h, st, _ := newTestRouterSvc(t, d, &fakeResticEngine{})

	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex", IncludeInSchedule: false}); err != nil {
		t.Fatal(err)
	}
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersEnabled = true
	s.ContainersSchedule = "daily 02:00"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	_, m := doJSON(t, h, http.MethodGet, "/api/coverage", "")
	if !coverageNames(t, m)["plex"] {
		t.Fatalf("a container excluded from the schedule is not backed up by anything and must be named")
	}
}

// TestCoverageIgnoresBombVaultsOwnContainer: BombVault never backs itself up
// through the normal container path, so listing it as unprotected would be a
// permanent false alarm nobody can clear.
func TestCoverageIgnoresBombVaultsOwnContainer(t *testing.T) {
	d := &fakeServiceDocker{
		listOut:  []dockercli.ContainerInfo{{Name: "BombVault"}, {Name: "plex"}},
		selfName: "BombVault",
	}
	h, st, _ := newTestRouterSvc(t, d, &fakeResticEngine{})

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersEnabled = true
	s.ContainersSchedule = "daily 02:00"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	names := coverageNames(t, mustCoverage(t, h))
	if names["BombVault"] {
		t.Fatalf("BombVault's own container must never be reported as unprotected, got %v", names)
	}
	if !names["plex"] {
		t.Fatalf("the ordinary container should still be reported, got %v", names)
	}
}

// TestCoverageReportsADisabledDomainAsOffRatherThanUnprotected: switching a
// domain off is a decision, not a failure. Counting every VM as unprotected
// because the operator does not use VMs would make the card cry wolf on a
// correctly configured server, and a card that cries wolf gets hidden.
func TestCoverageReportsADisabledDomainAsOffRatherThanUnprotected(t *testing.T) {
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{{Name: "plex"}}}
	h, st, _ := newTestRouterSvc(t, d, &fakeResticEngine{})

	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "win11", IncludeInSchedule: false}); err != nil {
		t.Fatal(err)
	}
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersEnabled = true
	s.ContainersSchedule = "daily 02:00"
	s.VMsEnabled = false // the operator does not back up VMs
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	m := mustCoverage(t, h)
	if coverageNames(t, m)["win11"] {
		t.Fatalf("a VM in a switched-off domain must not be counted as unprotected")
	}

	report, _ := m["coverage"].(map[string]any)
	domains, _ := report["domains"].([]any)
	var sawVMsOff bool
	for _, dd := range domains {
		dm, _ := dd.(map[string]any)
		if dm["domain"] == "vms" {
			if dm["enabled"] == false {
				sawVMsOff = true
			}
		}
	}
	if !sawVMsOff {
		t.Fatalf("the report must still name the VM domain and say it is off: %v", domains)
	}
}

func mustCoverage(t *testing.T, h http.Handler) map[string]any {
	t.Helper()
	w, m := doJSON(t, h, http.MethodGet, "/api/coverage", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	return m
}

// coverageReasons pulls the reason of every unprotected item, keyed by name.
func coverageReasons(t *testing.T, m map[string]any) map[string]string {
	t.Helper()
	out := map[string]string{}
	report, _ := m["coverage"].(map[string]any)
	domains, _ := report["domains"].([]any)
	for _, d := range domains {
		dm, _ := d.(map[string]any)
		items, _ := dm["unprotected"].([]any)
		for _, it := range items {
			im, _ := it.(map[string]any)
			name, _ := im["name"].(string)
			reason, _ := im["reason"].(string)
			out[name] = reason
		}
	}
	return out
}

// TestCoverageNamesDatabaseDumpGaps: a database whose dump is the only
// consistent copy is unprotected in a way no schedule reports. An acknowledged
// failure clears the dashboard badge and leaves the database without a fresh
// dump, so the card has to keep saying so.
func TestCoverageNamesDatabaseDumpGaps(t *testing.T) {
	stackLabels := map[string]string{
		"com.docker.compose.project":             "immich",
		"com.docker.compose.project.working_dir": "/mnt/user/stacks/immich",
	}
	pgMount := []model.Mount{{Type: "bind", Source: "/mnt/user/stacks/immich/pgdata", Destination: "/var/lib/postgresql/data"}}

	d := &fakeServiceDocker{
		listOut: []dockercli.ContainerInfo{
			{Name: "failing_db", Image: "postgres:16"},
			{Name: "switched_off_db", Image: "postgres:16"},
			{Name: "unscheduled_db", Image: "postgres:16"},
			{Name: "healthy_db", Image: "postgres:16"},
		},
		inspects: map[string]model.Inspect{
			"failing_db": {Running: true, Config: model.Config{
				Image: "postgres:16", Env: []string{"POSTGRES_PASSWORD=x"}, Labels: stackLabels,
			}, Mounts: pgMount},
			"switched_off_db": {Running: true, Config: model.Config{
				Image: "postgres:16", Env: []string{"POSTGRES_PASSWORD=x"}, Labels: stackLabels,
			}, Mounts: pgMount},
			"unscheduled_db": {Running: true, Config: model.Config{
				Image: "postgres:16", Env: []string{"POSTGRES_PASSWORD=x"},
			}},
			"healthy_db": {Running: true, Config: model.Config{
				Image: "postgres:16", Env: []string{"POSTGRES_PASSWORD=x"}, Labels: stackLabels,
			}, Mounts: pgMount},
		},
	}
	h, st := dbFieldsRouterHarness(t, d)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersEnabled = true
	s.ContainersSchedule = "daily 02:00"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"failing_db", "switched_off_db", "healthy_db"} {
		if _, err := st.UpsertTarget(store.Target{ContainerName: name, IncludeInSchedule: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "unscheduled_db", IncludeInSchedule: false}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetDBDumpOff("switched_off_db", true); err != nil {
		t.Fatal(err)
	}

	failing, err := st.GetTargetByContainer("failing_db")
	if err != nil {
		t.Fatal(err)
	}
	runID, err := st.StartRun(failing.ID, "dbdump")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(runID, "failed", "", 0, "database dump failed: no progress"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AcknowledgeRuns([]string{runID}); err != nil {
		t.Fatal(err)
	}
	healthy, err := st.GetTargetByContainer("healthy_db")
	if err != nil {
		t.Fatal(err)
	}
	okRun, err := st.StartRun(healthy.ID, "dbdump")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(okRun, "success", "aaaa1111", 42, ""); err != nil {
		t.Fatal(err)
	}

	_, m := doJSON(t, h, http.MethodGet, "/api/coverage", "")
	if m["ok"] != true {
		t.Fatalf("coverage must succeed, got %v", m)
	}
	reasons := coverageReasons(t, m)

	if reasons["failing_db"] != "db-dump-failing" {
		t.Errorf("failing_db reason = %q, want db-dump-failing", reasons["failing_db"])
	}
	if reasons["switched_off_db"] != "db-dump-only-copy-off" {
		t.Errorf("switched_off_db reason = %q, want db-dump-only-copy-off", reasons["switched_off_db"])
	}
	if reasons["unscheduled_db"] != "db-not-scheduled" {
		t.Errorf("unscheduled_db reason = %q, want db-not-scheduled", reasons["unscheduled_db"])
	}
	if got, listed := reasons["healthy_db"]; listed {
		t.Errorf("a scheduled database with a fresh dump must not be listed, got %q", got)
	}
}

// TestCoverageLeavesADumpingDatabaseAloneWhenTheFilesAreConsistent: a dump that
// is switched off matters because the files copy is taken while the server
// runs. Where the data folder is inside the container's own backup, the switch
// is a decision, not a gap.
func TestCoverageLeavesADumpingDatabaseAloneWhenTheFilesAreConsistent(t *testing.T) {
	d := &fakeServiceDocker{
		listOut: []dockercli.ContainerInfo{{Name: "immich_postgres", Image: "postgres:16"}},
		inspects: map[string]model.Inspect{
			"immich_postgres": {Running: true, Config: model.Config{
				Image: "postgres:16", Env: []string{"POSTGRES_PASSWORD=x"},
			}, Mounts: []model.Mount{
				{Type: "bind", Source: "/mnt/user/appdata/immich_postgres", Destination: "/var/lib/postgresql/data"},
			}},
		},
	}
	h, st := dbFieldsRouterHarness(t, d)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersEnabled = true
	s.ContainersSchedule = "daily 02:00"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "immich_postgres", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetDBDumpOff("immich_postgres", true); err != nil {
		t.Fatal(err)
	}

	_, m := doJSON(t, h, http.MethodGet, "/api/coverage", "")
	if got, listed := coverageReasons(t, m)["immich_postgres"]; listed {
		t.Fatalf("reason = %q, want no entry: the files backup stops the server and copies its data", got)
	}
}

// A ZFS item with no schedule of its own is protected once the Backup
// Everything pass runs, because the pass backs up ZFS items like the other
// types; an item left out of the schedule stays named.
func TestCoverageCountsZFSItemsBackupEverythingReaches(t *testing.T) {
	h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ZFSEnabled = true
	s.ZFSSchedule = "off"
	s.EverythingSchedule = "daily 05:00"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "tank/media", Enabled: false}); err != nil {
		t.Fatal(err)
	}

	m := mustCoverage(t, h)
	reasons := coverageReasons(t, m)
	if reason, named := reasons["cache/appdata"]; named {
		t.Errorf("an item the Everything pass backs up is reported as %q", reason)
	}
	if reasons["tank/media"] != "not-included" {
		t.Errorf("an item left out of the schedule reads %q, want not-included", reasons["tank/media"])
	}

	report, _ := m["coverage"].(map[string]any)
	domains, _ := report["domains"].([]any)
	for _, d := range domains {
		dm, _ := d.(map[string]any)
		if dm["domain"] != "zfs" {
			continue
		}
		if dm["total"] != float64(2) || dm["protected"] != float64(1) {
			t.Fatalf("the ZFS domain counts %v of %v protected, want 1 of 2", dm["protected"], dm["total"])
		}
		return
	}
	t.Fatalf("the report has no ZFS domain: %v", domains)
}
