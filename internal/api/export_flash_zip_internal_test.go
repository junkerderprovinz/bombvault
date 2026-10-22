package api

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// flashZipFakeEngine implements only DumpZip, the one engine method
// exportFlashZip calls. Any other call panics on the nil embedded interface.
type flashZipFakeEngine struct {
	ResticEngine
	dumpBytes []byte
	dumpErr   error
	dumpCalls int
}

func (f *flashZipFakeEngine) DumpZip(_ context.Context, _, _, _ string, w io.Writer, _ restic.Mode) error {
	f.dumpCalls++
	if f.dumpErr != nil {
		return f.dumpErr
	}
	_, err := w.Write(f.dumpBytes)
	return err
}

// newFlashExportStore opens a migrated in-memory store, where exportFlashZip
// records its run.
func newFlashExportStore(t *testing.T) *store.Repo {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

// latestExportRun returns the newest kind="export" run, or fails the test.
func latestExportRun(t *testing.T, st *store.Repo) store.Run {
	t.Helper()
	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		if r.Kind == "export" {
			return r
		}
	}
	t.Fatalf("no export run recorded, got %+v", runs)
	return store.Run{}
}

// readZipEntry returns the content of one entry of the zip at file.
func readZipEntry(t *testing.T, file, name string) []byte {
	t.Helper()
	zr, err := zip.OpenReader(file)
	if err != nil {
		t.Fatalf("open %s: %v", file, err)
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = r.Close() }()
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	t.Fatalf("%s has no entry %s", file, name)
	return nil
}

// With Keep 0 the export writes flash-latest.zip, removes its temp file and
// overwrites the same file on the next run.
func TestExportFlashZipKeep0(t *testing.T) {
	root := t.TempDir()
	fake := &flashZipFakeEngine{dumpBytes: buildTestStoredZip(t, map[string][]byte{"bzimage": []byte("first")})}
	svc := &Service{
		cfg:    config.Config{HostMountRoot: root, DataDir: t.TempDir(), FlashDir: "/boot"},
		engine: fake,
		store:  newFlashExportStore(t),
	}
	settings := store.Settings{
		FlashZipExportEnabled: true,
		FlashZipExportPath:    "export",
		FlashZipExportKeep:    0,
	}

	if err := svc.exportFlashZip(context.Background(), settings, "deadbeef", restic.Mode{}, "/repo"); err != nil {
		t.Fatalf("exportFlashZip: %v", err)
	}

	dir := filepath.Join(root, "export")
	latest := filepath.Join(dir, "flash-latest.zip")
	if got := readZipEntry(t, latest, "bzimage"); string(got) != "first" {
		t.Fatalf("flash-latest.zip holds %q, want the dumped snapshot", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".flash-export.tmp.zip")); !os.IsNotExist(err) {
		t.Fatalf("temp file must be gone after a successful export, stat err = %v", err)
	}

	fake.dumpBytes = buildTestStoredZip(t, map[string][]byte{"bzimage": []byte("second")})
	if err := svc.exportFlashZip(context.Background(), settings, "cafebabe", restic.Mode{}, "/repo"); err != nil {
		t.Fatalf("second exportFlashZip: %v", err)
	}
	if got := readZipEntry(t, latest, "bzimage"); string(got) != "second" {
		t.Fatalf("flash-latest.zip not overwritten, holds %q", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "flash-latest.zip" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("Keep==0 export dir = %v, want only [flash-latest.zip]", names)
	}
}

// The scheduled export is the same zip as the download: every entry deflated
// and the junk left out.
func TestTheScheduledExportIsTheZipTheUSBCreatorExtracts(t *testing.T) {
	root := t.TempDir()
	fake := &flashZipFakeEngine{dumpBytes: buildTestStoredZip(t, map[string][]byte{
		"bzfirmware":       bytes.Repeat([]byte("firmware-bytes-"), 100),
		"previous/bzimage": []byte("old"),
		".git/config":      []byte("[core]"),
	})}
	svc := &Service{
		cfg:    config.Config{HostMountRoot: root, DataDir: t.TempDir(), FlashDir: "/boot"},
		engine: fake,
		store:  newFlashExportStore(t),
	}
	settings := store.Settings{FlashZipExportEnabled: true, FlashZipExportPath: "export"}

	if err := svc.exportFlashZip(context.Background(), settings, "deadbeef", restic.Mode{}, "/repo"); err != nil {
		t.Fatalf("exportFlashZip: %v", err)
	}
	zr, err := zip.OpenReader(filepath.Join(root, "export", "flash-latest.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	if len(zr.File) != 1 || zr.File[0].Name != "bzfirmware" || zr.File[0].Method != zip.Deflate {
		var got []string
		for _, f := range zr.File {
			got = append(got, fmt.Sprintf("%s (method %d)", f.Name, f.Method))
		}
		t.Fatalf("export entries = %v, want only bzfirmware, deflated", got)
	}
}

// pruneFlashZips keeps the newest keep timestamped zips and leaves other files
// alone.
func TestPruneFlashZips(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"flash-20260101-000000.zip",
		"flash-20260102-000000.zip",
		"flash-20260103-000000.zip",
		"keepme.zip",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	(&Service{}).pruneFlashZips(dir, 2)

	survivors := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		survivors[e.Name()] = true
	}

	want := []string{"flash-20260103-000000.zip", "flash-20260102-000000.zip", "keepme.zip"}
	for _, w := range want {
		if !survivors[w] {
			t.Errorf("%s should have survived pruning, dir now = %v", w, keys(survivors))
		}
	}
	if survivors["flash-20260101-000000.zip"] {
		t.Errorf("oldest timestamped zip should have been pruned, dir now = %v", keys(survivors))
	}
	if len(survivors) != len(want) {
		t.Errorf("survivor count = %d (%v), want %d", len(survivors), keys(survivors), len(want))
	}
}

// With keep 0 every timestamped zip goes, while flash-latest.zip and unrelated
// files stay.
func TestPruneFlashZipsKeepZeroDeletesAllTimestamped(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"flash-20260101-000000.zip",
		"flash-20260102-000000.zip",
		"flash-20260103-000000.zip",
		"flash-latest.zip",
		"keepme.zip",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	(&Service{}).pruneFlashZips(dir, 0)

	survivors := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		survivors[e.Name()] = true
	}

	for _, ts := range []string{"flash-20260101-000000.zip", "flash-20260102-000000.zip", "flash-20260103-000000.zip"} {
		if survivors[ts] {
			t.Errorf("%s should have been deleted by keep==0 prune, dir now = %v", ts, keys(survivors))
		}
	}
	want := []string{"flash-latest.zip", "keepme.zip"}
	for _, w := range want {
		if !survivors[w] {
			t.Errorf("%s should have survived keep==0 prune, dir now = %v", w, keys(survivors))
		}
	}
	if len(survivors) != len(want) {
		t.Errorf("survivor count = %d (%v), want %d", len(survivors), keys(survivors), len(want))
	}
}

