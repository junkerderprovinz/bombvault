package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// signInEngine opens a repository when answer says so, by its location and
// the sign-in the probe carries, and records every location it was asked for.
type signInEngine struct {
	ResticEngine
	answer func(repo string, env []string) error
	probed []string
}

func (e *signInEngine) RepoOpensErr(_ context.Context, repo string, mode restic.Mode) error {
	e.probed = append(e.probed, repo)
	return e.answer(repo, mode.Env)
}

// noRepository is restic's answer at a place that is reached and holds no
// repository.
type noRepository struct{}

func (noRepository) Error() string { return "Fatal: repository does not exist" }

// opensWithPassword opens every repository for a sign-in with the given
// rest-server password and refuses any other.
func opensWithPassword(password string) func(string, []string) error {
	return func(repo string, env []string) error {
		if slices.Contains(env, "RESTIC_REST_PASSWORD="+password) {
			return nil
		}
		return &probeFailure{repo: repo}
	}
}

func newConnectionSvc(t *testing.T, answer func(string, []string) error) (*Service, *store.Repo, *signInEngine) {
	t.Helper()
	svc, st, _ := newProbeSvc(t, nil)
	svc.cfg.DataDir = t.TempDir()
	eng := &signInEngine{answer: answer}
	svc.engine = eng
	return svc, st, eng
}

// restDestination saves a rest-server destination signed into as bv with the
// password "old" and gives each domain its target under it.
func restDestination(t *testing.T, svc *Service, st *store.Repo, domains ...string) (store.OffsiteTarget, []store.OffsiteTarget) {
	t.Helper()
	d, err := svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "rest", Name: "Box", Settings: map[string]string{"url": "https://box:8000/bv", "user": "bv", "pass": "old"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var held []store.OffsiteTarget
	for _, domain := range domains {
		target, _, err := st.EnsureDestinationTarget(d.ID, domain, destinationLocation(d.Repo, domain))
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, target)
	}
	return d, held
}

func copyArrivedAt(t *testing.T, st *store.Repo, target store.OffsiteTarget) {
	t.Helper()
	id, err := st.RecordOffsiteRunForTarget(target.Domain, target.ID, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishOffsiteRun(id, true, ""); err != nil {
		t.Fatal(err)
	}
}

func signInOf(t *testing.T, svc *Service, d store.OffsiteTarget) CloudCreds {
	t.Helper()
	settings, err := svc.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	creds, err := svc.decodeCloudFor(settings, d.CredsRef)
	if err != nil {
		t.Fatal(err)
	}
	return creds
}

func folder(s string) *string { return &s }

func TestANewPasswordIsSavedOnceEveryHeldRepositoryOpensWithIt(t *testing.T) {
	svc, st, eng := newConnectionSvc(t, opensWithPassword("new"))
	d, _ := restDestination(t, svc, st, "vms", "flash")

	saved, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Settings: map[string]string{"pass": "new"}})
	if err != nil {
		t.Fatal(err)
	}

	if got := signInOf(t, svc, saved); got.RESTPassword != "new" || got.RESTUser != "bv" {
		t.Fatalf("sign-in = %+v, want the new password and the user it had", got)
	}
	if want := []string{"rest:https://box:8000/bv/flash", "rest:https://box:8000/bv/vms"}; !slices.Equal(eng.probed, want) {
		t.Fatalf("probed %v, want %v", eng.probed, want)
	}
}

func TestAPasswordThatDoesNotOpenTheHeldRepositoriesIsNotSaved(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, opensWithPassword("old"))
	d, _ := restDestination(t, svc, st, "vms")

	_, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Settings: map[string]string{"pass": "typo"}})

	if !errors.Is(err, errBackupsNotThere) {
		t.Fatalf("err = %v, want the refusal that the backups do not open", err)
	}
	if got := signInOf(t, svc, d); got.RESTPassword != "old" {
		t.Fatalf("the stored password is %q after a refused change", got.RESTPassword)
	}
}

