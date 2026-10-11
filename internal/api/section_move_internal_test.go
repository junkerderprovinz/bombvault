package api

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const vaultRepo = "rclone:vault:bv"

func (f *placementFixture) movePreview(target, dest store.OffsiteTarget) map[string]any {
	f.t.Helper()
	return f.do("GET", "/api/offsite/targets/"+target.ID+"/move?to="+dest.ID, nil)
}

func moveOf(t *testing.T, res map[string]any) sectionMove {
	t.Helper()
	if res["ok"] != true {
		t.Fatalf("answer = %v", res)
	}
	b, err := json.Marshal(res["move"])
	if err != nil {
		t.Fatal(err)
	}
	var out sectionMove
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// move starts a move and waits for its run to end.
func (f *placementFixture) move(target, dest store.OffsiteTarget) sectionMove {
	f.t.Helper()
	started := moveOf(f.t, f.do("POST", "/api/offsite/targets/"+target.ID+"/move", map[string]any{"to": dest.ID}))
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		f.svc.cancelMu.Lock()
		_, running := f.svc.runCancels[started.Key]
		f.svc.cancelMu.Unlock()
		if !running {
			return started
		}
		time.Sleep(5 * time.Millisecond)
	}
	f.t.Fatalf("the move %s did not end", started.Key)
	return started
}

func (f *placementFixture) stored(id string) store.OffsiteTarget {
	f.t.Helper()
	t, ok, err := f.st.GetOffsiteTarget(id)
	if err != nil || !ok {
		f.t.Fatalf("target %s: found %v, %v", id, ok, err)
	}
	return t
}

func (f *placementFixture) snapsAt(location string) []restic.Snapshot {
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	return slices.Clone(f.eng.snaps[location])
}

func TestTheMovePreviewCountsWhatTheNewPlaceStillLacks(t *testing.T) {
	f := newPlacementFixture(t)
	_, vms := boxWithVMs(f)
	vault := f.destination("Vault", vaultRepo)
	f.hold(vaultRepo+"/vms", copied("v1", "a1", 100, "vm:win"))

	got := moveOf(t, f.movePreview(vms, vault))

	want := sectionMove{
		Domain: "vms", TargetID: vms.ID, From: boxRepo + "/vms", To: vaultRepo + "/vms",
		Backups: 5, Pending: 4, Key: "move:" + vms.ID,
	}
	if got != want {
		t.Fatalf("preview = %+v, want %+v", got, want)
	}
	if len(f.eng.copies) != 0 || len(f.eng.ensured) != 0 {
		t.Fatalf("the preview copied %+v and created %v", f.eng.copies, f.eng.ensured)
	}
}

func TestAMoveCopiesTheHistoryAndThenPointsTheSectionAtTheNewPlace(t *testing.T) {
	f := newPlacementFixture(t)
	box, vms := boxWithVMs(f)
	vault := f.destination("Vault", vaultRepo)
	f.rule("vms", "vm:win", vms.ID)
	if err := f.st.RecordTamperTestForTarget("vms", vms.ID, true, "delete refused"); err != nil {
		t.Fatal(err)
	}
	before := f.snapsAt(boxRepo + "/vms")

	f.move(vms, vault)

	arrived := f.snapsAt(vaultRepo + "/vms")
	if len(arrived) != 5 {
		t.Fatalf("the new place holds %d backups, want the 5 of the section: %+v", len(arrived), arrived)
	}
	identities := map[string][]string{}
	for _, sn := range arrived {
		identities[restic.Identity(sn)] = sn.Tags
	}
	for identity, tags := range map[string][]string{"a1": {"vm:win"}, "a2": {"vm:win"}, "o1": {"vm:old"}, "o2": {"vm:old"}, "o3": {"vm:old"}} {
		if !slices.Equal(identities[identity], tags) {
			t.Fatalf("the backup first written as %s arrived with the tags %v, want %v", identity, identities[identity], tags)
		}
	}
	if after := f.snapsAt(boxRepo + "/vms"); len(after) != len(before) || len(f.eng.deletes) != 0 || len(f.eng.prunes) != 0 || len(f.eng.forgets) != 0 {
		t.Fatalf("the old place changed: %d backups before, %d after, forgot %+v", len(before), len(after), f.eng.deletes)
	}
	if !slices.Equal(f.eng.checks, []string{vaultRepo + "/vms"}) {
		t.Fatalf("checked %v, want the new repository once", f.eng.checks)
	}

	moved := f.stored(vms.ID)
	if moved.Repo != vaultRepo+"/vms" || moved.DestinationID != vault.ID || moved.Name != "Vault" {
		t.Fatalf("the target reads %+v, want it at Vault under its old id", moved)
	}
	if left, _ := f.st.DestinationTargets(box.ID); len(left) != 0 {
		t.Fatalf("Box still has the sections %+v", left)
	}
	rules, err := f.st.CopyRulesForDomain("vms")
	if err != nil || !slices.Equal(rules["vm:win"].Skip, []string{vms.ID}) {
		t.Fatalf("the copy rule of vm:win = %+v, %v; want it to name the target as before", rules["vm:win"], err)
	}
	if _, found, _ := f.st.LatestTamperTestForTarget("vms", vms.ID); found {
		t.Fatal("the delete test of the old place still stands for the new one")
	}
	run, err := f.st.LastRunOfKind("vms", "offsite")
	if err != nil || run == nil || run.Status != "success" {
		t.Fatalf("the run = %+v, %v; want a successful one", run, err)
	}
}

