package places_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

func TestJoinSpellsAddressesTheWayTheirBackendReadsThem(t *testing.T) {
	for _, c := range []struct{ base, folder, want string }{
		{"user/bombvault", "container", "user/bombvault/container"},
		{"remotes/nas/bv", "", "remotes/nas/bv"},
		{"s3:https://s3.us-west-004.backblazeb2.com/bv", "vms", "s3:https://s3.us-west-004.backblazeb2.com/bv/vms"},
		{"s3:https://minio.lan:9000/bv/prefix", "flash", "s3:https://minio.lan:9000/bv/prefix/flash"},
		{"rest:http://tower:8000", "containers", "rest:http://tower:8000/containers"},
		{"sftp:pi:", "flash", "sftp:pi:flash"},
		{"sftp://bv@nas:2222/srv", "flash", "sftp://bv@nas:2222/srv/flash"},
		{"rclone:r:", "config", "rclone:r:config"},
		{"rclone:r:bucket", "config", "rclone:r:bucket/config"},
		{"rclone:r:", "", "rclone:r:"},
		{"/mnt/x/", "vms", "/mnt/x/vms"},
		{"azure:c:/", "vms", "azure:c:/vms"},
	} {
		if got := places.Join(c.base, c.folder); got != c.want {
			t.Errorf("Join(%q, %q) = %q, want %q", c.base, c.folder, got, c.want)
		}
	}
}

func TestAddressAddsTheSuffixForAnOfferedDomain(t *testing.T) {
	const base = "s3:https://s3.example.com/bucket"
	folders := places.Folders{"containers": "container", "files": ""}
	for _, c := range []struct {
		domain, suffix, want string
		ok                   bool
	}{
		{"containers", "", base + "/container", true},
		{"containers", "-copies", base + "/container-copies", true},
		{"containers", "-direct", base + "/container-direct", true},
		{"files", "", base, true},
		{"", "", base, true},
		{"vms", "", "", false},
	} {
		got, ok := places.Address(base, folders, c.domain, c.suffix)
		if got != c.want || ok != c.ok {
			t.Errorf("Address(%q, %q) = %q, %v, want %q, %v", c.domain, c.suffix, got, ok, c.want, c.ok)
		}
	}
}

func TestDefaultFoldersOfferEveryDomainUnderItsUsualName(t *testing.T) {
	want := map[string]string{"containers": "container", "vms": "vms", "flash": "flash", "config": "config", "files": "files", "zfs": "zfs"}
	got := places.DefaultFolders()
	if len(got) != len(places.Domains) {
		t.Fatalf("DefaultFolders = %v, want one folder per domain", got)
	}
	for _, d := range places.Domains {
		if got[d] != want[d] {
			t.Errorf("DefaultFolders()[%q] = %q, want %q", d, got[d], want[d])
		}
	}
	got["vms"] = "changed"
	if places.DefaultFolders()["vms"] != "vms" {
		t.Error("DefaultFolders hands every caller the same map")
	}
}
