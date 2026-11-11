package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"filippo.io/age"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// zipBackup starts like the zip archive most apps hand over.
var zipBackup = []byte("PK\x03\x04 the app's own backup")

func appExportSnap(id string, at int64, container, app string, tags ...string) restic.Snapshot {
	s := snap(id, at, append([]string{"appexport:" + container, "app:" + app}, tags...)...)
	s.Paths = []string{"/appexport/" + container + "/" + app}
	return s
}

// appExportEngine is a placement engine that also streams a snapshot back out.
type appExportEngine struct {
	*placementEngine
	raw    []byte
	dumped []string
}

func (e *appExportEngine) DumpRaw(_ context.Context, _, id, path string, w io.Writer, _ restic.Mode) error {
	e.dumped = append(e.dumped, id+" "+path)
	_, err := w.Write(e.raw)
	return err
}

// appExportFixture holds a backup of the container knightloader and the own
// backup KnightLoader handed over just before it.
func appExportFixture(t *testing.T) (*placementFixture, *appExportEngine) {
	t.Helper()
	f := newPlacementFixture(t)
	f.container("knightloader", "")
	f.hold(f.domainPath("containers"),
		snap("a1a1a1a1", 1_758_000_100, "container:knightloader"),
		appExportSnap("e1e1e1e1", 1_758_000_000, "knightloader", "knightloader", "appname:KnightLoader"))
	eng := &appExportEngine{placementEngine: f.eng, raw: zipBackup}
	f.svc.engine = eng
	return f, eng
}

func (f *placementFixture) download(path string) *httptest.ResponseRecorder {
	f.t.Helper()
	r := jsonReq(http.MethodGet, path, nil)
	if f.session != "" {
		r.AddCookie(&http.Cookie{Name: f.h.sessionCookieNameFor(), Value: f.session}) //nolint:gosec // G124: a cookie on a test request
	}
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, r)
	return rec
}

func TestTheOwnBackupOfTheAppInAContainerIsInItsTimeline(t *testing.T) {
	f, _ := appExportFixture(t)
	f.hold(f.domainPath("containers"),
		snap("a1a1a1a1", 1_758_000_100, "container:knightloader"),
		appExportSnap("e1e1e1e1", 1_758_000_000, "knightloader", "knightloader", "appname:KnightLoader"),
		appExportSnap("e2e2e2e2", 1_757_000_000, "knightloader", "arrowloop"),
		appExportSnap("e3e3e3e3", 1_758_000_000, "plex", "towertorch", "appname:TowerTorch"))

	tl, err := f.svc.Timeline(context.Background(), "containers", "knightloader")
	if err != nil {
		t.Fatal(err)
	}
	if got := rowKeys(tl.Rows); !slices.Equal(got, []string{"a1a1a1a1", "e1e1e1e1", "e2e2e2e2"}) {
		t.Fatalf("rows = %v, want the backup and this container's two app backups", got)
	}
	if tl.Rows[0].AppExport != nil {
		t.Fatalf("the container's own backup reads as an app's: %+v", tl.Rows[0].AppExport)
	}
	if got := tl.Rows[1].AppExport; got == nil || *got != (timelineAppExport{App: "knightloader", Name: "KnightLoader"}) {
		t.Fatalf("app backup = %+v, want KnightLoader's", got)
	}
	if got := tl.Rows[2].AppExport; got == nil || *got != (timelineAppExport{App: "arrowloop", Name: "arrowloop"}) {
		t.Fatalf("app backup without a name = %+v, want it named by its id", got)
	}
	markOf(t, tl.Rows, "e1e1e1e1", "local")
}

func TestTheTagsOfAnAppBackupAreNotSetByHand(t *testing.T) {
	for _, tag := range []string{"appexport:knightloader", "app:knightloader", "appname:KnightLoader"} {
		if _, err := sanitizeTags([]string{tag}); err == nil {
			t.Errorf("%s was taken as a tag of a person", tag)
		}
	}
}

func TestTheOwnBackupOfAnAppIsDeletedLikeABackup(t *testing.T) {
	f, _ := appExportFixture(t)
	ctx := context.Background()

	del, _, err := f.svc.timelineDeletePreview(ctx, "containers", "knightloader", "e1e1e1e1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(del) != 1 || !slices.Equal(del[0].SnapshotIDs, []string{"e1e1e1e1"}) {
		t.Fatalf("delete = %+v, want the app backup at the home", del)
	}
	if _, _, err := f.svc.timelineDelete(ctx, "containers", "knightloader", "e1e1e1e1", del); err != nil {
		t.Fatal(err)
	}
	if len(f.eng.deletes) != 1 || !slices.Equal(f.eng.deletes[0].IDs, []string{"e1e1e1e1"}) {
		t.Fatalf("forgot %+v, want the app backup alone", f.eng.deletes)
	}
}