func TestAMoveThatFailsItsCheckLeavesTheSectionWhereItIs(t *testing.T) {
	f := newPlacementFixture(t)
	box, vms := boxWithVMs(f)
	vault := f.destination("Vault", vaultRepo)
	f.eng.checkErr = map[string]error{vaultRepo + "/vms": errors.New("pack 1a2b: not found in the index")}

	f.move(vms, vault)

	if got := f.stored(vms.ID); got.Repo != boxRepo+"/vms" || got.DestinationID != box.ID {
		t.Fatalf("the target reads %+v, want it still at Box", got)
	}
	run, err := f.st.LastRunOfKind("vms", "offsite")
	if err != nil || run == nil || run.Status != "failed" {
		t.Fatalf("the run = %+v, %v; want a failed one", run, err)
	}
}

func TestAMoveStartedAgainCopiesOnlyWhatIsStillMissing(t *testing.T) {
	f := newPlacementFixture(t)
	box, vms := boxWithVMs(f)
	vault := f.destination("Vault", vaultRepo)
	f.eng.copyErr[boxRepo+"/vms"] = errors.New("connection reset by peer")

	f.move(vms, vault)
	if got := f.stored(vms.ID); got.DestinationID != box.ID {
		t.Fatalf("a copy that failed moved the target: %+v", got)
	}

	// What an interrupted copy leaves behind: two of the five have landed.
	f.hold(vaultRepo+"/vms", copied("v1", "a1", 100, "vm:win"), copied("v2", "o1", 50, "vm:old"))
	delete(f.eng.copyErr, boxRepo+"/vms")
	f.eng.copies = nil

	f.move(vms, vault)

	if len(f.eng.copies) == 0 {
		t.Fatal("the second move copied nothing")
	}
	sent := slices.Clone(f.eng.copies[0].IDs)
	slices.Sort(sent)
	if !slices.Equal(sent, []string{"c2", "o2", "o3"}) {
		t.Fatalf("the second move sent %v, want the three the new place lacked", sent)
	}
	if got := f.stored(vms.ID); got.DestinationID != vault.ID {
		t.Fatalf("the target reads %+v, want it at Vault", got)
	}
}

func TestMovingThePrimaryCopyRewritesTheSectionsOffsiteField(t *testing.T) {
	f := newPlacementFixture(t)
	primary := f.fieldTarget("files", "sftp:nas:/backups/files")
	f.hold("sftp:nas:/backups/files", snap("f1", 10, "fileset:docs"))
	vault := f.destination("Vault", vaultRepo)

	f.move(primary, vault)

	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.FilesOffsite != vaultRepo+"/files" {
		t.Fatalf("the off-site field of files reads %q, want the new place", settings.FilesOffsite)
	}
	if got := f.stored(primary.ID); got.Repo != vaultRepo+"/files" || got.SortOrder != 0 || !got.Enabled {
		t.Fatalf("the primary target reads %+v", got)
	}
}

func TestAMoveIsRefusedWhereItCannotBeDone(t *testing.T) {
	f := newPlacementFixture(t)
	box, vms := boxWithVMs(f)
	vault := f.destination("Vault", vaultRepo)

	if res := f.movePreview(vms, box); res["code"] != "move-same-place" {
		t.Fatalf("moving a section to where it is = %v, want move-same-place", res)
	}

	f.tickDestination(vault, "vms")
	if res := f.movePreview(vms, vault); res["code"] != "move-section-there" {
		t.Fatalf("moving to a destination the section already copies to = %v, want move-section-there", res)
	}

	files := f.tickDestination(box, "files")
	f.direct(files)
	if res := f.movePreview(files, vault); res["code"] != "move-direct-repo" {
		t.Fatalf("moving a target entries back up straight to = %v, want move-direct-repo", res)
	}
	if res := f.do("POST", "/api/offsite/targets/"+files.ID+"/move", map[string]any{"to": vault.ID}); res["ok"] != false {
		t.Fatalf("the refused move started: %v", res)
	}
}

func TestTwoPlacesOfOneKindUnderDifferentSignInsCannotBeMovedBetween(t *testing.T) {
	f := newPlacementFixture(t)
	ctx := context.Background()
	old, err := f.svc.SaveNewDestination(ctx, draftRequest{
		Provider: "rest", Name: "Old", Settings: map[string]string{"url": "https://old:8000/bv", "user": "bv", "pass": "one"},
	})
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := f.svc.SaveNewDestination(ctx, draftRequest{
		Provider: "rest", Name: "New", Settings: map[string]string{"url": "https://new:8000/bv", "user": "bv", "pass": "two"},
	})
	if err != nil {
		t.Fatal(err)
	}
	vms := f.tickDestination(old, "vms")

	if res := f.movePreview(vms, fresh); res["code"] != "move-sign-in" {
		t.Fatalf("answer = %v, want move-sign-in", res)
	}

	if env, ok := bothSignIns([]string{"AWS_ACCESS_KEY_ID=a"}, []string{"RESTIC_REST_USERNAME=bv", "AWS_ACCESS_KEY_ID=a"}); !ok || len(env) != 2 {
		t.Fatalf("an S3 place and a rest-server sharing what they share joined to %v, %v", env, ok)
	}
}
