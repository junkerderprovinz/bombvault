package restic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// fakeResticEnv turns a child copy of the test binary into a restic stand-in
// whatever arguments it gets, which a test needs for a command whose argv a
// builder makes.
const fakeResticEnv = "BOMBVAULT_FAKE_RESTIC"

// unreadableLines are what restic --json writes for two files it could not
// read: one error object each, then a plain warning.
const unreadableLines = `{"message_type":"error","error":{"message":"open /data/private/key.pem: permission denied"},"during":"archival","item":"/data/private/key.pem"}
{"message_type":"error","error":{"message":"open /data/private/db.sqlite: permission denied"},"during":"archival","item":"/data/private/db.sqlite"}`

func TestMain(m *testing.M) {
	switch os.Getenv(fakeResticEnv) {
	case "sleep":
		time.Sleep(30 * time.Second)
		os.Exit(1)
	case "snapshots-unreadable":
		// What restic 0.19.1 does with a snapshots folder it cannot read.
		fmt.Println("[]")
		fmt.Fprintln(os.Stderr, "List(snapshot) failed: open /repo/snapshots: permission denied")
		fmt.Fprintln(os.Stderr, "could not load snapshots: open /repo/snapshots: permission denied")
		os.Exit(0)
	case "snapshot-damaged":
		fmt.Println(`[{"id":"aa11","time":"2026-11-05T07:16:14Z","paths":["/s"]}]`)
		fmt.Fprintln(os.Stderr, `Ignoring "00": failed to load snapshot 00: LoadRaw(<snapshot/00>): invalid data returned`)
		os.Exit(0)
	case "restore-unwritten", "restore-owner-only":
		fakeRestoreAnswer(os.Getenv(fakeResticEnv))
	case "exit3":
		fmt.Println(`{"message_type":"summary","snapshot_id":"5e1f","files_new":2}`)
		fmt.Fprintln(os.Stderr, unreadableLines)
		fmt.Fprintln(os.Stderr, "Warning: at least one source file could not be read")
		os.Exit(3)
	}
	os.Exit(m.Run())
}

// fakeRestoreAnswer is restic 0.19.1 on a target that refuses chown: the restore
// fails on metadata errors alone. Its dry run then says whether the files arrived.
func fakeRestoreAnswer(mode string) {
	if !slices.Contains(os.Args, "--dry-run") {
		fmt.Fprintln(os.Stderr, `{"message_type":"error","error":{"message":"lchown /t/a.txt: operation not permitted"},"during":"restore","item":"/a.txt"}`)
		fmt.Fprintln(os.Stderr, `{"message_type":"exit_error","code":1,"message":"Fatal: There were 1 errors"}`)
		os.Exit(1)
	}
	if mode == "restore-unwritten" {
		fmt.Println(`{"message_type":"verbose_status","action":"updated","item":"/a.txt","size":4}`)
	} else {
		fmt.Println(`{"message_type":"verbose_status","action":"unchanged","item":"/a.txt","size":4}`)
	}
	// A directory that is there and a link that is there read "restored" in every
	// dry run.
	fmt.Println(`{"message_type":"verbose_status","action":"restored","item":"/","size":0}`)
	os.Exit(0)
}

func TestARestoreThatOnlyReportsOwnershipErrorsAndWroteNothingFails(t *testing.T) {
	r := Restic{Bin: os.Args[0]}
	t.Setenv(fakeResticEnv, "restore-unwritten")

	err := r.RestoreInclude(context.Background(), "/repo", "aa11", "/a.txt", t.TempDir(), Mode{})

	if errors.Is(err, ErrRestoreMetadataOnly) {
		t.Fatalf("err = %v, which reads as all data present", err)
	}
	if err == nil || !strings.HasPrefix(err.Error(), ReasonFilesNotWritten+": 1 ") {
		t.Fatalf("err = %v, want it to say one file did not arrive", err)
	}
}

func TestARestoreWhereOnlyOwnershipIsRefusedStaysAWarning(t *testing.T) {
	r := Restic{Bin: os.Args[0]}
	t.Setenv(fakeResticEnv, "restore-owner-only")

	err := r.RestoreInclude(context.Background(), "/repo", "aa11", "/a.txt", t.TempDir(), Mode{})

	if !errors.Is(err, ErrRestoreMetadataOnly) {
		t.Fatalf("err = %v, want the metadata-only warning", err)
	}
}

func TestACancelledCopyUnwrapsToContextCanceled(t *testing.T) {
	r := Restic{Bin: os.Args[0]}
	t.Setenv(fakeResticEnv, "sleep")

	for _, withSink := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if withSink {
			ctx = progress.WithCopySink(ctx, func(progress.CopyProgress) {})
		}
		time.AfterFunc(200*time.Millisecond, cancel)
		err := r.Copy(ctx, "/dest", "/src", nil, Limits{}, Mode{})
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("sink=%v: a cancelled copy returned %v, want context.Canceled", withSink, err)
		}
	}
}

func TestASnapshotListingThatFailedIsAnError(t *testing.T) {
	r := Restic{Bin: os.Args[0]}
	t.Setenv(fakeResticEnv, "snapshots-unreadable")
	for _, ctx := range []context.Context{context.Background(), progress.WithSink(context.Background(), func(float64) {})} {
		snaps, err := r.Snapshots(ctx, "/repo", Mode{})
		if err == nil || !strings.Contains(err.Error(), "could not load snapshots") {
			t.Fatalf("listing = %v, %v, want the error restic printed", snaps, err)
		}
		if _, err := r.SnapshotsMeta(ctx, "/repo", Mode{}); err == nil {
			t.Fatal("the listing with counters reads a failure as no backups")
		}
	}
}

func TestADamagedSnapshotLeavesTheOthersListed(t *testing.T) {
	r := Restic{Bin: os.Args[0]}
	t.Setenv(fakeResticEnv, "snapshot-damaged")
	snaps, err := r.Snapshots(context.Background(), "/repo", Mode{})
	if err != nil || len(snaps) != 1 {
		t.Fatalf("listing = %v, %v, want the one good snapshot", snaps, err)
	}
}

func TestABackupThatSkippedUnreadableFilesSaysWhich(t *testing.T) {
	r := Restic{Bin: os.Args[0]}
	t.Setenv(fakeResticEnv, "exit3")

	sum, err := r.Backup(context.Background(), "/repo", []string{"/data"}, nil, Mode{})
	if err != nil {
		t.Fatalf("an exit-3 backup with a snapshot failed: %v", err)
	}
	if sum.SnapshotID != "5e1f" || !strings.Contains(sum.Unreadable, "/data/private/key.pem") || !strings.Contains(sum.Unreadable, "/data/private/db.sqlite") {
		t.Fatalf("summary = %+v, want the snapshot and the files restic could not read", sum)
	}
}
