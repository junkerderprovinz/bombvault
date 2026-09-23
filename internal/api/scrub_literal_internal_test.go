package api

import (
	"context"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// absPathRe cannot tell a path from a slash inside a word, so scrubError would
// turn "BombVault/restic" into "BombVault[path]". The regexp stays strict and
// our own messages avoid such slashes instead.
func TestOurOwnSentencesSurviveTheScrubber(t *testing.T) {
	svc := &Service{
		cfg:    config.Config{AppKey: strings.Repeat("a", 64), HostMountRoot: "/host/user"},
		engine: restic.Restic{Bin: "restic"},
	}

	// A valid relative path that holds no repository.
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

	_, _, ferr := svc.OpenForeign(context.Background(), "no-such-repo-here", strings.Repeat("b", 64), nil)
	if ferr == nil {
		t.Fatal("opening a non-existent foreign repo must fail")
	}
	if s := scrubError(ferr); strings.Contains(s, "[path]") {
		t.Errorf("the scrubber mangled the foreign sentence: %q", s)
	}
}
