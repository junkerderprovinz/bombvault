package api_test

import (
	"net/http"
	"strings"
	"testing"
)

// The look of the interface lives on the server so it survives a browser
// (issue #191). These pin the three things a client depends on.

// An untouched installation must answer "nothing stored", NOT an error and not
// a set of defaults. That distinction is the whole upgrade path: a first load
// seeds the server from whatever the browser already had, and it can only know
// to do that if "no opinion" is distinguishable from "the opinion is default".
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

// What goes in comes back out, verbatim and unread. The server deliberately
// does not know what a "theme" is: the axes belong to the frontend and grow
// with it, so a backend that understood them would need a release per axis.
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

	// A second save MERGES. This test used to assert the opposite, on the
	// assumption that "the client always sends the whole look" — and that
	// assumption is what reopened #191. A browser sends what it HAS, and a
	// browser whose site data was just cleared has nothing, so replacing let it
	// publish its own emptiness and wipe the stored look for good.
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

// The exact shape that cost manilx his settings three times over: a browser
// with nothing left in it must not be able to clear the server. An empty object
// is a valid payload and a no-op, not a reset.
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

// Only an OBJECT is accepted. A bare string or number would round-trip through
// the column happily and then break the client that expects to spread it, and
// an oversized body would turn a settings column into free storage.
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

	// And none of that may have left anything behind.
	_, m := doJSON(t, h, http.MethodGet, "/api/display-prefs", "")
	if m["stored"] != false {
		t.Errorf("a rejected save still stored something: %v", m)
	}
}
