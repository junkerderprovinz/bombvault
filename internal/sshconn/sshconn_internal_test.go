package sshconn

// The parts of this package that need no SSH ([346]).
//
// Coverage here was 20.5%, and the first reaction to that number is the wrong
// one: most of this package dials a real host, and a test that cannot dial
// proves nothing about it. But the argument list, the remote-command assembly
// and the config file are not that. They are string building and one file
// write, and every one of them is a place where a silent mistake produces a
// connection that fails for a reason nobody can see from the error.
//
// That matters more here than the number does: this package carries VM
// backups, the Unraid notification path and the dashboard-widget install. When
// it is wrong, three unrelated features break at once and none of them says
// why.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testConn(t *testing.T) *Conn {
	t.Helper()
	return New("nas.local", "root", "1004", t.TempDir(), "")
}

func TestSSHArgsCarryEveryOptionThatMattered(t *testing.T) {
	c := testConn(t)
	args := c.sshArgs()
	joined := strings.Join(args, " ")

	// Each of these is here because leaving it out fails in a way that reads
	// as something else entirely.
	for _, want := range []string{
		// Without BatchMode the client PROMPTS for a password and the call
		// hangs until its context dies, which surfaces as a timeout rather
		// than as "the key is not authorised".
		"BatchMode=yes",
		// accept-new writes an unknown key on first use. Strict checking
		// instead fails with "Host key verification failed", which is what
		// libvirt reports on Unraid when nothing has pinned the key yet.
		"StrictHostKeyChecking=accept-new",
		// A container on br0 that cannot route to the host would otherwise
		// hang for the OS default, minutes rather than seconds.
		"ConnectTimeout=10",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("sshArgs lost %q: %v", want, args)
		}
	}

	// The known_hosts file must be the one this package owns. Falling back to
	// ~/.ssh/known_hosts is the bug the pinned path exists to prevent: it
	// works on a developer's machine and fails in the container.
	if !strings.Contains(joined, filepath.ToSlash(c.knownHostsPath())) &&
		!strings.Contains(joined, c.knownHostsPath()) {
		t.Errorf("sshArgs does not pin our own known_hosts: %v", args)
	}
	if !strings.Contains(joined, c.keyPath()) {
		t.Errorf("sshArgs does not pass our own key: %v", args)
	}

	// The destination is last, which is what ssh requires: an option after it
	// is read as part of the remote command.
	if got := args[len(args)-1]; got != "root@nas.local" {
		t.Errorf("last arg = %q, want the destination", got)
	}
	// The port comes from the Conn, not a default.
	if !strings.Contains(joined, "1004") {
		t.Errorf("sshArgs dropped the port: %v", args)
	}
}

func TestSSHExecQuotesTheRemoteCommand(t *testing.T) {
	c := testConn(t)
	// The real case this exists for: a VM called "Windows 11" gives an NVRAM
	// path with a space in it. OpenSSH joins the remote args into ONE string
	// that the remote shell re-splits, so unquoted it arrives as two.
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

	// An embedded single quote has to survive too, or the quoting that fixes
	// spaces introduces a worse break of its own.
	got := c.sshExec("echo", "it's")
	if last := got[len(got)-1]; last != `'it'\''s'` {
		t.Errorf("embedded quote mishandled: %q", last)
	}
}

func TestSSHExecDoesNotMutateSSHArgs(t *testing.T) {
	// sshExec appends to the slice sshArgs returns. If that slice ever gains
	// spare capacity, two calls would write into the same backing array and
	// the second command would inherit fragments of the first - a bug that
	// only appears under a particular call order, which is the worst kind to
	// find in production.
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

func TestWriteSSHConfigPointsAtOurOwnKeyAndHosts(t *testing.T) {
	// libvirt's qemu+ssh transport shells out to the ssh BINARY on some
	// builds, and that binary ignores the URI's key and known_hosts
	// parameters. This file is the only thing that reaches it, so a mistake
	// here is invisible until virsh reports "Host key verification failed".
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir reads this one on Windows

	c := testConn(t)
	if err := c.WriteSSHConfig(); err != nil {
		t.Fatalf("WriteSSHConfig: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".ssh", "config"))
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

func TestLastLineFindsTheRealMessage(t *testing.T) {
	// This is what turns a failed command into the text a user reads, so an
	// off-by-one here means an error dialog showing a blank line or a banner.
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
