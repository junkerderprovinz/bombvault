package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTimelineRouteListsPlacesAndRows(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	f.hold("b2:bucket:containers", copied("b9b9b9b9", "a1a1a1a1", 1_758_000_000, "container:nginx"))

	m := f.do("GET", "/api/items/containers/nginx/timeline", nil)
	places := m["places"].([]any)
	if m["ok"] != true || len(places) != 2 || places[1].(map[string]any)["state"] != "unchecked" {
		t.Fatalf("timeline = %v", m)
	}
	row := m["rows"].([]any)[0].(map[string]any)
	mark := row["places"].([]any)[0].(map[string]any)
	if row["key"] != "a1a1a1a1" || mark["place"] != "local" || mark["snapshotIds"].([]any)[0] != "a1a1a1a1" {
		t.Fatalf("row = %v", row)
	}

	m = f.do("GET", "/api/items/containers/nginx/timeline?place=offsite:"+b2.ID, nil)
	if m["ok"] != true || m["place"].(map[string]any)["state"] != "read" || len(m["rows"].([]any)) != 1 {
		t.Fatalf("B2 = %v", m)
	}
	m = f.do("GET", "/api/items/containers/nginx/timeline?place=offsite:ffffffffffffffffffffffffffffffff", nil)
	if m["ok"] != false || m["code"] != "unknown-target" {
		t.Fatalf("unknown place = %v, want unknown-target", m)
	}
	if m := f.do("GET", "/api/items/containers/ghost/timeline", nil); m["ok"] != true || len(m["rows"].([]any)) != 0 {
		t.Fatalf("an item without backups = %v, want rows []", m)
	}
	if m := f.do("GET", "/api/items/flash/flash/timeline", nil); m["ok"] != true {
		t.Fatalf("flash = %v", m)
	}
}

func TestTimelineRouteRejectsMalformedPlacesAndNames(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	for _, path := range []string{
		"/api/items/containers/nginx/timeline?place=offsite",
		"/api/items/containers/nginx/timeline?place=offsite:NOPE",
		"/api/items/containers/nginx/timeline?place=elsewhere",
		"/api/items/flash/other/timeline",
		"/api/items/nowhere/nginx/timeline",
	} {
		rec := httptest.NewRecorder()
		f.h.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", path, rec.Code)
		}
	}
}
