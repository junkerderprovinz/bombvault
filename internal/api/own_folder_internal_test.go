package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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
	f.waitRun("own:" + domain)
}

// waitRun waits until the background run under key has ended.
func (f *placementFixture) waitRun(key string) {
	f.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		f.svc.cancelMu.Lock()
		_, running := f.svc.runCancels[key]
		f.svc.cancelMu.Unlock()
		if !running {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	f.t.Fatalf("%s did not end", key)
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

func TestAnOwnFolderInALocationWritesInTheLocationsStorageClass(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "garage", Name: "Garage", CloudCreds: CloudCreds{S3KeyID: "GARAGE", S3Secret: "g", S3StorageClass: "STANDARD"}},
	}); err != nil {
		t.Fatal(err)
	}
	cold, err := f.st.SaveDestination(store.OffsiteTarget{Name: "Cold", Repo: "s3:https://garage/cold", CredsRef: "garage", StorageClass: "GLACIER_IR", Provider: "box"})
	if err != nil {
		t.Fatal(err)
	}
	req := ownFolderRequest{Domains: []string{"files"}, Location: "destination:" + cold.ID, CopyOver: true}
	steps, err := f.svc.planOwnFolders(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got := steps[0].plan.mode.StorageClass; got != "GLACIER_IR" {
		t.Fatalf("the copy over writes in %q, want the location's class", got)
	}

	req.CopyOver = false
	ownSteps(t, f.do("POST", "/api/storage/own-folders", req))
	settings := settingsOf(t, f.svc)
	if got := f.svc.primaryModeFor(settings, "files", settings.FilesPath).StorageClass; got != "GLACIER_IR" {
		t.Fatalf("the section backs up in %q, want the location's class over the sign-in's", got)
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

func TestAnOwnFolderOnAShareNotMountedDoesNotSwitchWithoutItsBackups(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	old := f.domainPath("containers")
	if err := f.st.MarkRepoEstablished(old); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(filepath.FromSlash(old), "config")); err != nil {
		t.Fatal(err)
	}
	f.eng.opens[old] = false
	move := map[string]any{"domains": []string{"containers"}, "location": "destination:" + vault.ID, "copyOver": true}

	if got := f.do("POST", "/api/storage/own-folders", move); got["ok"] != false || got["code"] != "own-folder-unreadable" {
		t.Fatalf("answer = %v, want the move refused because the old backups cannot be read", got)
	}
	if settings, _ := f.st.GetSettings(); settings.ContainersPath != "backups/containers" {
		t.Fatalf("path = %q, want the section left at its old folder", settings.ContainersPath)
	}

	move["copyOver"] = false
	ownSteps(t, f.do("POST", "/api/storage/own-folders", move))
	if settings, _ := f.st.GetSettings(); settings.ContainersPath != vaultRepo+"/containers" {
		t.Fatalf("path = %q, want a move without the copy to go ahead", settings.ContainersPath)
	}
}

func TestABucketThatHeldTheOwnFolderAndHoldsNoRepositoryNowIsNotEmpty(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	const old = "rest:http://nas:8000/bv/containers"
	settings, _ := f.st.GetSettings()
	settings.ContainersPath = old
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.eng.opens[old] = false
	preview := map[string]any{"domains": []string{"containers"}, "location": "destination:" + vault.ID, "copyOver": true}

	if steps := ownSteps(t, f.do("POST", "/api/storage/own-folders/preview", preview)); steps[0].Pending != 0 {
		t.Fatalf("step = %+v, want a bucket never written to moved with nothing to copy", steps[0])
	}
	if err := f.st.MarkRepoEstablished(old); err != nil {
		t.Fatal(err)
	}
	if got := f.do("POST", "/api/storage/own-folders/preview", preview); got["code"] != "own-folder-unreadable" {
		t.Fatalf("answer = %v, want the move refused once the bucket held backups", got)
	}
}

func TestAnOwnFolderMovesWithItsBackupsBetweenTwoS3LocationsWithOtherKeys(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "garage", Name: "Garage", CloudCreds: CloudCreds{S3KeyID: "GARAGE", S3Secret: "g", S3Region: "garage"}},
		{ID: "rustfs", Name: "RustFS", CloudCreds: CloudCreds{S3KeyID: "RUSTFS", S3Secret: "r"}},
	}); err != nil {
		t.Fatal(err)
	}
	garage, err := f.st.SaveDestination(store.OffsiteTarget{Name: "Garage", Repo: "s3:http://garage:3900/bv", CredsRef: "garage", Provider: "garage"})
	if err != nil {
		t.Fatal(err)
	}
	rustfs, err := f.st.SaveDestination(store.OffsiteTarget{Name: "RustFS", Repo: "s3:http://rustfs:9000/bv", CredsRef: "rustfs", Provider: "rustfs"})
	if err != nil {
		t.Fatal(err)
	}
	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"flash"}, "location": "destination:" + garage.ID,
	}))
	f.hold("s3:http://garage:3900/bv/flash", snap("f1", 100, "flash"))

	steps, err := f.svc.planOwnFolders(context.Background(), ownFolderRequest{
		Domains: []string{"flash"}, Location: "destination:" + rustfs.ID, CopyOver: true,
	})
	if err != nil {
		t.Fatalf("moving back to RustFS with the backups = %v", err)
	}
	plan := steps[0].plan
	if steps[0].Pending != 1 || plan.copySrc != "rclone:"+sourceRemote+":bv/flash" {
		t.Fatalf("step = %+v, copy from %q, want one backup read through Garage's own remote", steps[0], plan.copySrc)
	}
	env := strings.Join(plan.copyMode.Env, "\n")
	if !strings.Contains(env, "AWS_ACCESS_KEY_ID=RUSTFS") || strings.Contains(env, "AWS_ACCESS_KEY_ID=GARAGE") || !strings.Contains(env, sourceRemote+"_REGION=garage") {
		t.Fatalf("env %v, want RustFS's keys for RustFS and Garage's only on its remote", env)
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

func TestACopyThatBecomesTheOwnFolderKeepsWhatItsRuleKept(t *testing.T) {
	f := newPlacementFixture(t)
	localKeepLast(t, f, 3)
	box, err := f.st.SaveDestination(store.OffsiteTarget{Name: "Box", Repo: boxRepo, Provider: "box", RetentionKeepMonthly: 24})
	if err != nil {
		t.Fatal(err)
	}
	f.tickDestination(box, "vms")

	steps := ownSteps(t, f.do("POST", "/api/storage/own-folders/preview", map[string]any{
		"domains": []string{"vms"}, "location": "destination:" + box.ID,
	}))
	if !steps[0].WiderKeep {
		t.Fatalf("step = %+v, want it to say the copy keeps more", steps[0])
	}
	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"vms"}, "location": "destination:" + box.ID,
	}))

	settings, _ := f.st.GetSettings()
	if got := localKeep(settings, "vms"); got != (store.RetentionKeep{KeepLast: 3, KeepMonthly: 24}) {
		t.Fatalf("vms keeps %+v, want both rules", got)
	}
	if got := localKeep(settings, "containers"); got != (store.RetentionKeep{KeepLast: 3}) {
		t.Fatalf("containers keeps %+v, want its rule untouched", got)
	}
}

