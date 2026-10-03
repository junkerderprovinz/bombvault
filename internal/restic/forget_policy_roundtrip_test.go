package restic_test

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// TestForgetsPredictsWhatResticForgetRemoves writes snapshots at chosen times
// and holds the prediction against restic's own dry run for several policies.
func TestForgetsPredictsWhatResticForgetRemoves(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	ctx := context.Background()
	dir := resolved(t, t.TempDir())
	repo, src := filepath.Join(dir, "repo"), filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, m := restic.Restic{Bin: "restic"}, restic.Mode{}
	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal("Init:", err)
	}
	backups := []struct {
		when string
		tags []string
	}{
		{"2025-11-03 10:00:00", nil},
		{"2026-06-10 10:00:00", nil},
		{"2026-07-15 10:00:00", nil},
		{"2026-08-12 10:00:00", []string{restic.DirectTag}},
		{"2026-09-14 10:00:00", nil},
		{"2026-09-21 08:00:00", nil},
		{"2026-09-21 20:00:00", nil},
		{"2026-09-22 08:00:00", nil},
		{"2026-09-22 20:00:00", nil},
		{"2026-09-23 23:59:00", nil},
	}
	for _, b := range backups {
		args := []string{"-r", repo, "backup", "--insecure-no-password", "--no-scan", "--time", b.when, "--tag", "fileset:a"}
		for _, tag := range b.tags {
			args = append(args, "--tag", tag)
		}
		if out, err := exec.CommandContext(ctx, "restic", append(args, src)...).CombinedOutput(); err != nil { //nolint:gosec // G204: fixed binary, test arguments
			t.Fatalf("backup at %s: %v\n%s", b.when, err, out)
		}
	}
	snaps, err := r.Snapshots(ctx, repo, m)
	if err != nil {
		t.Fatal("Snapshots:", err)
	}
	for _, p := range []restic.RetentionPolicy{
		{KeepLast: 2},
		{KeepLast: 20},
		{KeepDaily: 2},
		{KeepDaily: 5},
		{KeepWeekly: 2},
		{KeepMonthly: 3},
		{KeepLast: 1, KeepWeekly: 2, KeepMonthly: 2},
		{KeepLast: 1, Direct: true},
		{KeepYearly: 1},
		{KeepMonthly: 1, KeepYearly: 2},
	} {
		groups, err := r.ForgetPreview(ctx, repo, p, m, "fileset:a")
		if err != nil {
			t.Fatalf("%+v: ForgetPreview: %v", p, err)
		}
		var want []string
		for _, g := range groups {
			for _, sn := range g.Remove {
				want = append(want, sn.ID)
			}
		}
		slices.Sort(want)
		predicted, ok := p.Forgets(snaps)
		if !ok {
			t.Fatalf("%+v: Forgets could not tell", p)
		}
		if got := slices.Sorted(maps.Keys(predicted)); !slices.Equal(got, want) {
			t.Errorf("%+v: predicted %v, restic removes %v", p, got, want)
		}
	}
}
