package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// probeEngine lists a fixed snapshot and restores what it is asked for with
// the listed sizes, or with sizeOff bytes more to fake a damaged file.
type probeEngine struct {
	ResticEngine
	entries  []restic.FileEntry
	sizeOff  int64
	restores [][]string
	dumped   []string
	dumpErr  error
	target   string
	calls    []string
	// dumps are whole files DumpRaw hands back instead of an endless stream.
	dumps map[string][]byte
}

func (e *probeEngine) Unlock(_ context.Context, _ string, removeAll bool, _ restic.Mode) error {
	e.calls = append(e.calls, fmt.Sprintf("Unlock(removeAll=%v)", removeAll))
	return nil
}

func (e *probeEngine) LsStream(_ context.Context, _, _ string, _ restic.Mode, onEntry func(restic.FileEntry)) error {
	for _, en := range e.entries {
		onEntry(en)
	}
	return nil
}

func (e *probeEngine) RestoreVerify(_ context.Context, _, _ string, files []string, target string, _ restic.Mode) error {
	e.restores = append(e.restores, files)
	e.calls = append(e.calls, "RestoreVerify")
	e.target = target
	sizes := map[string]int64{}
	for _, en := range e.entries {
		sizes[en.Path] = en.Size
	}
	for _, f := range files {
		dst := filepath.Join(target, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(dst, make([]byte, sizes[f]+e.sizeOff), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (e *probeEngine) DumpRaw(_ context.Context, _, _, path string, w io.Writer, _ restic.Mode) error {
	e.dumped = append(e.dumped, path)
	if e.dumpErr != nil {
		return e.dumpErr
	}
	if body, ok := e.dumps[path]; ok {
		_, err := w.Write(body)
		return err
	}
	buf := make([]byte, 1<<20)
	for {
		if _, err := w.Write(buf); err != nil {
			return fmt.Errorf("restic dump: %w", err)
		}
	}
}

func newProbeService(t *testing.T, eng *probeEngine) (*Service, store.Target) {
	s, tg, _ := newProbeServiceDB(t, eng)
	return s, tg
}

func newProbeServiceDB(t *testing.T, eng *probeEngine) (*Service, store.Target, *sql.DB) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	settings.RestoreFolder = "restore"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "nextcloud"})
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{store: st, engine: eng, cfg: config.Config{HostMountRoot: t.TempDir()}}
	return s, tg, db
}

func recordBackup(t *testing.T, st *store.Repo, targetID, snapshotID string) {
	t.Helper()
	id, err := st.StartRun(targetID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(id, "success", snapshotID, 10, ""); err != nil {
		t.Fatal(err)
	}
}

func fileEntry(p string, size int64) restic.FileEntry {
	return restic.FileEntry{Path: p, Type: "file", Size: size}
}

func TestProbeSamplerStaysWithinItsLimits(t *testing.T) {
	sampler := newProbeSampler(rand.New(rand.NewPCG(1, 2))) //nolint:gosec // G404: a fixed seed keeps the sample reproducible
	for i := range 5000 {
		sampler.add(fileEntry(fmt.Sprintf("/data/f%05d", i), int64(i)*4096))
	}
	sampler.add(fileEntry("/data/disk.img", 900<<30))
	sampler.add(fileEntry("/data/odd[1].txt", 10))
	sampler.add(restic.FileEntry{Path: "/data", Type: "dir"})

	files, total, big := sampler.pick()
	if len(files) == 0 || len(files) > probeMaxFiles {
		t.Fatalf("sample has %d files, want 1..%d", len(files), probeMaxFiles)
	}
	var sum int64
	for _, f := range files {
		sum += f.Size
		if strings.ContainsAny(f.Path, "[]") {
			t.Fatalf("a path with a glob character was sampled: %s", f.Path)
		}
	}
	if sum != total || total > probeMaxBytes {
		t.Fatalf("sample bytes = %d (reported %d), budget %d", sum, total, probeMaxBytes)
	}
	if big == nil || big.Path != "/data/disk.img" {
		t.Fatalf("the file too big for the sample must be kept for the head read, got %+v", big)
	}
}

func TestItemProbePassesWhenTheSampleComesBackWhole(t *testing.T) {
	eng := &probeEngine{entries: []restic.FileEntry{
		fileEntry("/host/user/appdata/nextcloud/config.php", 1200),
		fileEntry("/host/user/appdata/nextcloud/data/db.sqlite", 64<<10),
		{Path: "/host/user/appdata/nextcloud", Type: "dir"},
	}}
	s, tg := newProbeService(t, eng)
	recordBackup(t, s.store, tg.ID, "abcd1234")

	rec, err := s.ProbeItem(context.Background(), tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !rec.OK || rec.Files != 2 || rec.Trigger != "manual" || rec.SnapshotID != "abcd1234" {
		t.Fatalf("probe = %+v, want a passed manual probe of both files", rec)
	}
	if _, err := os.Stat(eng.target); !os.IsNotExist(err) {
		t.Fatalf("the sandbox must be gone after the probe, stat=%v", err)
	}
	latest, found, err := s.store.LatestItemProbe(tg.ID)
	if err != nil || !found || !latest.OK {
		t.Fatalf("recorded probe = %+v found=%v err=%v", latest, found, err)
	}
}

func TestItemProbeFailsOnAFileThatCameBackWrong(t *testing.T) {
	eng := &probeEngine{entries: []restic.FileEntry{fileEntry("/data/a.bin", 100)}, sizeOff: 3}
	s, tg := newProbeService(t, eng)
	recordBackup(t, s.store, tg.ID, "abcd1234")

	rec, err := s.ProbeItem(context.Background(), tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.OK || !strings.Contains(rec.Detail, "came back with 103 bytes") {
		t.Fatalf("probe = %+v, want a failure naming the size that came back", rec)
	}
}

func TestItemProbeReadsTheHeadOfAFileTooBigToRestore(t *testing.T) {
	eng := &probeEngine{entries: []restic.FileEntry{fileEntry("/domains/win11/vdisk1.img", 80<<30)}}
	s, tg := newProbeService(t, eng)
	recordBackup(t, s.store, tg.ID, "abcd1234")

	rec, err := s.ProbeItem(context.Background(), tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !rec.OK || rec.Bytes != probeDumpBytes || len(eng.restores) != 0 {
		t.Fatalf("probe = %+v restores=%v, want only the head read", rec, eng.restores)
	}

	eng.dumpErr = errors.New("ciphertext verification failed")
	rec, err = s.ProbeItem(context.Background(), tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.OK {
		t.Fatal("a head read restic refused must fail the probe")
	}
}

func TestFirstProbeRunsOnceAndOnlyForNewItems(t *testing.T) {
	eng := &probeEngine{entries: []restic.FileEntry{fileEntry("/data/a.bin", 100)}}
	s, tg := newProbeService(t, eng)
	ctx := context.Background()

	s.firstProbe(ctx, tg.ID)
	if len(eng.restores) != 0 {
		t.Fatal("an item without a backup has nothing to probe")
	}

	recordBackup(t, s.store, tg.ID, "abcd1234")
	s.firstProbe(ctx, tg.ID)
	recordBackup(t, s.store, tg.ID, "ef567890")
	s.firstProbe(ctx, tg.ID)
	if len(eng.restores) != 1 {
		t.Fatalf("probes run = %d, want exactly one after the first backup", len(eng.restores))
	}
	rec, _, _ := s.store.LatestItemProbe(tg.ID)
	if rec.Trigger != "first" || rec.SnapshotID != "abcd1234" {
		t.Fatalf("first probe = %+v", rec)
	}
}

func TestFirstProbeSkipsItemsBackedUpBeforeProbesExisted(t *testing.T) {
	eng := &probeEngine{entries: []restic.FileEntry{fileEntry("/data/a.bin", 100)}}
	s, tg, db := newProbeServiceDB(t, eng)
	recordBackup(t, s.store, tg.ID, "abcd1234")
	if _, err := db.Exec(`UPDATE runs SET finished_at = 1000 WHERE target_id = ?`, tg.ID); err != nil {
		t.Fatal(err)
	}
	s.firstProbe(context.Background(), tg.ID)
	if len(eng.restores) != 0 {
		t.Fatal("an item first backed up before the update must not be probed on its next backup")
	}
}

func TestQueueFirstProbeIsOffUntilEnabled(t *testing.T) {
	s := &Service{}
	s.queueFirstProbe("abc")
	if len(s.probeQueue) != 0 || s.probeWorking {
		t.Fatal("a Service nobody enabled probes on must not queue any")
	}
}

func TestItemProbeClearsStaleLocksBeforeItReads(t *testing.T) {
	eng := &probeEngine{entries: []restic.FileEntry{fileEntry("/data/a.bin", 100)}}
	s, tg := newProbeService(t, eng)
	recordBackup(t, s.store, tg.ID, "abcd1234")
	if _, err := s.ProbeItem(context.Background(), tg.ID); err != nil {
		t.Fatal(err)
	}
	if len(eng.calls) < 2 || eng.calls[0] != "Unlock(removeAll=false)" || eng.calls[1] != "RestoreVerify" {
		t.Fatalf("calls = %v, want only stale locks cleared before the restore", eng.calls)
	}
}

// vmProbeService is a probe of VM win11 whose newest backup is a changed-block
// snapshot of one 25-byte disk in 10-byte segments.
func vmProbeService(t *testing.T, segments ...restic.FileEntry) (*Service, *probeEngine, string) {
	t.Helper()
	manifest := `{"version":1,"checkpoint":"bombvault-1","disks":[{"dev":"vda","path":"/domains/win11/vdisk1.img","format":"qcow2","size":25,"segment":10}]}`
	eng := &probeEngine{
		entries: append([]restic.FileEntry{fileEntry("/bombvault-vm.json", int64(len(manifest)))}, segments...),
		dumps:   map[string][]byte{"/bombvault-vm.json": []byte(manifest)},
	}
	s, _ := newProbeService(t, eng)
	vm, err := s.store.UpsertVMTarget(store.VMTarget{Name: "win11", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	recordBackup(t, s.store, vm.ID, "abcd1234")
	return s, eng, vm.ID
}

func TestItemProbePassesAChangedBlockSnapshotWithEverySegment(t *testing.T) {
	s, eng, id := vmProbeService(t,
		fileEntry("/vda/00000000", 10), fileEntry("/vda/00000001", 10), fileEntry("/vda/00000002", 5))
	rec, err := s.ProbeItem(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !rec.OK || rec.Files != 4 || len(eng.restores) != 1 {
		t.Fatalf("probe = %+v restores=%v, want the segments and the manifest read back", rec, eng.restores)
	}
}

func TestItemProbeFailsAChangedBlockSnapshotMissingASegment(t *testing.T) {
	s, eng, id := vmProbeService(t, fileEntry("/vda/00000000", 10), fileEntry("/vda/00000002", 5))
	rec, err := s.ProbeItem(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if rec.OK || !strings.Contains(rec.Detail, "2 of 3 segments") {
		t.Fatalf("probe = %+v, want a failure naming the missing segment", rec)
	}
	if len(eng.restores) != 0 {
		t.Fatalf("a disk that cannot be put back together needs no sample: %v", eng.restores)
	}
}

func TestItemProbeFailsAChangedBlockSnapshotWithAShortSegment(t *testing.T) {
	s, _, id := vmProbeService(t,
		fileEntry("/vda/00000000", 10), fileEntry("/vda/00000001", 4), fileEntry("/vda/00000002", 5))
	rec, err := s.ProbeItem(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if rec.OK || !strings.Contains(rec.Detail, "holds 4 bytes, want 10") {
		t.Fatalf("probe = %+v, want a failure naming the short segment", rec)
	}
}
