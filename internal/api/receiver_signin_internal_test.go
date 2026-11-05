package api

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// openRecorder answers every open and remembers where it was sent and how.
type openRecorder struct {
	ResticEngine
	repo string
	mode restic.Mode
}

func (e *openRecorder) RepoOpensErr(_ context.Context, repo string, m restic.Mode) error {
	e.repo, e.mode = repo, m
	return nil
}

func receivedRepoAt(t *testing.T, f *receiverFixture, loc, member string) store.ReceivedRepo {
	t.Helper()
	enc, err := secret.Encrypt(f.svc.cfg.AppKey, []byte("the-senders-restic-password"))
	if err != nil {
		t.Fatal(err)
	}
	return store.ReceivedRepo{Name: "tower", Repo: loc, MemberID: member, ResticPasswordEnc: enc, Enabled: true}
}

func serverOnThisInstance(t *testing.T, f *receiverFixture) {
	t.Helper()
	if err := f.st.SaveReceiverServer(store.ReceiverServer{
		ContainerName: restServerName, Folder: "user/appdata/rest-server", HostPath: "/mnt/user/appdata/rest-server",
		Port: 8000, User: "outside", Host: "vault",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAnOfferedAddressOnThisInstancesServerIsReadFromItsFolder(t *testing.T) {
	cases := []struct {
		name, loc, want string
	}{
		{"the server's address", "rest:http://vault:8000/tower/containers", "user/appdata/rest-server/tower/containers"},
		{"a login in the address changes nothing", "rest:http://tower:pw@vault:8000/tower/files", "user/appdata/rest-server/tower/files"},
		{"the member's login at another name for this box", "rest:http://192.168.1.5:8000/tower-box/vms", "user/appdata/rest-server/tower-box/vms"},
		{"dots stay inside the folder", "rest:http://vault:8000/../../boot/x", "user/appdata/rest-server/boot/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newReceiverFixture(t)
			serverOnThisInstance(t, f)
			if err := f.st.CreateReceiverLogin(store.ReceiverLogin{MemberID: "m-tower", MemberName: "tower", User: "tower-box"}); err != nil {
				t.Fatal(err)
			}
			eng := &openRecorder{}
			f.svc.engine = eng
			if err := f.svc.receiverProbe(context.Background(), receivedRepoAt(t, f, tc.loc, "m-tower")); err != nil {
				t.Fatal(err)
			}
			if want := filepath.ToSlash(f.mount) + "/" + tc.want; filepath.ToSlash(eng.repo) != want {
				t.Fatalf("opened %q, want %q", eng.repo, want)
			}
			if len(eng.mode.Env) != 0 {
				t.Fatalf("reading a folder took a login: %v", eng.mode.Env)
			}
		})
	}
}

func TestAnAddressOnAnotherServerIsNotTakenForThisInstancesFolder(t *testing.T) {
	cases := []struct {
		name, loc string
	}{
		{"another host", "rest:http://nas:8000/somebody/files"},
		{"this host at another port", "rest:http://vault:8001/tower/files"},
		{"not a rest-server", "s3:https://vault:8000/tower/files"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newReceiverFixture(t)
			serverOnThisInstance(t, f)
			eng := &openRecorder{}
			f.svc.engine = eng
			if err := f.svc.receiverProbe(context.Background(), receivedRepoAt(t, f, tc.loc, "m-tower")); err != nil {
				t.Fatal(err)
			}
			if eng.repo != tc.loc {
				t.Fatalf("opened %q, want the address as it is", eng.repo)
			}
		})
	}
}

func TestAReceivedRepoSignsInWithItsSavedSignIn(t *testing.T) {
	f := newReceiverFixture(t)
	eng := &openRecorder{}
	f.svc.engine = eng
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "set-1", Name: "nas", CloudCreds: CloudCreds{RESTUser: "tower", RESTPassword: "pw"}}}); err != nil {
		t.Fatal(err)
	}
	rr := receivedRepoAt(t, f, "rest:http://nas:8000/tower/files", "m-tower")
	rr.CredsRef = "set-1"
	if err := f.svc.receiverProbe(context.Background(), rr); err != nil {
		t.Fatal(err)
	}
	if env := strings.Join(eng.mode.Env, "\n"); !strings.Contains(env, "RESTIC_REST_USERNAME=tower") {
		t.Fatalf("env %q, want the sign-in's login", env)
	}

	rr.CredsRef = "gone"
	if err := f.svc.receiverProbe(context.Background(), rr); err == nil || !strings.Contains(err.Error(), "saved sign-in") {
		t.Fatalf("a sign-in that is gone: %v", err)
	}
}

