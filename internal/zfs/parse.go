package zfs

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	treeFields = 10
	listFields = 11
	snapFields = 3
)

// ParseTree reads the listing of one item's tree. The first line must be the
// root and every other line a dataset below it, so a listing that came back
// for the wrong root can never be mistaken for the item's own.
func ParseTree(out, root string) ([]ListEntry, error) {
	lines := splitLines(out)
	if len(lines) == 0 {
		return nil, fmt.Errorf("zfs list of %q returned nothing", root)
	}
	entries := make([]ListEntry, 0, len(lines))
	for _, line := range lines {
		f := strings.Split(line, "\t")
		if len(f) != treeFields {
			return nil, fmt.Errorf("zfs list of %q: %d fields, want %d", root, len(f), treeFields)
		}
		e := ListEntry{
			Name:       f[0],
			Type:       f[1],
			Mountpoint: f[2],
			Mounted:    f[3] == "yes",
			Canmount:   f[4],
			Encryption: f[5],
			Keystatus:  f[6],
			Snapdir:    f[7],
		}
		var err error
		if e.Referenced, err = parseNum(f[8]); err != nil {
			return nil, fmt.Errorf("zfs list of %q: referenced of %q: %w", root, e.Name, err)
		}
		if e.UsedByDataset, err = parseNum(f[9]); err != nil {
			return nil, fmt.Errorf("zfs list of %q: usedbydataset of %q: %w", root, e.Name, err)
		}
		entries = append(entries, e)
	}
	if entries[0].Name != root {
		return nil, fmt.Errorf("zfs list of %q begins with %q", root, entries[0].Name)
	}
	for _, e := range entries[1:] {
		if !DescendantOf(e.Name, root) {
			return nil, fmt.Errorf("zfs list of %q returned %q, which is not below it", root, e.Name)
		}
	}
	return entries, nil
}

// ParseList reads the listing of every pool, which carries used on top of the
// tree properties.
func ParseList(out string) ([]ListEntry, error) {
	lines := splitLines(out)
	entries := make([]ListEntry, 0, len(lines))
	for _, line := range lines {
		f := strings.Split(line, "\t")
		if len(f) != listFields {
			return nil, fmt.Errorf("zfs list: %d fields, want %d", len(f), listFields)
		}
		e := ListEntry{
			Name:       f[0],
			Type:       f[1],
			Mountpoint: f[2],
			Mounted:    f[3] == "yes",
			Canmount:   f[4],
			Encryption: f[5],
			Keystatus:  f[6],
			Snapdir:    f[7],
		}
		var err error
		if e.Used, err = parseNum(f[8]); err != nil {
			return nil, fmt.Errorf("zfs list: used of %q: %w", e.Name, err)
		}
		if e.Referenced, err = parseNum(f[9]); err != nil {
			return nil, fmt.Errorf("zfs list: referenced of %q: %w", e.Name, err)
		}
		if e.UsedByDataset, err = parseNum(f[10]); err != nil {
			return nil, fmt.Errorf("zfs list: usedbydataset of %q: %w", e.Name, err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// ParseSnapshots reads a snapshot listing and splits each name at its single
// '@'. Dataset names may hold spaces but never a tab or an '@'.
func ParseSnapshots(out string) ([]SnapshotEntry, error) {
	lines := splitLines(out)
	snaps := make([]SnapshotEntry, 0, len(lines))
	for _, line := range lines {
		f := strings.Split(line, "\t")
		if len(f) != snapFields {
			return nil, fmt.Errorf("zfs list -t snapshot: %d fields, want %d", len(f), snapFields)
		}
		dataset, name, ok := strings.Cut(f[0], "@")
		if !ok || dataset == "" || name == "" {
			return nil, fmt.Errorf("zfs list -t snapshot: %q is not a snapshot name", f[0])
		}
		s := SnapshotEntry{Dataset: dataset, Name: name}
		var err error
		if s.Creation, err = parseNum(f[1]); err != nil {
			return nil, fmt.Errorf("zfs list -t snapshot: creation of %q: %w", f[0], err)
		}
		if s.Used, err = parseNum(f[2]); err != nil {
			return nil, fmt.Errorf("zfs list -t snapshot: used of %q: %w", f[0], err)
		}
		snaps = append(snaps, s)
	}
	return snaps, nil
}

// LeakedStamps returns the BombVault snapshot names present on at least one
// filesystem of the tree, in the order the listing gives them. A stamp found
// only on volumes is left alone: the VM domain names its zvol snapshots the
// same way and has no sweeper of its own.
func LeakedStamps(tree []ListEntry, snaps []SnapshotEntry) []string {
	filesystems := make(map[string]bool, len(tree))
	for _, e := range tree {
		if e.Type == "filesystem" {
			filesystems[e.Name] = true
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, s := range snaps {
		if !IsBombVaultSnapshot(s.Name) || seen[s.Name] || !filesystems[s.Dataset] {
			continue
		}
		seen[s.Name] = true
		out = append(out, s.Name)
	}
	return out
}

func splitLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimRight(line, "\r"); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// parseNum reads a byte count from -p output. ZFS prints "-" where a property
// does not apply.
func parseNum(s string) (int64, error) {
	if s == "-" || s == "" || s == "none" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}
