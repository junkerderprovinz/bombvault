package api

import (
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// snapshotFiles leaves n snapshot files in a local repository the way restic
// writes them, plus one it is still writing.
func snapshotFiles(t *testing.T, repo string, n int) {
	t.Helper()
	dir := filepath.Join(filepath.FromSlash(repo), "snapshots")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for i := range n {
		id := fmt.Sprintf("%064x", i+1)
		if err := os.WriteFile(filepath.Join(dir, id), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, strings.Repeat("f", 64)+"-tmp-123"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestARefusedMoveCountsTheSnapshotsTheRepositoryHoldsNow(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nginx := f.container("nginx", "")
	f.backupRun(nginx.ID, 100)
	if err := f.st.AddRepoStat(store.RepoStat{Domain: "containers", Source: "local", At: 100, Snapshots: 1}); err != nil {
		t.Fatal(err)
	}
	snapshotFiles(t, f.domainPath("containers"), 4)
	folders := maps.Clone(unraid.Folders)
	folders["containers"] = "ct"

	res := f.do(http.MethodPatch, "/api/places/"+unraid.ID, map[string]any{"folders": folders})

	if res["code"] != "place-location-established" || res["snapshots"] != float64(4) {
		t.Fatalf("PATCH = %v, want place-location-established with the 4 snapshots there now", res)
	}
}

func TestTheStorageCardCountsTheSnapshotsALocalRepositoryHoldsNow(t *testing.T) {
	f := newPlacementFixture(t)
	for _, at := range []int64{100, 200} {
		if err := f.st.AddRepoStat(store.RepoStat{Domain: "containers", Source: "local", At: at, RawSize: 10, Snapshots: 1}); err != nil {
			t.Fatal(err)
		}
	}
	snapshotFiles(t, f.domainPath("containers"), 4)

	res := f.do(http.MethodGet, "/api/stats?domain=containers", nil)

	latest, _ := res["latest"].(map[string]any)
	stats, _ := res["stats"].([]any)
	if latest["snapshots"] != float64(4) || latest["rawSize"] != float64(10) {
		t.Fatalf("latest = %v, want the sample's size and the 4 snapshots there now", latest)
	}
	if first, _ := stats[0].(map[string]any); first["snapshots"] != float64(1) {
		t.Fatalf("stats = %v, want the older sample as measured", stats)
	}
}
