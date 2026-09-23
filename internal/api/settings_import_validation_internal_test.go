package api

// An import may not store a value the settings save would refuse. The SPA always
// PUTs the full settings object, so one such field makes every later save fail.

import (
	"encoding/json"
	"strings"
	"testing"
)

// exportWith returns a valid export body with one field changed by mutate.
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

// stillSaveable reports whether the stored settings row passes the checks
// handlePutSettings runs.
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

// A file produced on a box with a different mount root carries an absolute path
// instead of the relative subpath the save requires.
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

	if ok, msg := stillSaveable(t, dst); !ok {
		t.Fatalf("a refused import must leave the instance saveable, got %q", msg)
	}
}

// "BackBlaze:bucket" lacks the rclone: prefix and would be stored as a folder of
// that name.
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

func TestImportRefusesAnInvalidDRDrillTarget(t *testing.T) {
	body := exportWith(t, func(v *settingsView) { v.DRDrillTarget = "web; rm -rf /" })

	dst, _ := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] == true {
		t.Fatal("a DR-drill target the save path refuses must not be importable")
	}
}

// The refusal happens in validateExport, so the preview shows it before the user
// confirms.
func TestImportPreviewRefusesInvalidPaths(t *testing.T) {
	body := exportWith(t, func(v *settingsView) { v.VMsPath = "/mnt/user/vms" })

	dst, _ := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, ""); env["ok"] == true {
		t.Fatal("the preview must refuse what the apply refuses, so the user is told before confirming")
	}
}

func TestImportAcceptsAValidFile(t *testing.T) {
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
