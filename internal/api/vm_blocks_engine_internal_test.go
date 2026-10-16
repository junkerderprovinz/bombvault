package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// runningBlockVirsh is a running VM whose host has libvirt's backup API.
type runningBlockVirsh struct{ checkpointVirsh }

func (v *runningBlockVirsh) State(context.Context, string) (string, error) { return "running", nil }

// forwardingSSH is a host connection that can reach the NBD socket.
type forwardingSSH struct {
	HostSSH
	recordingForwarder
}

func TestTheServiceEngineOffersTheChangedBlockPath(t *testing.T) {
	s := NewService(config.Config{DataDir: t.TempDir()}, nil, nil, &runningBlockVirsh{}, &restic.Restic{Bin: "restic"})
	s.SetHostSSH(forwardingSSH{recordingForwarder: recordingForwarder{calls: new([]string)}})
	s.diskFree = func(string) (uint64, error) { return 1 << 40, nil }
	domain := virshcli.DomainInfo{Disks: []virshcli.DiskRef{{Dev: "vda", Source: "/mnt/user/domains/win/vdisk1.qcow2", Format: "qcow2"}}}

	plan := s.planBlockBackup(t.Context(), "win", domain, []string{"/host/user/domains/win/vdisk1.qcow2"}, 0)
	if plan.reason != "" || plan.host == nil {
		t.Fatalf("plan = %+v, want the changed-block path", plan)
	}
}

func TestAChangedBlockBackupRunsThroughTheServiceEngine(t *testing.T) {
	eng := &tagRecordingBlockEngine{locked: 1}
	s := NewService(config.Config{}, nil, nil, nil, eng)
	r := &vmBlockRestic{engine: s.engine, svc: s}
	if _, err := r.Nodes(t.Context(), "repo", "snap", "/disks"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.BackupDir(t.Context(), "s3:https://s3.example/bucket/vms", "dir", "", []string{"vm:win"}); err != nil {
		t.Fatal(err)
	}
	if len(eng.tags) != 2 || eng.unlocks != 1 {
		t.Fatalf("%d backups and %d unlocks, want 2 and 1", len(eng.tags), eng.unlocks)
	}
}

// blockTargetEngine is targetEngine with the changed-block methods.
type blockTargetEngine struct{ *targetEngine }

func (blockTargetEngine) BackupDirFrom(context.Context, string, string, string, []string, restic.Mode) (restic.Summary, error) {
	return restic.Summary{}, nil
}

func (blockTargetEngine) LsTimes(context.Context, string, string, string, restic.Mode) ([]restic.NodeTime, error) {
	return nil, nil
}

func TestAnEngineWithChangedBlocksStillRefusesRestoresIntoTheDataFolder(t *testing.T) {
	host := t.TempDir()
	data := filepath.Join(host, "user", "appdata", "bombvault")
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	eng := &targetEngine{}
	s := NewService(config.Config{DataDir: data, HostMountRoot: host}, nil, nil, nil, blockTargetEngine{eng})
	if _, ok := s.engine.(vmBlockEngine); !ok {
		t.Fatal("the service engine lost the changed-block methods")
	}
	ctx := t.Context()
	restores := map[string]func() error{
		"RestorePath":      func() error { return s.engine.RestorePath(ctx, "r", "s", data, restic.Mode{}) },
		"RestoreAll":       func() error { return s.engine.RestoreAll(ctx, "r", "s", data, restic.Mode{}) },
		"RestoreVerify":    func() error { return s.engine.RestoreVerify(ctx, "r", "s", []string{"/a"}, data, restic.Mode{}) },
		"RestoreSubtreeTo": func() error { return s.engine.RestoreSubtreeTo(ctx, "r", "s", "/x", data, restic.Mode{}) },
		"RestoreInclude": func() error {
			return s.engine.RestoreInclude(ctx, "r", "s", filepath.ToSlash(filepath.Join(data, "bombvault.sqlite")), "/", restic.Mode{})
		},
		"RestoreSubtreeInclude": func() error {
			return s.engine.RestoreSubtreeInclude(ctx, "r", "s", "/x", "/bombvault.sqlite", data, restic.Mode{})
		},
	}
	for name, restore := range restores {
		if err := restore(); err == nil {
			t.Errorf("%s into the data folder was let through", name)
		}
	}
	if len(eng.targets) != 0 {
		t.Fatalf("restic was asked to write into %v", eng.targets)
	}
}
