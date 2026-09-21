package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// httpDumpEngine serves the two restic calls the dump routes make: listing a
// container's snapshots and streaming one of them back out.
type httpDumpEngine struct {
	ResticEngine
	snaps  []restic.Snapshot
	raw    []byte
	rawErr error

	snapsCalls int
	dumpedID   string
	dumpedPath string
}

func (e *httpDumpEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	e.snapsCalls++
	return e.snaps, nil
}

func (e *httpDumpEngine) DumpRaw(_ context.Context, _, snapshotID, path string, w io.Writer, _ restic.Mode) error {
	e.dumpedID, e.dumpedPath = snapshotID, path
	if len(e.raw) == 0 {
		return e.rawErr
	}
	if _, err := w.Write(e.raw); err != nil {
		return err
	}
	return e.rawErr
}

// dumpSnapshot is one dump snapshot of the container "pg" as restic reports it.
func dumpSnapshot(id, at string, size uint64, tags ...string) restic.Snapshot {
	return restic.Snapshot{
		ID:      id,
		Time:    at,
		Paths:   []string{"/dbdump/pg.sql"},
		Tags:    append([]string{"dbdump:pg", "p1"}, tags...),
		Summary: &restic.SnapshotSummary{TotalBytesProcessed: size},
	}
}

func dumpHandler(svc *Service) *Handler {
	return &Handler{cfg: svc.cfg, store: svc.store, svc: svc}
}

func dumpDownloadRequest(name, id, query string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/containers/"+name+"/dbdumps/"+id+"/download"+query, nil)
	r.SetPathValue("name", name)
	r.SetPathValue("id", id)
	return r
}

