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

	// CREATE
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

	// LIST (filtered by domain)
	rec = httptest.NewRecorder()
	h.handleListOffsiteTargets(rec, httptest.NewRequest(http.MethodGet, "/api/offsite/targets?domain=containers", nil))
	env = decodeEnvelope(t, rec)
	list, _ := env["targets"].([]any)
	if len(list) != 1 {
		t.Fatalf("list(containers) = %d, want 1", len(list))
	}

	// UPDATE
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

	// DELETE
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

func TestTheServerDecidesWhereANewTargetGoes(t *testing.T) {
	h, st := newCRUDHandler(t)
	field, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Primary", Repo: "s3:b2", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(offsiteTargetView{Domain: "containers", Name: "Hetzner", Repo: "sftp:u1@hetzner:/c", Enabled: true, SortOrder: 0})
	rec := httptest.NewRecorder()
	h.handleCreateOffsiteTarget(rec, jsonReq(http.MethodPost, "/api/offsite/targets", bytes.NewReader(body)))
	env := decodeEnvelope(t, rec)
	created, _ := env["target"].(map[string]any)
	if env["ok"] != true || created["sortOrder"] != float64(1) {
		t.Fatalf("create = %v, want ok and sortOrder 1", env)
	}
	id, _ := created["id"].(string)

	body, _ = json.Marshal(offsiteTargetView{Domain: "containers", Name: "Hetzner", Repo: "sftp:u1@hetzner:/c", Enabled: true, SortOrder: 0})
	req := jsonReq(http.MethodPut, "/api/offsite/targets/"+id, bytes.NewReader(body))
	req.SetPathValue("id", id)
	rec = httptest.NewRecorder()
	h.handleUpdateOffsiteTarget(rec, req)
	env = decodeEnvelope(t, rec)
	if upd, _ := env["target"].(map[string]any); env["ok"] != true || upd["sortOrder"] != float64(1) {
		t.Fatalf("update = %v, want ok and sortOrder still 1", env)
	}
	if got, _, _ := st.FieldOffsiteTarget("containers"); got.ID != field.ID {
		t.Fatalf("field target = %s, want %s", got.ID, field.ID)
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

// TestUpdateOffsiteTargetRefusesADomainChange: PUT keeps the stored sort_order,
// so moving a row to another domain through the same request would leave it
// sitting on that domain's slot 0 alongside whatever is already there. The UI
// never sends a domain change, but the API must refuse one rather than trust
// the body.
func TestUpdateOffsiteTargetRefusesADomainChange(t *testing.T) {
	h, st := newCRUDHandler(t)
	tg, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Primary", Repo: "s3:c1", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(offsiteTargetView{Domain: "vms", Name: "Primary", Repo: "s3:c1", Enabled: true})
	req := jsonReq(http.MethodPut, "/api/offsite/targets/"+tg.ID, bytes.NewReader(body))
	req.SetPathValue("id", tg.ID)
	rec := httptest.NewRecorder()
	h.handleUpdateOffsiteTarget(rec, req)
	env := decodeEnvelope(t, rec)
	if env["ok"] != false {
		t.Fatalf("update = %v, want a domain change refused", env)
	}

	got, ok, err := st.GetOffsiteTarget(tg.ID)
	if err != nil || !ok || got.Domain != "containers" {
		t.Fatalf("GetOffsiteTarget: %+v (ok=%v err=%v), want it still on containers", got, ok, err)
	}
}

// TestUpdateOffsiteTargetMissing: PUT to an unknown id is a clean not-found.
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
