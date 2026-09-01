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
	"github.com/junkerderprovinz/bombvault/internal/store"
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
// records NO VERDICT in that case; only a real HTTP verdict is persisted, and a
// recorded protected→unprotected flip fires a protection-loss notification.
// Independently of the verdict store, EVERY outcome settles a run row in the
// shared runs table (open-at-start, mirroring verify/prune): "success" =
// protected, "failed" = NOT protected, "skipped" = ran but produced no verdict
// (non-REST backend, transport error, inconclusive probe) — so a scheduled test
// against e.g. a B2 object-lock off-site never runs invisibly (#109).
func (s *Service) RunTamperTest(ctx context.Context, domain string) (verdict TamperVerdict, err error) {
	switch domain {
	case "containers", "vms", "flash", "config", "files":
	default:
		return TamperVerdict{}, fmt.Errorf("unknown domain %q", domain)
	}
	// Serialise per domain so read-prev → record → notify is atomic: a second
	// concurrent test then reads the verdict this one recorded (no double / dropped
	// protection-loss alert on a flip).
	defer s.lockTamper(domain)()
	// Publish a live "maintenance" progress pair keyed "tamper:<domain>" (mirroring
	// prune:/verify:/drill:) so a running tamper test shows on the dashboard
	// activity log WHILE it probes the far side (#109). The terminal event is
	// deferred so an early error/panic can never leave a stuck live line.
	tkey := "tamper:" + domain
	_, startedAt := s.progBegin(ctx, tkey, "maintenance")
	defer func() { s.progEnd(tkey, "maintenance", err == nil, startedAt) }()
	settings, err := s.store.GetSettings()
	if err != nil {
		return TamperVerdict{}, fmt.Errorf("read settings: %w", err)
	}
	// Resolve the domain's off-site DESTINATIONS (one per domain for a
	// single-off-site install; the settings-synthesized target for an un-backfilled
	// one). Each is probed in turn and the verdicts are folded worst-of, mirroring
	// how copyToOffsite fans replication out per target while keeping ONE per-domain
	// run row + progress line. For N=1 this is a single iteration → byte-identical.
	targets := s.offsiteReplicationTargets(domain, settings)
	if len(targets) == 0 {
		// Nothing to test, so no run row either: the manual caller gets this clear
		// error directly, and the schedule only dispatches domains flagged immutable
		// (immutableOffsiteDomains) — a flag without a repo is a misconfiguration the
		// scheduler already logs, not a nightly no-op worth a log line each fire.
		return TamperVerdict{}, errors.New("no off-site repo configured for this domain")
	}
	// Open the run row NOW (mirroring verify/prune) and settle it from the named
	// returns in the deferred finish below, so EVERY outcome past this point —
	// protected, unprotected, non-REST backend, transport error, inconclusive
	// probe — leaves a dated row in the activity log. Pre-fix, only a decisive
	// REST verdict recorded a run, so a non-REST (e.g. B2 object-lock) off-site's
	// scheduled tamper test ran invisibly (#109 follow-up).
	runID, rErr := s.store.StartRun(domainRunTargetID(domain), "tamper")
	if rErr != nil {
		log.Printf("api: tamper %s: could not start run record (continuing): %v", domain, rErr) //nolint:gosec // G706: domain is a fixed literal
		runID = ""
	}
	defer func() {
		if runID == "" {
			return
		}
		// Status vocabulary mirrors the rest of the runs table: "success" = probed
		// and protected; "failed" = probed and NOT protected (the alarming outcome);
		// "skipped" = the test ran but produced no verdict (non-REST backend,
		// transport error, inconclusive status) — visible, but never a false red.
		status := "success"
		detail := verdict.Detail
		switch {
		case err != nil:
			status = statusSkipped
			detail = truncateRunErr(err)
		case !verdict.Testable:
			status = statusSkipped
		case !verdict.Protected:
			status = "failed"
		}
		const maxDetail = 500 // mirror truncateRunErr's cap for the runs.error column
		if len(detail) > maxDetail {
			detail = detail[:maxDetail]
		}
		if fErr := s.store.FinishRun(runID, status, "", 0, detail); fErr != nil {
			log.Printf("api: tamper %s: could not finish run record: %v", domain, fErr) //nolint:gosec // G706: domain is a fixed literal
		}
	}()

	// Basic-auth credentials for the rest-server come from the encrypted cloud
	// config (best-effort: a decode failure just means no auth header, and the
	// server then answers 401, which this probe treats as inconclusive rather
	// than as a delete verdict).
	//
	// Resolved PER TARGET, inside the loop below. It used to be hoisted here as
	// a single decodeCloud(settings) and handed to every destination, which
	// meant a target pointing at its own named credential set (CredsRef, since
	// 7.11.0) was probed with the shared credentials instead of its own. The
	// rest-server answers 401 to that, 401 is inconclusive by design, so the
	// tamper test for exactly those destinations recorded no verdict and
	// stayed permanently "skipped" while looking like it had run. Replication
	// itself always resolved per target (offsiteModeForTarget), so only the
	// verification was blind, which is the worst half to get wrong.

	// Fold each destination's verdict worst-of: testable if ANY is testable,
	// protected only if EVERY testable destination refused the delete, details
	// joined. A destination whose probe is INCONCLUSIVE (transport/ambiguous
	// status) records no verdict and contributes its error; for N=1 that single
	// error is returned as-is (skipped run, no verdict) exactly as before.
	var (
		anyTestable  bool
		allProtected = true
		details      []string
		errs         []error
	)
	for _, t := range targets {
		creds, _ := s.decodeCloudFor(settings, t.CredsRef)
		v, perr := s.runTamperTestForTarget(ctx, domain, t, creds)
		if perr != nil {
			errs = append(errs, perr)
			continue
		}
		if !v.Testable {
			continue
		}
		anyTestable = true
		if !v.Protected {
			allProtected = false
			if v.Detail != "" {
				details = append(details, v.Detail)
			}
		}
	}
	if len(errs) > 0 {
		// Inconclusive probe(s): record no aggregate verdict and surface the error so
		// the deferred finish settles a "skipped" run. errors.Join of a single error
		// reads identically to that error for N=1.
		return TamperVerdict{}, errors.Join(errs...)
	}
	if !anyTestable {
		// No destination could be probed this way (e.g. all non-REST backends).
		return TamperVerdict{Testable: false, Detail: "only REST repos are verifiable"}, nil
	}
	verdict = TamperVerdict{Testable: true, Protected: allProtected}
	if !allProtected {
		verdict.Detail = strings.Join(details, "; ")
	}
	return verdict, nil
}

