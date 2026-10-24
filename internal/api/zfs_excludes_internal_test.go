package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestPreviewZFSExcludesMarksALineThatWouldExcludeEverything(t *testing.T) {
	root := filepath.ToSlash(t.TempDir())
	if err := os.MkdirAll(filepath.Join(root, "tank", "media", "x.tmp"), 0o750); err != nil {
		t.Fatal(err)
	}
	st := newTestStore(t)
	d, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "tank/media", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceZFSMembers(d.ID, []store.ZFSMember{{Dataset: "tank/media", HostMountpoint: "/mnt/tank/media"}}); err != nil {
		t.Fatal(err)
	}
	s := &Service{store: st, cfg: config.Config{HostSourceRoot: "/mnt", HostMountRoot: root}}

	rows, err := s.PreviewZFSExcludes(context.Background(), d.ID, []string{"/", "*", "/**", "*.tmp"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[:3] {
		if !row.Everything || row.Matches != 0 {
			t.Fatalf("row %+v, want a line that excludes everything and counts nothing", row)
		}
	}
	if last := rows[3]; last.Everything || last.Matches != 1 {
		t.Fatalf("row %+v, want one match for an ordinary line", last)
	}
}
