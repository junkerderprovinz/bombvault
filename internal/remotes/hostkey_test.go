package remotes

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// sshServer listens on a free local port with a fresh ed25519 host key and
// accepts nobody. It returns its address and the key it proves itself with.
func sshServer(t *testing.T) (host, port string, key ssh.PublicKey) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, net.ErrClosed },
	}
	config.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_, _, _, _ = ssh.NewServerConn(c, config)
				_ = c.Close()
			}()
		}
	}()
	host, port, _ = net.SplitHostPort(ln.Addr().String())
	return host, port, signer.PublicKey()
}

func TestTheHostKeyIsReadInTheFormRclonePinsIt(t *testing.T) {
	host, port, key := sshServer(t)

	got, err := ReadHostKey(context.Background(), host, port)
	if err != nil {
		t.Fatal(err)
	}

	if want := "ssh-ed25519 " + base64.StdEncoding.EncodeToString(key.Marshal()); got.Pin != want {
		t.Fatalf("pin = %q, want %q", got.Pin, want)
	}
	if got.Fingerprint != ssh.FingerprintSHA256(key) || !strings.HasPrefix(got.Fingerprint, "SHA256:") {
		t.Fatalf("fingerprint = %q, want %q", got.Fingerprint, ssh.FingerprintSHA256(key))
	}
}

func TestAHostWithoutAnSSHServerHasNoHostKey(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	_ = ln.Close()

	if _, err := ReadHostKey(context.Background(), host, port); err == nil {
		t.Fatal("a closed port gave a host key")
	}
}
