package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func toolImportFixture(t *testing.T, dirs ...string) *placementFixture {
	t.Helper()
	f := newPlacementFixture(t)
	f.svc.cfg.HostSourceRoot = "/mnt"
	f.h.cfg.HostSourceRoot = "/mnt"
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(filepath.FromSlash(f.root), filepath.FromSlash(d)), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func duplicatiFile(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "jobimport", "testdata", name)) //nolint:gosec // G304: a fixture of the jobimport package
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{"name": name, "text": string(data)}
}

// copyFixtureTree copies a folder of the jobimport fixtures to rel under the mount root.
func copyFixtureTree(t *testing.T, f *placementFixture, from, rel string) {
	t.Helper()
	src := filepath.Join("..", "jobimport", "testdata", from)
	dst := filepath.Join(filepath.FromSlash(f.root), filepath.FromSlash(rel))
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		out := filepath.Join(dst, strings.TrimPrefix(p, src))
		if d.IsDir() {
			return os.MkdirAll(out, 0o750)
		}
		data, err := os.ReadFile(p) //nolint:gosec // G304: a fixture of the jobimport package
		if err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o600) //nolint:gosec // G703: the test's own TempDir
	})
	if err != nil {
		t.Fatal(err)
	}
}

func asJSON[T any](t *testing.T, v any) T {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func previewJobs(t *testing.T, f *placementFixture, body map[string]any) []toolJobView {
	t.Helper()
	res := f.do(http.MethodPost, "/api/import/tools/preview", body)
	if res["ok"] != true {
		t.Fatalf("preview = %v", res)
	}
	return asJSON[[]toolJobView](t, res["jobs"])
}

func TestToolImportPreviewFindsTheFoldersAndKeepsTheSecretsOut(t *testing.T) {
	f := toolImportFixture(t, "user/photos")
	res := f.do(http.MethodPost, "/api/import/tools/preview", map[string]any{
		"tool": "duplicati", "files": []any{duplicatiFile(t, "duplicati-photos.json")},
	})
	raw, _ := json.Marshal(res)
	for _, secret := range []string{"exampleSecretKey", "example-access-key", "correct horse"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("the preview carries %q: %s", secret, raw)
		}
	}
	jobs := asJSON[[]toolJobView](t, res["jobs"])
	if len(jobs) != 1 {
		t.Fatalf("jobs = %+v", jobs)
	}
	job := jobs[0]
	if len(job.Sets) != 2 {
		t.Fatalf("sets = %+v", job.Sets)
	}
	photos, videos := job.Sets[0], job.Sets[1]
	if photos.Name != "Family-photos-photos" || photos.Path != "user/photos" {
		t.Errorf("photos = %+v", photos)
	}
	if want := []string{"*.tmp", "*thumbnails*", "/mnt/user/photos/.trash"}; !reflect.DeepEqual(photos.Excludes, want) {
		t.Errorf("photos excludes = %q, want %q", photos.Excludes, want)
	}
	if videos.Path != "" || videos.Source != "/source/videos" || videos.Name != "Family-photos-videos" {
		t.Errorf("a folder that is not there should be left for the user to pick: %+v", videos)
	}
	if job.Dest.Location != "s3:https://s3.eu-central-1.amazonaws.com/example-photos/duplicati/bombvault" || job.Dest.SignIn != signInCarried {
		t.Errorf("dest = %+v", job.Dest)
	}
	if job.Schedule == "" {
		t.Error("the schedule was lost")
	}
}

