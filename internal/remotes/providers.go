// Package remotes knows the storage products an off-site destination can be,
// how rclone describes the settings each one takes, and how to reach one
// before it is saved.
package remotes

import (
	"sort"
	"strings"
)

// Provider is a product somebody looks for, as opposed to the rclone backend
// that reaches it: Nextcloud, ownCloud and OpenCloud are three products and
// one webdav backend.
type Provider struct {
	// ID is stable and is what the interface sends back. Display names are
	// not translated, but some are spelled with characters an id should not
	// carry.
	ID      string `json:"id"`
	Name    string `json:"name"`
	Backend string `json:"backend"`
	Group   Group  `json:"group"`
	Route   Route  `json:"route"`

	// Preset is written into the remote without anybody being asked, such as
	// the vendor that makes WebDAV work properly against a self-hosted cloud.
	// For an S3 service it is rclone's own spelling of the provider; rclone
	// treats an unknown one as plain S3 without saying so.
	Preset map[string]string `json:"preset,omitempty"`

	// Mark names the logo the interface draws. Empty draws a generic glyph,
	// which beats a mark of the wrong service.
	Mark string `json:"mark,omitempty"`

	// URLHint is the shape of the address where it is not the one people have
	// in their browser.
	URLHint string `json:"urlHint,omitempty"`

	Auth AuthStyle `json:"auth,omitempty"`

	// Lock says the service can keep old versions even from someone holding
	// the credentials: object lock, a bucket or container immutability policy,
	// or an append-only server.
	Lock bool `json:"lock,omitempty"`

	// SelfHosted marks a cloud that runs on somebody's own server, which is
	// as fast as that server rather than throttled like a consumer service.
	SelfHosted bool `json:"selfHosted,omitempty"`
}

// Group is the section a provider is listed in.
type Group string

const (
	// GroupStorage is a hosted bucket store.
	GroupStorage Group = "storage"
	// GroupSelfS3 is an S3 server somebody runs on their own hardware.
	GroupSelfS3 Group = "selfs3"
	// GroupServer is a machine, a share or an address somebody types in.
	GroupServer Group = "server"
	// GroupCloud is a service for documents and photos, signed into by name.
	GroupCloud Group = "cloud"
)

// Route is how restic reaches a destination.
type Route string

const (
	// RouteS3 uses restic's own S3 backend, which is what lets a storage class
	// and object lock apply. The rclone remote of the same settings is used
	// only to look inside before saving.
	RouteS3 Route = "s3"
	// RouteRest is a rest-server, reached by restic directly.
	RouteRest Route = "rest"
	// RouteRclone goes through the rclone binary BombVault ships.
	RouteRclone Route = "rclone"
	// RoutePath is a folder under the host data mount.
	RoutePath Route = "path"
)

// AuthStyle is how a product wants to be signed into. The set is closed,
// because every value needs a sentence in every language.
type AuthStyle string

const (
	// AuthAppPassword is generated in the account's security settings and used
	// instead of the login password, which stops working once two-factor
	// sign-in is on.
	AuthAppPassword AuthStyle = "apppassword"
	// AuthToken is a browser sign-in, done with rclone authorize on a machine
	// that has a browser and pasted in as a token.
	AuthToken AuthStyle = "token"
	// AuthAPIKey is a key created in the service's own console.
	AuthAPIKey AuthStyle = "apikey"
	// AuthAccessKey is a key id and a secret, as S3 has them.
	AuthAccessKey AuthStyle = "accesskey"
	// AuthLogin is the ordinary user name and password.
	AuthLogin AuthStyle = "login"
	// AuthSSHKey is BombVault's own SSH key, put into the server's authorized
	// keys once.
	AuthSSHKey AuthStyle = "sshkey"
)

func s3(id, name, provider, mark string, lock bool) Provider {
	return Provider{ID: id, Name: name, Backend: "s3", Group: GroupStorage, Route: RouteS3,
		Preset: map[string]string{"provider": provider}, Mark: mark, Auth: AuthAccessKey, Lock: lock}
}

func ownS3(id, name, provider, mark string, lock bool) Provider {
	p := s3(id, name, provider, mark, lock)
	p.Group = GroupSelfS3
	return p
}

func cloud(id, name, backend, mark string, auth AuthStyle) Provider {
	return Provider{ID: id, Name: name, Backend: backend, Group: GroupCloud, Route: RouteRclone, Mark: mark, Auth: auth}
}

func webdavCloud(id, name, vendor, mark, hint string) Provider {
	return Provider{ID: id, Name: name, Backend: "webdav", Group: GroupCloud, Route: RouteRclone,
		Preset: map[string]string{"vendor": vendor}, Mark: mark, URLHint: hint, Auth: AuthAppPassword, SelfHosted: true}
}

