package api

import (
	"context"
	"fmt"
	"log"
	"path"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/qcow2"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// vmBacking says that the qcow2 image Image reads through to File, both
// container paths. A disk with snapshots, such as the ones Unraid takes, is
// such a chain, and its newest image holds only what changed since the last
// snapshot.
type vmBacking struct {
	Image string `json:"image"`
	File  string `json:"file"`
}

// vmBackingChains follows the backing chain of every qcow2 disk of domain.
// A chain that cannot be followed to its end fails the backup, which would
// otherwise hold disks that only restore next to the original's files.
func (s *Service) vmBackingChains(domain virshcli.DomainInfo) ([]vmBacking, error) {
	var links []vmBacking
	for _, d := range domain.Disks {
		if d.Format != "qcow2" {
			continue
		}
		chain, err := qcow2.Chain(d.Source, s.toContainerPath)
		if err != nil {
			return nil, fmt.Errorf("disk %s: %w", d.Dev, err)
		}
		for _, l := range chain {
			if b := (vmBacking{Image: l.Image, File: l.File}); !slices.Contains(links, b) {
				links = append(links, b)
			}
		}
	}
	return links, nil
}

// backingFiles are the files links read through to, other than disks.
func backingFiles(links []vmBacking, disks []string) []string {
	var out []string
	for _, l := range links {
		if !slices.Contains(disks, l.File) && !slices.Contains(out, l.File) {
			out = append(out, l.File)
		}
	}
	return out
}

// heldBackingFiles are the backing files of def that snapshotPaths holds,
// matched by name as snapshotDiskSources matches a disk. A snapshot taken
// before the chain was backed up holds none and restores as it did then.
func heldBackingFiles(def vmDefinition, snapshotPaths []string) []string {
	var out []string
	for _, f := range backingFiles(def.Backing, def.DiskPaths) {
		if slices.ContainsFunc(snapshotPaths, func(p string) bool { return path.Base(p) == path.Base(f) }) {
			out = append(out, f)
		}
	}
	return out
}

// restoredBacking maps the links of def onto where a restore puts the files:
// restoredAt pairs each file as backed up with its place after the restore.
// A link whose image or backing file the restore leaves out is dropped.
func restoredBacking(def vmDefinition, restoredAt map[string]string) []vmBacking {
	var out []vmBacking
	for _, l := range def.Backing {
		img, okImg := restoredAt[l.Image]
		file, okFile := restoredAt[l.File]
		if okImg && okFile {
			out = append(out, vmBacking{Image: img, File: file})
		}
	}
	return out
}

// backingLinks is links as the restore repoints them.
func (s *Service) backingLinks(links []vmBacking) []backup.VMBackingLink {
	out := make([]backup.VMBackingLink, len(links))
	for i, l := range links {
		out[i] = backup.VMBackingLink{Image: l.Image, ImageHost: s.toHostPath(l.Image), Backing: s.toHostPath(l.File)}
	}
	return out
}

// unraidSnapshotDB is the file in which Unraid records the snapshots of the
// VM name.
func unraidSnapshotDB(name string) string {
	return "/etc/libvirt/qemu/snapshotdb/" + name + "/snapshots.db"
}

// writeSnapshotDB puts Unraid's record of a restored VM's snapshots back. It
// never fails the restore: without it the VM boots the same and Unraid only
// lists no snapshots for it.
func (s *Service) writeSnapshotDB(ctx context.Context, name string, db []byte) {
	file := unraidSnapshotDB(name)
	if _, err := s.ssh.Run(ctx, "mkdir", "-p", path.Dir(file)); err != nil {
		log.Printf("api: RestoreVM: WARN could not create the snapshot folder of %q: %v", name, err) //nolint:gosec // G706: %q-quoted
		return
	}
	if err := s.ssh.WriteFile(ctx, file, db); err != nil {
		log.Printf("api: RestoreVM: WARN could not write Unraid's snapshot list of %q, so the VM restores without it: %v", name, err) //nolint:gosec // G706: %q-quoted
	}
}
