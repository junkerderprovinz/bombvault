// Package sshconn manages BombVault's SSH access to the libvirt host (Unraid,
// TrueNAS Scale, or a generic Docker host with a reachable libvirtd) for
// libvirt control (qemu+ssh://) and NVRAM file transfer. No libvirt path is
// ever bind-mounted; the container runs virsh ON the host over SSH, so it can
// never interfere with the host's own VM Manager's lifecycle.
package sshconn

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Conn holds the SSH identity + target for reaching the host's libvirt.
type Conn struct {
	Host string // e.g. "host.docker.internal"
	User string // e.g. "root"
	Port string // SSH port on the host, e.g. "22" or "1004"
	dir  string // <dataDir>/ssh

	// explicitURI, when non-empty, is returned verbatim by VirshURI instead
	// of the built qemu+ssh://... string. See VirshURI's doc comment.
	explicitURI string
}

// New returns a Conn storing its key material under dataDir/ssh. An empty port
// defaults to 22. explicitURI, when non-empty (config.Config.LibvirtURI, the
// LIBVIRT_URI env var), is returned verbatim by VirshURI instead of building
// the qemu+ssh://... string from host/user/port — see VirshURI's doc comment.
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
		return nil // already present
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

// VirshURI is the libvirt connection URI for `virsh -c`. When explicitURI is
// set (config.Config.LibvirtURI, the LIBVIRT_URI env var), it is returned
// VERBATIM instead of the built string below — this exists for TrueNAS
// Scale, whose libvirtd runs on a non-standard socket
// (/run/truenas_libvirt/libvirt-sock), needing an extra ?socket=... query
// param the built qemu+ssh:// form below has no way to express (see
// docs/superpowers/specs/2026-08-16-bombvault-platform-expansion-design.md
// §5 and docs/vm-backup-ssh-setup.md's "TrueNAS Scale" section for the exact
// value to set: qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_
// libvirt/libvirt-sock). Unset (the default) reproduces today's Unraid/
// generic behavior exactly.
//
// The built form's keyfile/known_hosts are container (Linux) paths, so they
// are always forward-slash (ToSlash is a no-op on the Linux runtime target;
// it only matters for tests on Windows). known_hosts_verify=auto accepts +
// pins the host key on first connect WITHOUT an interactive prompt — `normal`
// would hang the (non-interactive) virsh call the first time the host key is
// unknown.
func (c *Conn) VirshURI() string {
	if c.explicitURI != "" {
		return c.explicitURI
	}
	return fmt.Sprintf("qemu+ssh://%s@%s:%s/system?keyfile=%s&known_hosts=%s&known_hosts_verify=auto",
		c.User, c.Host, c.Port, filepath.ToSlash(c.keyPath()), filepath.ToSlash(c.knownHostsPath()))
}

// sshArgs are the common ssh options (key, pinned known_hosts, no prompts).
// ConnectTimeout fails fast instead of hanging when the host is unreachable
// (e.g. a macvlan/br0 container that cannot route to the host).
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

// shellQuote single-quotes s so it survives the REMOTE shell. OpenSSH joins the
// remote command args into one string that the remote `$SHELL -c` re-splits, so
// an unquoted path with a space — e.g. the NVRAM file of a VM named
// "Windows 11" (…/Windows 11_VARS.fd) — would break into two args. Single quotes
// are literal in sh; an embedded ' is closed, escaped, and reopened.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// sshExec builds the full ssh argv: the connection options, the "--" end-of-
// options marker, then the remote command with every token shell-quoted.
func (c *Conn) sshExec(remote ...string) []string {
	full := append(c.sshArgs(), "--")
	for _, a := range remote {
		full = append(full, shellQuote(a))
	}
	return full
}

// WriteSSHConfig writes ~/.ssh/config so libvirt's qemu+ssh transport — which
// uses the EXTERNAL ssh binary and ignores the URI's keyfile/known_hosts/
// known_hosts_verify params — still picks up our key, our known_hosts, and an
// accept-new host-key policy. Without this, virsh fails with "Host key
// verification failed" because the bare ssh binary uses ~/.ssh defaults (empty)
// and strict checking. (libssh/libssh2 builds honour the URI params instead;
// this covers the ssh-binary builds, e.g. Unraid.)
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

