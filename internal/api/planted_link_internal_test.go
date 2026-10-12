package api

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// plantLink puts a link at rel below root that leads to a folder outside it,
// as a container that writes into its own share could.
func plantLink(t *testing.T, root, rel string) string {
	t.Helper()
	outside := filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	return outside
}

func TestARestoreToAFolderDoesNotFollowALinkOutOfTheShare(t *testing.T) {
	r := newFolderRestoreRig(t)
	outside := plantLink(t, r.root, "appdata/app/x")
	ctx := context.Background()
	if _, _, err := r.svc.StartRestoreToPath(ctx, "plex", "local", "aaaa1111", "appdata/app/x"); err == nil {
		t.Error("a container restore went through the link")
	}
	if _, _, err := r.svc.StartRestoreFileSet(ctx, r.set, "bbbb2222", "local", "appdata/app/x/fotos", false, RestoreOptions{}); err == nil {
		t.Error("a file set restore went through the link")
	}
	if got := r.eng.written(); len(got) != 0 {
		t.Fatalf("restores wrote into %v", got)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("the folder outside the share holds %v (%v)", entries, err)
	}
}

func TestAnExportDoesNotWriteThroughALinkedTempFile(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "settings.db")
	if err := os.WriteFile(victim, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "plex.tar.gz")
	if err := os.Symlink(victim, dest+".tmp"); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	final, err := publishExport(dest, nil, func(w io.Writer) error {
		_, err := io.WriteString(w, "export")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(victim); string(got) != "keep" {
		t.Fatalf("the linked file holds %q", got)
	}
	if got, _ := os.ReadFile(final); string(got) != "export" {
		t.Fatalf("the export holds %q", got)
	}
}
