package api_test

import (
	"net/http"
	"strings"
	"testing"
)

// TestDisplayPrefsEmptyUntilSomethingIsStored: a fresh installation answers
// "nothing stored" rather than defaults, so the first load knows to seed the
// server from what the browser already has.
func TestDisplayPrefsEmptyUntilSomethingIsStored(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	w, m := doJSON(t, h, http.MethodGet, "/api/display-prefs", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if m["ok"] != true {
		t.Fatalf("ok = %v, want true", m["ok"])
	}
	if m["stored"] != false {
		t.Errorf("stored = %v, want false on an installation that never saved", m["stored"])
	}
	prefs, ok := m["prefs"].(map[string]any)
	if !ok {
		t.Fatalf("prefs is not an object: %#v", m["prefs"])
	}
	if len(prefs) != 0 {
		t.Errorf("prefs = %v, want empty", prefs)
	}
}

// TestDisplayPrefsRoundTrip: the server stores the preferences without
// interpreting them; the keys belong to the frontend.
func TestDisplayPrefsRoundTrip(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	body := `{"bv-theme":"light","bv-accent":"#1D99F3","bv-labels-buttons":"textGlyph","bombvault.advanced":"1"}`
	w, _ := doJSON(t, h, http.MethodPut, "/api/display-prefs", body)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d body=%s", w.Code, w.Body.String())
	}

	_, m := doJSON(t, h, http.MethodGet, "/api/display-prefs", "")
	if m["stored"] != true {
		t.Fatalf("stored = %v, want true after a save", m["stored"])
	}
	prefs, ok := m["prefs"].(map[string]any)
	if !ok {
		t.Fatalf("prefs is not an object: %#v", m["prefs"])
	}
	for k, want := range map[string]string{
		"bv-theme":           "light",
		"bv-accent":          "#1D99F3",
		"bv-labels-buttons":  "textGlyph",
		"bombvault.advanced": "1",
	} {
		if got := prefs[k]; got != want {
			t.Errorf("prefs[%q] = %v, want %q", k, got, want)
		}
	}

	// A second save merges. A browser whose site data was just cleared sends
	// almost nothing, and a replace would let it wipe the stored look.
	w, _ = doJSON(t, h, http.MethodPut, "/api/display-prefs", `{"bv-theme":"dark"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("second PUT status = %d", w.Code)
	}
	_, m = doJSON(t, h, http.MethodGet, "/api/display-prefs", "")
	prefs, _ = m["prefs"].(map[string]any)
	if prefs["bv-theme"] != "dark" {
		t.Errorf("bv-theme = %v, want dark: the axis that WAS sent must change", prefs["bv-theme"])
	}
	for k, want := range map[string]string{
		"bv-accent":          "#1D99F3",
		"bv-labels-buttons":  "textGlyph",
		"bombvault.advanced": "1",
	} {
		if got := prefs[k]; got != want {
			t.Errorf("prefs[%q] = %v, want %q: an axis the payload did not mention must survive", k, got, want)
		}
	}
}

// TestDisplayPrefsEmptyPayloadKeepsTheStoredLook: an empty object is a valid
// payload that changes nothing, so a browser with no preferences left cannot
// clear the server's.
func TestDisplayPrefsEmptyPayloadKeepsTheStoredLook(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	if w, _ := doJSON(t, h, http.MethodPut, "/api/display-prefs",
		`{"bv-theme":"light","bv-lang":"de"}`); w.Code != http.StatusOK {
		t.Fatalf("setup PUT status = %d", w.Code)
	}
	if w, _ := doJSON(t, h, http.MethodPut, "/api/display-prefs", `{}`); w.Code != http.StatusOK {
		t.Fatalf("empty PUT status = %d, an empty object is a valid payload", w.Code)
	}

	_, m := doJSON(t, h, http.MethodGet, "/api/display-prefs", "")
	if m["stored"] != true {
		t.Fatalf("stored = %v, want true: an empty payload must not erase the record", m["stored"])
	}
	prefs, _ := m["prefs"].(map[string]any)
	if prefs["bv-theme"] != "light" || prefs["bv-lang"] != "de" {
		t.Errorf("an empty payload wiped the stored look: %v", prefs)
	}
}

// TestDisplayPrefsRejectsNonObjectAndOversize: only an object is accepted,
// since a bare value would break the client that spreads it, and the size cap
// keeps the column from becoming free storage.
func TestDisplayPrefsRejectsNonObjectAndOversize(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	for _, bad := range []string{`"just a string"`, `42`, `[1,2,3]`, `not json at all`} {
		w, _ := doJSON(t, h, http.MethodPut, "/api/display-prefs", bad)
		if w.Code != http.StatusBadRequest {
			t.Errorf("PUT %q: status = %d, want 400", bad, w.Code)
		}
	}

	// 16 KiB is the cap; this is comfortably past it.
	big := `{"bv-theme":"` + strings.Repeat("x", 20000) + `"}`
	w, _ := doJSON(t, h, http.MethodPut, "/api/display-prefs", big)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized PUT: status = %d, want 413", w.Code)
	}

	_, m := doJSON(t, h, http.MethodGet, "/api/display-prefs", "")
	if m["stored"] != false {
		t.Errorf("a rejected save still stored something: %v", m)
	}
}
