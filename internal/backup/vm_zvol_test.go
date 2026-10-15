package backup_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// fakeZFSHost records the host commands of a zvol backup and restore and
// keeps the zvols that exist with their sizes.
type fakeZFSHost struct {
	log []string

	snapshotCreateErr error
	cloneErr          error
	destroyErr        error
	readErr           error
	readWait          error // what the wait func of ReadVolume returns
	writeErr          error

	readData []byte
	volumes  map[string]int64
	written  map[string][]byte

	snapshots    []backup.ZFSSnapshot
	snapshotsErr error
	// onRead runs while a backup reads its clone.
	onRead func()
}

func (f *fakeZFSHost) SnapshotCreate(_ context.Context, dataset, snapName string) error {
	f.log = append(f.log, "snapshot:"+dataset+"@"+snapName)
	return f.snapshotCreateErr
}

func (f *fakeZFSHost) Snapshots(_ context.Context, dataset string) ([]backup.ZFSSnapshot, error) {
	f.log = append(f.log, "snapshots:"+dataset)
	return f.snapshots, f.snapshotsErr
}

func (f *fakeZFSHost) Clone(_ context.Context, snapshot, clone string) error {
	f.log = append(f.log, "clone:"+snapshot+"->"+clone)
	return f.cloneErr
}

func (f *fakeZFSHost) Destroy(_ context.Context, name string) error {
	f.log = append(f.log, "destroy:"+name)
	return f.destroyErr
}

func (f *fakeZFSHost) VolSize(_ context.Context, dataset string) (int64, bool, error) {
	f.log = append(f.log, "volsize:"+dataset)
	size, ok := f.volumes[dataset]
	return size, ok, nil
}

func (f *fakeZFSHost) CreateVolume(_ context.Context, dataset string, size int64) error {
	f.log = append(f.log, "create:"+dataset+":"+strconv.FormatInt(size, 10))
	return nil
}

func (f *fakeZFSHost) SetVolSize(_ context.Context, dataset string, size int64) error {
	f.log = append(f.log, "resize:"+dataset+":"+strconv.FormatInt(size, 10))
	return nil
}

func (f *fakeZFSHost) ReadVolume(_ context.Context, dataset string) (io.ReadCloser, func() error, error) {
	f.log = append(f.log, "read:"+dataset)
	if f.onRead != nil {
		f.onRead()
	}
	if f.readErr != nil {
		return nil, nil, f.readErr
	}
	return io.NopCloser(bytes.NewReader(f.readData)), func() error { return f.readWait }, nil
}

func (f *fakeZFSHost) WriteVolume(_ context.Context, rd io.Reader, dataset string) error {
	f.log = append(f.log, "write:"+dataset)
	if f.writeErr != nil {
		return f.writeErr
	}
	data, _ := io.ReadAll(rd)
	if f.written == nil {
		f.written = map[string][]byte{}
	}
	f.written[dataset] = data
	return nil
}

type fakeZvolRestic struct {
	log []string

	backupErr     error
	backupSummary backup.Summary
	capturedStdin []byte
	capturedPath  string
	capturedTags  []string
	// stdinByPath keeps what each call read, and summaryByPath, when it names
	// a path, answers that call in place of backupSummary.
	stdinByPath   map[string][]byte
	summaryByPath map[string]backup.Summary

	dumpErr  error
	dumpData []byte
}

func (r *fakeZvolRestic) BackupStdin(_ context.Context, repo string, rd io.Reader, path string, tags []string) (backup.Summary, error) {
	r.log = append(r.log, "backupStdin:"+repo+":"+path+":"+strings.Join(tags, ","))
	r.capturedPath = path
	r.capturedTags = tags
	data, _ := io.ReadAll(rd)
	r.capturedStdin = data
	if r.stdinByPath == nil {
		r.stdinByPath = map[string][]byte{}
	}
	r.stdinByPath[path] = data
	if r.backupErr != nil {
		return backup.Summary{}, r.backupErr
	}
	if sum, ok := r.summaryByPath[path]; ok {
		return sum, nil
	}
	return r.backupSummary, nil
}

