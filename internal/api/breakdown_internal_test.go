package api

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// breakdownWhenDone asks for a breakdown until the worker has finished it.
func breakdownWhenDone(f *placementFixture, query url.Values) map[string]any {
	f.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		m := f.do(http.MethodGet, "/api/breakdown?"+query.Encode(), nil)
		b, _ := m["breakdown"].(map[string]any)
		if m["ok"] != true || b["state"] != breakdownRunning {
			return m
		}
		if time.Now().After(deadline) {
			f.t.Fatal("the breakdown never finished")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func file(path string, size int64) restic.FileEntry {
	return restic.FileEntry{Path: path, Type: "file", Size: size}
}

func TestZFSBreakdownIsOneTreeOverTheDatasetsOfTheNewestRun(t *testing.T) {
	f := newPlacementFixture(t)
	tank, home := f.zfsItem("tank/apps")
	f.hold(home,
		zfsSnap("a1a1a1a1", "tank/apps", firstRun),
		zfsSnap("a2a2a2a2", "tank/apps/db", firstRun),
		zfsSnap("b1b1b1b1", "tank/apps", secondRun),
		zfsSnap("b2b2b2b2", "tank/apps/db", secondRun),
		zfsSnap("b3b3b3b3", "tank/apps/logs", secondRun),
	)
	f.eng.files = map[string][]restic.FileEntry{
		"b1b1b1b1": {file("/compose.yml", 100), {Path: "/data", Type: "dir"}, file("/data/x", 50)},
		"b2b2b2b2": {file("/base", 1000)},
		"b3b3b3b3": {file("/app.log", 7)},
	}
	f.eng.diffChanges = map[string][]restic.DiffChange{"a1a1a1a1->b1b1b1b1": {{Path: "/compose.yml", Modifier: "M"}}}

	query := url.Values{"domain": {zfsDomain}, "item": {tank.ID}}
	m := breakdownWhenDone(f, query)
	top, _ := m["breakdown"].(map[string]any)
	if top["state"] != breakdownReady || top["size"] != float64(1157) || top["snapshot"] != secondRun || top["root"] != "tank/apps" {
		t.Fatalf("top = %v", m)
	}
	// The changed file of the root and the whole of the dataset the run before
	// did not hold.
	if top["added"] != float64(107) || top["first"] == true {
		t.Errorf("added = %v, first = %v, want 107 and a later backup", top["added"], top["first"])
	}
	sizes := map[string]float64{}
	for _, c := range top["children"].([]any) {
		child := c.(map[string]any)
		sizes[child["name"].(string)] = child["size"].(float64)
	}
	if sizes["db"] != 1000 || sizes["logs"] != 7 || sizes["data"] != 50 || sizes["compose.yml"] != 100 {
		t.Errorf("children = %v, want each dataset below the root next to its folders", sizes)
	}
	query.Set("path", "db")
	if db, _ := breakdownWhenDone(f, query)["breakdown"].(map[string]any); db["size"] != float64(1000) || db["added"] != float64(0) {
		t.Errorf("db = %v", db)
	}
}

func TestFlashAndConfigHaveABreakdown(t *testing.T) {
	for _, domain := range []string{"flash", "config"} {
		t.Run(domain, func(t *testing.T) {
			f := newPlacementFixture(t)
			older, newer := snap("a1a1a1a1", 100), snap("b1b1b1b1", 200)
			older.Paths, newer.Paths = []string{"/host/boot"}, []string{"/host/boot"}
			f.hold(f.singletonRepo(domain), older, newer)
			f.eng.files = map[string][]restic.FileEntry{"b1b1b1b1": {
				{Path: "/host", Type: "dir"}, {Path: "/host/boot", Type: "dir"},
				file("/host/boot/bzimage", 500), {Path: "/host/boot/config", Type: "dir"}, file("/host/boot/config/go", 10),
			}}
			f.eng.diffChanges = map[string][]restic.DiffChange{"a1a1a1a1->b1b1b1b1": {{Path: "/host/boot/bzimage", Modifier: "M"}}}

			m := breakdownWhenDone(f, url.Values{"domain": {domain}})
			top, _ := m["breakdown"].(map[string]any)
			if top["state"] != breakdownReady || top["size"] != float64(510) || top["added"] != float64(500) {
				t.Fatalf("top = %v", m)
			}
		})
	}
}
