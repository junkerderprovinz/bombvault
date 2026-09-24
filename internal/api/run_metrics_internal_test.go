package api

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestBackupSummaryFromNeedsTheDuration(t *testing.T) {
	secs := func(v float64) *float64 { return &v }

	t.Run("a line without the duration stays unmeasured", func(t *testing.T) {
		got := backupSummaryFrom(restic.Summary{
			SnapshotID:          "abc",
			BytesAdded:          1024,
			TotalBytesProcessed: 40 << 30,
			TotalFilesProcessed: 900,
		})
		if got.Measured {
			t.Fatal("a summary line without total_duration read as measured")
		}
		if got.SourceBytes != 0 || got.SourceFiles != 0 || got.ResticMS != 0 {
			t.Fatalf("an unmeasured line carried figures: %+v", got)
		}
		if got.SnapshotID != "abc" || got.Bytes != 1024 {
			t.Fatalf("the snapshot and the new data were dropped: %+v", got)
		}
	})

	t.Run("the duration is rounded to milliseconds", func(t *testing.T) {
		got := backupSummaryFrom(restic.Summary{SnapshotID: "abc", TotalDuration: secs(2.0004)})
		if !got.Measured {
			t.Fatal("a line with total_duration did not read as measured")
		}
		if got.ResticMS != 2000 {
			t.Fatalf("restic_ms = %d, want 2000", got.ResticMS)
		}
		if got := backupSummaryFrom(restic.Summary{TotalDuration: secs(2.0006)}); got.ResticMS != 2001 {
			t.Fatalf("restic_ms = %d, want 2001", got.ResticMS)
		}
	})

	t.Run("a negative duration is clamped", func(t *testing.T) {
		got := backupSummaryFrom(restic.Summary{TotalDuration: secs(-0.5)})
		if got.ResticMS != 0 {
			t.Fatalf("restic_ms = %d, want 0", got.ResticMS)
		}
	})

	t.Run("a measured source of nothing is a measurement", func(t *testing.T) {
		got := backupSummaryFrom(restic.Summary{SnapshotID: "abc", TotalDuration: secs(0.4)})
		if !got.Measured || got.SourceBytes != 0 {
			t.Fatalf("an emptied source did not record a measured zero: %+v", got)
		}
	})
}

func TestAdapterAsksForTheParentOnlyWhenEveryFileIsNew(t *testing.T) {
	secs := func(v float64) *float64 { return &v }
	measured := func(filesNew int, totalFiles uint64) restic.Summary {
		return restic.Summary{
			SnapshotID:          "deadbeef12345678",
			FilesNew:            filesNew,
			TotalFilesProcessed: totalFiles,
			TotalDuration:       secs(1),
		}
	}

	t.Run("a normal backup is never asked about", func(t *testing.T) {
		eng := &parentProbeEngine{summary: measured(3, 900)}
		sum := backupOnce(t, eng)
		if eng.parentCalls != 0 {
			t.Fatalf("the parent was read %d times for a backup that changed three files", eng.parentCalls)
		}
		if sum.HasParent != nil {
			t.Fatalf("HasParent = %v, want unknown", *sum.HasParent)
		}
	})

	t.Run("every file new and a parent found", func(t *testing.T) {
		eng := &parentProbeEngine{summary: measured(900, 900), parent: "c0ffee"}
		sum := backupOnce(t, eng)
		if eng.parentCalls != 1 {
			t.Fatalf("the parent was read %d times, want once", eng.parentCalls)
		}
		if sum.HasParent == nil || !*sum.HasParent {
			t.Fatal("restic rewrote every file under a parent and the run does not say so")
		}
	})

	t.Run("every file new and no parent", func(t *testing.T) {
		eng := &parentProbeEngine{summary: measured(900, 900)}
		sum := backupOnce(t, eng)
		if sum.HasParent == nil || *sum.HasParent {
			t.Fatal("a first backup into an empty repository was not recorded as parentless")
		}
	})

	t.Run("an unreadable parent stays unknown", func(t *testing.T) {
		eng := &parentProbeEngine{summary: measured(900, 900), parentErr: errors.New("repository is locked")}
		sum := backupOnce(t, eng)
		if sum.HasParent != nil {
			t.Fatalf("HasParent = %v after the probe failed, want unknown", *sum.HasParent)
		}
	})

	t.Run("an empty source is not asked about", func(t *testing.T) {
		eng := &parentProbeEngine{summary: measured(0, 0)}
		backupOnce(t, eng)
		if eng.parentCalls != 0 {
			t.Fatal("a backup that saw no files at all asked for a parent")
		}
	})
}

