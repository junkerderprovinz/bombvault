package api_test

// GET /api/coverage — what on this server is NOT backed up.
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

// TestCoverageNamesAContainerNobodyEverAdded is the whole point. A container
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