func (r *fakeZvolRestic) DumpTo(_ context.Context, repo, snapshotID, path string, w io.Writer) error {
	r.log = append(r.log, "dumpTo:"+repo+":"+snapshotID+":"+path)
	if r.dumpErr != nil {
		return r.dumpErr
	}
	_, err := w.Write(r.dumpData)
	return err
}

func sampleBackupZvolDeps(host *fakeZFSHost, r *fakeZvolRestic) backup.BackupZvolDeps {
	return backup.BackupZvolDeps{
		Dataset:  "tank/vms/truenasvm/disk0",
		SnapName: "bombvault-20260816120000",
		RepoPath: "/repo/vms",
		Tags:     []string{"vm:truenasvm"},
		Host:     host,
		Restic:   r,
	}
}

func TestAZvolIsBackedUpAsTheRawImageOfASnapshotClone(t *testing.T) {
	host := &fakeZFSHost{readData: []byte("raw disk image")}
	r := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "abc123", Bytes: 42}}

	sum, err := backup.BackupZvolDisk(t.Context(), sampleBackupZvolDeps(host, r))
	if err != nil {
		t.Fatalf("BackupZvolDisk: %v", err)
	}
	if sum.SnapshotID != "abc123" || sum.Bytes != 42 {
		t.Fatalf("summary = %+v, want restic's", sum)
	}
	want := []string{
		"snapshots:tank/vms/truenasvm/disk0",
		"snapshot:tank/vms/truenasvm/disk0@bombvault-20260816120000",
		"clone:tank/vms/truenasvm/disk0@bombvault-20260816120000->tank/vms/truenasvm/disk0-bombvault-20260816120000",
		"read:tank/vms/truenasvm/disk0-bombvault-20260816120000",
		"destroy:tank/vms/truenasvm/disk0-bombvault-20260816120000",
		"destroy:tank/vms/truenasvm/disk0@bombvault-20260816120000",
	}
	if strings.Join(host.log, "\n") != strings.Join(want, "\n") {
		t.Fatalf("host calls\n%s\nwant\n%s", strings.Join(host.log, "\n"), strings.Join(want, "\n"))
	}
	if string(r.capturedStdin) != "raw disk image" {
		t.Fatalf("restic read %q, want the clone's contents", r.capturedStdin)
	}
	if r.capturedPath != "/vm-disks/tank/vms/truenasvm/disk0.img" {
		t.Fatalf("image path = %q, want one that is the same on every backup", r.capturedPath)
	}
}

func TestAZvolBackupCleansUpWhateverItCreated(t *testing.T) {
	const snap = "destroy:tank/vms/truenasvm/disk0@bombvault-20260816120000"
	const clone = "destroy:tank/vms/truenasvm/disk0-bombvault-20260816120000"
	cases := []struct {
		name      string
		host      *fakeZFSHost
		restic    *fakeZvolRestic
		destroyed []string
	}{
		{"snapshot fails", &fakeZFSHost{snapshotCreateErr: errors.New("no such dataset")}, &fakeZvolRestic{}, nil},
		{"clone fails", &fakeZFSHost{cloneErr: errors.New("out of space")}, &fakeZvolRestic{}, []string{snap}},
		{"read does not start", &fakeZFSHost{readErr: errors.New("no device")}, &fakeZvolRestic{}, []string{clone, snap}},
		{"read dies halfway", &fakeZFSHost{readWait: errors.New("dd: input/output error")}, &fakeZvolRestic{}, []string{clone, snap}},
		{"restic fails", &fakeZFSHost{}, &fakeZvolRestic{backupErr: errors.New("repository locked")}, []string{clone, snap}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := backup.BackupZvolDisk(t.Context(), sampleBackupZvolDeps(c.host, c.restic)); err == nil {
				t.Fatal("want an error")
			}
			var destroyed []string
			for _, call := range c.host.log {
				if strings.HasPrefix(call, "destroy:") {
					destroyed = append(destroyed, call)
				}
			}
			if strings.Join(destroyed, ",") != strings.Join(c.destroyed, ",") {
				t.Fatalf("destroyed %v, want %v", destroyed, c.destroyed)
			}
		})
	}
}

