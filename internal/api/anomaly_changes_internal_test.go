package api

import (
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// comparePair compares two backups of the container the way a finding's
// window asks for it and returns what the window is shown.
func comparePair(t *testing.T, f *placementFixture, itemID, from, to string) ChangesView {
	t.Helper()
	item, err := f.svc.breakdownItemByID("containers", itemID)
	if err != nil {
		t.Fatal(err)
	}
	pair := changePair{item: item, from: store.Run{SnapshotID: from}, to: store.Run{SnapshotID: to}}
	f.svc.changes.results = map[string]ChangesView{}
	f.svc.compareBackups(pair)
	return f.svc.changes.results[pair.key()]
}

func plexSnap(id string) restic.Snapshot {
	return restic.Snapshot{ID: id, Tags: []string{"container:plex"}, Paths: []string{"/host/user/appdata/plex"}}
}

func TestAComparisonOfBackupsLeftInTheOldOwnFolderSaysSoWithoutAnError(t *testing.T) {
	f := newPlacementFixture(t)
	plex := f.container("plex", "")
	f.hold(f.domainPath("containers"), plexSnap("c3c3c3c3"))

	var view ChangesView
	out := captureLog(t, func() { view = comparePair(t, f, plex.ID, "a1a1a1a1", "b1b1b1b1") })

	if view.State != changesApart || view.Error != "" {
		t.Fatalf("view = %+v, want the pair said to be apart", view)
	}
	if !strings.Contains(out, "are not compared") || strings.Contains(out, "compare the backups of") {
		t.Fatalf("log = %q, want the plain line and no error", out)
	}
	if len(f.eng.diffs) != 0 {
		t.Fatalf("restic compared %v, want nothing run", f.eng.diffs)
	}
}

func TestAComparisonReadsTheBackupsWhereTheItemKeptThemBefore(t *testing.T) {
	f := newPlacementFixture(t)
	old := f.namedRepo("Old", "s3:old/containers")
	plex := f.container("plex", old.ID)
	ref := store.ItemRef{Domain: "containers", Key: "plex"}
	if ok, err := f.st.FollowSection(ref, f.home(ref)); err != nil || !ok {
		t.Fatalf("follow the section: %v, %v", ok, err)
	}
	f.hold(f.domainPath("containers"), plexSnap("c3c3c3c3"))
	f.hold("s3:old/containers", plexSnap("a1a1a1a1"), plexSnap("b1b1b1b1"))

	view := comparePair(t, f, plex.ID, "a1a1a1a1", "b1b1b1b1")

	if view.State != breakdownReady {
		t.Fatalf("view = %+v, want the comparison done in the old repository", view)
	}
	if !slices.Equal(f.eng.diffs, []string{"a1a1a1a1->b1b1b1b1"}) {
		t.Fatalf("restic compared %v", f.eng.diffs)
	}
}
