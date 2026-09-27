package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestRedactExportLocationsCoversEveryLocationSlot pins that a plain settings
// export carries no URL-embedded credential in ANY repo-location field.
//
// The off-site slots were covered from the start. The five per-domain BACKUP
// PATHS were not, and they are not merely paths: a domain path may itself be a
// restic remote (see itemRepoPath's note in service.go, "A domain path could
// already be a restic remote"), and restic accepts
// rest:https://user:pass@host:8000/repo. A plain export is the file that gets
// mailed around and pasted into a forum thread, so a password reaching it is
// the exact failure scrubRepoLocation exists to prevent.
//
// Asserted over the MARSHALLED envelope rather than field by field: a new
// location slot added later is then covered by this test on the day it appears,
// instead of on the day someone remembers to extend a list.
func TestRedactExportLocationsCoversEveryLocationSlot(t *testing.T) {
	const secret = "hunter2"
	withCreds := func(host string) string {
		return "rest:https://admin:" + secret + "@" + host + ":8000/repo"
	}

	exp := settingsExport{
		Settings: settingsView{
			ContainersOffsite: withCreds("offsite-containers"),
			VMsOffsite:        withCreds("offsite-vms"),
			FlashOffsite:      withCreds("offsite-flash"),
			ConfigOffsite:     withCreds("offsite-config"),
			FilesOffsite:      withCreds("offsite-files"),

			ContainersPath: withCreds("primary-containers"),
			VMsPath:        withCreds("primary-vms"),
			FlashPath:      withCreds("primary-flash"),
			ConfigPath:     withCreds("primary-config"),
			FilesPath:      withCreds("primary-files"),
		},
		OffsiteTargets: []offsiteTargetView{{Repo: withCreds("target")}},
		NamedRepos:     []offsiteTargetView{{Repo: withCreds("named")}},
	}

	redactExportLocations(&exp)

	raw, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatalf("a plain export still carries a password.\n"+
			"Every repo-location slot must go through scrubRepoLocation, including the five\n"+
			"per-domain backup paths, which can themselves be remote repositories.\n\nexport: %s", raw)
	}

	// The location itself must survive: the point is to remove the credential,
	// not to leave the file naming no destination at all.
	if !strings.Contains(string(raw), "primary-containers") {
		t.Fatalf("scrubbing removed the destination as well as the credential: %s", raw)
	}
}

// TestAnImportedRedactedPathNeverOverwritesAWorkingOne is the other half of the
// same change, and without it the first half breaks a working install.
//
// Scrubbing the backup paths means a plain export can now carry
// "rest:https://[redacted]@host/repo" in a path slot. The off-site slots have
// always been protected against exactly that on the way back in: a redacted
// location must not replace a location that works. The paths were assigned
// straight through, so importing such a file would have written the marker into
// the live settings and pointed the domain at a repository that cannot be
// opened.
func TestAnImportedRedactedPathNeverOverwritesAWorkingOne(t *testing.T) {
	existing := store.Settings{
		ContainersPath: "rest:https://admin:hunter2@host:8000/repo",
		VMsPath:        "rest:https://admin:hunter2@host:8000/vms",
		FlashPath:      "rest:https://admin:hunter2@host:8000/flash",
		ConfigPath:     "rest:https://admin:hunter2@host:8000/config",
		FilesPath:      "rest:https://admin:hunter2@host:8000/files",
	}
	imported := settingsView{
		ContainersPath: "rest:https://[redacted]@host:8000/repo",
		VMsPath:        "rest:https://[redacted]@host:8000/vms",
		FlashPath:      "rest:https://[redacted]@host:8000/flash",
		ConfigPath:     "rest:https://[redacted]@host:8000/config",
		FilesPath:      "rest:https://[redacted]@host:8000/files",
	}

	out := mergeImportedSettings(existing, imported)

	for name, got := range map[string]string{
		"ContainersPath": out.ContainersPath,
		"VMsPath":        out.VMsPath,
		"FlashPath":      out.FlashPath,
		"ConfigPath":     out.ConfigPath,
		"FilesPath":      out.FilesPath,
	} {
		if strings.Contains(got, redactedLocationMarker) {
			t.Errorf("%s was overwritten with a redacted location (%q).\n"+
				"A location whose credential an export stripped must never replace a working one.", name, got)
		}
	}
}
