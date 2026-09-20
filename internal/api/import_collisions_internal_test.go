package api

// ---------------------------------------------------------------------------
// rejectImportCollisions has to model the state the apply LEAVES BEHIND, and
// that state is assembled from two sources: the file's own rows and the stored
// rows the apply keeps. The guard had no test at all, and the shape it missed
// was the second source - a repository that is in use here and absent from the
// file survives, because the delete goes through DeleteNamedRepoIfUnused.
// ---------------------------------------------------------------------------

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// seedNamedRepoInUse creates a named repository at loc and points a container at
// it, so ItemsUsingNamedRepo answers non-zero and the apply would keep the row.
func seedNamedRepoInUse(t *testing.T, st *store.Repo, name, loc string) store.OffsiteTarget {
	t.Helper()
	r, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: name, Repo: loc, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.WritePlacement(store.ItemRef{Domain: "containers", Key: "plex"}, &store.HomeWrite{Repo: r.ID, Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}
	return r
}

// TestAnImportIsCheckedAgainstTheRowsTheApplyKEEPS is the third of the three
// shapes the doc block names, and the one that was not modelled.
//
// The file carries repositories, so the guard validated those alone - but a
// stored row that is IN USE and absent from the file is not deleted, and the
// file's domain path was never checked against it. The result is the exact state
// both UI write paths refuse: two rows answering for one repository, one of them
// carrying an append-only flag and credentials for a domain that never set them.
func TestAnImportIsCheckedAgainstTheRowsTheApplyKEEPS(t *testing.T) {
	h, st := newPortableHandler(t, appKeyA)
	seedNamedRepoInUse(t, st, "Cold", "backups/cold")

	exp := settingsExport{
		Settings:   settingsView{ContainersPath: "backups/cold"},
		NamedRepos: []offsiteTargetView{{Name: "Warm", Repo: "backups/warm", Enabled: true}},
	}
	msg := h.rejectImportCollisions(exp)
	if msg == "" {
		t.Fatal("the import was accepted although the Containers path lands on a repository the apply keeps.\n" +
			"The file does not carry that row, so it is not deleted - it is in use - and after the\n" +
			"apply the instance holds two rows over one repository, which is what both write paths refuse.")
	}
	if !strings.Contains(msg, "Cold") {
		t.Errorf("the refusal must name the repository it collided with, got %q", msg)
	}
}

// TestTheImportGuardDoesNotRewriteTheFileItValidates pins that the validator
// leaves its input alone.
//
// The export travels on to summarizeExport and applyImport. Pinning an in-use
// row's location in place made `t.Repo != wanted` false in replaceNamedRepos, so
// the apply's own "its location was in use here and was NOT moved" notice became
// unreachable - the one line that says part of the file was ignored.
func TestTheImportGuardDoesNotRewriteTheFileItValidates(t *testing.T) {
	h, st := newPortableHandler(t, appKeyA)
	r := seedNamedRepoInUse(t, st, "Cold", "backups/cold")

	exp := settingsExport{
		Settings:   settingsView{ContainersPath: "backups/containers"},
		NamedRepos: []offsiteTargetView{{ID: r.ID, Name: "Cold", Repo: "backups/moved", Enabled: true}},
	}
	if msg := h.rejectImportCollisions(exp); msg != "" {
		t.Fatalf("nothing collides here: %s", msg)
	}
	if got := exp.NamedRepos[0].Repo; got != "backups/moved" {
		t.Errorf("the guard rewrote the export it was handed: repo = %q, want the file's own %q.\n"+
			"handleImportSettings passes this same value on to the apply, where a pinned location\n"+
			"silently suppresses the notice that the move was refused.", got, "backups/moved")
	}
}

// TestTheGuardValidatesTheLocationTheApplyWillActuallyStore closes the gap the
// round-nine audit named and left below its floor.
//
// A location that arrived REDACTED is not written over a working one:
// importedLocation keeps the stored value. The guard has to validate the same
// thing, or it checks "rest:https://[redacted]@host/repo" for collisions while
// the instance keeps its real location - and the real one is the only one that
// could actually collide with a domain path.
func TestTheGuardValidatesTheLocationTheApplyWillActuallyStore(t *testing.T) {
	h, st := newPortableHandler(t, appKeyA)
	// An UNUSED row (no item points at it), so the apply is free to move it.
	r, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "Cold", Repo: "backups/cold", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The file carries the same row with its location REDACTED, which is what a
	// plain export from an instance with a credentialed location looks like. The
	// apply keeps "backups/cold"; so must the guard - and that collides with the
	// Containers path below.
	exp := settingsExport{
		Settings: settingsView{ContainersPath: "backups/cold"},
		NamedRepos: []offsiteTargetView{
			{ID: r.ID, Name: "Cold", Repo: "rest:https://" + redactedLocationMarker + "@host:8000/repo", Enabled: true},
		},
	}
	msg := h.rejectImportCollisions(exp)
	if msg == "" {
		t.Fatal("the import was accepted although the Containers path lands on the location this\n" +
			"repository KEEPS. The guard validated the redacted string from the file, which the\n" +
			"apply never writes, so it checked a location that cannot collide with anything.")
	}
	if !strings.Contains(msg, "repository #1") {
		t.Errorf("the refusal must name the row, got %q", msg)
	}
}

// TestAnImportWithNoRepositoriesIsStillCheckedAgainstTheStoredOnes pins the
// first of the three shapes, so a later simplification cannot drop it.
//
// The repository here is deliberately NOT in use: an empty namedRepos block
// means applyImport never calls replaceNamedRepos at all, so EVERY stored row
// survives, not only the ones a delete would have refused. That is what
// separates this branch from the kept-rows half of the other one.
func TestAnImportWithNoRepositoriesIsStillCheckedAgainstTheStoredOnes(t *testing.T) {
	h, st := newPortableHandler(t, appKeyA)
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "Cold", Repo: "backups/cold", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	exp := settingsExport{Settings: settingsView{VMsPath: "backups/cold"}}
	if msg := h.rejectImportCollisions(exp); msg == "" {
		t.Fatal("a file with no repositories leaves the stored rows in place, so an imported\n" +
			"domain path still has to be checked against them")
	}
}
