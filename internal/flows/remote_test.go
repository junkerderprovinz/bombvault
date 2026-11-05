//go:build flows

package flows

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// restServer speaks restic's REST protocol over a folder, which is what lets a
// flow use a remote location that can be emptied behind the server's back.
type restServer struct {
	dir string
	url string
}

// newRESTServer stays up until the test binary ends. The size sample after a copy
// is a restic of its own that outlives the install, and a server that went first
// would keep it retrying long after the suite.
func newRESTServer(t *testing.T) *restServer {
	t.Helper()
	s := &restServer{dir: t.TempDir()}
	s.url = httptest.NewServer(s).URL
	return s
}

func (s *restServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := filepath.Join(s.dir, filepath.FromSlash(r.URL.Path))
	var err error
	switch {
	case r.Method == http.MethodPost && r.URL.Query().Get("create") == "true":
		err = os.MkdirAll(p, 0o755)
	case r.Method == http.MethodPost:
		err = writeObject(p, r.Body)
	case r.Method == http.MethodDelete:
		err = os.Remove(p)
	case strings.HasSuffix(r.URL.Path, "/"):
		err = listFolder(w, p)
	default:
		err = serveObject(w, r, p)
	}
	switch {
	case errors.Is(err, os.ErrNotExist):
		http.Error(w, "not found", http.StatusNotFound)
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeObject(p string, body io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, body)
	return errors.Join(err, f.Close())
}

func listFolder(w http.ResponseWriter, dir string) error {
	type entry struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	all := []entry{}
	files, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			return err
		}
		all = append(all, entry{file.Name(), info.Size()})
	}
	w.Header().Set("Content-Type", "application/vnd.x.restic.rest.v2")
	return json.NewEncoder(w).Encode(all)
}

func serveObject(w http.ResponseWriter, r *http.Request, p string) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // opened for reading
	info, err := f.Stat()
	if err != nil {
		return err
	}
	http.ServeContent(w, r, "", info.ModTime(), f)
	return nil
}

// empty takes everything off the server's disk, as a bucket emptied by hand or a
// disk that did not mount looks from outside.
func (s *restServer) empty(t *testing.T) {
	t.Helper()
	files, err := os.ReadDir(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if err := os.RemoveAll(filepath.Join(s.dir, file.Name())); err != nil {
			t.Fatal(err)
		}
	}
}

func (s *restServer) holdsConfig(t *testing.T, folder string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(s.dir, "nas", folder, "config"))
	return err == nil
}

// addRESTLocation adds the location as the add window does, with Folders ticked.
func (f *flow) addRESTLocation(s *restServer) string {
	f.t.Helper()
	var added struct {
		Destination struct {
			ID string `json:"id"`
		} `json:"destination"`
	}
	f.b.call("POST", "/api/offsite/destinations", map[string]any{"provider": "rest", "name": "NAS", "settings": map[string]string{"url": s.url + "/nas"}}, &added)
	if added.Destination.ID == "" {
		f.t.Fatal("no id in the answer for the new location")
	}
	f.b.call("POST", "/api/offsite/destinations/"+added.Destination.ID+"/domains/files", nil, nil)
	var planned struct {
		Impact json.RawMessage `json:"impact"`
	}
	change := map[string]any{"skip": []string{}}
	f.b.call("POST", "/api/placement/default/files/preview", change, &planned)
	change["expect"] = planned.Impact
	f.b.call("PUT", "/api/placement/default/files", change, nil)
	return added.Destination.ID
}

func (f *flow) removeLocation(id string) {
	f.t.Helper()
	var preview struct {
		Removal json.RawMessage `json:"removal"`
	}
	f.b.call("GET", "/api/offsite/destinations/"+id+"/removal", nil, &preview)
	f.b.call("POST", "/api/offsite/destinations/"+id+"/removal", map[string]any{"expect": preview.Removal}, nil)
}

func (f *flow) lastCopy() run {
	f.t.Helper()
	copies := f.b.runsOf("offsite")
	if len(copies) == 0 {
		f.t.Fatal("no copy ran")
	}
	return copies[0]
}

// pageOf reads what the page of the location says.
func (f *flow) pageOf(id string) (gone bool, refused string) {
	f.t.Helper()
	var got struct {
		Locations []struct {
			ID      string `json:"id"`
			Gone    bool   `json:"gone"`
			Refused string `json:"refused"`
		} `json:"locations"`
	}
	f.b.call("GET", "/api/storage/locations", nil, &got)
	for _, l := range got.Locations {
		if l.ID == "destination:"+id {
			return l.Gone, l.Refused
		}
	}
	f.t.Fatalf("the location %s is not listed", id)
	return false, ""
}

func (f *flow) foundGone(id string) bool {
	f.t.Helper()
	gone, _ := f.pageOf(id)
	return gone
}

func (f *flow) refusedAt(id string) string {
	f.t.Helper()
	_, refused := f.pageOf(id)
	return refused
}

func (f *flow) wantCopyAt(s *restServer, set string) {
	f.t.Helper()
	f.backupSet(set)
	if c := f.lastCopy(); c.Status != "success" {
		f.errorf("the copy after the backup is %s: %s", c.Status, c.Error)
	}
	if !s.holdsConfig(f.t, "files") {
		f.errorf("no repository was created at the location")
		return
	}
	f.b.restic("rest:"+s.url+"/nas/files", "check")
}

