package backup_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// fakeZFSHost records the ZFS calls of BackupZvolDisk and RestoreZvolDisk.
// These tests check the order of operations and the cleanup, not how a real
// zfs send or receive behaves on TrueNAS.
type fakeZFSHost struct {
	log []string

	snapshotCreateErr  error
	snapshotDestroyErr error

	streamSendErr  error
	streamSendData []byte
	streamSendWait error // returned by the wait() func StreamSend hands back

	streamReceiveErr    error
	streamReceiveData   []byte
	streamReceiveTarget string
}

func (f *fakeZFSHost) SnapshotCreate(_ context.Context, dataset, snapName string) error {
	f.log = append(f.log, "snapshotCreate:"+dataset+"@"+snapName)
	return f.snapshotCreateErr
}

func (f *fakeZFSHost) SnapshotDestroy(_ context.Context, dataset, snapName string) error {
	f.log = append(f.log, "snapshotDestroy:"+dataset+"@"+snapName)
	return f.snapshotDestroyErr
}

func (f *fakeZFSHost) StreamSend(_ context.Context, dataset, snapName string) (io.ReadCloser, func() error, error) {
	f.log = append(f.log, "streamSend:"+dataset+"@"+snapName)
	if f.streamSendErr != nil {
		return nil, nil, f.streamSendErr
	}
	rc := io.NopCloser(bytes.NewReader(f.streamSendData))
	wait := func() error { return f.streamSendWait }
	return rc, wait, nil
}

func (f *fakeZFSHost) StreamReceive(_ context.Context, rd io.Reader, targetDataset string) error {
	f.log = append(f.log, "streamReceive:"+targetDataset)
	f.streamReceiveTarget = targetDataset
	data, _ := io.ReadAll(rd)
	f.streamReceiveData = data
	return f.streamReceiveErr
}

type fakeZvolRestic struct {
	log []string

	backupErr     error
	backupSummary backup.Summary
	capturedStdin []byte
	capturedPath  string
	capturedTags  []string

	dumpErr  error
	dumpData []byte
}

func (r *fakeZvolRestic) BackupStdin(_ context.Context, repo string, rd io.Reader, path string, tags []string) (backup.Summary, error) {
	r.log = append(r.log, "backupStdin:"+repo+":"+path+":"+strings.Join(tags, ","))
	r.capturedPath = path
	r.capturedTags = tags
	data, _ := io.ReadAll(rd)
	r.capturedStdin = data
	if r.backupErr != nil {
		return backup.Summary{}, r.backupErr
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
		Name:     "truenasvm",
		Dataset:  "tank/vms/truenasvm/disk0",
		SnapName: "bombvault-20260816120000",
		RepoPath: "/repo/vms",
		Tags:     []string{"vm:truenasvm"},
		Host:     host,
		Restic:   r,
	}
}

// TestBackupZvolDiskHappyPath checks the sequence: snapshot, zfs send into
// restic backup --stdin, destroy.
func TestBackupZvolDiskHappyPath(t *testing.T) {
	host := &fakeZFSHost{streamSendData: []byte("fake zfs send stream bytes")}
	r := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "abc123", Bytes: 42}}

	sum, err := backup.BackupZvolDisk(context.Background(), sampleBackupZvolDeps(host, r))
	if err != nil {
		t.Fatalf("BackupZvolDisk: %v", err)
	}
	if sum.SnapshotID != "abc123" || sum.Bytes != 42 {
		t.Fatalf("summary = %+v, want the restic summary passed through", sum)
	}

	wantOrder := []string{
		"snapshotCreate:tank/vms/truenasvm/disk0@bombvault-20260816120000",
		"streamSend:tank/vms/truenasvm/disk0@bombvault-20260816120000",
		"snapshotDestroy:tank/vms/truenasvm/disk0@bombvault-20260816120000",
	}
	if len(host.log) != len(wantOrder) {
		t.Fatalf("host.log = %v, want %v", host.log, wantOrder)
	}
	for i, want := range wantOrder {
		if host.log[i] != want {
			t.Fatalf("host.log[%d] = %q, want %q (full log: %v)", i, host.log[i], want, host.log)
		}
	}

	if string(r.capturedStdin) != "fake zfs send stream bytes" {
		t.Fatalf("restic BackupStdin received %q, want the zfs send stream bytes", r.capturedStdin)
	}
	if r.capturedTags == nil || r.capturedTags[0] != "vm:truenasvm" {
		t.Fatalf("restic BackupStdin tags = %v, want [vm:truenasvm]", r.capturedTags)
	}
	wantPath := "/vm-disks/tank/vms/truenasvm/disk0@bombvault-20260816120000"
	if r.capturedPath != wantPath {
		t.Fatalf("restic BackupStdin path = %q, want %q", r.capturedPath, wantPath)
	}
}