func TestToolImportCreatesTheSignInTheLocationAndTheSets(t *testing.T) {
	f := toolImportFixture(t, "user/photos", "user/videos-new")
	body := map[string]any{
		"tool":  "duplicati",
		"files": []any{duplicatiFile(t, "duplicati-photos.json")},
		"jobs": []any{map[string]any{"key": "0", "sets": []any{
			map[string]any{"key": "0"},
			map[string]any{"key": "1", "path": "user/videos-new"},
		}}},
	}
	res := f.do(http.MethodPost, "/api/import/tools", body)
	if res["ok"] != true {
		t.Fatalf("apply = %v", res)
	}
	results := asJSON[[]toolJobResult](t, res["jobs"])
	if len(results) != 1 || results[0].Error != "" || len(results[0].Sets) != 2 || results[0].Location != "Family photos" {
		t.Fatalf("results = %+v", results)
	}

	repos, err := f.st.ListNamedRepos()
	if err != nil || len(repos) != 1 {
		t.Fatalf("named repos = %+v, %v", repos, err)
	}
	repo := repos[0]
	if repo.Repo != "s3:https://s3.eu-central-1.amazonaws.com/example-photos/duplicati/bombvault" || repo.StorageClass != "STANDARD_IA" || repo.CredsRef == "" {
		t.Errorf("repo = %+v", repo)
	}
	settings, _ := f.st.GetSettings()
	sets, err := f.svc.decodeCloudCredSets(settings)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == repo.CredsRef })
	if i < 0 || sets[i].S3KeyID != "example-access-key-id" || sets[i].S3Secret != "exampleSecretKey/not-real" || sets[i].S3Region != "eu-central-1" {
		t.Fatalf("sign-in = %+v", sets)
	}

	at := time.Date(2026, 3, 2, 1, 30, 0, 0, time.UTC).In(time.Local).Format("15:04")
	photos, err := f.st.GetFileSetByName("Family-photos-photos")
	if err != nil {
		t.Fatal(err)
	}
	if photos.Path != "user/photos" || photos.Repo != repo.ID || photos.RepoChosen != store.RepoChosen || !photos.Enabled {
		t.Errorf("photos = %+v", photos)
	}
	if photos.ScheduleCadence != "weekly Mon,Tue,Wed,Thu,Fri "+at {
		t.Errorf("cadence = %q", photos.ScheduleCadence)
	}
	videos, err := f.st.GetFileSetByName("Family-photos-videos")
	if err != nil {
		t.Fatal(err)
	}
	if videos.Path != "user/videos-new" || !reflect.DeepEqual(videos.Excludes, []string{"*.tmp", "*thumbnails*"}) {
		t.Errorf("videos = %+v", videos)
	}

	again := previewJobs(t, f, map[string]any{"tool": "duplicati", "files": body["files"]})
	if again[0].Sets[0].Taken != "Family-photos-photos" || again[0].Dest.Existing != "Family photos" {
		t.Errorf("a second look should name what is there already: %+v", again[0])
	}
	body["jobs"] = []any{map[string]any{"key": "0", "sets": []any{map[string]any{"key": "0"}}}}
	if res := f.do(http.MethodPost, "/api/import/tools", body); res["ok"] != true {
		t.Fatalf("second apply = %v", res)
	}
	repos, _ = f.st.ListNamedRepos()
	settings, _ = f.st.GetSettings()
	after, _ := f.svc.decodeCloudCredSets(settings)
	if len(repos) != 1 || len(after) != len(sets) {
		t.Errorf("a second import should reuse the location and the sign-in: %d repos, %d sign-ins", len(repos), len(after))
	}
}

func TestToolImportAsksForWhatTheFileDoesNotHold(t *testing.T) {
	f := toolImportFixture(t, "user/documents")
	jobs := previewJobs(t, f, map[string]any{"tool": "duplicati", "files": []any{duplicatiFile(t, "duplicati-documents.json")}})
	if d := jobs[0].Dest; d.Kind != "b2" || d.Location != "" || d.B2Path != "example-docs/duplicati/documents/bombvault" || d.SignIn != signInNeeded {
		t.Fatalf("dest = %+v", d)
	}
	location := "s3:https://s3.eu-central-003.backblazeb2.com/example-docs/duplicati/documents/bombvault"
	apply := func(choice map[string]any) toolJobResult {
		choice["key"] = "0"
		choice["sets"] = []any{map[string]any{"key": "0"}}
		res := f.do(http.MethodPost, "/api/import/tools", map[string]any{
			"tool": "duplicati", "files": []any{duplicatiFile(t, "duplicati-documents.json")}, "jobs": []any{choice},
		})
		if res["ok"] != true {
			t.Fatalf("apply = %v", res)
		}
		return asJSON[[]toolJobResult](t, res["jobs"])[0]
	}
	if r := apply(map[string]any{}); r.Error == "" {
		t.Errorf("a B2 job without its address went through: %+v", r)
	}
	if r := apply(map[string]any{"location": location}); r.Error == "" {
		t.Errorf("a B2 job without a sign-in went through: %+v", r)
	}
	if _, err := f.st.GetFileSetByName("Documents"); err == nil {
		t.Fatal("a refused job left a folder set behind")
	}
	if repos, _ := f.st.ListNamedRepos(); len(repos) != 0 {
		t.Fatalf("a refused job left a location behind: %+v", repos)
	}
	r := apply(map[string]any{"location": location, "keyId": "0031234567890ab0000000002", "secret": "K003notARealKey"})
	if r.Error != "" {
		t.Fatalf("apply = %+v", r)
	}
	set, err := f.st.GetFileSetByName("Documents")
	if err != nil || set.Enabled {
		t.Errorf("a job that was off should come over switched off: %+v, %v", set, err)
	}
}

