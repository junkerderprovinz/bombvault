package api

import (
	"strings"
	"testing"
)

// Building an rclone remote from a form instead of from a pasted config file.
//
// SMB and WebDAV are the two destinations people ask for most and the two that
// BombVault could technically already reach, as long as the operator wrote an
// rclone INI section by hand. That is a real barrier, and the restic
// documentation makes the alternative worse than it looks: it explicitly warns
// against putting a repository on a CIFS mount, which is exactly what the
// "mount the share on Unraid and point a path at it" route does. Going through
// rclone avoids the mount entirely.

func TestSMBSectionCarriesEveryFieldRcloneNeeds(t *testing.T) {
	got := rcloneSection(rcloneRemote{
		Name: "nas", Type: rcloneTypeSMB,
		Host: "192.168.1.10", User: "backup", ObscuredPass: "OBSCURED", Share: "backups",
	})

	for _, want := range []string{
		"[nas]",
		"type = smb",
		"host = 192.168.1.10",
		"user = backup",
		"pass = OBSCURED",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the section is missing %q:\n%s", want, got)
		}
	}
	// The share is NOT part of the remote: rclone addresses it as the first
	// path segment (nas:backups/repo). Writing it into the section would make
	// every path double up.
	if strings.Contains(got, "share") {
		t.Errorf("the share must not be a config key, it belongs in the path:\n%s", got)
	}
}

func TestWebDAVSectionCarriesItsURLAndVendor(t *testing.T) {
	got := rcloneSection(rcloneRemote{
		Name: "cloud", Type: rcloneTypeWebDAV,
		URL: "https://nextcloud.example/remote.php/dav/files/jdp", User: "jdp", ObscuredPass: "OBSCURED",
	})

	for _, want := range []string{
		"[cloud]",
		"type = webdav",
		"url = https://nextcloud.example/remote.php/dav/files/jdp",
		"user = jdp",
		"pass = OBSCURED",
		// Without a vendor rclone assumes the generic one, which silently loses
		// modification times on a Nextcloud server.
		"vendor = ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the section is missing %q:\n%s", want, got)
		}
	}
}

// A plaintext password must never reach the config file. rclone reads only its
// own obscured form, so an unobscured value would not merely be insecure, it
// would not work either, which is the kind of bug that gets "fixed" by pasting
// the password in twice.
func TestSectionRefusesAPlaintextPassword(t *testing.T) {
	r := rcloneRemote{Name: "nas", Type: rcloneTypeSMB, Host: "h", User: "u", ObscuredPass: ""}
	if err := r.validate(); err == nil {
		t.Fatal("a remote with no obscured password must not validate")
	}
}

// The name becomes a path prefix in every repository location, so it has to be
// something rclone can address at all. Rejecting it here beats a confusing
// failure later, inside restic.
func TestRemoteNameIsChecked(t *testing.T) {
	for _, name := range []string{"", "with space", "with:colon", "with/slash", "[bracket]"} {
		r := rcloneRemote{Name: name, Type: rcloneTypeSMB, Host: "h", User: "u", ObscuredPass: "x"}
		if err := r.validate(); err == nil {
			t.Errorf("name %q must be rejected", name)
		}
	}
	for _, name := range []string{"nas", "nas-backup", "nas_backup", "NAS2"} {
		r := rcloneRemote{Name: name, Type: rcloneTypeSMB, Host: "h", User: "u", ObscuredPass: "x"}
		if err := r.validate(); err != nil {
			t.Errorf("name %q must be accepted, got %v", name, err)
		}
	}
}

// Appending must not disturb what is already configured: an operator with ten
// working cloud remotes cannot have them rewritten because they added an SMB
// share.
func TestAppendKeepsExistingSectionsIntact(t *testing.T) {
	existing := "[b2]\ntype = b2\naccount = 123\nkey = abc\n"
	got, err := appendRcloneSection(existing, rcloneRemote{
		Name: "nas", Type: rcloneTypeSMB, Host: "h", User: "u", ObscuredPass: "x",
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if !strings.Contains(got, "[b2]") || !strings.Contains(got, "account = 123") {
		t.Fatalf("the existing remote was damaged:\n%s", got)
	}
	if !strings.Contains(got, "[nas]") {
		t.Fatalf("the new remote is missing:\n%s", got)
	}
}

// Replacing a remote of the same name edits that section only. Without this,
// correcting a typo in a password would silently create a duplicate section and
// rclone would use whichever it read first.
func TestAppendReplacesARemoteOfTheSameName(t *testing.T) {
	existing := "[nas]\ntype = smb\nhost = old\nuser = old\npass = OLD\n\n[b2]\ntype = b2\n"
	got, err := appendRcloneSection(existing, rcloneRemote{
		Name: "nas", Type: rcloneTypeSMB, Host: "new", User: "new", ObscuredPass: "NEW",
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if strings.Count(got, "[nas]") != 1 {
		t.Fatalf("want exactly one [nas] section:\n%s", got)
	}
	if strings.Contains(got, "OLD") || strings.Contains(got, "host = old") {
		t.Fatalf("the old values survived:\n%s", got)
	}
	if !strings.Contains(got, "[b2]") {
		t.Fatalf("an unrelated remote was lost:\n%s", got)
	}
}
