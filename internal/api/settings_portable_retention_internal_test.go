package api

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// One file carries the yearly rules, the compression of every repository and
// the streaming and idle cards together, and a fresh instance comes out of the
// import with all of them.
func TestSettingsExportImportCarriesRetentionCompressionAndTrafficTogether(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	s, err := srcStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.RetentionKeepYearly = 3
	s.OffsiteRetentionKeepYearly = 5
	s.SetCompression("containers", "max")
	s.SetCompression("offsite:containers", "off")
	if err := srcStore.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	archive, _, err := srcStore.GetOffsiteTarget("tgt-2")
	if err != nil {
		t.Fatal(err)
	}
	archive.RetentionKeepYearly = 2
	archive.Compression = "max"
	if _, err := srcStore.UpsertOffsiteTarget(archive); err != nil {
		t.Fatal(err)
	}

	traffic := store.TrafficSettings{StreamThrottle: true, MediaServers: []string{"plex"}, StreamMbit: 6, StreamLimitKiB: 300, StreamHoldMin: 9,
		IdleCPUPct: 30, IdleNetMbit: 2, IdleQuietMin: 7}
	if err := srcStore.SetTrafficSettings(traffic); err != nil {
		t.Fatal(err)
	}

	body, _ := doExport(t, src, "?includeCredentials=true")
	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}

	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.RetentionKeepYearly != 3 || got.OffsiteRetentionKeepYearly != 5 {
		t.Fatalf("yearly rules = %d local, %d off-site", got.RetentionKeepYearly, got.OffsiteRetentionKeepYearly)
	}
	if got.CompressionFor("containers") != "max" || got.CompressionFor("offsite:containers") != "off" {
		t.Fatalf("compression = %q", got.Compression)
	}
	target, found, err := dstStore.GetOffsiteTarget("tgt-2")
	if err != nil || !found || target.RetentionKeepYearly != 2 || target.Compression != "max" {
		t.Fatalf("the archive target came back as %+v (found=%v, err=%v)", target, found, err)
	}
	if gotTraffic, err := dstStore.TrafficSettings(); err != nil || !reflect.DeepEqual(gotTraffic, traffic) {
		t.Fatalf("traffic = %+v (err=%v), want %+v", gotTraffic, err, traffic)
	}
}

// A file written before the yearly rule and per-destination compression has no
// keys for them, which says nothing about this instance's own values.
func TestImportOfFileWithoutYearlyRulesKeepsTheStoredOnes(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "")

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	settings := raw["settings"].(map[string]any)
	delete(settings, "retentionKeepYearly")
	delete(settings, "offsiteRetentionKeepYearly")
	for _, tv := range raw["offsiteTargets"].([]any) {
		delete(tv.(map[string]any), "retentionKeepYearly")
		delete(tv.(map[string]any), "compression")
	}
	older, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	s, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.RetentionKeepYearly = 4
	s.OffsiteRetentionKeepYearly = 6
	if err := dstStore.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := dstStore.UpsertOffsiteTarget(store.OffsiteTarget{
		ID: "tgt-2", Domain: "containers", Name: "Archive", Repo: "s3:offsite-archive", Enabled: true, CreatedAt: 2000, SortOrder: 1,
		RetentionKeepYearly: 3, Compression: "max",
	}); err != nil {
		t.Fatal(err)
	}

	if env := doImport(t, dst, older, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.RetentionKeepYearly != 4 || got.OffsiteRetentionKeepYearly != 6 {
		t.Fatalf("yearly rules = %d local, %d off-site, want 4 and 6", got.RetentionKeepYearly, got.OffsiteRetentionKeepYearly)
	}
	if got.RetentionKeepDaily != 7 {
		t.Fatalf("the rest of the retention must still apply: keep daily %d", got.RetentionKeepDaily)
	}
	archive, found, err := dstStore.GetOffsiteTarget("tgt-2")
	if err != nil || !found || archive.RetentionKeepYearly != 3 || archive.Compression != "max" {
		t.Fatalf("the archive came back as %+v (found=%v, err=%v)", archive, found, err)
	}
}
