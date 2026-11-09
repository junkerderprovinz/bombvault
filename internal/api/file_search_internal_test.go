package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func foundAt(p string, size int64) restic.FoundFile {
	return restic.FoundFile{Path: p, Type: "file", Size: size, ModTime: time.Unix(1_757_000_000, 0).UTC()}
}

func (f *placementFixture) holdsFiles(snapshotID string, files ...restic.FoundFile) {
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	if f.eng.found == nil {
		f.eng.found = map[string][]restic.FoundFile{}
	}
	f.eng.found[snapshotID] = files
}

func (f *placementFixture) search(text string, offsite bool, items ...fileSearchItem) fileSearch {
	f.t.Helper()
	res, err := f.svc.SearchFiles(context.Background(), items, text, offsite)
	if err != nil {
		f.t.Fatal(err)
	}
	return res
}

func placeIn(t *testing.T, res fileSearch, label string) searchedPlace {
	t.Helper()
	for _, p := range res.Places {
		if p.Label == label {
			return p
		}
	}
	t.Fatalf("no place %q in %+v", label, res.Places)
	return searchedPlace{}
}

func versionRows(file foundFile) []string {
	rows := make([]string, 0, len(file.Versions))
	for _, v := range file.Versions {
		rows = append(rows, v.Row)
	}
	return rows
}

func TestFileSearchListsEveryBackupThatHoldsTheFile(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.hold(f.domainPath("containers"),
		snap("a1a1a1a1", 1_758_000_000, "container:nginx"),
		snap("a2a2a2a2", 1_759_000_000, "container:nginx"),
		snap("a3a3a3a3", 1_760_000_000, "container:nginx"))
	f.holdsFiles("a1a1a1a1", foundAt("/appdata/nginx/nginx.conf", 10))
	f.holdsFiles("a3a3a3a3", foundAt("/appdata/nginx/nginx.conf", 30), foundAt("/appdata/nginx/mime.types", 5))

	res := f.search("NGINX.conf", false, fileSearchItem{"containers", "nginx"})

	if len(res.Files) != 1 || res.More {
		t.Fatalf("files = %+v, want nginx.conf alone", res.Files)
	}
	file := res.Files[0]
	if file.Domain != "containers" || file.Key != "nginx" || file.Path != "/appdata/nginx/nginx.conf" || file.Type != "file" {
		t.Fatalf("file = %+v", file)
	}
	if got := versionRows(file); !slices.Equal(got, []string{"a3a3a3a3", "a1a1a1a1"}) {
		t.Fatalf("versions = %v, want the two backups holding it, newest first", got)
	}
	if v := file.Versions[0]; v.Place != "local" || v.Snapshot != "a3a3a3a3" || v.Size != 30 || v.Time != time.Unix(1_760_000_000, 0).UTC().Format(time.RFC3339) {
		t.Fatalf("newest version = %+v", v)
	}
	if home := placeIn(t, res, ""); home.State != "searched" || home.Backups != 3 || home.Domain != "containers" || home.Remote {
		t.Fatalf("home = %+v", home)
	}
	if len(f.eng.finds) != 1 || f.eng.finds[0].Pattern != "*nginx.conf*" {
		t.Fatalf("finds = %+v, want one over the home", f.eng.finds)
	}
}

func TestFileSearchLeavesRemotePlacesUnreadUntilAsked(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	f.hold("b2:bucket:containers",
		copied("b1b1b1b1", "a1a1a1a1", 1_758_000_000, "container:nginx"),
		snap("b0b0b0b0", 1_700_000_000, "container:nginx"))
	f.holdsFiles("a1a1a1a1", foundAt("/appdata/nginx/nginx.conf", 10))
	f.holdsFiles("b0b0b0b0", foundAt("/appdata/nginx/nginx.conf", 7))
	item := fileSearchItem{"containers", "nginx"}

	res := f.search("nginx.conf", false, item)
	if p := placeIn(t, res, "B2"); p.State != "notAsked" || !p.Remote {
		t.Fatalf("B2 = %+v, want left alone", p)
	}
	if n := f.eng.lists["b2:bucket:containers"]; n != 0 {
		t.Fatalf("the search listed B2 %d times without being asked", n)
	}
	if got := versionRows(res.Files[0]); !slices.Equal(got, []string{"a1a1a1a1"}) {
		t.Fatalf("versions = %v, want the home's backup alone", got)
	}

	f.eng.finds = nil
	res = f.search("nginx.conf", true, item)
	if p := placeIn(t, res, "B2"); p.State != "searched" || p.Backups != 1 {
		t.Fatalf("B2 = %+v, want searched for the one backup only it holds", p)
	}
	b2Find := f.eng.finds[1]
	if b2Find.Repo != "b2:bucket:containers" || !slices.Equal(b2Find.IDs, []string{"b0b0b0b0"}) {
		t.Fatalf("B2 find = %+v, want only the backup the home lacks", b2Find)
	}
	if got := versionRows(res.Files[0]); !slices.Equal(got, []string{"a1a1a1a1", "b0b0b0b0"}) {
		t.Fatalf("versions = %v", got)
	}
	if v := res.Files[0].Versions[1]; v.Place != "offsite:"+b2.ID || v.Snapshot != "b0b0b0b0" || res.Places[v.At].Label != "B2" {
		t.Fatalf("B2 version = %+v", v)
	}
}

