package api_test

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
)

func TestEnsureSelfSignedGeneratesAndReuses(t *testing.T) {
	dir := t.TempDir()

	cert1, key1, err := api.EnsureSelfSigned(dir)
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	// Files exist and load as a valid TLS keypair.
	if _, err := tls.LoadX509KeyPair(cert1, key1); err != nil {
		t.Fatalf("generated keypair invalid: %v", err)
	}

	// Capture contents to prove reuse (not regeneration) on the second call.
	keyBefore, err := os.ReadFile(key1) //nolint:gosec // G304: key1 is a path returned by EnsureSelfSigned under the test's own TempDir, not user input
	if err != nil {
		t.Fatal(err)
	}

	cert2, key2, err := api.EnsureSelfSigned(dir)
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if cert1 != cert2 || key1 != key2 {
		t.Fatalf("paths changed between calls")
	}
	keyAfter, err := os.ReadFile(key2) //nolint:gosec // G304: key2 is a path returned by EnsureSelfSigned under the test's own TempDir, not user input
	if err != nil {
		t.Fatal(err)
	}
	if string(keyBefore) != string(keyAfter) {
		t.Fatal("key was regenerated; expected reuse of the existing cert")
	}
}

func TestEnsureSelfSignedKeyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file-mode semantics not enforced on windows")
	}
	dir := t.TempDir()
	_, key, err := api.EnsureSelfSigned(dir)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(key)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key mode = %o, want 600", info.Mode().Perm())
	}
}

func TestEnsureSelfSignedWritesUnderCertsDir(t *testing.T) {
	dir := t.TempDir()
	cert, key, err := api.EnsureSelfSigned(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantCert := filepath.Join(dir, "certs", "cert.pem")
	wantKey := filepath.Join(dir, "certs", "key.pem")
	if cert != wantCert {
		t.Fatalf("cert path = %q, want %q", cert, wantCert)
	}
	if key != wantKey {
		t.Fatalf("key path = %q, want %q", key, wantKey)
	}
}