func TestADestinationThatHoldsBackupsKeepsItsAddress(t *testing.T) {
	svc, st, eng := newConnectionSvc(t, opensWithPassword("old"))
	d, _ := restDestination(t, svc, st, "vms")

	_, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Settings: map[string]string{"url": "https://other:8000/bv"}})

	if !errors.Is(err, errDestinationHoldsBackups) {
		t.Fatalf("err = %v, want the refusal to move", err)
	}
	if len(eng.probed) != 0 {
		t.Fatalf("a refused move still probed %v", eng.probed)
	}
	if got, _, _ := st.GetDestination(d.ID); got.Repo != d.Repo {
		t.Fatalf("location = %q, want it unchanged", got.Repo)
	}
}

func TestARepositoryThatReceivedACopyHasToBeThereAfterTheChange(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, func(string, []string) error { return noRepository{} })
	d, held := restDestination(t, svc, st, "vms")
	edit := connectionEdit{Settings: map[string]string{"user": "other"}}

	if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, edit); err != nil {
		t.Fatalf("a target nothing was copied to yet stopped the change: %v", err)
	}

	copyArrivedAt(t, st, held[0])
	if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, edit); !errors.Is(err, errBackupsNotThere) {
		t.Fatalf("err = %v, want a refusal once a copy has arrived there", err)
	}
}

func TestADestinationNothingCopiesToMovesAfterItsNewAddressAnswers(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, func(repo string, _ []string) error {
		if repo == "rest:https://new:8000/bv" {
			return noRepository{}
		}
		return &probeFailure{repo: repo}
	})
	d, _ := restDestination(t, svc, st)

	if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Settings: map[string]string{"url": "https://dead:8000/bv"}}); err == nil {
		t.Fatal("an address that does not answer was saved")
	}
	saved, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Settings: map[string]string{"url": "https://new:8000/bv/"}})
	if err != nil {
		t.Fatal(err)
	}

	if saved.Repo != "rest:https://new:8000/bv" {
		t.Fatalf("location = %q, want the new address", saved.Repo)
	}
}

func TestAnS3DestinationThatHoldsBackupsTakesANewKeyAndKeepsItsBucket(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, func(repo string, env []string) error {
		if slices.Contains(env, "AWS_SECRET_ACCESS_KEY=rotated") && slices.Contains(env, "AWS_ACCESS_KEY_ID=AKID2") {
			return nil
		}
		return &probeFailure{repo: repo}
	})
	d, err := svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "wasabi", Dir: "bucket/bv",
		Settings: map[string]string{"endpoint": "s3.eu-central-2.wasabisys.com", "access_key_id": "AKID", "secret_access_key": "SECRET", "region": "eu-central-2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.EnsureDestinationTarget(d.ID, "files", destinationLocation(d.Repo, "files")); err != nil {
		t.Fatal(err)
	}

	saved, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{
		Settings: map[string]string{"access_key_id": "AKID2", "secret_access_key": "rotated"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := signInOf(t, svc, saved); got.S3KeyID != "AKID2" || got.S3Secret != "rotated" || got.S3Region != "eu-central-2" {
		t.Fatalf("sign-in = %+v, want the new key with the region it had", got)
	}
	if saved.Repo != d.Repo {
		t.Fatalf("location = %q, want %q", saved.Repo, d.Repo)
	}

	for name, edit := range map[string]connectionEdit{
		"another bucket":   {Dir: folder("other-bucket/bv")},
		"another endpoint": {Settings: map[string]string{"endpoint": "s3.us-east-1.wasabisys.com"}},
	} {
		if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, edit); !errors.Is(err, errDestinationHoldsBackups) {
			t.Errorf("%s: err = %v, want the refusal to move", name, err)
		}
	}
}

