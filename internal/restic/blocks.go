package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"
)

// BackupDirFromArgs is BackupDirArgs with an explicit parent and without the
// inode and ctime checks, so a file whose size and mtime match its node in
// parent is taken over from there without being read. The changed-block VM
// backup relies on it: its unchanged disk segments are empty placeholders.
func BackupDirFromArgs(repo, parent string, tags []string, m Mode) []string {
	args := backupArgs(repo, tags, m, "", nil, []string{"."})
	extra := []string{"--ignore-inode", "--ignore-ctime"}
	if parent != "" {
		extra = append(extra, "--parent", parent)
	} else {
		// Without --parent restic would pick one by host and path itself.
		extra = append(extra, "--force")
	}
	sep := slices.Index(args, "--")
	return slices.Insert(args, sep, extra...)
}

// BackupDirFrom backs up the contents of dir as the snapshot's tree root,
// building on parent (see BackupDirFromArgs). An empty parent reads every
// file.
func (r Restic) BackupDirFrom(ctx context.Context, repo, dir, parent string, tags []string, m Mode) (Summary, error) {
	if !filepath.IsAbs(dir) {
		return Summary{}, fmt.Errorf("restic backup directory %q is not absolute", dir)
	}
	if !IsRemoteRepo(repo) && !filepath.IsAbs(repo) {
		return Summary{}, fmt.Errorf("restic repository %q is not absolute", repo)
	}
	m = snapshotDirMode(m)
	start := time.Now()
	out, err := r.runIn(ctx, dir, BackupDirFromArgs(repo, parent, tags, m), m)
	if err != nil {
		// A placeholder that cannot be read would be one that restic chose to
		// read, so an exit 3 is a failure here, not a partial success.
		return Summary{}, err
	}
	return summarySince(out, start)
}

// NodeTime is a file node of a snapshot with the size and mtime restic
// recorded for it.
type NodeTime struct {
	Path  string    `json:"path"`
	Type  string    `json:"type"`
	Size  int64     `json:"size"`
	Mtime time.Time `json:"mtime"`
}

// LsTimes lists dirPath of a snapshot, its own node and direct children, with
// each node's mtime.
func (r Restic) LsTimes(ctx context.Context, repo, snapshotID, dirPath string, m Mode) ([]NodeTime, error) {
	out, err := r.run(ctx, LsPathArgs(repo, snapshotID, dirPath, m), m)
	if err != nil {
		return nil, err
	}
	return parseNodeTimes(out)
}

func parseNodeTimes(out []byte) ([]NodeTime, error) {
	var nodes []NodeTime
	for _, line := range bytes.Split(out, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var n struct {
			NodeTime
			StructType string `json:"struct_type"`
		}
		if err := json.Unmarshal(line, &n); err != nil {
			return nil, fmt.Errorf("restic ls: %w", err)
		}
		if n.StructType == "node" && n.Path != "" {
			nodes = append(nodes, n.NodeTime)
		}
	}
	if nodes == nil {
		return nil, errors.New("restic ls: no such path in the snapshot")
	}
	return nodes, nil
}
