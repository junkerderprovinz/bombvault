package api

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (f *placementFixture) destination(name, repo string) store.OffsiteTarget {
	f.t.Helper()
	d, err := f.st.SaveDestination(store.OffsiteTarget{Name: name, Repo: repo, Provider: "box"})
	if err != nil {
		f.t.Fatal(err)
	}
	return d
}

func (f *placementFixture) tickDestination(d store.OffsiteTarget, domain string) store.OffsiteTarget {
	f.t.Helper()
	t, _, err := f.st.EnsureDestinationTarget(d.ID, domain, d.Repo+"/"+domain)
	if err != nil {
		f.t.Fatal(err)
	}
	return t
}

func TestADestinationAndItsTargetSurviveARoundTrip(t *testing.T) {
	src := newPlacementFixture(t)
	box := src.destination("Box", "rclone:box:BombVault")
	tgt := src.tickDestination(box, "containers")
	exp := src.do(http.MethodGet, "/api/settings/export", nil)

	dst := newPlacementFixture(t)
	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	got, ok, err := dst.st.GetDestination(box.ID)
	if err != nil || !ok {
		t.Fatalf("destination after import: ok=%v err=%v", ok, err)
	}
	if got.Name != "Box" || got.Repo != box.Repo || got.Provider != "box" {
		t.Errorf("destination = %+v, want the exported one", got)
	}
	targets, err := dst.st.DestinationTargets(box.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].ID != tgt.ID || targets[0].Provider != "box" {
		t.Fatalf("targets of the destination = %+v, want the containers target", targets)
	}
}

func TestAFileWithoutDestinationsLeavesThemAlone(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rclone:box:BombVault")
	res := importEdited(t, f, func(exp map[string]any) { delete(exp, "destinations") })
	if res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	if _, ok, _ := f.st.GetDestination(box.ID); !ok {
		t.Error("the destination went although the file carried no destinations block")
	}
}

func TestAnEmptyDestinationsBlockKeepsOnlyTheOnesInUse(t *testing.T) {
	f := newPlacementFixture(t)
	used := f.destination("Box", "rclone:box:BombVault")
	f.tickDestination(used, "vms")
	spare := f.destination("Drive", "rclone:drive:BombVault")
	res := importEdited(t, f, func(exp map[string]any) { exp["destinations"] = []any{} })
	if res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	if _, ok, _ := f.st.GetDestination(used.ID); !ok {
		t.Error("a destination a target is derived from was deleted")
	}
	if _, ok, _ := f.st.GetDestination(spare.ID); ok {
		t.Error("an unused destination the file does not carry was kept")
	}
}

func TestAnImportDoesNotMoveADestinationThatHoldsRepositories(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rclone:box:BombVault")
	f.tickDestination(box, "vms")
	res := importEdited(t, f, func(exp map[string]any) {
		exp["destinations"].([]any)[0].(map[string]any)["repo"] = "rclone:box:Elsewhere"
	})
	if res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	got, _, _ := f.st.GetDestination(box.ID)
	if got.Repo != box.Repo {
		t.Errorf("location = %q, want it kept at %q", got.Repo, box.Repo)
	}
}

func TestAPlainExportRedactsADestinationsPassword(t *testing.T) {
	f := newPlacementFixture(t)
	f.destination("Rest", "rest:https://bv:secret@backup.example:8000/bv")
	exp := f.do(http.MethodGet, "/api/settings/export", nil)
	repo := exp["destinations"].([]any)[0].(map[string]any)["repo"].(string)
	if repo != "rest:https://"+redactedLocationMarker+"backup.example:8000/bv" {
		t.Errorf("exported location = %q, want the password redacted", repo)
	}
}

func TestATargetMadeFromADestinationKeepsItsLocationAndName(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rclone:box:BombVault")
	tgt := f.tickDestination(box, "vms")
	body := map[string]any{"domain": "vms", "name": "Renamed", "repo": tgt.Repo, "enabled": true, "retentionKeepLast": 3}
	if res := f.do(http.MethodPut, "/api/offsite/targets/"+tgt.ID, body); res["ok"] != true {
		t.Fatalf("update = %v", res)
	}
	got, _, _ := f.st.GetOffsiteTarget(tgt.ID)
	if got.Name != "Box" || got.RetentionKeepLast != 3 {
		t.Errorf("target = %+v, want the destination's name and the new retention", got)
	}
	body["repo"] = "rclone:box:Elsewhere/vms"
	if res := f.do(http.MethodPut, "/api/offsite/targets/"+tgt.ID, body); res["ok"] == true {
		t.Fatal("the target moved away from its destination")
	}
}
