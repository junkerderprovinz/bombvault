package restic_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func TestParseRepoIDReadsTheConfigsID(t *testing.T) {
	const id = "4d7f7cfd3c6f2a3d7e1f8a0b9c2d4e6f8a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d"
	out := []byte("{\n  \"version\": 2,\n  \"id\": \"" + id + "\",\n  \"chunker_polynomial\": \"25b468838dcb75\"\n}\n")
	if got, err := restic.ParseRepoID(out); err != nil || got != id {
		t.Fatalf("ParseRepoID = %q, %v", got, err)
	}
	for _, bad := range []string{`{"version": 2}`, "Fatal: unable to open config file"} {
		if _, err := restic.ParseRepoID([]byte(bad)); err == nil {
			t.Errorf("ParseRepoID(%q) found an id", bad)
		}
	}
}

// TestRepoIDIsTheSameForOneRepository runs against the real restic binary and
// skips when restic is not on PATH.
func TestRepoIDIsTheSameForOneRepository(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	ctx := context.Background()
	dir := t.TempDir()
	r := restic.Restic{Bin: "restic"}
	m := restic.Mode{}
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	for _, repo := range []string{a, b} {
		if err := r.Init(ctx, repo, m); err != nil {
			t.Fatal(err)
		}
	}
	idA, err := r.RepoID(ctx, a, m)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := r.RepoID(ctx, a, m)
	idB, _ := r.RepoID(ctx, b, m)
	if len(idA) != 64 || idA != again || idA == idB {
		t.Fatalf("ids = %q, %q, %q", idA, again, idB)
	}
}