// EnsureKnownHost opens a throwaway SSH connection so the host key is pinned in
// known_hosts BEFORE libvirt's qemu+ssh transport verifies it. Some libvirt
// builds (e.g. Unraid 12.2) do NOT self-populate known_hosts even with
// known_hosts_verify=auto, so virsh fails with "Host key verification failed"
// on an empty file. The raw ssh client uses StrictHostKeyChecking=accept-new,
// which auto-accepts + writes the key on first use; afterwards virsh's auto
// verify finds the entry and connects. This also confirms key auth works.
func (c *Conn) EnsureKnownHost(ctx context.Context) error {
	if _, err := c.Run(ctx, "true"); err != nil {
		return fmt.Errorf("ssh to %s@%s:%s failed (key authorized? host reachable?): %w", c.User, c.Host, c.Port, err)
	}
	return nil
}

// Run executes a command on the host over SSH and returns trimmed stdout.
// The collected stdout is returned even when the command exits non-zero, so a
// caller can surface the failing command's output (e.g. the tail of a `plugin
// install` transcript) alongside the error.
func (c *Conn) Run(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "ssh", c.sshExec(args...)...).Output() //nolint:gosec // remote args shell-quoted; host/user from config
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("sshconn: run %q: %w", args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ReadFile returns the bytes of a file on the host (used for NVRAM).
func (c *Conn) ReadFile(ctx context.Context, path string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "ssh", c.sshExec("cat", path)...).Output() //nolint:gosec // remote args shell-quoted
	if err != nil {
		return nil, fmt.Errorf("sshconn: read %q: %w", filepath.Base(path), err)
	}
	return out, nil
}

// WriteFile writes data to a file on the host (used to restore NVRAM) by piping
// it to `tee <path>` over SSH. The path is shell-quoted (sshExec) so it survives
// the remote shell even with spaces (e.g. an NVRAM file from a VM named
// "Windows 11"). The nvram directory already exists on the host (libvirt owns
// it), so no mkdir is needed.
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
// as a stream, plus a wait function the caller MUST call exactly once after
// it is done reading (success or failure) to reap the ssh process and surface
// any non-zero exit. Unlike Run (which buffers the ENTIRE output in memory
// via .Output()), this never buffers — it exists for a command whose output
// can be many gigabytes, which Run would be unsafe for.
//
// Used by the zvol VM-disk backup path (internal/backup/vm_orchestrator.go,
// v8.0.0 TrueNAS platform expansion Task 10) to stream a remote `zfs send`
// straight into a restic backup with no local staging file. ⚠ This method
// itself is structurally identical to Run/ReadFile/WriteFile above (same
// sshExec plumbing, just wired to a pipe instead of .Output()/bytes.Reader)
// and is not independently tested against a real SSH server here, matching
// this file's existing convention — see zvol.go's package doc comment for the
// "reasoned from documentation, unverified against real hardware" caveat that
// applies to its actual callers.
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

// RunWithStdin runs a command on the host over SSH with its stdin fed from rd,
// streamed (never buffered in memory) — the restore-side counterpart of
// StreamCommand, for piping a large restore stream (e.g. a restic dump
// output) into a remote command (e.g. `zfs receive`) without holding the
// whole stream in memory the way WriteFile's []byte parameter would. Blocks
// until the remote command exits.
//
// Same "reasoned, not independently tested against a real SSH server" note as
// StreamCommand applies — see zvol.go's package doc comment.
func (c *Conn) RunWithStdin(ctx context.Context, rd io.Reader, args ...string) error {
	cmd := exec.CommandContext(ctx, "ssh", c.sshExec(args...)...) //nolint:gosec // remote args shell-quoted
	cmd.Stdin = rd
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil { // stdout (if any) is discarded, mirrors WriteFile
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