func TestRunsAdapterFinishWritesMetricsOnlyOnSuccess(t *testing.T) {
	sum := backup.Summary{
		SnapshotID:  "deadbeef12345678",
		Bytes:       2048,
		Measured:    true,
		SourceBytes: 40 << 30,
		SourceFiles: 900,
		FilesNew:    3,
		ResticMS:    2000,
		SelectionFP: "1234567890abcdef",
	}

	t.Run("a measured success records what restic read", func(t *testing.T) {
		st := newTestStore(t)
		runID, err := st.StartRun("tg1", "backup")
		if err != nil {
			t.Fatal(err)
		}
		a := runsAdapter{st: st, ctx: context.Background()}
		if err := a.Finish(runID, "success", sum, ""); err != nil {
			t.Fatal(err)
		}
		got := onlyRun(t, st, "tg1")
		if got.SourceBytes == nil || *got.SourceBytes != 40<<30 {
			t.Fatalf("source_bytes = %v", got.SourceBytes)
		}
		if got.SourceFiles == nil || *got.SourceFiles != 900 {
			t.Fatalf("source_files = %v", got.SourceFiles)
		}
		if got.FilesNew == nil || *got.FilesNew != 3 {
			t.Fatalf("files_new = %v", got.FilesNew)
		}
		if got.ResticMS == nil || *got.ResticMS != 2000 {
			t.Fatalf("restic_ms = %v", got.ResticMS)
		}
		if got.SelectionFP == nil || *got.SelectionFP != "1234567890abcdef" {
			t.Fatalf("selection_fp = %v", got.SelectionFP)
		}
	})

	t.Run("a failed run records nothing", func(t *testing.T) {
		st := newTestStore(t)
		runID, err := st.StartRun("tg2", "backup")
		if err != nil {
			t.Fatal(err)
		}
		a := runsAdapter{st: st, ctx: context.Background()}
		if err := a.Finish(runID, "failed", backup.Summary{}, "restic exploded"); err != nil {
			t.Fatal(err)
		}
		got := onlyRun(t, st, "tg2")
		if got.SourceBytes != nil || got.SelectionFP != nil {
			t.Fatalf("a failed run carries metrics: %+v", got)
		}
	})

	t.Run("a cancelled backup never reaches the history", func(t *testing.T) {
		st := newTestStore(t)
		s := &Service{store: st}
		s.registerBackupCancel("files:set9", func() {})
		s.CancelBackupRun("files:set9", "")
		runID, err := st.StartRun("tg3", "backup")
		if err != nil {
			t.Fatal(err)
		}
		a := runsAdapter{st: st, ctx: context.Background(), svc: s, cancelKey: "files:set9"}
		if err := a.Finish(runID, "failed", backup.Summary{}, "context canceled"); err != nil {
			t.Fatal(err)
		}
		runs, err := st.ListRuns(10)
		if err != nil {
			t.Fatal(err)
		}
		if runs[0].Status != "cancelled" {
			t.Fatalf("status = %q, want cancelled", runs[0].Status)
		}
		series, err := st.ItemSeries("tg3", "backup", 1<<62, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(series) != 0 {
			t.Fatalf("a backup the user cancelled entered the series as %+v", series[0])
		}
	})

	t.Run("the vm adapter writes them too", func(t *testing.T) {
		st := newTestStore(t)
		runID, err := st.StartRun("tg4", "backup")
		if err != nil {
			t.Fatal(err)
		}
		a := startedRunsAdapter{st: st, runID: runID}
		if err := a.Finish(runID, "success", sum, ""); err != nil {
			t.Fatal(err)
		}
		got := onlyRun(t, st, "tg4")
		if got.SourceBytes == nil || got.SelectionFP == nil {
			t.Fatalf("a measured VM run carries no metrics: %+v", got)
		}
	})
}

func onlyRun(t *testing.T, st *store.Repo, targetID string) store.SeriesRun {
	t.Helper()
	runs, err := st.ItemSeries(targetID, "backup", 1<<62, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("got %d runs for %s, want 1", len(runs), targetID)
	}
	return runs[0]
}

func backupOnce(t *testing.T, eng ResticEngine) backup.Summary {
	t.Helper()
	a := &resticAdapter{engine: eng, mode: restic.Mode{}}
	sum, err := a.Backup(context.Background(), "/repo", []string{"/data"}, []string{"container:x"})
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

// parentProbeEngine answers one backup and counts how often the run's parent
// was read back out of the repository.
type parentProbeEngine struct {
	ResticEngine
	summary     restic.Summary
	parent      string
	parentErr   error
	parentCalls int
}

func (e *parentProbeEngine) Backup(context.Context, string, []string, []string, restic.Mode, ...string) (restic.Summary, error) {
	return e.summary, nil
}

func (e *parentProbeEngine) SnapshotParent(context.Context, string, string, restic.Mode) (string, error) {
	e.parentCalls++
	return e.parent, e.parentErr
}
