package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// latestReleaseURL answers with BombVault's newest published release. It is a
// variable so a test can point the check at a server of its own.
var latestReleaseURL = "https://api.github.com/repos/junkerderprovinz/bombvault/releases/latest"

const releasePageURL = "https://github.com/junkerderprovinz/bombvault/releases/tag/"

// updateHTTPClient sends the update check. It keeps the default transport, so
// a proxy named in the environment is used.
var updateHTTPClient = &http.Client{Timeout: 10 * time.Second}

// The reasons a check cannot run, as the codes the page translates.
const (
	updateCheckDevBuild    = "dev-build"
	updateCheckOffline     = "offline"
	updateCheckRateLimited = "rate-limited"
	updateCheckBadResponse = "bad-response"
)

// UpdateCheck is the answer to one press of "Check for updates".
type UpdateCheck struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl"`
}

// checkForUpdate asks url for the newest release and compares it with the
// running version as semantic versions. When the check cannot run, the string
// says why in one of the updateCheck codes.
func checkForUpdate(ctx context.Context, client *http.Client, url, running string) (UpdateCheck, string, error) {
	current := running
	if !strings.HasPrefix(current, "v") {
		current = "v" + current
	}
	// A build without a version has nothing to compare, so it asks nobody.
	if !semver.IsValid(current) {
		return UpdateCheck{}, updateCheckDevBuild, fmt.Errorf("this build carries no version (%q)", running)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return UpdateCheck{}, updateCheckOffline, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "BombVault/"+strings.TrimPrefix(current, "v"))
	resp, err := client.Do(req)
	if err != nil {
		return UpdateCheck{}, updateCheckOffline, fmt.Errorf("GitHub could not be reached: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // nothing to do about a failed close of a response that was read

	if rateLimited(resp) {
		return UpdateCheck{}, updateCheckRateLimited, fmt.Errorf("GitHub is rate limiting this address (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return UpdateCheck{}, updateCheckBadResponse, fmt.Errorf("GitHub answered HTTP %d", resp.StatusCode)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return UpdateCheck{}, updateCheckBadResponse, fmt.Errorf("GitHub's answer is not a release: %w", err)
	}
	// The tag ends up in a link, so only a plain version is taken from it.
	if !semver.IsValid(release.TagName) {
		return UpdateCheck{}, updateCheckBadResponse, fmt.Errorf("the newest release carries no version (%q)", release.TagName)
	}
	return UpdateCheck{
		Current:         running,
		Latest:          release.TagName,
		UpdateAvailable: semver.Compare(release.TagName, current) > 0,
		ReleaseURL:      releasePageURL + release.TagName,
	}, "", nil
}

// rateLimited tells GitHub's rate limit from any other refusal. GitHub answers
// 403 for both, and a proxy on the way out can answer 403 as well.
func rateLimited(resp *http.Response) bool {
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusForbidden:
		return resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != ""
	}
	return false
}

// handleUpdateCheck is the one place BombVault asks the outside about itself.
// It runs when a person presses the button and at no other time.
func (h *Handler) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	check, reason, err := checkForUpdate(r.Context(), updateHTTPClient, latestReleaseURL, Version)
	if err != nil {
		log.Printf("update check: %v", err)
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, reason))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"update": check}))
}
