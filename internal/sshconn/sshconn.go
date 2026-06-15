// Package sshconn manages BombVault's SSH access to the Unraid host for libvirt
// control (qemu+ssh://) and NVRAM file transfer. No libvirt path is ever
// bind-mounted; the container runs virsh ON the host over SSH, so it can never
// interfere with the host VM Manager's lifecycle.
package sshconn

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Conn holds the SSH identity + target for reaching the host's libvirt.
type Conn struct {
	Host string // e.g. "host.docker.internal"
	User string // e.g. "root"
	dir  string // <dataDir>/ssh
}

// New returns a Conn storing its key material under dataDir/ssh.
func New(host, user, dataDir string) *Conn {
	return &Conn{Host: host, User: user, dir: filepath.Join(dataDir, "ssh")}
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

// VirshURI is the libvirt connection URI for `virsh -c`. The keyfile/known_hosts
// are container (Linux) paths, so they are always forward-slash (ToSlash is a
// no-op on the Linux runtime target; it only matters for tests on Windows).
// known_hosts_verify=auto accepts + pins the host key on first connect WITHOUT
// an interactive prompt — `normal` would hang the (non-interactive) virsh call
// the first time the host key is unknown.
func (c *Conn) VirshURI() string {
	return fmt.Sprintf("qemu+ssh://%s@%s/system?keyfile=%s&known_hosts=%s&known_hosts_verify=auto",
		c.User, c.Host, filepath.ToSlash(c.keyPath()), filepath.ToSlash(c.knownHostsPath()))
}

// sshArgs are the common ssh options (key, pinned known_hosts, no prompts).
// ConnectTimeout fails fast instead of hanging when the host is unreachable
// (e.g. a macvlan/br0 container that cannot route to the host).
func (c *Conn) sshArgs() []string {
	return []string{
		"-i", c.keyPath(),
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		"-o", "UserKnownHostsFile=" + c.knownHostsPath(),
		c.User + "@" + c.Host,
	}
}

// Run executes a command on the host over SSH and returns trimmed stdout.
func (c *Conn) Run(ctx context.Context, args ...string) (string, error) {
	full := append(c.sshArgs(), append([]string{"--"}, args...)...)
	out, err := exec.CommandContext(ctx, "ssh", full...).Output() //nolint:gosec // argv-separated; host/user from config
	if err != nil {
		return "", fmt.Errorf("sshconn: run %q: %w", args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ReadFile returns the bytes of a file on the host (used for NVRAM).
func (c *Conn) ReadFile(ctx context.Context, path string) ([]byte, error) {
	full := append(c.sshArgs(), "--", "cat", path)
	out, err := exec.CommandContext(ctx, "ssh", full...).Output() //nolint:gosec // argv-separated
	if err != nil {
		return nil, fmt.Errorf("sshconn: read %q: %w", filepath.Base(path), err)
	}
	return out, nil
}

// WriteFile writes data to a file on the host (used to restore NVRAM) by piping
// it to `tee <path>` over SSH. tee takes the path as a separate argv argument —
// no remote shell, so the path can never be interpreted (defence even though
// nvram paths come from libvirt, not the user). The nvram directory already
// exists on the host (libvirt owns it), so no mkdir is needed.
func (c *Conn) WriteFile(ctx context.Context, path string, data []byte) error {
	full := append(c.sshArgs(), "--", "tee", path)
	cmd := exec.CommandContext(ctx, "ssh", full...) //nolint:gosec // argv-separated; no shell
	cmd.Stdin = bytes.NewReader(data)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil { // stdout (tee's echo) is discarded
		return fmt.Errorf("sshconn: write %q: %s", filepath.Base(path), strings.TrimSpace(stderr.String()))
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