func TestAZvolBackupThatCannotRemoveItsCloneStillSucceeds(t *testing.T) {
	t.Cleanup(backup.SetDestroyRetryDelayForTest(0))
	host := &fakeZFSHost{destroyErr: errors.New("dataset is busy")}
	r := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "abc123"}}

	sum, err := backup.BackupZvolDisk(t.Context(), sampleBackupZvolDeps(host, r))
	if err != nil || sum.SnapshotID != "abc123" {
		t.Fatalf("got %+v, %v; want the backup despite the leftover clone", sum, err)
	}
	var tries int
	for _, call := range host.log {
		if call == "destroy:tank/vms/truenasvm/disk0-bombvault-20260816120000" {
			tries++
		}
	}
	if tries < 2 {
		t.Fatalf("the clone was tried %d times, want it tried again after udev let go of it", tries)
	}
}

func TestABackupRemovesWhatAnInterruptedOneLeftOnItsZvol(t *testing.T) {
	t.Cleanup(backup.SetDestroyRetryDelayForTest(0))
	const ds = "tank/vms/truenasvm/disk0"
	host := &fakeZFSHost{snapshots: []backup.ZFSSnapshot{
		{Name: ds + "@bombvault-vm-20261015010000", Clones: []string{ds + "-bombvault-vm-20261015010000"}},
		{Name: ds + "@bombvault-vm-20261015020000"},
		{Name: ds + "@bombvault-before-restore-20261015010000"},
		{Name: ds + "@bombvault-20261015010000"},
		{Name: ds + "@bombvault-vm-2026101503000"},
		{Name: ds + "@mine"},
		{Name: ds + "@bombvault-vm-20261015040000", Clones: []string{"tank/vms/mytest"}},
		{Name: ds + "1@bombvault-vm-20261015050000"},
	}}

	if _, err := backup.BackupZvolDisk(t.Context(), sampleBackupZvolDeps(host, &fakeZvolRestic{})); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"snapshots:" + ds,
		"destroy:" + ds + "-bombvault-vm-20261015010000",
		"destroy:" + ds + "@bombvault-vm-20261015010000",
		"destroy:" + ds + "@bombvault-vm-20261015020000",
		"snapshot:" + ds + "@bombvault-20260816120000",
	}
	if len(host.log) < len(want) || !slices.Equal(host.log[:len(want)], want) {
		t.Fatalf("host calls %v, want them to start with %v", host.log, want)
	}
}

func TestABackupLeavesTheZvolSnapshotOfAnotherRunningBackup(t *testing.T) {
	const ds = "tank/vms/truenasvm/disk0"
	running := ds + "@bombvault-20260816120000"
	second := &fakeZFSHost{snapshots: []backup.ZFSSnapshot{{Name: running, Clones: []string{ds + "-bombvault-20260816120000"}}}}
	first := &fakeZFSHost{onRead: func() {
		d := sampleBackupZvolDeps(second, &fakeZvolRestic{})
		d.SnapName = "bombvault-vm-20260816120500"
		if _, err := backup.BackupZvolDisk(t.Context(), d); err != nil {
			t.Error(err)
		}
	}}
	deps := sampleBackupZvolDeps(first, &fakeZvolRestic{})
	deps.SnapName = "bombvault-vm-20260816120000"
	running = ds + "@" + deps.SnapName
	second.snapshots[0] = backup.ZFSSnapshot{Name: running, Clones: []string{ds + "-" + deps.SnapName}}

	if _, err := backup.BackupZvolDisk(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	for _, call := range second.log {
		if strings.Contains(call, deps.SnapName) {
			t.Fatalf("the second backup touched the first one's %s", call)
		}
	}
}

func TestABackupWhoseLeftoverCheckFailsStillRuns(t *testing.T) {
	host := &fakeZFSHost{snapshotsErr: errors.New("ssh: connection reset")}
	r := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "abc123"}}

	if sum, err := backup.BackupZvolDisk(t.Context(), sampleBackupZvolDeps(host, r)); err != nil || sum.SnapshotID != "abc123" {
		t.Fatalf("got %+v, %v; want the backup to go on", sum, err)
	}
}

func sampleRestoreZvolDeps(host *fakeZFSHost, r *fakeZvolRestic) backup.RestoreZvolDeps {
	return backup.RestoreZvolDeps{
		Dataset:    "tank/vms/truenasvm/disk0",
		Size:       1 << 30,
		RepoPath:   "/repo/vms",
		SnapshotID: "abc123def456",
		StdinPath:  "/vm-disks/tank/vms/truenasvm/disk0.img",
		Host:       host,
		Restic:     r,
	}
}

