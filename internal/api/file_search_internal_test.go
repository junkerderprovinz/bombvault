package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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

// callSearchFiles calls search_files once, the way the transport would.
func (f *placementFixture) callSearchFiles(args string) *mcp.CallToolResult {
	f.t.Helper()
	res, err := f.h.toolSearchFiles(mcpStartCaller("0b7e", false), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name: "search_files", Arguments: json.RawMessage(args),
	}})
	if err != nil {
		f.t.Fatal(err)
	}
	return res
}

// searchFilesUntilDone calls search_files the way an assistant does, again and
// again while the search runs.
func (f *placementFixture) searchFilesUntilDone(args string) map[string]any {
	f.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		out := mcpStructured(f.t, f.callSearchFiles(args))
		if out["state"] != "running" {
			return out
		}
		if time.Now().After(deadline) {
			f.t.Fatal("the search never finished")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func foundFilesOf(t *testing.T, out map[string]any) []map[string]any {
	t.Helper()
	list, ok := out["files"].([]any)
	if !ok {
		t.Fatalf("files = %v, want a list", out["files"])
	}
	files := make([]map[string]any, 0, len(list))
	for _, f := range list {
		files = append(files, f.(map[string]any))
	}
	return files
}

func repositoryNamed(t *testing.T, out map[string]any, name string) map[string]any {
	t.Helper()
	list, _ := out["repositories"].([]any)
	for _, r := range list {
		if repo := r.(map[string]any); repo["name"] == name {
			return repo
		}
	}
	t.Fatalf("no repository %q in %v", name, out["repositories"])
	return nil
}

func TestMCPFileSearchHandsTheResultOverOnALaterCall(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	older := snap("a1a1a1a1", 1_758_000_000, "container:nginx")
	newer := snap("a2a2a2a2", 1_759_000_000, "container:nginx")
	older.Paths = []string{"/host/user/appdata/nginx"}
	newer.Paths = older.Paths
	f.hold(f.domainPath("containers"), older, newer)
	f.holdsFiles("a1a1a1a1", foundAt("/host/user/appdata/nginx/conf/nginx.conf", 10))
	f.holdsFiles("a2a2a2a2", foundAt("/host/user/appdata/nginx/conf/nginx.conf", 30))
	args := `{"name":"nginx.conf"}`

	if first := mcpStructured(t, f.callSearchFiles(args)); first["state"] != "running" {
		t.Fatalf("first call = %v, want the search started", first)
	}
	out := f.searchFilesUntilDone(args)

	if out["state"] != "ready" || out["truncated"] != false {
		t.Fatalf("result = %v", out)
	}
	files := foundFilesOf(t, out)
	if len(files) != 1 {
		t.Fatalf("files = %v, want nginx.conf alone", files)
	}
	file := files[0]
	if file["domain"] != "containers" || file["item"] != "nginx" || file["path"] != "conf/nginx.conf" || file["type"] != "file" {
		t.Fatalf("file = %v, want its path inside the backup", file)
	}
	versions, _ := file["versions"].([]any)
	if len(versions) != 2 {
		t.Fatalf("versions = %v, want both backups", versions)
	}
	newest := versions[0].(map[string]any)
	if newest["size"] != float64(30) || newest["snapshot"] != "a2a2a2a2" || newest["repository"] != "folder containers" ||
		newest["time"] != time.Unix(1_759_000_000, 0).UTC().Format(time.RFC3339) {
		t.Fatalf("newest version = %v", newest)
	}
	if home := repositoryNamed(t, out, "folder containers"); home["state"] != "searched" || home["backups"] != float64(2) || home["remote"] != false {
		t.Fatalf("home = %v", home)
	}
	if raw, _ := json.Marshal(out); strings.Contains(string(raw), "/host") {
		t.Fatalf("a host path reached the assistant: %s", raw)
	}
	if again := mcpStructured(t, f.callSearchFiles(args)); again["state"] != "running" {
		t.Fatalf("a call after the handover = %v, want a new search", again)
	}
	f.searchFilesUntilDone(args)
}

func TestMCPFileSearchLeavesRemoteRepositoriesOutUnlessAsked(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	f.hold("b2:bucket:containers", snap("b0b0b0b0", 1_700_000_000, "container:nginx"))
	f.holdsFiles("b0b0b0b0", foundAt("/appdata/nginx/nginx.conf", 7))

	out := f.searchFilesUntilDone(`{"name":"nginx.conf"}`)
	if b2 := repositoryNamed(t, out, "B2"); b2["state"] != "notAsked" || b2["remote"] != true {
		t.Fatalf("B2 = %v, want left alone", b2)
	}
	if files := foundFilesOf(t, out); len(files) != 0 {
		t.Fatalf("files = %v, want none from the home", files)
	}

	out = f.searchFilesUntilDone(`{"name":"nginx.conf","offsite":true}`)
	if b2 := repositoryNamed(t, out, "B2"); b2["state"] != "searched" {
		t.Fatalf("B2 = %v, want searched", b2)
	}
	if files := foundFilesOf(t, out); len(files) != 1 {
		t.Fatalf("files = %v, want the one B2 holds", files)
	}
}

func TestMCPFileSearchNarrowsToADomainOrOneItem(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.container("caddy", "")
	f.fileSet("configs", "")
	f.hold(f.domainPath("containers"),
		snap("a1a1a1a1", 1_758_000_000, "container:nginx"),
		snap("c1c1c1c1", 1_758_000_000, "container:caddy"))
	f.hold(f.domainPath("files"), snap("f1f1f1f1", 1_758_000_000, "fileset:configs"))
	for _, id := range []string{"a1a1a1a1", "c1c1c1c1", "f1f1f1f1"} {
		f.holdsFiles(id, foundAt("/data/site.conf", 1))
	}

	for _, tc := range []struct {
		name string
		args string
		want []string
	}{
		{"everything", `{"name":"site.conf"}`, []string{"caddy", "configs", "nginx"}},
		{"one domain", `{"name":"site.conf","domain":"containers"}`, []string{"caddy", "nginx"}},
		{"one container", `{"name":"site.conf","domain":"containers","item":"nginx"}`, []string{"nginx"}},
		{"one folder set by name", `{"name":"site.conf","domain":"files","item":"configs"}`, []string{"configs"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var items []string
			for _, file := range foundFilesOf(t, f.searchFilesUntilDone(tc.args)) {
				items = append(items, file["item"].(string))
			}
			slices.Sort(items)
			if !slices.Equal(items, tc.want) {
				t.Fatalf("items = %v, want %v", items, tc.want)
			}
		})
	}
}

func TestMCPFileSearchRefusesWhatItCannotSearch(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")

	for _, tc := range []struct {
		name string
		args string
		code string
	}{
		{"a path", `{"name":"etc/passwd"}`, "invalid_argument"},
		{"no name", `{}`, "invalid_argument"},
		{"a domain saved as whole disks", `{"name":"x","domain":"vms"}`, "invalid_argument"},
		{"an item without its domain", `{"name":"x","item":"nginx"}`, "invalid_argument"},
		{"an unknown argument", `{"name":"x","path":"/"}`, "invalid_argument"},
		{"an item nobody protects", `{"name":"x","domain":"containers","item":"ghost"}`, "not_found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if code := mcpErrorCode(t, f.callSearchFiles(tc.args)); code != tc.code {
				t.Fatalf("code = %q, want %q", code, tc.code)
			}
		})
	}
	if n := len(f.eng.finds); n != 0 {
		t.Fatalf("a refused call searched %d times", n)
	}
}

