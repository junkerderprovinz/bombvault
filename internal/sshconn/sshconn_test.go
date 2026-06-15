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
	// Reuse: a second call must keep the same key.
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
	c := &Conn{Host: "1.2.3.4", User: "root", dir: "/config/ssh"}
	got := c.VirshURI()
	want := "qemu+ssh://root@1.2.3.4/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=normal"
	if got != want {
		t.Fatalf("VirshURI = %q, want %q", got, want)
	}
}

func TestNewDerivesSSHDir(t *testing.T) {
	c := New("h", "root", "/config")
	if c.dir != filepath.Join("/config", "ssh") {
		t.Fatalf("dir = %q, want /config/ssh", c.dir)
	}
}
