package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/selfrestore"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const wizardRepo = "rest:http://box:8000/bv"

// kitWithWizardLocation is the kit of a fresh install with one location made
// with Add storage location, whose sign-in is therefore not the default.
func kitWithWizardLocation(t *testing.T) string {
	t.Helper()
	old := newPlacementFixture(t)
	old.svc.cfg.AppKey = oldKey
	old.svc.forgetBackupKey()
	if _, err := old.st.MutateSettings(func(s *store.Settings) error {
		s.EncryptionEnabled, s.ContainersPath, s.VMsPath, s.FilesPath = true, "dead/c", "dead/v", "dead/f"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	d, err := old.svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "rest", Settings: map[string]string{"url": "http://box:8000/bv", "user": "boxuser", "pass": "box-secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, domain := range []string{"files", "config"} {
		if _, _, err := old.st.EnsureDestinationTarget(d.ID, domain, destinationLocation(d.Repo, domain)); err != nil {
			t.Fatal(err)
		}
	}
	text, err := old.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestAKitCarriesTheSignInOfALocationMadeWithTheWizard(t *testing.T) {
	text := kitWithWizardLocation(t)
	if strings.Contains(text, "No off-site/cloud credentials are stored") {
		t.Error("the kit says it carries no sign-in")
	}
	kit, err := readRecoveryKit(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(kit.signIns) != 1 || kit.signIns[0].creds.RESTUser != "boxuser" || kit.signIns[0].creds.RESTPassword != "box-secret" {
		t.Fatalf("the kit read back as %+v", kit.signIns)
	}
	if !kit.signIns[0].holds(wizardRepo+"/files") || kit.signIns[0].holds("rest:http://box:8000/bv2") {
		t.Fatalf("the sign-in says it reaches %v", kit.signIns[0].repos)
	}
}

func TestANewServerOpensALocationWithTheSignInFromTheKit(t *testing.T) {
	text := kitWithWizardLocation(t)
	f := newPlacementFixture(t)
	f.eng.opens[wizardRepo+"/files"] = true
	f.eng.opens[wizardRepo+"/config"] = true
	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": text})
	if !view.Session.CloudSignIn {
		t.Error("the page is not told that the kit carries a sign-in")
	}
	readable := 0
	for _, p := range view.Session.Places {
		if p.State == placeReadable {
			readable++
		}
	}
	if readable == 0 || view.Session.Refusal != "" {
		t.Fatalf("readable=%d refusal=%q places=%+v", readable, view.Session.Refusal, view.Session.Places)
	}

	if res, _ := f.emergency("POST", "/api/emergency/sign-ins", nil); res["ok"] != true {
		t.Fatalf("sign-ins: %v", res)
	}
	sets, err := f.svc.CloudCredSets()
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0].RESTUser != "boxuser" || sets[0].Default {
		t.Fatalf("the saved sign-ins: %+v", sets)
	}
}

func TestSigningInByHandOpensThePlacesOfAKitThatCarriesNone(t *testing.T) {
	f := newPlacementFixture(t)
	f.eng.opens["rclone:box:files"] = true
	f.eng.opens["rest:http://box:8000/files"] = true
	key := strings.Repeat("e", 64)
	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{
		"key": key, "places": []string{"rclone:box:files", "rest:http://box:8000/files"},
	})
	if view.Session.Refusal != "adopt-nothing-readable" {
		t.Fatalf("before the sign-in: %+v", view.Session)
	}
	if err := f.svc.SetRcloneConf("[box]\ntype = sftp"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetCloudCreds(CloudCreds{RESTUser: "bv", RESTPassword: "typed"}); err != nil {
		t.Fatal(err)
	}
	res, view := f.emergency("POST", "/api/emergency/try-again", map[string]any{"session": view.Session.ID})
	if res["ok"] != true {
		t.Fatalf("try again: %v", res)
	}
	for _, p := range view.Session.Places {
		if p.State != placeReadable {
			t.Errorf("%s reads %s", p.Location, p.State)
		}
	}
	if view.Session.Refusal != "" {
		t.Errorf("the key still cannot be taken over: %q", view.Session.Refusal)
	}
}

func TestARefusedSignInReadsAsOneToEnterAgain(t *testing.T) {
	f := newPlacementFixture(t)
	f.eng.opens["rest:http://box:8000/files"] = false
	f.eng.openErr["rest:http://box:8000/files"] = errors.New("restic cat failed: Fatal: unexpected HTTP response (401): 401 Unauthorized")
	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{
		"key": strings.Repeat("e", 64), "places": []string{"rest:http://box:8000/files"},
		"creds": map[string]any{"restUser": "stale", "restPassword": "stale"},
	})
	if got := view.Session.Places[0].State; got != placeNeedsSignIn {
		t.Fatalf("a refused sign-in reads %q", got)
	}
}

func TestAttachingTheKitGivesTheCopiesTheirSignIn(t *testing.T) {
	text := kitWithWizardLocation(t)
	f := newPlacementFixture(t)
	f.eng.opens[wizardRepo+"/files"] = true
	f.eng.opens[wizardRepo+"/config"] = true
	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": text})
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["ok"] != true {
		t.Fatalf("adopt: %v", res)
	}
	if res, _ := f.emergency("POST", "/api/emergency/attach", map[string]any{"session": view.Session.ID}); res["ok"] != true {
		t.Fatalf("attach: %v", res)
	}
	targets, err := f.st.OffsiteTargetsForDomain("files")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Repo != wizardRepo+"/files" || targets[0].CredsRef == "" || !targets[0].Enabled {
		t.Fatalf("the folder sets' targets: %+v", targets)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	mode := f.svc.offsiteModeForTarget(settings, targets[0])
	if !strings.Contains(strings.Join(mode.Env, " "), "RESTIC_REST_USERNAME=boxuser") {
		t.Fatalf("the target signs in with %v", mode.Env)
	}

	if err := f.svc.UndoAdoption(); err != nil {
		t.Fatal(err)
	}
	targets, err = f.st.OffsiteTargetsForDomain("files")
	if err != nil || len(targets) != 0 {
		t.Fatalf("after the undo: %+v, %v", targets, err)
	}
	sets, err := f.svc.CloudCredSets()
	if err != nil || len(sets) != 0 {
		t.Fatalf("after the undo the saved sign-ins are %+v, %v", sets, err)
	}
}

func TestADeadOffsiteCopyDoesNotHideTheOtherOne(t *testing.T) {
	f := newPlacementFixture(t)
	d1 := f.destination("House", "rest:http://dead:8000/bv")
	d2 := f.destination("Cloud", "s3:https://s3.example/bucket/bv")
	f.tickDestination(d1, "files")
	second := f.tickDestination(d2, "files").ID
	f.eng.listErr[d1.Repo+"/files"] = errors.New("connection refused")
	f.eng.snaps[d2.Repo+"/files"] = []restic.Snapshot{{ID: "bbbbbbbb22222222", Time: "2026-11-01T03:00:00Z", Tags: []string{"fileset:photos"}}}

	got, err := f.svc.EmergencyOffsiteEntries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.FileSets["photos"] == 0 {
		t.Fatalf("the healthy copy was not read: %+v", got)
	}
	if got.From.FileSets["photos"] != offsiteSourcePrefix+second {
		t.Errorf("photos is read from %q", got.From.FileSets["photos"])
	}
	if !strings.Contains(got.Unread["files"], "House") {
		t.Errorf("the dead copy is not named: %v", got.Unread)
	}
}

func TestAFolderSetOnlyACopyHoldsCanBeRestored(t *testing.T) {
	f := newPlacementFixture(t)
	d := f.destination("Cloud", "s3:https://s3.example/bucket/bv")
	f.tickDestination(d, "files")
	f.eng.snaps[d.Repo+"/files"] = []restic.Snapshot{{ID: "bbbbbbbb22222222", Time: "2026-11-01T03:00:00Z", Tags: []string{"fileset:photos"}, Paths: []string{"/host/user/photos"}}}
	res := f.do("POST", "/api/emergency/offsite-entries", map[string]any{})
	if res["ok"] != true {
		t.Fatalf("entries: %v", res)
	}
	set, err := f.st.GetFileSetByName("photos")
	if err != nil {
		t.Fatalf("the set has no row: %v", err)
	}
	if set.Enabled || set.Path != "" {
		t.Errorf("an adopted set starts off and without a path: %+v", set)
	}
}

func TestAnOffsiteCopyOpensUnderThisServersKey(t *testing.T) {
	f := newPlacementFixture(t)
	d := f.destination("Cloud", "s3:https://s3.example/bucket/bv")
	id := f.tickDestination(d, "containers").ID
	session, err := f.svc.OpenOffsiteCopy(context.Background(), "containers", offsiteSourcePrefix+id)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := f.svc.foreignSession(session)
	if err != nil {
		t.Fatal(err)
	}
	if sess.repo != d.Repo+"/containers" || !sess.mode.NoLock {
		t.Fatalf("the session: %+v", sess)
	}
	if _, err := f.svc.foreignContainerTarget(sess, "plex"); err == nil {
		t.Error("an off-site copy has no definition to recreate from")
	}
}

func TestAFetchedCopyOpensUnderThisServersKey(t *testing.T) {
	f := newPlacementFixture(t)
	source, err := f.st.CreatePullSource(store.PullSource{Name: "Tower", Repo: "rest:http://tower:8000/files", Domain: "files", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.OpenPulledCopy(context.Background(), source.ID); !errors.Is(err, errNothingFetched) {
		t.Fatalf("before a fetch: %v", err)
	}
	_, domainRepo, err := f.svc.domainRepoSource("files", "local")
	if err != nil {
		t.Fatal(err)
	}
	repo := pulledRepo(domainRepo, source.ID)
	f.makeRepo(repo)
	f.eng.snaps[filepath.ToSlash(repo)] = []restic.Snapshot{{ID: "aaaaaaaa11111111", Time: "2026-11-01T03:00:00Z", Tags: []string{"fileset:plexdata"}, Paths: []string{"/host/user/plex"}}}
	session, inv, err := f.svc.OpenPulledCopy(context.Background(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if session == "" || len(inv.FileSets) != 1 || inv.FileSets[0].Name != "plexdata" {
		t.Fatalf("session %q, inventory %+v", session, inv)
	}
}

func TestDiscoverRebuildsAContainerThatHasOnlyItsDefinition(t *testing.T) {
	f := newPlacementFixture(t)
	if _, err := f.st.MutateSettings(func(s *store.Settings) error { s.ContainersPath = "backups/containers"; return nil }); err != nil {
		t.Fatal(err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	repo, err := f.svc.containersRepoPath(settings)
	if err != nil {
		t.Fatal(err)
	}
	f.makeRepo(repo)
	def := []byte(`{"appdata_paths":[],"inspect":{"Config":{"Labels":{"com.docker.compose.project":"site"}}}}`)
	if err := f.svc.writeDefToStorage(settings, "site-web-1", repo, def, nil); err != nil {
		t.Fatal(err)
	}

	probe, err := f.svc.Discover(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if probe.Found != 1 {
		t.Fatalf("the probe found %d", probe.Found)
	}
	if _, err := f.st.GetTargetByContainer("site-web-1"); err == nil {
		t.Fatal("the probe wrote a row")
	}

	res, err := f.svc.Discover(context.Background(), false)
	if err != nil || res.Found != 1 {
		t.Fatalf("found %d, %v", res.Found, err)
	}
	row, err := f.st.GetTargetByContainer("site-web-1")
	if err != nil {
		t.Fatalf("no row: %v", err)
	}
	if row.Definition == "" || row.RepoChosen != store.RepoOpen {
		t.Fatalf("the row: %+v", row)
	}
	members, err := f.svc.prepareRestoreStack("site", "local", true)
	if err != nil || len(members) != 1 {
		t.Fatalf("the stack finds %+v, %v", members, err)
	}
}

func TestDiscoverDoesNotBringBackAFormerName(t *testing.T) {
	f := newPlacementFixture(t)
	if _, err := f.st.MutateSettings(func(s *store.Settings) error { s.ContainersPath = "backups/containers"; return nil }); err != nil {
		t.Fatal(err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	repo, err := f.svc.containersRepoPath(settings)
	if err != nil {
		t.Fatal(err)
	}
	f.makeRepo(repo)
	if err := f.svc.writeDefToStorage(settings, "old", repo, []byte(`{}`), nil); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.writeDefToStorage(settings, "new", repo, []byte(`{}`), []store.Alias{{OldName: "old", LinkedAt: 1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Discover(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.GetTargetByContainer("old"); err == nil {
		t.Error("a former name came back as a container of its own")
	}
	if _, err := f.st.GetTargetByContainer("new"); err != nil {
		t.Errorf("the container itself did not: %v", err)
	}
}

// metadataOnlyEngine restores as restic does and then answers as it does on a
// share that refuses only the ownership.
type metadataOnlyEngine struct{ ResticEngine }

func (e metadataOnlyEngine) RestoreSubtreeTo(ctx context.Context, repo, id, subtree, target string, mode restic.Mode) error {
	if err := e.ResticEngine.RestoreSubtreeTo(ctx, repo, id, subtree, target, mode); err != nil {
		return err
	}
	return fmt.Errorf("restic restore failed: %w", restic.ErrRestoreMetadataOnly)
}

func TestAContainerBesideTheOriginalSurvivesAShareThatRefusesTheOwnership(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedPlex(t)
	r.svc.engine = metadataOnlyEngine{r.svc.engine}
	if started, err := r.svc.StartRestore(context.Background(), "plex", snap, "local", RestoreOptions{Beside: true}); err != nil || !started {
		t.Fatalf("StartRestore: started=%v err=%v", started, err)
	}
	r.wait(t)
	run := r.lastRun(t, tg.ID, "restore")
	if run.Status != "success" || !strings.Contains(run.Error, restic.RestoreMetadataWarning) {
		t.Fatalf("run = %+v", run)
	}
	if _, err := os.Stat(filepath.Join(r.root, "appdata", "plex"+besideSuffix)); err != nil {
		t.Fatalf("the finished copy is gone: %v", err)
	}
	if _, made := r.docker.container("plex-restored"); !made {
		t.Fatal("the copy was not created")
	}
}

func TestAnErrorThatIsMoreThanTheOwnershipStillFailsTheRestore(t *testing.T) {
	refused := new(atomic.Bool)
	a := &resticAdapter{metadataRefused: refused}
	cause := errors.New("no space left on device")
	if err := a.restored(cause); !errors.Is(err, cause) || refused.Load() {
		t.Fatalf("err=%v refused=%v", err, refused.Load())
	}
	if err := a.restored(fmt.Errorf("x: %w", restic.ErrRestoreMetadataOnly)); err != nil || !refused.Load() {
		t.Fatalf("err=%v refused=%v", err, refused.Load())
	}
}

// otherDataFolderEngine restores the way restic does: the snapshot's absolute
// path comes back below the target.
type otherDataFolderEngine struct {
	ResticEngine
	include string
}

func (e *otherDataFolderEngine) RestoreInclude(_ context.Context, _, _, include, target string, _ restic.Mode) error {
	e.include = include
	dir := filepath.Join(target, filepath.FromSlash(include))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "bombvault.sqlite"), []byte("x"), 0o600)
}

func TestASelfBackupFromAnotherDataFolderLandsWhereTheBootLooks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the staging path nests the absolute data folder, which Windows refuses")
	}
	f := newPlacementFixture(t)
	eng := &otherDataFolderEngine{ResticEngine: f.svc.engine}
	f.svc.engine = eng
	root := selfrestore.StagingRoot(f.svc.cfg.DataDir)
	snaps := []restic.Snapshot{{ID: "aaaaaaaa11111111", Paths: []string{"/appconf/.snapshot"}}}
	if err := f.svc.stageConfigRestore(context.Background(), "repo", "aaaaaaaa11111111", snaps, root, restic.Mode{}); err != nil {
		t.Fatal(err)
	}
	if eng.include != "/appconf/.snapshot" {
		t.Errorf("restored %q", eng.include)
	}
	if _, err := os.Stat(filepath.Join(selfrestore.RestoredSnapshotDir(f.svc.cfg.DataDir), "bombvault.sqlite")); err != nil {
		t.Fatalf("the boot would find nothing: %v", err)
	}
}

func TestARemoteOwnFolderMirrorsNoDefinition(t *testing.T) {
	f := newPlacementFixture(t)
	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.ContainersPath = "s3:https://s3.example/bucket/containers"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if err := f.svc.writeDefToStorage(settings, "nextcloud", settings.ContainersPath, []byte(`{}`), nil); err != nil {
		t.Fatal(err)
	}
	left, err := os.ReadDir(".")
	if err != nil || len(left) != 0 {
		t.Fatalf("the mirror wrote %v below the working folder, %v", left, err)
	}
}

func TestASettingsRestoreOfNothingFailsAndLeavesNoMarker(t *testing.T) {
	f := newPlacementFixture(t)
	err := f.svc.takeOverStagedSettings(context.Background())
	if !errors.Is(err, errNothingStaged) {
		t.Fatalf("without a staged database: %v", err)
	}
}

func TestTheSelfBackupCarriesTheReplicaKeyAndTheCertificates(t *testing.T) {
	f := newPlacementFixture(t)
	data := f.svc.cfg.DataDir
	for _, p := range []string{"ssh/id_ed25519", "ssh-replica/id_ed25519", "ssh-replica/hosts/x/known_hosts", "certs/cert.pem"} {
		full := filepath.Join(data, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dir, err := f.svc.stageConfigSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"ssh/id_ed25519", "ssh-replica/id_ed25519", "ssh-replica/hosts/x/known_hosts", "certs/cert.pem"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			t.Errorf("%s is not in the self-backup: %v", p, err)
		}
	}
	if !strings.Contains(strings.Join(selfrestore.StagedDirs, " "), "ssh-replica") {
		t.Error("the boot swap would not bring the replica key back")
	}
}
