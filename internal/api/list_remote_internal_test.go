package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// lastBackupOf finds a row by name in a list answer and returns its lastBackup.
func lastBackupOf(t *testing.T, res map[string]any, key, name string) any {
	t.Helper()
	rows, _ := res[key].([]any)
	for _, r := range rows {
		row, _ := r.(map[string]any)
		if row["name"] == name {
			return row["lastBackup"]
		}
	}
	t.Fatalf("%s has no row %s: %v", key, name, res[key])
	return nil
}

func TestTheFolderListReadsNoRemoteRepository(t *testing.T) {
	f := newPlacementFixture(t)
	const remote = "rest:http://rest.example:8000/files-direct"
	cloud := f.namedRepo("Cloud", remote)
	a := f.fileSet("A", "")
	e := f.fileSet("E", cloud.ID)
	f.hold(f.domainPath("files"), snap("a1", 1_700_000_100, "fileset:A"))
	f.hold(remote, snap("e1", 1_700_000_200, "fileset:E"))
	f.backupRun(a.ID, 1_700_000_100)
	f.backupRun(e.ID, 1_700_000_300)

	res := f.do(http.MethodGet, "/api/files", nil)
	if n := f.eng.lists[remote]; n != 0 {
		t.Fatalf("GET /api/files listed the remote repository %d times", n)
	}
	if got := lastBackupOf(t, res, "fileSets", "E"); got != float64(1_700_000_300) {
		t.Fatalf("E's last backup = %v, want the run's 1700000300", got)
	}
	if got := lastBackupOf(t, res, "fileSets", "A"); got != float64(1_700_000_100) {
		t.Fatalf("A's last backup = %v, want its snapshot's 1700000100", got)
	}
}

func TestAFolderListOnARemoteDomainPathDatesFromTheRuns(t *testing.T) {
	f := newPlacementFixture(t)
	const remote = "s3:https://s3.example/files"
	f.settings(func(s *store.Settings) { s.FilesPath = remote })
	a := f.fileSet("A", "")
	f.hold(remote, snap("a1", 1_700_000_100, "fileset:A"))
	f.backupRun(a.ID, 1_700_000_300)

	res := f.do(http.MethodGet, "/api/files", nil)
	if n := f.eng.lists[remote]; n != 0 {
		t.Fatalf("GET /api/files listed the remote domain path %d times", n)
	}
	if got := lastBackupOf(t, res, "fileSets", "A"); got != float64(1_700_000_300) {
		t.Fatalf("A's last backup = %v, want the run's 1700000300", got)
	}
}

func TestTheContainerAndVMListsReadNoRemoteRepository(t *testing.T) {
	f := newPlacementFixture(t)
	const remote = "b2:bucket:named"
	cloud := f.namedRepo("Cloud", remote)
	nginx := f.container("nginx", cloud.ID)
	f.dock.installed["nginx"] = true
	win := f.vm("win11", cloud.ID)
	f.virsh.defined = []string{"win11"}
	f.hold(remote, snap("n1", 1_700_000_100, "container:nginx"), snap("w1", 1_700_000_100, "vm:win11"))
	f.backupRun(nginx.ID, 1_700_000_300)
	f.backupRun(win.ID, 1_700_000_300)

	containers := f.do(http.MethodGet, "/api/containers", nil)
	vms := f.do(http.MethodGet, "/api/vms", nil)
	if n := f.eng.lists[remote]; n != 0 {
		t.Fatalf("the lists read the remote repository %d times", n)
	}
	if got := lastBackupOf(t, containers, "containers", "nginx"); got != float64(1_700_000_300) {
		t.Fatalf("nginx's last backup = %v, want the run's", got)
	}
	if got := lastBackupOf(t, vms, "vms", "win11"); got != float64(1_700_000_300) {
		t.Fatalf("win11's last backup = %v, want the run's", got)
	}
}

// homePlaceOf returns the local place of a row's observed placement.
func homePlaceOf(t *testing.T, res map[string]any, key, name string) map[string]any {
	t.Helper()
	rows, _ := res[key].([]any)
	for _, r := range rows {
		row, _ := r.(map[string]any)
		if row["name"] != name {
			continue
		}
		placement, _ := row["placement"].(map[string]any)
		observed, _ := placement["observed"].(map[string]any)
		places, _ := observed["places"].([]any)
		for _, p := range places {
			if place, _ := p.(map[string]any); place["place"] == "local" {
				return place
			}
		}
	}
	t.Fatalf("%s has no local place for %s: %v", key, name, res[key])
	return nil
}

