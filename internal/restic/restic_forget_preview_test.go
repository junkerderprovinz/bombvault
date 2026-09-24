package restic

import (
	"context"
	"testing"
)

// TestForgetPreviewInertPolicy pins the same guard ForgetPolicy carries: with
// no keep dimension set, retention is OFF and restic is never invoked at all.
//
// This is not a micro-optimisation. `restic forget` with no --keep-* flag keeps
// nothing, so a preview that reached the binary with an inert policy would
// report every snapshot in the repository as about to be removed, and the
// operator would be looking at a list that says their whole history is going
// away, produced by a policy that in reality does nothing.
//
// The repository path below does not exist and the binary name is nonsense on
// purpose: if the guard ever stops working, the call reaches exec and the test
// fails with an error instead of returning nil.
func TestForgetPreviewInertPolicy(t *testing.T) {
	r := Restic{Bin: "restic-does-not-exist"}
	groups, err := r.ForgetPreview(context.Background(), "/no/such/repo", RetentionPolicy{}, Mode{Encrypted: true}, "container:plex")
	if err != nil {
		t.Fatalf("an inert policy must be a no-op, got error: %v", err)
	}
	if groups != nil {
		t.Fatalf("an inert policy must report nothing, got %+v", groups)
	}
}

// TestParseForgetGroups pins how `restic forget --json` output is read back.
//
// The empty case is the one that matters in practice. restic 0.17.3 guards its
// JSON print with `if gopts.JSON && len(jsonGroups) > 0`, so a policy that
// matches no group prints NOTHING AT ALL, not even `[]`. That is the perfectly
// normal "nothing would be removed" answer, and a naive json.Unmarshal turns it
// into a parse error on the user's screen.
func TestParseForgetGroups(t *testing.T) {
	t.Run("empty output is zero groups, not an error", func(t *testing.T) {
		for _, out := range [][]byte{nil, {}, []byte("   \n\t "), []byte("\n")} {
			groups, err := parseForgetGroups(out)
			if err != nil {
				t.Fatalf("empty restic output must parse as nothing to remove, got error: %v", err)
			}
			if len(groups) != 0 {
				t.Fatalf("got %d groups, want 0", len(groups))
			}
		}
	})

	t.Run("a real group carries its keep and remove snapshots", func(t *testing.T) {
		out := []byte(`[
		  {
		    "tags": ["container:plex"],
		    "host": "tower",
		    "paths": ["/host/user/appdata/plex"],
		    "keep": [
		      {"id": "1a1a1a1a", "short_id": "1a1a", "time": "2026-09-15T02:00:00.000000000+02:00",
		       "paths": ["/host/user/appdata/plex"], "tags": ["container:plex"], "hostname": "tower"}
		    ],
		    "remove": [
		      {"id": "2b2b2b2b", "short_id": "2b2b", "time": "2026-09-14T02:00:00.000000000+02:00",
		       "paths": ["/host/user/appdata/plex"], "tags": ["container:plex"], "hostname": "tower"},
		      {"id": "3c3c3c3c", "short_id": "3c3c", "time": "2026-09-13T02:00:00.000000000+02:00",
		       "paths": ["/host/user/appdata/plex"], "tags": ["container:plex"], "hostname": "tower"}
		    ]
		  }
		]`)
		groups, err := parseForgetGroups(out)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(groups) != 1 {
			t.Fatalf("got %d groups, want 1", len(groups))
		}
		g := groups[0]
		if len(g.Keep) != 1 || g.Keep[0].ID != "1a1a1a1a" {
			t.Fatalf("keep: got %+v", g.Keep)
		}
		if len(g.Remove) != 2 {
			t.Fatalf("got %d removals, want 2", len(g.Remove))
		}
		if g.Remove[0].ID != "2b2b2b2b" || g.Remove[1].ID != "3c3c3c3c" {
			t.Fatalf("remove ids: got %+v", g.Remove)
		}
		if g.Remove[0].Time != "2026-09-14T02:00:00.000000000+02:00" {
			t.Fatalf("remove time: got %q", g.Remove[0].Time)
		}
		if len(g.Tags) != 1 || g.Tags[0] != "container:plex" {
			t.Fatalf("tags: got %v", g.Tags)
		}
	})

	t.Run("malformed output is reported as a parse error", func(t *testing.T) {
		if _, err := parseForgetGroups([]byte("not json")); err == nil {
			t.Fatal("want a parse error, got nil")
		}
	})
}
