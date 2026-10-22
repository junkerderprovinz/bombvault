package trust

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAddKeepsTheSystemFoldersBesideTheCertsFolder(t *testing.T) {
	t.Setenv("SSL_CERT_DIR", "")
	dir := filepath.Join(t.TempDir(), "certs")
	if err := Add(dir); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatalf("certs folder not created: %v", err)
	}
	if got, want := os.Getenv("SSL_CERT_DIR"), "/etc/ssl/certs:/etc/pki/tls/certs:"+dir; got != want {
		t.Errorf("SSL_CERT_DIR = %q, want %q", got, want)
	}
}

func TestAddExtendsAnSSLCertDirSetBefore(t *testing.T) {
	t.Setenv("SSL_CERT_DIR", "/opt/certs")
	dir := t.TempDir()
	if err := Add(dir); err != nil {
		t.Fatal(err)
	}
	if got, want := os.Getenv("SSL_CERT_DIR"), "/opt/certs:"+dir; got != want {
		t.Errorf("SSL_CERT_DIR = %q, want %q", got, want)
	}
}

func TestAClientTrustsASelfSignedServerWhoseCertificateIsInTheFolder(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	empty := t.TempDir()
	if _, err := Client(empty, 5*time.Second).Get(ts.URL); err == nil {
		t.Fatal("a self-signed server was trusted without its certificate")
	}

	dir := t.TempDir()
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})
	if err := os.WriteFile(filepath.Join(dir, "rest-server.pem"), cert, 0o600); err != nil {
		t.Fatal(err)
	}
	resp, err := Client(dir, 5*time.Second).Get(ts.URL)
	if err != nil {
		t.Fatalf("with the certificate in the folder: %v", err)
	}
	_ = resp.Body.Close()
}