func TestTheOwnBackupOfAnAppDownloadsUnderItsName(t *testing.T) {
	f, eng := appExportFixture(t)
	f.signIn()

	w := f.download("/api/containers/knightloader/appexports/e1e1e1e1/download")
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), zipBackup) {
		t.Fatalf("status %d with %q, want the app's backup", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Disposition"); got != `attachment; filename="knightloader-backup-20250916-052000-e1e1e1e1.zip"` {
		t.Errorf("Content-Disposition = %q", got)
	}
	if got := w.Header().Get("Content-Type"); got != "application/zip" {
		t.Errorf("Content-Type = %q", got)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Cache-Control") != "no-store" {
		t.Error("a download answers with nosniff and no-store")
	}
	if want := []string{"e1e1e1e1 /appexport/knightloader/knightloader"}; !slices.Equal(eng.dumped, want) {
		t.Errorf("streamed %v, want %v", eng.dumped, want)
	}
}

func TestAnAppBackupOfAnUnknownKindDownloadsWithoutAnExtension(t *testing.T) {
	f, eng := appExportFixture(t)
	eng.raw = []byte("KnightLoader sealed backup 1\n...")
	f.signIn()

	w := f.download("/api/containers/knightloader/appexports/e1e1e1e1/download")
	if got := w.Header().Get("Content-Disposition"); got != `attachment; filename="knightloader-backup-20250916-052000-e1e1e1e1"` {
		t.Errorf("Content-Disposition = %q", got)
	}
	if got := w.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q", got)
	}
}

func TestAnAppBackupIsSealedWhenExportsAre(t *testing.T) {
	f, _ := appExportFixture(t)
	f.signIn()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	enableExportSealing(t, f.svc, id.Recipient().String())

	w := f.download("/api/containers/knightloader/appexports/e1e1e1e1/download")
	if got := w.Header().Get("Content-Disposition"); !strings.HasSuffix(got, `-e1e1e1e1.zip.age"`) {
		t.Errorf("Content-Disposition = %q, want the .zip.age name", got)
	}
	if got := ageRoundTripDecrypt(t, w.Body.Bytes(), id); !bytes.Equal(got, zipBackup) {
		t.Errorf("decrypted body = %q, want the app's backup", got)
	}
}

func TestAnAppBackupDownloadNeedsAPersonSignedIn(t *testing.T) {
	f, eng := appExportFixture(t)

	w := f.download("/api/containers/knightloader/appexports/e1e1e1e1/download")
	var out struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusForbidden || out.Code != codeNoPassword {
		t.Fatalf("status %d with code %q, want the refusal that asks for a password", w.Code, out.Code)
	}
	if len(eng.dumped) != 0 || w.Header().Get("Content-Disposition") != "" {
		t.Fatalf("streamed %v under %q", eng.dumped, w.Header().Get("Content-Disposition"))
	}
}

func TestOnlyAnAppBackupOfThisContainerDownloads(t *testing.T) {
	for _, tc := range []struct {
		name, container, id string
	}{
		{"the container's own backup", "knightloader", "a1a1a1a1"},
		{"an app backup of another container", "knightloader", "e3e3e3e3"},
		{"a backup that is not there", "knightloader", "f0f0f0f0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, eng := appExportFixture(t)
			f.hold(f.domainPath("containers"),
				snap("a1a1a1a1", 1_758_000_100, "container:knightloader"),
				appExportSnap("e3e3e3e3", 1_758_000_000, "plex", "towertorch"))
			f.signIn()
			base := "/api/containers/" + tc.container + "/appexports/" + tc.id + "/download"

			code, out := f.call(http.MethodGet, base+"?check=1", nil)
			if code != http.StatusOK || out["ok"] != false {
				t.Fatalf("preflight = %d %v, want a refusal in the envelope", code, out)
			}
			w := f.download(base)
			if w.Code != http.StatusConflict || w.Header().Get("Content-Disposition") != "" {
				t.Fatalf("status %d under %q, want 409 and no attachment", w.Code, w.Header().Get("Content-Disposition"))
			}
			if len(eng.dumped) != 0 {
				t.Fatalf("streamed %v", eng.dumped)
			}
		})
	}
}

func TestAnAppBackupWhosePathIsNotItsOwnStaysIn(t *testing.T) {
	f, eng := appExportFixture(t)
	odd := appExportSnap("e1e1e1e1", 1_758_000_000, "knightloader", "knightloader")
	odd.Paths = []string{"/appexport/plex/knightloader"}
	f.hold(f.domainPath("containers"), odd)
	f.signIn()

	if w := f.download("/api/containers/knightloader/appexports/e1e1e1e1/download"); w.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", w.Code)
	}
	if len(eng.dumped) != 0 {
		t.Fatalf("streamed %v", eng.dumped)
	}
}
