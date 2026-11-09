// Package jobimport reads the backup jobs of Duplicati and Duplicacy and says
// what each one means in BombVault's terms: the folders, the exclusions, the
// schedule and the place the backups go. It reads settings only. The backup
// data stays with the other tool, and nothing here touches it.
package jobimport

import "strings"

// The tools a job can come from.
const (
	ToolDuplicati    = "duplicati"
	ToolDuplicacy    = "duplicacy"
	ToolDuplicacyWeb = "duplicacy-web"
)

// The kinds of destination BombVault can write to.
const (
	KindLocal  = "local"
	KindS3     = "s3"
	KindB2     = "b2"
	KindSFTP   = "sftp"
	KindRclone = "rclone"
)

// Subfolder is where BombVault's repository goes inside the other tool's
// destination, so its files never mix with the old backups.
const Subfolder = "bombvault"

// The codes a note can carry. The interface words each one in the reader's
// language, with Detail filled in.
const (
	NoteEncryptionKey   = "encryption-key"
	NoteRetention       = "retention"
	NoteFilter          = "filter"
	NoteInclude         = "include"
	NoteOptions         = "options"
	NoteSchedule        = "schedule"
	NoteNoSchedule      = "no-schedule"
	NoteDestination     = "destination"
	NoteSSHKey          = "ssh-key"
	NoteRcloneRemote    = "rclone-remote"
	NoteDisabled        = "disabled"
	NoteSignInEncrypted = "sign-in-encrypted"
)

// Note is something the import cannot carry over, or that needs a hand.
// Detail never holds a credential.
type Note struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// Job is one backup job of another tool.
type Job struct {
	Tool    string
	Name    string
	Enabled bool
	// Sources are the folders as the other tool saw them, which is often
	// inside its own container.
	Sources []string
	// Excludes are restic patterns. One that starts with a slash is anchored
	// and still names the path as the other tool saw it.
	Excludes []string
	// Schedule is a BombVault cadence, empty when the job follows the Folders
	// schedule.
	Schedule string
	Dest     Destination
	Notes    []Note
}

// Destination is where the job's backups go. Kind is empty when BombVault
// cannot write there.
type Destination struct {
	Kind string
	// Location is the restic location of a remote destination.
	Location string
	// Path is the folder of a local destination as the other tool saw it, or
	// the bucket and folder of a B2 one, whose S3 address the files lack.
	Path         string
	Region       string
	StorageClass string
	KeyID        string
	Secret       string
}

// Reader returns a file the job refers to. A relative name is taken from the
// folder the configuration was read from, an absolute one is a path as the
// other tool saw it.
type Reader func(name string) ([]byte, error)

func (j *Job) note(code, detail string) {
	j.Notes = append(j.Notes, Note{Code: code, Detail: detail})
}

// slashPath writes a path from either platform with forward slashes and
// without a trailing one.
func slashPath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}

// underSubfolder joins the non-empty parts and puts BombVault's subfolder below them.
func underSubfolder(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p = strings.Trim(p, "/"); p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(append(kept, Subfolder), "/")
}

// sftpDestination writes the URL form restic takes. A folder that starts with
// a slash is absolute there and one without is below the login's home, which
// is how Duplicati and Duplicacy read their own URLs too.
func sftpDestination(host, folder string) Destination {
	folder = strings.TrimRight(folder, "/")
	if folder != "" {
		folder += "/"
	}
	return Destination{Kind: KindSFTP, Location: "sftp://" + host + "/" + folder + Subfolder}
}
