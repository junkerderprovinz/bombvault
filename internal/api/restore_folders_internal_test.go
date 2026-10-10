package api

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func storedSettings(t *testing.T, st *store.Repo) store.Settings {
	t.Helper()
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSettingsAnswerWhereEachSectionRestoresTo(t *testing.T) {
	f := newPlacementFixture(t)
	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.RestoreFolder = "user/restore"
		s.RestoreFolderPerSection = true
		s.SetOwnRestoreFolders(map[string]string{"vms": "user/domains-restored"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	res := f.do("GET", "/api/settings", nil)

	got := res["settings"].(map[string]any)["restoreFolders"]
	want := map[string]any{
		"containers": "user/restore/containers", "vms": "user/domains-restored", "flash": "user/restore/flash",
		"config": "user/restore/config", "files": "user/restore/files", "zfs": "user/restore/zfs",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("restoreFolders = %v, want %v", got, want)
	}
}

func TestASettingsSaveStoresTheRestoreFolderOfASection(t *testing.T) {
	f := newPlacementFixture(t)
	v := toView(storedSettings(t, f.st))
	on := true
	v.RestoreFolderPerSection = &on
	v.RestoreFolderOverrides = map[string]string{"vms": " user/domains-restored ", "flash": ""}

	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("PUT: %v", res["error"])
	}

	got := storedSettings(t, f.st)
	if !got.RestoreFolderPerSection {
		t.Fatal("the switch was not stored")
	}
	if own := got.OwnRestoreFolders(); !reflect.DeepEqual(own, map[string]string{"vms": "user/domains-restored"}) {
		t.Fatalf("own folders = %v, want only the trimmed one for vms", own)
	}
}

func TestASettingsSaveKeepsTheRestoreFoldersItDoesNotMention(t *testing.T) {
	f := newPlacementFixture(t)
	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.RestoreFolderPerSection = true
		s.SetOwnRestoreFolders(map[string]string{"files": "user/files-restored"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(toView(storedSettings(t, f.st)))
	if err != nil {
		t.Fatal(err)
	}
	var older map[string]any
	if err := json.Unmarshal(b, &older); err != nil {
		t.Fatal(err)
	}
	delete(older, "restoreFolderPerSection")
	delete(older, "restoreFolderOverrides")

	if res := f.do("PUT", "/api/settings", older); res["ok"] != true {
		t.Fatalf("PUT: %v", res["error"])
	}

	got := storedSettings(t, f.st)
	if !got.RestoreFolderPerSection || got.OwnRestoreFolders()["files"] != "user/files-restored" {
		t.Fatalf("a client that sends neither field changed them: switch %v, own %v", got.RestoreFolderPerSection, got.OwnRestoreFolders())
	}
}

func TestASettingsSaveRefusesARestoreFolderOutsideTheMount(t *testing.T) {
	f := newPlacementFixture(t)
	for name, own := range map[string]map[string]string{
		"a remote":           {"vms": "s3:bucket/restore"},
		"an absolute path":   {"vms": "/etc"},
		"a way out":          {"vms": "user/../../etc"},
		"an unknown section": {"photos": "user/restore"},
	} {
		v := toView(storedSettings(t, f.st))
		v.RestoreFolderOverrides = own
		if res := f.do("PUT", "/api/settings", v); res["ok"] != false {
			t.Errorf("%s was stored: %v", name, own)
		}
	}
	if own := storedSettings(t, f.st).OwnRestoreFolders(); len(own) != 0 {
		t.Fatalf("a refused save left %v behind", own)
	}
}

func TestSettingsExportImportCarriesTheRestoreFolders(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	own := map[string]string{"vms": "user/domains-restored"}
	if _, err := srcStore.MutateSettings(func(s *store.Settings) error {
		s.RestoreFolderPerSection = true
		s.SetOwnRestoreFolders(own)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}

	got := storedSettings(t, dstStore)
	if !got.RestoreFolderPerSection || !reflect.DeepEqual(got.OwnRestoreFolders(), own) {
		t.Fatalf("imported switch %v and own folders %v, want true and %v", got.RestoreFolderPerSection, got.OwnRestoreFolders(), own)
	}
}

func TestARestoreFromAnotherInstanceLandsInTheFolderOfItsSection(t *testing.T) {
	s := newForeignTestService(t, nil)
	s.cfg.HostMountRoot = "/host/user"
	s.SetPlatform(platform.Generic{})
	if _, err := s.store.MutateSettings(func(st *store.Settings) error {
		st.RestoreFolder = "restore"
		st.RestoreFolderPerSection = true
		st.SetOwnRestoreFolders(map[string]string{"vms": "domains-restored"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		call   func(target string) (string, error)
		target string
		want   string
	}{
		{"a container goes to its section's subfolder", s.foreignContainerDestBase, "", "/host/user/restore/containers"},
		{"a VM goes to the folder its section has for itself", s.foreignVMDestBase, "", "/host/user/domains-restored"},
		{"a folder named in the request wins", s.foreignVMDestBase, "elsewhere", "/host/user/elsewhere"},
	} {
		got, err := tc.call(tc.target)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
