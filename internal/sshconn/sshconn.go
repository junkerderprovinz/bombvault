// Package sshconn manages BombVault's SSH access to the libvirt host (Unraid,
// TrueNAS Scale, or a generic Docker host running libvirtd), used for virsh
// over qemu+ssh:// and for copying NVRAM files. No libvirt path is
// bind-mounted, so the container cannot interfere with the host's own VM
// manager.
package sshconn

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Conn holds the SSH identity and target for reaching the host's libvirt.
type Conn struct {
	Host string // e.g. "host.docker.internal"
	User string // e.g. "root"
	Port string // SSH port on the host, e.g. "22" or "1004"
	dir  string // <dataDir>/ssh

	// explicitURI replaces the URI VirshURI would build; see VirshURI.
	explicitURI string
}

// New returns a Conn that keeps its key under dataDir/ssh. An empty port
// defaults to 22. A non-empty explicitURI (LIBVIRT_URI) replaces the URI
// VirshURI would build from host, user and port.
func New(host, user, port, dataDir, explicitURI string) *Conn {
	if port == "" {
		port = "22"
	}
	return &Conn{Host: host, User: user, Port: port, dir: filepath.Join(dataDir, "ssh"), explicitURI: explicitURI}
}

func (c *Conn) keyPath() string        { return filepath.Join(c.dir, "id_ed25519") }
func (c *Conn) knownHostsPath() string { return filepath.Join(c.dir, "known_hosts") }

// EnsureKey generates an ed25519 keypair on first use and reuses it thereafter.
func (c *Conn) EnsureKey() error {
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("sshconn: mkdir: %w", err)
	}
	if _, err := os.Stat(c.keyPath()); err == nil {
		return nil
	}
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-C", "bombvault", "-f", c.keyPath()) //nolint:gosec // fixed args, no user input
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sshconn: keygen: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// PublicKey returns the authorized_keys line to add on the host.
func (c *Conn) PublicKey() (string, error) {
	b, err := os.ReadFile(c.keyPath() + ".pub")
	if err != nil {
		return "", fmt.Errorf("sshconn: read pubkey: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// VirshURI is the libvirt connection URI for `virsh -c`. A configured
// explicitURI is returned as is: TrueNAS Scale runs libvirtd on
// /run/truenas_libvirt/libvirt-sock, which needs a ?socket= parameter the
// built form cannot express (see docs/vm-backup-ssh-setup.md).
//
// The key and known_hosts paths are container paths, so ToSlash only matters
// for tests on Windows. known_hosts_verify=auto pins the host key on first
// connect without a prompt; "normal" would hang the non-interactive virsh call.
func (c *Conn) VirshURI() string {
	if c.explicitURI != "" {
		return c.explicitURI
	}
	return fmt.Sprintf("qemu+ssh://%s@%s:%s/system?keyfile=%s&known_hosts=%s&known_hosts_verify=auto",
		c.User, c.Host, c.Port, filepath.ToSlash(c.keyPath()), filepath.ToSlash(c.knownHostsPath()))
}

// sshArgs are the common ssh options (key, pinned known_hosts, no prompts).
// ConnectTimeout fails fast when the host is unreachable, for example from a
// macvlan/br0 container that cannot route to it.
func (c *Conn) sshArgs() []string {
	return []string{
		"-i", c.keyPath(),
		"-p", c.Port,
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		"-o", "UserKnownHostsFile=" + c.knownHostsPath(),
		c.User + "@" + c.Host,
	}
}

// shellQuote single-quotes s for the remote shell. OpenSSH joins the remote
// arguments into one string that the remote shell splits again, so the NVRAM
// path of a VM named "Windows 11" would otherwise arrive as two arguments. An
// embedded ' is closed, escaped and reopened.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// sshExec builds the full ssh argv: the connection options, "--", then the
// remote command with every token shell-quoted.
func (c *Conn) sshExec(remote ...string) []string {
	full := append(c.sshArgs(), "--")
	for _, a := range remote {
		full = append(full, shellQuote(a))
	}
	return full
}

// WriteSSHConfig writes ~/.ssh/config for libvirt builds whose qemu+ssh
// transport runs the external ssh binary (Unraid among them). That binary
// ignores the URI's keyfile and known_hosts parameters, so without this file
// it falls back to the empty ~/.ssh defaults with strict checking and virsh
// fails with "Host key verification failed".
func (c *Conn) WriteSSHConfig() error {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "/root"
	}
	dir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("sshconn: mkdir %s: %w", dir, err)
	}
	cfg := fmt.Sprintf("Host *\n  IdentityFile %s\n  UserKnownHostsFile %s\n  StrictHostKeyChecking accept-new\n  BatchMode yes\n  ConnectTimeout 10\n",
		filepath.ToSlash(c.keyPath()), filepath.ToSlash(c.knownHostsPath()))
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte(cfg), 0o600); err != nil {
		return fmt.Errorf("sshconn: write %s/config: %w", dir, err)
	}
	return nil
}

// EnsureKnownHost opens a throwaway SSH connection so the host key is in
// known_hosts before libvirt checks it. Some libvirt builds (e.g. Unraid 12.2)
// do not fill known_hosts even with known_hosts_verify=auto and then fail on
// the empty file. It also confirms that key authentication works.
func (c *Conn) EnsureKnownHost(ctx context.Context) error {
	if _, err := c.Run(ctx, "true"); err != nil {
		return fmt.Errorf("ssh to %s@%s:%s failed (key authorized? host reachable?): %w", c.User, c.Host, c.Port, err)
	}
	return nil
}