func TestFileSearchSkipsAPlaceHoldingOnlyWhatWasSearchedAlready(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	nasLoc := f.localRepo("copies/containers")
	f.target("containers", "NAS copy", "copies/containers")
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	f.hold(nasLoc, copied("c1c1c1c1", "a1a1a1a1", 1_758_000_000, "container:nginx"))

	res := f.search("nginx.conf", false, fileSearchItem{"containers", "nginx"})

	if nas := placeIn(t, res, "NAS copy"); nas.State != "covered" || nas.Backups != 0 {
		t.Fatalf("NAS copy = %+v, want covered by the home", nas)
	}
	for _, call := range f.eng.finds {
		if call.Repo == nasLoc {
			t.Fatalf("the search read the NAS copy: %+v", call)
		}
	}
}

func TestFileSearchReportsAPlaceItCouldNotRead(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	set := f.fileSet("photos", "")
	f.eng.listErr[f.domainPath("containers")] = errors.New("wrong password or no key found")
	f.hold(f.domainPath("files"), snap("f1f1f1f1", 1_758_000_000, "fileset:photos"))
	f.holdsFiles("f1f1f1f1", foundAt("/photos/nginx.conf.jpg", 1))

	res := f.search("nginx.conf", false, fileSearchItem{"containers", "nginx"}, fileSearchItem{"files", set.ID})

	failed := 0
	for _, p := range res.Places {
		if p.State == "failed" {
			failed++
			if p.Domain != "containers" || p.Error == "" {
				t.Errorf("failed place = %+v, want the containers folder with its reason", p)
			}
		}
	}
	if failed != 1 || len(res.Files) != 1 || res.Files[0].Domain != "files" {
		t.Fatalf("result = %+v, want the folder set searched beside the failed place", res)
	}
}

func TestFileSearchReportsAFailedSearchPerPlace(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	f.eng.findErr = map[string]error{f.domainPath("containers"): errors.New("unable to load tree")}

	res := f.search("nginx.conf", false, fileSearchItem{"containers", "nginx"})

	if home := placeIn(t, res, ""); home.State != "failed" || home.Error == "" {
		t.Fatalf("home = %+v, want failed with the reason", home)
	}
}

func TestFileSearchLeavesDatabaseDumpsOut(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("postgres", "")
	f.hold(f.domainPath("containers"),
		snap("a1a1a1a1", 1_758_000_000, "container:postgres"),
		snap("d1d1d1d1", 1_758_000_000, "dbdump:postgres", "dbengine:postgres"))

	f.search("dump.sql", false, fileSearchItem{"containers", "postgres"})

	if len(f.eng.finds) != 1 || !slices.Equal(f.eng.finds[0].IDs, []string{"a1a1a1a1"}) {
		t.Fatalf("finds = %+v, want the volume backup alone", f.eng.finds)
	}
}

func TestFileSearchNamesTheDatasetOfAZFSHit(t *testing.T) {
	f := newPlacementFixture(t)
	tank, home := f.zfsItem("tank/apps")
	f.hold(home, zfsSnap("a1a1a1a1", "tank/apps", firstRun), zfsSnap("a2a2a2a2", "tank/apps/db", firstRun))
	f.zfsRun(tank.ID, firstRun, "tank/apps", "tank/apps/db")
	f.holdsFiles("a2a2a2a2", foundAt("/pg_hba.conf", 3))

	res := f.search("pg_hba", false, fileSearchItem{zfsDomain, tank.ID})

	if len(res.Files) != 1 {
		t.Fatalf("files = %+v", res.Files)
	}
	file := res.Files[0]
	if file.Dataset != "tank/apps/db" || file.Path != "/pg_hba.conf" || file.Versions[0].Row != firstRun || file.Versions[0].Snapshot != "a2a2a2a2" {
		t.Fatalf("file = %+v, want the dataset, the run's stamp and the member snapshot", file)
	}
	if !slices.Equal(f.eng.finds[0].IDs, []string{"a1a1a1a1", "a2a2a2a2"}) {
		t.Fatalf("find ids = %v, want every dataset of the run", f.eng.finds[0].IDs)
	}
}

func TestFileSearchStopsTakingNewFilesAtTheCap(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	files := make([]restic.FoundFile, 0, maxFoundFiles+1)
	for i := range maxFoundFiles + 1 {
		files = append(files, foundAt(fmt.Sprintf("/appdata/log-%03d.txt", i), 1))
	}
	f.holdsFiles("a1a1a1a1", files...)

	res := f.search("log", false, fileSearchItem{"containers", "nginx"})

	if len(res.Files) != maxFoundFiles || !res.More {
		t.Fatalf("%d files, more %v, want %d and more", len(res.Files), res.More, maxFoundFiles)
	}
}

func TestFileSearchRouteRefusesAPathAndSearchesAName(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	f.holdsFiles("a1a1a1a1", foundAt("/appdata/nginx/nginx.conf", 10))

	code, out := f.call(http.MethodGet, "/api/search/files?q=etc%2Fpasswd", nil)
	if code != http.StatusBadRequest || out["code"] != "bad-file-name" {
		t.Fatalf("a path = %d %v, want 400 bad-file-name", code, out)
	}
	m := f.do(http.MethodGet, "/api/search/files?q=nginx.conf", nil)
	files, _ := m["files"].([]any)
	if m["ok"] != true || len(files) != 1 || files[0].(map[string]any)["key"] != "nginx" {
		t.Fatalf("search = %v", m)
	}
}

func TestFileSearchRouteIsOpenToAPairedPhone(t *testing.T) {
	if !pairedRoutes["GET /api/search/files"] {
		t.Fatal("a paired phone cannot search for a file it can restore")
	}
}
