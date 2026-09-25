package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

func TestAnAddressSplitsIntoItsBaseAndLastFolder(t *testing.T) {
	for loc, want := range map[string]placeSplit{
		"backups/containers":                                      {places.KindLocal, "backups", "containers"},
		"remotes/nas/bombvault/vms":                               {places.KindLocal, "remotes/nas/bombvault", "vms"},
		"s3:https://s3.us-west-004.backblazeb2.com/bv/containers": {places.KindS3, "s3:https://s3.us-west-004.backblazeb2.com/bv", "containers"},
		"s3:https://minio.lan:9000/bv/prefix/containers":          {places.KindS3, "s3:https://minio.lan:9000/bv/prefix", "containers"},
		"s3:s3.amazonaws.com/bv/containers":                       {places.KindS3, "s3:s3.amazonaws.com/bv", "containers"},
		"rest:http://tower:8000/bombvault-containers/containers":  {places.KindREST, "rest:http://tower:8000/bombvault-containers", "containers"},
		"rest:http://tower:8000/containers":                       {places.KindREST, "rest:http://tower:8000", "containers"},
		"sftp:pi:flash":                                           {places.KindSFTP, "sftp:pi:", "flash"},
		"sftp:bv@nas:/srv/restic/flash":                           {places.KindSFTP, "sftp:bv@nas:/srv/restic", "flash"},
		"sftp://bv@nas:2222/srv/flash":                            {places.KindSFTP, "sftp://bv@nas:2222/srv", "flash"},
		"rclone:r:config":                                         {places.KindRclone, "rclone:r:", "config"},
		"rclone:r:bucket/config":                                  {places.KindRclone, "rclone:r:bucket", "config"},
	} {
		got, ok := splitPlaceAddress(loc)
		if !ok || got != want {
			t.Errorf("splitPlaceAddress(%q) = %+v, %v, want %+v", loc, got, ok, want)
		}
	}
}

func TestTheRootOfABucketOrServerIsARepositoryOfItsOwn(t *testing.T) {
	for _, loc := range []string{
		"s3:https://s3.example.com/bucket", "s3:https://s3.example.com/bucket/",
		"rest:http://tower:8000", "rest:http://tower:8000/", "sftp:pi:", "rclone:r:",
	} {
		got, ok := splitPlaceAddress(loc)
		if !ok || got.base != loc || got.folder != "" {
			t.Errorf("splitPlaceAddress(%q) = %+v, %v, want the address itself as base", loc, got, ok)
		}
	}
}

func TestAnAddressNoPlaceCanSpellStaysUnsplit(t *testing.T) {
	for _, loc := range []string{
		"b2:bucket:files", "gs:bucket:/files", "swift:container:/files", "azure:container:/files",
		"BackBlaze:bucket/files", "", "backups", "/backups", "backups/containers/",
		"s3:https://s3.example.com/bucket/containers/", "s3:https://s3.example.com/", "sftp:pi:/flash", "sftp:pi",
	} {
		if got, ok := splitPlaceAddress(loc); ok {
			t.Errorf("splitPlaceAddress(%q) = %+v, want no split", loc, got)
		}
	}
}
