package places

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

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

// MissingField is a form field a place cannot be built without.
type MissingField string

func (m MissingField) Error() string { return "the field " + string(m) + " is required" }

var placeholderRe = regexp.MustCompile(`\{(\w+)\}`)

// fill replaces each {key} of tmpl with that form field and names the first
// one left empty.
func fill(tmpl string, fields map[string]string) (string, error) {
	var missing MissingField
	out := placeholderRe.ReplaceAllStringFunc(tmpl, func(m string) string {
		key := m[1 : len(m)-1]
		v := strings.TrimSpace(fields[key])
		if v == "" && missing == "" {
			missing = MissingField(key)
		}
		return v
	})
	if missing != "" {
		return "", missing
	}
	return out, nil
}

// Endpoint is the S3 endpoint of a provider: its template filled from the
// form, or the endpoint field where it has none. An endpoint typed without a
// scheme gets https.
func Endpoint(p Provider, fields map[string]string) (string, error) {
	endpoint := strings.TrimSpace(fields["endpoint"])
	if p.EndpointTemplate != "" {
		var err error
		if endpoint, err = fill(p.EndpointTemplate, fields); err != nil {
			return "", err
		}
	}
	if endpoint == "" {
		return "", MissingField("endpoint")
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	return strings.TrimRight(endpoint, "/"), nil
}

// S3Region is the region a provider's S3 requests are signed for. R2 and GCS
// sign for "auto", B2 for the region in its endpoint, and IONOS's Frankfurt
// endpoint for user-owned buckets for "de". DigitalOcean, Contabo and Vultr put
// a data centre into the endpoint rather than a signing region, and Storj has
// none, so they get none and the S3 client looks it up.
func S3Region(p Provider, fields map[string]string) string {
	region := strings.TrimSpace(fields["region"])
	switch p.ID {
	case "r2", "gcs":
		return "auto"
	case "ionos":
		if region == "eu-central-1" {
			return "de"
		}
	case "b2":
		host := strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(fields["endpoint"], "https://"), "http://"), "/")
		if rest, ok := strings.CutPrefix(host, "s3."); ok {
			if b2Region, ok := strings.CutSuffix(rest, ".backblazeb2.com"); ok {
				return b2Region
			}
		}
		return ""
	case "digitalocean", "contabo", "vultr", "storj":
		return ""
	}
	return region
}

// WebDAVURL is where a user's own files sit on a Nextcloud, ownCloud or
// OpenCloud server: all three serve them under remote.php/dav/files/<user>/.
// An address that names a WebDAV path already, such as an OpenCloud space
// copied from its web interface, is kept.
func WebDAVURL(server, user string) string {
	server = strings.TrimRight(strings.TrimSpace(server), "/")
	if server == "" {
		return ""
	}
	if !strings.Contains(server, "://") {
		server = "https://" + server
	}
	if strings.Contains(server, "/remote.php/") || strings.Contains(server, "/dav/") {
		return server + "/"
	}
	return server + "/remote.php/dav/files/" + url.PathEscape(strings.TrimSpace(user)) + "/"
}

