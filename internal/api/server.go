package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/pem"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
)

// brandArt is the shared Junker der Provinz brand ASCII art, embedded from
// banner.txt (which mirrors .github/assets/banner-raw.txt at build time).
//
//go:embed banner.txt
var brandArt string

// bindAddr is the listen address. We bind 0.0.0.0 explicitly (NOT $HOSTNAME) —
// binding to the container hostname was a real boot bug in the old version that
// made the WebUI unreachable.
const bindHost = "0.0.0.0"

// Server runs the HTTP(S) server serving the API + embedded SPA.
type Server struct {
	cfg     config.Config
	handler http.Handler
}

// NewServer wires the SPA handler over the embedded FS and the API router.
// The combined handler is wrapped in securityHeaders so every response —
// both API and SPA — carries the baseline HTTP security headers.
func NewServer(cfg config.Config, spaFS fs.FS, apiRouter http.Handler) *Server {
	return &Server{cfg: cfg, handler: securityHeaders(NewSPAHandler(spaFS, apiRouter))}
}

// securityHeaders is a middleware that sets minimal HTTP security headers on
// every response served by the handler.
//
// TODO(pre-public): add CSP once the SPA build is final
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// Run starts the server, blocking until it stops. It serves HTTPS with a
// self-signed cert by default, or plain HTTP when cfg.HTTPOnly is set.
func (s *Server) Run() error {
	if s.cfg.HTTPOnly {
		addr := net.JoinHostPort(bindHost, strconv.Itoa(s.cfg.Port))
		printBanner()
		printReady("HTTP", s.cfg.Port)
		srv := &http.Server{
			Addr:              addr,
			Handler:           s.handler,
			ReadHeaderTimeout: 15 * time.Second,
		}
		return srv.ListenAndServe()
	}

	certPath, keyPath, err := EnsureSelfSigned(s.cfg.DataDir)
	if err != nil {
		return fmt.Errorf("server: ensure cert: %w", err)
	}
	addr := net.JoinHostPort(bindHost, strconv.Itoa(s.cfg.HTTPSPort))
	printBanner()
	printReady("HTTPS", s.cfg.HTTPSPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.handler,
		ReadHeaderTimeout: 15 * time.Second,
	}
	return srv.ListenAndServeTLS(certPath, keyPath)
}

// EnsureSelfSigned generates a self-signed ECDSA (P-256) certificate in PURE GO
// (no openssl) under dataDir/certs on first boot and reuses it afterwards.
// It returns the cert and key file paths. The key file is written 0o600.
func EnsureSelfSigned(dataDir string) (certPath, keyPath string, err error) {
	certDir := filepath.Join(dataDir, "certs")
	if mkErr := os.MkdirAll(certDir, 0o700); mkErr != nil {
		return "", "", fmt.Errorf("create certs dir: %w", mkErr)
	}
	certPath = filepath.Join(certDir, "cert.pem")
	keyPath = filepath.Join(certDir, "key.pem")

	// Reuse an existing pair.
	if fileExists(certPath) && fileExists(keyPath) {
		return certPath, keyPath, nil
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("generate serial: %w", err)
	}

	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "bombvault", Organization: []string{"BombVault"}},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.AddDate(10, 0, 0), // 10 years
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return "", "", fmt.Errorf("create certificate: %w", err)
	}

	// Write cert.pem (0o644 — public).
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if wErr := os.WriteFile(certPath, certPEM, 0o644); wErr != nil { //nolint:gosec // G306: a self-signed server certificate is public, not a secret
		return "", "", fmt.Errorf("write cert: %w", wErr)
	}

	// Write key.pem (0o600 — private).
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if wErr := os.WriteFile(keyPath, keyPEM, 0o600); wErr != nil {
		return "", "", fmt.Errorf("write key: %w", wErr)
	}

	return certPath, keyPath, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ---------------------------------------------------------------------------
// banners
// ---------------------------------------------------------------------------

const (
	bannerSep      = "───────────────────────────────────────────────────────────────────"
	bannerName     = "bombvault"
	bannerSubtitle = "Backup & disaster recovery for Docker containers and KVM/libvirt VMs"
)

// printBanner prints the shared brand ASCII art followed by the app name and
// subtitle, matching the house print-banner.sh format used by all own-image
// containers.
//
// Output format (mirrors print-banner.sh exactly):
//
//	<blank>
//	<brand ASCII art>
//	<blank>
//	  ───×67
//	  bombvault
//	  Backup & disaster recovery for Docker containers and KVM/libvirt VMs
//	  ───×67
//	<blank>
// printBanner prints the shared Junker-der-Provinz brand banner exactly like the
// other containers' print-banner.sh: a leading blank, the brand art, ONE blank
// line, then the name/subtitle block framed by the ─ rule. TrimRight makes the
// spacing deterministic regardless of the embedded file's trailing newline.
func printBanner() {
	sep := "  " + bannerSep
	art := strings.TrimRight(brandArt, "\n")
	fmt.Println()
	fmt.Println(art)
	fmt.Println()
	fmt.Println(sep)
	fmt.Println("  " + bannerName)
	fmt.Println("  " + bannerSubtitle)
	fmt.Println(sep)
	fmt.Println()
}

// readyHashes is the 60-char '#' rule used by the house "<APP> IS READY" box
// (matches the jdownloader/krusader/matrix init banners).
const readyHashes = "############################################################"

// printReady prints the loud READY box once the server is about to listen, in the
// shared house format ('#' box). Writes to stdout (via fmt) so it shares the
// banner's stream and always appears after the ASCII art.
func printReady(scheme string, port int) {
	fmt.Println("  " + readyHashes)
	fmt.Printf("   BOMBVAULT IS READY  ->  open the WebUI now (%s %d)\n", scheme, port)
	fmt.Println("  " + readyHashes)
}