func TestMCPFileSearchAnswersBusyWhileAnotherSearchRuns(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f.eng.onFind = func() {
		once.Do(func() { close(entered) })
		<-release
	}

	f.callSearchFiles(`{"name":"nginx.conf"}`)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the search never reached restic")
	}
	if code := mcpErrorCode(t, f.callSearchFiles(`{"name":"mime.types"}`)); code != "busy" {
		t.Fatalf("another name while one runs = %q, want busy", code)
	}
	if same := mcpStructured(t, f.callSearchFiles(`{"name":"nginx.conf"}`)); same["state"] != "running" {
		t.Fatalf("the same search again = %v, want running", same)
	}
	close(release)
	if out := f.searchFilesUntilDone(`{"name":"nginx.conf"}`); out["state"] != "ready" {
		t.Fatalf("result = %v", out)
	}
}

func TestMCPFileSearchSaysWhenMoreFilesMatched(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	files := make([]restic.FoundFile, 0, maxFoundFiles+1)
	for i := range maxFoundFiles + 1 {
		files = append(files, foundAt(fmt.Sprintf("/appdata/log-%03d.txt", i), 1))
	}
	f.holdsFiles("a1a1a1a1", files...)

	out := f.searchFilesUntilDone(`{"name":"log"}`)

	if got := len(foundFilesOf(t, out)); got != maxFoundFiles || out["truncated"] != true {
		t.Fatalf("%d files, truncated %v, want %d and truncated", got, out["truncated"], maxFoundFiles)
	}
}

func TestMCPFileSearchNamesTheDatasetOfAZFSHit(t *testing.T) {
	f := newPlacementFixture(t)
	tank, home := f.zfsItem("tank/apps")
	f.hold(home, zfsSnap("a1a1a1a1", "tank/apps", firstRun), zfsSnap("a2a2a2a2", "tank/apps/db", firstRun))
	f.zfsRun(tank.ID, firstRun, "tank/apps", "tank/apps/db")
	f.holdsFiles("a2a2a2a2", foundAt("/pg_hba.conf", 3))

	files := foundFilesOf(t, f.searchFilesUntilDone(`{"name":"pg_hba","domain":"zfs","item":"tank/apps"}`))

	if len(files) != 1 || files[0]["item"] != "tank/apps" || files[0]["dataset"] != "tank/apps/db" || files[0]["path"] != "pg_hba.conf" {
		t.Fatalf("files = %v, want pg_hba.conf in tank/apps/db", files)
	}
}

func TestMCPFileSearchKeepsHostPathsOutOfAFailedRepository(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	f.eng.findErr = map[string]error{f.domainPath("containers"): errors.New("unable to load tree of /host/user/appdata/nginx")}

	home := repositoryNamed(t, f.searchFilesUntilDone(`{"name":"nginx.conf"}`), "folder containers")

	msg, _ := home["error"].(string)
	if home["state"] != "failed" || msg == "" || strings.Contains(msg, "/host") {
		t.Fatalf("home = %v, want failed with the reason and no host path", home)
	}
}
