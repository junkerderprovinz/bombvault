package restic_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func encryptedRepo(t *testing.T, password string) string {
	t.Helper()
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	repo := filepath.Join(resolved(t, t.TempDir()), "repo")
	if err := (restic.Restic{Bin: "restic"}).Init(context.Background(), repo, restic.Mode{Encrypted: true, Password: password}); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestAKeyAddedForAServerNamesItsHostAndOpensTheRepository(t *testing.T) {
	ctx := context.Background()
	r := restic.Restic{Bin: "restic"}
	owner := restic.Mode{Encrypted: true, Password: "owner-password"}
	repo := encryptedRepo(t, owner.Password)

	if err := r.AddKey(ctx, repo, owner, "member-password", ownHost, "attic"); err != nil {
		t.Fatal(err)
	}
	member := restic.Mode{Encrypted: true, Password: "member-password"}
	keys, err := r.Keys(ctx, repo, member)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(keys, func(k restic.Key) bool { return k.Current })
	if len(keys) != 2 || i < 0 || keys[i].HostName != ownHost || keys[i].UserName != "attic" || keys[i].ID == "" {
		t.Fatalf("keys seen with the new password = %+v, want two with the current one under %s and attic", keys, ownHost)
	}
}

func TestARemovedKeyNoLongerOpensTheRepository(t *testing.T) {
	ctx := context.Background()
	r := restic.Restic{Bin: "restic"}
	owner := restic.Mode{Encrypted: true, Password: "owner-password"}
	repo := encryptedRepo(t, owner.Password)
	if err := r.AddKey(ctx, repo, owner, "once", ownHost, "join"); err != nil {
		t.Fatal(err)
	}
	once := restic.Mode{Encrypted: true, Password: "once"}
	keys, err := r.Keys(ctx, repo, once)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(keys, func(k restic.Key) bool { return k.Current })
	if i < 0 {
		t.Fatalf("no current key in %+v", keys)
	}

	if err := r.RemoveKey(ctx, repo, owner, keys[i].ID); err != nil {
		t.Fatal(err)
	}
	if r.RepoOpens(ctx, repo, once) {
		t.Fatal("the removed key still opens the repository")
	}
	if !r.RepoOpens(ctx, repo, owner) {
		t.Fatal("the owner's key stopped opening the repository")
	}
}

func TestAKeyIsAddedWithoutThePasswordOnTheCommandLine(t *testing.T) {
	args := restic.KeyAddArgs("/repo", restic.Mode{Encrypted: true, Password: "owner"}, "/tmp/new", ownHost, "attic")
	for _, want := range [][]string{{"key", "add"}, {"--new-password-file", "/tmp/new"}, {"--host", ownHost}, {"--user", "attic"}} {
		if !slices.ContainsFunc(windows(args, len(want)), func(w []string) bool { return slices.Equal(w, want) }) {
			t.Errorf("%v not in %v", want, args)
		}
	}
	for _, a := range args {
		if a == "owner" {
			t.Fatalf("the password is on the command line: %v", args)
		}
	}
}

func windows(args []string, n int) [][]string {
	var out [][]string
	for i := 0; i+n <= len(args); i++ {
		out = append(out, args[i:i+n])
	}
	return out
}
