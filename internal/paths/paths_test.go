package paths_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/paths"
)

func TestResolveHappyPath(t *testing.T) {
	got, err := paths.Resolve("/host/user", "backups/x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/host/user/backups/x" {
		t.Fatalf("expected /host/user/backups/x, got %s", got)
	}
}

func TestResolveRejectsTraversal(t *testing.T) {
	if _, err := paths.Resolve("/host/user", "../etc"); err == nil {
		t.Fatal("must reject .. traversal")
	}
	got, err := paths.Resolve("/host/user", "backups/x")
	if err != nil || got != "/host/user/backups/x" {
		t.Fatalf("expected /host/user/backups/x, got %s, err: %v", got, err)
	}
}

func TestResolveRejectsAbsoluteSub(t *testing.T) {
	if _, err := paths.Resolve("/host/user", "/etc/passwd"); err == nil {
		t.Fatal("must reject absolute sub path")
	}
}

func TestResolveRejectsHiddenTraversal(t *testing.T) {
	// sub that after cleaning resolves outside root
	if _, err := paths.Resolve("/host/user", "a/../../etc"); err == nil {
		t.Fatal("must reject traversal via a/../../etc")
	}
}

func TestResolveDeepPath(t *testing.T) {
	got, err := paths.Resolve("/host/user", "backups/bombvault/containers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/host/user/backups/bombvault/containers" {
		t.Fatalf("unexpected result: %s", got)
	}
}

func TestResolveRejectsEmptySub(t *testing.T) {
	// sub="" cleans to root itself — must be rejected (not a strict child).
	_, err := paths.Resolve("/host/user", "")
	if err == nil {
		t.Fatal("must reject empty sub (resolves to root, not a strict child)")
	}
}

func TestResolveRejectsDotSub(t *testing.T) {
	// sub="." cleans to root itself — must be rejected (not a strict child).
	_, err := paths.Resolve("/host/user", ".")
	if err == nil {
		t.Fatal("must reject sub='.' (resolves to root, not a strict child)")
	}
}
