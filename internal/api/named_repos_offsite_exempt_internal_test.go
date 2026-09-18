package api

// ---------------------------------------------------------------------------
// A LOCAL named repository marked "already off site" (#204 follow-up). Why the
// mark exists is on alreadyOffSite; these tests pin both replication paths, the
// PATCH the Repositories card sends, and the settings file.
//
// Internal, for the same reason offsite_hook_retention_internal_test.go is:
// replicateOffsite and offsiteReplicationSources are unexported.
// ---------------------------------------------------------------------------

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func markTheNamedRepositoryOffSite(t *testing.T, st *store.Repo) {
	t.Helper()
	editTheNamedRepository(t, st, func(r *store.OffsiteTarget) { r.AlreadyOffsite = true })
}

// TestTheHookCopiesALocalNamedRepositoryUnlessMarked drives "Back up now" on an
// item whose repository is a local share. On an install with no separate
// off-site schedule this hook IS the replication, so it is the path the reporter
// hit. The unmarked case is the control: without it the marked case could pass
// on a fixture that never copies anything.
func TestTheHookCopiesALocalNamedRepositoryUnlessMarked(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mark       bool
		wantCopied bool
	}{
		{"unmarked share is copied", false, true},
		{"marked share is left alone", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eng := &hookFakeEngine{snapsByRepo: map[string][]restic.Snapshot{}}
			svc, st, _, cold := hookSvc(t, eng)
			eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "1111aaaa", Tags: []string{"container:plex"}}}
			if tc.mark {
				markTheNamedRepositoryOffSite(t, st)
			}
			settings, err := st.GetSettings()
			if err != nil {
				t.Fatal(err)
			}

			svc.replicateOffsite(context.Background(), "containers", settings, restic.Mode{}, cold)

			copied := false
			for _, c := range eng.copied {
				if sameRepoLocation(strings.SplitN(c, "->", 2)[0], cold) {
					copied = true
				}
			}
			if copied != tc.wantCopied {
				t.Fatalf("copied = %v, want the share copied: %v", eng.copied, tc.wantCopied)
			}
		})
	}
}

// TestTheDomainPassLeavesAnOffSiteNamedRepositoryOut covers "Replicate now" and
// the scheduled pass. The marked repository is neither a source nor a skip: a
// skip becomes the pass's error, and a red run row every night over a setting
// the operator chose on purpose is the same trap the remote case was kept out of.
func TestTheDomainPassLeavesAnOffSiteNamedRepositoryOut(t *testing.T) {
	eng := &hookFakeEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, st, own, cold := hookSvc(t, eng)
	markTheNamedRepositoryOffSite(t, st)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	sources, skipped := svc.offsiteReplicationSources(settings, "containers")

	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want none: leaving a marked repository out is not a failure", skipped)
	}
	if len(sources) != 1 || !sources[0].Own || !sameRepoLocation(sources[0].Loc, filepath.ToSlash(own)) {
		t.Fatalf("sources = %+v, want only the domain's own repository (not %s)", sources, cold)
	}
}

// TestTheRepositoriesCardSetsTheOffSiteMark drives the PATCH the card sends, and
// then one that does not mention the field: a form that does not know about the
// switch must not clear it. The second PATCH starts from the stored row, so its
// answer also proves the mark reached the database.
func TestTheRepositoriesCardSetsTheOffSiteMark(t *testing.T) {
	eng := &hookFakeEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, st, _, _ := hookSvc(t, eng)
	h := &Handler{cfg: svc.cfg, store: st, svc: svc}
	rows, err := st.ListNamedRepos()
	if err != nil || len(rows) != 1 {
		t.Fatalf("fixture: %v (%v)", rows, err)
	}
	id := rows[0].ID

	patch := func(body string) map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		r := jsonReq(http.MethodPatch, "/api/repos/"+id, strings.NewReader(body))
		r.SetPathValue("id", id)
		h.handleUpdateNamedRepo(w, r)
		resp := decodeResp(t, w)
		if resp["ok"] != true {
			t.Fatalf("PATCH %s: %v", body, resp)
		}
		view, _ := resp["repo"].(map[string]any)
		return view
	}

	if view := patch(`{"alreadyOffsite": true}`); view["alreadyOffsite"] != true {
		t.Fatalf("the response does not carry the mark: %v", view)
	}
	if view := patch(`{"name": "NAS cold"}`); view["alreadyOffsite"] != true {
		t.Fatalf("a PATCH that did not mention the mark cleared it: %v", view)
	}
}

// TestTheSettingsFileCarriesTheOffSiteMark runs a named repository through the
// file and back. A restored instance that silently dropped the mark would start
// copying the NAS to the cloud again on its first backup.
func TestTheSettingsFileCarriesTheOffSiteMark(t *testing.T) {
	in := store.OffsiteTarget{Role: store.RoleRepo, Name: "NAS", Repo: "remotes/nas", AlreadyOffsite: true}
	b, err := json.Marshal(namedReposToFileViews([]store.OffsiteTarget{in}))
	if err != nil {
		t.Fatal(err)
	}
	var back []namedRepoFileView
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || !back[0].toStoreTarget().AlreadyOffsite || back[0].Name != "NAS" {
		t.Fatalf("the mark is lost between the file and the imported row: %s", b)
	}
}
