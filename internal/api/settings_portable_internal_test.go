package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// newPortableHandler builds a Handler backed by an in-memory store and a Service
// keyed by appKey (64 hex chars). It is the export/import test rig: two of these
// with DIFFERENT app keys stand in for two BombVault instances.
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
	// DataDir points at a temp dir so SetRcloneConf writes its 0600 rclone.conf
	// there, never into the package working directory. HostMountRoot carries the
	// production default because the import path now applies the SAME repo-path
	// containment the settings save does, and a handler without a root would
	// refuse every relative path this file seeds.
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
	s.RecoveryKitAck = true // per-instance state — must NOT leak into the export
	// A login password is a PRECONDITION of the CREDENTIALED export: it hands out
	// every backend secret in the clear, so it fails closed when auth is off, the
	// same way the recovery kit does (requireAuthForSecrets; the gate itself is
	// covered by settings_export_gate_test.go). The plain export needs no
	// password — seeding it here keeps one seed serving both.
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

// doExport runs the export handler and returns the raw body + the decoded envelope.
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

// TestSettingsExportImportRoundTrip proves an export -> preview -> apply reproduces
// the settings + off-site targets on a DIFFERENT-keyed instance, and that the
// re-encrypted credentials are readable there.
func TestSettingsExportImportRoundTrip(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)

	body, exp := doExport(t, src, "?includeCredentials=true")

	// The export must carry the config, both targets, and the credentials block.
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

	// PREVIEW on a fresh, different-keyed instance: reports counts, writes nothing.
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

	// APPLY on the destination.
	applyEnv := doImport(t, dst, body, "?apply=true")
	if applyEnv["ok"] != true || applyEnv["applied"] != true {
		t.Fatalf("apply envelope wrong: %v", applyEnv)
	}

	// Settings reproduced.
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.ContainersSchedule != "daily 02:00" || got.DefaultLanguage != "de" || got.RetentionKeepDaily != 7 || !got.DrillsEnabled {
		t.Fatalf("settings not reproduced: %+v", got)
	}

	// Off-site targets reproduced (id + timestamp preserved).
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

	// Credentials re-encrypted with appKeyB: the destination can read them back.
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
	h.handleImportSettings(rec, httptest.NewRequest(http.MethodPost, "/api/settings/import"+query, bytes.NewReader(body)))
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode import envelope: %v (body=%s)", err, rec.Body.String())
	}
	return m
}

// TestExportOmitsCredentialsWhenNotRequested: includeCredentials absent/false must
// omit the credentials block AND never emit a secret value anywhere in the file.
func TestExportOmitsCredentialsWhenNotRequested(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)

	body, exp := doExport(t, src, "") // no includeCredentials
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

// TestImportRejectsBadSchemaAndMalformed: an unsupported schemaVersion and a
// syntactically-broken body are both rejected with ok:false and write nothing.
func TestImportRejectsBadSchemaAndMalformed(t *testing.T) {
	dst, dstStore := newPortableHandler(t, appKeyB)

	// Unsupported schema.
	bad := settingsExport{SchemaVersion: 999, Settings: settingsView{ContainersSchedule: "daily 02:00"}}
	badBody, _ := json.Marshal(bad)
	env := doImport(t, dst, badBody, "?apply=true")
	if env["ok"] != false {
		t.Fatalf("bad schema must be rejected: %v", env)
	}

	// Malformed JSON.
	env2 := doImport(t, dst, []byte("{not json"), "?apply=true")
	if env2["ok"] != false {
		t.Fatalf("malformed body must be rejected: %v", env2)
	}

	// Nothing was written: the schedule stays at its seeded default ("off"), not
	// the "daily 02:00" the rejected files carried.
	s, _ := dstStore.GetSettings()
	if s.ContainersSchedule == "daily 02:00" {
		t.Fatalf("a rejected import must not write settings: %+v", s)
	}
}

