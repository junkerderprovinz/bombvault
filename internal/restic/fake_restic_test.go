package restic

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	case "exit3":
		fmt.Println(`{"message_type":"summary","snapshot_id":"5e1f","files_new":2}`)
		fmt.Fprintln(os.Stderr, unreadableLines)
		fmt.Fprintln(os.Stderr, "Warning: at least one source file could not be read")
		os.Exit(3)
	}
	os.Exit(m.Run())
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