// TestBackupZvolDiskAlwaysDestroysSnapshotOnBackupFailure checks that the
// snapshot, only a consistency point, goes even when the restic backup fails.
func TestBackupZvolDiskAlwaysDestroysSnapshotOnBackupFailure(t *testing.T) {
	host := &fakeZFSHost{streamSendData: []byte("stream")}
	r := &fakeZvolRestic{backupErr: errors.New("restic: repository locked")}

	_, err := backup.BackupZvolDisk(context.Background(), sampleBackupZvolDeps(host, r))
	if err == nil {
		t.Fatal("expected an error from the failed restic backup")
	}
	if !vmContains(host.log, "snapshotDestroy:") {
		t.Fatalf("snapshot destroy was not attempted after a backup failure; host.log = %v", host.log)
	}
}

// TestBackupZvolDiskAlwaysDestroysSnapshotOnStreamFailure fails zfs send
// before restic runs.
func TestBackupZvolDiskAlwaysDestroysSnapshotOnStreamFailure(t *testing.T) {
	host := &fakeZFSHost{streamSendErr: errors.New("ssh: connection refused")}
	r := &fakeZvolRestic{}

	_, err := backup.BackupZvolDisk(context.Background(), sampleBackupZvolDeps(host, r))
	if err == nil {
		t.Fatal("expected an error from the failed zfs send stream")
	}
	if !vmContains(host.log, "snapshotDestroy:") {
		t.Fatalf("snapshot destroy was not attempted after a stream-start failure; host.log = %v", host.log)
	}
	if len(r.log) != 0 {
		t.Fatalf("restic must never be called when the stream never started; r.log = %v", r.log)
	}
}

func TestBackupZvolDiskSnapshotCreateFailureIsFatalAndSkipsRestic(t *testing.T) {
	host := &fakeZFSHost{snapshotCreateErr: errors.New("dataset does not exist")}
	r := &fakeZvolRestic{}

	_, err := backup.BackupZvolDisk(context.Background(), sampleBackupZvolDeps(host, r))
	if err == nil {
		t.Fatal("expected an error when the snapshot could not be created")
	}
	if len(r.log) != 0 {
		t.Fatalf("restic must never be called when the snapshot could not be created; r.log = %v", r.log)
	}
	if vmContains(host.log, "snapshotDestroy:") {
		t.Fatalf("nothing to destroy when the snapshot was never created; host.log = %v", host.log)
	}
}

// TestBackupZvolDiskDestroyFailureDoesNotMaskBackupSuccess checks that a failed
// destroy does not fail the backup: a leftover snapshot is untidy, not data
// loss.
func TestBackupZvolDiskDestroyFailureDoesNotMaskBackupSuccess(t *testing.T) {
	host := &fakeZFSHost{
		streamSendData:     []byte("stream"),
		snapshotDestroyErr: errors.New("dataset busy"),
	}
	r := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "abc123"}}

	sum, err := backup.BackupZvolDisk(context.Background(), sampleBackupZvolDeps(host, r))
	if err != nil {
		t.Fatalf("a snapshot-destroy failure must not fail the backup: %v", err)
	}
	if sum.SnapshotID != "abc123" {
		t.Fatalf("summary = %+v, want the successful restic summary", sum)
	}
	if !vmContains(host.log, "snapshotDestroy:") {
		t.Fatalf("snapshot destroy must still be attempted even though it fails; host.log = %v", host.log)
	}
}

