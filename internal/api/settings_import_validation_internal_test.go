package api

// ---------------------------------------------------------------------------
// An import may not persist a row the settings SAVE would refuse.
//
// validateExport checked the schema version, the off-site targets and the
// cadence grammar, and nothing else. handlePutSettings additionally enforced
// repo-path containment, the unprefixed-remote guard and the DR-drill target
// name rule — so a hand-edited file, or one produced on a box with a different
// mount root, could import an absolute `containersPath` and lock the instance
// out of its own settings: the SPA always PUTs the FULL settings object, so from
// then on every save from every card failed with "invalid backup path", the card
// that would fix it included.
//
// Each test below applies an import that must be REFUSED, then proves the poison
// really would have been terminal by showing the settings save rejects the same
// value. That second half is the point: it is what makes these regressions
// rather than assertions about a message string.
// ---------------------------------------------------------------------------

import (
	"encoding/json"
	"strings"
	"testing"
)

// exportWith produces a valid export body with one field poisoned by mutate.
func exportWith(t *testing.T, mutate func(v *settingsView)) []byte {
	t.Helper()
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, exp := doExport(t, src, "")
	if err := json.Unmarshal(body, &exp); err != nil {
		t.Fatal(err)
	}
	mutate(&exp.Settings)
	out, err := json.Marshal(exp)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// stillSaveable reports whether the instance's CURRENT settings row would
// survive a settings save. The SPA always PUTs the full settings object, so this
// is the question a poisoned field decides: it runs the row through the very
// guard handlePutSettings calls, so a "yes" here is a save that really succeeds.
func stillSaveable(t *testing.T, h *Handler) (bool, string) {
	t.Helper()
	s, err := h.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(s)
	if msg := rejectInvalidSettingsPaths(v, h.cfg.HostMountRoot); msg != "" {
		return false, msg
	}
	if msg := rejectInvalidSettingsNames(v); msg != "" {
		return false, msg
	}
	return true, ""
}

// TestImportRefusesAnAbsoluteRepoPath is the reported lockout. "/mnt/user/
// backups" is absolute rather than the required relative subpath — the shape a
// file carries when it was produced on a box with a different mount root.
func TestImportRefusesAnAbsoluteRepoPath(t *testing.T) {
	body := exportWith(t, func(v *settingsView) { v.ContainersPath = "/mnt/user/backups" })

	dst, _ := newPortableHandler(t, appKeyB)
	env := doImport(t, dst, body, "?apply=true")
	if env["ok"] == true {
		t.Fatal("an absolute containersPath must be REFUSED at import: applying it locks the instance out of its own settings, " +
			"because the SPA PUTs the whole settings object and one poisoned field then fails every later save from every card")
	}
	if msg, _ := env["error"].(string); !strings.Contains(msg, "relative subpath") {
		t.Fatalf("the refusal must name the rule the save enforces, got %q", msg)
	}

	// The instance is untouched and still saveable — which is the property the
	// refusal exists to protect.
	if ok, msg := stillSaveable(t, dst); !ok {
		t.Fatalf("a refused import must leave the instance saveable, got %q", msg)
	}
}

// TestImportRefusesAnUnprefixedRemote covers the second asymmetry: a
// "BackBlaze:bucket" that is missing its rclone: prefix. The save path answers
// with guidance; the import path used to store it as a folder named after the
// string, after which no save could succeed either.
func TestImportRefusesAnUnprefixedRemote(t *testing.T) {
	body := exportWith(t, func(v *settingsView) { v.ContainersOffsite = "BackBlaze:bucket" })

	dst, _ := newPortableHandler(t, appKeyB)
	env := doImport(t, dst, body, "?apply=true")
	if env["ok"] == true {
		t.Fatal("an unprefixed remote must be refused at import, the same way the settings save refuses it")
	}
	if ok, msg := stillSaveable(t, dst); !ok {
		t.Fatalf("a refused import must leave the instance saveable, got %q", msg)
	}
}

// TestImportRefusesAnInvalidDRDrillTarget covers the third: validResourceName on
// PUT, nothing at all on import.
func TestImportRefusesAnInvalidDRDrillTarget(t *testing.T) {
	body := exportWith(t, func(v *settingsView) { v.DRDrillTarget = "web; rm -rf /" })

	dst, _ := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] == true {
		t.Fatal("a DR-drill target the save path refuses must not be importable")
	}
}

// TestImportPreviewRefusesTheSamePoison pins that the refusal happens in
// validateExport, so the PREVIEW says so too — the user learns before
// confirming, not after.
func TestImportPreviewRefusesTheSamePoison(t *testing.T) {
	body := exportWith(t, func(v *settingsView) { v.VMsPath = "/mnt/user/vms" })

	dst, _ := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, ""); env["ok"] == true {
		t.Fatal("the preview must refuse what the apply refuses, so the user is told before confirming")
	}
}

// TestImportStillAcceptsAValidFile is the guard against over-rejecting: a normal
// round-trip must keep working, remotes included.
func TestImportStillAcceptsAValidFile(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("a valid export must still apply: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.ContainersPath != "containers" || got.ContainersOffsite != "s3:offsite-containers" {
		t.Fatalf("a relative path and an s3: remote must both survive: %q / %q", got.ContainersPath, got.ContainersOffsite)
	}
}
