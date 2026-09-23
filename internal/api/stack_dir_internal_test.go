package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStackDirRouteSaysWhetherTheProjectFolderIsAtTheSource(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("d1d1d1d1", 1_758_000_000, "stack:immich"))
	f.hold("b2:bucket:containers", copied("b1b1b1b1", "a1a1a1a1", 1_758_000_000, "container:immich-server"))

	if m := f.do("GET", "/api/stacks/immich/dir?source=offsite:"+b2.ID, nil); m["ok"] != true || m["found"] != false || m["time"] != "" {
		t.Fatalf("B2 = %v, want found false", m)
	}
	want := time.Unix(1_758_000_000, 0).UTC().Format(time.RFC3339)
	if m := f.do("GET", "/api/stacks/immich/dir", nil); m["found"] != true || m["time"] != want {
		t.Fatalf("local = %v, want found at %s", m, want)
	}
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/stacks/a..b/dir", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}
