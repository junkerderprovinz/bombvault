package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestCloudEnv: only the set credentials become env vars, with restic's names.
func TestCloudEnv(t *testing.T) {
	env := cloudEnv(CloudCreds{
		S3KeyID: "AK", S3Secret: "SEC", S3Region: "eu-west-1",
		RESTUser: "u", RESTPassword: "p",
	})
	joined := strings.Join(env, "\n")
	for _, want := range []string{
		"AWS_ACCESS_KEY_ID=AK", "AWS_SECRET_ACCESS_KEY=SEC", "AWS_DEFAULT_REGION=eu-west-1",
		"RESTIC_REST_USERNAME=u", "RESTIC_REST_PASSWORD=p",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, env)
		}
	}
	if len(cloudEnv(CloudCreds{})) != 0 {
		t.Fatal("empty creds must yield no env")
	}
	if got := cloudEnv(CloudCreds{S3Region: "x"}); len(got) != 1 {
		t.Fatalf("only set fields become env, got %v", got)
	}
}

// TestSetCloudCredsMergeAndModeEnv: round-trips the creds, keeps secrets on a
// blank re-save (so non-secret fields can be edited), and ModeFor injects them.
func TestSetCloudCredsMergeAndModeEnv(t *testing.T) {
	s := unraidNotifyService(t, nil)

	if err := s.SetCloudCreds(CloudCreds{
		S3KeyID: "AK", S3Secret: "SEC", S3Region: "eu", RESTUser: "u", RESTPassword: "p",
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.CloudConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.S3Secret != "SEC" || got.RESTPassword != "p" {
		t.Fatalf("round-trip lost secrets: %+v", got)
	}

	// Edit non-secret fields with blank secrets → secrets are kept.
	if err := s.SetCloudCreds(CloudCreds{S3KeyID: "AK2", S3Region: "us", RESTUser: "u2"}); err != nil {
		t.Fatal(err)
	}
	got2, _ := s.CloudConfig()
	if got2.S3Secret != "SEC" || got2.RESTPassword != "p" {
		t.Fatalf("blank secrets must keep the stored ones: %+v", got2)
	}
	if got2.S3KeyID != "AK2" || got2.S3Region != "us" || got2.RESTUser != "u2" {
		t.Fatalf("non-secret edits must apply: %+v", got2)
	}

	// ModeFor injects the credentials as restic env.
	settings, _ := s.store.GetSettings()
	env := strings.Join(s.ModeFor(settings).Env, "\n")
	if !strings.Contains(env, "AWS_SECRET_ACCESS_KEY=SEC") || !strings.Contains(env, "AWS_ACCESS_KEY_ID=AK2") {
		t.Fatalf("ModeFor must inject the cloud env: %v", env)
	}

	// A fully-blank save clears the stored credentials, even after secrets existed.
	if err := s.SetCloudCreds(CloudCreds{}); err != nil {
		t.Fatal(err)
	}
	cleared, _ := s.CloudConfig()
	if (cleared != CloudCreds{}) {
		t.Fatalf("a blank save must clear stored creds, got %+v", cleared)
	}
}

// TestSetCloudCredsStorageClass: the off-site S3 storage class round-trips through
// SetCloudCreds -> decodeCloud -> ModeFor (the mode carries it), is normalized to
// uppercase, and a non-whitelisted (archival) class is rejected on save.
func TestSetCloudCredsStorageClass(t *testing.T) {
	s := unraidNotifyService(t, nil)

	// A lowercase whitelisted class is normalized and persisted.
	if err := s.SetCloudCreds(CloudCreds{S3KeyID: "AK", S3StorageClass: "standard_ia"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.CloudConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.S3StorageClass != "STANDARD_IA" {
		t.Fatalf("storage class must be uppercased/persisted, got %q", got.S3StorageClass)
	}

	// ModeFor carries the class into the restic Mode.
	settings, _ := s.store.GetSettings()
	if mode := s.ModeFor(settings); mode.StorageClass != "STANDARD_IA" {
		t.Fatalf("ModeFor must carry the storage class, got %q", mode.StorageClass)
	}

	// A non-whitelisted (archival) class is rejected and never stored.
	for _, bad := range []string{"GLACIER", "DEEP_ARCHIVE", "nonsense"} {
		if err := s.SetCloudCreds(CloudCreds{S3KeyID: "AK", S3StorageClass: bad}); err == nil {
			t.Fatalf("class %q must be rejected", bad)
		}
	}
	// The rejected saves left the previous valid value intact.
	again, _ := s.CloudConfig()
	if again.S3StorageClass != "STANDARD_IA" {
		t.Fatalf("a rejected save must not overwrite the stored class, got %q", again.S3StorageClass)
	}
}

// TestHandleGetCloudReturnsStorageClass: unlike the secret fields, handleGetCloud
// echoes s3StorageClass so the UI can show and re-edit it.
func TestHandleGetCloudReturnsStorageClass(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCreds(CloudCreds{S3KeyID: "AK", S3Secret: "SEC", S3StorageClass: "GLACIER_IR"}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{svc: s}
	w := httptest.NewRecorder()
	h.handleGetCloud(w, httptest.NewRequest(http.MethodGet, "/api/cloud", nil))

	var env struct {
		OK             bool   `json:"ok"`
		S3StorageClass string `json:"s3StorageClass"`
		S3Secret       string `json:"s3Secret"`
		S3SecretSet    bool   `json:"s3SecretSet"`
	}
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.S3StorageClass != "GLACIER_IR" {
		t.Fatalf("handleGetCloud must return the storage class, got %q", env.S3StorageClass)
	}
	if env.S3Secret != "" {
		t.Fatal("the secret must never be echoed")
	}
	if !env.S3SecretSet {
		t.Fatal("secret presence flag should still report set")
	}
}

// TestOffsiteModeForTargetStorageClassFallback pins the #1 multi-off-site
// regression trap: the per-DESTINATION restic mode must PRESERVE the global S3
// storage class when a target does not carry its own. The stage-1 backfill left
// offsite_targets.storage_class = "" (the pure-SQL migration cannot decrypt the
// cloud_conf blob), while the global class still lives in CloudCreds — so a
// target with class "" must fall back to it, and only a non-empty target class
// overrides. Unconditionally copying the (empty) target class would wipe the
// global to "" for every existing single-off-site install.
func TestOffsiteModeForTargetStorageClassFallback(t *testing.T) {
	s := unraidNotifyService(t, nil)

	// Global class set on the shared cloud creds (as an existing install has it).
	if err := s.SetCloudCreds(CloudCreds{S3KeyID: "AK", S3StorageClass: "STANDARD_IA"}); err != nil {
		t.Fatal(err)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	// A backfilled N=1 target has an empty StorageClass → the global is preserved.
	backfilled := store.OffsiteTarget{Domain: "containers", Repo: "s3:c", Enabled: true, StorageClass: ""}
	if got := s.offsiteModeForTarget(settings, backfilled).StorageClass; got != "STANDARD_IA" {
		t.Fatalf("empty target class must preserve the global class, got %q, want STANDARD_IA", got)
	}

	// A target that sets its own class overrides the global for that destination.
	override := store.OffsiteTarget{Domain: "containers", Repo: "s3:c", Enabled: true, StorageClass: "GLACIER_IR"}
	if got := s.offsiteModeForTarget(settings, override).StorageClass; got != "GLACIER_IR" {
		t.Fatalf("a non-empty target class must override, got %q, want GLACIER_IR", got)
	}
}

// TestDecodeCloudForEmptyRefUsesSharedCreds: the #141 stage-2 resolver must be a
// no-op for every existing off-site target (empty CredsRef, today's behavior).
func TestDecodeCloudForEmptyRefUsesSharedCreds(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCreds(CloudCreds{S3KeyID: "SHARED-KEY", S3Secret: "SHARED-SEC"}); err != nil {
		t.Fatal(err)
	}
	settings, _ := s.store.GetSettings()
	got, err := s.decodeCloudFor(settings, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.S3KeyID != "SHARED-KEY" || got.S3Secret != "SHARED-SEC" {
		t.Fatalf("empty credsRef must resolve to the shared creds, got %+v", got)
	}
}

// TestDecodeCloudForResolvesNamedSet: a target with a CredsRef gets THAT set's
// credentials, not the shared ones — the actual bug manilx hit in #141 (two S3
// targets, e.g. Hetzner + a local Garage instance, needing different keys).
func TestDecodeCloudForResolvesNamedSet(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCreds(CloudCreds{S3KeyID: "SHARED-KEY", S3Secret: "SHARED-SEC"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "garage", Name: "Local Garage", CloudCreds: CloudCreds{S3KeyID: "GARAGE-KEY", S3Secret: "GARAGE-SEC", S3Region: "garage"}},
	}); err != nil {
		t.Fatal(err)
	}
	settings, _ := s.store.GetSettings()
	got, err := s.decodeCloudFor(settings, "garage")
	if err != nil {
		t.Fatal(err)
	}
	if got.S3KeyID != "GARAGE-KEY" || got.S3Secret != "GARAGE-SEC" {
		t.Fatalf("a named credsRef must resolve to THAT set, got %+v (would mean both targets share one key, the exact #141 bug)", got)
	}
}

// TestDecodeCloudForUnknownRefFallsBack: a credsRef that no longer resolves (the
// set was deleted after a target referenced it) falls back to the shared creds
// rather than erroring the caller — restic then fails loudly on auth if that
// fallback genuinely has no usable credentials, a clearer signal than an opaque
// config error deep in the replication path.
func TestDecodeCloudForUnknownRefFallsBack(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCreds(CloudCreds{S3KeyID: "SHARED-KEY"}); err != nil {
		t.Fatal(err)
	}
	settings, _ := s.store.GetSettings()
	got, err := s.decodeCloudFor(settings, "does-not-exist")
	if err != nil {
		t.Fatalf("an unresolved credsRef must fall back, not error: %v", err)
	}
	if got.S3KeyID != "SHARED-KEY" {
		t.Fatalf("unresolved credsRef must fall back to shared creds, got %+v", got)
	}
}

// TestOffsiteModeForTargetUsesNamedCredsEnv: the actual restic Mode a
// replication/test-connection run uses must carry the NAMED set's env vars
// (not the shared ones) when the target names a credsRef — this is the
// integration point manilx's #141 report hit in practice.
func TestOffsiteModeForTargetUsesNamedCredsEnv(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCreds(CloudCreds{S3KeyID: "SHARED-KEY", S3Secret: "SHARED-SEC"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "garage", Name: "Local Garage", CloudCreds: CloudCreds{S3KeyID: "GARAGE-KEY", S3Secret: "GARAGE-SEC", S3StorageClass: "STANDARD"}},
	}); err != nil {
		t.Fatal(err)
	}
	settings, _ := s.store.GetSettings()

	named := store.OffsiteTarget{Domain: "containers", Repo: "s3:garage", Enabled: true, CredsRef: "garage"}
	mode := s.offsiteModeForTarget(settings, named)
	env := strings.Join(mode.Env, "\n")
	if !strings.Contains(env, "AWS_ACCESS_KEY_ID=GARAGE-KEY") {
		t.Fatalf("target with CredsRef must use the named set's env, got %v", mode.Env)
	}
	if strings.Contains(env, "SHARED-KEY") {
		t.Fatalf("target with CredsRef must NOT also carry the shared key, got %v", mode.Env)
	}
	if mode.StorageClass != "STANDARD" {
		t.Fatalf("target with CredsRef must inherit that set's storage class when its own is empty, got %q", mode.StorageClass)
	}

	// An empty CredsRef on a second target still gets the shared creds — the two
	// targets are independent (this is the actual "can I have two S3 targets with
	// different keys" case from #141).
	shared := store.OffsiteTarget{Domain: "containers", Repo: "s3:hetzner", Enabled: true}
	mode2 := s.offsiteModeForTarget(settings, shared)
	if !strings.Contains(strings.Join(mode2.Env, "\n"), "AWS_ACCESS_KEY_ID=SHARED-KEY") {
		t.Fatalf("target with empty CredsRef must keep using the shared creds, got %v", mode2.Env)
	}
}

// TestSetCloudCredSetsValidation: a blank name, a duplicate id, or a
// non-whitelisted storage class must all be rejected and never stored.
func TestSetCloudCredSetsValidation(t *testing.T) {
	s := unraidNotifyService(t, nil)

	if err := s.SetCloudCredSets([]CloudCredSet{{ID: "a", Name: "  "}}); err == nil {
		t.Fatal("a blank name must be rejected")
	}
	if err := s.SetCloudCredSets([]CloudCredSet{{ID: "", Name: "x"}}); err == nil {
		t.Fatal("an empty id must be rejected")
	}
	if err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "a", Name: "A"}, {ID: "a", Name: "B"},
	}); err == nil {
		t.Fatal("a duplicate id must be rejected")
	}
	if err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "a", Name: "A", CloudCreds: CloudCreds{S3StorageClass: "GLACIER"}},
	}); err == nil {
		t.Fatal("a non-whitelisted storage class must be rejected")
	}

	sets, err := s.CloudCredSets()
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 0 {
		t.Fatalf("every rejected save must leave storage empty, got %+v", sets)
	}
}

