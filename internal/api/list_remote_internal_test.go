package api

import (
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
