package places

import (
	"errors"
	"testing"
)

func mustProvider(t *testing.T, id string) Provider {
	t.Helper()
	p, ok := ProviderByID(id)
	if !ok {
		t.Fatalf("no provider %s", id)
	}
	return p
}

func TestBaseBuildsTheAddressOfEachKind(t *testing.T) {
	for _, c := range []struct {
		provider string
		fields   map[string]string
		want     string
	}{
		{"unraid-folder", map[string]string{"path": "/user/bombvault/"}, "user/bombvault"},
		{"s3", map[string]string{"region": "eu-central-1", "bucket": "bv"}, "s3:https://s3.eu-central-1.amazonaws.com/bv"},
		{"wasabi", map[string]string{"region": "eu-central-2", "bucket": "bv", "path": "/tower/"}, "s3:https://s3.eu-central-2.wasabisys.com/bv/tower"},
		{"r2", map[string]string{"account": "abc123", "bucket": "bv"}, "s3:https://abc123.r2.cloudflarestorage.com/bv"},
		{"hetzner-os", map[string]string{"region": "fsn1", "bucket": "bv"}, "s3:https://fsn1.your-objectstorage.com/bv"},
		{"storj", map[string]string{"bucket": "bv"}, "s3:https://gateway.storjshare.io/bv"},
		{"idrive", map[string]string{"endpoint": "s3.eu-central-2.idrivee2.com", "bucket": "bv"}, "s3:https://s3.eu-central-2.idrivee2.com/bv"},
		{"scaleway", map[string]string{"region": "fr-par", "bucket": "bv"}, "s3:https://s3.fr-par.scw.cloud/bv"},
		{"ovh", map[string]string{"region": "gra", "bucket": "bv"}, "s3:https://s3.gra.io.cloud.ovh.net/bv"},
		{"digitalocean", map[string]string{"region": "fra1", "bucket": "bv"}, "s3:https://fra1.digitaloceanspaces.com/bv"},
		{"ionos", map[string]string{"region": "eu-central-1", "bucket": "bv"}, "s3:https://s3.eu-central-1.ionoscloud.com/bv"},
		{"contabo", map[string]string{"region": "eu2", "bucket": "bv"}, "s3:https://eu2.contabostorage.com/bv"},
		{"exoscale", map[string]string{"region": "de-fra-1", "bucket": "bv"}, "s3:https://sos-de-fra-1.exo.io/bv"},
		{"vultr", map[string]string{"region": "ams1", "bucket": "bv"}, "s3:https://ams1.vultrobjects.com/bv"},
		{"gcs", map[string]string{"bucket": "bv"}, "s3:https://storage.googleapis.com/bv"},
		{"b2", map[string]string{"endpoint": "https://s3.us-west-004.backblazeb2.com", "bucket": "bv", "path": "bombvault"}, "s3:https://s3.us-west-004.backblazeb2.com/bv/bombvault"},
		{"minio", map[string]string{"endpoint": "http://192.168.1.10:9000/", "bucket": "bv"}, "s3:http://192.168.1.10:9000/bv"},
		{"rest-server", map[string]string{"url": "https://nas:8000/", "user": "tower"}, "rest:https://nas:8000/tower"},
		{"rest-server", map[string]string{"url": "https://nas:8000", "user": "tower", "path": "tower/bv"}, "rest:https://nas:8000/tower/bv"},
		{"storagebox", map[string]string{"user": "u123456"}, "sftp://u123456@u123456.your-storagebox.de:23"},
		{"storagebox", map[string]string{"user": "u123456-sub1", "host": "u123456.your-storagebox.de", "path": "bombvault"}, "sftp://u123456-sub1@u123456.your-storagebox.de:23/bombvault"},
		{"sftp", map[string]string{"host": "backup.lan", "user": "bv"}, "sftp://bv@backup.lan:22"},
		{"nextcloud", map[string]string{"url": "https://cloud.example.com", "user": "anna", "path": "bombvault"}, "rclone:bvp0a1b:bombvault"},
		{"azure", map[string]string{"account": "acct", "container": "backups"}, "azure:backups:"},
		{"azure", map[string]string{"account": "acct", "container": "backups", "path": "tower"}, "azure:backups:/tower"},
		{"rclone", map[string]string{"remote": "gdrive", "path": "bombvault"}, "rclone:gdrive:bombvault"},
	} {
		got, err := Base(mustProvider(t, c.provider), c.fields, "0a1b")
		if err != nil || got != c.want {
			t.Errorf("Base(%s, %v) = %q, %v; want %q", c.provider, c.fields, got, err, c.want)
		}
	}
}

