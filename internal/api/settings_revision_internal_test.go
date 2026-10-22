package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func putSettingsAt(t *testing.T, f *placementFixture, v settingsView, revision string) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	r := jsonReq("PUT", "/api/settings", bytes.NewReader(b))
	r.Header.Set(settingsRevisionHeader, revision)
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestASettingsSaveBuiltOnAnOlderStateIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	read := f.do("GET", "/api/settings", nil)
	revision, _ := read["revision"].(string)
	if revision == "" {
		t.Fatalf("GET carries no revision: %v", read)
	}
	old, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	stale := toView(old)

	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.ContainersPath = "backups/moved"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	stale.CatchUpMissed = !old.CatchUpMissed
	res := putSettingsAt(t, f, stale, revision)
	if res["ok"] != false || res["code"] != "settings-stale" {
		t.Fatalf("answer = %v, want the stale save refused", res)
	}
	if now, _ := f.st.GetSettings(); now.ContainersPath != "backups/moved" || now.CatchUpMissed != old.CatchUpMissed {
		t.Fatalf("stored %q, catch-up %v: the stale save went through", now.ContainersPath, now.CatchUpMissed)
	}

	fresh := f.do("GET", "/api/settings", nil)
	current, _ := fresh["revision"].(string)
	now, _ := f.st.GetSettings()
	v := toView(now)
	v.CatchUpMissed = !old.CatchUpMissed
	res = putSettingsAt(t, f, v, current)
	if res["ok"] != true {
		t.Fatalf("a save on the current state was refused: %v", res)
	}
	saved, _ := f.st.GetSettings()
	if saved.ContainersPath != "backups/moved" || saved.CatchUpMissed == old.CatchUpMissed {
		t.Fatalf("stored %+v", saved)
	}
	if res["revision"] != settingsRevision(saved) {
		t.Fatalf("revision = %v, want the one the save left", res["revision"])
	}
}