func TestAFolderDestinationMovesOnlyWhileNothingCopiesToIt(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, nil)
	for _, folder := range []string{"remotes/NAS2", "remotes/NAS3"} {
		if err := os.MkdirAll(filepath.Join(svc.cfg.HostMountRoot, folder), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	d, err := svc.SaveNewDestination(context.Background(), draftRequest{Provider: "path", Settings: map[string]string{"path": "remotes/NAS2"}})
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Settings: map[string]string{"path": "remotes/missing"}}); err == nil {
		t.Fatal("a folder that is not there was saved")
	}
	saved, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Settings: map[string]string{"path": "remotes/NAS3/"}})
	if err != nil || saved.Repo != "remotes/NAS3" {
		t.Fatalf("saved %q, %v, want the new folder", saved.Repo, err)
	}

	if _, _, err := st.EnsureDestinationTarget(d.ID, "flash", destinationLocation(saved.Repo, "flash")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Settings: map[string]string{"path": "remotes/NAS2"}}); !errors.Is(err, errDestinationHoldsBackups) {
		t.Fatalf("err = %v, want the refusal to move", err)
	}
}

// smbDestination saves an SMB destination through the fake rclone and returns
// it with the name of its remote.
func smbDestination(t *testing.T, svc *Service) (store.OffsiteTarget, string) {
	t.Helper()
	useFakeRclone(t, svc)
	d, err := svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "smb", Name: "NAS", Dir: "backups/bv",
		Settings: map[string]string{"host": "nas.lan", "user": "bv", "pass": "old", "port": "4450"},
	})
	if err != nil {
		t.Fatal(err)
	}
	remote, _, _ := strings.Cut(strings.TrimPrefix(d.Repo, "rclone:"), ":")
	return d, remote
}

func storedRemote(t *testing.T, svc *Service, remote string) (map[string]string, bool) {
	t.Helper()
	settings, err := svc.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	conf, err := svc.decodeRcloneConf(settings)
	if err != nil {
		t.Fatal(err)
	}
	return remoteOptions(conf, remote)
}

func TestARemoteThatHoldsBackupsIsTriedUnderAnotherNameBeforeItIsChanged(t *testing.T) {
	svc, st, eng := newConnectionSvc(t, func(repo string, _ []string) error {
		if strings.HasPrefix(repo, "rclone:smb-check:") {
			return nil
		}
		return &probeFailure{repo: repo}
	})
	d, remote := smbDestination(t, svc)
	if _, _, err := st.EnsureDestinationTarget(d.ID, "vms", destinationLocation(d.Repo, "vms")); err != nil {
		t.Fatal(err)
	}

	_, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{
		Settings: map[string]string{"pass": "new", "port": "", "host": ""},
	})
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"rclone:smb-check:backups/bv/vms"}; !slices.Equal(eng.probed, want) {
		t.Fatalf("probed %v, want %v", eng.probed, want)
	}
	got, _ := storedRemote(t, svc, remote)
	want := map[string]string{"host": "nas.lan", "user": "bv", "pass": "obscured-new"}
	if len(got) != len(want) || got["host"] != want["host"] || got["user"] != want["user"] || got["pass"] != want["pass"] {
		t.Fatalf("remote = %v, want %v: the new password, the port gone, the host it does not show kept", got, want)
	}
	if _, left := storedRemote(t, svc, "smb-check"); left {
		t.Fatal("the remote it was tried under is still in the config")
	}
}

func TestARemoteWhoseChangeDoesNotOpenTheHeldRepositoriesStaysAsItWas(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, func(repo string, _ []string) error { return &probeFailure{repo: repo} })
	d, remote := smbDestination(t, svc)
	if _, _, err := st.EnsureDestinationTarget(d.ID, "vms", destinationLocation(d.Repo, "vms")); err != nil {
		t.Fatal(err)
	}

	_, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Settings: map[string]string{"pass": "typo"}})
	if !errors.Is(err, errBackupsNotThere) {
		t.Fatalf("err = %v, want the refusal that the backups do not open", err)
	}
	if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Dir: folder("elsewhere")}); !errors.Is(err, errDestinationHoldsBackups) {
		t.Fatalf("err = %v, want the refusal to move", err)
	}

	if got, _ := storedRemote(t, svc, remote); got["pass"] != "obscured-old" || got["port"] != "4450" {
		t.Fatalf("remote = %v, want it as it was saved", got)
	}
	if _, left := storedRemote(t, svc, "smb-check"); left {
		t.Fatal("the remote it was tried under is still in the config")
	}
}

