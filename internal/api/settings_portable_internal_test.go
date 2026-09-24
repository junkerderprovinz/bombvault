package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// newPortableHandler builds a Handler backed by an in-memory store and a Service
// keyed by appKey (64 hex chars). Two of them with different keys stand in for
// two BombVault instances.
func newPortableHandler(t *testing.T, appKey string) (*Handler, *store.Repo) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	// DataDir keeps SetRcloneConf from writing rclone.conf into the package
	// directory. The import checks repo paths against HostMountRoot, so it gets
	// the production default.
	cfg := config.Config{AppKey: appKey, DataDir: t.TempDir(), HostMountRoot: "/host/user"}
	svc := &Service{cfg: cfg, store: st}
	return &Handler{cfg: cfg, store: st, svc: svc}, st
}

const (
	appKeyA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	appKeyB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// seedSource populates an instance with a representative configuration: settings,
// two off-site targets, and all three credential kinds.
func seedSource(t *testing.T, h *Handler, st *store.Repo) {
	t.Helper()
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersEnabled = true
	s.ContainersPath = "containers"
	s.ContainersSchedule = "daily 02:00"
	s.ContainersOffsite = "s3:offsite-containers"
	s.RetentionKeepDaily = 7
	s.DefaultLanguage = "de"
	s.DrillsEnabled = true
	s.DrillsSchedule = "weekly Sun 04:00"
	s.RecoveryKitAck = true // per-instance state, never exported
	// The credentialed export requires a login password (requireAuthForSecrets).
	s.AuthPasswordHash = "seeded-login-password-hash"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		ID: "tgt-1", Domain: "containers", Name: "Primary", Repo: "s3:offsite-containers", Enabled: true, CreatedAt: 1000, SortOrder: 0,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		ID: "tgt-2", Domain: "containers", Name: "Archive", Repo: "s3:offsite-archive", Enabled: true, CreatedAt: 2000, SortOrder: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.SetCloudCreds(CloudCreds{S3KeyID: "AKIA", S3Secret: "topsecret", S3Region: "eu-west-1"}); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.SetRcloneConf("[b2]\ntype = b2\naccount = acct\nkey = rclonesecret\n"); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.SetNotifyConfig(notify.Config{On: "always", MatrixHomeserver: "https://m.example", MatrixToken: "matrixsecret", MatrixRoom: "!r:example"}); err != nil {
		t.Fatal(err)
	}
}

// doExport runs the export handler and returns the raw body and the decoded envelope.
func doExport(t *testing.T, h *Handler, query string) ([]byte, settingsExport) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.handleExportSettings(rec, httptest.NewRequest(http.MethodGet, "/api/settings/export"+query, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("export status = %d", rec.Code)
	}
	body := rec.Body.Bytes()
	var exp settingsExport
	if err := json.Unmarshal(body, &exp); err != nil {
		t.Fatalf("decode export: %v (body=%s)", err, body)
	}
	return body, exp
}

// Export, preview and apply reproduce the settings and off-site targets on an
// instance with a different key, and the credentials are readable there.
func TestSettingsExportImportRoundTrip(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)

	body, exp := doExport(t, src, "?includeCredentials=true")

	if exp.SchemaVersion != settingsExportSchema {
		t.Fatalf("schemaVersion = %d", exp.SchemaVersion)
	}
	if exp.Settings.ContainersSchedule != "daily 02:00" || exp.Settings.DefaultLanguage != "de" {
		t.Fatalf("settings not exported: %+v", exp.Settings)
	}
	if exp.Settings.RecoveryKitAck {
		t.Fatal("per-instance recovery-kit ack must not be exported")
	}
	if len(exp.OffsiteTargets) != 2 {
		t.Fatalf("want 2 off-site targets, got %d", len(exp.OffsiteTargets))
	}
	if exp.Credentials == nil {
		t.Fatal("credentials block missing on includeCredentials=true export")
	}
	if exp.Credentials.Cloud.S3Secret != "topsecret" || !strings.Contains(exp.Credentials.Rclone, "rclonesecret") || exp.Credentials.Notify.MatrixToken != "matrixsecret" {
		t.Fatalf("credentials not decrypted into the export: %+v", exp.Credentials)
	}

	// The preview reports counts and writes nothing.
	dst, dstStore := newPortableHandler(t, appKeyB)
	previewEnv := doImport(t, dst, body, "")
	if previewEnv["ok"] != true || previewEnv["preview"] != true {
		t.Fatalf("preview envelope wrong: %v", previewEnv)
	}
	summary := previewEnv["summary"].(map[string]any)
	if int(summary["offsiteTargets"].(float64)) != 2 {
		t.Fatalf("preview target count wrong: %v", summary)
	}
	if got, _ := dstStore.ListOffsiteTargets(); len(got) != 0 {
		t.Fatalf("preview must not write off-site targets, got %d", len(got))
	}

	applyEnv := doImport(t, dst, body, "?apply=true")
	if applyEnv["ok"] != true || applyEnv["applied"] != true {
		t.Fatalf("apply envelope wrong: %v", applyEnv)
	}

	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.ContainersSchedule != "daily 02:00" || got.DefaultLanguage != "de" || got.RetentionKeepDaily != 7 || !got.DrillsEnabled {
		t.Fatalf("settings not reproduced: %+v", got)
	}

	// Target IDs and timestamps survive.
	targets, err := dstStore.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("want 2 targets after apply, got %d", len(targets))
	}
	byID := map[string]store.OffsiteTarget{}
	for _, tg := range targets {
		byID[tg.ID] = tg
	}
	if byID["tgt-1"].Repo != "s3:offsite-containers" || byID["tgt-1"].CreatedAt != 1000 {
		t.Fatalf("target tgt-1 not reproduced: %+v", byID["tgt-1"])
	}
	if byID["tgt-2"].Repo != "s3:offsite-archive" {
		t.Fatalf("target tgt-2 not reproduced: %+v", byID["tgt-2"])
	}

	// The credentials were re-encrypted with appKeyB.
	cloud, err := dst.svc.CloudConfig()
	if err != nil || cloud.S3Secret != "topsecret" || cloud.S3KeyID != "AKIA" {
		t.Fatalf("cloud creds not re-encrypted/readable on dst: %+v (err=%v)", cloud, err)
	}
	rc, err := dst.svc.decodeRcloneConf(got)
	if err != nil || !strings.Contains(rc, "rclonesecret") {
		t.Fatalf("rclone conf not re-encrypted/readable on dst: %q (err=%v)", rc, err)
	}
	nc, err := dst.svc.NotifyConfig()
	if err != nil || nc.MatrixToken != "matrixsecret" {
		t.Fatalf("notify conf not re-encrypted/readable on dst: %+v (err=%v)", nc, err)
	}
}

