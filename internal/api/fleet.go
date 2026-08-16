package api

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ---------------------------------------------------------------------------
// Fleet view — GET /api/fleet/status (self-gating, polled by OTHER instances)
// + the CRUD/poll endpoints in fleet_handlers.go (session-protected) this
// instance uses to watch its own list of peers.
//
// Read-only monitoring only: a peer's Fleet page can see another instance's
// protection scorecard, never trigger an action on it. Two distinct tokens are
// involved per peer relationship: THIS instance's own FleetToken (what OTHER
// instances present to poll THIS instance, managed via POST/DELETE
// /api/fleet/token) and the PEER's FleetToken (what THIS instance presents
// when polling THAT peer, stored encrypted per-row in fleet_peers.token_enc).
// Modeled closely on the receiver dashboard (received_repos): a named registry
// row with a location + encrypted credential + last-check-verdict columns,
// polled on a schedule, with a manual on-demand check too.
// ---------------------------------------------------------------------------

// fleetPollTimeout bounds a single peer status poll — a peer on the LAN or a
// remote site should answer in well under this; a wedged/unreachable peer must
// not hold up the sweep for every other peer.
const fleetPollTimeout = 15 * time.Second

// fleetResponseMax caps how much of a peer's response body is read — a
// malicious or misbehaving peer answering with an unbounded body must not
// exhaust memory on the polling side.
const fleetResponseMax = 1 << 20 // 1 MiB

// fleetHTTPClient is the bounded HTTP client for peer status polls. Redirects
// are not followed (a redirect is not the peer answering) and the timeout
// backstops the per-request context, mirroring tamperHTTPClient. TLS
// verification is skipped: every BombVault instance serves HTTPS off a
// self-signed certificate scoped to loopback names (see the Dockerfile/
// healthcheckAt), never one valid for its real LAN/WAN address, so a peer at
// a real IP would otherwise always fail verification. This matches the
// app's existing LAN-trust posture (the same InsecureSkipVerify choice
// healthcheckAt already makes, and the "LAN trust model" /metrics already
// documents) — the fleet token itself is still the real access control.
var fleetHTTPClient = &http.Client{
	Timeout: fleetPollTimeout,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // G402: self-signed peer certs are the norm (see comment above); the fleet token is the real access control
	},
}

// fleetStatusResponse is the JSON shape GET /api/fleet/status returns, and
// what a peer poll decodes on the other side.
type fleetStatusResponse struct {
	OK           bool                `json:"ok"`
	InstanceName string              `json:"instanceName"`
	Version      string              `json:"version"`
	Domains      []DomainStatusEntry `json:"domains"`
}

// fleetTokenOK reports whether the request carries the stored fleet token, via
// the X-Fleet-Token header or the ?token= query parameter (the header wins
// when both are present). Constant-time compare; an EMPTY stored token always
// fails (feature off = fail closed), mirroring widgetTokenOK exactly.
func fleetTokenOK(r *http.Request, stored string) bool {
	if stored == "" {
		return false
	}
	got := r.Header.Get("X-Fleet-Token")
	if got == "" {
		got = r.URL.Query().Get("token")
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(stored)) == 1
}

// fleetGate loads settings and enforces the fleet token, mirroring widgetGate.
// On success it returns the loaded settings (so the handler needs no second
// read for InstanceName) and true; on failure it has already written the
// refusal (503 on a store error, 403 on a missing/mismatched/absent token).
func (h *Handler) fleetGate(w http.ResponseWriter, r *http.Request) (store.Settings, bool) {
	s, err := h.store.GetSettings()
	if err != nil {
		log.Printf("api: fleet: settings read failed: %v", err)
		http.Error(w, "fleet status unavailable", http.StatusServiceUnavailable)
		return store.Settings{}, false
	}
	if !fleetTokenOK(r, s.FleetToken) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return store.Settings{}, false
	}
	return s, true
}

// handleFleetStatus serves GET /api/fleet/status?token=… — the read-only
// protection-scorecard summary a peer's Fleet view polls. Same payload shape
// as GET /api/status (DomainStatusEntry[]) plus this instance's name/version,
// so a Fleet page can reuse the same rendering as the local dashboard.
func (h *Handler) handleFleetStatus(w http.ResponseWriter, r *http.Request) {
	s, ok := h.fleetGate(w, r)
	if !ok {
		return
	}
	domains, err := h.svc.DomainStatus()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, fleetStatusResponse{
		OK:           true,
		InstanceName: s.InstanceName,
		Version:      Version,
		Domains:      domains,
	})
}