func TestARemoteNothingCopiesToIsListedBeforeItIsChanged(t *testing.T) {
	svc, _, _ := newConnectionSvc(t, nil)
	d, remote := smbDestination(t, svc)

	if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Settings: map[string]string{"host": "down"}}); err == nil {
		t.Fatal("a host that refuses the listing was saved")
	}
	if got, _ := storedRemote(t, svc, remote); got["host"] != "nas.lan" {
		t.Fatalf("host = %q after a refused change", got["host"])
	}

	saved, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{
		Settings: map[string]string{"host": "nas2.lan"}, Dir: folder("/backups/second/"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := storedRemote(t, svc, remote); got["host"] != "nas2.lan" || got["pass"] != "obscured-old" {
		t.Fatalf("remote = %v, want the new host and the password it had", got)
	}
	if saved.Repo != "rclone:"+remote+":backups/second" {
		t.Fatalf("location = %q, want the new folder on the same remote", saved.Repo)
	}
}

func TestTheConnectionRouteAnswersARefusedMoveWithItsCode(t *testing.T) {
	f := newPlacementFixture(t)
	d, err := f.svc.SaveNewDestination(context.Background(), draftRequest{Provider: "path", Settings: map[string]string{"path": "offsite"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.st.EnsureDestinationTarget(d.ID, "flash", destinationLocation(d.Repo, "flash")); err != nil {
		t.Fatal(err)
	}

	res := f.do("PUT", "/api/offsite/destinations/"+d.ID+"/connection", map[string]any{"settings": map[string]string{"path": "elsewhere"}})

	if res["ok"] != false || res["code"] != "destination-holds-backups" {
		t.Fatalf("answer = %v, want the refusal with its code", res)
	}

	res = f.do("PUT", "/api/offsite/destinations/"+d.ID+"/connection", map[string]any{"settings": map[string]string{}})
	if res["ok"] != true || res["destination"].(map[string]any)["id"] != d.ID {
		t.Fatalf("answer = %v, want the destination back", res)
	}
}

func TestAConnectionEditCannotMoveADestinationOntoAHomeOrAnotherDestination(t *testing.T) {
	f := newPlacementFixture(t)
	ctx := context.Background()
	folder, err := f.svc.SaveNewDestination(ctx, draftRequest{Provider: "path", Settings: map[string]string{"path": "offsite"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.EditDestinationConnection(ctx, folder.ID, connectionEdit{Settings: map[string]string{"path": "backups"}}); !errors.Is(err, errNestedLocation) {
		t.Fatalf("moving a folder destination around the repositories at home = %v, want errNestedLocation", err)
	}

	if _, err := f.svc.SaveNewDestination(ctx, draftRequest{Provider: "rest", Name: "Box", Settings: map[string]string{"url": "https://box:8000/bv", "user": "bv", "pass": "one"}}); err != nil {
		t.Fatal(err)
	}
	other, err := f.svc.SaveNewDestination(ctx, draftRequest{Provider: "rest", Name: "Other", Settings: map[string]string{"url": "https://other:8000/bv", "user": "bv", "pass": "two"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.EditDestinationConnection(ctx, other.ID, connectionEdit{Settings: map[string]string{"url": "https://box:8000/bv/inner"}}); !errors.Is(err, errNestedLocation) {
		t.Fatalf("moving a rest-server destination inside another = %v, want errNestedLocation", err)
	}
	if got, _, _ := f.st.GetDestination(other.ID); got.Repo != other.Repo {
		t.Fatalf("the refused edit moved the destination to %q", got.Repo)
	}
}

// secondRestDestination saves another rest-server destination that signs in
// with the saved sign-in ref.
func secondRestDestination(t *testing.T, svc *Service, ref string) store.OffsiteTarget {
	t.Helper()
	d, err := svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "rest", Name: "Other box", CredsRef: ref, Settings: map[string]string{"url": "https://other:8000/bv"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestANewPasswordForASharedSignInStaysWithTheDestinationItWasTypedFor(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, opensWithPassword("new"))
	a, _ := restDestination(t, svc, st, "vms")
	b := secondRestDestination(t, svc, a.CredsRef)

	saved, _, err := svc.EditDestinationConnection(context.Background(), a.ID, connectionEdit{Settings: map[string]string{"pass": "new"}})
	if err != nil {
		t.Fatal(err)
	}

	if saved.CredsRef == a.CredsRef {
		t.Fatal("the changed destination still names the sign-in it shares")
	}
	if got := signInOf(t, svc, saved); got.RESTPassword != "new" || got.RESTUser != "bv" {
		t.Fatalf("changed destination signs in with %+v, want the new password", got)
	}
	if got := signInOf(t, svc, b); got.RESTPassword != "old" {
		t.Fatalf("the other destination signs in with %q, want the password nothing tested away", got.RESTPassword)
	}
	targets, err := st.DestinationTargets(a.ID)
	if err != nil || len(targets) != 1 || targets[0].CredsRef != saved.CredsRef {
		t.Fatalf("targets = %+v, %v, want the vms target on the new sign-in", targets, err)
	}
}

func TestAPasswordForTheDefaultSignInLeavesTheDefaultAsItWas(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, opensWithPassword("new"))
	a, _ := restDestination(t, svc, st, "vms")
	if err := svc.MakeDefaultSignIn(a.CredsRef); err != nil {
		t.Fatal(err)
	}

	saved, _, err := svc.EditDestinationConnection(context.Background(), a.ID, connectionEdit{Settings: map[string]string{"pass": "new"}})
	if err != nil {
		t.Fatal(err)
	}

	if got := signInOf(t, svc, store.OffsiteTarget{}); got.RESTPassword != "old" {
		t.Fatalf("the default sign-in took %q, want it unchanged", got.RESTPassword)
	}
	if saved.CredsRef == a.CredsRef || signInOf(t, svc, saved).RESTPassword != "new" {
		t.Fatalf("destination = %+v, want a sign-in of its own with the new password", saved)
	}
}

func TestASignInOnlyOneDestinationUsesIsChangedWhereItIs(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, opensWithPassword("new"))
	a, _ := restDestination(t, svc, st, "vms")

	saved, _, err := svc.EditDestinationConnection(context.Background(), a.ID, connectionEdit{Settings: map[string]string{"pass": "new"}})
	if err != nil {
		t.Fatal(err)
	}

	if saved.CredsRef != a.CredsRef {
		t.Fatalf("sign-in = %q, want %q changed in place", saved.CredsRef, a.CredsRef)
	}
}

func TestANewPasswordForADestinationWhoseSignInIsGoneIsKept(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, opensWithPassword("new"))
	a, _ := restDestination(t, svc, st, "vms")
	a.CredsRef = "gone"
	if _, err := st.SaveDestination(a); err != nil {
		t.Fatal(err)
	}

	saved, _, err := svc.EditDestinationConnection(context.Background(), a.ID, connectionEdit{Settings: map[string]string{"user": "bv", "pass": "new"}})
	if err != nil {
		t.Fatal(err)
	}

	settings, err := svc.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := svc.savedSignIn(settings, saved.CredsRef); err != nil || got.RESTPassword != "new" {
		t.Fatalf("sign-in %q = %+v, %v, want one that holds the new password", saved.CredsRef, got, err)
	}
}