func TestToolImportReadsADuplicacyWebFolder(t *testing.T) {
	f := toolImportFixture(t, "user/documents", "user/usb-disk/duplicacy")
	copyFixtureTree(t, f, "duplicacy-web", "user/appdata/duplicacy/config")
	jobs := previewJobs(t, f, map[string]any{"tool": "duplicacy", "path": "user/appdata/duplicacy/config"})
	if len(jobs) != 3 {
		t.Fatalf("jobs = %+v", jobs)
	}
	docs := jobs[0]
	if docs.Tool != "duplicacy-web" || docs.Sets[0].Path != "user/documents" || docs.Schedule != "daily 02:30" {
		t.Errorf("documents = %+v", docs)
	}
	if want := []string{"/mnt/user/documents/Scans/raw", "*.lnk"}; !reflect.DeepEqual(docs.Sets[0].Excludes, want) {
		t.Errorf("excludes = %q", docs.Sets[0].Excludes)
	}
	if docs.Dest.SignIn != signInNeeded {
		t.Errorf("an encrypted sign-in should be asked for: %+v", docs.Dest)
	}
	if jobs[1].Dest.Location != "user/usb-disk/duplicacy/bombvault" {
		t.Errorf("local destination = %+v", jobs[1].Dest)
	}
}

func TestToolImportReadsADuplicacyRepository(t *testing.T) {
	f := toolImportFixture(t)
	copyFixtureTree(t, f, "duplicacy-cli", "user/music")
	jobs := previewJobs(t, f, map[string]any{"tool": "duplicacy", "path": "user/music"})
	if len(jobs) != 2 || jobs[0].Sets[0].Path != "user/music" {
		t.Fatalf("jobs = %+v", jobs)
	}
	want := []string{"*.tmp", "/mnt/user/music/.cache", "node_modules", "/mnt/user/music/Downloads/incomplete", "*.part"}
	if !reflect.DeepEqual(jobs[0].Sets[0].Excludes, want) {
		t.Errorf("excludes = %q, want %q", jobs[0].Sets[0].Excludes, want)
	}
	if jobs[0].Dest.SignIn != signInCarried {
		t.Errorf("keys saved in the preferences should come along: %+v", jobs[0].Dest)
	}
	viaPrefs := previewJobs(t, f, map[string]any{"tool": "duplicacy", "path": "user/music/.duplicacy"})
	if len(viaPrefs) != 2 || viaPrefs[0].Sets[0].Path != "user/music" {
		t.Errorf("choosing the .duplicacy folder itself = %+v", viaPrefs)
	}
}

func TestToolImportRefusesWhatItCannotRead(t *testing.T) {
	f := toolImportFixture(t, "user/empty")
	for _, body := range []map[string]any{
		{"tool": "duplicacy", "path": "user/empty"},
		{"tool": "duplicacy", "path": "../outside"},
		{"tool": "duplicati", "files": []any{duplicatiFile(t, "duplicati-encrypted.json.aes")}},
		{"tool": "duplicati", "files": []any{}},
		{"tool": "borg"},
	} {
		if res := f.do(http.MethodPost, "/api/import/tools/preview", body); res["ok"] != false {
			t.Errorf("%v was read: %v", body, res)
		}
	}
}
