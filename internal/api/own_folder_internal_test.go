package api

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func ownSteps(t *testing.T, res map[string]any) []ownFolderStep {
	t.Helper()
	if res["ok"] != true {
		t.Fatalf("answer = %v", res)
	}
	b, err := json.Marshal(res["steps"])
	if err != nil {
		t.Fatal(err)
	}
	var out []ownFolderStep
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// waitOwnFolder waits until the copy before a switch has ended.
func (f *placementFixture) waitOwnFolder(domain string) {
	f.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		f.svc.cancelMu.Lock()
		_, running := f.svc.runCancels["own:"+domain]
		f.svc.cancelMu.Unlock()
		if !running {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	f.t.Fatalf("the own folder of %s did not move", domain)
}

func TestTheOwnFolderPreviewSaysACopyThereBecomesTheOwnFolder(t *testing.T) {
	f := newPlacementFixture(t)
	box, vms := boxWithVMs(f)

	steps := ownSteps(t, f.do("POST", "/api/storage/own-folders/preview", map[string]any{
		"domains": []string{"vms"}, "location": "destination:" + box.ID, "copyOver": true,
	}))

	if len(steps) != 1 {
		t.Fatalf("steps = %+v", steps)
	}
	got := steps[0]
	if got.CopyAt != vms.ID || got.To != boxRepo+"/vms" || got.Backups != 2 || got.Pending != 0 || got.Same {
		t.Fatalf("step = %+v, want the copy at Box named, two backups and none missing there", got)
	}
	if len(f.eng.copies) != 0 {
		t.Fatalf("the preview copied %+v", f.eng.copies)
	}
}

func TestMovingAnOwnFolderOntoItsCopyEndsTheCopyAndLinksTheLocation(t *testing.T) {
	f := newPlacementFixture(t)
	box, vms := boxWithVMs(f)

	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"vms"}, "location": "destination:" + box.ID, "copyOver": true,
	}))

	settings, _ := f.st.GetSettings()
	if settings.VMsPath != boxRepo+"/vms" {
		t.Fatalf("path = %q", settings.VMsPath)
	}
	if _, ok, _ := f.st.GetOffsiteTarget(vms.ID); ok {
		t.Fatal("the section still copies onto its own folder")
	}
	row, ok := f.svc.primaryRemoteTarget("vms")
	if !ok || row.DestinationID != box.ID {
		t.Fatalf("row = %+v, want it linked to Box", row)
	}
	locations, err := f.svc.StorageLocations()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range locations {
		home := slices.ContainsFunc(l.Sections, func(s locationSection) bool { return s.Domain == "vms" && s.Use == useHome })
		if home != (l.ID == "destination:"+box.ID) {
			t.Fatalf("%s lists the own folder of vms: %v", l.ID, home)
		}
	}
}

func TestMovingAnOwnFolderCopiesTheBackupsAlreadyMadeFirst(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:web"), snap("a2", 200, "container:web"))

	steps := ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers"}, "location": "destination:" + vault.ID, "copyOver": true,
	}))
	if steps[0].Pending != 2 {
		t.Fatalf("step = %+v, want two backups to copy", steps[0])
	}
	f.waitOwnFolder("containers")

	if got := f.snapsAt(vaultRepo + "/containers"); len(got) != 2 {
		t.Fatalf("the new own folder holds %d backups, want 2", len(got))
	}
	if settings, _ := f.st.GetSettings(); settings.ContainersPath != vaultRepo+"/containers" {
		t.Fatalf("path = %q", settings.ContainersPath)
	}
}

func TestMovingAnOwnFolderWithoutCopyingSwitchesAtOnce(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:web"))

	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers"}, "location": "destination:" + vault.ID,
	}))

	if settings, _ := f.st.GetSettings(); settings.ContainersPath != vaultRepo+"/containers" {
		t.Fatalf("path = %q", settings.ContainersPath)
	}
	if len(f.eng.copies) != 0 {
		t.Fatalf("copied %+v although the toggle was off", f.eng.copies)
	}
}

