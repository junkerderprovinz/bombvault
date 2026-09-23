package api

import "github.com/junkerderprovinz/bombvault/internal/store"

// runOpt fills in the columns a run only sometimes carries, so a series reads
// as the history it stands for and not as a wall of zero values.
type runOpt func(*store.SeriesRun)

func mkRun(id string, startedAt int64, status string, bytes int64, opts ...runOpt) store.SeriesRun {
	run := store.SeriesRun{ID: id, Status: status, StartedAt: startedAt, Bytes: bytes}
	for _, opt := range opts {
		opt(&run)
	}
	return run
}

func withSource(b, files int64) runOpt {
	return func(run *store.SeriesRun) { run.SourceBytes, run.SourceFiles = &b, &files }
}

func withFilesNew(n int64) runOpt {
	return func(run *store.SeriesRun) { run.FilesNew = &n }
}

func withParent(has bool) runOpt {
	return func(run *store.SeriesRun) {
		flag := int64(0)
		if has {
			flag = 1
		}
		run.HasParent = &flag
	}
}

func withSelection(fp string) runOpt {
	return func(run *store.SeriesRun) { run.SelectionFP = &fp }
}

// daily builds n runs a day apart, the oldest at start, and returns them newest
// first as the store's series readers do. f describes run i; its start comes
// from the spacing.
func daily(n int, start int64, f func(i int) store.SeriesRun) []store.SeriesRun {
	return spacedRuns(n, start, 86400, f)
}

func hourly(n int, start int64, f func(i int) store.SeriesRun) []store.SeriesRun {
	return spacedRuns(n, start, 3600, f)
}

func spacedRuns(n int, start, step int64, f func(i int) store.SeriesRun) []store.SeriesRun {
	out := make([]store.SeriesRun, n)
	for i := range n {
		run := f(i)
		run.StartedAt = start + int64(i)*step
		out[n-1-i] = run
	}
	return out
}

func withResticMS(ms int64) runOpt {
	return func(run *store.SeriesRun) { run.ResticMS = &ms }
}

func withSnapshot(id string) runOpt {
	return func(run *store.SeriesRun) { run.SnapshotID = id }
}

func withError(msg string) runOpt {
	return func(run *store.SeriesRun) { run.Error = msg }
}

// prefsPatch is a change that names both controls, the way a form does that
// has a value for each of them.
func prefsPatch(sensitivity, notifyMin string) AnomalyPrefsPatch {
	return AnomalyPrefsPatch{Sensitivity: &sensitivity, NotifyMin: &notifyMin}
}
