package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// stagesOf runs start with a progress store on svc and returns the stage of
// every event it publishes under key, up to and including the last one.
func stagesOf(t *testing.T, svc *Service, key string, start func()) []string {
	t.Helper()
	svc.progress = progress.NewStore()
	events, stop := svc.progress.Subscribe()
	defer stop()

	start()
	var stages []string
	for {
		select {
		case e := <-events:
			if e.Key != key {
				continue
			}
			stages = append(stages, e.Stage)
			if !e.Active {
				return stages
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no last event under %s, stages so far %v", key, stages)
		}
	}
}

// The restore window takes a restore event without a stage for a restore of
// the container itself and gives up its form for it, so a dump job that
// reports one loses the result it is about to show.
func TestADumpJobNamesItsStageOnEveryProgressEvent(t *testing.T) {
	t.Run("an import", func(t *testing.T) {
		rig := newImportRig(t)
		stages := stagesOf(t, rig.svc, "container:pg", func() {
			if started, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil || !started {
				t.Fatalf("started=%v err=%v", started, err)
			}
		})
		waitForDetachedRun(t, rig.svc)
		for _, s := range stages {
			if s != "dbimport" {
				t.Fatalf("stages = %v, want dbimport on every event", stages)
			}
		}
	})

	t.Run("a save into a folder", func(t *testing.T) {
		svc, _ := downloadFixture(t, []byte("-- dump\nSELECT 1;\n"))
		if err := os.MkdirAll(filepath.Join(svc.cfg.HostMountRoot, "user", "restore"), 0o750); err != nil {
			t.Fatal(err)
		}
		stages := stagesOf(t, svc, "container:pg", func() {
			if _, started, err := svc.StartSaveDBDumpToPath(context.Background(), "pg", "local", "3f9c2a1be0d4aaaa", "user/restore", false); err != nil || !started {
				t.Fatalf("started=%v err=%v", started, err)
			}
		})
		waitForDetachedRun(t, svc)
		for _, s := range stages {
			if s != "dbdumpsave" {
				t.Fatalf("stages = %v, want dbdumpsave on every event", stages)
			}
		}
	})
}
