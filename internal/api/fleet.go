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

// fleetPollTimeout bounds one peer poll, so an unreachable peer does not hold
// up the others.
const fleetPollTimeout = 15 * time.Second

// fleetResponseMax caps how much of a peer's response is read, so a peer that
// sends an endless body cannot exhaust memory here.
const fleetResponseMax = 1 << 20 // 1 MiB

// fleetHTTPClient polls peers. Redirects are not followed, since a redirect is
// not the peer answering. TLS verification is off because every instance
// serves a self-signed certificate for loopback names only; the fleet token is
// the access control.
var fleetHTTPClient = &http.Client{
	Timeout: fleetPollTimeout,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // G402: self-signed peer certs are the norm (see comment above); the fleet token is the real access control
	},
}

// fleetStatusResponse is what GET /api/fleet/status returns and a peer poll
// decodes. Domains has the shape of GET /api/status, so the Fleet page reuses
// the dashboard's rendering.
type fleetStatusResponse struct {
	OK           bool                `json:"ok"`
	InstanceName string              `json:"instanceName"`
	Version      string              `json:"version"`
	Domains      []DomainStatusEntry `json:"domains"`
}

// fleetTokenOK reports whether the request carries the stored fleet token in
// the X-Fleet-Token header. An empty stored token never matches. The token is
// not accepted as a query parameter, because URLs end up in browser history
// and in reverse proxy access logs.
func fleetTokenOK(r *http.Request, stored string) bool {
	if stored == "" {
		return false
	}
	got := r.Header.Get("X-Fleet-Token")
	return subtle.ConstantTimeCompare([]byte(got), []byte(stored)) == 1
}

// fleetGate loads the settings and checks the fleet token. On failure it has
// already written the response: 503 on a store error, 403 on a bad token.
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

// handleFleetStatus serves the protection summary a peer's Fleet view polls.
// It is read-only: a peer can see the scorecard but never act on it.
// GET /api/fleet/status
func (h *Handler) handleFleetStatus(w http.ResponseWriter, r *http.Request) {
	s, ok := h.fleetGate(w, r)
	if !ok {
		return
	}
	domains, err := h.svc.domainStatusFrom(s)
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

// decryptFleetPeerToken returns the token this instance presents when polling
// peer p.
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

// pollFleetPeer fetches /api/fleet/status from peerURL. A non-200 status or an
// "ok": false body is an error.
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
		return fleetStatusResponse{}, err
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

// pollAndRecordFleetPeer polls one peer and stores the outcome, keeping the
// last good scorecard on failure. It also returns the result, for the poll-now
// endpoint.
func (s *Service) pollAndRecordFleetPeer(ctx context.Context, p store.FleetPeer) (fleetStatusResponse, error) {
	token, err := s.decryptFleetPeerToken(p)
	var resp fleetStatusResponse
	if err == nil {
		resp, err = pollFleetPeer(ctx, p.URL, token)
	}

	ok := sql.NullBool{Valid: true, Bool: err == nil}
	detail := ""
	domainsJSON := p.LastPollDomainsJSON
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

// RunFleetPolls is the scheduler's fleet job: it polls every enabled peer once
// and records each result. Only failing to list the peers is an error; an
// unreachable peer is a recorded outcome.
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