// TestImportMatchesTheSaveOnEveryN pins that the import path applies EXACTLY the
// everyN split handlePutSettings applies (#166) — one guard, both write paths,
// via the shared rejectEveryNSchedules.
//
// The guard exists because the two paths once disagreed: the settings import only
// checked that a cadence PARSED, so a hand-written or older-build export could
// smuggle in a value the save itself refused, which then made EVERY later
// settings save fail from ANY card (the UI always PUTs the full settings object,
// so one poisoned field rejected the whole Schedules tab with nothing on screen
// pointing at it).
//
// Both halves of the split are asserted here, because the split has moved once
// and the import path must move WITH it rather than keeping its own stale list:
//
//	off-site cadence      still refused — no last-run fact, would fire daily
//	drills / tamper /     now ACCEPTED and persisted, exactly like a UI-set one:
//	digest cadence        schedule_job_runs (migration v89) makes the interval
//	                      enforceable, so refusing it on import while the save
//	                      accepts it would be the same drift in mirror image
//	domain cadence        accepted, as always (each domain has a due-gate)
func TestImportMatchesTheSaveOnEveryN(t *testing.T) {
	dst, dstStore := newPortableHandler(t, appKeyB)

	// Still refused: an off-site replication cadence has nothing to count from.
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

	// Now accepted, and actually persisted: BaukeZwart's exact cadence on the
	// three schedules that gained a last-run record. An imported value must land
	// wherever a UI-set one would.
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

	// The long-standing counterpart: everyN on a domain schedule still imports.
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

// TestImportWithoutCredentialsPreservesExisting: an apply of a file with NO
// credentials block leaves the destination's stored secrets untouched.
func TestImportWithoutCredentialsPreservesExisting(t *testing.T) {
	dst, dstStore := newPortableHandler(t, appKeyB)
	// Destination already has cloud + notify creds.
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
	// The settings themselves DID import.
	s, _ := dstStore.GetSettings()
	if s.ContainersSchedule != "daily 02:00" {
		t.Fatalf("settings should still import: %+v", s)
	}
}

// A repo location an operator is entitled to write and restic is happy to use:
// the credential lives inside the URL. The generated recovery kit documents this
// exact shape ("They can also live inside the URL, e.g. rest:https://user:pass@
// host:8000/path"), and s3:, sftp: and b2: locations take the same syntax.
const (
	// The fake credential is the fixture: this file exists to prove a password
	// embedded in a repo URL never leaves in an export, so the literal has to
	// look exactly like one gosec would flag. Nothing here is real or reachable.
	locWithCreds = "rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers" //nolint:gosec // G101: deliberate fixture, see above
	locRepoPass  = "Tr0ub4dor&3"                                                             //nolint:gosec // G101: deliberate fixture, see above
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

// TestPlainExportRedactsCredentialInsideRepoLocation is the regression proof for
// the finding that the plain export emitted every repo location VERBATIM while
// claiming to carry no secrets. A rest:/s3:/sftp:/b2: location may hold a live
// "user:pass@", and the plain export is the variant any host on the LAN can fetch
// unauthenticated in trusted-LAN mode — so the password (and the username) must
// not be in the file, while the location itself, which is legitimately portable
// configuration, must survive so the file still names a destination.
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
	// …and it is still a usable settings file: scheme, host, port and path stay.
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

	// The CREDENTIALED variant is the opposite case: it is gated on a login
	// password precisely because it hands out every secret in the clear, so it
	// must keep the location whole — otherwise the one export meant to be a
	// complete portable copy would be the only lossy one.
	_, full := doExport(t, src, "?includeCredentials=true")
	if full.Settings.ContainersOffsite != locWithCreds {
		t.Fatalf("the credentialed export must carry the location verbatim, got %q", full.Settings.ContainersOffsite)
	}
}

// TestImportKeepsWorkingLocationWhenFileArrivesRedacted pins the other half of
// the round trip: a plain export cannot carry the credential, so applying one
// must not overwrite a location that already WORKS on the destination with the
// redacted stand-in. That would break the off-site replication of an instance
// that was fine before the import, and break it quietly.
func TestImportKeepsWorkingLocationWhenFileArrivesRedacted(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedCredentialInLocation(t, srcStore)
	body, _ := doExport(t, src, "")

	// The destination already has the same targets, with WORKING locations.
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

// TestImportOnFreshInstanceKeepsRedactedLocationVisible: where the destination
// has NOTHING to keep, the redacted location still lands. Dropping it would leave
// a box with no off-site destination at all — the silent failure — whereas the
// marker is on screen in Settings and the next run fails against a location the
// operator can repair by typing the password back in.
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

// TestScrubRepoLocationLeavesCredentialFreeLocationsAlone: the scrub must not
// fire on the ordinary locations this app is full of — a bare s3:/b2: bucket, an
// rclone remote, a rest: URL with no userinfo, a relative local path — or every
// export would come back mangled.
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

// TestImportDoesNotTouchRunHistory: an apply writes settings/targets/creds but
// never creates a run record (proxy for "never touches repos/snapshots/history").
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