// Base builds a place's address from its provider and form, in restic's
// spelling and without a folder. placeID names the rclone remote a WebDAV place
// is reached through.
func Base(p Provider, fields map[string]string, placeID string) (string, error) {
	get := func(key string) string { return strings.TrimSpace(fields[key]) }
	need := func(key string) (string, error) {
		if v := get(key); v != "" {
			return v, nil
		}
		return "", MissingField(key)
	}
	sub := strings.Trim(get("path"), "/")
	under := func(base string) string {
		if sub == "" {
			return base
		}
		return base + "/" + sub
	}
	switch p.Kind {
	case KindLocal:
		if sub == "" {
			return "", MissingField("path")
		}
		return sub, nil
	case KindS3:
		endpoint, err := Endpoint(p, fields)
		if err != nil {
			return "", err
		}
		bucket, err := need("bucket")
		if err != nil {
			return "", err
		}
		return under("s3:" + endpoint + "/" + strings.Trim(bucket, "/")), nil
	case KindREST:
		server, err := need("url")
		if err != nil {
			return "", err
		}
		// rest-server with --private-repos serves each user only the tree under
		// its own name, so the path starts with the user unless one is typed.
		if sub == "" {
			sub = get("user")
		}
		return under("rest:" + strings.TrimRight(server, "/")), nil
	case KindSFTP:
		user, err := need("user")
		if err != nil {
			return "", err
		}
		host := get("host")
		if host == "" && p.EndpointTemplate != "" {
			if host, err = fill(p.EndpointTemplate, fields); err != nil {
				return "", err
			}
		}
		if host == "" {
			return "", MissingField("host")
		}
		port := get("port")
		if port == "" {
			port = strconv.Itoa(p.DefaultPort)
		}
		return under("sftp://" + user + "@" + host + ":" + port), nil
	case KindWebDAV:
		return "rclone:" + RemoteName(placeID) + ":" + sub, nil
	case KindAzure:
		container, err := need("container")
		if err != nil {
			return "", err
		}
		if sub == "" {
			return "azure:" + container + ":", nil
		}
		return "azure:" + container + ":/" + sub, nil
	case KindRclone:
		remote, err := need("remote")
		if err != nil {
			return "", err
		}
		return "rclone:" + strings.TrimSuffix(remote, ":") + ":" + sub, nil
	}
	return "", fmt.Errorf("provider %s has no connection kind", p.ID)
}

// CredsFromFields reads a place's credentials from its form. Secrets are taken
// as typed, since a space can belong to a password.
func CredsFromFields(p Provider, fields map[string]string) Creds {
	get := func(key string) string { return strings.TrimSpace(fields[key]) }
	switch p.Kind {
	case KindS3:
		return Creds{S3KeyID: get("keyId"), S3Secret: fields["secret"], S3Region: S3Region(p, fields)}
	case KindREST:
		return Creds{RESTUser: get("user"), RESTPassword: fields["password"]}
	case KindWebDAV:
		return Creds{
			WebDAVURL: WebDAVURL(get("url"), get("user")), WebDAVVendor: p.WebDAVVendor,
			WebDAVUser: get("user"), WebDAVPass: fields["password"],
		}
	case KindAzure:
		return Creds{AzureAccount: get("account"), AzureKey: fields["secret"]}
	}
	return Creds{}
}

// s3Hosts names the provider behind an S3 endpoint by its domain.
var s3Hosts = []struct{ domain, id string }{
	{"backblazeb2.com", "b2"},
	{"r2.cloudflarestorage.com", "r2"},
	{"wasabisys.com", "wasabi"},
	{"your-objectstorage.com", "hetzner-os"},
	{"amazonaws.com", "s3"},
	{"storjshare.io", "storj"},
	{"scw.cloud", "scaleway"},
	{"cloud.ovh.net", "ovh"},
	{"digitaloceanspaces.com", "digitalocean"},
	{"ionoscloud.com", "ionos"},
	{"contabostorage.com", "contabo"},
	{"exo.io", "exoscale"},
	{"vultrobjects.com", "vultr"},
	{"storage.googleapis.com", "gcs"},
}

// DetectProvider names the provider an existing address most likely belongs
// to, for the places the migration builds from existing rows. A wrong guess
// changes the mark and the form, never the address.
func DetectProvider(repo string) string {
	repo = strings.TrimSpace(repo)
	scheme, rest, remote := strings.Cut(repo, ":")
	if !remote || strings.Contains(scheme, "/") {
		if p := strings.TrimPrefix(repo, "/"); p == "remotes" || strings.HasPrefix(p, "remotes/") {
			return "share"
		}
		return "unraid-folder"
	}
	switch scheme {
	case "rest":
		return "rest-server"
	case "sftp":
		if strings.Contains(rest, ".your-storagebox.de") {
			return "storagebox"
		}
		return "sftp"
	case "rclone":
		return "rclone"
	case "azure":
		return "azure"
	case "s3":
		host, _, _ := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(rest, "https://"), "http://"), "/")
		host = strings.ToLower(host)
		// IDrive e2 endpoints carry the region in the domain itself.
		if strings.Contains(host, ".idrivee2") {
			return "idrive"
		}
		for _, h := range s3Hosts {
			if host == h.domain || strings.HasSuffix(host, "."+h.domain) {
				return h.id
			}
		}
	}
	return "s3-other"
}
