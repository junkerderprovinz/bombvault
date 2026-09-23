package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestCloudEnv: only the set credentials become env vars, under the names
// restic reads.
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

// TestSetCloudCredsMergeAndModeEnv: a blank secret on re-save keeps the stored
// one, so the other fields can be edited without retyping it.
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

	settings, _ := s.store.GetSettings()
	env := strings.Join(s.ModeFor(settings).Env, "\n")
	if !strings.Contains(env, "AWS_SECRET_ACCESS_KEY=SEC") || !strings.Contains(env, "AWS_ACCESS_KEY_ID=AK2") {
		t.Fatalf("ModeFor must inject the cloud env: %v", env)
	}

	// An all-blank save clears everything, secrets included.
	if err := s.SetCloudCreds(CloudCreds{}); err != nil {
		t.Fatal(err)
	}
	cleared, _ := s.CloudConfig()
	if (cleared != CloudCreds{}) {
		t.Fatalf("a blank save must clear stored creds, got %+v", cleared)
	}
}

// TestSetCloudCredsStorageClass: the S3 storage class is uppercased on save and
// carried into the restic mode. Archival classes are rejected and leave the
// stored class alone.
func TestSetCloudCredsStorageClass(t *testing.T) {
	s := unraidNotifyService(t, nil)

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

	settings, _ := s.store.GetSettings()
	if mode := s.ModeFor(settings); mode.StorageClass != "STANDARD_IA" {
		t.Fatalf("ModeFor must carry the storage class, got %q", mode.StorageClass)
	}

	for _, bad := range []string{"GLACIER", "DEEP_ARCHIVE", "nonsense"} {
		if err := s.SetCloudCreds(CloudCreds{S3KeyID: "AK", S3StorageClass: bad}); err == nil {
			t.Fatalf("class %q must be rejected", bad)
		}
	}
	again, _ := s.CloudConfig()
	if again.S3StorageClass != "STANDARD_IA" {
		t.Fatalf("a rejected save must not overwrite the stored class, got %q", again.S3StorageClass)
	}
}

// TestHandleGetCloudReturnsStorageClass: unlike the secrets, the storage class
// is returned so the UI can show and edit it.
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

// TestOffsiteModeForTargetStorageClassFallback: a target without its own
// storage class inherits the global one. Targets migrated from the single
// off-site setup all have an empty class, because the SQL migration cannot
// decrypt cloud_conf, so copying it verbatim would wipe the global class on
// every existing install.
func TestOffsiteModeForTargetStorageClassFallback(t *testing.T) {
	s := unraidNotifyService(t, nil)

	if err := s.SetCloudCreds(CloudCreds{S3KeyID: "AK", S3StorageClass: "STANDARD_IA"}); err != nil {
		t.Fatal(err)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	backfilled := store.OffsiteTarget{Domain: "containers", Repo: "s3:c", Enabled: true, StorageClass: ""}
	if got := s.offsiteModeForTarget(settings, backfilled).StorageClass; got != "STANDARD_IA" {
		t.Fatalf("empty target class must preserve the global class, got %q, want STANDARD_IA", got)
	}

	override := store.OffsiteTarget{Domain: "containers", Repo: "s3:c", Enabled: true, StorageClass: "GLACIER_IR"}
	if got := s.offsiteModeForTarget(settings, override).StorageClass; got != "GLACIER_IR" {
		t.Fatalf("a non-empty target class must override, got %q, want GLACIER_IR", got)
	}
}

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

// TestDecodeCloudForResolvesNamedSet: two S3 targets on different providers
// need different keys, so a CredsRef selects its own set instead of the shared
// one.
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

// TestDecodeCloudForUnknownRefFallsBack: a CredsRef whose set was deleted falls
// back to the shared credentials instead of failing. If those do not work,
// restic reports an auth error, which says more than a config error deep in the
// replication path.
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

	// A second target without a CredsRef still uses the shared credentials.
	shared := store.OffsiteTarget{Domain: "containers", Repo: "s3:hetzner", Enabled: true}
	mode2 := s.offsiteModeForTarget(settings, shared)
	if !strings.Contains(strings.Join(mode2.Env, "\n"), "AWS_ACCESS_KEY_ID=SHARED-KEY") {
		t.Fatalf("target with empty CredsRef must keep using the shared creds, got %v", mode2.Env)
	}
}

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

// TestSetCloudCredSetsKeepsSecretOnBlank: as with SetCloudCreds, a blank secret
// keeps the one stored under the same set id.
func TestSetCloudCredSetsKeepsSecretOnBlank(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "a", Name: "First", CloudCreds: CloudCreds{S3KeyID: "K1", S3Secret: "SEC1", RESTPassword: "PW1"}},
	}); err != nil {
		t.Fatal(err)
	}
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

// TestCloudCredSetsBlanksSecretsForUI: CloudCredSets feeds the UI, so it blanks
// secrets like handleGetCloud does.
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
