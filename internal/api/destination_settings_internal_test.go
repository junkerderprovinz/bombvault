package api

import (
	"net/http"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (f *placementFixture) editDestination(d store.OffsiteTarget, edit map[string]any) map[string]any {
	f.t.Helper()
	body := map[string]any{"name": d.Name, "storageClass": d.StorageClass, "immutable": d.Immutable}
	for k, v := range edit {
		body[k] = v
	}
	res := f.do(http.MethodPut, "/api/offsite/destinations/"+d.ID, body)
	if res["ok"] != true {
		f.t.Fatalf("destination update = %v", res)
	}
	return res
}

func (f *placementFixture) storedTarget(id string) store.OffsiteTarget {
	f.t.Helper()
	got, ok, err := f.st.GetOffsiteTarget(id)
	if err != nil || !ok {
		f.t.Fatalf("target %s: found=%v err=%v", id, ok, err)
	}
	return got
}

func stringList(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, item.(string))
	}
	return out
}

func TestADestinationsSettingsAreWhatCopiesAndPruningUse(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rclone:box:BombVault")
	tgt := f.tickDestination(box, "flash")

	res := f.editDestination(box, map[string]any{
		"retention":   map[string]any{"keepDaily": 7, "keepWeekly": 4, "keepLast": -3},
		"compression": " MAX ", "limitUpload": 512, "limitDownload": 256, "offPremises": false,
	})
	view := res["destination"].(map[string]any)
	if view["compression"] != "max" || view["limitUpload"] != 512.0 || view["offPremises"] != false || view["enabled"] != true {
		t.Errorf("destination view = %v", view)
	}
	if keep := view["retention"].(map[string]any); keep["keepDaily"] != 7.0 || keep["keepLast"] != 0.0 {
		t.Errorf("retention in the view = %v, want 7 daily and the negative count floored", keep)
	}

	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	copies := f.svc.offsiteReplicationTargets("flash", settings)
	if len(copies) != 1 || copies[0].ID != tgt.ID {
		t.Fatalf("flash copies to %+v, want the derived target", copies)
	}
	if p := targetAgingPolicy(copies[0]); p.KeepDaily != 7 || p.KeepWeekly != 4 || p.KeepLast != 0 {
		t.Errorf("keep-policy the copy ages by = %+v", p)
	}
	if l := targetOffsiteLimits(copies[0]); l.UploadKBps != 512 || l.DownloadKBps != 256 {
		t.Errorf("limits of the copy = %+v", l)
	}
	if mode := f.svc.offsiteModeForTarget(settings, copies[0]); mode.Compression != restic.CompressionMax {
		t.Errorf("compression of the copy = %q", mode.Compression)
	}

	f.editDestination(box, map[string]any{"enabled": false})
	if got := f.svc.offsiteReplicationTargets("flash", settings); len(got) != 0 {
		t.Errorf("flash still copies to %+v after the destination was switched off", got)
	}
}

func TestASaveWithoutTheNewSettingsLeavesThemAlone(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rclone:box:BombVault")
	tgt := f.tickDestination(box, "flash")
	f.editDestination(box, map[string]any{"retention": map[string]any{"keepDaily": 7}, "limitUpload": 64, "enabled": false, "offPremises": false})

	f.editDestination(box, map[string]any{"name": "Renamed"})
	d, _, err := f.st.GetDestination(box.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Renamed" || d.RetentionKeepDaily != 7 || d.LimitUpload != 64 || d.Enabled || d.OffPremises {
		t.Errorf("destination = %+v, want the new name and every setting kept", d)
	}
	if got := f.storedTarget(tgt.ID); got.Name != "Renamed" || got.RetentionKeepDaily != 7 || got.Enabled {
		t.Errorf("target = %+v, want it renamed with the settings it follows", got)
	}
}

