package api

import (
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// One file carries the yearly rules, the compression of every repository, the
// streaming and idle cards and the Home Assistant link together, and a fresh
// instance comes out of the import with all of them.
func TestSettingsExportImportCarriesRetentionCompressionTrafficAndIntegrationsTogether(t *testing.T) {
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
	seedIntegrations(t, srcStore, appKeyA, false)

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
	mqtt, err := dstStore.GetMQTTSettings()
	if err != nil || mqtt.Host != "127.0.0.1" || mqtt.Prefix != "home/bv" || !mqtt.TLS {
		t.Fatalf("Home Assistant = %+v (err=%v)", mqtt, err)
	}
	if pw := brokerPasswordOf(t, dstStore, appKeyB); pw != brokerPassword {
		t.Fatalf("broker password = %q", pw)
	}
	if on, err := dstStore.MDNSEnabled(); err != nil || on {
		t.Fatalf("network announcement = %v (err=%v), want off", on, err)
	}
}