// doImport runs the import handler and returns the decoded envelope.
func doImport(t *testing.T, h *Handler, body []byte, query string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	h.handleImportSettings(rec, jsonReq(http.MethodPost, "/api/settings/import"+query, bytes.NewReader(body)))
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode import envelope: %v (body=%s)", err, rec.Body.String())
	}
	return m
}

// Without includeCredentials no secret value appears anywhere in the file.
func TestExportOmitsCredentialsWhenNotRequested(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)

	body, exp := doExport(t, src, "")
	if exp.Credentials != nil {
		t.Fatalf("credentials must be omitted by default: %+v", exp.Credentials)
	}
	for _, secretVal := range []string{"topsecret", "rclonesecret", "matrixsecret"} {
		if bytes.Contains(body, []byte(secretVal)) {
			t.Fatalf("secret %q leaked into a credential-free export", secretVal)
		}
	}
	// Explicit false behaves the same.
	_, exp2 := doExport(t, src, "?includeCredentials=false")
	if exp2.Credentials != nil {
		t.Fatal("includeCredentials=false must omit credentials")
	}
}

func TestImportRejectsBadSchemaAndMalformed(t *testing.T) {
	dst, dstStore := newPortableHandler(t, appKeyB)

	bad := settingsExport{SchemaVersion: 999, Settings: settingsView{ContainersSchedule: "daily 02:00"}}
	badBody, _ := json.Marshal(bad)
	env := doImport(t, dst, badBody, "?apply=true")
	if env["ok"] != false {
		t.Fatalf("bad schema must be rejected: %v", env)
	}

	env2 := doImport(t, dst, []byte("{not json"), "?apply=true")
	if env2["ok"] != false {
		t.Fatalf("malformed body must be rejected: %v", env2)
	}

	s, _ := dstStore.GetSettings()
	if s.ContainersSchedule == "daily 02:00" {
		t.Fatalf("a rejected import must not write settings: %+v", s)
	}
}