// runTamperTestForTarget probes ONE off-site destination's delete path and — on a
// decisive REST verdict — records it (offsite_target_id-stamped) and fires the
// protection-loss alert on a per-destination protected→unprotected flip. It
// returns Testable=false (nil error) for a non-REST backend, and a non-nil error
// (recording nothing) for an INCONCLUSIVE probe (transport/ambiguous status) so an
// unreachable server never flips a stored verdict. Serialisation is the caller's
// per-domain lock, so read-prev → record → notify stays atomic.
func (s *Service) runTamperTestForTarget(ctx context.Context, domain string, target store.OffsiteTarget, creds CloudCreds) (TamperVerdict, error) {
	loc := target.Repo
	// Stage-1 tamper testing speaks the REST protocol directly (raw HTTP DELETE to
	// the rest-server). Other backends (rclone/s3/sftp/local) can't be probed this
	// way — say so honestly instead of guessing a verdict.
	if !strings.HasPrefix(loc, "rest:") {
		return TamperVerdict{Testable: false, Detail: "only REST repos are verifiable"}, nil
	}
	// rest:http://host:8000/path -> http://host:8000/path (the HTTP base the
	// rest-server serves; a trailing slash is trimmed so path joins are clean).
	base := strings.TrimRight(strings.TrimPrefix(loc, "rest:"), "/")

	// Two provably non-existent object IDs, BOTH 64 hex characters ([555]).
	// Deleting them can never touch real repo data.
	//
	// The snapshot id used to be 8 hex, mirroring the short id restic prints.
	// rest-server does not accept that: every object it serves — data, index,
	// keys, snapshots alike — is named by its full 64-hex id, and a name of any
	// other length is not an object path at all. Measured against a real
	// rest-server, both with and without --append-only:
	//
	//   DELETE /repo/data/<64 hex>        403 (append-only)   200 (plain)
	//   DELETE /repo/snapshots/<64 hex>   403 (append-only)   200 (plain)
	//   DELETE /repo/snapshots/<8 hex>    404                 404
	//
	// So the short id made the second probe blind, and because a 404 was read as
	// "not protected" and the verdict is AND-ed across both probes, THE TAMPER
	// TEST COULD NEVER PASS on any rest-server, correctly configured or not.
	// Reported from a server whose own log said "Append only mode enabled".
	//
	// Never probe /locks/: measured 200 even under --append-only, because
	// deleting locks is rest-server's one documented append-only exception.
	// A locks probe would report every protected server as unprotected.
	dataID, err := randomHex(32) // 64 hex chars
	if err != nil {
		return TamperVerdict{}, err
	}
	snapID, err := randomHex(32) // 64 hex chars — see above
	if err != nil {
		return TamperVerdict{}, err
	}
	probes := []string{base + "/data/" + dataID, base + "/snapshots/" + snapID}

	protected := true
	var details []string
	for _, url := range probes {
		p, detail, perr := tamperProbe(ctx, url, creds.RESTUser, creds.RESTPassword)
		if perr != nil {
			// Transport/network error → INCONCLUSIVE. Record NO verdict and return the
			// error: an unreachable server is neither protected nor unprotected.
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

	// Read the previous verdict for THIS destination BEFORE recording the new one so
	// a protected→unprotected flip fires exactly one protection-loss alert.
	prev, hadPrev, _ := s.store.LatestTamperTestForTarget(domain, target.ID)
	if recErr := s.store.RecordTamperTestForTarget(domain, target.ID, verdict.Protected, verdict.Detail); recErr != nil {
		return TamperVerdict{}, fmt.Errorf("record tamper test: %w", recErr)
	}
	if hadPrev && prev.Protected && !verdict.Protected {
		s.notifyProtectionLost(ctx, domain, verdict.Detail)
	}
	return verdict, nil
}

// tamperProbe issues one authenticated DELETE and maps the status code to a
// protection verdict. ONLY decisive statuses yield a verdict; everything else is
// treated as INCONCLUSIVE and returned as a non-nil error (exactly like a
// transport error), so the caller records no verdict and notifies nothing rather
// than flip a stored verdict on an ambiguous response.
//
//   - 403 / 405 → protected (the delete was refused — append-only enforced)
//   - 2xx       → NOT protected (the server accepted a delete)
//   - 404 / 401 / 3xx / 5xx / anything else → INCONCLUSIVE (non-nil error): auth
//     failure (rotated creds), a redirect, or far-side/proxy maintenance is not a
//     delete verdict and must never be read as "not protected".
//
// 404 moved from "NOT protected" to inconclusive in [556], and the old reading
// was wrong on its own terms. It argued "the object did not exist, so the server
// would have deleted a real one" — but a rest-server refuses on append-only
// BEFORE it looks for the object, which is measurable: a plain server answers a
// DELETE for a non-existent 64-hex object with 200, an append-only one with 403.
// Neither answers 404. A 404 therefore means the URL was not an object path at
// all, which is the one thing we cannot draw a verdict from.
//
// Reading "I do not recognise that" as "you are unprotected" is the worst
// available default here: it does not merely mislead, it fires the
// protection-lost alert. An unknown answer must stay unknown.
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
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return false, fmt.Sprintf("the server accepted a delete (HTTP %d)", resp.StatusCode), nil
	case resp.StatusCode == http.StatusNotFound:
		// Not a verdict — see the doc comment. A rest-server answers a DELETE for a
		// non-existent object with 200 (plain) or 403 (append-only), never 404, so a
		// 404 says the URL was not an object path rather than anything about
		// protection.
		return false, "", fmt.Errorf("inconclusive tamper probe: the far side does not serve this path (HTTP 404) — check that the repository URL is the one restic itself uses")
	default:
		// 401/3xx/5xx/unexpected: not a delete verdict → inconclusive, like a
		// transport error. Returning an error makes RunTamperTest record no verdict
		// and notify nothing, so a rotated credential or a far-side maintenance
		// window can never masquerade as a lost append-only guarantee.
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
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}
