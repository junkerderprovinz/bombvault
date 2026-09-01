package api

import (
	"context"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestOurOwnSentencesSurviveTheScrubber ([584]).
//
// Every error leaving this package goes through scrubError, which redacts
// absolute paths with absPathRe — `(/[^\s:"']+)+`. That regexp cannot tell a
// filesystem path from a slash inside a word, so a message of OURS that
// contains one comes out mangled. "not a BombVault/restic repository" reached
// users as "not a BombVault[path] repository", and it was reported from the
// forum exactly that way.
//
// The redaction is correct and must not be loosened: the fix is that our own
// prose does not put a slash where a path could be. This pins that for the two
// sentences it happened to, which are the two most likely to be read by someone
// already confused about why their repository will not open.
func TestOurOwnSentencesSurviveTheScrubber(t *testing.T) {
	svc := &Service{
		cfg:    config.Config{AppKey: strings.Repeat("a", 64), HostMountRoot: "/host/user"},
		engine: restic.Restic{Bin: "restic"},
	}

	// A relative subpath that resolves fine but holds no repository: the open
	// fails and produces the sentence under test.
	rr := makeReceivedRepo(t, strings.Repeat("a", 64), strings.Repeat("b", 64), "no-such-repo-here", 0)
	_, _, err := svc.receiverOpen(context.Background(), rr)
	if err == nil {
		t.Fatal("opening a non-existent repo must fail")
	}
	scrubbed := scrubError(err)
	if strings.Contains(scrubbed, "[path]") {
		t.Errorf("the scrubber mangled our own sentence: %q", scrubbed)
	}
	if !strings.Contains(scrubbed, "BombVault or restic") {
		t.Errorf("the sentence must name both engines readably, got %q", scrubbed)
	}

	// The foreign flow carries the same sentence and the same hazard.
	_, _, ferr := svc.OpenForeign(context.Background(), "no-such-repo-here", strings.Repeat("b", 64))
	if ferr == nil {
		t.Fatal("opening a non-existent foreign repo must fail")
	}
	if s := scrubError(ferr); strings.Contains(s, "[path]") {
		t.Errorf("the scrubber mangled the foreign sentence: %q", s)
	}
	_ = store.ReceivedRepo{}
}