// The import applies the same everyN rules as handlePutSettings, through the
// shared rejectEveryNSchedules. A value the save refuses would make every later
// save fail, because the UI always PUTs the full settings object.
func TestImportMatchesTheSaveOnEveryN(t *testing.T) {
	dst, dstStore := newPortableHandler(t, appKeyB)

	// An off-site cadence has no last run to count from, so everyN would fire daily.
	poisoned := settingsExport{
		SchemaVersion: settingsExportSchema,
		Settings:      settingsView{ContainersSchedule: "off", ContainersOffsiteSchedule: "everyN 3 04:00"},
	}
	body, _ := json.Marshal(poisoned)
	env := doImport(t, dst, body, "?apply=true")
	if env["ok"] != false {
		t.Fatalf("an everyN off-site schedule must be rejected on import: %v", env)
	}
	if msg, _ := env["error"].(string); !strings.Contains(msg, "does not support 'everyN'") {
		t.Fatalf("want the everyN guidance error, got %q", msg)
	}
	if s, _ := dstStore.GetSettings(); s.ContainersOffsiteSchedule == "everyN 3 04:00" {
		t.Fatal("the rejected everyN off-site schedule must never reach the store")
	}

	// These schedules record their last run in schedule_job_runs, so everyN is
	// enforceable and the import stores it like the save does.
	for _, tc := range []struct {
		name    string
		cadence string
		view    func(string) settingsView
		stored  func(store.Settings) string
	}{
		{"drills", "everyN 3 04:00",
			func(c string) settingsView { return settingsView{DrillsSchedule: c} },
			func(s store.Settings) string { return s.DrillsSchedule }},
		{"tamperTest", "everyN 5 03:00",
			func(c string) settingsView { return settingsView{TamperTestSchedule: c} },
			func(s store.Settings) string { return s.TamperTestSchedule }},
		{"digest", "everyN 7 09:00",
			func(c string) settingsView { return settingsView{DigestSchedule: c} },
			func(s store.Settings) string { return s.DigestSchedule }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(settingsExport{
				SchemaVersion: settingsExportSchema,
				Settings:      tc.view(tc.cadence),
			})
			if env := doImport(t, dst, b, "?apply=true"); env["ok"] != true {
				t.Fatalf("everyN on %s must import now that its interval is enforced: %v", tc.name, env)
			}
			s, _ := dstStore.GetSettings()
			if got := tc.stored(s); got != tc.cadence {
				t.Fatalf("%s not applied: got %q, want %q", tc.name, got, tc.cadence)
			}
		})
	}

	// Domain schedules have their own due check and accept everyN.
	ok := settingsExport{
		SchemaVersion: settingsExportSchema,
		Settings:      settingsView{ContainersSchedule: "everyN 3 04:00"},
	}
	okBody, _ := json.Marshal(ok)
	if env := doImport(t, dst, okBody, "?apply=true"); env["ok"] != true {
		t.Fatalf("everyN on containersSchedule must still import: %v", env)
	}
	if s, _ := dstStore.GetSettings(); s.ContainersSchedule != "everyN 3 04:00" {
		t.Fatalf("containersSchedule not applied: %q", s.ContainersSchedule)
	}
}

func TestImportWithoutCredentialsPreservesExisting(t *testing.T) {
	dst, dstStore := newPortableHandler(t, appKeyB)
	if err := dst.svc.SetCloudCreds(CloudCreds{S3KeyID: "EXIST", S3Secret: "keepme"}); err != nil {
		t.Fatal(err)
	}
	if err := dst.svc.SetNotifyConfig(notify.Config{On: "always", MatrixHomeserver: "https://keep", MatrixToken: "keeptoken", MatrixRoom: "!k:x"}); err != nil {
		t.Fatal(err)
	}

	// A credential-free export from another instance.
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, exp := doExport(t, src, "")
	if exp.Credentials != nil {
		t.Fatal("precondition: export should have no credentials")
	}

	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}

	cloud, _ := dst.svc.CloudConfig()
	if cloud.S3Secret != "keepme" || cloud.S3KeyID != "EXIST" {
		t.Fatalf("missing credentials must not wipe existing cloud creds: %+v", cloud)
	}
	nc, _ := dst.svc.NotifyConfig()
	if nc.MatrixToken != "keeptoken" {
		t.Fatalf("missing credentials must not wipe existing notify creds: %+v", nc)
	}
	// The settings themselves still import.
	s, _ := dstStore.GetSettings()
	if s.ContainersSchedule != "daily 02:00" {
		t.Fatalf("settings should still import: %+v", s)
	}
}