func sampleRestoreZvolDeps(host *fakeZFSHost, r *fakeZvolRestic) backup.RestoreZvolDeps {
	return backup.RestoreZvolDeps{
		SourceDataset: "tank/vms/truenasvm/disk0",
		RepoPath:      "/repo/vms",
		SnapshotID:    "abc123def456",
		StdinPath:     "/vm-disks/tank/vms/truenasvm/disk0@bombvault-20260816120000",
		Host:          host,
		Restic:        r,
	}
}

// TestRestoreZvolDiskNeverTargetsSourceDataset checks the dataset the host
// received into. zfs receive into an existing dataset can destroy live data,
// so it must never be the source.
func TestRestoreZvolDiskNeverTargetsSourceDataset(t *testing.T) {
	host := &fakeZFSHost{}
	r := &fakeZvolRestic{dumpData: []byte("restic dump bytes")}

	deps := sampleRestoreZvolDeps(host, r)
	target, err := backup.RestoreZvolDisk(context.Background(), deps)
	if err != nil {
		t.Fatalf("RestoreZvolDisk: %v", err)
	}
	if target == deps.SourceDataset {
		t.Fatalf("RestoreZvolDisk returned the source dataset %q as the restore target", target)
	}
	if host.streamReceiveTarget != target {
		t.Fatalf("RestoreZvolDisk returned target %q but issued `zfs receive` against %q", target, host.streamReceiveTarget)
	}
	if host.streamReceiveTarget == deps.SourceDataset {
		t.Fatalf("the actual StreamReceive call targeted the live source dataset %q", deps.SourceDataset)
	}
	if !strings.HasPrefix(target, deps.SourceDataset+"-bombvault-restore-") {
		t.Fatalf("target dataset %q does not carry the expected bombvault-restore marker", target)
	}

	if string(host.streamReceiveData) != "restic dump bytes" {
		t.Fatalf("zfs receive stdin = %q, want the restic dump bytes", host.streamReceiveData)
	}
}

// TestRestoreZvolDiskUsesRestoreBaseDatasetWhenSet covers a restore onto
// another instance, where prepareRestoreVMForTarget rebases the dataset onto
// the destination pool because the source pool does not exist there.
func TestRestoreZvolDiskUsesRestoreBaseDatasetWhenSet(t *testing.T) {
	host := &fakeZFSHost{}
	r := &fakeZvolRestic{dumpData: []byte("restic dump bytes")}

	deps := sampleRestoreZvolDeps(host, r)
	deps.RestoreBaseDataset = "flashpool/vms/truenasvm/disk0"

	target, err := backup.RestoreZvolDisk(context.Background(), deps)
	if err != nil {
		t.Fatalf("RestoreZvolDisk: %v", err)
	}
	if !strings.HasPrefix(target, deps.RestoreBaseDataset+"-bombvault-restore-") {
		t.Fatalf("target dataset %q must be derived from RestoreBaseDataset %q, not SourceDataset %q", target, deps.RestoreBaseDataset, deps.SourceDataset)
	}
	if strings.HasPrefix(target, deps.SourceDataset+"-bombvault-restore-") {
		t.Fatalf("target dataset %q was derived from the source dataset %q; the cross-instance rebase did not take effect", target, deps.SourceDataset)
	}
	if host.streamReceiveTarget != target {
		t.Fatalf("RestoreZvolDisk returned target %q but issued `zfs receive` against %q", target, host.streamReceiveTarget)
	}
	if strings.HasPrefix(host.streamReceiveTarget, "tank/") {
		t.Fatalf("the actual StreamReceive call targeted the source pool %q instead of the destination pool", host.streamReceiveTarget)
	}
}