func TestACopyThatKeepsEverythingLeavesTheOwnFolderUnaged(t *testing.T) {
	if got := widerKeep(store.RetentionKeep{KeepLast: 3}, store.RetentionKeep{}); got != (store.RetentionKeep{}) {
		t.Fatalf("widerKeep = %+v, want a rule that keeps everything", got)
	}
}

func TestATypedOwnFolderOnAnAppendOnlyCopyStaysAppendOnly(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.target("files", "NAS", "rest:http://nas:8000/bv/files")
	f.appendOnly(nas.ID)

	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"files"}, "address": "rest:http://nas:8000/bv",
	}))

	row, ok := f.svc.primaryRemoteTarget("files")
	if !ok || !row.Immutable {
		t.Fatalf("row = %+v, want it append-only like the copy it took over", row)
	}
}

func TestTakingOverTheCopyOfASectionWhoseOwnFolderIsOffSwitchesItOn(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", boxRepo)
	f.tickDestination(box, "flash")
	if err := f.st.SetLocal("flash", false); err != nil {
		t.Fatal(err)
	}

	steps := ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"flash"}, "location": "destination:" + box.ID,
	}))
	if !steps[0].SwitchesOn {
		t.Fatalf("step = %+v, want it to say the own folder goes on", steps[0])
	}

	settings, _ := f.st.GetSettings()
	places, err := f.svc.sectionPlaces(settings, "flash", settings.FlashPath)
	if err != nil {
		t.Fatalf("flash has no place: %v", err)
	}
	if len(places) != 1 || !places[0].own() || places[0].repo != boxRepo+"/flash" {
		t.Fatalf("flash backs up to %+v, want its own folder at Box", places)
	}
}