func finishedRun(t *testing.T, st *store.Repo, targetID, kind, status, snapshotID string) string {
	t.Helper()
	id, err := st.StartRun(targetID, kind)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(id, status, snapshotID, 0, ""); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestListDBDumpsPairsWithBackupRun(t *testing.T) {
	eng := &httpDumpEngine{}
	svc := serviceWithContainersRepo(t, eng)
	tg, err := svc.store.UpsertTarget(store.Target{ContainerName: "pg"})
	if err != nil {
		t.Fatal(err)
	}
	paired := finishedRun(t, svc.store, tg.ID, "backup", "success", "dddd4444dddd4444")
	finishedRun(t, svc.store, tg.ID, "dbdump", "failed", "cccc3333cccc3333")

	eng.snaps = []restic.Snapshot{
		dumpSnapshot("aaaa1111aaaa1111", "2026-09-15T02:00:00.000000000+02:00", 734912311,
			"bvrun:"+paired, "dbengine:postgres", "dbimage:postgres:16.4", "dbversion:16.4",
			"dbname:immich", "dbname:postgres"),
		dumpSnapshot("bbbb2222bbbb2222", "2026-09-16T02:00:00.000000000+02:00", 0,
			"bvrun:0123456789abcdef0123456789abcdef"),
		dumpSnapshot("cccc3333cccc3333", "2026-09-17T02:00:00.000000000+02:00", 0),
		snapWithTags("eeee5555eeee5555", "container:pg"),
	}

	views, err := svc.DBDumps(context.Background(), "pg", "local")
	if err != nil {
		t.Fatalf("DBDumps: %v", err)
	}
	if len(views) != 3 {
		t.Fatalf("got %d dumps, want the three dump snapshots and not the volume snapshot: %+v", len(views), views)
	}
	if got := []string{views[0].ID, views[1].ID, views[2].ID}; got[0] != "cccc3333cccc3333" || got[2] != "aaaa1111aaaa1111" {
		t.Fatalf("dumps = %v, want the newest first", got)
	}
	if eng.snapsCalls != 1 {
		t.Errorf("the repository was listed %d times, want once", eng.snapsCalls)
	}

	full := views[2]
	if full.PairedSnapshotID != "dddd4444dddd4444" {
		t.Errorf("pairedSnapshotId = %q, want the snapshot of the backup run the dump was taken for", full.PairedSnapshotID)
	}
	if full.Engine != "postgres" || full.Image != "postgres:16.4" || full.Version != "16.4" {
		t.Errorf("engine/image/version = %q/%q/%q, want them read off the tags", full.Engine, full.Image, full.Version)
	}
	if got := strings.Join(full.Databases, ","); got != "immich,postgres" {
		t.Errorf("databases = %q, want both dbname tags", got)
	}
	if full.Bytes != 734912311 {
		t.Errorf("bytes = %d, want the snapshot summary's total", full.Bytes)
	}
	if full.Damaged {
		t.Error("a dump whose run succeeded must not be marked damaged")
	}

	if views[1].PairedSnapshotID != "" {
		t.Errorf("pairedSnapshotId = %q, want it omitted when the backup run is gone", views[1].PairedSnapshotID)
	}
	if views[1].Databases == nil {
		t.Error("databases must be an empty array, never null: the client renders it without a guard")
	}
	if !views[0].Damaged {
		t.Error("the snapshot a failed dump run left behind must be marked damaged")
	}
}

// downloadFixture is a service holding exactly one downloadable dump of "pg".
func downloadFixture(t *testing.T, payload []byte) (*Service, *httpDumpEngine) {
	t.Helper()
	eng := &httpDumpEngine{
		raw: payload,
		snaps: []restic.Snapshot{
			dumpSnapshot("3f9c2a1be0d4aaaa", "2026-09-17T02:14:03.123456789+02:00", uint64(len(payload)),
				"dbengine:postgres", "dbversion:14.19"),
		},
	}
	svc := serviceWithContainersRepo(t, eng)
	if _, err := svc.store.UpsertTarget(store.Target{ContainerName: "pg"}); err != nil {
		t.Fatal(err)
	}
	return svc, eng
}

func enableExportSealing(t *testing.T, svc *Service, recipient string) {
	t.Helper()
	settings, err := svc.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ExportEncryptEnabled = true
	settings.ExportAgeRecipients = recipient
	if err := svc.store.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadDBDumpStreamsAndNames(t *testing.T) {
	payload := []byte("-- PostgreSQL database dump\nSELECT 1;\n-- dump complete\n")

	t.Run("plain", func(t *testing.T) {
		svc, eng := downloadFixture(t, payload)
		w := httptest.NewRecorder()
		dumpHandler(svc).handleDownloadDBDump(w, dumpDownloadRequest("pg", "3f9c2a1be0d4aaaa", ""))

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if !bytes.Equal(w.Body.Bytes(), payload) {
			t.Errorf("body = %q, want the raw dump", w.Body.String())
		}
		if got := w.Header().Get("Content-Disposition"); got != `attachment; filename="pg-20260917-021403-3f9c2a1b.sql"` {
			t.Errorf("Content-Disposition = %q", got)
		}
		if got := w.Header().Get("Content-Type"); got != "application/sql" {
			t.Errorf("Content-Type = %q", got)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Cache-Control") != "no-store" {
			t.Error("a download answers with nosniff and no-store")
		}
		if eng.dumpedPath != "/dbdump/pg.sql" || eng.dumpedID != "3f9c2a1be0d4aaaa" {
			t.Errorf("streamed %q of %q, want the dump path of the requested snapshot", eng.dumpedPath, eng.dumpedID)
		}
	})

	t.Run("gzip", func(t *testing.T) {
		svc, _ := downloadFixture(t, payload)
		w := httptest.NewRecorder()
		dumpHandler(svc).handleDownloadDBDump(w, dumpDownloadRequest("pg", "3f9c2a1be0d4aaaa", "?gz=1"))

		if got := w.Header().Get("Content-Disposition"); !strings.HasSuffix(got, `-3f9c2a1b.sql.gz"`) {
			t.Errorf("Content-Disposition = %q, want the .sql.gz name", got)
		}
		if got := w.Header().Get("Content-Type"); got != "application/gzip" {
			t.Errorf("Content-Type = %q", got)
		}
		zr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
		if err != nil {
			t.Fatalf("body is not gzip: %v", err)
		}
		got, err := io.ReadAll(zr)
		if err != nil {
			t.Fatalf("read gzip: %v", err)
		}
		if !bytes.Equal(got, payload) {
			t.Errorf("gunzipped body = %q, want the raw dump", got)
		}
	})

	t.Run("sealed", func(t *testing.T) {
		svc, _ := downloadFixture(t, payload)
		id, err := age.GenerateX25519Identity()
		if err != nil {
			t.Fatal(err)
		}
		enableExportSealing(t, svc, id.Recipient().String())

		w := httptest.NewRecorder()
		dumpHandler(svc).handleDownloadDBDump(w, dumpDownloadRequest("pg", "3f9c2a1be0d4aaaa", ""))

		if got := w.Header().Get("Content-Disposition"); !strings.HasSuffix(got, `-3f9c2a1b.sql.age"`) {
			t.Errorf("Content-Disposition = %q, want the .sql.age name", got)
		}
		if got := w.Header().Get("Content-Type"); got != "application/octet-stream" {
			t.Errorf("Content-Type = %q", got)
		}
		if bytes.Equal(w.Body.Bytes(), payload) {
			t.Fatal("the body must be ciphertext, not the plain dump")
		}
		if got := ageRoundTripDecrypt(t, w.Body.Bytes(), id); !bytes.Equal(got, payload) {
			t.Errorf("decrypted body = %q, want the raw dump", got)
		}
	})
}

func TestDownloadDBDumpThatFailsMidStreamIsNotComplete(t *testing.T) {
	sealed := func(t *testing.T, svc *Service) {
		t.Helper()
		id, err := age.GenerateX25519Identity()
		if err != nil {
			t.Fatal(err)
		}
		enableExportSealing(t, svc, id.Recipient().String())
	}
	for _, tc := range []struct {
		name  string
		query string
		seal  bool
	}{
		{"plain", "", false},
		{"gzip", "?gz=1", false},
		{"sealed", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Enough bytes that do not compress to push the headers and a part of
			// the body out before the break.
			partial := make([]byte, 256<<10)
			if _, err := rand.Read(partial); err != nil {
				t.Fatal(err)
			}
			svc, eng := downloadFixture(t, partial)
			eng.rawErr = errors.New("restic dump failed: pack 5e1f not found")
			if tc.seal {
				sealed(t, svc)
			}
			h := dumpHandler(svc)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.SetPathValue("name", "pg")
				r.SetPathValue("id", "3f9c2a1be0d4aaaa")
				h.handleDownloadDBDump(w, r)
			}))
			defer srv.Close()

			resp, err := srv.Client().Get(srv.URL + tc.query)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want the download under way when the dump breaks off", resp.StatusCode)
			}
			if _, err := io.ReadAll(resp.Body); err == nil {
				t.Error("the body ended cleanly, so a browser saves the truncated dump as finished")
			}
		})
	}

	t.Run("sealed, failing before the first byte", func(t *testing.T) {
		svc, eng := downloadFixture(t, nil)
		eng.rawErr = errors.New("restic dump failed: repository is already locked exclusively")
		sealed(t, svc)

		w := httptest.NewRecorder()
		dumpHandler(svc).handleDownloadDBDump(w, dumpDownloadRequest("pg", "3f9c2a1be0d4aaaa", ""))
		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409 rather than a sealed file holding only its header", w.Code)
		}
		if got := w.Header().Get("Content-Disposition"); got != "" {
			t.Errorf("Content-Disposition = %q, want none", got)
		}
	})
}

