package api

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// App-specific advisories: what the exclusion assistant cannot measure.
//
// The assistant scans folders and offers the big, regenerable ones. That is a
// size question, and some of the most expensive mistakes are not size
// questions at all. The one this exists for: Immich keeps every photo's
// metadata in a PostgreSQL database that runs in a SEPARATE container, so a
// perfect file-level backup of the Immich container restores the pictures and
// loses the albums, faces, dates and sharing. No folder scan can see that.
//
// Advisories are therefore IDs, never English prose: the UI maps them to a
// translation key, an unknown id from a newer server renders nothing, and the
// 42-table parity test keeps every language honest. That is the same shape
// liveReason already uses.

func TestImmichIsAdvisedAboutItsSeparateDatabase(t *testing.T) {
	for _, image := range []string{
		"ghcr.io/immich-app/immich-server:v1.119.0",
		"ghcr.io/immich-app/immich-server",
		"ghcr.io/imagegenius/immich:latest",
		"GHCR.IO/Immich-App/Immich-Server:release", // registries are case-folded
	} {
		got := appAdvisoriesFor(image)
		if !hasAdvisory(got, advisoryImmichSeparateDB) {
			t.Errorf("image %q must carry the separate-database advisory, got %v", image, got)
		}
	}
}

// A container whose photos and database live together has no such caveat, and
// inventing one would train people to ignore the banner.
func TestAnOrdinaryImageGetsNoAdvisory(t *testing.T) {
	for _, image := range []string{
		"linuxserver/sonarr:latest",
		"plexinc/pms-docker",
		"",
		"some-private-registry.example/team/app@sha256:abc123",
	} {
		if got := appAdvisoriesFor(image); len(got) != 0 {
			t.Errorf("image %q must carry no advisory, got %v", image, got)
		}
	}
}

// The matcher works on the repository part, so a tag or a digest never changes
// the answer. A rule that only fired on ":latest" would miss every pinned
// install, which is most of them.
func TestTagsAndDigestsDoNotChangeTheMatch(t *testing.T) {
	base := appAdvisoriesFor("ghcr.io/immich-app/immich-server")
	for _, variant := range []string{
		"ghcr.io/immich-app/immich-server:v1.119.0",
		"ghcr.io/immich-app/immich-server@sha256:0123456789abcdef",
		"ghcr.io/immich-app/immich-server:v1.119.0@sha256:0123456789abcdef",
	} {
		if got := appAdvisoriesFor(variant); len(got) != len(base) {
			t.Errorf("variant %q answered differently from the bare reference: %v vs %v", variant, got, base)
		}
	}
}

// Nextcloud keeps its own caveat for the same reason: its data lives in a
// database too, and the file scan cannot say so.
func TestNextcloudIsAdvisedAboutItsDatabase(t *testing.T) {
	if got := appAdvisoriesFor("nextcloud:29-apache"); !hasAdvisory(got, advisoryNextcloudSeparateDB) {
		t.Errorf("nextcloud must carry its database advisory, got %v", got)
	}
}

func hasAdvisory(got []string, want string) bool {
	for _, g := range got {
		if g == want {
			return true
		}
	}
	return false
}

// TestAdvisoryReachesAStatelessContainer is the case the assistant would
// otherwise miss entirely.
//
// A container with no configured backup paths returns early with an empty
// suggestion list, and that early return is exactly where an Immich install
// with its media on a share nobody selected ends up. The caveat is true whether
// or not a single exclusion is offered: the database is in another container
// either way. So it has to survive the early return, not be attached to the
// suggestion list.
func TestAdvisoryReachesAStatelessContainer(t *testing.T) {
	svc, _, _ := suggestFixture(t)
	svc.docker = &suggestFakeDocker{inspect: model.Inspect{
		Name:   "/immich",
		Config: model.Config{Image: "ghcr.io/immich-app/immich-server:v1.119.0"},
	}}

	// "immich" has no target row and no backup paths, so this takes the
	// stateless early return.
	res, err := svc.SuggestExcludes(context.Background(), "immich", "")
	if err != nil {
		t.Fatalf("SuggestExcludes: %v", err)
	}
	if len(res.Suggestions) != 0 {
		t.Fatalf("precondition: expected the stateless path, got %d suggestions", len(res.Suggestions))
	}
	if !hasAdvisory(res.Advisories, advisoryImmichSeparateDB) {
		t.Fatalf("the separate-database caveat must survive the stateless early return, got %v", res.Advisories)
	}
}
