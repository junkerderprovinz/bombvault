package restic_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func TestCheckDataReportsPackCountsToTheSink(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	dir := resolved(t, t.TempDir())
	repo := filepath.Join(dir, "repo")
	data := filepath.Join(dir, "data")
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "a"), make([]byte, 1<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	r := restic.Restic{Bin: "restic"}
	m := restic.Mode{}
	ctx := context.Background()
	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Backup(ctx, repo, []string{data}, nil, m); err != nil {
		t.Fatal(err)
	}

	var got []progress.CountProgress
	cctx := progress.WithCountSink(ctx, func(c progress.CountProgress) { got = append(got, c) })
	if err := r.CheckData(cctx, repo, 100, m); err != nil {
		t.Fatal(err)
	}
	last := progress.CountProgress{}
	for _, c := range got {
		if c.Unit == "packs" {
			last = c
		}
	}
	if last.Total == 0 || last.Done != last.Total {
		t.Fatalf("pack counts = %+v, want a finished pack count", got)
	}
}