func TestLoweringADestinationsKeepPolicyWarnsAboutItsDirectRepositories(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rclone:box:BombVault")
	tgt := f.tickDestination(box, "containers")
	f.editDestination(box, map[string]any{"retention": map[string]any{"keepDaily": 30}})
	direct, err := f.st.CreateCompanionRepo(tgt.ID, "", "rclone:box:BombVault/containers-direct")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.UpsertTarget(store.Target{ContainerName: "plex", Repo: direct.ID, RepoChosen: store.RepoChosen}); err != nil {
		t.Fatal(err)
	}

	res := f.editDestination(box, map[string]any{"retention": map[string]any{"keepDaily": 3}})
	warnings, _ := res["warnings"].([]any)
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want one for the direct repository", res["warnings"])
	}
	if w := warnings[0].(map[string]any); w["code"] != "direct-retention-lowered" || w["targetId"] != tgt.ID || w["items"] != 1.0 {
		t.Errorf("warning = %v", w)
	}
	if res := f.editDestination(box, map[string]any{"retention": map[string]any{"keepDaily": 5}}); len(res["warnings"].([]any)) != 0 {
		t.Errorf("warnings after keeping more = %v, want none", res["warnings"])
	}
}

func TestAnUnknownCompressionIsRefusedOnADestination(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rclone:box:BombVault")
	res := f.do(http.MethodPut, "/api/offsite/destinations/"+box.ID, map[string]any{"name": "Box", "compression": "zstd"})
	if res["ok"] != false {
		t.Fatalf("update = %v, want a refusal", res)
	}
}

func TestATargetKeepsItsOwnValueUntilItFollowsAgain(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rclone:box:BombVault")
	tgt := f.tickDestination(box, "vms")
	f.editDestination(box, map[string]any{"retention": map[string]any{"keepDaily": 7}, "limitUpload": 64})

	body := map[string]any{"domain": "vms", "repo": tgt.Repo, "enabled": true, "retentionKeepDaily": 2, "limitUpload": 64}
	res := f.do(http.MethodPut, "/api/offsite/targets/"+tgt.ID, body)
	if own := stringList(res["target"].(map[string]any)["own"]); !slices.Equal(own, []string{"retention"}) {
		t.Fatalf("own settings after saving another keep-policy = %v, want retention alone", own)
	}
	f.editDestination(box, map[string]any{"retention": map[string]any{"keepDaily": 30}, "limitUpload": 128})
	if got := f.storedTarget(tgt.ID); got.RetentionKeepDaily != 2 || got.LimitUpload != 128 {
		t.Fatalf("target = %+v, want its own 2 daily and the destination's limit", got)
	}

	// The body still carries the old count; follow wins over it.
	body["limitUpload"], body["follow"] = 128, []string{"retention"}
	res = f.do(http.MethodPut, "/api/offsite/targets/"+tgt.ID, body)
	if res["ok"] != true || len(stringList(res["target"].(map[string]any)["own"])) != 0 {
		t.Fatalf("follow = %v, want nothing kept", res)
	}
	if got := f.storedTarget(tgt.ID); got.RetentionKeepDaily != 30 {
		t.Errorf("keep daily = %d, want the destination's 30", got.RetentionKeepDaily)
	}

	body["follow"] = []string{"schedule"}
	if res := f.do(http.MethodPut, "/api/offsite/targets/"+tgt.ID, body); res["ok"] != false {
		t.Errorf("following an unknown setting = %v, want a refusal", res)
	}
}

func TestOnlyADerivedTargetOutsideThePrimarySlotCanFollow(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rclone:box:BombVault")
	loose, err := f.st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "By hand", Repo: "s3:bucket/vms", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"domain": "vms", "repo": loose.Repo, "enabled": true, "follow": []string{"retention"}}
	if res := f.do(http.MethodPut, "/api/offsite/targets/"+loose.ID, body); res["ok"] != false {
		t.Errorf("a target without a destination followed one: %v", res)
	}

	primary, err := f.st.PrimaryFromDestination(box.ID, "flash", "rclone:box:BombVault/flash",
		func(s *store.Settings) { s.FlashOffsite = "rclone:box:BombVault/flash" })
	if err != nil {
		t.Fatal(err)
	}
	body = map[string]any{"domain": "flash", "repo": primary.Repo, "enabled": true, "follow": []string{"retention"}}
	if res := f.do(http.MethodPut, "/api/offsite/targets/"+primary.ID, body); res["ok"] != false {
		t.Errorf("the primary followed its destination: %v", res)
	}
}

