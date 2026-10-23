package sshconn

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const testPassword = "correct horse battery"

// fakeServer is an SSH server that takes one password and offers what a test
// switches on: sessions, an SFTP subsystem rooted in a folder, and commands.
type fakeServer struct {
	t        *testing.T
	addr     string
	hostKey  ssh.Signer
	cfg      *ssh.ServerConfig
	noSess   bool
	sftpDir  string
	run      func(cmd string, stdin []byte) (string, uint32)
	mu       sync.Mutex
	commands []string
	stdins   []string
}

// ran returns the commands the server ran and what each got on stdin.
func (f *fakeServer) ran() (commands, stdins []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.commands), slices.Clone(f.stdins)
}

func newSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newPublicKey(t *testing.T) string {
	t.Helper()
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(newSigner(t).PublicKey()))) + " bombvault"
}

func startFakeServer(t *testing.T, setup func(*fakeServer)) *fakeServer {
	t.Helper()
	f := &fakeServer{t: t, hostKey: newSigner(t)}
	f.cfg = &ssh.ServerConfig{
		PasswordCallback: func(_ ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if string(pass) == testPassword {
				return nil, nil
			}
			return nil, errors.New("wrong password")
		},
	}
	if setup != nil {
		setup(f)
	}
	f.cfg.AddHostKey(f.hostKey)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	f.addr = ln.Addr().String()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn)
		}
	}()
	return f
}

func (f *fakeServer) target(t *testing.T, storageBox bool) KeyTarget {
	host, port, _ := net.SplitHostPort(f.addr)
	return KeyTarget{Host: host, User: "u1", Port: port, StorageBox: storageBox, KnownHosts: filepath.Join(t.TempDir(), "known_hosts")}
}

func (f *fakeServer) serve(conn net.Conn) {
	_, chans, reqs, err := ssh.NewServerConn(conn, f.cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" || f.noSess {
			_ = nc.Reject(ssh.Prohibited, "no sessions for this account")
			continue
		}
		ch, chReqs, err := nc.Accept()
		if err != nil {
			continue
		}
		go f.session(ch, chReqs)
	}
}

func (f *fakeServer) session(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close() //nolint:errcheck // the test server's channel
	exit := func(status uint32) {
		_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
	}
	for req := range reqs {
		var arg struct{ Value string }
		_ = ssh.Unmarshal(req.Payload, &arg)
		switch {
		case req.Type == "subsystem" && arg.Value == "sftp" && f.sftpDir != "":
			_ = req.Reply(true, nil)
			srv, err := sftp.NewServer(ch, sftp.WithServerWorkingDirectory(f.sftpDir))
			if err == nil {
				_ = srv.Serve()
			}
			exit(0)
			return
		case req.Type == "exec" && f.run != nil:
			_ = req.Reply(true, nil)
			stdin, _ := io.ReadAll(ch)
			f.mu.Lock()
			f.commands = append(f.commands, arg.Value)
			f.stdins = append(f.stdins, string(stdin))
			f.mu.Unlock()
			out, status := f.run(arg.Value, stdin)
			_, _ = io.WriteString(ch, out)
			exit(status)
			return
		default:
			_ = req.Reply(false, nil)
		}
	}
}

func install(t *testing.T, target KeyTarget, password, pubKey string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return InstallKey(ctx, target, password, pubKey)
}

func TestInstallKeyOnAStorageBoxPipesTheKeyIntoInstallSSHKey(t *testing.T) {
	srv := startFakeServer(t, func(f *fakeServer) {
		f.run = func(string, []byte) (string, uint32) {
			return "Key No. 1 (ssh-ed25519 bombvault) was installed in RFC4716 format\n", 0
		}
	})
	pub := newPublicKey(t)
	target := srv.target(t, true)
	if err := install(t, target, testPassword, pub); err != nil {
		t.Fatalf("InstallKey: %v", err)
	}
	commands, stdins := srv.ran()
	if len(commands) != 1 || commands[0] != "install-ssh-key" {
		t.Fatalf("commands = %q, want install-ssh-key alone", commands)
	}
	if stdins[0] != pub+"\n" {
		t.Fatalf("stdin = %q, want the public key", stdins[0])
	}
	pinned, err := os.ReadFile(target.KnownHosts)
	if err != nil {
		t.Fatal(err)
	}
	want := knownhosts.Line([]string{knownhosts.Normalize(srv.addr)}, srv.hostKey.PublicKey())
	if strings.TrimSpace(string(pinned)) != want {
		t.Fatalf("known_hosts = %q, want %q", pinned, want)
	}
}

func TestInstallKeyReportsWhatInstallSSHKeySaid(t *testing.T) {
	srv := startFakeServer(t, func(f *fakeServer) {
		f.run = func(string, []byte) (string, uint32) { return "invalid key\n", 1 }
	})
	err := install(t, srv.target(t, true), testPassword, newPublicKey(t))
	if err == nil || !strings.Contains(err.Error(), "invalid key") {
		t.Fatalf("err = %v, want the command's own words", err)
	}
}

