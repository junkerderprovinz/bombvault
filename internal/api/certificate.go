package api

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CertInfo describes the certificate the web interface serves, so the settings
// card can tell whether the address the operator opened is one clients will
// accept.
type CertInfo struct {
	SelfIssued  bool     `json:"selfIssued"`
	Names       []string `json:"names"`
	Fingerprint string   `json:"fingerprint"`
}

// certExtraNameLimit bounds how many addresses an operator may add on top of
// the three the first certificate carries.
const certExtraNameLimit = 16

var (
	errCertNameInvalid = errors.New("that is not a valid host name or IP address")
	errCertNameLimit   = fmt.Errorf("a certificate carries at most %d added addresses", certExtraNameLimit)
	errCertNotOwn      = errors.New("the certificate in use was not issued by BombVault")
	errCertWriteFailed = errors.New("the new certificate could not be written")
)

// servedCertificate is what the TLS listener hands to clients. The listener and
// the service that reissues the certificate share nothing else, and reading it
// through a pointer is what lets an added address take effect without a restart.
var servedCertificate atomic.Pointer[tls.Certificate]

// certReissue serialises the read, sign and write of a reissue, so two clicks
// at once cannot drop one of the two names.
var certReissue sync.Mutex

// servedTLSConfig makes the listener read servedCertificate on every
// handshake instead of holding a copy from start-up.
func servedTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return servedCertificate.Load(), nil
		},
	}
}

// loadServedCertificate parses the certificate files and hands the result to
// the listener.
func loadServedCertificate(certPath, keyPath string) error {
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return err
	}
	servedCertificate.Store(&pair)
	return nil
}

func certificateFiles(dataDir string) (certPath, keyPath string) {
	certDir := filepath.Join(dataDir, "certs")
	return filepath.Join(certDir, "cert.pem"), filepath.Join(certDir, "key.pem")
}

// CertificatePEM returns the certificate the web interface serves, for an
// operator who wants to trust it on a client machine. It reports false when the
// interface runs plain HTTP or the file cannot be read.
func (s *Service) CertificatePEM() ([]byte, bool) {
	if s.cfg.HTTPOnly {
		return nil, false
	}
	certPath, _ := certificateFiles(s.cfg.DataDir)
	pemBytes, err := os.ReadFile(certPath) //nolint:gosec // G304: the path is built from the configured data dir, not from a request
	if err != nil {
		log.Printf("api: certificate: could not read the certificate: %v", err)
		return nil, false
	}
	return pemBytes, true
}

// CertificateInfo describes the certificate the web interface serves. It
// reports false when the interface runs plain HTTP or the file cannot be read.
func (s *Service) CertificateInfo() (CertInfo, bool) {
	pemBytes, ok := s.CertificatePEM()
	if !ok {
		return CertInfo{}, false
	}
	leaf, err := parseCertificatePEM(pemBytes)
	if err != nil {
		log.Printf("api: certificate: %v", err)
		return CertInfo{}, false
	}
	return certInfoOf(leaf), true
}

// AddCertificateName reissues BombVault's own certificate with host among its
// names and hands it to the running listener. The key pair from key.pem stays
// the same, so a client that pinned the key keeps working, and anything that
// goes wrong leaves the certificate in use untouched.
func (s *Service) AddCertificateName(host string) (CertInfo, error) {
	host, ok := normalizeCertName(host)
	if !ok {
		return CertInfo{}, errCertNameInvalid
	}

	certReissue.Lock()
	defer certReissue.Unlock()

	certPath, keyPath := certificateFiles(s.cfg.DataDir)
	pemBytes, err := os.ReadFile(certPath) //nolint:gosec // G304: the path is built from the configured data dir, not from a request
	if err != nil {
		return CertInfo{}, fmt.Errorf("%w: %w", errCertWriteFailed, err)
	}
	leaf, err := parseCertificatePEM(pemBytes)
	if err != nil {
		return CertInfo{}, fmt.Errorf("%w: %w", errCertWriteFailed, err)
	}
	if !selfIssued(leaf) {
		return CertInfo{}, errCertNotOwn
	}
	names := certificateNames(leaf)
	if slices.Contains(names, host) {
		return certInfoOf(leaf), nil
	}
	if addedCertNames(names) >= certExtraNameLimit {
		return CertInfo{}, errCertNameLimit
	}

	reissued, err := reissueCertificate(leaf, certPath, keyPath, host)
	if err != nil {
		return CertInfo{}, fmt.Errorf("%w: %w", errCertWriteFailed, err)
	}
	return certInfoOf(reissued), nil
}

