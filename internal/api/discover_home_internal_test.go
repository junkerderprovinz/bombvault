package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// writeContainerDef leaves an encrypted recreate definition where Discover looks.
func writeContainerDef(t *testing.T, f *placementFixture, repoLoc, name string) {
	t.Helper()
	dir := filepath.Join(filepath.FromSlash(repoLoc), "def")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(f.svc.cfg.AppKey, []byte(`{"appdataPaths":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".def"), enc, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverWritesTheFoundLocationChosen(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.hold(f.root+"/nas", snap("aaaa0001", 200, "container:nginx"))
	writeContainerDef(t, f, f.root+"/nas", "nginx")
	res, err := f.svc.Discover(context.Background(), false)
	if err != nil || res.Found != 1 {
		t.Fatalf("Discover = %+v, %v", res, err)
	}
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	if got := f.home(item); got.Repo != nas.ID || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want NAS chosen", got)
	}
	f.setDefault("containers", "")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if repoID, _, err := f.svc.settleHome(context.Background(), settings, item); err != nil || repoID != nas.ID {
		t.Fatalf("after a default on the domain path the item goes to %q, %v, want NAS", repoID, err)
	}
	if err := f.svc.DeleteBackups(context.Background(), "nginx"); err != nil {
		t.Fatal(err)
	}
	if len(f.eng.deletes) != 1 || filepath.ToSlash(f.eng.deletes[0].Repo) != f.root+"/nas" {
		t.Fatalf("deletes = %+v, want one on NAS", f.eng.deletes)
	}
}

func TestAnUnreadableDomainPathLeavesTheLocationEmptyAndUnread(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.hold(f.root+"/nas", snap("aaaa0001", 200, "container:nginx"))
	writeContainerDef(t, f, f.root+"/nas", "nginx")
	f.eng.listErr = map[string]error{f.domainPath("containers"): errors.New("wrong password or no key found")}
	if _, err := f.svc.Discover(context.Background(), false); err == nil {
		t.Fatal("the unreadable domain path must still reach the caller")
	}
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	if got := f.home(item); got.Repo != "" || got.Choice != store.RepoChosenUnread {
		t.Fatalf("home = %+v, want empty and unread", got)
	}
	f.eng.listErr = nil
	if _, err := f.svc.Discover(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if got := f.home(item); got.Repo != nas.ID || got.Choice != store.RepoChosen {
		t.Fatalf("a complete pass left %+v, want NAS chosen", got)
	}
}

func TestDiscoverKeepsAChosenDomainPath(t *testing.T) {
	f := newPlacementFixture(t)
	f.namedRepo("NAS", "nas")
	f.container("nginx", "")
	f.hold(f.root+"/nas", snap("aaaa0001", 200, "container:nginx"))
	writeContainerDef(t, f, f.root+"/nas", "nginx")
	if _, err := f.svc.Discover(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); got.Repo != "" || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want the explicit domain path kept", got)
	}
}

func TestDiscoverChoosesAnOpenRowWithHistoryWhereItIs(t *testing.T) {
	f := newPlacementFixture(t)
	f.openVM("win11")
	f.hold(f.domainPath("vms"), snap("aaaa0001", 200, "vm:win11"))
	dir := filepath.Join(filepath.FromSlash(f.domainPath("vms")), "vm-def")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(f.svc.cfg.AppKey, []byte(`{"method":"graceful"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "win11.def"), enc, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.DiscoverVMs(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if got := f.home(store.ItemRef{Domain: "vms", Key: "win11"}); got.Repo != "" || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want the domain path chosen", got)
	}
}

func TestDiscoverLeavesAnOpenRowAloneWhileABackupHoldsTheDomain(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	set := f.openFileSet("Photos")
	f.hold(f.root+"/nas", snap("aaaa0001", 200, "fileset:Photos"))
	unlock, ok := f.svc.tryLockDomainFor("files", "backup")
	if !ok {
		t.Fatal("could not take the files lock")
	}
	res, err := f.svc.DiscoverFileSets(context.Background(), false)
	unlock()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.LeftOpen) != 1 || res.LeftOpen[0] != "Photos" {
		t.Fatalf("leftOpen = %v, want Photos", res.LeftOpen)
	}
	if got := f.home(store.ItemRef{Domain: "files", Key: set.ID}); got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want it untouched", got)
	}
	if _, err := f.svc.DiscoverFileSets(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if got := f.home(store.ItemRef{Domain: "files", Key: set.ID}); got.Repo != nas.ID {
		t.Fatalf("home = %+v, want NAS once the domain is free", got)
	}
}

func TestDiscoverInAFreshDatabasePausesTheDomain(t *testing.T) {
	f := newPlacementFixture(t)
	f.hold(f.domainPath("files"), snap("aaaa0001", 200, "fileset:Photos"))
	res := f.do(http.MethodPost, "/api/files/discover", nil)
	if res["paused"] != true {
		t.Fatalf("discover = %v, want paused", res)
	}
	if left, ok := res["leftOpen"].([]any); !ok || len(left) != 0 {
		t.Fatalf("leftOpen = %v, want an empty list", res["leftOpen"])
	}
	d, found, err := f.st.PlacementDefaultFor("files")
	if err != nil || !found || !d.Paused() {
		t.Fatalf("default = %+v, %v, %v, want paused", d, found, err)
	}
}

func TestDiscoverAfterABackupDoesNotPause(t *testing.T) {
	f := newPlacementFixture(t)
	tg := f.container("nginx", "")
	f.backupRun(tg.ID, 100)
	f.hold(f.domainPath("containers"), snap("aaaa0001", 200, "container:nginx"))
	writeContainerDef(t, f, f.domainPath("containers"), "nginx")
	res, err := f.svc.Discover(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Found != 1 {
		t.Fatalf("Found = %d, want the one container the pass rediscovered", res.Found)
	}
	if res.Paused {
		t.Fatal("a database that has backed up this domain was paused")
	}
	if _, found, _ := f.st.PlacementDefaultFor("containers"); found {
		t.Fatal("a default row appeared without a pause")
	}
}

func TestARepeatDiscoverDoesNotRepauseAConfirmedDomain(t *testing.T) {
	f := newPlacementFixture(t)
	f.hold(f.domainPath("containers"), snap("aaaa0001", 200, "container:nginx"))
	writeContainerDef(t, f, f.domainPath("containers"), "nginx")
	if res, err := f.svc.Discover(context.Background(), false); err != nil || !res.Paused {
		t.Fatalf("Discover = %+v, %v, want the fresh database paused", res, err)
	}
	if err := f.st.ConfirmPlacement("containers", nil); err != nil {
		t.Fatal(err)
	}

	// A restore rebuilds the same rows without writing a backup run, so
	// DomainHasHistory looks exactly as it did before the confirmation.
	sent := placementWebhook(t, f)
	res, err := f.svc.Discover(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Paused {
		t.Fatal("a repeat Discover repaused a domain the operator already confirmed")
	}
	if pausedDefault(t, f, "containers") {
		t.Fatal("the confirmed domain paused again")
	}
	if msgs := sent(); len(msgs) != 0 {
		t.Fatalf("notifications = %q, want none: the operator already confirmed this domain", msgs)
	}
}

func TestAProbeNeverPauses(t *testing.T) {
	f := newPlacementFixture(t)
	f.hold(f.domainPath("files"), snap("aaaa0001", 200, "fileset:Photos"))
	res, err := f.svc.DiscoverFileSets(context.Background(), true)
	if err != nil || res.Paused || res.Found != 1 {
		t.Fatalf("probe = %+v, %v, want one found and no pause", res, err)
	}
}

// TestDiscoverFailureEnvelopeCarriesPausedAndLeftOpen pins that a failing pass
// still answers with the same paused/leftOpen shape as a successful one, so the
// caller never has to branch on ok to read them.
func TestDiscoverFailureEnvelopeCarriesPausedAndLeftOpen(t *testing.T) {
	f := newPlacementFixture(t)
	f.eng.listErr = map[string]error{f.domainPath("containers"): errors.New("wrong password or no key found")}
	res := f.do(http.MethodPost, "/api/discover", nil)
	if res["ok"] != false {
		t.Fatalf("discover = %v, want a failure envelope", res)
	}
	if _, ok := res["paused"].(bool); !ok {
		t.Fatalf("discover = %v, want a paused field even on failure", res)
	}
	left, ok := res["leftOpen"].([]any)
	if !ok || len(left) != 0 {
		t.Fatalf("leftOpen = %v, want an empty list, not null", res["leftOpen"])
	}
}
