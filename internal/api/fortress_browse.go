package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/junkerderprovinz/parleyport/kit/fortress"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

const actionSnapshotsBrowse = "snapshots.browse"

type browseSnapshot struct {
	ID    string   `json:"id"`
	Time  string   `json:"time"`
	Paths []string `json:"paths"`
}

type browseEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size,omitempty"`
}

// browseSnapshots lets a member look through a container's backups without
// restoring anything: the snapshots, and in one of them a folder at a time.
func (s *Service) browseSnapshots(ctx context.Context, _ fortress.Caller, body []byte) ([]byte, error) {
	var in struct {
		Domain   string `json:"domain"`
		Item     string `json:"item"`
		Snapshot string `json:"snapshot"`
		Path     string `json:"path"`
	}
	if err := readAction(body, &in); err != nil {
		return nil, err
	}
	switch {
	case in.Domain != "containers":
		return nil, refuse(http.StatusBadRequest, "unknown_domain")
	case in.Item == "":
		return nil, refuse(http.StatusNotFound, "unknown_item")
	case in.Snapshot != "" && !backup.ValidSnapshotID(in.Snapshot):
		return nil, refuse(http.StatusBadRequest, "bad_snapshot")
	}
	if _, err := s.store.GetTargetByContainer(in.Item); errors.Is(err, sql.ErrNoRows) {
		return nil, refuse(http.StatusNotFound, "unknown_item")
	} else if err != nil {
		return nil, err
	}
	snaps, err := s.Snapshots(ctx, in.Item, "")
	if err != nil {
		return nil, err
	}
	if in.Snapshot == "" {
		list := make([]browseSnapshot, 0, len(snaps))
		for _, sn := range snaps {
			list = append(list, browseSnapshot{ID: sn.ID, Time: sn.Time, Paths: sn.Paths})
		}
		return json.Marshal(map[string]any{"snapshots": list})
	}
	var paths []string
	for _, sn := range snaps {
		if sn.ID == in.Snapshot || strings.HasPrefix(sn.ID, in.Snapshot) {
			paths = sn.Paths
		}
	}
	files, err := s.ListSnapshotFiles(ctx, in.Item, in.Snapshot, "")
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"entries": folderOf(files, paths, in.Path)})
}

// folderOf picks the entries right inside dir out of a snapshot's listing,
// or the snapshot's own paths for an empty dir, so that an answer stays small
// whatever the backup holds.
func folderOf(files []restic.FileEntry, paths []string, dir string) []browseEntry {
	out := []browseEntry{}
	if dir == "" {
		for _, p := range paths {
			out = append(out, browseEntry{Path: p, Type: "dir"})
		}
		return out
	}
	dir = path.Clean(dir)
	for _, f := range files {
		if path.Dir(f.Path) == dir && f.Path != dir {
			out = append(out, browseEntry{Path: f.Path, Type: f.Type, Size: f.Size})
		}
	}
	return out
}
