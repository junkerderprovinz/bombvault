package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func newCRUDHandler(t *testing.T) (*Handler, *store.Repo) {
	t.Helper()
	s, st := newSyncTestService(t)
	return &Handler{store: st, svc: s}, st
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode envelope: %v (body=%s)", err, rec.Body.String())
	}
	return m
}

func TestOffsiteTargetCRUDHandlers(t *testing.T) {
	h, _ := newCRUDHandler(t)

	body, _ := json.Marshal(offsiteTargetView{Domain: "containers", Name: "Second", Repo: "s3:c2", StorageClass: "standard_ia", Enabled: true, SortOrder: 5})
	rec := httptest.NewRecorder()
	h.handleCreateOffsiteTarget(rec, jsonReq(http.MethodPost, "/api/offsite/targets", bytes.NewReader(body)))
	env := decodeEnvelope(t, rec)
	if env["ok"] != true {
		t.Fatalf("create not ok: %v", env)
	}
	created := env["target"].(map[string]any)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("create did not return an id")
	}
	if created["storageClass"] != "STANDARD_IA" {
		t.Fatalf("storage class not normalized/uppercased: %v", created["storageClass"])
	}

	rec = httptest.NewRecorder()
	h.handleListOffsiteTargets(rec, httptest.NewRequest(http.MethodGet, "/api/offsite/targets?domain=containers", nil))
	env = decodeEnvelope(t, rec)
	list, _ := env["targets"].([]any)
	if len(list) != 1 {
		t.Fatalf("list(containers) = %d, want 1", len(list))
	}

	body, _ = json.Marshal(offsiteTargetView{Domain: "containers", Name: "Second", Repo: "s3:c2-moved", Enabled: false, SortOrder: 5})
	req := jsonReq(http.MethodPut, "/api/offsite/targets/"+id, bytes.NewReader(body))
	req.SetPathValue("id", id)
	rec = httptest.NewRecorder()
	h.handleUpdateOffsiteTarget(rec, req)
	env = decodeEnvelope(t, rec)
	if env["ok"] != true {
		t.Fatalf("update not ok: %v", env)
	}
	upd := env["target"].(map[string]any)
	if upd["repo"] != "s3:c2-moved" || upd["enabled"] != false {
		t.Fatalf("update did not apply: %v", upd)
	}
	if upd["id"] != id {
		t.Fatalf("update changed id: %v != %v", upd["id"], id)
	}

	req = jsonReq(http.MethodDelete, "/api/offsite/targets/"+id, nil)
	req.SetPathValue("id", id)
	rec = httptest.NewRecorder()
	h.handleDeleteOffsiteTarget(rec, req)
	if env := decodeEnvelope(t, rec); env["ok"] != true {
		t.Fatalf("delete not ok: %v", env)
	}
	rec = httptest.NewRecorder()
	h.handleListOffsiteTargets(rec, httptest.NewRequest(http.MethodGet, "/api/offsite/targets", nil))
	env = decodeEnvelope(t, rec)
	if list, _ := env["targets"].([]any); len(list) != 0 {
		t.Fatalf("list after delete = %d, want 0", len(list))
	}
}

func TestOffsiteTargetCreateValidation(t *testing.T) {
	h, _ := newCRUDHandler(t)

	cases := []struct {
		name string
		v    offsiteTargetView
	}{
		{"bad domain", offsiteTargetView{Domain: "nope", Repo: "s3:x"}},
		{"empty repo", offsiteTargetView{Domain: "containers", Repo: ""}},
		{"bad storage class", offsiteTargetView{Domain: "containers", Repo: "s3:x", StorageClass: "DEEP_ARCHIVE"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.v)
			rec := httptest.NewRecorder()
			h.handleCreateOffsiteTarget(rec, jsonReq(http.MethodPost, "/api/offsite/targets", bytes.NewReader(body)))
			if env := decodeEnvelope(t, rec); env["ok"] == true {
				t.Fatalf("%s: expected rejection, got %v", tc.name, env)
			}
		})
	}
}

// A target created without a sort order goes after the domain's existing
// targets, so it cannot take the primary's place.
func TestCreateOffsiteTargetWithoutSortOrderGoesLast(t *testing.T) {
	h, st := newCRUDHandler(t)
	for _, existing := range []store.OffsiteTarget{
		{Domain: "containers", Name: "Primary", Repo: "s3:c", Enabled: true, SortOrder: 0},
		{Domain: "containers", Name: "Second", Repo: "s3:c2", Enabled: true, SortOrder: 3},
	} {
		if _, err := st.UpsertOffsiteTarget(existing); err != nil {
			t.Fatal(err)
		}
	}

	body := []byte(`{"domain":"containers","name":"Third","repo":"s3:c3","enabled":true}`)
	rec := httptest.NewRecorder()
	h.handleCreateOffsiteTarget(rec, jsonReq(http.MethodPost, "/api/offsite/targets", bytes.NewReader(body)))
	env := decodeEnvelope(t, rec)
	if env["ok"] != true {
		t.Fatalf("create not ok: %v", env)
	}
	if got := env["target"].(map[string]any)["sortOrder"]; got != float64(4) {
		t.Fatalf("sortOrder = %v, want 4", got)
	}
}

// Sort order 0 is the primary, which the off-site setting in Settings manages,
// so the create route refuses it and anything below it.
func TestCreateOffsiteTargetRefusesPrimarySortOrder(t *testing.T) {
	h, st := newCRUDHandler(t)
	for _, order := range []int{0, -1} {
		body, _ := json.Marshal(offsiteTargetView{Domain: "containers", Name: "Second", Repo: "s3:c2", Enabled: true, SortOrder: order})
		rec := httptest.NewRecorder()
		h.handleCreateOffsiteTarget(rec, jsonReq(http.MethodPost, "/api/offsite/targets", bytes.NewReader(body)))
		if env := decodeEnvelope(t, rec); env["ok"] == true {
			t.Fatalf("sortOrder %d: want a refusal, got %v", order, env)
		}
	}
	all, err := st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("a refused create stored targets: %+v", all)
	}
}

func TestUpdateOffsiteTargetMissing(t *testing.T) {
	h, _ := newCRUDHandler(t)
	body, _ := json.Marshal(offsiteTargetView{Domain: "containers", Repo: "s3:x"})
	req := jsonReq(http.MethodPut, "/api/offsite/targets/deadbeef", bytes.NewReader(body))
	req.SetPathValue("id", "deadbeef")
	rec := httptest.NewRecorder()
	h.handleUpdateOffsiteTarget(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
