package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func zfsPreviewService(t *testing.T, root string, members ...store.ZFSMember) (*Service, string) {
	t.Helper()
	st := newTestStore(t)
	d, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "tank/media", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceZFSMembers(d.ID, members); err != nil {
		t.Fatal(err)
	}
	return &Service{store: st, cfg: config.Config{HostSourceRoot: "/mnt", HostMountRoot: root}}, d.ID
}

func TestPreviewZFSExcludesMarksALineThatNamesThePathOfTheSnapshot(t *testing.T) {
	root := filepath.ToSlash(t.TempDir())
	if err := os.MkdirAll(filepath.Join(root, "tank", "media", "films"), 0o750); err != nil {
		t.Fatal(err)
	}
	s, id := zfsPreviewService(t, root, store.ZFSMember{Dataset: "tank/media", HostMountpoint: "/mnt/tank/media"})
	for line, want := range map[string]bool{"snapshot": true, ".zfs": true, "bombvault-*": true, "bombvault-old": false, "films": false} {
		rows, err := s.PreviewZFSExcludes(context.Background(), id, []string{line})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].Root != want {
			t.Errorf("%s: rows = %+v, want root %v", line, rows, want)
		}
	}
}

func TestPreviewZFSExcludesCountsTheFilesOfAChildDatasetOnce(t *testing.T) {
	root := filepath.ToSlash(t.TempDir())
	kid := filepath.Join(root, "tank", "media", "kid")
	if err := os.MkdirAll(kid, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{filepath.Join(root, "tank", "media", "junk.tmp"), filepath.Join(kid, "kid.tmp")} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s, id := zfsPreviewService(t, root,
		store.ZFSMember{Dataset: "tank/media", HostMountpoint: "/mnt/tank/media"},
		store.ZFSMember{Dataset: "tank/media/kid", HostMountpoint: "/mnt/tank/media/kid"},
	)
	rows, err := s.PreviewZFSExcludes(context.Background(), id, []string{"*.tmp"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Matches != 2 || len(rows[0].Sample) != 2 || rows[0].Sample[0] == rows[0].Sample[1] {
		t.Fatalf("rows = %+v, want /junk.tmp and /kid/kid.tmp once each", rows)
	}
}
