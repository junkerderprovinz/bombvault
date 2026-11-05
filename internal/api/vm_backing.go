package api

import (
	"context"
	"encoding/json"
	"errors"
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

// imageHeadSize is how much of an image in a backup is read to learn its
// backing file. The name sits in the first cluster, right after the header.
const imageHeadSize = 64 << 10

// imageHead keeps the first imageHeadSize bytes written to it and ends the dump
// with errHeadRead.
type imageHead struct{ head []byte }

func (w *imageHead) Write(p []byte) (int, error) {
	n := min(imageHeadSize-len(w.head), len(p))
	w.head = append(w.head, p[:n]...)
	if len(w.head) == imageHeadSize {
		return n, errHeadRead
	}
	return n, nil
}

// snapshotImageHeader reads the qcow2 header of file in a snapshot. A file that
// is no qcow2 image gives qcow2.ErrNotQCOW2.
func (s *Service) snapshotImageHeader(ctx context.Context, ref repoRef, snapshotID, file string) (qcow2.Header, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	w := &imageHead{}
	// The dump ends in an error once the writer has what it wants, so only a
	// dump that gave too little counts as failed.
	if err := s.engine.DumpRaw(ctx, ref.repo, snapshotID, file, w, ref.mode); err != nil && len(w.head) < imageHeadSize {
		return qcow2.Header{}, err
	}
	return qcow2.HeaderOf(w.head)
}

// snapshotFiles is the files a snapshot holds a VM's disks in.
type snapshotFiles []string

// has reports whether the snapshot holds file, at its path or under its name
// in another folder, which is where a snapshot from before a rename keeps it.
func (h snapshotFiles) has(file string) bool {
	_, ok := h.at(file)
	return ok
}

// at returns where the snapshot holds file.
func (h snapshotFiles) at(file string) (string, bool) {
	if slices.Contains(h, file) {
		return file, true
	}
	for _, p := range h {
		if path.Base(p) == path.Base(file) {
			return p, true
		}
	}
	return "", false
}

// definitionAsBackedUp gives def, the newest definition, the disks of the
// chosen snapshot. Snapshots taken and merged since then change which file a
// disk boots from, and the snapshot's own files are what tell: a backup from
// before a snapshot holds the disk as the file below today's, and a backup from
// before a merge holds it as a file above. Restoring either under today's disk
// path gives a guest that mixes two points in time. held lists the files of the
// snapshot, or for a changed-block snapshot the flat images of its manifest.
func (s *Service) definitionAsBackedUp(ctx context.Context, ref repoRef, snapshotID string, def vmDefinition, held snapshotFiles, manifest *backup.BlocksManifest) (vmDefinition, error) {
	top := make(map[string]string, len(def.DiskPaths))
	for _, d := range def.DiskPaths {
		top[d] = heldBelow(d, def.Backing, held)
	}
	var links []vmBacking
	if manifest != nil {
		movedByDevice(def, manifest, top)
	} else {
		var err error
		if links, err = s.overlaysAbove(ctx, ref, snapshotID, def, held, top); err != nil {
			return vmDefinition{}, err
		}
	}

	out := def
	out.DomainXML = virshcli.StripBackingStore(def.DomainXML)
	out.DiskPaths = make([]string, len(def.DiskPaths))
	remap := map[string]string{}
	for i, d := range def.DiskPaths {
		out.DiskPaths[i] = top[d]
		if top[d] != d {
			remap[s.toHostPath(d)] = s.toHostPath(top[d])
		}
	}
	out.Backing = reachableLinks(links, out.DiskPaths)
	if len(remap) == 0 {
		return out, nil
	}
	// Unraid's list names the chain of the newest backup, not of this one.
	out.SnapshotDB = nil
	out.DomainXML = virshcli.RewriteDiskSources(out.DomainXML, remap)
	for _, d := range def.DiskPaths {
		if top[d] == d {
			continue
		}
		format, err := s.diskFormatInBackup(ctx, ref, snapshotID, top[d], held, manifest)
		if err != nil {
			return vmDefinition{}, err
		}
		out.DomainXML = virshcli.RewriteDiskFormat(out.DomainXML, s.toHostPath(top[d]), format)
		log.Printf("api: RestoreVM: this backup holds the disk %s as %s, so the VM is defined on that file", s.toHostPath(d), s.toHostPath(top[d])) //nolint:gosec // G706: paths from the VM's own definition
	}
	return out, nil
}

// heldBelow follows the chain of disk down to the first file the snapshot
// holds. A disk with no such file stays as it is: it is younger than the
// snapshot, and snapshotDiskSources decides what that means.
func heldBelow(disk string, links []vmBacking, held snapshotFiles) string {
	if held.has(disk) {
		return disk
	}
	cur := disk
	for range links {
		i := slices.IndexFunc(links, func(l vmBacking) bool { return l.Image == cur })
		if i < 0 {
			break
		}
		cur = links[i].File
		if held.has(cur) {
			return cur
		}
	}
	return disk
}

// movedByDevice finds the disk of a changed-block image whose file the newest
// definition does not know, as after a merge, through its target device.
func movedByDevice(def vmDefinition, manifest *backup.BlocksManifest, top map[string]string) {
	domain, err := virshcli.ParseDomain(def.DomainXML)
	if err != nil {
		return
	}
	for _, img := range manifest.Disks {
		if snapshotFiles(def.DiskPaths).has(img.Path) {
			continue
		}
		j := slices.IndexFunc(domain.Disks, func(d virshcli.DiskRef) bool { return d.Dev == img.Dev })
		if j < 0 {
			continue
		}
		for _, d := range def.DiskPaths {
			if path.Base(d) == path.Base(domain.Disks[j].Source) && top[d] == d {
				top[d] = img.Path
			}
		}
	}
}

// overlaysAbove moves each disk in top up to the image above it that only the
// snapshot knows, and returns the links of def with those images added.
func (s *Service) overlaysAbove(ctx context.Context, ref repoRef, snapshotID string, def vmDefinition, held snapshotFiles, top map[string]string) ([]vmBacking, error) {
	// A VM with zvol disks only has its domain XML in the snapshot.
	if len(def.DiskPaths) == 0 {
		return def.Backing, nil
	}
	known := make(snapshotFiles, 0, len(def.DiskPaths)+2*len(def.Backing))
	known = append(known, def.DiskPaths...)
	for _, l := range def.Backing {
		known = append(known, l.Image, l.File)
	}
	above := map[string][]string{}
	for _, p := range held {
		if known.has(p) {
			continue
		}
		h, err := s.snapshotImageHeader(ctx, ref, snapshotID, p)
		if errors.Is(err, qcow2.ErrNotQCOW2) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("restore vm: read the header of %s in this backup: %w", path.Base(p), err)
		}
		if h.BackingFile == "" {
			continue
		}
		below := s.containerPathOr(qcow2.BackingPath(s.toHostPath(p), h.BackingFile))
		at, ok := held.at(below)
		if !ok {
			if known.has(below) {
				return nil, destinationRefusal("this backup holds %s but not %s, the file it reads through to, so the disk cannot be restored as it was backed up; pick another backup", path.Base(p), path.Base(below))
			}
			continue
		}
		above[at] = append(above[at], p)
	}

	links := slices.Clone(def.Backing)
	for _, d := range def.DiskPaths {
		cur, ok := held.at(top[d])
		if !ok {
			continue
		}
		dir, moved := path.Dir(top[d]), cur != top[d]
		name := func(p string) string {
			if moved {
				return path.Join(dir, path.Base(p))
			}
			return p
		}
		for range held {
			over := above[cur]
			if len(over) == 0 {
				break
			}
			if len(over) > 1 {
				return nil, destinationRefusal("this backup holds several images on top of %s, so it cannot tell which one the VM ran on; pick another backup", path.Base(cur))
			}
			links = append(links, vmBacking{Image: name(over[0]), File: name(cur)})
			cur = over[0]
			top[d] = name(cur)
		}
	}
	return links, nil
}