func TestAnOwnFolderInALocationSignsInWithTheLocationsSignIn(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "def", Name: "Def", Default: true, CloudCreds: CloudCreds{S3KeyID: "DEF", S3Secret: "d"}},
		{ID: "garage", Name: "Garage", CloudCreds: CloudCreds{S3KeyID: "GARAGE", S3Secret: "g"}},
	}); err != nil {
		t.Fatal(err)
	}
	garage, err := f.st.SaveDestination(store.OffsiteTarget{Name: "Garage", Repo: "s3:https://garage/bv", CredsRef: "garage", Provider: "box"})
	if err != nil {
		t.Fatal(err)
	}

	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"files", "containers"}, "location": "destination:" + garage.ID,
	}))

	settings, _ := f.st.GetSettings()
	for _, domain := range []string{"files", "containers"} {
		path := domainPathRaw(domain, settings)
		env := strings.Join(f.svc.primaryModeFor(settings, domain, path).Env, "\n")
		if !strings.Contains(env, "AWS_ACCESS_KEY_ID=GARAGE") {
			t.Fatalf("%s backs up to %s with %v, want Garage's sign-in", domain, path, env)
		}
	}
}

func TestATypedOwnFolderSignsInWithTheSavedSignInPicked(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "nas", Name: "NAS", CloudCreds: CloudCreds{RESTUser: "u", RESTPassword: "p"}}}); err != nil {
		t.Fatal(err)
	}

	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"files"}, "address": "rest:http://nas:8000/bv/", "credsRef": "nas",
	}))

	settings, _ := f.st.GetSettings()
	if settings.FilesPath != "rest:http://nas:8000/bv/files" {
		t.Fatalf("path = %q", settings.FilesPath)
	}
	env := strings.Join(f.svc.primaryModeFor(settings, "files", settings.FilesPath).Env, "\n")
	if !strings.Contains(env, "RESTIC_REST_USERNAME=u") {
		t.Fatalf("env = %v, want the NAS sign-in", env)
	}
}

func TestAnOwnFolderAlreadyThereIsLeftAlone(t *testing.T) {
	f := newPlacementFixture(t)
	steps := ownSteps(t, f.do("POST", "/api/storage/own-folders/preview", map[string]any{
		"domains": []string{"files"}, "address": "backups",
	}))
	if !steps[0].Same {
		t.Fatalf("step = %+v, want it marked as already there", steps[0])
	}
}

func TestAnOwnFolderMovesFromThisServerToAnS3LocationWithItsOwnSignIn(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "def", Name: "Def", Default: true, CloudCreds: CloudCreds{S3KeyID: "DEF", S3Secret: "d", S3Region: "eu-1"}},
		{ID: "garage", Name: "Garage", CloudCreds: CloudCreds{S3KeyID: "GARAGE", S3Secret: "g", S3Region: "garage"}},
	}); err != nil {
		t.Fatal(err)
	}
	garage, err := f.st.SaveDestination(store.OffsiteTarget{Name: "Garage", Repo: "s3:https://garage/bv", CredsRef: "garage", Provider: "box"})
	if err != nil {
		t.Fatal(err)
	}
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:web"), snap("a2", 200, "container:web"))

	steps := ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers"}, "location": "destination:" + garage.ID, "copyOver": true,
	}))
	if steps[0].Pending != 2 {
		t.Fatalf("step = %+v, want two backups to copy", steps[0])
	}
	f.waitOwnFolder("containers")

	to := "s3:https://garage/bv/containers"
	if got := f.snapsAt(to); len(got) != 2 {
		t.Fatalf("the new own folder holds %d backups, want 2", len(got))
	}
	f.eng.mu.Lock()
	envs := f.eng.signIns["copy "+to]
	f.eng.mu.Unlock()
	if len(envs) == 0 {
		t.Fatal("nothing was copied to Garage")
	}
	env := strings.Join(envs[0], "\n")
	if !strings.Contains(env, "AWS_ACCESS_KEY_ID=GARAGE") || strings.Contains(env, "=DEF") || strings.Contains(env, "=eu-1") {
		t.Fatalf("the copy signed in with %v, want only Garage's sign-in", env)
	}
}

func TestEachPlaceBringsOnlyTheSignInItsBackendReads(t *testing.T) {
	local := []string{"AWS_ACCESS_KEY_ID=DEF", "AWS_SECRET_ACCESS_KEY=d", "RESTIC_REST_USERNAME=bv"}
	s3 := []string{"AWS_ACCESS_KEY_ID=GARAGE", "AWS_SECRET_ACCESS_KEY=g"}

	env, ok := bothSignIns("s3:https://garage/bv", s3, "/mnt/user/bombvault/flash", local)
	if !ok || !slices.Equal(env, s3) {
		t.Fatalf("a folder here and an S3 bucket joined to %v, %v, want only the bucket's keys", env, ok)
	}
	if _, ok := bothSignIns("s3:https://garage/bv", s3, "s3:https://aws/bv", local); ok {
		t.Fatal("two S3 places with different keys joined")
	}
}
