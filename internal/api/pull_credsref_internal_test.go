package api

import (
	"context"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func pullHandlerFixture(t *testing.T, appKey string) *Handler {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	cfg := config.Config{AppKey: appKey}
	svc := &Service{cfg: cfg, store: st}
	return &Handler{cfg: cfg, store: st, svc: svc}
}

// TestRepoHostExtraction covers the location shapes the auto-pick has to
// compare: rest: and s3: carry a host, sftp: carries one after the userinfo,
// and a bucket-only b2: location or a bare path carries none.
func TestRepoHostExtraction(t *testing.T) {
	cases := []struct{ loc, want string }{
		{"rest:http://192.168.1.50:8000/tower-containers", "192.168.1.50"},
		{"rest:https://box.example:8000/x", "box.example"},
		{"s3:s3.example.com/bucket/path", "s3.example.com"},
		{"s3:https://minio.internal:9000/bucket", "minio.internal"},
		{"sftp:user@box.example:/path", "box.example"},
		{"b2:bucketname:path", ""},
		{"/mnt/user/backups", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := repoHost(c.loc); got != c.want {
			t.Errorf("repoHost(%q) = %q, want %q", c.loc, got, c.want)
		}
	}
}

// A pull source left with no explicit credentials picks up an existing
// off-site target's credential set once it reaches the same host, exactly
// the case an "Offer storage" mesh setup leaves behind.
func TestBuildPullSourceAutoPicksACredSetForAMatchingHost(t *testing.T) {
	h := pullHandlerFixture(t, strings.Repeat("a", 64))
	if _, err := h.store.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "mesh: attic", Repo: "rest:http://192.168.1.50:8000/attic-containers",
		CredsRef: "cred-attic", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	ps, msg := h.buildPullSource(context.Background(), pullSourceInput{
		Name: "attic", Repo: "rest:http://192.168.1.50:8000/attic-vms", Domain: "vms",
	}, store.PullSource{}, false)
	if msg != "" {
		t.Fatalf("buildPullSource refused: %s", msg)
	}
	if ps.CredsRef != "cred-attic" {
		t.Fatalf("CredsRef = %q, want the matching host's set cred-attic", ps.CredsRef)
	}
}

// A different host must never borrow a credential set it was not issued:
// BombVault would present the wrong login to an endpoint that never gave it
// out, and a REST login rejected there is the mild failure mode.
func TestBuildPullSourceNeverPicksACredSetForAMismatchedHost(t *testing.T) {
	h := pullHandlerFixture(t, strings.Repeat("a", 64))
	if _, err := h.store.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "mesh: attic", Repo: "rest:http://192.168.1.50:8000/attic-containers",
		CredsRef: "cred-attic", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	ps, msg := h.buildPullSource(context.Background(), pullSourceInput{
		Name: "cellar", Repo: "rest:http://192.168.1.51:8000/cellar-vms", Domain: "vms",
	}, store.PullSource{}, false)
	if msg != "" {
		t.Fatalf("buildPullSource refused: %s", msg)
	}
	if ps.CredsRef != "" {
		t.Fatalf("CredsRef = %q, want none: the host does not match", ps.CredsRef)
	}
}

// A credential set already tied to another pull source at the same host is
// just as good a match as one on an off-site target.
func TestBuildPullSourceMatchesAnotherPullSourcesHost(t *testing.T) {
	h := pullHandlerFixture(t, strings.Repeat("a", 64))
	if _, err := h.store.CreatePullSource(store.PullSource{
		Name: "cellar containers", Repo: "rest:http://192.168.1.60:8000/cellar-containers",
		Domain: "containers", CredsRef: "cred-cellar", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	ps, msg := h.buildPullSource(context.Background(), pullSourceInput{
		Name: "cellar vms", Repo: "rest:http://192.168.1.60:8000/cellar-vms", Domain: "vms",
	}, store.PullSource{}, false)
	if msg != "" {
		t.Fatalf("buildPullSource refused: %s", msg)
	}
	if ps.CredsRef != "cred-cellar" {
		t.Fatalf("CredsRef = %q, want cred-cellar", ps.CredsRef)
	}
}

// An explicit choice is never overridden by the auto-pick, matched host or not.
func TestBuildPullSourceKeepsAnExplicitCredsRef(t *testing.T) {
	h := pullHandlerFixture(t, strings.Repeat("a", 64))
	if _, err := h.store.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "mesh: attic", Repo: "rest:http://192.168.1.50:8000/attic-containers",
		CredsRef: "cred-attic", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	ps, msg := h.buildPullSource(context.Background(), pullSourceInput{
		Name: "attic", Repo: "rest:http://192.168.1.50:8000/attic-vms", Domain: "vms", CredsRef: "cred-chosen-by-hand",
	}, store.PullSource{}, false)
	if msg != "" {
		t.Fatalf("buildPullSource refused: %s", msg)
	}
	if ps.CredsRef != "cred-chosen-by-hand" {
		t.Fatalf("CredsRef = %q, want the explicit choice kept", ps.CredsRef)
	}
}