// A DumpZip failure is returned, recorded as a failed run, and leaves no file
// behind.
func TestExportFlashZipDumpError(t *testing.T) {
	root := t.TempDir()
	fake := &flashZipFakeEngine{dumpErr: errors.New("boom")}
	st := newFlashExportStore(t)
	svc := &Service{
		cfg:    config.Config{HostMountRoot: root, DataDir: t.TempDir(), FlashDir: "/boot"},
		engine: fake,
		store:  st,
	}
	settings := store.Settings{
		FlashZipExportEnabled: true,
		FlashZipExportPath:    "export",
	}

	err := svc.exportFlashZip(context.Background(), settings, "deadbeef", restic.Mode{}, "/repo")
	if err == nil {
		t.Fatal("expected an error when DumpZip fails")
	}

	run := latestExportRun(t, st)
	if run.TargetID != store.FlashTargetID || run.Status != "failed" {
		t.Fatalf("a failed export must record a failed export run on the flash target, got %+v", run)
	}
	if run.Error == "" {
		t.Fatalf("the failed export run must carry the error text, got %+v", run)
	}

	dir := filepath.Join(root, "export")
	if _, statErr := os.Stat(filepath.Join(dir, ".flash-export.tmp.zip")); !os.IsNotExist(statErr) {
		t.Fatalf("temp file must be removed on dump error, stat err = %v", statErr)
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("no zip must be written on dump error, dir = %v", names)
	}
}

