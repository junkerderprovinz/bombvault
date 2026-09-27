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
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TamperVerdict is the result of an off-site tamper test, which probes the far
// side's delete path to check that append-only is enforced. Only REST repos can
// be probed; for any other, Testable is false and Detail says why. Protected is
// true only when every probe's delete was refused (403/405); otherwise Detail
// names what the server accepted.
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

// RunTamperTest checks a domain's off-site destinations: it sends two
// authenticated HTTP DELETEs for object IDs that cannot exist and reads the
// status codes. Only rest: repos can be tested this way.
//
// A transport error or an ambiguous status is inconclusive: RunTamperTest then
// returns an error and records no verdict. A recorded flip from protected to
// unprotected sends a protection-loss notification. Every outcome settles a run
// row ("success" protected, "failed" not protected, "skipped" no verdict), so a
// scheduled test against a non-REST off-site still shows up.
func (s *Service) RunTamperTest(ctx context.Context, domain string) (verdict TamperVerdict, err error) {
	switch domain {
	case "containers", "vms", "flash", "config", "files", "zfs":
	default:
		return TamperVerdict{}, fmt.Errorf("unknown domain %q", domain)
	}
	// Serialise per domain, so reading the previous verdict, recording the new
	// one and alerting cannot interleave and a flip alerts exactly once.
	defer s.lockTamper(domain)()
	// The end event is deferred so an early return or panic cannot leave a stuck
	// line in the activity log.
	tkey := "tamper:" + domain
	_, startedAt := s.progBegin(ctx, tkey, "maintenance")
	defer func() { s.progEnd(tkey, "maintenance", err == nil, startedAt) }()
	settings, err := s.store.GetSettings()
	if err != nil {
		return TamperVerdict{}, fmt.Errorf("read settings: %w", err)
	}
	// Each destination is probed in turn and the verdicts are folded worst-of,
	// with one run row and progress line per domain, like copyToOffsite.
	targets := s.offsiteReplicationTargets(domain, settings)
	if len(targets) == 0 {
		// No run row: a manual caller gets the error, and the scheduler already
		// logs an immutable flag without a repo.
		return TamperVerdict{}, errNoOffsiteRepo
	}
	// Open the run row now and settle it from the named returns, so every
	// outcome from here on leaves a dated row.
	runID := s.startTamperRun(ctx, domain)
	defer func() { s.finishTamperRun(runID, domain, verdict, err) }()

	var fold tamperFold
	for _, t := range targets {
		// A target may use its own named credential set, so credentials are
		// resolved per target. Wrong or missing credentials get a 401, which is
		// inconclusive.
		creds, _ := s.decodeCloudFor(settings, t.CredsRef)
		fold.add(s.runTamperTestForTarget(ctx, domain, t, creds))
	}
	// An error leaves no verdict; the deferred finish records a skipped run.
	return fold.verdict()
}

