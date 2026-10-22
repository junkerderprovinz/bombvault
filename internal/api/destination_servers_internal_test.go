package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"slices"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/junkerderprovinz/bombvault/internal/remotes"
)

// sshServer listens on a free local port with a fresh host key and lets
// nobody in. It returns its port and the host_keys entry rclone pins it with.
func sshServer(t *testing.T) (port, pin string) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, net.ErrClosed },
	}
	config.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_, _, _, _ = ssh.NewServerConn(c, config)
				_ = c.Close()
			}()
		}
	}()
	_, port, _ = net.SplitHostPort(ln.Addr().String())
	return port, "ssh-ed25519 " + base64.StdEncoding.EncodeToString(signer.PublicKey().Marshal())
}

func TestAnSSHKeyServerAlsoGetsThePasswordTypedForIt(t *testing.T) {
	svc, _, _ := newProbeSvc(t, nil)
	box, _ := remotes.FindProvider("storagebox")

	d, err := svc.draftFor(box, map[string]string{"pass": "typed", "key_pem": "a pasted key", "ssh": "ssh -i other"})
	if err != nil {
		t.Fatal(err)
	}

	if d.Settings["pass"] != "typed" || d.Settings["key_file"] != svc.sshKeyPath() || d.Settings["port"] != "23" {
		t.Fatalf("storage box draft = %+v, want BombVault's key, the password and port 23", d)
	}
	if _, ok := d.Settings["key_pem"]; ok {
		t.Fatal("a pasted key replaced BombVault's key")
	}
	if _, ok := d.Settings["ssh"]; ok {
		t.Fatal("an ssh command replaced BombVault's own sign-in")
	}
}

func TestAPasswordKeepsItsSpacesAndAServerNameLosesThem(t *testing.T) {
	svc, _, _ := newConnectionSvc(t, nil)
	useFakeRclone(t, svc)

	d, err := svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "smb", Name: "NAS", Dir: "backups",
		Settings: map[string]string{"host": " nas.lan ", "user": "bv", "pass": " secret"},
	})
	if err != nil {
		t.Fatal(err)
	}

	remote, _, _ := strings.Cut(strings.TrimPrefix(d.Repo, "rclone:"), ":")
	got, _ := storedRemote(t, svc, remote)
	if got["pass"] != "obscured- secret" || got["host"] != "nas.lan" {
		t.Fatalf("remote = %v, want the password with its space and the host without", got)
	}
}

func TestAnSMBDestinationSitsInsideAShare(t *testing.T) {
	svc, _, _ := newConnectionSvc(t, nil)
	useFakeRclone(t, svc)
	login := map[string]string{"host": "nas.lan", "user": "bv", "pass": "pw"}

	if _, err := svc.SaveNewDestination(context.Background(), draftRequest{Provider: "smb", Settings: login, Dir: "/"}); !errors.Is(err, errPickShare) {
		t.Fatalf("saving at the share list: err = %v, want the share asked for", err)
	}
	if _, err := svc.DraftMakeFolder(context.Background(), draftRequest{Provider: "smb", Settings: login, Folder: "new"}); !errors.Is(err, errShareMaker) {
		t.Fatalf("a folder among the shares: err = %v, want the refusal", err)
	}
	d, _ := smbDestination(t, svc)
	if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Dir: folder("")}); !errors.Is(err, errPickShare) {
		t.Fatalf("moving to the share list: err = %v, want the share asked for", err)
	}
}

func TestAFolderWalkOfASavedRemoteReachesItWithWhatTheConfigHolds(t *testing.T) {
	svc, _, _ := newConnectionSvc(t, nil)
	d, _ := smbDestination(t, svc)

	folders, _, err := svc.DraftFolders(context.Background(), draftRequest{
		Provider: "smb", Destination: d.ID, Dir: "backups", Settings: map[string]string{"port": "4450"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"host=nas.lan", "user=bv", "pass=obscured-old", "port=4450"} {
		if !slices.Contains(folders, want) {
			t.Fatalf("rclone was handed %v, want %s from the config", folders, want)
		}
	}

	folders, _, err = svc.DraftFolders(context.Background(), draftRequest{
		Provider: "smb", Destination: d.ID, Dir: "backups", Settings: map[string]string{"pass": "new"},
	})
	if err != nil || !slices.Contains(folders, "pass=obscured-new") {
		t.Fatalf("rclone was handed %v, %v, want the typed password over the stored one", folders, err)
	}
}

func TestAnSFTPServerIsPinnedToTheHostKeyItShowedAtTheTest(t *testing.T) {
	svc, _, _ := newConnectionSvc(t, nil)
	useFakeRclone(t, svc)
	port, pin := sshServer(t)
	settings := map[string]string{"host": "127.0.0.1", "port": port, "user": "bv"}

	check, err := svc.CheckDraft(context.Background(), draftRequest{Provider: "sftp", Settings: settings})
	if err != nil {
		t.Fatal(err)
	}
	if check.HostKey == nil || check.HostKey.Pin != pin || !strings.HasPrefix(check.HostKey.Fingerprint, "SHA256:") {
		t.Fatalf("test found %+v, want the server's key %s", check.HostKey, pin)
	}

	folders, _, err := svc.DraftFolders(context.Background(), draftRequest{Provider: "sftp", Settings: settings})
	if err != nil || !slices.Contains(folders, "host_keys="+pin) {
		t.Fatalf("rclone was handed %v, %v, want the host key pinned", folders, err)
	}

	d, err := svc.SaveNewDestination(context.Background(), draftRequest{Provider: "sftp", Settings: settings, Dir: "bv"})
	if err != nil {
		t.Fatal(err)
	}
	remote, _, _ := strings.Cut(strings.TrimPrefix(d.Repo, "rclone:"), ":")
	if got, _ := storedRemote(t, svc, remote); got["host_keys"] != pin {
		t.Fatalf("remote = %v, want host_keys %s", got, pin)
	}

	moved, other := sshServer(t)
	if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Settings: map[string]string{"port": moved}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := storedRemote(t, svc, remote); got["host_keys"] != other {
		t.Fatalf("remote = %v, want the key of the server it moved to", got)
	}
}