func TestAnOwnFolderThatDoesNotAnswerIsNotCopiedAsIfEmpty(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	const old = "rest:https://old:8000/bv/containers"
	settings, _ := f.st.GetSettings()
	settings.ContainersPath = old
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.eng.opens[old] = false
	f.eng.openErr[old] = errors.New(`Fatal: unable to open config file: Head "https://old:8000/bv/containers/config": dial tcp: connection refused`)
	req := map[string]any{"domains": []string{"containers"}, "location": "destination:" + vault.ID, "copyOver": true}

	if res := f.do("POST", "/api/storage/own-folders", req); res["ok"] != false || res["code"] != "own-folder-unreadable" {
		t.Fatalf("answer = %v, want the old own folder reported unreadable", res)
	}
	if settings, _ := f.st.GetSettings(); settings.ContainersPath != old {
		t.Fatalf("path = %q, want the section left where its backups are", settings.ContainersPath)
	}

	req["copyOver"] = false
	ownSteps(t, f.do("POST", "/api/storage/own-folders", req))
	if settings, _ := f.st.GetSettings(); settings.ContainersPath != vaultRepo+"/containers" {
		t.Fatalf("path = %q, want the switch without the copy", settings.ContainersPath)
	}
}

func TestAnOwnFolderThatWasNeverCreatedMovesAtOnce(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	const old = "rest:https://old:8000/bv/containers"
	settings, _ := f.st.GetSettings()
	settings.ContainersPath = old
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.eng.opens[old] = false

	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers"}, "location": "destination:" + vault.ID, "copyOver": true,
	}))
	if settings, _ := f.st.GetSettings(); settings.ContainersPath != vaultRepo+"/containers" {
		t.Fatalf("path = %q", settings.ContainersPath)
	}
}

// copyHeld makes every copy wait until the returned func runs.
func (f *placementFixture) copyHeld() (release func()) {
	gate := make(chan struct{})
	f.eng.onCopy = func() { <-gate }
	var once sync.Once
	release = func() { once.Do(func() { close(gate) }) }
	f.t.Cleanup(release)
	return release
}

func TestAMoveOfSeveralSectionsSwitchesNoneWhileOneIsStillMoving(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:web"))
	f.hold(vaultRepo+"/vms", snap("v0", 50, "vm:old"))
	release := f.copyHeld()
	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers"}, "location": "destination:" + vault.ID, "copyOver": true,
	}))

	res := f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers", "vms"}, "location": "destination:" + vault.ID, "copyOver": true,
	})
	if res["ok"] != false || res["code"] != "move-running" {
		t.Fatalf("answer = %v, want the running move named", res)
	}
	if settings, _ := f.st.GetSettings(); settings.VMsPath != "backups/vms" {
		t.Fatalf("vms moved to %q although the request was refused", settings.VMsPath)
	}

	job := f.svc.destinationErasureJob(vault.ID)
	plan, err := job.plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var last []string
	for _, entry := range plan.LastCopies {
		last = append(last, entry.Identity)
	}
	_, err = f.svc.StartErasure(context.Background(), job, plan.Removal, last, func(store.Settings) {})
	if !errors.Is(err, errMoveRunning) {
		t.Fatalf("erasing Vault during the copy: %v, want it refused", err)
	}
	release()
	f.waitOwnFolder("containers")
}