func TestRestoreZvolDiskFallsBackToSourceDatasetWhenRestoreBaseDatasetEmpty(t *testing.T) {
	host := &fakeZFSHost{}
	r := &fakeZvolRestic{dumpData: []byte("restic dump bytes")}

	deps := sampleRestoreZvolDeps(host, r)
	deps.RestoreBaseDataset = "" // restore on the same instance

	target, err := backup.RestoreZvolDisk(context.Background(), deps)
	if err != nil {
		t.Fatalf("RestoreZvolDisk: %v", err)
	}
	if !strings.HasPrefix(target, deps.SourceDataset+"-bombvault-restore-") {
		t.Fatalf("target dataset %q must fall back to SourceDataset %q when RestoreBaseDataset is empty", target, deps.SourceDataset)
	}
}

func TestRestoreZvolDiskPropagatesDumpFailure(t *testing.T) {
	host := &fakeZFSHost{}
	r := &fakeZvolRestic{dumpErr: errors.New("restic: snapshot not found")}

	_, err := backup.RestoreZvolDisk(context.Background(), sampleRestoreZvolDeps(host, r))
	if err == nil {
		t.Fatal("expected an error when restic dump fails")
	}
}

func TestRestoreZvolDiskPropagatesReceiveFailure(t *testing.T) {
	host := &fakeZFSHost{streamReceiveErr: errors.New("zfs: receive failed: dataset already exists")}
	r := &fakeZvolRestic{dumpData: []byte("data")}

	_, err := backup.RestoreZvolDisk(context.Background(), sampleRestoreZvolDeps(host, r))
	if err == nil {
		t.Fatal("expected an error when zfs receive fails")
	}
}

// TestRestoreZvolDiskReceiveNeverBlocksForeverOnEarlyFailure has StreamReceive
// fail without reading its input; the goroutine writing restic's dump into the
// pipe must not hang.
func TestRestoreZvolDiskReceiveNeverBlocksForeverOnEarlyFailure(t *testing.T) {
	host := &earlyFailZFSHost{err: errors.New("ssh: connection refused")}
	r := &fakeZvolRestic{dumpData: bytes.Repeat([]byte("x"), 1<<20)} // 1 MiB: bigger than a pipe's implicit buffering

	done := make(chan struct{})
	go func() {
		_, _ = backup.RestoreZvolDisk(context.Background(), backup.RestoreZvolDeps{
			SourceDataset: "tank/d", RepoPath: "/repo", SnapshotID: "abc123", StdinPath: "/p",
			Host: host, Restic: r,
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RestoreZvolDisk hung: the DumpTo writer goroutine was never unblocked after StreamReceive failed early")
	}
}

// earlyFailZFSHost's StreamReceive fails without reading rd, like an SSH
// connection that never accepted stdin.
type earlyFailZFSHost struct{ err error }

func (h *earlyFailZFSHost) SnapshotCreate(context.Context, string, string) error  { return nil }
func (h *earlyFailZFSHost) SnapshotDestroy(context.Context, string, string) error { return nil }
func (h *earlyFailZFSHost) StreamSend(context.Context, string, string) (io.ReadCloser, func() error, error) {
	return nil, nil, nil
}
func (h *earlyFailZFSHost) StreamReceive(_ context.Context, _ io.Reader, _ string) error {
	return h.err
}

// TestRestoreZvolDiskDistinctAcrossRepeatedCalls checks that two restores of
// the same dataset, such as a retry, land on two different new datasets.
func TestRestoreZvolDiskDistinctAcrossRepeatedCalls(t *testing.T) {
	host := &fakeZFSHost{}
	r := &fakeZvolRestic{dumpData: []byte("data")}
	deps := sampleRestoreZvolDeps(host, r)

	first, err := backup.RestoreZvolDisk(context.Background(), deps)
	if err != nil {
		t.Fatalf("first RestoreZvolDisk: %v", err)
	}
	time.Sleep(time.Millisecond) // ensure the nanosecond clock advances
	second, err := backup.RestoreZvolDisk(context.Background(), deps)
	if err != nil {
		t.Fatalf("second RestoreZvolDisk: %v", err)
	}
	if first == second {
		t.Fatalf("two restore calls produced the same target dataset %q", first)
	}
}