// decryptFleetPeerToken decrypts a stored peer's token (the credential THIS
// instance presents when polling THAT peer) with this instance's own APP_KEY.
// An empty/unset token is a clear configuration error, not a transport one.
func (s *Service) decryptFleetPeerToken(p store.FleetPeer) (string, error) {
	if len(p.TokenEnc) == 0 {
		return "", errors.New("no token configured for this peer")
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, p.TokenEnc)
	if err != nil {
		return "", fmt.Errorf("decrypt peer token: %w", err)
	}
	return string(plain), nil
}

// pollFleetPeer issues one GET against peerURL's /api/fleet/status with token
// as the X-Fleet-Token header and decodes the response. A non-200 status or an
// "ok": false body is returned as an error — a peer poll either fully succeeds
// with a usable scorecard or is treated as failed, no partial credit.
func pollFleetPeer(ctx context.Context, peerURL, token string) (fleetStatusResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, fleetPollTimeout)
	defer cancel()

	statusURL := strings.TrimRight(peerURL, "/") + "/api/fleet/status"
	if _, err := url.Parse(statusURL); err != nil {
		return fleetStatusResponse{}, fmt.Errorf("invalid peer URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return fleetStatusResponse{}, fmt.Errorf("build fleet poll request: %w", err)
	}
	req.Header.Set("X-Fleet-Token", token)

	resp, err := fleetHTTPClient.Do(req)
	if err != nil {
		return fleetStatusResponse{}, err // transport error — propagate unchanged
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable
	body := io.LimitReader(resp.Body, fleetResponseMax)

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, body)
		return fleetStatusResponse{}, fmt.Errorf("peer returned HTTP %d (check the peer's fleet token)", resp.StatusCode)
	}
	var out fleetStatusResponse
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return fleetStatusResponse{}, fmt.Errorf("decode peer response: %w", err)
	}
	if !out.OK {
		return fleetStatusResponse{}, errors.New("peer reported an error")
	}
	return out, nil
}

// pollAndRecordFleetPeer polls one peer and persists the result (success or
// failure) via UpdateFleetPeerPollResult. It never returns an error itself —
// a poll failure is a normal, recorded outcome, not a caller-facing error —
// except the returned fleetStatusResponse/error pair for callers (like the
// manual poll-now endpoint) that want the live result immediately.
func (s *Service) pollAndRecordFleetPeer(ctx context.Context, p store.FleetPeer) (fleetStatusResponse, error) {
	token, err := s.decryptFleetPeerToken(p)
	var resp fleetStatusResponse
	if err == nil {
		resp, err = pollFleetPeer(ctx, p.URL, token)
	}

	ok := sql.NullBool{Valid: true, Bool: err == nil}
	detail := ""
	domainsJSON := p.LastPollDomainsJSON // keep the last-good cache on failure
	instanceName := p.LastPollInstanceName
	version := p.LastPollVersion
	if err != nil {
		detail = scrubError(err)
		if len(detail) > 200 {
			detail = detail[:200]
		}
	} else {
		instanceName = resp.InstanceName
		version = resp.Version
		if b, mErr := json.Marshal(resp.Domains); mErr == nil {
			domainsJSON = string(b)
		}
	}
	if uErr := s.store.UpdateFleetPeerPollResult(p.ID, time.Now().Unix(), ok, detail, instanceName, version, domainsJSON); uErr != nil {
		log.Printf("api: fleet: record poll result for %q: %v", p.Name, uErr)
	}
	return resp, err
}

// RunFleetPolls polls every enabled fleet peer once and records each result.
// It is the scheduler's fleet job (SetFleetJob in cmd/bombvault/main.go). Only
// a genuine failure to even LIST the peers is returned as an error; an
// individual unreachable peer is a normal recorded outcome, not a sweep
// failure (mirrors RunReceiverChecks's per-row best-effort discipline).
func (s *Service) RunFleetPolls(ctx context.Context) error {
	peers, err := s.store.ListFleetPeers()
	if err != nil {
		return fmt.Errorf("list fleet peers: %w", err)
	}
	for _, p := range peers {
		if !p.Enabled {
			continue
		}
		_, _ = s.pollAndRecordFleetPeer(ctx, p) //nolint:errcheck,gosec // recorded per-peer, not surfaced to the sweep caller
	}
	return nil
}
