package sshconn

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureKeyGeneratesAndReuses(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}
	dir := t.TempDir()
	c := &Conn{Host: "host.docker.internal", User: "root", dir: filepath.Join(dir, "ssh")}

	if err := c.EnsureKey(); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	pub, err := c.PublicKey()
	if err != nil || !strings.HasPrefix(pub, "ssh-ed25519 ") {
		t.Fatalf("PublicKey = %q, err=%v", pub, err)
	}
	first, _ := os.ReadFile(c.keyPath())
	if err := c.EnsureKey(); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(c.keyPath())
	if string(first) != string(second) {
		t.Fatal("EnsureKey regenerated the key instead of reusing it")
	}
}

func TestVirshURI(t *testing.T) {
	c := &Conn{Host: "1.2.3.4", User: "root", Port: "1004", dir: "/config/ssh"}
	got := c.VirshURI()
	want := "qemu+ssh://root@1.2.3.4:1004/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto"
	if got != want {
		t.Fatalf("VirshURI = %q, want %q", got, want)
	}
}

// TestVirshURIExplicitOverrideReturnsVerbatim uses a TrueNAS Scale URI, whose
// ?socket= parameter the built URI cannot express.
func TestVirshURIExplicitOverrideReturnsVerbatim(t *testing.T) {
	override := "qemu+ssh://root@truenas.local/system?socket=/run/truenas_libvirt/libvirt-sock"
	c := &Conn{Host: "1.2.3.4", User: "root", Port: "1004", dir: "/config/ssh", explicitURI: override}
	if got := c.VirshURI(); got != override {
		t.Fatalf("VirshURI = %q, want the configured override %q verbatim", got, override)
	}
}

func TestNewDerivesSSHDir(t *testing.T) {
	c := New("h", "root", "22", "/config", "")
	if c.dir != filepath.Join("/config", "ssh") {
		t.Fatalf("dir = %q, want /config/ssh", c.dir)
	}
}

func TestNewPassesExplicitURI(t *testing.T) {
	override := "qemu+ssh://root@truenas.local/system?socket=/run/truenas_libvirt/libvirt-sock"
	withOverride := New("truenas.local", "root", "22", "/config", override)
	if got := withOverride.VirshURI(); got != override {
		t.Fatalf("VirshURI() = %q, want override %q verbatim", got, override)
	}

	withoutOverride := New("host.docker.internal", "root", "22", "/config", "")
	wantDefault := "qemu+ssh://root@host.docker.internal:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto"
	if got := withoutOverride.VirshURI(); got != wantDefault {
		t.Fatalf("VirshURI() = %q, want %q (today's built string, unset override)", got, wantDefault)
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"/etc/libvirt/qemu/nvram/Windows 11_VARS.fd": `'/etc/libvirt/qemu/nvram/Windows 11_VARS.fd'`,
		"/plain/path.fd": `'/plain/path.fd'`,
		"true":           `'true'`,
		"a'b":            `'a'\''b'`,
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
