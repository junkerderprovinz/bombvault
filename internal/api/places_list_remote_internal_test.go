package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The Storage tab and the Recovery page list the places and the domain rows on
// every visit. Neither may open a remote repository to do so: a cloud listing
// costs time and requests, and the answers come from the database.
func TestThePlaceListsOpenNoRemoteRepository(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.VMsPath = "rest:http://tower:8000/bv/vms" })
	f.target("containers", "B2", "s3:https://s3.example.com/bv/containers")
	f.namedRepo("Box", "sftp:u1@box:/bv")
	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("MigrateToPlaces: %v", err)
	}
	f.eng.mu.Lock()
	f.eng.lists, f.eng.opened = map[string]int{}, map[string]int{}
	f.eng.mu.Unlock()

	for _, route := range []struct {
		path  string
		serve func(http.ResponseWriter, *http.Request)
	}{
		{"/api/places", f.h.handleListPlaces},
		{"/api/storage/domains", f.h.handleStorageDomains},
	} {
		rec := httptest.NewRecorder()
		route.serve(rec, httptest.NewRequest(http.MethodGet, route.path, nil))
		if env := decodeEnvelope(t, rec); env["ok"] != true {
			t.Fatalf("GET %s = %v", route.path, env)
		}
	}

	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	if len(f.eng.lists) != 0 || len(f.eng.opened) != 0 {
		t.Fatalf("the lists listed %v and opened %v", f.eng.lists, f.eng.opened)
	}
}
