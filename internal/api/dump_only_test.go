package api_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// A stack member whose data sits under the compose working directory has no
// volume backup of its own, so its dumps are the only trace it leaves. The
// recovery wizard has to keep it out of "restore everything", which would
// otherwise bring it back with an empty database and no sign that anything is
// missing.
func TestLatestContainerBackupTimesMarksTheDumpOnlyContainer(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, _, own, _ := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(own)] = []restic.Snapshot{
		{ID: "1111aaaa", Time: "2026-09-01T00:00:00Z", Tags: []string{"dbdump:immich_pg"}},
		{ID: "2222bbbb", Time: "2026-09-01T00:00:00Z", Tags: []string{"container:sonarr"}},
		{ID: "3333cccc", Time: "2026-09-02T00:00:00Z", Tags: []string{"dbdump:sonarr"}},
	}

	got, err := svc.LatestContainerBackupTimes(context.Background())
	if err != nil {
		t.Fatalf("LatestContainerBackupTimes: %v", err)
	}
	if !got["immich_pg"].DumpOnly() {
		t.Errorf("immich_pg = %+v, want a container whose only backups are dumps", got["immich_pg"])
	}
	if got["immich_pg"].Dump == 0 {
		t.Error("the dump time is missing, so the row would read as never backed up")
	}
	if got["sonarr"].DumpOnly() {
		t.Errorf("sonarr = %+v, want the container with a files backup not to count as dump-only", got["sonarr"])
	}
	if got["sonarr"].Newest() != got["sonarr"].Dump {
		t.Errorf("sonarr newest = %d, want its newer dump at %d", got["sonarr"].Newest(), got["sonarr"].Dump)
	}
}

func TestContainerSnapshotTimesWithoutSnapshots(t *testing.T) {
	var none api.ContainerSnapshotTimes
	if none.DumpOnly() || none.Newest() != 0 {
		t.Errorf("zero value = %+v, want nothing backed up", none)
	}
}
