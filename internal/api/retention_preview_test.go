package api_test

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// withLocalRetention switches the local keep-policy on. twoRepoDomain leaves
// retention off, which is the honest default for a fresh install, and an
// inert policy is a no-op by design, so without this every assertion below
// would pass for the wrong reason.
func withLocalRetention(t *testing.T, st *store.Repo) {
	t.Helper()
	s, err := st.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	s.RetentionKeepLast = 5
	if err := st.UpdateSettings(s); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
}

// TestPreviewRetentionCoversEveryRepositoryOfTheDomain pins the #204 rule for
// the preview: a domain is no longer one repository. Previewing only the
// domain's own repo would give a confident answer about a domain it looked at
// half of, the exact bug the per-repository rewrite exists to prevent.
func TestPreviewRetentionCoversEveryRepositoryOfTheDomain(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, st, own, cold := twoRepoDomain(t, eng)
	withLocalRetention(t, st)

	got, err := svc.PreviewRetention(context.Background(), "containers", "local")
	if err != nil {
		t.Fatalf("PreviewRetention: %v", err)
	}
	if !hasRepo(eng.previewRepos, own) || !hasRepo(eng.previewRepos, cold) {
		t.Fatalf("previewed = %v, want both %q and %q", eng.previewRepos, own, cold)
	}
	if len(got.Repos) != 2 {
		t.Fatalf("want a report per repository, got %d: %+v", len(got.Repos), got.Repos)
	}
}

// TestPreviewRetentionNeverWrites is the promise the whole feature rests on.
// A preview answers a question; it must not forget, prune, or clear a lock,
// not even the "stale" clear the real retention pass opens with, because a
// read-only endpoint that deletes lock files is a repository writer.
func TestPreviewRetentionNeverWrites(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, st, _, _ := twoRepoDomain(t, eng)
	withLocalRetention(t, st)

	if _, err := svc.PreviewRetention(context.Background(), "containers", "local"); err != nil {
		t.Fatalf("PreviewRetention: %v", err)
	}
	if len(eng.forgetTags) != 0 || len(eng.prunedRepos) != 0 {
		t.Fatalf("a preview must not forget: tags=%v pruned=%v", eng.forgetTags, eng.prunedRepos)
	}
	if len(eng.manualPruned) != 0 {
		t.Fatalf("a preview must not prune, got %v", eng.manualPruned)
	}
	if len(eng.unlockedRepos) != 0 {
		t.Fatalf("a preview must not clear locks, got Unlock on %v", eng.unlockedRepos)
	}
}

// TestPreviewRetentionUsesEachRepositoriesOwnCredentials pins that the mode is
// built per repository rather than hoisted out of the loop. The named
// repository in twoRepoDomain carries a marker upload cap, so a hoisted mode
// shows up as the wrong cap on the wrong repo instead of silently working.
func TestPreviewRetentionUsesEachRepositoriesOwnCredentials(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, st, _, cold := twoRepoDomain(t, eng)
	withLocalRetention(t, st)

	if _, err := svc.PreviewRetention(context.Background(), "containers", "local"); err != nil {
		t.Fatalf("PreviewRetention: %v", err)
	}
	var found bool
	for i, repo := range eng.previewRepos {
		// Compared through filepath.Clean for the same reason hasRepo does it:
		// the stored location and the one restic is handed differ in separator
		// on Windows, and a raw != would silently never match.
		if filepath.Clean(repo) != filepath.Clean(cold) {
			continue
		}
		found = true
		if eng.previewModes[i].Limits.UploadKBps != 4242 {
			t.Fatalf("named repo previewed with the wrong mode: %+v", eng.previewModes[i])
		}
	}
	if !found {
		t.Fatalf("the named repository was never previewed: %v", eng.previewRepos)
	}
}