// reissueCertificate signs the certificate again with the same key pair and the
// added name, writes it atomically and puts it in front of the listener. The
// pointer is only advanced once the new certificate parses, so a half-written
// file never reaches a client.
func reissueCertificate(leaf *x509.Certificate, certPath, keyPath, host string) (*x509.Certificate, error) {
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	signer, ok := pair.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, errors.New("the key in key.pem cannot sign")
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               leaf.Subject,
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              leaf.DNSNames,
		IPAddresses:           leaf.IPAddresses,
		BasicConstraintsValid: true,
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = append(slices.Clone(leaf.IPAddresses), ip)
	} else {
		tmpl.DNSNames = append(slices.Clone(leaf.DNSNames), host)
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, signer.Public(), signer)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := writeCertificateFile(certPath, certPEM); err != nil {
		return nil, err
	}
	if err := loadServedCertificate(certPath, keyPath); err != nil {
		return nil, err
	}
	return x509.ParseCertificate(der)
}

// writeCertificateFile replaces the certificate in one step, so a client that
// connects during the write gets either the old file or the new one.
func writeCertificateFile(certPath string, certPEM []byte) error {
	f, err := os.CreateTemp(filepath.Dir(certPath), "cert-*.pem")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if _, err := f.Write(certPEM); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil { //nolint:gosec // G302: a server certificate is public material, and clients read it
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, certPath)
}

func parseCertificatePEM(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("the certificate file holds no PEM block")
	}
	return x509.ParseCertificate(block.Bytes)
}

func certInfoOf(leaf *x509.Certificate) CertInfo {
	sum := sha256.Sum256(leaf.Raw)
	return CertInfo{
		SelfIssued:  selfIssued(leaf),
		Names:       certificateNames(leaf),
		Fingerprint: hex.EncodeToString(sum[:]),
	}
}

// certificateNames lists the addresses a client will accept the certificate
// for, in the order the settings card shows them.
func certificateNames(leaf *x509.Certificate) []string {
	names := make([]string, 0, len(leaf.DNSNames)+len(leaf.IPAddresses))
	names = append(names, leaf.DNSNames...)
	for _, ip := range leaf.IPAddresses {
		names = append(names, ip.String())
	}
	return names
}

// certDefaultNames are the names EnsureSelfSigned puts in the first
// certificate; everything beyond them counts against certExtraNameLimit.
var certDefaultNames = []string{"localhost", "127.0.0.1", "::1"}

func addedCertNames(names []string) int {
	n := 0
	for _, name := range names {
		if !slices.Contains(certDefaultNames, name) {
			n++
		}
	}
	return n
}

// selfIssued reports whether the certificate is the one EnsureSelfSigned
// writes. An operator who put their own certificate in place keeps it:
// BombVault has no business signing someone else's name.
func selfIssued(leaf *x509.Certificate) bool {
	if leaf.Subject.CommonName != "bombvault" || !slices.Contains(leaf.Subject.Organization, "BombVault") {
		return false
	}
	return leaf.Issuer.String() == leaf.Subject.String()
}

// normalizeCertName accepts an IP literal or a DNS name of at most 253
// characters and returns it in the spelling the certificate carries.
func normalizeCertName(host string) (string, bool) {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), true
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" || len(host) > 253 {
		return "", false
	}
	for _, label := range strings.Split(host, ".") {
		if !validHostLabel(label) {
			return "", false
		}
	}
	return host, true
}

func validHostLabel(label string) bool {
	if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, r := range label {
		if r != '-' && (r < '0' || r > '9') && (r < 'a' || r > 'z') {
			return false
		}
	}
	return true
}
