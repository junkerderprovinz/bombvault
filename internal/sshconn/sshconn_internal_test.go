package sshconn

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testConn(t *testing.T) *Conn {
	t.Helper()
	return New("nas.local", "root", "1004", t.TempDir(), "")
}

func TestSSHArgsCarryRequiredOptions(t *testing.T) {
	c := testConn(t)
	args := c.sshArgs()
	joined := strings.Join(args, " ")

	for _, want := range []string{
		// Without BatchMode the client prompts for a password and the call
		// hangs until its context dies, which looks like a timeout rather
		// than an unauthorized key.
		"BatchMode=yes",
		// Strict checking fails on an unknown key with "Host key verification
		// failed", which is what libvirt reports on Unraid when nothing has
		// pinned the key yet.
		"StrictHostKeyChecking=accept-new",
		// A container on br0 that cannot route to the host would otherwise
		// hang for the OS default of several minutes.
		"ConnectTimeout=10",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("sshArgs lost %q: %v", want, args)
		}
	}

	// ~/.ssh/known_hosts works on a developer machine and fails in the
	// container.
	if !strings.Contains(joined, filepath.ToSlash(c.knownHostsPath())) &&
		!strings.Contains(joined, c.knownHostsPath()) {
		t.Errorf("sshArgs does not pin our own known_hosts: %v", args)
	}
	if !strings.Contains(joined, c.keyPath()) {
		t.Errorf("sshArgs does not pass our own key: %v", args)
	}

	// ssh reads anything after the destination as the remote command.
	if got := args[len(args)-1]; got != "root@nas.local" {
		t.Errorf("last arg = %q, want the destination", got)
	}
	if !strings.Contains(joined, "1004") {
		t.Errorf("sshArgs dropped the port: %v", args)
	}
}

func TestSSHExecQuotesRemoteCommand(t *testing.T) {
	c := testConn(t)
	// A VM called "Windows 11" gives an NVRAM path with a space in it.
	args := c.sshExec("cp", "/etc/libvirt/qemu/nvram/Windows 11_VARS.fd", "/tmp/x")

	sep := -1
	for i, a := range args {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep < 0 {
		t.Fatalf("no -- end-of-options marker: %v", args)
	}
	remote := args[sep+1:]
	if len(remote) != 3 {
		t.Fatalf("remote command has %d tokens, want 3: %v", len(remote), remote)
	}
	if remote[1] != `'/etc/libvirt/qemu/nvram/Windows 11_VARS.fd'` {
		t.Errorf("path not quoted for the remote shell: %q", remote[1])
	}

	got := c.sshExec("echo", "it's")
	if last := got[len(got)-1]; last != `'it'\''s'` {
		t.Errorf("embedded quote mishandled: %q", last)
	}
}

// TestSSHExecDoesNotMutateSSHArgs guards the append in sshExec: if the slice
// sshArgs returns had spare capacity, a second call would overwrite the first
// command's tail.
func TestSSHExecDoesNotMutateSSHArgs(t *testing.T) {
	c := testConn(t)
	first := c.sshExec("one")
	second := c.sshExec("two")
	if first[len(first)-1] != "'one'" {
		t.Errorf("the first command was rewritten by the second: %v", first)
	}
	if second[len(second)-1] != "'two'" {
		t.Errorf("second command wrong: %v", second)
	}
}

func TestWriteSSHConfigUsesOwnKeyAndKnownHosts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir reads this one on Windows

	c := testConn(t)
	if err := c.WriteSSHConfig(); err != nil {
		t.Fatalf("WriteSSHConfig: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".ssh", "config")) //nolint:gosec // G304: home is this test's own t.TempDir(), not input
	if err != nil {
		t.Fatalf("no config written: %v", err)
	}
	cfg := string(raw)
	for _, want := range []string{
		"Host *",
		filepath.ToSlash(c.keyPath()),
		filepath.ToSlash(c.knownHostsPath()),
		"StrictHostKeyChecking acc",
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("ssh config missing %q:\n%s", want, cfg)
		}
	}
}

func TestLastLine(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"plain", "boom", "boom"},
		{"trailing newline", "one\ntwo\n", "two"},
		{"trailing blank lines", "one\ntwo\n\n  \n", "two"},
		{"leading noise kept out", "warning: x\nreal error", "real error"},
		{"whitespace trimmed", "  padded  ", "padded"},
		{"empty falls back", "", "unknown error"},
		{"only whitespace falls back", "\n  \n\t\n", "unknown error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := lastLine(tc.in); got != tc.want {
				t.Errorf("lastLine(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCappedBufferReportsTheCut(t *testing.T) {
	full := &cappedBuffer{max: 4}
	if _, err := full.Write([]byte("abcd")); err != nil || full.cut {
		t.Fatalf("a write that fits exactly: err %v, cut %v", err, full.cut)
	}
	if n, err := full.Write([]byte("\n")); err != nil || n != 1 || !full.cut {
		t.Fatalf("a byte past the limit: n %d, err %v, cut %v", n, err, full.cut)
	}
	if full.buf.String() != "abcd" {
		t.Fatalf("kept %q, want the first four bytes", full.buf.String())
	}
}

// TestRunCaptureFailsWhenStdoutIsCut floods stdout with whole lines, so the
// cut lands on a line boundary and the trimmed output looks complete.
func TestRunCaptureFailsWhenStdoutIsCut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}
	script := filepath.Join(t.TempDir(), "fake-ssh")
	body := "#!/bin/sh\nyes 'cache/appdata/child' | head -n 2000000\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil { //nolint:gosec // G306: an executable stand-in for ssh
		t.Fatal(err)
	}
	old := sshBinary
	sshBinary = script
	t.Cleanup(func() { sshBinary = old })

	_, _, err := testConn(t).RunCapture(context.Background(), "zfs", "list")
	if !errors.Is(err, ErrStdoutCut) {
		t.Fatalf("err = %v, want ErrStdoutCut", err)
	}
}

// TestRunCaptureKeepsStdoutAndStderrApart points sshBinary at a script that
// writes to both streams and fails, which is what a zfs call looks like when
// the remote user may not snapshot.
func TestRunCaptureKeepsStdoutAndStderrApart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}
	script := filepath.Join(t.TempDir(), "fake-ssh")
	body := "#!/bin/sh\necho out-line\necho err-line >&2\nexit 1\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil { //nolint:gosec // G306: an executable stand-in for ssh
		t.Fatal(err)
	}
	old := sshBinary
	sshBinary = script
	t.Cleanup(func() { sshBinary = old })

	c := testConn(t)
	stdout, stderr, err := c.RunCapture(context.Background(), "zfs", "snapshot", "cache/appdata@bombvault-20260917031500")
	if err == nil {
		t.Fatal("RunCapture reported success for a command that exited 1")
	}
	if stdout != "out-line" {
		t.Errorf("stdout = %q", stdout)
	}
	if stderr != "err-line" {
		t.Errorf("stderr = %q", stderr)
	}
}