func TestASignInAReceivedRepoUsesStaysAndNamesTheRepo(t *testing.T) {
	f := newReceiverFixture(t)
	set := CloudCredSet{ID: "set-1", Name: "nas", CloudCreds: CloudCreds{RESTUser: "tower", RESTPassword: "pw"}}
	if err := f.svc.SetCloudCredSets([]CloudCredSet{set}); err != nil {
		t.Fatal(err)
	}
	rr := receivedRepoAt(t, f, "rest:http://nas:8000/tower/files", "m-tower")
	rr.CredsRef = set.ID
	if _, err := f.st.CreateReceivedRepo(rr); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetCloudCredSets(nil); err == nil || !strings.Contains(err.Error(), "tower") {
		t.Fatalf("removing the sign-in: %v, want a refusal naming the repo", err)
	}
}

func TestARefusedSignInSaysWhatToPickAndNamesNoConnectionTest(t *testing.T) {
	bin, err := exec.LookPath("restic")
	if err != nil {
		t.Skip("no restic")
	}
	serverBin, err := exec.LookPath("rest-server")
	if err != nil {
		t.Skip("no rest-server")
	}
	f := newReceiverFixture(t)
	f.svc.engine = restic.Restic{Bin: bin, CacheDir: t.TempDir()}

	host, stop := startRestServer(t, serverBin, "tower", "secret-pw")
	defer stop()
	loc := "rest:http://" + host + "/tower/files"

	password := "the-senders-restic-password"
	initAt := exec.Command(bin, "init", "--repo", loc)
	initAt.Env = append(os.Environ(), "RESTIC_PASSWORD="+password, "RESTIC_REST_USERNAME=tower", "RESTIC_REST_PASSWORD=secret-pw")
	if out, err := initAt.CombinedOutput(); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}

	rr := receivedRepoAt(t, f, loc, "m-tower")
	err = f.svc.receiverProbe(context.Background(), rr)
	if err == nil {
		t.Fatal("a server with a login opened without one")
	}
	msg := scrubError(err)
	if strings.Contains(msg, "connection test") || !strings.Contains(msg, "Credentials") {
		t.Fatalf("message %q, want it to point at the Credentials field and at no connection test", msg)
	}

	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "set-1", Name: "nas", CloudCreds: CloudCreds{RESTUser: "tower", RESTPassword: "secret-pw"}}}); err != nil {
		t.Fatal(err)
	}
	rr.CredsRef = "set-1"
	if err := f.svc.receiverProbe(context.Background(), rr); err != nil {
		t.Fatalf("with its sign-in: %v", err)
	}
}

// startRestServer runs the real rest-server with the options a recipe gives it,
// with one login, and returns its host and port.
func startRestServer(t *testing.T, bin string, logins ...string) (string, func()) {
	t.Helper()
	data := t.TempDir()
	var lines strings.Builder
	for i := 0; i+1 < len(logins); i += 2 {
		line, err := htpasswdLine(logins[i], logins[i+1])
		if err != nil {
			t.Fatal(err)
		}
		lines.WriteString(line + "\n")
	}
	if err := os.WriteFile(filepath.Join(data, htpasswdName), []byte(lines.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "--path", data, "--listen", host, "--append-only", "--private-repos", "--htpasswd-file", filepath.Join(data, htpasswdName))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stop := func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	for range 100 {
		c, err := net.DialTimeout("tcp", host, 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return host, stop
		}
		time.Sleep(50 * time.Millisecond)
	}
	stop()
	t.Fatal("the rest-server did not start")
	return "", nil
}