func TestALocationAddedAgainOverAnEmptiedPlaceStartsClean(t *testing.T) {
	f := typedFlow(t, "files")
	s := newRESTServer(t)
	f.b.write(photosPath, "a.txt", "photo\n", 0o640)
	set := f.b.createSet("photos", photosPath)
	id := f.addRESTLocation(s)
	f.wantCopyAt(s, set)

	f.removeLocation(id)
	s.empty(t)
	id = f.addRESTLocation(s)

	if f.foundGone(id) {
		f.errorf("the page says a repository this location never had is gone")
	}
	f.wantCopyAt(s, set)
	f.wantNoFailedRuns()
	f.finish()
}

func TestALocationThatStayedIsNotFilledWhenItsDiskDropsOut(t *testing.T) {
	f := typedFlow(t, "files")
	s := newRESTServer(t)
	f.b.write(photosPath, "a.txt", "photo\n", 0o640)
	set := f.b.createSet("photos", photosPath)
	id := f.addRESTLocation(s)
	f.wantCopyAt(s, set)

	s.empty(t)
	f.backupSet(set)
	if c := f.lastCopy(); c.Status != "failed" || !strings.Contains(c.Error, "is gone") {
		f.errorf("the copy after the disk dropped out is %s: %s", c.Status, c.Error)
	}
	if s.holdsConfig(t, "files") {
		f.errorf("a new, empty repository was put where the old one dropped out")
	}
	if !f.foundGone(id) {
		f.errorf("the page does not say the repository is gone")
	}

	f.b.call("POST", "/api/storage/locations/destination:"+id+"/start-over", nil, nil)
	f.wantCopyAt(s, set)
	f.finish()
}

func TestAnUpdateFreesALocationAddedAgainBeforeIt(t *testing.T) {
	t.Parallel()
	f := newFlow(t)
	s := newRESTServer(t)
	var set, id string
	starts[1].boot(f, func() {
		f.own = typedPath.prepare(f.b, "files")
		f.b.write(photosPath, "a.txt", "photo\n", 0o640)
		set = f.b.createSet("photos", photosPath)
		id = f.addRESTLocation(s)
		f.backupSet(set)
		f.removeLocation(id)
		s.empty(t)
		id = f.addRESTLocation(s)
		f.backupSet(set)
		if c := f.lastCopy(); c.Status != "failed" {
			f.t.Fatalf("the previous release copied after the location was added again: %s", c.Status)
		}
	})

	if f.foundGone(id) {
		f.errorf("the page still says a repository this location never had is gone")
	}
	f.wantCopyAt(s, set)
	f.finish()
}

func TestALocationOverARepositoryOfAnotherKeySaysSo(t *testing.T) {
	f := typedFlow(t, "files")
	s := newRESTServer(t)
	foreign := exec.Command("restic", "-r", "rest:"+s.url+"/nas/files", "init")
	foreign.Env = append(os.Environ(), "RESTIC_PASSWORD=the key of another install")
	if out, err := foreign.CombinedOutput(); err != nil {
		t.Fatalf("restic init: %v\n%s", err, out)
	}
	f.b.write(photosPath, "a.txt", "photo\n", 0o640)
	set := f.b.createSet("photos", photosPath)
	id := f.addRESTLocation(s)

	f.backupSet(set)
	if c := f.lastCopy(); c.Status != "failed" || !strings.Contains(c.Error, "key does not open it") {
		f.errorf("the copy is %s: %s", c.Status, c.Error)
	}
	if got := f.refusedAt(id); got != "repo-wrong-key" {
		f.errorf("the page has the refusal %q", got)
	}
	f.finish()
}

// noBucketServer answers as an S3 server does for a bucket that does not exist.
// restic reads the code from a header on a HEAD, which has no body.
func noBucketServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("x-minio-error-code", "NoSuchBucket")
		w.Header().Set("x-minio-error-desc", `"The specified bucket does not exist"`)
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `<Error><Code>NoSuchBucket</Code><Message>The specified bucket does not exist</Message></Error>`)
	}))
	return srv.URL
}

func TestACopyToAMissingBucketFailsSoonAndSaysSo(t *testing.T) {
	f := typedFlow(t, "files")
	f.b.write(photosPath, "a.txt", "photo\n", 0o640)
	set := f.b.createSet("photos", photosPath)
	var added struct {
		Destination struct {
			ID string `json:"id"`
		} `json:"destination"`
	}
	f.b.call("POST", "/api/offsite/destinations", map[string]any{"provider": "s3", "name": "NAS", "dir": "missing/primary", "settings": map[string]string{
		"endpoint": noBucketServer(t), "region": "us-east-1", "access_key_id": "invented", "secret_access_key": "invented-too",
	}}, &added)
	id := added.Destination.ID
	f.b.call("POST", "/api/offsite/destinations/"+id+"/domains/files", nil, nil)
	var planned struct {
		Impact json.RawMessage `json:"impact"`
	}
	change := map[string]any{"skip": []string{}}
	f.b.call("POST", "/api/placement/default/files/preview", change, &planned)
	change["expect"] = planned.Impact
	f.b.call("PUT", "/api/placement/default/files", change, nil)

	start := time.Now()
	f.backupSet(set)
	if took := time.Since(start); took > 2*time.Minute {
		f.errorf("the backup and its copy took %s", took)
	}
	if c := f.lastCopy(); c.Status != "failed" || !strings.Contains(c.Error, "bucket does not exist") {
		f.errorf("the copy is %s: %s", c.Status, c.Error)
	}
	if got := f.refusedAt(id); got != "bucket-missing" {
		f.errorf("the page has the refusal %q", got)
	}
	f.finish()
}