// providers is kept by hand, since rclone's registry does not know what
// products exist or what they are called. It leaves out what cannot hold a
// restic repository: read-only and media-only backends, rclone's wrappers
// around other remotes, and backends that expire files on their own.
var providers = []Provider{
	s3("b2", "Backblaze B2", "Other", "IconBackblaze", true),
	s3("wasabi", "Wasabi", "Wasabi", "IconWasabi", true),
	s3("hetznerobj", "Hetzner Object Storage", "Hetzner", "IconHetzner", true),
	s3("r2", "Cloudflare R2", "Cloudflare", "IconCloudflare", false),
	s3("idrivee2", "IDrive e2", "IDrive", "IconIdrive", true),
	s3("scaleway", "Scaleway Object Storage", "Scaleway", "IconScaleway", true),
	s3("ovh", "OVHcloud Object Storage", "OVHcloud", "IconOvh", true),
	s3("ionosobj", "IONOS Object Storage", "IONOS", "IconIonos", true),
	s3("synologyc2", "Synology C2", "Synology", "IconSynology", true),
	s3("spaces", "DigitalOcean Spaces", "DigitalOcean", "IconDigitalOcean", false),
	s3("linode", "Linode Object Storage", "Linode", "IconLinode", false),
	s3("huaweiobs", "Huawei Cloud OBS", "HuaweiOBS", "IconHuaweiCloud", true),
	s3("aws", "Amazon S3", "AWS", "", true),
	s3("storj", "Storj", "Storj", "IconStorj", false),
	{ID: "azureblob", Name: "Azure Blob Storage", Backend: "azureblob", Group: GroupStorage, Route: RouteRclone,
		Mark: "IconAzure", Auth: AuthAccessKey, Lock: true},
	{ID: "gcs", Name: "Google Cloud Storage", Backend: "google cloud storage", Group: GroupStorage, Route: RouteRclone,
		Mark: "IconGoogleCloud", Lock: true},
	{ID: "oracle", Name: "Oracle Object Storage", Backend: "oracleobjectstorage", Group: GroupStorage, Route: RouteRclone,
		Mark: "IconOracleCloud"},
	{ID: "swift", Name: "OpenStack Swift", Backend: "swift", Group: GroupStorage, Route: RouteRclone, Mark: "IconOpenstack"},
	{ID: "netstorage", Name: "Akamai NetStorage", Backend: "netstorage", Group: GroupStorage, Route: RouteRclone,
		Mark: "IconAkamai", Auth: AuthAccessKey},
	{ID: "qingstor", Name: "QingStor", Backend: "qingstor", Group: GroupStorage, Route: RouteRclone, Auth: AuthAccessKey},
	{ID: "azurefiles", Name: "Azure Files", Backend: "azurefiles", Group: GroupStorage, Route: RouteRclone,
		Mark: "IconAzure", Auth: AuthAccessKey},
	{ID: "sia", Name: "Sia", Backend: "sia", Group: GroupStorage, Route: RouteRclone},

	ownS3("garage", "Garage", "Other", "IconGarage", false),
	ownS3("seaweedfs", "SeaweedFS", "SeaweedFS", "IconSeaweedfs", false),
	ownS3("rustfs", "RustFS", "Other", "rustfs", true),
	ownS3("ceph", "Ceph", "Ceph", "IconCeph", true),
	ownS3("juicefs", "JuiceFS", "Other", "juicefs", false),
	ownS3("versity", "Versity S3 Gateway", "Other", "versity", false),
	ownS3("s3", "S3 compatible", "Other", "IconBuckets", true),

	{ID: "rest", Name: "rest-server", Group: GroupServer, Route: RouteRest, Mark: "IconShield", Auth: AuthLogin, Lock: true},
	{ID: "storagebox", Name: "Hetzner Storage Box", Backend: "sftp", Group: GroupServer, Route: RouteRclone,
		Mark: "IconHetzner", Auth: AuthSSHKey, Preset: map[string]string{"port": "23"}},
	{ID: "sftp", Name: "SFTP", Backend: "sftp", Group: GroupServer, Route: RouteRclone, Mark: "IconServer", Auth: AuthSSHKey},
	{ID: "smb", Name: "SMB share", Backend: "smb", Group: GroupServer, Route: RouteRclone, Mark: "IconFolder", Auth: AuthLogin},
	{ID: "webdav", Name: "WebDAV", Backend: "webdav", Group: GroupServer, Route: RouteRclone, Mark: "IconLink",
		Auth: AuthAppPassword, URLHint: "https://server.example.com/dav/"},
	{ID: "ftp", Name: "FTP", Backend: "ftp", Group: GroupServer, Route: RouteRclone, Mark: "IconTransfer", Auth: AuthLogin},
	{ID: "hdfs", Name: "HDFS", Backend: "hdfs", Group: GroupServer, Route: RouteRclone, Mark: "IconHadoop"},
	{ID: "path", Name: "Mounted path", Group: GroupServer, Route: RoutePath, Mark: "IconDrive"},

	cloud("onedrive", "OneDrive", "onedrive", "IconOnedrive", AuthToken),
	cloud("gdrive", "Google Drive", "drive", "IconGoogleDrive", AuthToken),
	cloud("dropbox", "Dropbox", "dropbox", "IconDropbox", AuthToken),
	cloud("pcloud", "pCloud", "pcloud", "IconPcloud", AuthToken),
	cloud("jottacloud", "Jottacloud", "jottacloud", "IconJottacloud", AuthToken),
	cloud("box", "Box", "box", "IconBox", AuthToken),
	cloud("yandex", "Yandex Disk", "yandex", "IconYandex", AuthToken),
	cloud("mailru", "Mail.ru Cloud", "mailru", "IconMailru", AuthLogin),
	cloud("zoho", "Zoho WorkDrive", "zoho", "IconZoho", AuthToken),
	cloud("hidrive", "IONOS HiDrive", "hidrive", "IconIonos", AuthToken),
	cloud("sharefile", "ShareFile", "sharefile", "IconCitrix", AuthToken),
	cloud("putio", "put.io", "putio", "IconPutio", AuthToken),
	cloud("opendrive", "OpenDrive", "opendrive", "IconOpendrive", AuthLogin),
	cloud("huaweidrive", "Huawei Drive", "huaweidrive", "IconHuaweiCloud", AuthToken),
	cloud("mega", "MEGA", "mega", "IconMega", AuthLogin),
	cloud("protondrive", "Proton Drive", "protondrive", "IconProtonDrive", AuthLogin),
	cloud("filen", "Filen", "filen", "IconFilen", AuthLogin),
	cloud("internxt", "Internxt", "internxt", "IconInternxt", AuthLogin),
	cloud("pikpak", "PikPak", "pikpak", "IconPikpak", AuthLogin),
	cloud("sugarsync", "SugarSync", "sugarsync", "IconSugarsync", AuthAPIKey),
	cloud("quatrix", "Quatrix", "quatrix", "IconQuatrix", AuthAPIKey),
	cloud("filescom", "Files.com", "filescom", "IconFilesCom", AuthAPIKey),
	cloud("ulozto", "Ulož.to", "ulozto", "IconUlozto", AuthLogin),
	cloud("linkbox", "Linkbox", "linkbox", "IconLinkbox", AuthAPIKey),
	cloud("pixeldrain", "Pixeldrain", "pixeldrain", "IconPixeldrain", AuthAPIKey),
	cloud("koofr", "Koofr", "koofr", "IconKoofr", AuthAppPassword),
	cloud("icloud", "iCloud Drive", "iclouddrive", "IconICloud", AuthAppPassword),
	cloud("fichier", "1Fichier", "fichier", "", AuthAPIKey),
	cloud("filefabric", "Enterprise File Fabric", "filefabric", "", AuthToken),
	cloud("filelu", "FileLu", "filelu", "", AuthAPIKey),
	cloud("premiumizeme", "premiumize.me", "premiumizeme", "", AuthToken),
	cloud("drime", "Drime", "drime", "", AuthAPIKey),
	cloud("shade", "Shade", "shade", "", AuthAPIKey),
	webdavCloud("nextcloud", "Nextcloud", "nextcloud", "IconNextcloud", "https://cloud.example.com/remote.php/dav/files/USER/"),
	webdavCloud("owncloud", "ownCloud", "owncloud", "IconOwncloud", "https://cloud.example.com/remote.php/webdav/"),
	// OpenCloud forked ownCloud Infinite Scale, whose dialect is not that of
	// the older PHP ownCloud.
	webdavCloud("opencloud", "OpenCloud", "infinitescale", "IconOpencloud", "https://cloud.example.com/remote.php/webdav"),
	{ID: "seafile", Name: "Seafile", Backend: "seafile", Group: GroupCloud, Route: RouteRclone, Mark: "IconSeafile",
		Auth: AuthAppPassword, SelfHosted: true, URLHint: "https://seafile.example.com/"},
}

var groupRank = map[Group]int{GroupStorage: 0, GroupSelfS3: 1, GroupServer: 2, GroupCloud: 3}

// Providers lists every provider by group, alphabetically within a group.
func Providers() []Provider {
	out := make([]Provider, len(providers))
	copy(out, providers)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return groupRank[out[i].Group] < groupRank[out[j].Group]
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// FindProvider returns the provider with this id.
func FindProvider(id string) (Provider, bool) {
	for _, p := range providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}
