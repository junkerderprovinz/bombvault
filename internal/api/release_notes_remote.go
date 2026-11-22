package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// releaseByTagURL answers with one of BombVault's releases when the tag is
// appended. It is a variable so a test can point it at a server of its own.
var releaseByTagURL = "https://api.github.com/repos/junkerderprovinz/bombvault/releases/tags/"

// Only the release the last update check found newer than this build is
// fetched, so a made-up tag costs no request to GitHub. One answer serves
// every page for a while, and a failure is asked again sooner.
const (
	remoteNotesFresh  = time.Hour
	remoteNotesFailed = 5 * time.Minute
)

var plainTag = regexp.MustCompile(`^v\d{1,4}\.\d{1,4}\.\d{1,4}$`)

// errNotFound answers a tag no update check has named.
var errNotFound = errors.New("no update check found this release")

// remoteNotes holds the release the last update check found newer than this
// build, and GitHub's answer about its notes with when it asked.
type remoteNotes struct {
	mu      sync.Mutex
	found   string
	body    string
	err     error
	checked time.Time
}

var githubNotes = &remoteNotes{}

// allow names the release whose notes get may fetch.
func (n *remoteNotes) allow(tag string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if tag != n.found {
		n.found, n.body, n.err, n.checked = tag, "", nil, time.Time{}
	}
}

// get answers from memory while the last answer is fresh. The lock is held
// during the request, so pages that ask at once share one call.
func (n *remoteNotes) get(client *http.Client, base, tag string) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if tag == "" || tag != n.found {
		return "", errNotFound
	}
	keep := remoteNotesFresh
	if n.err != nil {
		keep = remoteNotesFailed
	}
	if !n.checked.IsZero() && time.Since(n.checked) < keep {
		return n.body, n.err
	}
	// A page that closes mid-request does not cancel the answer the next one gets.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n.body, n.err = fetchReleaseNotes(ctx, client, base+tag)
	n.checked = time.Now()
	return n.body, n.err
}

func fetchReleaseNotes(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // G704: GitHub's API and a tag that matched plainTag
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "BombVault")
	resp, err := client.Do(req) //nolint:gosec // G704: GitHub's API and a tag that matched plainTag
	if err != nil {
		return "", fmt.Errorf("GitHub could not be reached: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // nothing to do about a failed close of a response that was read
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub answered HTTP %d", resp.StatusCode)
	}
	var release struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return "", fmt.Errorf("GitHub's answer is not a release: %w", err)
	}
	if release.Body == "" {
		return "", errors.New("the release has no notes")
	}
	return release.Body, nil
}