// A disabled export, or one without a path, does nothing and records no run.
func TestExportFlashZipDisabled(t *testing.T) {
	cases := []struct {
		name     string
		settings store.Settings
	}{
		{"disabled", store.Settings{FlashZipExportEnabled: false, FlashZipExportPath: "export"}},
		{"empty path", store.Settings{FlashZipExportEnabled: true, FlashZipExportPath: ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			fake := &flashZipFakeEngine{dumpBytes: []byte("nope")}
			st := newFlashExportStore(t)
			svc := &Service{
				cfg:    config.Config{HostMountRoot: root, FlashDir: "/boot"},
				engine: fake,
				store:  st,
			}

			if err := svc.exportFlashZip(context.Background(), tc.settings, "deadbeef", restic.Mode{}, "/repo"); err != nil {
				t.Fatalf("exportFlashZip should be a nil no-op, got %v", err)
			}
			if fake.dumpCalls != 0 {
				t.Fatalf("DumpZip must not be called (calls = %d)", fake.dumpCalls)
			}
			if _, err := os.Stat(filepath.Join(root, "export")); !os.IsNotExist(err) {
				t.Fatalf("no output folder should be created, stat err = %v", err)
			}
			if runs, err := st.ListRuns(10); err != nil || len(runs) != 0 {
				t.Fatalf("a disabled export must record no run, got runs=%v err=%v", runs, err)
			}
		})
	}
}

// A successful export records a finished run on the flash target with the zip
// size as its byte count.
func TestExportFlashZipRecordsRun(t *testing.T) {
	root := t.TempDir()
	fake := &flashZipFakeEngine{dumpBytes: buildTestStoredZip(t, map[string][]byte{"bzimage": []byte("digest-me")})}
	st := newFlashExportStore(t)
	svc := &Service{
		cfg:    config.Config{HostMountRoot: root, DataDir: t.TempDir(), FlashDir: "/boot"},
		engine: fake,
		store:  st,
	}
	settings := store.Settings{
		FlashZipExportEnabled: true,
		FlashZipExportPath:    "export",
	}

	if err := svc.exportFlashZip(context.Background(), settings, "deadbeef", restic.Mode{}, "/repo"); err != nil {
		t.Fatalf("exportFlashZip: %v", err)
	}

	run := latestExportRun(t, st)
	if run.TargetID != store.FlashTargetID {
		t.Fatalf("export run target = %q, want %q", run.TargetID, store.FlashTargetID)
	}
	if run.Status != "success" || run.FinishedAt == nil {
		t.Fatalf("a successful export must record a finished success run, got %+v", run)
	}
	fi, err := os.Stat(filepath.Join(root, "export", "flash-latest.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if run.Bytes != fi.Size() {
		t.Fatalf("export run bytes = %d, want the zip size %d", run.Bytes, fi.Size())
	}
}

// The export publishes a begin and an end progress event keyed "export:flash",
// and a failed dump still sends the end event.
func TestExportFlashZipEmitsLiveProgress(t *testing.T) {
	root := t.TempDir()
	fake := &flashZipFakeEngine{dumpBytes: buildTestStoredZip(t, map[string][]byte{"bzimage": []byte("live")})}
	prog := progress.NewStore()
	svc := &Service{
		cfg:      config.Config{HostMountRoot: root, DataDir: t.TempDir(), FlashDir: "/boot"},
		engine:   fake,
		store:    newFlashExportStore(t),
		progress: prog,
	}
	settings := store.Settings{
		FlashZipExportEnabled: true,
		FlashZipExportPath:    "export",
	}
	ch, cancel := prog.Subscribe()
	defer cancel()

	if err := svc.exportFlashZip(context.Background(), settings, "deadbeef", restic.Mode{}, "/repo"); err != nil {
		t.Fatalf("exportFlashZip: %v", err)
	}
	begin, term := drainTwoProgressEvents(t, ch)
	if begin.Key != "export:flash" || begin.Phase != "maintenance" || !begin.Active {
		t.Fatalf("begin event = %+v, want Key=export:flash Phase=maintenance Active=true", begin)
	}
	if term.Key != "export:flash" || term.Phase != "maintenance" || term.Active || term.Percent != 100 {
		t.Fatalf("terminal event = %+v, want Key=export:flash Phase=maintenance Active=false Percent=100", term)
	}

	fake.dumpErr = errors.New("boom")
	if err := svc.exportFlashZip(context.Background(), settings, "cafebabe", restic.Mode{}, "/repo"); err == nil {
		t.Fatal("expected an error when DumpZip fails")
	}
	begin, term = drainTwoProgressEvents(t, ch)
	if begin.Key != "export:flash" || !begin.Active {
		t.Fatalf("failure begin event = %+v, want Key=export:flash Active=true", begin)
	}
	if term.Key != "export:flash" || term.Active || term.Percent != 0 {
		t.Fatalf("failure terminal event = %+v, want Key=export:flash Active=false Percent=0", term)
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