// containerPathOr is host as this process opens it, or host itself when it
// lies outside the mount, as a file of another instance can.
func (s *Service) containerPathOr(host string) string {
	if p, ok := s.toContainerPath(host); ok {
		return p
	}
	return host
}

// diskFormatInBackup is the format a disk file has in the snapshot.
func (s *Service) diskFormatInBackup(ctx context.Context, ref repoRef, snapshotID, disk string, held snapshotFiles, manifest *backup.BlocksManifest) (string, error) {
	if manifest != nil {
		for _, img := range manifest.Disks {
			if path.Base(img.Path) == path.Base(disk) {
				return img.Format, nil
			}
		}
		return "", fmt.Errorf("restore vm: %s is not in this backup's disk list", path.Base(disk))
	}
	at, _ := held.at(disk)
	_, err := s.snapshotImageHeader(ctx, ref, snapshotID, at)
	switch {
	case errors.Is(err, qcow2.ErrNotQCOW2):
		return "raw", nil
	case err != nil:
		return "", fmt.Errorf("restore vm: read the header of %s in this backup: %w", path.Base(disk), err)
	}
	return "qcow2", nil
}

// reachableLinks keeps the links on the way down from disks.
func reachableLinks(links []vmBacking, disks []string) []vmBacking {
	var out []vmBacking
	cur := slices.Clone(disks)
	for len(cur) > 0 {
		var next []string
		for _, l := range links {
			if slices.Contains(cur, l.Image) && !slices.Contains(out, l) {
				out = append(out, l)
				next = append(next, l.File)
			}
		}
		cur = next
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

// storedVMDefinition is the definition the last backup of the VM stored, empty
// when there is none to read.
func (s *Service) storedVMDefinition(name string) vmDefinition {
	var def vmDefinition
	tg, err := s.store.GetVMTargetByName(name)
	if err != nil || tg.Definition == "" {
		return def
	}
	if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
		log.Printf("api: reading the stored definition of %q: %v", name, err) //nolint:gosec // G706: %q-quoted
	}
	return def
}