func TestBaseNamesTheFieldItMisses(t *testing.T) {
	for _, c := range []struct {
		provider string
		fields   map[string]string
		want     MissingField
	}{
		{"s3", map[string]string{"bucket": "bv"}, "region"},
		{"r2", map[string]string{"bucket": "bv"}, "account"},
		{"minio", map[string]string{"bucket": "bv"}, "endpoint"},
		{"b2", map[string]string{"bucket": "bv"}, "endpoint"},
		{"s3", map[string]string{"region": "eu-central-1"}, "bucket"},
		{"unraid-folder", map[string]string{}, "path"},
		{"rest-server", map[string]string{"user": "bv"}, "url"},
		{"sftp", map[string]string{"user": "bv"}, "host"},
		{"azure", map[string]string{"account": "a"}, "container"},
		{"rclone", map[string]string{}, "remote"},
	} {
		_, err := Base(mustProvider(t, c.provider), c.fields, "0a1b")
		var missing MissingField
		if !errors.As(err, &missing) || missing != c.want {
			t.Errorf("Base(%s, %v) = %v, want the field %s named", c.provider, c.fields, err, c.want)
		}
	}
}

func TestS3RegionIsTheOneRequestsAreSignedFor(t *testing.T) {
	for _, c := range []struct {
		provider string
		fields   map[string]string
		want     string
	}{
		{"s3", map[string]string{"region": "eu-central-1"}, "eu-central-1"},
		{"r2", map[string]string{"account": "abc"}, "auto"},
		{"gcs", nil, "auto"},
		{"b2", map[string]string{"endpoint": "https://s3.us-west-004.backblazeb2.com"}, "us-west-004"},
		{"b2", map[string]string{"endpoint": "https://127.0.0.1:9000"}, ""},
		{"digitalocean", map[string]string{"region": "fra1"}, ""},
		{"ionos", map[string]string{"region": "eu-central-1"}, "de"},
		{"ionos", map[string]string{"region": "eu-central-3"}, "eu-central-3"},
		{"garage", map[string]string{"region": "garage"}, "garage"},
		{"minio", nil, ""},
	} {
		if got := S3Region(mustProvider(t, c.provider), c.fields); got != c.want {
			t.Errorf("S3Region(%s) = %q, want %q", c.provider, got, c.want)
		}
	}
}

func TestWebDAVURLPointsAtTheUsersFiles(t *testing.T) {
	for _, c := range []struct{ server, user, want string }{
		{"cloud.example.com/", "anna", "https://cloud.example.com/remote.php/dav/files/anna/"},
		{"https://cloud.example.com", "anna maria", "https://cloud.example.com/remote.php/dav/files/anna%20maria/"},
		{"https://cloud.example.com/remote.php/dav/spaces/1234$5678", "anna", "https://cloud.example.com/remote.php/dav/spaces/1234$5678/"},
		{"", "anna", ""},
	} {
		if got := WebDAVURL(c.server, c.user); got != c.want {
			t.Errorf("WebDAVURL(%q, %q) = %q, want %q", c.server, c.user, got, c.want)
		}
	}
}

func TestCredsFromFieldsTakesWhatEachKindNeeds(t *testing.T) {
	for _, c := range []struct {
		provider string
		fields   map[string]string
		want     Creds
	}{
		{"nextcloud", map[string]string{"url": "https://cloud.example.com", "user": "anna", "password": " pw "}, Creds{
			WebDAVURL: "https://cloud.example.com/remote.php/dav/files/anna/", WebDAVVendor: "nextcloud", WebDAVUser: "anna", WebDAVPass: " pw ",
		}},
		{"azure", map[string]string{"account": "acct", "secret": "k", "container": "c"}, Creds{AzureAccount: "acct", AzureKey: "k"}},
		{"r2", map[string]string{"keyId": " K ", "secret": "S", "account": "abc"}, Creds{S3KeyID: "K", S3Secret: "S", S3Region: "auto"}},
		{"rest-server", map[string]string{"url": "https://nas:8000", "user": "tower", "password": "p"}, Creds{RESTUser: "tower", RESTPassword: "p"}},
		{"sftp", map[string]string{"host": "box", "user": "bv"}, Creds{}},
	} {
		if got := CredsFromFields(mustProvider(t, c.provider), c.fields); got != c.want {
			t.Errorf("CredsFromFields(%s) = %+v\nwant %+v", c.provider, got, c.want)
		}
	}
}