func TestAnOwnFolderDoesNotMoveIntoALocationBeingEmptied(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	f.svc.cancelMu.Lock()
	f.svc.runCancels["erase:"+vault.ID] = func() {}
	f.svc.cancelMu.Unlock()

	res := f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"vms"}, "location": "destination:" + vault.ID,
	})
	if res["ok"] != false || res["code"] != "erase-running" {
		t.Fatalf("answer = %v, want the erasure named", res)
	}
}

func TestAPathSetWhileTheBackupsAreCopiedStays(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:web"))
	release := f.copyHeld()
	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers"}, "location": "destination:" + vault.ID, "copyOver": true,
	}))

	settings, _ := f.st.GetSettings()
	settings.ContainersPath = "elsewhere/containers"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	release()
	f.waitOwnFolder("containers")

	if settings, _ := f.st.GetSettings(); settings.ContainersPath != "elsewhere/containers" {
		t.Fatalf("path = %q, want the one set during the copy", settings.ContainersPath)
	}
}

// The read under the lock catches a backup that ran while the request was
// planned with nothing to copy.
func TestAnOwnFolderDoesNotSwitchAwayFromABackupMadeWhileItWasPlanned(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:web"))
	f.hold(vaultRepo+"/containers", snap("a1", 100, "container:web"))
	steps, err := f.svc.planOwnFolders(context.Background(), ownFolderRequest{
		Domains: []string{"containers"}, Location: "destination:" + vault.ID, CopyOver: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].Pending != 0 {
		t.Fatalf("steps = %+v, want one with nothing to copy", steps)
	}
	if err := f.svc.nothingNewToCopy(context.Background(), steps[0]); err != nil {
		t.Fatalf("nothing new: %v", err)
	}

	f.hold(f.domainPath("containers"), snap("a1", 100, "container:web"), snap("a2", 200, "container:web"))
	if err := f.svc.nothingNewToCopy(context.Background(), steps[0]); !errors.Is(err, errOwnFolderNewBackup) || !errors.Is(err, errPlacementStale) {
		t.Fatalf("err = %v, want the new backup named as a stale plan", err)
	}
}

func TestAnOwnFolderDoesNotSwitchUnderARunningBackup(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	unlock, ok := f.svc.tryLockDomainFor("vms", "backup")
	if !ok {
		t.Fatal("vms is locked already")
	}
	defer unlock()

	res := f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"vms"}, "location": "destination:" + vault.ID,
	})
	if res["ok"] != false || res["code"] != "domain-busy" {
		t.Fatalf("answer = %v, want the running backup named", res)
	}
	if settings, _ := f.st.GetSettings(); settings.VMsPath != "backups/vms" {
		t.Fatalf("path = %q", settings.VMsPath)
	}
}

func TestAnOwnFolderMoveClearsAStaleLockAtTheNewPlaceForItsCheck(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:web"))
	at := vaultRepo + "/containers"
	f.eng.checkErr = map[string]error{at: errors.New("unable to create lock in backend: repository is already locked by PID 2498 on BV-950 by root")}
	f.eng.onUnlock = func() {
		f.eng.mu.Lock()
		delete(f.eng.checkErr, at)
		f.eng.mu.Unlock()
	}

	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers"}, "location": "destination:" + vault.ID, "copyOver": true,
	}))
	f.waitOwnFolder("containers")

	if settings, _ := f.st.GetSettings(); settings.ContainersPath != at {
		t.Fatalf("path = %q, want the move done once the stale lock was cleared", settings.ContainersPath)
	}
}

func TestAnOwnFolderMoveChecksANewPlaceThatHoldsEveryBackupAlready(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:web"))
	at := vaultRepo + "/containers"
	// What a move whose check failed leaves there: every backup, unchecked.
	f.hold(at, copied("v1", "a1", 100, "container:web"))
	f.eng.checkErr = map[string]error{at: errors.New("pack 1a2b: not found in the index")}

	steps := ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers"}, "location": "destination:" + vault.ID, "copyOver": true,
	}))
	if len(steps) != 1 || steps[0].Pending != 0 || !steps[0].Verify {
		t.Fatalf("steps = %+v, want one with nothing to copy and a check before the switch", steps)
	}
	f.waitOwnFolder("containers")

	if settings, _ := f.st.GetSettings(); settings.ContainersPath == at {
		t.Fatal("the section switched onto a place whose check failed")
	}
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	if !slices.Contains(f.eng.checks, at) {
		t.Fatalf("checks = %v, want the new place checked", f.eng.checks)
	}
}