// restic accepts a credential inside the repo URL, and the recovery kit documents
// that form. s3:, sftp: and b2: locations take the same syntax.
const (
	// A fake credential that has to look real, since the tests check it never
	// leaves in a plain export.
	locWithCreds = "rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers" //nolint:gosec // G101: fake credential, see above
	locRepoPass  = "Tr0ub4dor&3"                                                             //nolint:gosec // G101: fake credential, see above
	locRepoUser  = "backupuser"
)

// seedCredentialInLocation puts a URL-embedded credential into the settings
// off-site location and into both off-site target rows.
func seedCredentialInLocation(t *testing.T, st *store.Repo) {
	t.Helper()
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersOffsite = locWithCreds
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"tgt-1", "tgt-2"} {
		tg, found, err := st.GetOffsiteTarget(id)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Fatalf("test setup: off-site target %q not seeded", id)
		}
		tg.Repo = locWithCreds
		if _, err := st.UpsertOffsiteTarget(tg); err != nil {
			t.Fatal(err)
		}
	}
}

// A rest:, s3:, sftp: or b2: location can hold "user:pass@", and in trusted-LAN
// mode any host can fetch the plain export without logging in. The export drops
// the user and password but keeps the rest, so the file still names a destination.
func TestPlainExportRedactsCredentialInsideRepoLocation(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedCredentialInLocation(t, srcStore)

	body, exp := doExport(t, src, "")

	if bytes.Contains(body, []byte(locRepoPass)) {
		t.Fatalf("the plain export leaked the password embedded in a repo location:\n%s", body)
	}
	if bytes.Contains(body, []byte(locRepoUser)) {
		t.Fatalf("the plain export leaked the username embedded in a repo location:\n%s", body)
	}
	// Scheme, host, port and path stay.
	if !strings.HasPrefix(exp.Settings.ContainersOffsite, "rest:https://") ||
		!strings.Contains(exp.Settings.ContainersOffsite, "storage.example.com:8000/containers") {
		t.Fatalf("redaction destroyed the location instead of just its credential: %q", exp.Settings.ContainersOffsite)
	}
	if !strings.Contains(exp.Settings.ContainersOffsite, redactedLocationMarker) {
		t.Fatalf("a stripped credential must leave a visible marker, got %q", exp.Settings.ContainersOffsite)
	}
	for _, tv := range exp.OffsiteTargets {
		if !strings.Contains(tv.Repo, redactedLocationMarker) || strings.Contains(tv.Repo, locRepoPass) {
			t.Fatalf("off-site target %q location not redacted: %q", tv.ID, tv.Repo)
		}
	}

	// The credentialed export needs a login password and hands out every secret
	// anyway, so it keeps the location whole.
	_, full := doExport(t, src, "?includeCredentials=true")
	if full.Settings.ContainersOffsite != locWithCreds {
		t.Fatalf("the credentialed export must carry the location verbatim, got %q", full.Settings.ContainersOffsite)
	}
}

// Applying a plain export must not replace a working location on the destination
// with the redacted one, which would quietly break its off-site replication.
func TestImportKeepsWorkingLocationWhenFileArrivesRedacted(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedCredentialInLocation(t, srcStore)
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	seedSource(t, dst, dstStore)
	seedCredentialInLocation(t, dstStore)

	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}

	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.ContainersOffsite != locWithCreds {
		t.Fatalf("a redacted location wiped the destination's working one: %q", got.ContainersOffsite)
	}
	targets, err := dstStore.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range targets {
		if tg.Repo != locWithCreds {
			t.Fatalf("off-site target %q lost its working location: %q", tg.ID, tg.Repo)
		}
	}
}

// With nothing to keep, the redacted location is stored anyway. The marker shows
// in Settings and the operator can type the password back in; dropping the
// location would leave the box with no off-site destination and no hint why.
func TestImportOnFreshInstanceKeepsRedactedLocationVisible(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedCredentialInLocation(t, srcStore)
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}

	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.ContainersOffsite, redactedLocationMarker) {
		t.Fatalf("a fresh instance must keep the redacted location, got %q", got.ContainersOffsite)
	}
	if strings.Contains(got.ContainersOffsite, locRepoPass) {
		t.Fatalf("the password must never reach the destination through a plain export: %q", got.ContainersOffsite)
	}
	targets, err := dstStore.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("want the 2 imported targets, got %d", len(targets))
	}
	for _, tg := range targets {
		if !strings.Contains(tg.Repo, redactedLocationMarker) {
			t.Fatalf("off-site target %q should keep the redacted location, got %q", tg.ID, tg.Repo)
		}
	}
}

