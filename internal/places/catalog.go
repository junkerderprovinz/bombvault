package places

import "slices"

// Group is the heading a provider's tile sits under in the add window.
type Group string

const (
	GroupCloud Group = "cloud"
	GroupSelf  Group = "self"
	GroupHere  Group = "here"
)

// Field is one input of a provider's form. Placeholder is an example value,
// never text to translate.
type Field struct {
	Key         string `json:"key"`
	Secret      bool   `json:"secret,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Optional    bool   `json:"optional,omitempty"`
}

// Provider is one tile of the add window: the connection kind it sets up and
// what its form asks for. EndpointTemplate fills {region}, {account} or {user}
// from the form; without one the endpoint is typed, or for B2 read from the
// key. OffPremises answers "Where is the device?" where the answer is fixed and
// is nil where the form asks.
type Provider struct {
	ID               string   `json:"id"`
	Group            Group    `json:"group"`
	Kind             Kind     `json:"kind"`
	Fields           []Field  `json:"fields"`
	EndpointTemplate string   `json:"endpointTemplate,omitempty"`
	DefaultPort      int      `json:"defaultPort,omitempty"`
	WebDAVVendor     string   `json:"webdavVendor,omitempty"`
	PickRoots        []string `json:"pickRoots,omitempty"`
	OffPremises      *bool    `json:"offPremises,omitempty"`
}

// Catalog is every provider, in the order of the tiles.
var Catalog = []Provider{
	cloudS3("b2", ""),
	cloudS3("s3", "https://s3.{region}.amazonaws.com", required("region", "eu-central-1")),
	cloudS3("r2", "https://{account}.r2.cloudflarestorage.com", required("account", "")),
	cloudS3("wasabi", "https://s3.{region}.wasabisys.com", required("region", "eu-central-1")),
	cloudS3("hetzner-os", "https://{region}.your-objectstorage.com", required("region", "fsn1")),
	cloudS3("storj", "https://gateway.storjshare.io"),
	cloudS3("idrive", "", required("endpoint", "s3.eu-central-2.idrivee2.com")),
	cloudS3("scaleway", "https://s3.{region}.scw.cloud", required("region", "fr-par")),
	cloudS3("ovh", "https://s3.{region}.io.cloud.ovh.net", required("region", "gra")),
	cloudS3("digitalocean", "https://{region}.digitaloceanspaces.com", required("region", "fra1")),
	cloudS3("ionos", "https://s3.{region}.ionoscloud.com", required("region", "eu-central-1")),
	cloudS3("contabo", "https://{region}.contabostorage.com", required("region", "eu2")),
	cloudS3("exoscale", "https://sos-{region}.exo.io", required("region", "de-fra-1")),
	cloudS3("vultr", "https://{region}.vultrobjects.com", required("region", "ams1")),
	cloudS3("gcs", "https://storage.googleapis.com"),
	// The container is optional like a bucket: the probe lists the account's
	// containers while none is named.
	{ID: "azure", Group: GroupCloud, Kind: KindAzure, OffPremises: fixedAnswer(true), Fields: []Field{
		required("account", ""), secret("secret"), optional("container", "backups"), optional("path", "bombvault"),
	}},
	{ID: "storagebox", Group: GroupCloud, Kind: KindSFTP, EndpointTemplate: "{user}.your-storagebox.de", DefaultPort: 23, OffPremises: fixedAnswer(true), Fields: []Field{
		required("user", "u123456"), optional("host", "u123456.your-storagebox.de"), optional("port", "23"), optional("path", "bombvault"),
	}},
	selfS3("minio", "http://192.168.1.10:9000", ""),
	selfS3("seaweedfs", "http://192.168.1.10:8333", ""),
	selfS3("garage", "http://192.168.1.10:3900", "garage"),
	selfS3("ceph", "http://192.168.1.10:7480", ""),
	selfS3("juicefs", "http://192.168.1.10:9000", ""),
	selfS3("rustfs", "http://192.168.1.10:9000", ""),
	selfS3("versitygw", "http://192.168.1.10:7070", ""),
	selfS3("s3-other", "https://s3.example.com", ""),
	webDAV("nextcloud", "nextcloud"),
	webDAV("owncloud", "owncloud"),
	webDAV("opencloud", "infinitescale"),
	restServer("rest-server", "https://backup.example.com:8000"),
	{ID: "sftp", Group: GroupSelf, Kind: KindSFTP, DefaultPort: 22, Fields: []Field{
		required("host", "backup.example.com"), optional("port", "22"), required("user", ""), optional("path", "bombvault"),
	}},
	restServer("bombvault", "https://tower.example.com:8000"),
	{ID: "rclone", Group: GroupSelf, Kind: KindRclone, Fields: []Field{required("remote", ""), optional("path", "bombvault")}},
	device("synology"),
	device("qnap"),
	device("truenas"),
	device("unraid-other"),
	device("share"),
	// Unraid pools carry names of the owner's choosing, so the second root is
	// all of /mnt, where the disks and every pool sit.
	{ID: "unraid-folder", Group: GroupHere, Kind: KindLocal, PickRoots: []string{"user", ""}, OffPremises: fixedAnswer(false), Fields: []Field{
		required("path", "user/bombvault"),
	}},
}

// ProviderByID looks a provider up in the catalog.
func ProviderByID(id string) (Provider, bool) {
	i := slices.IndexFunc(Catalog, func(p Provider) bool { return p.ID == id })
	if i < 0 {
		return Provider{}, false
	}
	return Catalog[i], true
}

func required(key, placeholder string) Field { return Field{Key: key, Placeholder: placeholder} }

func optional(key, placeholder string) Field {
	return Field{Key: key, Placeholder: placeholder, Optional: true}
}

func secret(key string) Field { return Field{Key: key, Secret: true} }

// fixedAnswer is an answer to "Where is the device?" that the form does not ask.
func fixedAnswer(offPremises bool) *bool { return &offPremises }

// cloudS3 is an S3 provider in the cloud; locate holds the fields its endpoint
// is found from. The bucket is optional because a key that may list its
// buckets offers them after the probe.
func cloudS3(id, template string, locate ...Field) Provider {
	fields := append([]Field{required("keyId", ""), secret("secret")}, locate...)
	fields = append(fields, optional("bucket", "backups"), optional("path", "bombvault"))
	return Provider{ID: id, Group: GroupCloud, Kind: KindS3, Fields: fields, EndpointTemplate: template, OffPremises: fixedAnswer(true)}
}

// selfS3 is an S3 service someone runs themselves, reached at a typed address.
func selfS3(id, endpoint, region string) Provider {
	return Provider{ID: id, Group: GroupSelf, Kind: KindS3, Fields: []Field{
		required("endpoint", endpoint), required("keyId", ""), secret("secret"), optional("region", region),
		optional("bucket", "backups"), optional("path", "bombvault"),
	}}
}

func webDAV(id, vendor string) Provider {
	return Provider{ID: id, Group: GroupSelf, Kind: KindWebDAV, WebDAVVendor: vendor, Fields: []Field{
		required("url", "https://cloud.example.com"), required("user", ""), secret("password"), optional("path", "bombvault"),
	}}
}

func restServer(id, address string) Provider {
	return Provider{ID: id, Group: GroupSelf, Kind: KindREST, Fields: []Field{
		required("url", address), required("user", ""), secret("password"), optional("path", ""),
	}}
}

// device is a NAS or another server whose share Unraid mounts under
// /mnt/remotes.
func device(id string) Provider {
	return Provider{ID: id, Group: GroupHere, Kind: KindLocal, PickRoots: []string{"remotes"}, Fields: []Field{
		required("path", "remotes/nas/bombvault"),
	}}
}
