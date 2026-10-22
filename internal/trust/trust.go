// Package trust lets BombVault, and the restic and rclone it runs, trust
// certificates the system does not, such as a rest-server's self-signed one.
// They are PEM files in a folder of the data directory.
package trust

import (
	"crypto/tls"
	"crypto/x509"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Dir is the folder under the data directory that holds the certificates.
func Dir(dataDir string) string { return filepath.Join(dataDir, "certs") }

// goCertDirs are the folders Go reads system certificates from on Linux. It
// stops reading them once SSL_CERT_DIR is set, so Add names them again.
var goCertDirs = []string{"/etc/ssl/certs", "/etc/pki/tls/certs"}

// Add creates dir and adds it to SSL_CERT_DIR, which every Go program this
// process starts reads beside the system certificates. restic reads the
// folder again on each run, so a certificate put there counts for the next
// test without a restart.
func Add(dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	dirs := goCertDirs
	if set := os.Getenv("SSL_CERT_DIR"); set != "" {
		dirs = strings.Split(set, ":")
	}
	if slices.Contains(dirs, dir) {
		return nil
	}
	return os.Setenv("SSL_CERT_DIR", strings.Join(append(slices.Clone(dirs), dir), ":"))
}

// Pool is the system pool with the files in dir read again, since the system
// pool this process loaded once stays as it was at its first use.
func Pool(dir string) *x509.CertPool {
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	certs := os.DirFS(dir)
	entries, err := fs.ReadDir(certs, ".")
	if err != nil {
		return pool
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if pem, err := fs.ReadFile(certs, e.Name()); err == nil {
			pool.AppendCertsFromPEM(pem)
		}
	}
	return pool
}

// Client is an HTTP client that trusts the system certificates and those in
// dir, follows no redirect and gives up after timeout.
func Client(dir string, timeout time.Duration) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{RootCAs: Pool(dir), MinVersion: tls.VersionTLS12}
	return &http.Client{
		Timeout:   timeout,
		Transport: tr,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