func TestScrubRepoLocationLeavesCredentialFreeLocationsAlone(t *testing.T) {
	for _, loc := range []string{
		"",
		"s3:offsite-containers",
		"rclone:b2:ark-backups/containers",
		"rest:http://192.168.1.2:8000/containers",
		"backups/containers-offsite",
	} {
		if got := scrubRepoLocation(loc); got != loc {
			t.Fatalf("scrubRepoLocation(%q) = %q, want it untouched", loc, got)
		}
	}
}

// An import writes settings, targets and credentials. The run history stands in
// for repos and snapshots, which it never touches.
func TestImportDoesNotTouchRunHistory(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "?includeCredentials=true")

	dst, dstStore := newPortableHandler(t, appKeyB)
	before, err := dstStore.ListRuns(100)
	if err != nil {
		t.Fatal(err)
	}
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	after, err := dstStore.ListRuns(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 0 || len(after) != 0 {
		t.Fatalf("import must not create run history: before=%d after=%d", len(before), len(after))
	}
}

// TestExportImportCarriesDBDumpsEnabled: the global dump switch is portable
// like every other domain setting, so a rebuilt instance dumps what the old one
// dumped.
func TestExportImportCarriesDBDumpsEnabled(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	s, err := srcStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.DBDumpsEnabled = false
	if err := srcStore.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	body, exp := doExport(t, src, "")
	if exp.Settings.DBDumpsEnabled == nil || *exp.Settings.DBDumpsEnabled {
		t.Fatalf("the export must name the switch, got %v", exp.Settings.DBDumpsEnabled)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply envelope wrong: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.DBDumpsEnabled {
		t.Fatal("an imported off switch must reach the row")
	}
}

// TestImportWithoutDBDumpsFieldKeepsIt: every other bool in the import view is
// copied unconditionally, so a file written before the switch existed would
// switch a default-on safety feature off without a word. Absent means keep.
func TestImportWithoutDBDumpsFieldKeepsIt(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "")

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	settings := raw["settings"].(map[string]any)
	delete(settings, "dbDumpsEnabled")
	older, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, older, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply envelope wrong: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !got.DBDumpsEnabled {
		t.Fatal("an export file that predates the switch must leave it alone")
	}
}

