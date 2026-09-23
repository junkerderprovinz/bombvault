package backup

import (
	"strings"
	"testing"
)

// repoScrubCases are failure messages that must still name their repository
// after the scrub: keep must survive it, gone must not appear in the output.
// internal/api and internal/restic carry their own copies of the scrubber and
// of this table, and identical tables catch the copies drifting apart.
var repoScrubCases = []struct {
	name string
	in   string
	keep []string
	gone []string
}{
	{
		name: "the reported case keeps its whole address",
		in:   "Fatal: create repository at s3:http://192.168.1.50:8333/bucket failed",
		keep: []string{"s3:http://192.168.1.50:8333/bucket", "create repository"},
		gone: []string{"[path]"},
	},
	{
		name: "a password in the location still goes, the host stays",
		in:   "unable to open repository at rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers",
		keep: []string{"storage.example.com:8000/containers", "[redacted]@"},
		gone: []string{"Tr0ub4dor", "backupuser"},
	},
	{
		// The generic credential regex stops at "/". With the location exempt
		// from path scrubbing, only the userinfo scrubber keeps this password
		// from surviving whole.
		name: "a password containing a slash goes whole",
		in:   "rest:https://backupuser:wJalrXUtnFEMI/K7MDENG@host:8000/repo is unreachable",
		keep: []string{"host:8000/repo", "[redacted]@"},
		gone: []string{"wJalrXUtnFEMI", "K7MDENG", "backupuser"},
	},
	{
		name: "the sftp form keeps its scheme",
		in:   "sftp:backupuser@nas.local:/srv/restic refused the connection",
		keep: []string{"sftp:", "nas.local", "[redacted]@"},
		gone: []string{"backupuser"},
	},
	{
		// Only remote locations are exempt. A local repository is a path on
		// this host, no different from the appdata and mount paths the
		// scrubber keeps out of surfaced errors.
		name: "a local path is still scrubbed",
		in:   "unable to open repository at /mnt/user/appdata/bombvault/containers",
		keep: []string{"[path]"},
		gone: []string{"/mnt/user", "appdata"},
	},
	{
		name: "a remote location and a local path in one sentence",
		in:   "b2:mybucket/prefix unreachable, see /mnt/user/logs/bombvault.log",
		keep: []string{"b2:mybucket/prefix", "[path]"},
		gone: []string{"/mnt/user/logs"},
	},
	{
		// Userinfo outside a repo location is left to the generic regex.
		name: "credentials outside a location still go",
		in:   "the proxy at admin:hunter2@proxy.local refused",
		keep: []string{"[redacted]@proxy.local"},
		gone: []string{"hunter2", "admin:"},
	},
	{
		// Without the word boundary the "rest" inside "latest" would start a
		// location and exempt the path behind it.
		name: "a scheme name inside another word is not a location",
		in:   "pulling latest:/mnt/user/appdata/thing failed",
		keep: []string{"[path]"},
		gone: []string{"/mnt/user/appdata"},
	},
}

func TestScrubRunErrKeepsRemoteRepoLocations(t *testing.T) {
	for _, c := range repoScrubCases {
		t.Run(c.name, func(t *testing.T) {
			got := scrubRunErr(c.in)
			for _, want := range c.keep {
				if !strings.Contains(got, want) {
					t.Errorf("scrubRunErr(%q)\n  = %q\n  lost %q, which the operator needs to act on it", c.in, got, want)
				}
			}
			for _, bad := range c.gone {
				if strings.Contains(got, bad) {
					t.Errorf("scrubRunErr(%q)\n  = %q\n  still carries %q", c.in, got, bad)
				}
			}
		})
	}
}

// TestScrubRunErrNamesTheRepository covers the text that lands in the runs
// table and the weekly digest.
func TestScrubRunErrNamesTheRepository(t *testing.T) {
	got := scrubRunErr("Fatal: create repository at s3:http://192.168.1.50:8333/bucket failed")
	if !strings.Contains(got, "s3:http://192.168.1.50:8333/bucket") {
		t.Fatalf("scrubRunErr = %q, want the repository named in full.\n"+
			"An operator with several repositories cannot tell which one failed from\n"+
			"\"s3:http:[path]:8333[path]\".", got)
	}
}
