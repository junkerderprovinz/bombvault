package api

import (
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (f *placementFixture) copyWaitsOn() []string {
	f.t.Helper()
	runs, err := f.st.ListRuns(50)
	if err != nil {
		f.t.Fatal(err)
	}
	var out []string
	for _, v := range f.h.runViews(runs) {
		if v.CopyWaits {
			out = append(out, v.ID)
		}
	}
	return out
}

func (f *placementFixture) newestRunOf(itemID string) string {
	f.t.Helper()
	runs, err := f.st.ListRuns(50)
	if err != nil {
		f.t.Fatal(err)
	}
	i := slices.IndexFunc(runs, func(r store.Run) bool { return r.TargetID == itemID })
	if i < 0 {
		f.t.Fatalf("%s has no run", itemID)
	}
	return runs[i].ID
}

func (f *placementFixture) awaitsDecision(d store.OffsiteTarget, domain string) bool {
	f.t.Helper()
	loc, err := f.svc.StorageLocation("destination:" + d.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	return slices.ContainsFunc(loc.Sections, func(s locationSection) bool { return s.Domain == domain && s.AwaitsDecision })
}

func TestALocationSaysWhenItsCopiesWaitForADecision(t *testing.T) {
	f := newPlacementFixture(t)
	d := f.copyingLocation()
	if f.awaitsDecision(d, "files") {
		t.Fatal("a section that copies freely waits for a decision")
	}

	f.paused("files")
	if !f.awaitsDecision(d, "files") {
		t.Fatal("the location does not say that its Folders copy waits for a decision")
	}

	if res := f.do("POST", "/api/placement/default/files/confirm", map[string]any{"exclude": []string{}}); res["ok"] != true {
		t.Fatalf("confirm = %v", res)
	}
	if f.awaitsDecision(d, "files") {
		t.Fatal("the copy still waits after the backups were taken as this server's own")
	}
}

func TestTheNewestBackupOfAnItemSaysItsCopyWaits(t *testing.T) {
	f := newPlacementFixture(t)
	f.copyingLocation()
	photos := f.fileSet("photos", "")
	kept := f.fileSet("kept", "")
	f.rule("files", "fileset:kept", store.SkipAll)
	now := time.Now().Unix()
	f.backupRun(photos.ID, now-7200)
	f.backupRun(photos.ID, now-3600)
	f.backupRun(kept.ID, now-3600)
	if got := f.copyWaitsOn(); len(got) != 0 {
		t.Fatalf("runs %v say their copy waits before anything does", got)
	}

	f.paused("files")
	if got, want := f.copyWaitsOn(), []string{f.newestRunOf(photos.ID)}; !slices.Equal(got, want) {
		t.Fatalf("runs saying their copy waits = %v, want only the newest backup of photos %v", got, want)
	}

	if res := f.do("POST", "/api/placement/default/files/confirm", map[string]any{"exclude": []string{}}); res["ok"] != true {
		t.Fatalf("confirm = %v", res)
	}
	if got := f.copyWaitsOn(); len(got) != 0 {
		t.Fatalf("runs %v still say their copy waits after the decision", got)
	}
}
