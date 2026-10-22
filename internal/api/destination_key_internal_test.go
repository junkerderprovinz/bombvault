package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/sshconn"
)

const typedPassword = "hunter2-storagebox"

type keyInstallCall struct {
	target   sshconn.KeyTarget
	password string
	pubKey   string
}

// stubKeyInstall stands in for the server, answering with err, and records
// the calls it got.
func stubKeyInstall(t *testing.T, err error) *[]keyInstallCall {
	t.Helper()
	var calls []keyInstallCall
	prev := installSSHKey
	installSSHKey = func(_ context.Context, target sshconn.KeyTarget, password, pubKey string) error {
		calls = append(calls, keyInstallCall{target, password, pubKey})
		return err
	}
	t.Cleanup(func() { installSSHKey = prev })
	return &calls
}

func keyInstallService(t *testing.T) *Service {
	t.Helper()
	svc, _ := newSyncTestService(t)
	svc.cfg = config.Config{DataDir: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(svc.cfg.DataDir, "ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(svc.sshKeyPath()+".pub", []byte("ssh-ed25519 AAAAC3Nza bombvault\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return svc
}

func postKeyInstall(t *testing.T, svc *Service, req keyInstallRequest) map[string]any {
	t.Helper()
	body, err := json.Marshal(req) //nolint:gosec // G117: the request carries the password the endpoint takes
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/offsite/drafts/install-key", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	(&Handler{svc: svc}).handleInstallDraftKey(rec, r)
	return dashDecode(t, rec)
}

func TestKeyInstallOnAStorageBoxUsesPort23AndTheStorageBoxCommand(t *testing.T) {
	svc := keyInstallService(t)
	calls := stubKeyInstall(t, nil)
	body := postKeyInstall(t, svc, keyInstallRequest{
		Provider: "storagebox",
		Settings: map[string]string{"host": " u1-sub2.your-storagebox.de ", "user": "u1-sub2", "port": "22"},
		Password: typedPassword,
	})
	if body["ok"] != true {
		t.Fatalf("body = %v", body)
	}
	if len(*calls) != 1 {
		t.Fatalf("calls = %+v, want one", *calls)
	}
	got := (*calls)[0]
	want := sshconn.KeyTarget{
		Host: "u1-sub2.your-storagebox.de", User: "u1-sub2", Port: "23", StorageBox: true,
		KnownHosts: filepath.Join(svc.cfg.DataDir, "ssh", "known_hosts"),
	}
	if got.target != want || got.password != typedPassword || got.pubKey != "ssh-ed25519 AAAAC3Nza bombvault" {
		t.Fatalf("call = %+v, want %+v with the password and the trimmed key", got, want)
	}
}

func TestKeyInstallOnAnSFTPServerKeepsItsPort(t *testing.T) {
	svc := keyInstallService(t)
	calls := stubKeyInstall(t, nil)
	for port, want := range map[string]string{"2222": "2222", "": "22"} {
		*calls = nil
		body := postKeyInstall(t, svc, keyInstallRequest{
			Provider: "sftp", Settings: map[string]string{"host": "nas", "user": "backup", "port": port}, Password: typedPassword,
		})
		if body["ok"] != true || len(*calls) != 1 {
			t.Fatalf("port %q: body = %v, calls = %+v", port, body, *calls)
		}
		if c := (*calls)[0].target; c.Port != want || c.StorageBox {
			t.Fatalf("port %q: target = %+v, want port %s and no Storage Box command", port, c, want)
		}
	}
}

func TestKeyInstallRefusesWhatItCannotSignInTo(t *testing.T) {
	svc := keyInstallService(t)
	calls := stubKeyInstall(t, nil)
	for name, req := range map[string]keyInstallRequest{
		"an S3 service":    {Provider: "wasabi", Settings: map[string]string{"host": "h", "user": "u"}, Password: typedPassword},
		"no host":          {Provider: "sftp", Settings: map[string]string{"user": "u"}, Password: typedPassword},
		"no password":      {Provider: "sftp", Settings: map[string]string{"host": "h", "user": "u"}},
		"a port of words":  {Provider: "sftp", Settings: map[string]string{"host": "h", "user": "u", "port": "ssh"}, Password: typedPassword},
		"an unknown kind":  {Provider: "nope", Settings: map[string]string{"host": "h", "user": "u"}, Password: typedPassword},
		"a port too large": {Provider: "sftp", Settings: map[string]string{"host": "h", "user": "u", "port": "70000"}, Password: typedPassword},
	} {
		if body := postKeyInstall(t, svc, req); body["ok"] != false {
			t.Fatalf("%s: body = %v, want a refusal", name, body)
		}
	}
	if len(*calls) != 0 {
		t.Fatalf("signed in for a refused request: %+v", *calls)
	}
}

func TestKeyInstallNamesEachFailureTheWizardExplains(t *testing.T) {
	for want, err := range map[string]error{
		"password-refused":    sshconn.ErrPasswordRefused,
		"no-password-sign-in": sshconn.ErrNoPasswordSignIn,
		"session-refused":     fmt.Errorf("%w: administratively prohibited", sshconn.ErrSessionRefused),
		"unreachable":         fmt.Errorf("%w: dial tcp: i/o timeout", sshconn.ErrUnreachable),
		"host-key-changed":    sshconn.ErrHostKeyChanged,
		"no-way-in":           sshconn.ErrNoWayIn,
		"":                    errors.New("sshconn: install-ssh-key: invalid key"),
	} {
		stubKeyInstall(t, err)
		body := postKeyInstall(t, keyInstallService(t), keyInstallRequest{
			Provider: "storagebox", Settings: map[string]string{"host": "h", "user": "u"}, Password: typedPassword,
		})
		code, _ := body["code"].(string)
		if body["ok"] != false || code != want || body["error"] == "" {
			t.Fatalf("%v: body = %v, want code %q", err, body, want)
		}
	}
}

func TestKeyInstallKeepsThePasswordOutOfTheAnswerTheLogAndTheDisk(t *testing.T) {
	svc := keyInstallService(t)
	req := keyInstallRequest{Provider: "storagebox", Settings: map[string]string{"host": "h", "user": "u"}, Password: typedPassword}
	logged := captureLog(t, func() {
		stubKeyInstall(t, fmt.Errorf("%w: the server said %s", sshconn.ErrSessionRefused, typedPassword))
		failed := postKeyInstall(t, svc, req)
		if msg, _ := failed["error"].(string); strings.Contains(msg, typedPassword) || !strings.Contains(msg, "[password]") || failed["code"] != "session-refused" {
			t.Fatalf("body = %v, want the password scrubbed and the code kept", failed)
		}
		stubKeyInstall(t, nil)
		if body := postKeyInstall(t, svc, req); body["ok"] != true {
			t.Fatalf("body = %v", body)
		}
	})
	if !strings.Contains(logged, "u@h:23") || strings.Contains(logged, typedPassword) {
		t.Fatalf("log = %q, want the install noted without the password", logged)
	}
	settings, err := svc.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := json.Marshal(settings); bytes.Contains(b, []byte(typedPassword)) {
		t.Fatal("the settings carry the password")
	}
	err = filepath.WalkDir(svc.cfg.DataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path) //nolint:gosec // a file of the test's own data folder
		if err == nil && bytes.Contains(b, []byte(typedPassword)) {
			t.Errorf("%s carries the password", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}