func TestALocationReportsItsSettingsAndTheSectionsThatDeviate(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rclone:box:BombVault")
	follows := f.tickDestination(box, "flash")
	f.editDestination(box, map[string]any{"retention": map[string]any{"keepDaily": 7}, "compression": "max", "offPremises": false})
	own := f.tickDestination(box, "vms")
	own.RetentionKeepDaily, own.Enabled = 1, false
	if _, err := f.st.UpsertOffsiteTarget(own); err != nil {
		t.Fatal(err)
	}

	loc := locationByID(t, f.svc, "destination:"+box.ID)
	if loc.Retention == nil || loc.Retention.KeepDaily != 7 || loc.Compression != "max" || loc.OffPremises || !loc.Enabled {
		t.Fatalf("location = %+v, want the destination's own settings", loc)
	}
	if sec := sectionFor(t, loc, "flash", useCopy); len(sec.Own) != 0 || sec.Retention.KeepDaily != 7 || sec.TargetID != follows.ID {
		t.Errorf("flash = %+v, want it on the location's settings", sec)
	}
	if sec := sectionFor(t, loc, "vms", useCopy); !slices.Equal(sec.Own, []string{"retention", "enabled"}) || sec.Retention.KeepDaily != 1 || sec.Enabled {
		t.Errorf("vms = %+v, want its own keep-policy and switch", sec)
	}

	res := f.do(http.MethodGet, "/api/storage/locations", nil)
	if list, _ := res["locations"].([]any); res["ok"] != true || len(list) == 0 {
		t.Fatalf("list through the router = %v", res)
	}
}

func TestAFileFromBeforeDestinationsHadSettingsKeepsThem(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rclone:box:BombVault")
	tgt := f.tickDestination(box, "vms")
	f.editDestination(box, map[string]any{"retention": map[string]any{"keepDaily": 7}, "enabled": false, "offPremises": false})

	res := importEdited(t, f, func(exp map[string]any) {
		d := exp["destinations"].([]any)[0].(map[string]any)
		delete(d, "offPremises")
		d["retentionKeepDaily"], d["enabled"] = 0, true
		target := exp["offsiteTargets"].([]any)[0].(map[string]any)
		delete(target, "own")
		target["retentionKeepDaily"], target["enabled"] = 3, true
	})
	if res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	d, _, err := f.st.GetDestination(box.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.RetentionKeepDaily != 7 || d.Enabled || d.OffPremises {
		t.Errorf("destination = %+v, want the settings it had before the import", d)
	}
	got := f.storedTarget(tgt.ID)
	if got.RetentionKeepDaily != 3 || !got.Enabled || got.Own != store.OwnRetention|store.OwnEnabled {
		t.Errorf("target = %+v, own %d, want the file's values kept as its own", got, got.Own)
	}
}

func TestOwnSettingsSurviveARoundTrip(t *testing.T) {
	src := newPlacementFixture(t)
	box := src.destination("Box", "rclone:box:BombVault")
	tgt := src.tickDestination(box, "vms")
	tgt.RetentionKeepDaily = 5
	if _, err := src.st.UpsertOffsiteTarget(tgt); err != nil {
		t.Fatal(err)
	}
	// The destination catches up, so only the stored mark still says the
	// keep-policy is the target's own.
	src.editDestination(box, map[string]any{"retention": map[string]any{"keepDaily": 5}, "limitUpload": 32, "offPremises": false})
	exp := src.do(http.MethodGet, "/api/settings/export", nil)

	dst := newPlacementFixture(t)
	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	d, ok, err := dst.st.GetDestination(box.ID)
	if err != nil || !ok {
		t.Fatalf("destination after import: ok=%v err=%v", ok, err)
	}
	if d.RetentionKeepDaily != 5 || d.LimitUpload != 32 || d.OffPremises || !d.Enabled {
		t.Errorf("destination = %+v, want the exported settings", d)
	}
	if got := dst.storedTarget(tgt.ID); got.Own != store.OwnRetention || got.LimitUpload != 32 {
		t.Errorf("target = %+v, own %d, want its keep-policy still its own and the limit followed", got, got.Own)
	}
}
