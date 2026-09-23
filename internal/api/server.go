package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/pem"
	"errors"
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

// brandArt is the brand ASCII art from banner.txt, which is copied from
// .github/assets/banner-raw.txt at build time.
//
//go:embed banner.txt
var brandArt string

// bindHost is explicit because binding to the container's hostname leaves the
// WebUI unreachable.
const bindHost = "0.0.0.0"

// Server serves the API and the embedded SPA over HTTP or HTTPS.
type Server struct {
	cfg     config.Config
	handler http.Handler
}

// NewServer returns a Server for the API router and the SPA in spaFS.
// securityHeaders is the outermost wrapper, so the compression layer below it
// cannot drop or reorder the security headers.
func NewServer(cfg config.Config, spaFS fs.FS, apiRouter http.Handler) *Server {
	return &Server{cfg: cfg, handler: securityHeaders(withCompression(NewSPAHandler(spaFS, apiRouter)))}
}

// securityHeaders sets baseline security headers on every response.
//
// The only inline script the CSP allows is the theme boot script in
// web/index.html, by its hash rather than 'unsafe-inline'. It sets data-theme
// before first paint so the page does not flash the wrong theme while the
// bundle loads. style-src needs 'unsafe-inline' for React style props, and
// img-src and font-src allow data: for flag-icons and inlined assets.
//
// GET /widget is meant to be framed by other dashboards, so it gets its own CSP
// with frame-ancestors * and no X-Frame-Options. Every other path, including
// the widget's /api/widget/data feed, is sent with DENY.
func securityHeaders(next http.Handler) http.Handler {
	// TestThemeBootScriptCSPHashMatches fails when this hash does not match the
	// inline script in web/index.html, whitespace included.
	const csp = "default-src 'self'; " +
		"script-src 'self' 'sha256-ijkCmxzYsyTqN0nAsR0mgUdCoqwAR/mw98d6MA0Ph4Y='; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"font-src 'self' data:; " +
		"connect-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"frame-ancestors 'none'"

	// The widget is a single page with inline style and script that only
	// fetches /api/widget/data.
	const widgetCSP = "default-src 'none'; " +
		"script-src 'unsafe-inline'; " +
		"style-src 'unsafe-inline'; " +
		"connect-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'none'; " +
		"form-action 'none'; " +
		"frame-ancestors *"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if r.URL.Path == "/widget" {
			w.Header().Set("Content-Security-Policy", widgetCSP)
		} else {
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Content-Security-Policy", csp)
		}
		next.ServeHTTP(w, r)
	})
}

// httpShutdownGrace bounds how long Run waits for in-flight requests on
// shutdown. Backups are handled by Service.BeginShutdown before this, and an
// SSE progress stream never closes on its own, so a long grace would only
// delay every stop.
const httpShutdownGrace = 3 * time.Second

// Run serves HTTPS with a self-signed certificate, or plain HTTP when
// cfg.HTTPOnly is set, until the listener fails or ctx is cancelled. On
// cancellation it shuts down gracefully, and the resulting ErrServerClosed is
// a clean stop, not an error.
func (s *Server) Run(ctx context.Context) error {
	var srv *http.Server
	var serve func() error

	if s.cfg.HTTPOnly {
		addr := net.JoinHostPort(bindHost, strconv.Itoa(s.cfg.Port))
		srv = &http.Server{
			Addr:              addr,
			Handler:           s.handler,
			ReadHeaderTimeout: 15 * time.Second,
		}
		printBanner()
		printReady("HTTP", s.cfg.Port)
		serve = srv.ListenAndServe
	} else {
		certPath, keyPath, err := EnsureSelfSigned(s.cfg.DataDir)
		if err != nil {
			return fmt.Errorf("server: ensure cert: %w", err)
		}
		addr := net.JoinHostPort(bindHost, strconv.Itoa(s.cfg.HTTPSPort))
		srv = &http.Server{
			Addr:              addr,
			Handler:           s.handler,
			ReadHeaderTimeout: 15 * time.Second,
		}
		printBanner()
		printReady("HTTPS", s.cfg.HTTPSPort)
		serve = func() error { return srv.ListenAndServeTLS(certPath, keyPath) }
	}

	errCh := make(chan error, 1)
	go func() { errCh <- serve() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), httpShutdownGrace)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("server: shutdown: %w", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

// EnsureSelfSigned creates a self-signed P-256 certificate under dataDir/certs
// on first boot and reuses it afterwards. It returns the certificate and key
// paths; the key file is written 0600.
func EnsureSelfSigned(dataDir string) (certPath, keyPath string, err error) {
	certDir := filepath.Join(dataDir, "certs")
	if mkErr := os.MkdirAll(certDir, 0o700); mkErr != nil {
		return "", "", fmt.Errorf("create certs dir: %w", mkErr)
	}
	certPath = filepath.Join(certDir, "cert.pem")
	keyPath = filepath.Join(certDir, "key.pem")

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
		NotAfter:              now.AddDate(10, 0, 0),
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

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if wErr := os.WriteFile(certPath, certPEM, 0o644); wErr != nil { //nolint:gosec // G306: a self-signed server certificate is public, not a secret
		return "", "", fmt.Errorf("write cert: %w", wErr)
	}

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

const (
	bannerName     = "bombvault"
	bannerSubtitle = "Backup & disaster recovery for Docker containers and KVM/libvirt VMs"
)

// Version is the build version, set with
// -ldflags "-X github.com/junkerderprovinz/bombvault/internal/api.Version=vX.Y.Z".
// Unstamped builds report "dev".
var Version = "dev"

// versionTag formats Version to follow the app name in the log: " vX.Y.Z", or
// " (dev)" for an unstamped build.
func versionTag() string {
	if Version == "" || Version == "dev" {
		return " (dev)"
	}
	return " " + Version
}

// printBanner prints the brand art and the app line in the same layout as
// print-banner.sh in the other container images:
//
//	<blank>
//	<brand ASCII art>
//	<blank>
//	  bombvault vX.Y.Z · Backup & disaster recovery for Docker containers and KVM/libvirt VMs
//	<blank>
func printBanner() {
	art := strings.TrimRight(brandArt, "\n")
	fmt.Println()
	fmt.Println(art)
	fmt.Println()
	fmt.Println("  " + bannerName + versionTag() + " · " + bannerSubtitle)
	fmt.Println()
}

// printReady prints the ready line in the format the other container images
// use, to stdout like the banner. It is the last output before the server
// starts listening.
func printReady(scheme string, port int) {
	fmt.Printf("  \033[0;32m✓ BOMBVAULT%s IS READY\033[0m - Open the WebUI now (%s %d)\n", versionTag(), scheme, port)
	fmt.Println()
}