// TestPreviewRetentionReportsAnAppendOnlyRepositoryAsSuch is the most dangerous
// false positive this feature could produce. An append-only archive is never
// pruned from this box at all (pruneDomain refuses outright), so showing a
// removal list for one would tell an operator their immutable off-site copy is
// about to be trimmed. It has to be named as append-only, and it must not even
// be asked.
func TestPreviewRetentionReportsAnAppendOnlyRepositoryAsSuch(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, st, _, cold := twoRepoDomain(t, eng)
	withLocalRetention(t, st)

	rows, err := st.ListNamedRepos()
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListNamedRepos: %v (%d rows)", err, len(rows))
	}
	row := rows[0]
	row.Immutable = true
	if _, err := st.UpsertOffsiteTarget(row); err != nil {
		t.Fatal(err)
	}

	got, err := svc.PreviewRetention(context.Background(), "containers", "local")
	if err != nil {
		t.Fatalf("PreviewRetention: %v", err)
	}
	if hasRepo(eng.previewRepos, cold) {
		t.Fatalf("an append-only repository must not be previewed at all, got %v", eng.previewRepos)
	}
	var marked bool
	for _, r := range got.Repos {
		if r.AppendOnly {
			marked = true
			if len(r.Items) != 0 {
				t.Fatalf("an append-only repository must show no removals, got %+v", r.Items)
			}
		}
	}
	if !marked {
		t.Fatalf("the append-only repository was not reported as such: %+v", got.Repos)
	}
}

// TestRetentionPreviewRoute pins the HTTP surface: a GET (so the CSRF gate lets
// it through untouched while the session gate still protects it), the same
// domain whitelist the prune route uses, and the ok/fail envelope every other
// handler answers with.
func TestRetentionPreviewRoute(t *testing.T) {
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	// A domain that has never been backed up is a STATE, not a crash: the route
	// answers 200 with the same ok/fail envelope every other handler uses and
	// says why, rather than 500-ing or returning an empty object the panel would
	// have to guess at. (The populated case is covered against a real
	// two-repository domain in the service tests above.)
	t.Run("a domain with no repository yet is explained, not crashed", func(t *testing.T) {
		w, m := doJSON(t, h, http.MethodGet, "/api/retention/preview/containers", "")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want a graceful 200", w.Code)
		}
		if m["ok"] != false {
			t.Fatalf("expected ok:false for a domain with no backups, got %v", m)
		}
		errStr, _ := m["error"].(string)
		if !strings.Contains(errStr, "no backups") {
			t.Fatalf("expected the answer to say there are no backups yet, got %q", errStr)
		}
	})

	t.Run("unknown domain is refused", func(t *testing.T) {
		w, m := doJSON(t, h, http.MethodGet, "/api/retention/preview/nonsense", "")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
		if m["ok"] != false {
			t.Fatalf("expected ok:false, got %v", m)
		}
	})
}

// A preview that showed a held item's snapshots as about to be removed would
// contradict the pass it exists to mirror.
func TestRetentionPreviewMarksPausedItem(t *testing.T) {
	eng := &fakeResticEngine{previewGroups: []restic.ForgetGroup{{
		Keep:   []restic.Snapshot{snapTagged("a1", "fileset:docs")},
		Remove: []restic.Snapshot{snapTagged("z9", "fileset:docs")},
	}}}
	svc, _, _ := heldRepo(t, eng)

	preview, err := svc.PreviewRetention(context.Background(), "files", "local")
	if err != nil {
		t.Fatalf("PreviewRetention: %v", err)
	}
	seen := 0
	for _, repo := range preview.Repos {
		for _, item := range repo.Items {
			if item.Tag != "container:plex" {
				continue
			}
			seen++
			if !item.Paused || len(item.Remove) != 0 {
				t.Fatalf("a held item previews as paused with nothing to remove, got %+v", item)
			}
		}
	}
	if seen != 1 {
		t.Fatalf("want the held item in the preview once, got %d: %+v", seen, preview.Repos)
	}
}
