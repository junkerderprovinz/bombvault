package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
)

// TamperVerdict is the result of a stage-1 off-site tamper test: an active probe
// of the far side's delete path that PROVES (rather than assumes) the append-only
// protection is actually enforced.
//
//   - Testable is false when the repo can't be probed this way (only REST repos
//     can); Protected/Detail are then unset.
//   - Protected is true only when EVERY probe's delete was refused (403/405).
//   - Detail carries the scrubbed reason when Protected is false.
type TamperVerdict struct {
	Testable  bool   `json:"testable"`
	Protected bool   `json:"protected"`
	Detail    string `json:"detail"`
}

// tamperHTTPClient is the bounded HTTP client for tamper probes. Redirects are not
// followed (a redirect is not a delete verdict) and the timeout backstops the
// per-request context so a wedged server can't hang the test.
var tamperHTTPClient = &http.Client{
	Timeout: 25 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// RunTamperTest runs a stage-1 off-site tamper test for a domain: it issues two
// side-effect-free, authenticated HTTP DELETEs against provably non-existent
// object IDs on the far-side rest-server and reads the status code to decide
// whether the server actually refuses deletes (append-only enforced).
//
// Only `rest:` repos are testable this way; anything else reports Testable=false
// honestly. A transport/network error (dial fail, timeout) is INCONCLUSIVE — it is
// neither protected nor unprotected — so RunTamperTest returns a non-nil error and
// records NOTHING in that case; only a real HTTP verdict is persisted. A recorded
// protected→unprotected flip fires a protection-loss notification.
func (s *Service) RunTamperTest(ctx context.Context, domain string) (TamperVerdict, error) {
	switch domain {
	case "containers", "vms", "flash", "config", "files":
	default:
		return TamperVerdict{}, fmt.Errorf("unknown domain %q", domain)
	}
	// Serialise per domain so read-prev → record → notify is atomic: a second
	// concurrent test then reads the verdict this one recorded (no double / dropped
	// protection-loss alert on a flip).
	defer s.lockTamper(domain)()
	settings, err := s.store.GetSettings()
	if err != nil {
		return TamperVerdict{}, fmt.Errorf("read settings: %w", err)
	}
	loc := s.offsiteRepoFor(domain, settings)
	if loc == "" {
		return TamperVerdict{}, errors.New("no off-site repo configured for this domain")
	}
	// Stage-1 tamper testing speaks the REST protocol directly (raw HTTP DELETE to
	// the rest-server). Other backends (rclone/s3/sftp/local) can't be probed this
	// way — say so honestly instead of guessing a verdict.
	if !strings.HasPrefix(loc, "rest:") {
		return TamperVerdict{Testable: false, Detail: "only REST repos are verifiable"}, nil
	}
	// rest:http://host:8000/path -> http://host:8000/path (the HTTP base the
	// rest-server serves; a trailing slash is trimmed so path joins are clean).
	base := strings.TrimRight(strings.TrimPrefix(loc, "rest:"), "/")

	// Basic-auth credentials for the rest-server come from the encrypted cloud
	// config (best-effort: a decode failure just means no auth header, and the
	// server then answers 401 — a real HTTP verdict, not a transport error).
	creds, _ := s.decodeCloud(settings)

	// Two provably non-existent object IDs: a 64-hex data blob id and an 8-hex
	// snapshot id. Deleting them can never touch real repo data.
	dataID, err := randomHex(32) // 64 hex chars
	if err != nil {
		return TamperVerdict{}, err
	}
	snapID, err := randomHex(4) // 8 hex chars
	if err != nil {
		return TamperVerdict{}, err
	}
	probes := []string{base + "/data/" + dataID, base + "/snapshots/" + snapID}

	protected := true
	var details []string
	for _, url := range probes {
		p, detail, perr := tamperProbe(ctx, url, creds.RESTUser, creds.RESTPassword)
		if perr != nil {
			// Transport/network error → the test is INCONCLUSIVE. Record NOTHING and
			// return the error: an unreachable server is neither protected nor
			// unprotected, and must never flip the stored verdict either way.
			return TamperVerdict{}, perr
		}
		if !p {
			protected = false
		}
		if detail != "" {
			details = append(details, detail)
		}
	}

	verdict := TamperVerdict{Testable: true, Protected: protected}
	if !protected {
		verdict.Detail = strings.Join(details, "; ")
	}

	// Read the previous verdict BEFORE recording the new one so a
	// protected→unprotected flip fires exactly one protection-loss alert.
	prev, hadPrev, _ := s.store.LatestTamperTest(domain)
	if recErr := s.store.RecordTamperTest(domain, verdict.Protected, verdict.Detail); recErr != nil {
		return TamperVerdict{}, fmt.Errorf("record tamper test: %w", recErr)
	}
	// Mirror the verdict into the shared runs table so the test shows in the
	// dashboard Activity Log/Run History. success = the delete was refused
	// (append-only enforced); failed = NOT protected — the alarming outcome.
	// Inconclusive/non-testable paths return above and record nothing here too.
	s.recordDomainRun(domain, "tamper", verdict.Protected, verdict.Detail)
	if hadPrev && prev.Protected && !verdict.Protected {
		s.notifyProtectionLost(ctx, domain, verdict.Detail)
	}
	return verdict, nil
}

// tamperProbe issues one authenticated DELETE and maps the status code to a
// protection verdict. ONLY decisive statuses yield a verdict; everything else is
// treated as INCONCLUSIVE and returned as a non-nil error (exactly like a
// transport error), so the caller records nothing and notifies nothing rather
// than flip a stored verdict on an ambiguous response.
//
//   - 403 / 405 → protected (the delete was refused — append-only enforced)
//   - 404       → NOT protected (the object did not exist; the server would have
//     deleted a real one — not append-only)
//   - 2xx       → NOT protected (the server accepted a delete)
//   - 401 / 3xx / 5xx / anything else → INCONCLUSIVE (non-nil error): auth
//     failure (rotated creds), a redirect, or far-side/proxy maintenance is not a
//     delete verdict and must never be read as "not protected".
func tamperProbe(ctx context.Context, url, user, pass string) (protected bool, detail string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return false, "", fmt.Errorf("build tamper request: %w", err)
	}
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := tamperHTTPClient.Do(req)
	if err != nil {
		return false, "", err // transport error — inconclusive, propagate unchanged
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable
	_, _ = io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusMethodNotAllowed:
		return true, "", nil
	case resp.StatusCode == http.StatusNotFound:
		return false, "server would have deleted (404) — not append-only", nil
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return false, "server accepted a delete", nil
	default:
		// 401/3xx/5xx/unexpected: not a delete verdict → inconclusive, like a
		// transport error. Returning an error makes RunTamperTest record + notify
		// nothing, so a rotated credential or a far-side maintenance window can never
		// masquerade as a lost append-only guarantee.
		return false, "", fmt.Errorf("inconclusive tamper probe: unexpected status %d", resp.StatusCode)
	}
}

// randomHex returns nBytes of cryptographically-random data as a lowercase hex
// string (2*nBytes characters).
func randomHex(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// notifyProtectionLost sends a best-effort alert when a domain's off-site
// append-only protection is newly LOST (a tamper test that used to pass now fails).
// It mirrors notifyDrillFailure's policy + Unraid fan-out; a no-op when
// notifications are off.
func (s *Service) notifyProtectionLost(ctx context.Context, domain, detail string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	subject := "Off-site protection LOST for " + domain
	msg := fmt.Sprintf("The off-site tamper test for %s reports the append-only protection is GONE — the far side accepted a delete: %s", domain, detail)
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: subject + " — " + msg, OK: false})
	if c.Unraid && s.ssh != nil {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}
