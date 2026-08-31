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
//
// Compression sits INSIDE securityHeaders ([364]): the security headers are
// then written by the outermost wrapper, where nothing below can drop or
// reorder them, and the encoding layer only ever sees a response whose headers
// are already settled.
func NewServer(cfg config.Config, spaFS fs.FS, apiRouter http.Handler) *Server {
	return &Server{cfg: cfg, handler: securityHeaders(withCompression(NewSPAHandler(spaFS, apiRouter)))}
}

// securityHeaders is a middleware that sets baseline HTTP security headers on
// every response served by the handler (both API and SPA).
//
// CSP notes: the SPA is bundled JS/CSS only, with ONE deliberate inline
// script — web/index.html's theme-boot script. It stamps data-theme on
// <html> synchronously before first paint (GlimStone form-engine #1's
// "system" default reads prefers-color-scheme, so without this the page
// flashes the wrong theme while the module bundle is still loading). It's
// allowed by a CSP hash source below, not 'unsafe-inline' — that would let
// ANY inline script run, not just this one. TestThemeBootScriptCSPHashMatches
// (widget_internal_test.go) recomputes the script's actual sha256 from
// web/index.html and fails the build if it no longer matches the hash
// configured here, so an edited script can't silently start failing CSP in
// production while dev/preview (which send no CSP at all) stay green.
// React inline style= props and CSS variables → style-src needs
// 'unsafe-inline'. 'unsafe-eval' is intentionally absent. img-src and
// font-src allow data: for flag-icons and any inline SVG/font the SPA
// embeds.
//
// GET /widget is the ONE deliberate exception: the embeddable dashboard-widget
// page exists to be framed by OTHER dashboards (Homepage/Organizr/…), so it
// gets its own CSP with `frame-ancestors *` and NO X-Frame-Options — and,
// being a single self-contained page, inline script/style instead of 'self'
// bundles. Every other path (the SPA and all /api routes, including the
// widget's own /api/widget/data feed) keeps the strict DENY/'none' posture.
func securityHeaders(next http.Handler) http.Handler {
	// The hash source below is the theme-boot script — see the securityHeaders
	// doc comment. TestThemeBootScriptCSPHashMatches pins it to the script's
	// actual current content; if you edit web/index.html's inline script
	// (including its whitespace), recompute the hash and update it here, or
	// that test fails on purpose.
	const csp = "default-src 'self'; " +
		"script-src 'self' 'sha256-OyogNhfMmFOmnpKoxuucDcL3wuNp1ArXH1kHMlcPetY='; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"font-src 'self' data:; " +
		"connect-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"frame-ancestors 'none'"

	// The widget page is fully self-contained (inline style + script, fetches
	// only its same-origin /api/widget/data feed) and must stay frame-able
	// cross-origin.
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
			// Frame-able by design: no X-Frame-Options, frame-ancestors *.
			w.Header().Set("Content-Security-Policy", widgetCSP)
		} else {
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Content-Security-Policy", csp)
		}
		next.ServeHTTP(w, r)
	})
}

// httpShutdownGrace bounds how long Run waits for in-flight HTTP requests after
// the context is cancelled. Short on purpose: by the time we get here the
// backups have already been dealt with (api.Service.BeginShutdown), so what is
// left is browser traffic and the SSE progress stream, and an SSE connection
// never closes on its own — waiting on it would mean always waiting the full
// grace, every single stop.
const httpShutdownGrace = 3 * time.Second

// Run starts the server, blocking until it stops or ctx is cancelled. It serves
// HTTPS with a self-signed cert by default, or plain HTTP when cfg.HTTPOnly is
// set.
//
// On ctx cancellation it calls srv.Shutdown, which stops accepting new
// connections and lets in-flight requests finish ([375]). ErrServerClosed is
// then the EXPECTED outcome, not a failure, so it is swallowed: reporting it
// would turn every clean stop into a non-zero exit and an error in the log.
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
		// The listener died on its own (port taken, cert unreadable). That is a
		// real error and must surface.
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
	bannerName     = "bombvault"
	bannerSubtitle = "Backup & disaster recovery for Docker containers and KVM/libvirt VMs"
)

// Version is the build version, injected at build time via
// -ldflags "-X github.com/junkerderprovinz/bombvault/internal/api.Version=vX.Y.Z".
// It defaults to "dev" for local/un-stamped builds and is printed in the startup
// banner + READY box so the running image's version is obvious in the log.
var Version = "dev"

// versionTag renders the version for the log banners: " vX.Y.Z" for a stamped
// build, " (dev)" otherwise, so it slots cleanly after the app name.
func versionTag() string {
	if Version == "" || Version == "dev" {
		return " (dev)"
	}
	return " " + Version
}

// printBanner prints the shared brand ASCII art followed by the app name and
// subtitle, matching the house print-banner.sh format used by all own-image
// containers.
//
// Output format (mirrors print-banner.sh exactly):
//
//	<blank>
//	<brand ASCII art>
//	<blank>
//	  bombvault vX.Y.Z · Backup & disaster recovery for Docker containers and KVM/libvirt VMs
//	<blank>
//
// A leading blank, the brand art, ONE blank line, then a clean name+subtitle
// line (no rules). TrimRight makes the spacing deterministic regardless of
// the embedded file's trailing newline.
func printBanner() {
	art := strings.TrimRight(brandArt, "\n")
	fmt.Println()
	fmt.Println(art)
	fmt.Println()
	fmt.Println("  " + bannerName + versionTag() + " · " + bannerSubtitle)
	fmt.Println()
}

// printReady prints the loud "<APP> IS READY" line once the server is about
// to listen, in the shared house one-line format (matches
// jdownloader/krusader/matrix/handbrake/featherdrop). Writes to stdout (via
// fmt) so it shares the banner's stream; this is always the LAST thing this
// process prints before it blocks on ListenAndServe.
func printReady(scheme string, port int) {
	fmt.Printf("  \033[0;32m✓ BOMBVAULT%s IS READY\033[0m - Open the WebUI now (%s %d)\n", versionTag(), scheme, port)
	fmt.Println()
}