// TestSetCloudCredSetsKeepsSecretOnBlank mirrors SetCloudCreds's keep-prior-
// if-blank contract: editing a set's non-secret fields with a blank secret
// keeps the previously stored secret (matched by id).
func TestSetCloudCredSetsKeepsSecretOnBlank(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "a", Name: "First", CloudCreds: CloudCreds{S3KeyID: "K1", S3Secret: "SEC1", RESTPassword: "PW1"}},
	}); err != nil {
		t.Fatal(err)
	}
	// Rename it, blank both secrets.
	if err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "a", Name: "Renamed", CloudCreds: CloudCreds{S3KeyID: "K1"}},
	}); err != nil {
		t.Fatal(err)
	}
	settings, _ := s.store.GetSettings()
	sets, err := s.decodeCloudCredSets(settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0].Name != "Renamed" {
		t.Fatalf("rename must apply, got %+v", sets)
	}
	if sets[0].S3Secret != "SEC1" || sets[0].RESTPassword != "PW1" {
		t.Fatalf("blank secrets on save must keep the stored ones, got %+v", sets[0])
	}
}

// TestCloudCredSetsBlanksSecretsForUI: CloudCredSets() (the UI-facing list) must
// never leak a real secret, same contract as CloudConfig()/handleGetCloud.
func TestCloudCredSetsBlanksSecretsForUI(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "a", Name: "A", CloudCreds: CloudCreds{S3KeyID: "K", S3Secret: "SEC", RESTPassword: "PW"}},
	}); err != nil {
		t.Fatal(err)
	}
	sets, err := s.CloudCredSets()
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 {
		t.Fatalf("expected 1 set, got %d", len(sets))
	}
	if sets[0].S3Secret != "" || sets[0].RESTPassword != "" {
		t.Fatalf("CloudCredSets() must never return real secrets, got %+v", sets[0])
	}
	if sets[0].S3KeyID != "K" {
		t.Fatalf("non-secret fields must still be returned, got %+v", sets[0])
	}
}
