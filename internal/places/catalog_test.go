package places

import (
	"encoding/json"
	"regexp"
	"slices"
	"testing"
)

func TestTheCatalogListsEveryProviderInTileOrder(t *testing.T) {
	want := []string{
		"b2", "s3", "r2", "wasabi", "hetzner-os", "storj", "idrive", "scaleway", "ovh", "digitalocean",
		"ionos", "contabo", "exoscale", "vultr", "gcs", "azure", "storagebox",
		"minio", "seaweedfs", "garage", "ceph", "juicefs", "rustfs", "versitygw", "s3-other",
		"nextcloud", "owncloud", "opencloud", "rest-server", "sftp", "bombvault", "rclone",
		"synology", "qnap", "truenas", "unraid-other", "share", "unraid-folder",
	}
	got := make([]string, 0, len(Catalog))
	for _, p := range Catalog {
		got = append(got, p.ID)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("tiles = %v\nwant    %v", got, want)
	}
}

func TestEachTileSitsInItsGroupWithItsKind(t *testing.T) {
	for _, c := range []struct {
		id    string
		group Group
		kind  Kind
	}{
		{"b2", GroupCloud, KindS3}, {"gcs", GroupCloud, KindS3}, {"azure", GroupCloud, KindAzure},
		{"storagebox", GroupCloud, KindSFTP}, {"minio", GroupSelf, KindS3}, {"s3-other", GroupSelf, KindS3},
		{"nextcloud", GroupSelf, KindWebDAV}, {"opencloud", GroupSelf, KindWebDAV}, {"rest-server", GroupSelf, KindREST},
		{"sftp", GroupSelf, KindSFTP}, {"bombvault", GroupSelf, KindREST}, {"rclone", GroupSelf, KindRclone},
		{"synology", GroupHere, KindLocal}, {"share", GroupHere, KindLocal}, {"unraid-folder", GroupHere, KindLocal},
	} {
		p, ok := ProviderByID(c.id)
		if !ok || p.Group != c.group || p.Kind != c.kind {
			t.Errorf("%s = %+v, %v; want group %s, kind %s", c.id, p, ok, c.group, c.kind)
		}
	}
	if _, ok := ProviderByID("dropbox"); ok {
		t.Error("a provider outside the catalog was found")
	}
}

func TestOnlyDevicesAskWhereTheyStand(t *testing.T) {
	for _, p := range Catalog {
		switch {
		case p.Group == GroupCloud:
			if p.OffPremises == nil || !*p.OffPremises {
				t.Errorf("%s: a cloud provider always stands elsewhere", p.ID)
			}
		case p.ID == "unraid-folder":
			if p.OffPremises == nil || *p.OffPremises {
				t.Errorf("%s: a folder on this server always stands here", p.ID)
			}
		case p.OffPremises != nil:
			t.Errorf("%s: a device has to be asked where it stands", p.ID)
		}
	}
}

func TestPortsVendorsAndFolderRoots(t *testing.T) {
	get := func(id string) Provider {
		t.Helper()
		p, ok := ProviderByID(id)
		if !ok {
			t.Fatalf("no provider %s", id)
		}
		return p
	}
	if get("storagebox").DefaultPort != 23 || get("sftp").DefaultPort != 22 {
		t.Error("the Storage Box listens on 23 and a plain SFTP server on 22")
	}
	for id, vendor := range map[string]string{"nextcloud": "nextcloud", "owncloud": "owncloud", "opencloud": "infinitescale"} {
		if got := get(id).WebDAVVendor; got != vendor {
			t.Errorf("%s: vendor %q, want %q", id, got, vendor)
		}
	}
	for _, id := range []string{"synology", "qnap", "truenas", "unraid-other", "share"} {
		if got := get(id).PickRoots; !slices.Equal(got, []string{"remotes"}) {
			t.Errorf("%s: roots %v, want the mounted remotes", id, got)
		}
	}
	if got := get("unraid-folder").PickRoots; !slices.Equal(got, []string{"user", ""}) {
		t.Errorf("unraid-folder: roots %v, want the shares, then all of /mnt", got)
	}
}

func TestSecretsAreMarked(t *testing.T) {
	for _, p := range Catalog {
		if len(p.Fields) == 0 {
			t.Errorf("%s has no fields", p.ID)
		}
		for _, f := range p.Fields {
			if (f.Key == "secret" || f.Key == "password") != f.Secret {
				t.Errorf("%s: field %s secret = %v", p.ID, f.Key, f.Secret)
			}
		}
	}
}

func TestEveryTemplatePlaceholderIsARequiredField(t *testing.T) {
	re := regexp.MustCompile(`\{(\w+)\}`)
	for _, p := range Catalog {
		for _, m := range re.FindAllStringSubmatch(p.EndpointTemplate, -1) {
			if !slices.ContainsFunc(p.Fields, func(f Field) bool { return f.Key == m[1] && !f.Optional }) {
				t.Errorf("%s: its endpoint needs %s, which the form does not require", p.ID, m[1])
			}
		}
	}
}

func TestAProviderReachesTheWebInCamelCase(t *testing.T) {
	p, _ := ProviderByID("storagebox")
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "group", "kind", "fields", "endpointTemplate", "defaultPort", "offPremises"} {
		if _, ok := got[key]; !ok {
			t.Errorf("no %q in %s", key, b)
		}
	}
}
