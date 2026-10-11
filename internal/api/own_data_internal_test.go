package api

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// targetEngine records where each restore would have written.
type targetEngine struct {
	ResticEngine
	targets []string
}

func (e *targetEngine) RestorePath(_ context.Context, _, _, p string, _ restic.Mode) error {
	e.targets = append(e.targets, p)
	return nil
}

func (e *targetEngine) RestoreAll(_ context.Context, _, _, target string, _ restic.Mode, _ ...string) error {
	e.targets = append(e.targets, target)
	return nil
}

func (e *targetEngine) RestoreInclude(_ context.Context, _, _, include, target string, _ restic.Mode) error {
	e.targets = append(e.targets, target+"|"+include)
	return nil
}

func (e *targetEngine) RestoreSubtreeTo(_ context.Context, _, _, _, target string, _ restic.Mode) error {
	e.targets = append(e.targets, target)
	return nil
}

func (e *targetEngine) RestoreSubtreeInclude(_ context.Context, _, _, _, include, target string, _ restic.Mode) error {
	e.targets = append(e.targets, target+"|"+include)
	return nil
}

func TestNoRestoreWritesIntoTheDataFolderOrAFolderThatHoldsIt(t *testing.T) {
	root := t.TempDir()
	host := filepath.Join(root, "host")
	data := filepath.Join(host, "user", "appdata", "bombvault")
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "bombvault.sqlite"), []byte("live"), 0o600); err != nil {
		t.Fatal(err)
	}
	eng := &targetEngine{}
	s := NewService(config.Config{DataDir: data, HostMountRoot: host}, newTestStore(t), nil, nil, eng)
	ctx := context.Background()

	refused := map[string]func() error{
		"the data folder": func() error { return s.engine.RestoreSubtreeTo(ctx, "r", "s", "/x", data, restic.Mode{}) },
		"the share that holds it": func() error {
			return s.engine.RestoreAll(ctx, "r", "s", filepath.Join(host, "user", "appdata"), restic.Mode{})
		},
		"the Host Data root": func() error { return s.engine.RestorePath(ctx, "r", "s", host, restic.Mode{}) },
		"the file system root": func() error {
			return s.engine.RestoreSubtreeTo(ctx, "r", "s", "/x", filepath.VolumeName(data)+"/", restic.Mode{})
		},
		"a folder inside it": func() error {
			return s.engine.RestoreSubtreeTo(ctx, "r", "s", "/x", filepath.Join(data, "ssh"), restic.Mode{})
		},
		"the database in place": func() error {
			return s.engine.RestoreInclude(ctx, "r", "s", filepath.ToSlash(filepath.Join(data, "bombvault.sqlite")), "/", restic.Mode{})
		},
		"selected files into it": func() error {
			return s.engine.RestoreSubtreeInclude(ctx, "r", "s", "/x", "/bombvault.sqlite", data, restic.Mode{})
		},
	}
	link := filepath.Join(host, "user", "share", "x")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(data, link); err == nil {
		refused["a link to it"] = func() error {
			return s.engine.RestoreSubtreeTo(ctx, "r", "s", "/x", filepath.Join(link, "sub"), restic.Mode{})
		}
	}
	for name, restore := range refused {
		if err := restore(); err == nil {
			t.Errorf("a restore into %s was let through", name)
		}
	}
	if len(eng.targets) != 0 {
		t.Fatalf("restic was asked to write into %v", eng.targets)
	}

	elsewhere := filepath.Join(host, "user", "appdata", "plex")
	if err := s.engine.RestoreSubtreeTo(ctx, "r", "s", "/x", elsewhere, restic.Mode{}); err != nil {
		t.Fatalf("a restore beside the data folder: %v", err)
	}
	if err := s.engine.RestoreSubtreeInclude(ctx, "r", "s", "/x", "/plex", filepath.Join(host, "user", "appdata"), restic.Mode{}); err != nil {
		t.Fatalf("selected files into the share beside the data folder: %v", err)
	}
	if len(eng.targets) != 2 {
		t.Fatalf("restores that miss the data folder reached restic as %v", eng.targets)
	}
}

func TestTheDataFolderIsFoundOnEveryMountThatShowsIt(t *testing.T) {
	mountinfo := strings.Join([]string{
		"30 1 0:40 / / rw - overlay overlay rw",
		"31 30 0:50 /appdata/bombvault /config rw - fuse.shfs shfs rw",
		"32 30 0:60 / /host/user rw - tmpfs tmpfs rw",
		"33 32 0:50 / /host/user/user rw - fuse.shfs shfs rw",
		"34 32 0:70 / /host/user/cache rw - xfs /dev/nvme0n1p1 rw",
	}, "\n")
	got := dataAliases(strings.NewReader(mountinfo), "/config", "/host/user", func(string) []string { return []string{"user", "cache", "disk1"} })
	for _, want := range []string{"/host/user/user/appdata/bombvault", "/host/user/cache/appdata/bombvault", "/host/user/disk1/appdata/bombvault"} {
		if !slices.Contains(got, want) {
			t.Errorf("%s is missing from %v", want, got)
		}
	}
}