func TestSettingsExportImportCarriesAnomalySettings(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	s, err := srcStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.AnomalyEnabled = false
	s.AnomalySensitivity = "strict"
	s.AnomalyNotifyMin = "warning"
	s.AnomalyRetentionHold = false
	if err := srcStore.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	body, exp := doExport(t, src, "")
	if exp.Settings.AnomalyEnabled == nil || *exp.Settings.AnomalyEnabled {
		t.Fatalf("the export must name the switch, got %v", exp.Settings.AnomalyEnabled)
	}
	if exp.Settings.AnomalySensitivity != "strict" || exp.Settings.AnomalyNotifyMin != "warning" {
		t.Fatalf("preset and minimum not exported: %+v", exp.Settings)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	preview := doImport(t, dst, body, "")
	groups := preview["summary"].(map[string]any)["settingsGroups"].([]any)
	if !slices.Contains(groups, any("anomalies")) {
		t.Fatalf("the preview has to name the group it would change, got %v", groups)
	}
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply envelope wrong: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.AnomalyEnabled || got.AnomalyRetentionHold ||
		got.AnomalySensitivity != "strict" || got.AnomalyNotifyMin != "warning" {
		t.Fatalf("the four fields did not survive the round trip: %+v", got)
	}

	bad := exp
	bad.Settings.AnomalySensitivity = "wild"
	badBody, err := json.Marshal(bad)
	if err != nil {
		t.Fatal(err)
	}
	env := doImport(t, dst, badBody, "?apply=true")
	want := rejectInvalidAnomalySettings(bad.Settings)
	if msg, _ := env["error"].(string); env["ok"] != false || !strings.Contains(msg, want) {
		t.Fatalf("an unknown preset must be refused with %q, got %v", want, env)
	}
}

// Every other switch in the import view is copied as it stands, so a file
// written before this version would turn detection and the data-loss pause off
// on the instance it is applied to.
func TestPreFeatureExportImportsAndKeepsDetectionOn(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "")

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	settings := raw["settings"].(map[string]any)
	for _, key := range []string{"anomalyEnabled", "anomalySensitivity", "anomalyNotifyMin", "anomalyRetentionHold"} {
		delete(settings, key)
	}
	older, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	before, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if env := doImport(t, dst, older, "?apply=true"); env["ok"] != true {
		t.Fatalf("a file from before the feature has to import: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !got.AnomalyEnabled || !got.AnomalyRetentionHold {
		t.Fatalf("detection and the pause must stay on: %+v", got)
	}
	if got.AnomalySensitivity != before.AnomalySensitivity || got.AnomalyNotifyMin != before.AnomalyNotifyMin {
		t.Fatalf("preset and minimum must keep their stored values: %+v", got)
	}
}

// seedMCPKeys stores two keys the way the API does, with fixed material so an
// export can be searched for every part of them.
func seedMCPKeys(t *testing.T, st *store.Repo) []string {
	t.Helper()
	var traces []string
	for _, k := range []struct{ id, label, digest, hint string }{
		{"0b7e0b7e0b7e0b7e0b7e0b7e0b7e0b7e", "Claude Code laptop", "9f1c4b2ade", "Zq7X"},
		{"77aa77aa77aa77aa77aa77aa77aa77aa", "Workshop desktop", "31e8d70ac5", "Wm4P"},
	} {
		if _, err := st.CreateMCPKey(k.id, k.label, k.digest, k.hint, "check-"+k.id, true, 1789600000); err != nil {
			t.Fatalf("seed key %s: %v", k.label, err)
		}
		traces = append(traces, k.label, k.digest, k.hint)
	}
	return traces
}

// activeMCPKeyLabels returns the labels of the keys a request could still
// authenticate with.
func activeMCPKeyLabels(t *testing.T, st *store.Repo) []string {
	t.Helper()
	keys, err := st.ActiveMCPKeys()
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k.Label)
	}
	return out
}

func TestExportNeverCarriesMCPKeys(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	traces := append(seedMCPKeys(t, srcStore), secret.MCPKeyPrefix)

	for _, query := range []string{"", "?includeCredentials=true"} {
		body, _ := doExport(t, src, query)
		for _, trace := range traces {
			if bytes.Contains(body, []byte(trace)) {
				t.Fatalf("export%q carries %q", query, trace)
			}
		}
	}
}

func TestImportLeavesMCPKeysAlone(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "?includeCredentials=true")

	dst, dstStore := newPortableHandler(t, appKeyB)
	seedMCPKeys(t, dstStore)
	before := activeMCPKeyLabels(t, dstStore)

	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply envelope wrong: %v", env)
	}
	if got := activeMCPKeyLabels(t, dstStore); !reflect.DeepEqual(got, before) {
		t.Fatalf("active keys after a plain import = %v, want %v", got, before)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	raw["mcpKeys"] = json.RawMessage(`[{"label":"x","keyDigest":"deadbeef"}]`)
	crafted, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if env := doImport(t, dst, crafted, "?apply=true"); env["ok"] != true {
		t.Fatalf("a file with an unknown top-level field must still import: %v", env)
	}
	got := activeMCPKeyLabels(t, dstStore)
	if !reflect.DeepEqual(got, before) {
		t.Fatalf("a crafted mcpKeys field changed the key set: %v", got)
	}
}

// encoding/json drops both fields when two of them claim the same key, so a
// setting named twice in the view would vanish from the export without an
// error.
func TestSettingsViewNamesEachFieldOnce(t *testing.T) {
	seen := map[string]string{}
	typ := reflect.TypeOf(settingsView{})
	for i := range typ.NumField() {
		f := typ.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if prev, ok := seen[name]; ok {
			t.Errorf("%s and %s are both exported as %q", prev, f.Name, name)
		}
		seen[name] = f.Name
	}
}

