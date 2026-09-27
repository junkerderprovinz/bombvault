package restic

import (
	"encoding/json"
	"testing"
)

func TestParseBackupSummaryReadsTotals(t *testing.T) {
	t.Run("a 0.17 summary line carries the totals", func(t *testing.T) {
		out := []byte(`{"message_type":"status","percent_done":0.5}
{"message_type":"summary","files_new":3,"files_changed":1,"files_unmodified":812,"data_added":4096,"total_files_processed":816,"total_bytes_processed":1048576,"total_duration":2.431,"snapshot_id":"deadbeef"}
`)
		sum, err := ParseBackupSummary(out)
		if err != nil {
			t.Fatalf("ParseBackupSummary: %v", err)
		}
		if sum.TotalFilesProcessed != 816 {
			t.Fatalf("TotalFilesProcessed = %d, want 816", sum.TotalFilesProcessed)
		}
		if sum.TotalBytesProcessed != 1048576 {
			t.Fatalf("TotalBytesProcessed = %d, want 1048576", sum.TotalBytesProcessed)
		}
		if sum.TotalDuration == nil {
			t.Fatalf("TotalDuration = nil, want the duration restic reported")
		}
		if *sum.TotalDuration != 2.431 {
			t.Fatalf("TotalDuration = %v, want 2.431", *sum.TotalDuration)
		}
	})

	t.Run("a line without the totals stays unmeasured", func(t *testing.T) {
		out := []byte(`{"message_type":"summary","files_new":1,"files_changed":0,"data_added":4096,"snapshot_id":"deadbeef"}` + "\n")
		sum, err := ParseBackupSummary(out)
		if err != nil {
			t.Fatalf("ParseBackupSummary: %v", err)
		}
		if sum.TotalDuration != nil {
			t.Fatalf("TotalDuration = %v, want nil so the run is not recorded as an emptied source", *sum.TotalDuration)
		}
	})

	t.Run("an empty source with a duration is a measured zero", func(t *testing.T) {
		out := []byte(`{"message_type":"summary","files_new":0,"files_changed":0,"data_added":0,"total_files_processed":0,"total_bytes_processed":0,"total_duration":0.412,"snapshot_id":"deadbeef"}` + "\n")
		sum, err := ParseBackupSummary(out)
		if err != nil {
			t.Fatalf("ParseBackupSummary: %v", err)
		}
		if sum.TotalDuration == nil {
			t.Fatalf("TotalDuration = nil, want a duration next to the zero totals")
		}
		if sum.TotalBytesProcessed != 0 || sum.TotalFilesProcessed != 0 {
			t.Fatalf("totals = %d bytes / %d files, want zeros", sum.TotalBytesProcessed, sum.TotalFilesProcessed)
		}
	})
}

// snapshotListJSON is one snapshot written by restic 0.17 and one written
// before it, as `restic snapshots --json` returns them.
const snapshotListJSON = `[
  {"time":"2026-09-01T02:00:00Z","parent":"aaaabbbbccccdddd","tree":"1111","paths":["/mnt/appdata/plex"],
   "hostname":"tower","tags":["container:plex","p1"],"id":"feedface00000001","short_id":"feedface",
   "summary":{"backup_start":"2026-09-01T02:00:00Z","backup_end":"2026-09-01T02:00:07Z","files_new":4,
              "files_changed":2,"data_added":40960,"total_files_processed":816,"total_bytes_processed":1048576}},
  {"time":"2026-08-01T02:00:00Z","tree":"2222","paths":["/mnt/appdata/plex"],
   "hostname":"tower","tags":["container:plex","p1"],"id":"feedface00000002","short_id":"feedface"}
]`

func TestSnapshotsMetaCarriesSummaryAndParent(t *testing.T) {
	var metas []SnapshotMeta
	if err := json.Unmarshal([]byte(snapshotListJSON), &metas); err != nil {
		t.Fatalf("unmarshal SnapshotMeta: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("got %d snapshots, want 2", len(metas))
	}

	newer := metas[0]
	if newer.Parent != "aaaabbbbccccdddd" {
		t.Fatalf("Parent = %q, want the parent snapshot id", newer.Parent)
	}
	if newer.Summary == nil {
		t.Fatalf("Summary = nil, want the counters restic 0.17 stores with a snapshot")
	}
	if newer.Summary.TotalBytesProcessed != 1048576 || newer.Summary.TotalFilesProcessed != 816 {
		t.Fatalf("totals = %d bytes / %d files, want 1048576 / 816",
			newer.Summary.TotalBytesProcessed, newer.Summary.TotalFilesProcessed)
	}
	if newer.Summary.FilesNew != 4 || newer.Summary.DataAdded != 40960 {
		t.Fatalf("files_new/data_added = %d/%d, want 4/40960", newer.Summary.FilesNew, newer.Summary.DataAdded)
	}
	if newer.Summary.BackupEnd.Sub(newer.Summary.BackupStart).Seconds() != 7 {
		t.Fatalf("backup window = %v, want 7s", newer.Summary.BackupEnd.Sub(newer.Summary.BackupStart))
	}

	older := metas[1]
	if older.Summary != nil {
		t.Fatalf("Summary = %+v for a pre-0.17 snapshot, want nil", older.Summary)
	}
	if older.Parent != "" {
		t.Fatalf("Parent = %q, want empty", older.Parent)
	}

	// The richer view stays on its own type: the snapshot list the SPA receives
	// grows no keys from it.
	var snaps []Snapshot
	if err := json.Unmarshal([]byte(snapshotListJSON), &snaps); err != nil {
		t.Fatalf("unmarshal Snapshot: %v", err)
	}
	got, err := json.Marshal(snaps)
	if err != nil {
		t.Fatalf("marshal Snapshot: %v", err)
	}
	want := `[{"id":"feedface00000001","time":"2026-09-01T02:00:00Z","paths":["/mnt/appdata/plex"],` +
		`"tags":["container:plex","p1"],"hostname":"tower","summary":{"total_bytes_processed":1048576}},` +
		`{"id":"feedface00000002","time":"2026-08-01T02:00:00Z","paths":["/mnt/appdata/plex"],` +
		`"tags":["container:plex","p1"],"hostname":"tower"}]`
	if string(got) != want {
		t.Fatalf("Snapshot JSON changed:\n got %s\nwant %s", got, want)
	}
}
