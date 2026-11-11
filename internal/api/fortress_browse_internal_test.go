package api

import (
	"context"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func TestBrowsingASnapshotShowsOneFolderAtATime(t *testing.T) {
	files := []restic.FileEntry{
		{Path: "/mnt/user/appdata/nextcloud", Type: "dir"},
		{Path: "/mnt/user/appdata/nextcloud/config", Type: "dir"},
		{Path: "/mnt/user/appdata/nextcloud/config/config.php", Type: "file", Size: 2048},
		{Path: "/mnt/user/appdata/nextcloud/nginx.conf", Type: "file", Size: 512},
	}
	paths := []string{"/mnt/user/appdata/nextcloud"}

	if got, want := folderOf(files, paths, ""), []browseEntry{{Path: "/mnt/user/appdata/nextcloud", Type: "dir"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("the top: %+v, want %+v", got, want)
	}
	want := []browseEntry{
		{Path: "/mnt/user/appdata/nextcloud/config", Type: "dir"},
		{Path: "/mnt/user/appdata/nextcloud/nginx.conf", Type: "file", Size: 512},
	}
	if got := folderOf(files, paths, "/mnt/user/appdata/nextcloud/"); !reflect.DeepEqual(got, want) {
		t.Fatalf("the container's folder: %+v, want %+v", got, want)
	}
	if got := folderOf(files, paths, "/mnt/user/appdata/nextcloud/config"); len(got) != 1 || got[0].Size != 2048 {
		t.Fatalf("a folder inside: %+v", got)
	}
}

func TestBrowsingRefusesWhatItCannotShow(t *testing.T) {
	svc, _ := newFortressSvc(t)
	for body, want := range map[string][2]any{
		`{"domain":"vms","item":"ubuntu"}`:                           {400, "unknown_domain"},
		`{"domain":"containers"}`:                                    {404, "unknown_item"},
		`{"domain":"containers","item":"nextcloud","snapshot":"zz"}`: {400, "bad_snapshot"},
		`{"domain":`: {400, "bad_request"},
	} {
		_, err := svc.browseSnapshots(context.Background(), fireSquire, []byte(body))
		refusedWith(t, err, want[0].(int), want[1].(string))
	}
}
