package places

import "testing"

func TestDetectProviderReadsTheAddress(t *testing.T) {
	for repo, want := range map[string]string{
		"s3:https://s3.us-west-004.backblazeb2.com/bucket/containers": "b2",
		"s3:https://abc.r2.cloudflarestorage.com/bv":                  "r2",
		"s3:https://s3.eu-central-1.wasabisys.com/bv":                 "wasabi",
		"s3:https://fsn1.your-objectstorage.com/bv":                   "hetzner-os",
		"s3:https://s3.eu-central-1.amazonaws.com/bv":                 "s3",
		"s3:https://s3.eu-central-1.amazonaws.com:443/bv":             "s3",
		"s3:HTTPS://fra1.digitaloceanspaces.com/bv":                   "digitalocean",
		"s3:s3.fr-par.scw.cloud/bv":                                   "scaleway",
		"s3:https://gateway.storjshare.io/bv":                         "storj",
		"s3:https://s3.eu-central-2.idrivee2.com/bv":                  "idrive",
		"s3:https://x1y2.fra.idrivee2-12.com/bv":                      "idrive",
		"s3:https://s3.fr-par.scw.cloud/bv":                           "scaleway",
		"s3:https://s3.gra.io.cloud.ovh.net/bv":                       "ovh",
		"s3:https://fra1.digitaloceanspaces.com/bv":                   "digitalocean",
		"s3:https://s3.eu-central-1.ionoscloud.com/bv":                "ionos",
		"s3:https://eu2.contabostorage.com/bv":                        "contabo",
		"s3:https://sos-de-fra-1.exo.io/bv":                           "exoscale",
		"s3:https://ams1.vultrobjects.com/bv":                         "vultr",
		"s3:https://storage.googleapis.com/bv":                        "gcs",
		"s3:http://192.168.1.10:9000/bv":                              "s3-other",
		"sftp:u123456@u123456.your-storagebox.de:/bv":                 "storagebox",
		"sftp://u1@box.example:2222/bv":                               "sftp",
		"rest:https://nas:8000/bv/containers":                         "rest-server",
		"rclone:r:":                                                   "rclone",
		"azure:c:/bv":                                                 "azure",
		"b2:bucket:containers":                                        "s3-other",
		"user/bombvault/containers":                                   "unraid-folder",
		"remotes/NAS/bombvault":                                       "share",
		"/mnt/remotes/NAS/bombvault":                                  "share",
		"/mnt/user/bombvault":                                         "unraid-folder",
	} {
		if got := DetectProvider(repo); got != want {
			t.Errorf("DetectProvider(%q) = %q, want %q", repo, got, want)
		}
	}
}

func TestUnderRemotesReadsTheCleanedAddress(t *testing.T) {
	for addr, want := range map[string]bool{
		"remotes":                 true,
		"remotes/nas/bombvault":   true,
		"/mnt/remotes/nas":        true,
		"user/../remotes/nas":     true,
		"./remotes//nas":          true,
		"remotes2/nas":            false,
		"user/remotes/nas":        false,
		"remotes/../user/backups": false,
		"mnt/remotes/nas":         false,
	} {
		if got := UnderRemotes(addr); got != want {
			t.Errorf("UnderRemotes(%q) = %v, want %v", addr, got, want)
		}
	}
}
