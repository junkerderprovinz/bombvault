package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
)

func certService(t *testing.T) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	if _, _, err := EnsureSelfSigned(dir); err != nil {
		t.Fatalf("ensure self signed: %v", err)
	}
	return NewService(config.Config{DataDir: dir}, nil, nil, nil, nil), filepath.Join(dir, "certs", "cert.pem")
}

func readCertFile(t *testing.T, path string) *x509.Certificate {
	t.Helper()
	pemBytes, err := os.ReadFile(path) //nolint:gosec // G304: path is the certificate under the test's own TempDir
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatalf("no PEM block in %s", path)
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return leaf
}

func servedNames(t *testing.T) []string {
	t.Helper()
	served := servedCertificate.Load()
	if served == nil {
		t.Fatal("the listener was never handed a certificate")
	}
	leaf, err := x509.ParseCertificate(served.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return certificateNames(leaf)
}

func TestAddCertificateNameReissuesOwnCertificate(t *testing.T) {
	svc, certPath := certService(t)
	before := readCertFile(t, certPath)

	info, err := svc.AddCertificateName("192.168.1.10")
	if err != nil {
		t.Fatalf("add name: %v", err)
	}
	want := []string{"localhost", "127.0.0.1", "::1", "192.168.1.10"}
	if !slices.Equal(info.Names, want) {
		t.Fatalf("names = %v, want %v", info.Names, want)
	}
	if !info.SelfIssued {
		t.Fatal("the reissued certificate must still be BombVault's own")
	}
	if got := certificateNames(readCertFile(t, certPath)); !slices.Equal(got, want) {
		t.Fatalf("names on disk = %v, want %v", got, want)
	}
	if got := servedNames(t); !slices.Equal(got, want) {
		t.Fatalf("names served to clients = %v, want %v", got, want)
	}
	after := readCertFile(t, certPath)
	pub, ok := after.PublicKey.(*ecdsa.PublicKey)
	if !ok || !pub.Equal(before.PublicKey) {
		t.Fatal("the key pair changed; every client that trusted the old certificate would break")
	}

	pemBefore, err := os.ReadFile(certPath) //nolint:gosec // G304: the certificate under the test's own TempDir
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddCertificateName("192.168.1.10"); err != nil {
		t.Fatalf("adding a covered address again: %v", err)
	}
	pemAfter, err := os.ReadFile(certPath) //nolint:gosec // G304: the certificate under the test's own TempDir
	if err != nil {
		t.Fatal(err)
	}
	if string(pemBefore) != string(pemAfter) {
		t.Fatal("adding an address the certificate already names rewrote it")
	}

	if _, err := svc.AddCertificateName("bad host!"); !errors.Is(err, errCertNameInvalid) {
		t.Fatalf("bad host: err = %v, want %v", err, errCertNameInvalid)
	}

	for i := range certExtraNameLimit - 1 {
		if _, err := svc.AddCertificateName(fmt.Sprintf("host%d.lan", i)); err != nil {
			t.Fatalf("added address %d: %v", i, err)
		}
	}
	if _, err := svc.AddCertificateName("one.too.many.lan"); !errors.Is(err, errCertNameLimit) {
		t.Fatalf("beyond the limit: err = %v, want %v", err, errCertNameLimit)
	}
}

func TestAddCertificateNameLeavesAnOperatorCertificateAlone(t *testing.T) {
	svc, certPath := certService(t)
	foreign := foreignCertificatePEM(t)
	if err := os.WriteFile(certPath, foreign, 0o644); err != nil { //nolint:gosec // G306: a server certificate is public material
		t.Fatal(err)
	}

	if _, err := svc.AddCertificateName("192.168.1.10"); !errors.Is(err, errCertNotOwn) {
		t.Fatalf("err = %v, want %v", err, errCertNotOwn)
	}
	got, err := os.ReadFile(certPath) //nolint:gosec // G304: the certificate under the test's own TempDir
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(foreign) {
		t.Fatal("the operator's certificate was rewritten")
	}
	info, ok := svc.CertificateInfo()
	if !ok || info.SelfIssued {
		t.Fatalf("info = %+v, ok = %v, want a certificate that is not self issued", info, ok)
	}
}

func TestAddCertificateNameKeepsTheOldCertificateWhenItCannotIssueANewOne(t *testing.T) {
	// Each case first adds an address, so the failing call has something to
	// leave in place and the assertion can tell the two certificates apart.
	cases := map[string]func(t *testing.T, certDir string){
		"a damaged key file": func(t *testing.T, certDir string) {
			if err := os.WriteFile(filepath.Join(certDir, "key.pem"), []byte("truncated"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"a certs directory the container cannot write": func(t *testing.T, certDir string) {
			if runtime.GOOS == "windows" || os.Geteuid() == 0 {
				t.Skip("a directory's write permission only stops an unprivileged unix process")
			}
			if err := os.Chmod(certDir, 0o500); err != nil { //nolint:gosec // G302: the case under test
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(certDir, 0o700) }) //nolint:gosec // G302: hands the directory back so TempDir can remove it
		},
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			svc, certPath := certService(t)
			if _, err := svc.AddCertificateName("10.0.0.1"); err != nil {
				t.Fatal(err)
			}
			breakIt(t, filepath.Dir(certPath))

			if _, err := svc.AddCertificateName("10.0.0.2"); !errors.Is(err, errCertWriteFailed) {
				t.Fatalf("err = %v, want %v", err, errCertWriteFailed)
			}
			names := servedNames(t)
			if !slices.Contains(names, "10.0.0.1") || slices.Contains(names, "10.0.0.2") {
				t.Fatalf("names served to clients = %v, want the certificate from before the failed call", names)
			}
			if got := certificateNames(readCertFile(t, certPath)); slices.Contains(got, "10.0.0.2") {
				t.Fatalf("names on disk = %v, want the file untouched", got)
			}
		})
	}
}

func TestTheListenerServesAReissuedCertificate(t *testing.T) {
	svc, certPath := certService(t)
	if err := loadServedCertificate(certPath, filepath.Join(filepath.Dir(certPath), "key.pem")); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{
		Handler:           http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig:         servedTLSConfig(),
	}
	go func() { _ = srv.ServeTLS(ln, "", "") }()
	defer func() { _ = srv.Close() }()

	if _, err := svc.AddCertificateName("10.1.2.3"); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // G402: the test reads back the certificate its own listener presents
	}}
	resp, err := client.Get("https://" + ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	names := certificateNames(resp.TLS.PeerCertificates[0])
	if !slices.Contains(names, "10.1.2.3") {
		t.Fatalf("the listener presents %v; a restart must not be needed for an added address", names)
	}
}

func TestCertificateInfoIsEmptyWithoutTLS(t *testing.T) {
	svc, _ := certService(t)
	if _, ok := svc.CertificateInfo(); !ok {
		t.Fatal("an HTTPS install reports its certificate")
	}
	svc.cfg.HTTPOnly = true
	if info, ok := svc.CertificateInfo(); ok {
		t.Fatalf("info = %+v, want nothing to report while the interface serves plain HTTP", info)
	}
}

// foreignCertificatePEM is a self-signed certificate with someone else's
// subject, the shape of a certificate an operator dropped into /config/certs.
func foreignCertificatePEM(t *testing.T) []byte {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(42),
		Subject:               pkix.Name{CommonName: "nas.example.com", Organization: []string{"Example"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"nas.example.com"},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