// Run executes a command on the host over SSH and returns trimmed stdout. The
// output is returned even when the command fails, so a caller can show it
// next to the error.
func (c *Conn) Run(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "ssh", c.sshExec(args...)...).Output() //nolint:gosec // remote args shell-quoted; host/user from config
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("sshconn: run %q: %w", args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}

// sshBinary is a var so a test can put a stand-in in its place; the argv
// sshExec builds starts with -i, which a go test helper process would take for
// one of its own flags.
var sshBinary = "ssh"

const (
	captureStdoutLimit = 16 << 20
	captureStderrLimit = 64 << 10
)

// RunCapture executes a command on the host over SSH and returns its two
// streams apart. Run discards stderr, which leaves a remote "permission
// denied" as a bare exit status; the ZFS domain reads its reason codes out of
// that text.
func (c *Conn) RunCapture(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, sshBinary, c.sshExec(args...)...) //nolint:gosec // remote args shell-quoted; host/user from config
	stdout := &cappedBuffer{max: captureStdoutLimit}
	stderr := &cappedBuffer{max: captureStderrLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	out, errOut := strings.TrimSpace(stdout.buf.String()), strings.TrimSpace(stderr.buf.String())
	if err != nil {
		return out, errOut, fmt.Errorf("sshconn: run %q: %w", args[0], err)
	}
	if stdout.cut {
		return out, errOut, fmt.Errorf("sshconn: run %q: %w", args[0], ErrStdoutCut)
	}
	return out, errOut, nil
}

// ErrStdoutCut is what RunCapture returns when stdout ran past its limit. The
// trimmed output cannot show a cut that fell on a line boundary, so a caller
// parsing a listing has to hear it from here.
var ErrStdoutCut = errors.New("the output is larger than the 16 MiB capture limit")

// cappedBuffer keeps the first max bytes and reports the rest as written, so a
// remote command that floods a stream neither fills the container's memory nor
// dies of a broken pipe.
type cappedBuffer struct {
	buf bytes.Buffer
	max int
	cut bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	room := b.max - b.buf.Len()
	if n > room {
		b.cut = true
		p = p[:max(room, 0)]
	}
	b.buf.Write(p) //nolint:errcheck // bytes.Buffer.Write never fails
	return n, nil
}

// ReadFile returns the bytes of a file on the host (used for NVRAM).
func (c *Conn) ReadFile(ctx context.Context, path string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "ssh", c.sshExec("cat", path)...).Output() //nolint:gosec // remote args shell-quoted
	if err != nil {
		return nil, fmt.Errorf("sshconn: read %q: %w", filepath.Base(path), err)
	}
	return out, nil
}

// WriteFile writes data to a file on the host (used to restore NVRAM) by
// piping it to `tee <path>` over SSH. libvirt owns the nvram directory, so it
// already exists.
func (c *Conn) WriteFile(ctx context.Context, path string, data []byte) error {
	cmd := exec.CommandContext(ctx, "ssh", c.sshExec("tee", path)...) //nolint:gosec // remote args shell-quoted
	cmd.Stdin = bytes.NewReader(data)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil { // stdout (tee's echo) is discarded
		return fmt.Errorf("sshconn: write %q: %s", filepath.Base(path), strings.TrimSpace(stderr.String()))
	}
	return nil
}

// StreamCommand starts a command on the host over SSH and returns its stdout
// as a stream, for output too large for Run to buffer, such as a `zfs send` of
// a zvol VM disk piped into restic. The caller must call wait exactly once
// after reading, whether reading succeeded or not, to reap ssh and get its
// exit status.
func (c *Conn) StreamCommand(ctx context.Context, args ...string) (io.ReadCloser, func() error, error) {
	cmd := exec.CommandContext(ctx, "ssh", c.sshExec(args...)...) //nolint:gosec // remote args shell-quoted
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("sshconn: stdout pipe for %q: %w", args[0], err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("sshconn: start %q: %w", args[0], err)
	}
	wait := func() error {
		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("sshconn: run %q: %s", args[0], strings.TrimSpace(stderr.String()))
		}
		return nil
	}
	return stdout, wait, nil
}

// RunWithStdin runs a command on the host over SSH with stdin streamed from rd,
// such as a restic dump piped into `zfs receive`. It is the restore-side
// counterpart of StreamCommand and blocks until the remote command exits.
func (c *Conn) RunWithStdin(ctx context.Context, rd io.Reader, args ...string) error {
	cmd := exec.CommandContext(ctx, "ssh", c.sshExec(args...)...) //nolint:gosec // remote args shell-quoted
	cmd.Stdin = rd
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil { // stdout is discarded
		return fmt.Errorf("sshconn: run %q: %s", args[0], strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Test verifies the SSH path reaches libvirt: runs `virsh -c <uri> list --all`.
func (c *Conn) Test(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "virsh", "-c", c.VirshURI(), "list", "--all").CombinedOutput() //nolint:gosec // uri from config
	if err != nil {
		return fmt.Errorf("libvirt over SSH not reachable: %s", lastLine(string(out)))
	}
	return nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return "unknown error"
}