// Every setting the database dumps, the ZFS domain and anomaly detection
// brought is written under one key and reaches the instance the file is
// applied to.
func TestExportImportCarriesEveryDumpZFSAndAnomalySettingOnce(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	s, err := srcStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.DBDumpsEnabled = false
	s.ZFSEnabled = true
	s.ZFSPath = "zfs"
	s.ZFSSchedule = "daily 03:30"
	s.ZFSOffsite = "s3:offsite-zfs"
	s.ZFSOffsiteSchedule = "weekly Sun 05:00"
	s.ZFSOffsiteImmutable = true
	s.AnomalyEnabled = false
	s.AnomalySensitivity = "permissive"
	s.AnomalyNotifyMin = "info"
	s.AnomalyRetentionHold = false
	if err := srcStore.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	body, _ := doExport(t, src, "")
	for _, key := range []string{
		"dbDumpsEnabled",
		"zfsEnabled", "zfsPath", "zfsSchedule", "zfsOffsite", "zfsOffsiteSchedule", "zfsOffsiteImmutable",
		"anomalyEnabled", "anomalySensitivity", "anomalyNotifyMin", "anomalyRetentionHold",
	} {
		if n := bytes.Count(body, []byte(`"`+key+`":`)); n != 1 {
			t.Errorf("the export names %q %d times, want once", key, n)
		}
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply envelope wrong: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	type portable struct {
		DBDumpsEnabled                                       bool
		ZFSEnabled                                           bool
		ZFSPath, ZFSSchedule, ZFSOffsite, ZFSOffsiteSchedule string
		ZFSOffsiteImmutable                                  bool
		AnomalyEnabled, AnomalyRetentionHold                 bool
		AnomalySensitivity, AnomalyNotifyMin                 string
	}
	pick := func(x store.Settings) portable {
		return portable{
			x.DBDumpsEnabled, x.ZFSEnabled,
			x.ZFSPath, x.ZFSSchedule, x.ZFSOffsite, x.ZFSOffsiteSchedule, x.ZFSOffsiteImmutable,
			x.AnomalyEnabled, x.AnomalyRetentionHold, x.AnomalySensitivity, x.AnomalyNotifyMin,
		}
	}
	if want, have := pick(s), pick(got); want != have {
		t.Fatalf("after the import:\n got %+v\nwant %+v", have, want)
	}
}

func TestImportOfFileWithoutZFSKeepsTheZFSSetup(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "")

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	settings := raw["settings"].(map[string]any)
	for key := range settings {
		if strings.HasPrefix(key, "zfs") {
			delete(settings, key)
		}
	}
	older, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	s, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ZFSEnabled = true
	s.ZFSPath = "zfs"
	s.ZFSSchedule = "daily 03:30"
	s.ZFSOffsite = "s3:offsite-zfs"
	s.ZFSOffsiteSchedule = "weekly Sun 05:00"
	s.ZFSOffsiteImmutable = true
	if err := dstStore.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	for _, tg := range []store.OffsiteTarget{
		{ID: "tgt-zfs", Domain: zfsDomain, Name: "Primary", Repo: "s3:offsite-zfs", Enabled: true, CreatedAt: 3000, SortOrder: 0},
		{ID: "tgt-zfs-2", Domain: zfsDomain, Name: "Second site", Repo: "s3:offsite-zfs-2", Enabled: true, CreatedAt: 4000, SortOrder: 1},
	} {
		if _, err := dstStore.UpsertOffsiteTarget(tg); err != nil {
			t.Fatal(err)
		}
	}

	if env := doImport(t, dst, older, "?apply=true"); env["ok"] != true {
		t.Fatalf("a file from before the ZFS domain has to import: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !got.ZFSEnabled || got.ZFSPath != "zfs" || got.ZFSSchedule != "daily 03:30" ||
		got.ZFSOffsite != "s3:offsite-zfs" || got.ZFSOffsiteSchedule != "weekly Sun 05:00" || !got.ZFSOffsiteImmutable {
		t.Fatalf("the ZFS setup must survive a file that does not know it: %+v", got)
	}
	if got.ContainersPath != "containers" {
		t.Fatalf("the rest of the file must still apply: containersPath=%q", got.ContainersPath)
	}
	targets, err := dstStore.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	var keptZFS bool
	for _, tg := range targets {
		if tg.ID == "tgt-zfs-2" && tg.Repo == "s3:offsite-zfs-2" {
			keptZFS = true
		}
	}
	if !keptZFS {
		t.Fatalf("the ZFS off-site destination must survive the import: %+v", targets)
	}
}