func TestInstallKeyTellsAWrongPasswordApart(t *testing.T) {
	srv := startFakeServer(t, nil)
	err := install(t, srv.target(t, true), "not it", newPublicKey(t))
	if !errors.Is(err, ErrPasswordRefused) {
		t.Fatalf("err = %v, want ErrPasswordRefused", err)
	}
}

// OpenSSH offers keyboard-interactive after a wrong password and then refuses
// it without asking anything.
func TestInstallKeyTellsAWrongPasswordApartWhenKeyboardInteractiveAsksNothing(t *testing.T) {
	srv := startFakeServer(t, func(f *fakeServer) {
		f.cfg.KeyboardInteractiveCallback = func(ssh.ConnMetadata, ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			return nil, errors.New("no more tries")
		}
	})
	err := install(t, srv.target(t, true), "not it", newPublicKey(t))
	if !errors.Is(err, ErrPasswordRefused) {
		t.Fatalf("err = %v, want ErrPasswordRefused", err)
	}
}

func TestHandshakeErrorKeepsOtherUnexpectedMessagesAsTheyAre(t *testing.T) {
	err := handshakeError(errors.New("ssh: handshake failed: ssh: unexpected message type 3 (expected 60)"))
	if errors.Is(err, ErrPasswordRefused) || !strings.Contains(err.Error(), "message type 3") {
		t.Fatalf("err = %v, want the protocol error itself", err)
	}
}

func TestInstallKeyTellsAServerWithoutPasswordSignInApart(t *testing.T) {
	srv := startFakeServer(t, func(f *fakeServer) {
		f.cfg.PasswordCallback = nil
		f.cfg.PublicKeyCallback = func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, errors.New("unknown key")
		}
	})
	err := install(t, srv.target(t, true), testPassword, newPublicKey(t))
	if !errors.Is(err, ErrNoPasswordSignIn) {
		t.Fatalf("err = %v, want ErrNoPasswordSignIn", err)
	}
}

func TestInstallKeyAnswersAKeyboardInteractivePasswordPrompt(t *testing.T) {
	srv := startFakeServer(t, func(f *fakeServer) {
		f.cfg.PasswordCallback = nil
		f.cfg.KeyboardInteractiveCallback = func(_ ssh.ConnMetadata, ask ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			answers, err := ask("", "", []string{"Password: "}, []bool{false})
			if err != nil || len(answers) != 1 || answers[0] != testPassword {
				return nil, errors.New("wrong password")
			}
			return nil, nil
		}
		f.run = func(string, []byte) (string, uint32) { return "", 0 }
	})
	if err := install(t, srv.target(t, true), testPassword, newPublicKey(t)); err != nil {
		t.Fatalf("InstallKey: %v", err)
	}
}

// A Storage Box sub-account whose SSH support is off takes the password and
// then refuses the session.
func TestInstallKeyTellsARefusedSessionApart(t *testing.T) {
	srv := startFakeServer(t, func(f *fakeServer) { f.noSess = true })
	err := install(t, srv.target(t, true), testPassword, newPublicKey(t))
	if !errors.Is(err, ErrSessionRefused) {
		t.Fatalf("err = %v, want ErrSessionRefused", err)
	}
}

func TestInstallKeyTellsAnUnreachableServerApart(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	_ = ln.Close()
	target := KeyTarget{Host: host, Port: port, User: "u1", KnownHosts: filepath.Join(t.TempDir(), "known_hosts")}
	if err := install(t, target, testPassword, newPublicKey(t)); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("err = %v, want ErrUnreachable", err)
	}
}