// startTamperRun opens a tamper run row on the domain. It returns "" when the
// store cannot, since the test itself can still run.
func (s *Service) startTamperRun(ctx context.Context, domain string) string {
	runID, err := s.startRun(ctx, domainRunTargetID(domain), "tamper")
	if err != nil {
		log.Printf("api: tamper %s: could not start run record (continuing): %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		return ""
	}
	return runID
}

// finishTamperRun settles a tamper run row from the test's outcome: success
// when every delete was refused, failed when one was accepted, and skipped,
// visible but not red, when the test ran without a verdict.
func (s *Service) finishTamperRun(runID, domain string, verdict TamperVerdict, err error) {
	if runID == "" {
		return
	}
	status, detail := "success", verdict.Detail
	switch {
	case err != nil:
		status, detail = statusSkipped, truncateRunErr(err)
	case !verdict.Testable:
		status = statusSkipped
	case !verdict.Protected:
		status = "failed"
	}
	const maxDetail = 500 // truncateRunErr's cap for runs.error
	if len(detail) > maxDetail {
		detail = detail[:maxDetail]
	}
	if fErr := s.store.FinishRun(runID, status, "", 0, detail); fErr != nil {
		log.Printf("api: tamper %s: could not finish run record: %v", domain, fErr) //nolint:gosec // G706: domain is a fixed literal
	}
}

// tamperFold folds tamper verdicts worst-of: testable if any repository is,
// protected only if every testable one refused the delete. An inconclusive
// probe contributes its error instead of a verdict, and any error leaves the
// fold without one.
type tamperFold struct {
	testable bool
	open     bool
	details  []string
	errs     []error
}

func (f *tamperFold) add(v TamperVerdict, err error) {
	switch {
	case err != nil:
		f.errs = append(f.errs, err)
	case v.Testable:
		f.testable = true
		if !v.Protected {
			f.open = true
			if v.Detail != "" && !slices.Contains(f.details, v.Detail) {
				f.details = append(f.details, v.Detail)
			}
		}
	}
}

func (f *tamperFold) verdict() (TamperVerdict, error) {
	switch {
	case len(f.errs) > 0:
		return TamperVerdict{}, errors.Join(f.errs...)
	case !f.testable:
		return TamperVerdict{Testable: false, Detail: "only REST repos are verifiable"}, nil
	}
	return TamperVerdict{Testable: true, Protected: !f.open, Detail: strings.Join(f.details, "; ")}, nil
}

// RunPlaceTamperTest folds the verdicts of the repositories at one place: the
// path of each domain it is home to, each enabled copy and each enabled named
// or direct repository it holds. Copies at other places play no part, so a
// second server without append-only does not speak for this one.
func (s *Service) RunPlaceTamperTest(ctx context.Context, id string) (TamperVerdict, error) {
	p, err := s.store.GetPlace(id)
	if err != nil {
		return TamperVerdict{}, err
	}
	switch {
	case p.Kind == string(places.KindLocal):
		return TamperVerdict{}, errLocalAppendOnly
	case !p.Enabled:
		return TamperVerdict{}, errPlaceOff
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return TamperVerdict{}, fmt.Errorf("read settings: %w", err)
	}
	byDomain, named, err := s.placeTamperRepos(id, settings)
	if err != nil {
		return TamperVerdict{}, err
	}
	if len(byDomain) == 0 && len(named) == 0 {
		return TamperVerdict{}, errPlaceNothingToTest
	}
	var fold tamperFold
	for _, d := range places.Domains {
		if len(byDomain[d]) > 0 {
			fold.add(s.tamperTestPlaceDomain(ctx, settings, d, byDomain[d]))
		}
	}
	for _, r := range named {
		creds, _ := s.decodeCloudFor(settings, r.CredsRef)
		fold.add(probeDeletes(ctx, r.Repo, creds))
	}
	return fold.verdict()
}

// placeTamperRepos lists what a place test probes: by domain the path of each
// domain whose home the place is, carrying the domain's primary row, and each
// enabled copy there, then the enabled named and direct repositories. The
// move onto places leaves a primary row off its home place, so the row is
// looked up by domain. A named or direct repository has no domain to keep a
// verdict under, so it is probed without one.
func (s *Service) placeTamperRepos(id string, settings store.Settings) (map[string][]store.OffsiteTarget, []store.OffsiteTarget, error) {
	homes, err := s.store.DomainPlaces()
	if err != nil {
		return nil, nil, err
	}
	byDomain := map[string][]store.OffsiteTarget{}
	for _, d := range places.Domains {
		if homes[d] != id {
			continue
		}
		path, _, err := s.store.PrimaryRemoteTarget(d)
		if err != nil {
			return nil, nil, err
		}
		path.Domain, path.Repo = d, domainPathRaw(d, settings)
		byDomain[d] = append(byDomain[d], path)
	}
	rows, err := s.store.PlaceRows(id)
	if err != nil {
		return nil, nil, err
	}
	var named []store.OffsiteTarget
	for _, r := range rows {
		switch {
		case !r.Enabled:
		case r.Role == store.RoleOffsite:
			byDomain[r.Domain] = append(byDomain[r.Domain], r)
		case r.Role == store.RoleRepo:
			named = append(named, r)
		}
	}
	return byDomain, named, nil
}

// tamperTestPlaceDomain probes one domain's repositories at a place, each
// with its own credentials, under the domain's lock and with the progress
// line and run row the domain test writes. A domain path without a primary
// row keeps no verdict: under the empty id its last one would be whatever
// another target of the domain got.
func (s *Service) tamperTestPlaceDomain(ctx context.Context, settings store.Settings, domain string, repos []store.OffsiteTarget) (verdict TamperVerdict, err error) {
	defer s.lockTamper(domain)()
	tkey := "tamper:" + domain
	_, startedAt := s.progBegin(ctx, tkey, "maintenance")
	defer func() { s.progEnd(tkey, "maintenance", err == nil, startedAt) }()
	runID := s.startTamperRun(ctx, domain)
	defer func() { s.finishTamperRun(runID, domain, verdict, err) }()

	var fold tamperFold
	for _, r := range repos {
		creds, _ := s.decodeCloudFor(settings, r.CredsRef)
		if r.ID == "" {
			fold.add(probeDeletes(ctx, r.Repo, creds))
			continue
		}
		fold.add(s.runTamperTestForTarget(ctx, domain, r, creds))
	}
	return fold.verdict()
}

// runTamperTestForTarget probes one off-site destination's delete path. A
// decisive verdict is recorded for that target, and a flip from protected to
// unprotected sends the protection-loss alert. A non-REST backend returns
// Testable false; an inconclusive probe returns an error and records nothing, so
// an unreachable server never flips a stored verdict. The caller holds the
// per-domain lock.
func (s *Service) runTamperTestForTarget(ctx context.Context, domain string, target store.OffsiteTarget, creds CloudCreds) (TamperVerdict, error) {
	verdict, err := probeDeletes(ctx, target.Repo, creds)
	if err != nil || !verdict.Testable {
		return verdict, err
	}
	// Read the previous verdict before recording the new one, so a flip to
	// unprotected alerts exactly once.
	prev, hadPrev, _ := s.store.LatestTamperTestForTarget(domain, target.ID)
	if recErr := s.store.RecordTamperTestForTarget(domain, target.ID, verdict.Protected, verdict.Detail); recErr != nil {
		return TamperVerdict{}, fmt.Errorf("record tamper test: %w", recErr)
	}
	if hadPrev && prev.Protected && !verdict.Protected {
		s.notifyProtectionLost(ctx, domain, verdict.Detail)
	}
	return verdict, nil
}

// probeDeletes sends both probes to the repository at loc and folds their
// answers into a verdict, recording nothing.
func probeDeletes(ctx context.Context, loc string, creds CloudCreds) (TamperVerdict, error) {
	// The probe is a raw HTTP DELETE to rest-server; rclone, s3, sftp and local
	// repos cannot be tested this way.
	if !strings.HasPrefix(loc, "rest:") {
		return TamperVerdict{Testable: false, Detail: "only REST repos are verifiable"}, nil
	}
	// rest:http://host:8000/path becomes http://host:8000/path, without a
	// trailing slash.
	base := strings.TrimRight(strings.TrimPrefix(loc, "rest:"), "/")

	// Two random object IDs, which cannot name real repo data. rest-server names
	// every object by its full 64-hex ID; any other length is not an object path.
	// Measured with and without --append-only:
	//
	//   DELETE /repo/data/<64 hex>        403 (append-only)   200 (plain)
	//   DELETE /repo/snapshots/<64 hex>   403 (append-only)   200 (plain)
	//   DELETE /repo/snapshots/<8 hex>    404                 404
	//
	// /locks/ is not probed: deleting locks is rest-server's documented
	// append-only exception and returns 200 either way.
	dataID, err := randomHex(32) // 64 hex chars
	if err != nil {
		return TamperVerdict{}, err
	}
	snapID, err := randomHex(32) // 64 hex chars
	if err != nil {
		return TamperVerdict{}, err
	}
	probes := []string{base + "/data/" + dataID, base + "/snapshots/" + snapID}

	protected := true
	var details []string
	for _, url := range probes {
		p, detail, perr := tamperProbe(ctx, url, creds.RESTUser, creds.RESTPassword)
		if perr != nil {
			// An unreachable server is neither protected nor unprotected.
			return TamperVerdict{}, perr
		}
		if !p {
			protected = false
		}
		if detail != "" && !slices.Contains(details, detail) {
			details = append(details, detail)
		}
	}

	verdict := TamperVerdict{Testable: true, Protected: protected}
	if !protected {
		verdict.Detail = strings.Join(details, "; ")
	}
	return verdict, nil
}

// tamperProbe sends one authenticated DELETE and maps the status code to a
// verdict:
//
//   - 403 or 405: protected, the delete was refused
//   - 2xx: not protected, the server accepted a delete
//   - anything else: inconclusive, returned as an error like a transport error
//
// A 401, a redirect or a far-side outage says nothing about deletes, and
// neither does a 404: rest-server checks append-only before it looks for the
// object, so a delete of a missing 64-hex object gets 200 or 403. A 404 means
// the URL is not an object path. Reading an unknown answer as unprotected would
// fire the protection-lost alert.
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
		return false, "", err // transport error: inconclusive
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable
	_, _ = io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusMethodNotAllowed:
		return true, "", nil
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return false, fmt.Sprintf("the server accepted a delete (HTTP %d)", resp.StatusCode), nil
	case resp.StatusCode == http.StatusNotFound:
		// Not a verdict; see the doc comment.
		return false, "", fmt.Errorf("inconclusive tamper probe: the far side does not serve this path (HTTP 404); check that the repository URL is the one restic itself uses")
	default:
		// A rotated credential or far-side maintenance must not look like lost
		// protection.
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

// notifyProtectionLost sends a best-effort alert when a tamper test that passed
// before now fails. Like notifyDrillFailure it also notifies Unraid, and it does
// nothing when notifications are off.
func (s *Service) notifyProtectionLost(ctx context.Context, domain, detail string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	subject := "Off-site protection LOST for " + domain
	msg := fmt.Sprintf("The off-site tamper test for %s reports that the append-only protection is gone. The far side accepted a delete: %s", domain, detail)
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: subject + ": " + msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: "+subject, msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// handleTamperTest runs an active off-site tamper test for a domain: it probes the
// far-side rest-server's delete path with side-effect-free DELETEs to verify the
// append-only protection is actually enforced (not just configured).
// POST /api/offsite/{domain}/tamper-test
func (h *Handler) handleTamperTest(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	switch domain {
	case "containers", "vms", "flash", "config", "files", "zfs":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	verdict, err := h.svc.RunTamperTest(r.Context(), domain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"testable":  verdict.Testable,
		"protected": verdict.Protected,
		"detail":    verdict.Detail,
	}))
}

// handlePlaceTamperTest serves POST /api/places/{id}/tamper-test, one verdict
// for the repositories at the place.
func (h *Handler) handlePlaceTamperTest(w http.ResponseWriter, r *http.Request) {
	verdict, err := h.svc.RunPlaceTamperTest(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, store.ErrPlaceNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such place"})
	case err != nil:
		placementFail(w, err, nil)
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
			"testable":  verdict.Testable,
			"protected": verdict.Protected,
			"detail":    verdict.Detail,
		}))
	}
}
