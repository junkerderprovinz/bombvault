package api_test

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// declineBackup sends PUT /api/items/{kind}/{key}/declined and returns the
// answer.
func declineBackup(t *testing.T, h http.Handler, kind, key string, declined bool) (*http.Response, map[string]any) {
	t.Helper()
	body := `{"declined":false}`
	if declined {
		body = `{"declined":true}`
	}
	w, m := doJSON(t, h, http.MethodPut, "/api/items/"+kind+"/"+key+"/declined", body)
	return w.Result(), m
}

// coverageDomain reads one domain of GET /api/coverage.
func coverageDomain(t *testing.T, h http.Handler, domain string) map[string]any {
	t.Helper()
	_, m := doJSON(t, h, http.MethodGet, "/api/coverage", "")
	report, _ := m["coverage"].(map[string]any)
	domains, _ := report["domains"].([]any)
	for _, raw := range domains {
		if d, _ := raw.(map[string]any); d["domain"] == domain {
			return d
		}
	}
	t.Fatalf("coverage has no domain %q: %v", domain, m)
	return nil
}

func TestADeclinedEntryStopsCountingAsUnprotected(t *testing.T) {
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{runningContainer("plex"), runningContainer("sonarr")}}
	h, st, _ := newItemsRouter(t, docker, fakeVirsh{})
	enableEveryKind(t, st, nil)
	docs, err := st.CreateFileSet(store.FileSet{Name: "Documents", Path: "documents"})
	if err != nil {
		t.Fatal(err)
	}

	before := coverageDomain(t, h, "containers")
	if before["total"] != float64(2) || len(before["unprotected"].([]any)) != 2 || before["declined"] != float64(0) {
		t.Fatalf("two containers nobody added read as %v", before)
	}

	for _, entry := range [][2]string{{"container", "sonarr"}, {"files", docs.ID}, {"flash", "flash"}} {
		res, m := declineBackup(t, h, entry[0], entry[1], true)
		item, _ := m["item"].(map[string]any)
		if res.StatusCode != http.StatusOK || m["ok"] != true || item["backupDeclined"] != true {
			t.Fatalf("declining %v: %d %v", entry, res.StatusCode, m)
		}
	}

	containers := coverageDomain(t, h, "containers")
	if containers["total"] != float64(1) || containers["declined"] != float64(1) {
		t.Fatalf("with sonarr declined the containers read as %v", containers)
	}
	if names := coverageNames(t, map[string]any{"coverage": map[string]any{"domains": []any{containers}}}); names["sonarr"] || !names["plex"] {
		t.Fatalf("unprotected containers = %v, want plex alone", names)
	}
	if files := coverageDomain(t, h, "files"); files["total"] != float64(0) || files["declined"] != float64(1) {
		t.Fatalf("with its only set declined the folder sets read as %v", files)
	}

	rows, _ := listItems(t, h)
	for id, want := range map[string]any{"container/sonarr": true, "files/" + docs.ID: true, "flash/flash": true, "container/plex": nil} {
		if got := rows[id]["backupDeclined"]; got != want {
			t.Fatalf("%s lists backupDeclined = %v, want %v", id, got, want)
		}
	}

	if res, m := declineBackup(t, h, "container", "sonarr", false); res.StatusCode != http.StatusOK || m["ok"] != true {
		t.Fatalf("taking it back: %d %v", res.StatusCode, m)
	}
	if after := coverageDomain(t, h, "containers"); after["total"] != float64(2) || after["declined"] != float64(0) {
		t.Fatalf("after taking it back the containers read as %v", after)
	}
}

func TestAScheduledOrUnknownEntryCannotBeDeclined(t *testing.T) {
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{runningContainer("plex")}}
	h, st, _ := newItemsRouter(t, docker, fakeVirsh{})
	enableEveryKind(t, st, func(s *store.Settings) { s.ContainersSchedule = "daily 02:00" })
	seedTarget(t, st, "plex")

	res, m := declineBackup(t, h, "container", "plex", true)
	if res.StatusCode != http.StatusOK || m["ok"] != false || m["code"] != "scheduled" {
		t.Fatalf("declining a scheduled container: %d %v, want the code scheduled", res.StatusCode, m)
	}
	if res, m := declineBackup(t, h, "container", "nobody", true); res.StatusCode != http.StatusNotFound || m["code"] != "not-found" {
		t.Fatalf("declining an unknown container: %d %v, want 404", res.StatusCode, m)
	}
	marks, err := st.DeclinedBackups()
	if err != nil || len(marks) != 0 {
		t.Fatalf("refused requests left the marks %v, %v", marks, err)
	}
}

func TestAScheduleOutranksADeclineWithoutReadingWritingAnything(t *testing.T) {
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{runningContainer("plex")}}
	h, st, _ := newItemsRouter(t, docker, fakeVirsh{})
	enableEveryKind(t, st, nil)
	seedTarget(t, st, "plex")
	if res, m := declineBackup(t, h, "container", "plex", true); res.StatusCode != http.StatusOK || m["ok"] != true {
		t.Fatalf("declining plex: %d %v", res.StatusCode, m)
	}

	setSchedule := func(spec string) {
		t.Helper()
		s := mustSettings(t, st)
		s.ContainersSchedule = spec
		if err := st.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
	}
	setSchedule("daily 02:00")
	if d := coverageDomain(t, h, "containers"); d["protected"] != float64(1) || d["declined"] != float64(0) {
		t.Fatalf("a scheduled container that was declined before reads as %v", d)
	}
	if item := getItem(t, h, "container", "plex"); item["backupDeclined"] != nil {
		t.Fatalf("the scheduled row carries backupDeclined: %v", item)
	}
	if marks, err := st.DeclinedBackups(); err != nil || !marks[store.BackupEntry{Kind: "container", Key: "plex"}] {
		t.Fatalf("reading the list dropped the mark: %v, %v", marks, err)
	}
	setSchedule("")
	if d := coverageDomain(t, h, "containers"); d["declined"] != float64(1) || len(d["unprotected"].([]any)) != 0 {
		t.Fatalf("taken out of the schedule before any backup the container reads as %v, want it declined", d)
	}
}

func TestABackupEndsADecline(t *testing.T) {
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{runningContainer("plex")}}
	h, st, _ := newItemsRouter(t, docker, fakeVirsh{})
	enableEveryKind(t, st, nil)
	plex, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	if res, m := declineBackup(t, h, "container", "plex", true); res.StatusCode != http.StatusOK || m["ok"] != true {
		t.Fatalf("declining plex: %d %v", res.StatusCode, m)
	}

	seedRun(t, st, plex.ID, "backup", "failed", store.RunMeta{})
	if item := getItem(t, h, "container", "plex"); item["backupDeclined"] != true {
		t.Fatalf("a failed backup ended the decline: %v", item)
	}
	seedRun(t, st, plex.ID, "backup", "success", store.RunMeta{})
	if item := getItem(t, h, "container", "plex"); item["backupDeclined"] != nil {
		t.Fatalf("a successful backup left the decline in place: %v", item)
	}
	if d := coverageDomain(t, h, "containers"); d["declined"] != float64(0) || len(d["unprotected"].([]any)) != 1 {
		t.Fatalf("a backed-up container without a schedule reads as %v, want it unprotected", d)
	}
}
