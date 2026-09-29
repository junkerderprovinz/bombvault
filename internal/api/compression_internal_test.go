package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestPrimaryModeForCarriesTheDomainsCompression(t *testing.T) {
	s := primaryCredsService(t)
	if _, err := s.store.MutateSettings(func(m *store.Settings) error {
		m.SetCompression("containers", "max")
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	settings := settingsOf(t, s)
	if got := s.primaryModeFor(settings, "containers", settings.ContainersPath).Compression; got != restic.CompressionMax {
		t.Fatalf("containers compress with %q, want max", got)
	}
	if got := s.primaryModeFor(settings, "vms", "/mnt/user/backups/vms").Compression; got != "" {
		t.Fatalf("a domain nobody configured must keep restic's default, got %q", got)
	}
}

func TestANamedRepositoryCompressesByItsOwnRow(t *testing.T) {
	s := primaryCredsService(t)
	if _, err := s.store.MutateSettings(func(m *store.Settings) error {
		m.SetCompression("containers", "max")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "Media", Repo: "b2:media", Enabled: true, Compression: "off",
	}); err != nil {
		t.Fatal(err)
	}

	settings := settingsOf(t, s)
	if got := s.primaryModeFor(settings, "containers", "b2:media").Compression; got != restic.CompressionOff {
		t.Fatalf("a named repository answers for itself, got %q", got)
	}
}

func TestOffsiteModeForTargetCarriesItsCompression(t *testing.T) {
	s := primaryCredsService(t)
	settings := settingsOf(t, s)
	mode := s.offsiteModeForTarget(settings, store.OffsiteTarget{ID: "t", Repo: "s3:far", Compression: "max"})
	if mode.Compression != restic.CompressionMax {
		t.Fatalf("the copy into this destination compresses with %q, want max", mode.Compression)
	}
}

func TestThePrimaryOffsiteRowTakesItsCompressionFromSettings(t *testing.T) {
	s := primaryCredsService(t)
	settings, err := s.store.MutateSettings(func(m *store.Settings) error {
		m.ContainersOffsite = "s3:https://far.example/containers"
		m.SetCompression("offsite:containers", "off")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := settingsOffsiteTarget("containers", settings, settings.ContainersOffsite).Compression; got != "off" {
		t.Fatalf("an install without a target row replicates with %q, want off", got)
	}
	if err := s.syncPrimaryOffsiteTarget("containers", settings); err != nil {
		t.Fatal(err)
	}
	targets, err := s.store.OffsiteTargetsForDomain("containers")
	if err != nil || len(targets) != 1 {
		t.Fatalf("targets = %v, err %v", targets, err)
	}
	if targets[0].Compression != "off" {
		t.Fatalf("the primary destination row stores %q, want off", targets[0].Compression)
	}
}

func TestSettingsCompressionRefusesUnknownKeysAndModes(t *testing.T) {
	if msg := rejectInvalidCompression(map[string]string{"vms": "max", "offsite:zfs": "off", "flash": ""}); msg != "" {
		t.Fatalf("a valid map was refused: %s", msg)
	}
	if msg := rejectInvalidCompression(map[string]string{"photos": "max"}); msg == "" {
		t.Fatal("a key that names no repository must be refused")
	}
	if msg := rejectInvalidCompression(map[string]string{"vms": "zstd"}); msg == "" {
		t.Fatal("a mode restic does not know must be refused")
	}
}

func TestSettingsCompressionStoresOnlyWhatDiffersFromTheDefault(t *testing.T) {
	var s store.Settings
	applyCompression(&s, map[string]string{"vms": "MAX", "flash": "auto", "offsite:files": "off"})
	if s.CompressionFor("vms") != "max" || s.CompressionFor("offsite:files") != "off" || s.CompressionFor("flash") != "" {
		t.Fatalf("stored %q", s.Compression)
	}
	applyCompression(&s, nil)
	if s.CompressionFor("vms") != "max" {
		t.Fatal("a client that sends no compression at all must not reset it")
	}
	view := compressionView(s)
	if len(view) != 12 || view["vms"] != "max" || view["containers"] != "auto" || view["offsite:files"] != "off" {
		t.Fatalf("view = %v, want every repository with its mode", view)
	}
}

func TestNamedRepositoryCompressionIsValidated(t *testing.T) {
	h, _ := newCRUDHandler(t)
	create := func(mode string) map[string]any {
		body, _ := json.Marshal(map[string]any{"name": "Media " + mode, "repo": "b2:media-" + mode, "compression": mode})
		rec := httptest.NewRecorder()
		h.handleCreateNamedRepo(rec, jsonReq(http.MethodPost, "/api/repos", bytes.NewReader(body)))
		return decodeEnvelope(t, rec)
	}
	if env := create("best"); env["ok"] != false {
		t.Fatalf("an unknown mode was stored: %v", env)
	}
	env := create("MAX")
	if env["ok"] != true {
		t.Fatalf("create not ok: %v", env)
	}
	if got := env["repo"].(map[string]any)["compression"]; got != "max" {
		t.Fatalf("compression = %v, want max", got)
	}
}

func TestOffsiteTargetCompressionIsValidated(t *testing.T) {
	if msg := validateOffsiteTargetInput(offsiteTargetView{Domain: "vms", Repo: "s3:x", Compression: "zstd"}.toStoreTarget()); msg == "" {
		t.Fatal("an unknown mode must be refused")
	}
	if got := offsiteTargetToView(store.OffsiteTarget{}).Compression; got != "auto" {
		t.Fatalf("a row from before the setting reads as %q, want auto", got)
	}
}

func TestRecoveryKitNamesEachRepositorysCompression(t *testing.T) {
	s := primaryCredsService(t)
	kit, err := s.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(kit, "--compression") {
		t.Fatal("with every repository on restic's default the kit has nothing to say about compression")
	}

	if _, err := s.store.MutateSettings(func(m *store.Settings) error {
		m.SetCompression("containers", "max")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "Media", Repo: "b2:media", Enabled: true, Compression: "off",
	}); err != nil {
		t.Fatal(err)
	}
	kit, err = s.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"- containers (local): s3:https://garage.example/bucket (compression: max)",
		"- Media: b2:media (compression: off)",
		"--compression max",
	} {
		if !strings.Contains(kit, want) {
			t.Errorf("the kit does not say %q", want)
		}
	}
}