func TestDownloadDBDumpRefusals(t *testing.T) {
	payload := []byte("-- dump\n")

	refusals := []struct {
		name    string
		id      string
		prepare func(t *testing.T, svc *Service, eng *httpDumpEngine)
		want    string
	}{
		{
			name: "an id that is not hex",
			id:   "not-hex",
			want: "snapshot id",
		},
		{
			name: "a snapshot of the container's volumes",
			id:   "eeee5555eeee5555",
			prepare: func(_ *testing.T, _ *Service, eng *httpDumpEngine) {
				eng.snaps = append(eng.snaps, snapWithTags("eeee5555eeee5555", "container:pg"))
			},
			want: "is not a database dump of this container",
		},
		{
			name: "a snapshot a failed dump left behind",
			id:   "3f9c2a1be0d4aaaa",
			prepare: func(t *testing.T, svc *Service, _ *httpDumpEngine) {
				tg, err := svc.store.GetTargetByContainer("pg")
				if err != nil {
					t.Fatal(err)
				}
				finishedRun(t, svc.store, tg.ID, "dbdump", "failed", "3f9c2a1be0d4aaaa")
			},
			want: "damaged",
		},
		{
			name: "a dump stored under another path",
			id:   "3f9c2a1be0d4aaaa",
			prepare: func(_ *testing.T, _ *Service, eng *httpDumpEngine) {
				eng.snaps[0].Paths = []string{"/dbdump/other.sql"}
			},
			want: "does not hold this container's dump",
		},
		{
			name: "sealing switched on without a recipient",
			id:   "3f9c2a1be0d4aaaa",
			prepare: func(t *testing.T, svc *Service, _ *httpDumpEngine) {
				enableExportSealing(t, svc, "")
			},
			want: "export encryption is on",
		},
	}

	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			svc, eng := downloadFixture(t, payload)
			if tc.prepare != nil {
				tc.prepare(t, svc, eng)
			}
			h := dumpHandler(svc)

			w := httptest.NewRecorder()
			h.handleDownloadDBDump(w, dumpDownloadRequest("pg", tc.id, "?check=1"))
			if w.Code != http.StatusOK {
				t.Fatalf("the preflight answers 200 with an envelope, got %d", w.Code)
			}
			var resp struct {
				OK    bool   `json:"ok"`
				Error string `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode preflight: %v (%s)", err, w.Body.String())
			}
			if resp.OK || !strings.Contains(resp.Error, tc.want) {
				t.Fatalf("preflight = %+v, want a refusal naming %q", resp, tc.want)
			}

			w = httptest.NewRecorder()
			h.handleDownloadDBDump(w, dumpDownloadRequest("pg", tc.id, ""))
			if w.Code != http.StatusConflict {
				t.Fatalf("status = %d, want 409 so the browser reports a failed download", w.Code)
			}
			if got := w.Header().Get("Content-Disposition"); got != "" {
				t.Fatalf("a refused download must carry no Content-Disposition, got %q", got)
			}
		})
	}

	t.Run("a name that would leave the container", func(t *testing.T) {
		svc, _ := downloadFixture(t, payload)
		w := httptest.NewRecorder()
		dumpHandler(svc).handleDownloadDBDump(w, dumpDownloadRequest("../etc", "3f9c2a1be0d4aaaa", ""))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
		if got := w.Header().Get("Content-Disposition"); got != "" {
			t.Fatalf("a refused download must carry no Content-Disposition, got %q", got)
		}
	})
}

func waitForDetachedRun(t *testing.T, svc *Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !svc.batchActive.Load() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the detached run to finish")
}

func latestRunOfKind(t *testing.T, st *store.Repo, kind string) store.Run {
	t.Helper()
	runs, err := st.ListRuns(20)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		if r.Kind == kind {
			return r
		}
	}
	t.Fatalf("no run of kind %q recorded, got %+v", kind, runs)
	return store.Run{}
}

func TestSaveDBDumpToPathContainedAndExclusive(t *testing.T) {
	payload := []byte("-- dump\nSELECT 1;\n")
	const savedName = "pg-20260917-021403-3f9c2a1b.sql"

	t.Run("a target outside the host mount is refused", func(t *testing.T) {
		svc, _ := downloadFixture(t, payload)
		if _, started, err := svc.StartSaveDBDumpToPath(context.Background(), "pg", "local", "3f9c2a1be0d4aaaa", "../outside", false); err == nil || started {
			t.Fatalf("started=%v err=%v, want a refusal", started, err)
		}
	})

	t.Run("the dump lands owned and through a partial file", func(t *testing.T) {
		svc, _ := downloadFixture(t, payload)
		var chowned []int
		svc.dbDumpChown = func(_ string, uid, gid int) error {
			chowned = append(chowned, uid, gid)
			return nil
		}

		target, started, err := svc.StartSaveDBDumpToPath(context.Background(), "pg", "local", "3f9c2a1be0d4aaaa", "user/restore", false)
		if err != nil || !started {
			t.Fatalf("started=%v err=%v", started, err)
		}
		waitForDetachedRun(t, svc)

		want := filepath.Join(svc.cfg.HostMountRoot, "user", "restore", savedName)
		if target != want {
			t.Fatalf("target = %q, want %q", target, want)
		}
		got, err := os.ReadFile(want) //nolint:gosec // G304: a path this test built
		if err != nil {
			t.Fatalf("read the saved dump: %v", err)
		}
		if !bytes.Equal(got, payload) {
			t.Errorf("saved dump = %q, want the raw dump", got)
		}
		if _, err := os.Stat(want + ".partial"); !errors.Is(err, os.ErrNotExist) {
			t.Error("the partial file must be renamed away, not left next to the dump")
		}
		if len(chowned) != 2 || chowned[0] != 99 || chowned[1] != 100 {
			t.Errorf("chown got %v, want the file owned by 99:100", chowned)
		}
		if runtime.GOOS != "windows" {
			info, sErr := os.Stat(want)
			if sErr != nil {
				t.Fatal(sErr)
			}
			if info.Mode().Perm() != 0o640 {
				t.Errorf("mode = %v, want 0640", info.Mode().Perm())
			}
		}
		if run := latestRunOfKind(t, svc.store, "dbdumpsave"); run.Status != "success" {
			t.Errorf("run status = %q, want success", run.Status)
		}
	})

	t.Run("an existing file of that name is kept", func(t *testing.T) {
		svc, _ := downloadFixture(t, payload)
		dir := filepath.Join(svc.cfg.HostMountRoot, "user", "restore")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, savedName), []byte("older"), 0o600); err != nil {
			t.Fatal(err)
		}

		_, started, err := svc.StartSaveDBDumpToPath(context.Background(), "pg", "local", "3f9c2a1be0d4aaaa", "user/restore", false)
		if err == nil || started {
			t.Fatalf("started=%v err=%v, want a refusal", started, err)
		}
		got, err := os.ReadFile(filepath.Join(dir, savedName)) //nolint:gosec // G304: a path this test built
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "older" {
			t.Errorf("the existing file was overwritten with %q", got)
		}
	})

	t.Run("a failed stream leaves nothing behind", func(t *testing.T) {
		svc, eng := downloadFixture(t, payload)
		eng.rawErr = errors.New("restic dump: exit status 1")
		svc.dbDumpChown = func(string, int, int) error { return nil }

		target, started, err := svc.StartSaveDBDumpToPath(context.Background(), "pg", "local", "3f9c2a1be0d4aaaa", "user/restore", false)
		if err != nil || !started {
			t.Fatalf("started=%v err=%v", started, err)
		}
		waitForDetachedRun(t, svc)

		if _, err := os.Stat(target + ".partial"); !errors.Is(err, os.ErrNotExist) {
			t.Error("a failed save must remove its partial file")
		}
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			t.Error("a failed save must not publish a file")
		}
		if run := latestRunOfKind(t, svc.store, "dbdumpsave"); run.Status != "failed" {
			t.Errorf("run status = %q, want failed", run.Status)
		}
	})
}
