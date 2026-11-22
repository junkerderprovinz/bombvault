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

// Only a release this build does not carry is fetched, which is a newer one
// the update check found. One answer serves every page for a while, and a
// failure is asked again sooner.
const (
	remoteNotesFresh  = time.Hour
	remoteNotesFailed = 5 * time.Minute
	// More tags than releases exist only come from someone making them up.
	remoteNotesKeys = 64
)

var plainTag = regexp.MustCompile(`^v\d{1,4}\.\d{1,4}\.\d{1,4}$`)

type remoteNotesEntry struct {
	body    string
	err     error
	checked time.Time
}

// remoteNotes remembers the notes GitHub gave for each tag and when it asked.
type remoteNotes struct {
	mu      sync.Mutex
	entries map[string]remoteNotesEntry
}

var githubNotes = &remoteNotes{}

// get answers from memory while the last answer for tag is fresh. The lock is
// held during the request, so pages that ask at once share one call.
func (n *remoteNotes) get(client *http.Client, base, tag string) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if e, ok := n.entries[tag]; ok {
		keep := remoteNotesFresh
		if e.err != nil {
			keep = remoteNotesFailed
		}
		if time.Since(e.checked) < keep {
			return e.body, e.err
		}
	}
	if n.entries == nil || len(n.entries) >= remoteNotesKeys {
		n.entries = map[string]remoteNotesEntry{}
	}
	// A page that closes mid-request does not cancel the answer the next one gets.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	body, err := fetchReleaseNotes(ctx, client, base+tag)
	n.entries[tag] = remoteNotesEntry{body: body, err: err, checked: time.Now()}
	return body, err
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