func TestAZvolIsRestoredOntoItselfAfterASafetySnapshot(t *testing.T) {
	host := &fakeZFSHost{volumes: map[string]int64{"tank/vms/truenasvm/disk0": 1 << 30}}
	r := &fakeZvolRestic{dumpData: []byte("raw disk image")}

	safety, err := backup.RestoreZvolDisk(t.Context(), sampleRestoreZvolDeps(host, r))
	if err != nil {
		t.Fatalf("RestoreZvolDisk: %v", err)
	}
	if !strings.HasPrefix(safety, "tank/vms/truenasvm/disk0@bombvault-before-restore-") {
		t.Fatalf("safety snapshot = %q", safety)
	}
	if got := string(host.written["tank/vms/truenasvm/disk0"]); got != "raw disk image" {
		t.Fatalf("wrote %q onto the zvol, want the image", got)
	}
	snapAt, writeAt := -1, -1
	for i, call := range host.log {
		switch {
		case strings.HasPrefix(call, "snapshot:tank/vms/truenasvm/disk0@bombvault-before-restore-"):
			snapAt = i
		case call == "write:tank/vms/truenasvm/disk0":
			writeAt = i
		case strings.HasPrefix(call, "resize:"), strings.HasPrefix(call, "create:"):
			t.Fatalf("%s for a zvol of the right size", call)
		}
	}
	if snapAt < 0 || writeAt < snapAt {
		t.Fatalf("host calls %v, want the snapshot before the write", host.log)
	}
}

func TestARestoreGivesTheZvolItsSizeBack(t *testing.T) {
	host := &fakeZFSHost{volumes: map[string]int64{"tank/vms/truenasvm/disk0": 2 << 30}}
	if _, err := backup.RestoreZvolDisk(t.Context(), sampleRestoreZvolDeps(host, &fakeZvolRestic{})); err != nil {
		t.Fatal(err)
	}
	if !vmContains(host.log, "resize:tank/vms/truenasvm/disk0:1073741824") {
		t.Fatalf("host calls %v, want a resize to the backed up size", host.log)
	}
}

func TestARestoreCreatesAZvolThatIsGone(t *testing.T) {
	host := &fakeZFSHost{}
	r := &fakeZvolRestic{dumpData: []byte("raw disk image")}

	safety, err := backup.RestoreZvolDisk(t.Context(), sampleRestoreZvolDeps(host, r))
	if err != nil {
		t.Fatal(err)
	}
	if safety != "" {
		t.Fatalf("safety snapshot %q of a zvol that did not exist", safety)
	}
	if !vmContains(host.log, "create:tank/vms/truenasvm/disk0:1073741824") || string(host.written["tank/vms/truenasvm/disk0"]) != "raw disk image" {
		t.Fatalf("host calls %v, want the zvol created and written", host.log)
	}
}

func TestAZvolRestoreReportsAFailedDumpOrWrite(t *testing.T) {
	for name, c := range map[string]struct {
		host   *fakeZFSHost
		restic *fakeZvolRestic
	}{
		"dump":  {&fakeZFSHost{}, &fakeZvolRestic{dumpErr: errors.New("snapshot not found")}},
		"write": {&fakeZFSHost{writeErr: errors.New("no space left on device")}, &fakeZvolRestic{dumpData: []byte("x")}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := backup.RestoreZvolDisk(t.Context(), sampleRestoreZvolDeps(c.host, c.restic)); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

// A write that fails before reading anything must not leave the dump blocked
// on the pipe.
func TestAZvolWriteThatFailsEarlyDoesNotHangTheRestore(t *testing.T) {
	host := &fakeZFSHost{writeErr: errors.New("ssh: connection refused")}
	r := &fakeZvolRestic{dumpData: bytes.Repeat([]byte("x"), 1<<20)}

	done := make(chan struct{})
	go func() {
		_, _ = backup.RestoreZvolDisk(context.Background(), sampleRestoreZvolDeps(host, r))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RestoreZvolDisk hung after the write failed")
	}
}