func TestTheHomePlaceCountsTheBackupsTheListingFound(t *testing.T) {
	f := newPlacementFixture(t)
	const remote = "rest:http://rest.example:8000/files-direct"
	cloud := f.namedRepo("Cloud", remote)
	a := f.fileSet("A", "")
	e := f.fileSet("E", cloud.ID)
	f.hold(f.domainPath("files"),
		snap("a1", 1_700_000_100, "fileset:A"), snap("a2", 1_700_000_200, "fileset:A"), snap("a3", 1_700_000_300, "fileset:A"))
	f.backupRun(a.ID, 1_700_000_300)
	f.backupRun(e.ID, 1_700_000_300)

	res := f.do(http.MethodGet, "/api/files", nil)
	if got := homePlaceOf(t, res, "fileSets", "A")["count"]; got != float64(3) {
		t.Fatalf("A's home counts %v, want 3", got)
	}
	if got, ok := homePlaceOf(t, res, "fileSets", "E")["count"]; !ok || got != nil {
		t.Fatalf("E's home counts %v, want null: its remote home is not listed", got)
	}
}

func TestTheContainerAndVMHomePlacesCountTheirBackups(t *testing.T) {
	f := newPlacementFixture(t)
	nginx := f.container("nginx", "")
	f.dock.installed["nginx"] = true
	win := f.vm("win11", "")
	f.virsh.defined = []string{"win11"}
	f.hold(f.domainPath("containers"), snap("n1", 1_700_000_100, "container:nginx"), snap("n2", 1_700_000_200, "container:nginx"),
		snap("d1", 1_700_000_200, "dbdump:nginx"))
	f.hold(f.domainPath("vms"), snap("w1", 1_700_000_100, "vm:win11"))
	f.backupRun(nginx.ID, 1_700_000_200)
	f.backupRun(win.ID, 1_700_000_100)

	if got := homePlaceOf(t, f.do(http.MethodGet, "/api/containers", nil), "containers", "nginx")["count"]; got != float64(3) {
		t.Fatalf("nginx's home counts %v, want its two backups and its dump", got)
	}
	if got := homePlaceOf(t, f.do(http.MethodGet, "/api/vms", nil), "vms", "win11")["count"]; got != float64(1) {
		t.Fatalf("win11's home counts %v, want 1", got)
	}
}

func TestTheHomePlaceCountsOnlyTheBackupsAtTheHome(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	a := f.fileSet("A", "")
	f.fileSet("C", nas.ID)
	f.hold(f.domainPath("files"), snap("a1", 1_700_000_100, "fileset:A"), snap("a2", 1_700_000_200, "fileset:A"))
	f.hold(f.root+"/nas", snap("x1", 1_700_000_100, "fileset:A"), snap("c1", 1_700_000_100, "fileset:C"))
	f.backupRun(a.ID, 1_700_000_200)

	if got := homePlaceOf(t, f.do(http.MethodGet, "/api/files", nil), "fileSets", "A")["count"]; got != float64(2) {
		t.Fatalf("A's home counts %v, want the two backups in the domain path", got)
	}
}

func TestAHomeThatCouldNotBeListedCountsNull(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	a := f.fileSet("A", "")
	c := f.fileSet("C", nas.ID)
	f.hold(f.domainPath("files"), snap("a1", 1_700_000_100, "fileset:A"))
	f.hold(f.root+"/nas", snap("c1", 1_700_000_100, "fileset:C"))
	f.eng.listErr[f.domainPath("files")] = errors.New("locked")
	f.backupRun(a.ID, 1_700_000_100)
	f.backupRun(c.ID, 1_700_000_100)

	res := f.do(http.MethodGet, "/api/files", nil)
	if got, ok := homePlaceOf(t, res, "fileSets", "A")["count"]; !ok || got != nil {
		t.Fatalf("A's home counts %v, want null: its home could not be listed", got)
	}
	if got := homePlaceOf(t, res, "fileSets", "C")["count"]; got != float64(1) {
		t.Fatalf("C's home counts %v, want 1", got)
	}
}