func TestInstallKeyRefusesAChangedHostKey(t *testing.T) {
	srv := startFakeServer(t, func(f *fakeServer) {
		f.run = func(string, []byte) (string, uint32) { return "", 0 }
	})
	target := srv.target(t, true)
	other := knownhosts.Line([]string{knownhosts.Normalize(srv.addr)}, newSigner(t).PublicKey())
	if err := os.WriteFile(target.KnownHosts, []byte(other+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := install(t, target, testPassword, newPublicKey(t)); !errors.Is(err, ErrHostKeyChanged) {
		t.Fatalf("err = %v, want ErrHostKeyChanged", err)
	}
	if commands, _ := srv.ran(); len(commands) != 0 {
		t.Fatalf("ran %q on a server whose host key changed", commands)
	}
}

func TestInstallKeyKeepsThePasswordOutOfEveryError(t *testing.T) {
	refusing := startFakeServer(t, func(f *fakeServer) { f.noSess = true })
	wrong := startFakeServer(t, nil)
	for _, err := range []error{
		install(t, refusing.target(t, true), testPassword, newPublicKey(t)),
		install(t, refusing.target(t, false), testPassword, newPublicKey(t)),
		install(t, wrong.target(t, true), testPassword+"x", newPublicKey(t)),
	} {
		if err == nil || strings.Contains(err.Error(), "horse") {
			t.Fatalf("err = %v, want an error without the password", err)
		}
	}
}

func TestInstallKeyOverSFTPCreatesSSHFolderAndFile(t *testing.T) {
	home := t.TempDir()
	srv := startFakeServer(t, func(f *fakeServer) { f.sftpDir = home })
	pub := newPublicKey(t)
	target := srv.target(t, false)
	if err := install(t, target, testPassword, pub); err != nil {
		t.Fatalf("InstallKey: %v", err)
	}
	if err := install(t, target, testPassword, pub); err != nil {
		t.Fatalf("InstallKey again: %v", err)
	}
	file := filepath.Join(home, ".ssh", "authorized_keys")
	got, err := os.ReadFile(file) //nolint:gosec // a test file
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != pub+"\n" {
		t.Fatalf("authorized_keys = %q, want the key once", got)
	}
	if runtime.GOOS == "windows" {
		return
	}
	for path, want := range map[string]os.FileMode{filepath.Dir(file): 0o700, file: 0o600} {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != want {
			t.Fatalf("%s mode = %v, want %v", path, fi.Mode().Perm(), want)
		}
	}
}

func TestInstallKeyOverSFTPAppendsAfterALastLineWithoutNewline(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(home, ".ssh", "authorized_keys")
	other := newPublicKey(t)
	if err := os.WriteFile(file, []byte(other), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := startFakeServer(t, func(f *fakeServer) { f.sftpDir = home })
	pub := newPublicKey(t)
	if err := install(t, srv.target(t, false), testPassword, pub); err != nil {
		t.Fatalf("InstallKey: %v", err)
	}
	got, _ := os.ReadFile(file) //nolint:gosec // a test file
	if string(got) != other+"\n"+pub+"\n" {
		t.Fatalf("authorized_keys = %q", got)
	}
}

func TestInstallKeyOverSFTPFindsTheKeyBehindOptions(t *testing.T) {
	home := t.TempDir()
	pub := newPublicKey(t)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(home, ".ssh", "authorized_keys")
	have := `no-pty,command="/bin/false" ` + strings.TrimSuffix(pub, " bombvault") + " old comment\n"
	if err := os.WriteFile(file, []byte(have), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := startFakeServer(t, func(f *fakeServer) { f.sftpDir = home })
	if err := install(t, srv.target(t, false), testPassword, pub); err != nil {
		t.Fatalf("InstallKey: %v", err)
	}
	if got, _ := os.ReadFile(file); string(got) != have { //nolint:gosec // a test file
		t.Fatalf("authorized_keys = %q, want it left as it was", got)
	}
}

func TestInstallKeyFallsBackToTheShellWithoutSFTP(t *testing.T) {
	srv := startFakeServer(t, func(f *fakeServer) {
		f.run = func(string, []byte) (string, uint32) { return "", 0 }
	})
	pub := newPublicKey(t)
	if err := install(t, srv.target(t, false), testPassword, pub); err != nil {
		t.Fatalf("InstallKey: %v", err)
	}
	if commands, _ := srv.ran(); len(commands) != 1 || !strings.Contains(commands[0], shellQuote(pub)) {
		t.Fatalf("commands = %q, want one script carrying the key", commands)
	}
}

func TestInstallKeyRefusesAServerWithNeitherSFTPNorAShell(t *testing.T) {
	noExec := startFakeServer(t, nil)
	noShell := startFakeServer(t, func(f *fakeServer) {
		f.run = func(string, []byte) (string, uint32) { return "sh: mkdir: not found\n", 127 }
	})
	for _, srv := range []*fakeServer{noExec, noShell} {
		if err := install(t, srv.target(t, false), testPassword, newPublicKey(t)); !errors.Is(err, ErrNoWayIn) {
			t.Fatalf("err = %v, want ErrNoWayIn", err)
		}
	}
}

func TestShellAppendScriptAddsTheKeyOnce(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	signer := newSigner(t)
	pub := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))) + " bombvault"
	other := newPublicKey(t)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(home, ".ssh", "authorized_keys")
	if err := os.WriteFile(file, []byte(other), 0o600); err != nil {
		t.Fatal(err)
	}
	script := shellAppendScript(signer.PublicKey(), pub)
	for range 2 {
		cmd := exec.Command(sh, "-c", script) //nolint:gosec // the script under test
		cmd.Env = append(os.Environ(), "HOME="+home)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("script: %v: %s", err, out)
		}
	}
	got, _ := os.ReadFile(file) //nolint:gosec // a test file
	if string(got) != other+"\n"+pub+"\n" {
		t.Fatalf("authorized_keys = %q", got)
	}
}
