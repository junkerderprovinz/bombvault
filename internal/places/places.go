// Package places describes storage places: how BombVault reaches a place and
// how the address of each domain's repository there is spelled.
package places

import "strings"

// Kind is how BombVault reaches a place.
type Kind string

const (
	KindLocal  Kind = "local"
	KindS3     Kind = "s3"
	KindREST   Kind = "rest"
	KindSFTP   Kind = "sftp"
	KindWebDAV Kind = "webdav"
	KindAzure  Kind = "azure"
	KindRclone Kind = "rclone"
)

// Domains is the fixed order used everywhere a place lists its folders.
var Domains = []string{"containers", "vms", "flash", "config", "files", "zfs"}

// Folders maps a domain to its folder at a place. A missing key means the
// place does not offer that domain; an empty value means the base itself.
type Folders map[string]string

// DefaultFolders are the folders of a new place, named like the default
// domain paths.
func DefaultFolders() Folders {
	return Folders{"containers": "container", "vms": "vms", "flash": "flash", "config": "config", "files": "files", "zfs": "zfs"}
}

// Join builds the address of folder under base. After a trailing ':' (the
// root of an rclone remote or an sftp host) or a trailing '/' the folder
// follows without another slash. An empty folder returns base unchanged.
func Join(base, folder string) string {
	switch {
	case folder == "":
		return base
	case strings.HasSuffix(base, ":"), strings.HasSuffix(base, "/"):
		return base + folder
	}
	return base + "/" + folder
}

// Address is Join(base, folders[domain]) + suffix, and false when the place
// does not offer the domain. An empty domain stands for the base itself,
// where a place that is itself a repository keeps its one row.
func Address(base string, folders Folders, domain, suffix string) (string, bool) {
	folder, ok := folders[domain]
	if domain != "" && !ok {
		return "", false
	}
	return Join(base, folder) + suffix, true
}
