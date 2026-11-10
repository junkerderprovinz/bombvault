package api

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// With restic and rest-server: cube took the shared repository over and runs
// its server on the same data folder at another port, the old one is gone,
// and tower follows it there with its own key and login.
func TestAMemberFollowsTheNewHolderToItsNewAddressWithRestic(t *testing.T) {
	bin, err := exec.LookPath("restic")
	if err != nil {
		t.Skip("no restic")
	}
	serverBin, err := exec.LookPath("rest-server")
	if err != nil {
		t.Skip("no rest-server")
	}
	ctx := context.Background()
	tower := newInstance(t, "tower", strings.Repeat("b2", 32))
	cube := newInstance(t, "cube", strings.Repeat("c3", 32))
	pairThroughRelay(t, cube, tower)
	runReceiver(t, cube)
	if err := cube.st.SetReceiverServerHost("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	for _, in := range []*instance{tower, cube} {
		if _, err := in.st.MutateSettings(func(s *store.Settings) error {
			s.EncryptionEnabled = true
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		in.svc.engine = restic.Restic{Bin: bin, CacheDir: t.TempDir()}
	}

	data := filepath.Join(cube.svc.cfg.HostMountRoot, "shared")
	folder := filepath.Join(data, sharedRepoFolder)
	if err := os.MkdirAll(data, 0o750); err != nil {
		t.Fatal(err)
	}
	login, err := htpasswdLine("tower", "tower-pw")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, htpasswdName), []byte(login+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	eng, err := cube.svc.sharedEngine()
	if err != nil {
		t.Fatal(err)
	}
	if err := cube.svc.engine.Init(ctx, folder, cube.svc.sharedOwnerMode()); err != nil {
		t.Fatal(err)
	}
	if err := eng.AddKey(ctx, folder, cube.svc.sharedOwnerMode(), tower.svc.backupPassword(), tower.svc.resticHost(), "tower"); err != nil {
		t.Fatal(err)
	}

	oldHost, stopOld := runRestServer(t, serverBin, data)
	defer stopOld()
	oldAt := "rest:http://" + oldHost + "/" + sharedRepoFolder
	if err := tower.svc.takeUpSharedRepo(oldAt, "nas-id", "nas", peerSharedOffer{User: "tower", Login: "tower-pw"}); err != nil {
		t.Fatal(err)
	}
	if err := tower.st.SaveSharedJoin(store.SharedJoin{Direction: store.SharedJoinOut, MemberID: "nas-id", MemberName: "nas", Host: tower.svc.resticHost(), State: store.SharedJoinJoined, Repo: oldAt}); err != nil {
		t.Fatal(err)
	}
	want, err := tower.svc.sharedRepoID(ctx, ownerAt(t, tower, oldAt))
	if err != nil {
		t.Fatalf("tower cannot read the repository at the old address: %v", err)
	}

	newHost, stopNew := runRestServer(t, serverBin, data)
	defer stopNew()
	_, port, err := net.SplitHostPort(newHost)
	if err != nil {
		t.Fatal(err)
	}
	newPort, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	if err := cube.st.SaveSharedRepoServer(store.SharedRepoServer{ContainerName: sharedServerName, Folder: "shared", Port: newPort, User: "cube", Repo: "shared/" + sharedRepoFolder}); err != nil {
		t.Fatal(err)
	}
	held, err := cube.st.PutSharedRepo(store.SharedRepo{Repo: "shared/" + sharedRepoFolder, Owner: cube.id(t), Members: []store.SharedMember{
		{Host: tower.svc.resticHost(), Instance: tower.id(t), Name: "tower"},
		{Host: cube.svc.resticHost(), Instance: cube.id(t), Name: "cube"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := cube.st.QueueSharedHolderNotices(held.ID, []string{tower.id(t)}); err != nil {
		t.Fatal(err)
	}
	stopOld()

	cube.svc.tellSharedHolders(ctx)

	newAt := "rest:http://" + newHost + "/" + sharedRepoFolder
	sr := ownerAt(t, tower, newAt)
	if sr.Owner != cube.id(t) {
		t.Errorf("tower names %s as the holder, want cube", sr.Owner)
	}
	if got := targetsAt(t, tower, newAt); len(got) != len(offsiteConfigDomains) {
		t.Errorf("%d of tower's targets point at the new address, want %d", len(got), len(offsiteConfigDomains))
	}
	repo, mode, err := tower.svc.sharedReach(sr)
	if err != nil || repo != newAt {
		t.Fatalf("tower reaches the repository at %s (%v), want the new address", repo, err)
	}
	if got, err := tower.svc.sharedRepoID(ctx, sr); err != nil || got != want {
		t.Errorf("tower reads the id %q (%v) at the new address, want %q", got, err, want)
	}
	src := filepath.Join(t.TempDir(), "docs")
	if err := os.MkdirAll(src, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := tower.svc.engine.BackupDir(ctx, repo, src, []string{"fileset:docs"}, mode); err != nil {
		t.Fatalf("tower cannot write at the new address: %v", err)
	}
	keys, err := eng.Keys(ctx, folder, cube.svc.sharedOwnerMode())
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if k.UserName == sharedProofUser {
			t.Errorf("the key %s of the check is left", k.ID)
		}
	}
}
