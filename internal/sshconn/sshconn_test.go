package sshconn

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

func TestIsolatedConnKeepsAKeyOfItsOwn(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}
	dataDir := t.TempDir()
	host := New("192.168.1.10", "root", "1004", dataDir, "")
	replica := NewIsolated("backup.lan", "root", "22", filepath.Join(dataDir, "ssh-replica", "t1"), "")
	for _, c := range []*Conn{host, replica} {
		if err := c.EnsureKey(); err != nil {
			t.Fatalf("EnsureKey: %v", err)
		}
	}
	hostPub, err := host.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	replicaPub, err := replica.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	if hostPub == replicaPub {
		t.Fatal("the replica Conn reuses the libvirt host's key")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "ssh-replica", "t1", "id_ed25519")); err != nil {
		t.Errorf("the replica key is not in its own directory: %v", err)
	}
}

func TestConnsSharingAKeyDirCreateOneKeyAtOnce(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}
	dir := t.TempDir()
	conns := make([]*Conn, 8)
	for i := range conns {
		conns[i] = NewIsolated("backup.lan", "replica", "", dir, filepath.Join(dir, "known_hosts", strconv.Itoa(i)))
	}
	var wg sync.WaitGroup
	errs := make([]error, len(conns))
	for i, c := range conns {
		wg.Go(func() { errs[i] = c.EnsureKey() })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("EnsureKey %d: %v", i, err)
		}
	}
	derived, err := exec.Command("ssh-keygen", "-y", "-f", conns[0].keyPath()).Output() //nolint:gosec // G204: the key the test just made
	if err != nil {
		t.Fatal(err)
	}
	pub, err := conns[0].PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Fields(string(derived)); len(want) < 2 || !strings.HasPrefix(pub, want[0]+" "+want[1]) {
		t.Fatalf("the public key %q does not belong to the private key (%q)", pub, derived)
	}
}