func TestMovingTheOwnFolderOfASectionThatBacksUpStraightToACopySendsNewBackupsThere(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	mani := f.target("containers", "Mani", "offsite/mani/containers")
	direct := f.direct(mani)
	f.setDefault("containers", direct.ID, store.SkipAll)
	f.container("web", direct.ID)
	req := map[string]any{"domains": []string{"containers"}, "location": "destination:" + vault.ID}

	if steps := ownSteps(t, f.do("POST", "/api/storage/own-folders/preview", req)); steps[0].WritesTo != "Mani" {
		t.Fatalf("step = %+v, want it to say new backups go straight to Mani now", steps[0])
	}
	ownSteps(t, f.do("POST", "/api/storage/own-folders", req))

	d, _, err := f.st.PlacementDefaultFor("containers")
	if err != nil {
		t.Fatal(err)
	}
	if d.Home != "" || skipsTarget(d.Skip, mani.ID) {
		t.Fatalf("default = %+v, want the own folder as its home and a copy at Mani", d)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := f.svc.settleHome(context.Background(), settings, store.ItemRef{Domain: "containers", Key: "web"})
	if err != nil || repo != "" {
		t.Fatalf("web backs up to %q (%v), want the new own folder", repo, err)
	}
}

func TestMovingAnOwnFolderKeepsADefaultOnANamedRepository(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	cold := f.namedRepo("Cold", "cold/containers")
	f.setDefault("containers", cold.ID)

	steps := ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers"}, "location": "destination:" + vault.ID,
	}))

	if steps[0].WritesTo != "" {
		t.Fatalf("step = %+v, want no word about a copy", steps[0])
	}
	if d, _, _ := f.st.PlacementDefaultFor("containers"); d.Home != cold.ID {
		t.Fatalf("default home = %q, want Cold", d.Home)
	}
}

// ownFolderOff reads whether the storage pages list a section's own folder as
// taking no new backups.
func (f *placementFixture) ownFolderOff(domain string) bool {
	f.t.Helper()
	locations, err := f.svc.StorageLocations()
	if err != nil {
		f.t.Fatal(err)
	}
	for _, l := range locations {
		for _, s := range l.Sections {
			if s.Domain == domain && s.Use == useHome && s.RepoID == "" && s.TargetID == "" {
				return s.Off
			}
		}
	}
	f.t.Fatalf("no location lists the own folder of %s", domain)
	return false
}

func TestAnOwnFolderThatNewBackupsGoPastIsListedAsOff(t *testing.T) {
	f := newPlacementFixture(t)
	mani := f.target("containers", "Mani", "offsite/mani/containers")
	direct := f.direct(mani)
	f.setDefault("containers", direct.ID, store.SkipAll)
	f.container("web", direct.ID)

	if !f.ownFolderOff("containers") {
		t.Fatal("the own folder is listed as in use although every new backup goes straight to Mani")
	}
	f.container("db", "")
	if f.ownFolderOff("containers") {
		t.Fatal("the own folder is listed as off although db keeps it as its home")
	}
}

func TestMovingAnOwnFolderClearsAnOffsiteFieldThatNamesItsNewPlace(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersOffsite = vaultRepo + "/containers"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	copyThere := f.tickDestination(vault, "containers")
	if copyThere.SortOrder == 0 {
		t.Fatalf("the ticked copy took the primary slot: %+v", copyThere)
	}

	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers"}, "location": "destination:" + vault.ID,
	}))

	if s, _ := f.st.GetSettings(); s.ContainersOffsite != "" {
		t.Fatalf("off-site field = %q, want it cleared with the own folder there", s.ContainersOffsite)
	}
	if faults, err := f.svc.locationFaults(); err != nil || len(faults) != 0 {
		t.Fatalf("faults = %+v, %v", faults, err)
	}
}
