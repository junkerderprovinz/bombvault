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

// putOffsiteTarget sends body to the update route for id and returns the
// envelope.
func putOffsiteTarget(t *testing.T, h *Handler, id, body string) map[string]any {
	t.Helper()
	req := jsonReq(http.MethodPut, "/api/offsite/targets/"+id, bytes.NewReader([]byte(body)))
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h.handleUpdateOffsiteTarget(rec, req)
	return decodeEnvelope(t, rec)
}

// An additional target moved onto sort order 0 would sit next to the primary,
// and the next settings save could take it for the primary and rewrite it.
func TestUpdateOffsiteTargetRefusesMovingOntoPrimarySortOrder(t *testing.T) {
	h, st := newCRUDHandler(t)
	extra, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Second", Repo: "s3:c2", Enabled: true, SortOrder: 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, order := range []string{"0", "-1"} {
		env := putOffsiteTarget(t, h, extra.ID, `{"domain":"containers","name":"Second","repo":"s3:c2","enabled":true,"sortOrder":`+order+`}`)
		if env["ok"] == true {
			t.Fatalf("sortOrder %s: want a refusal, got %v", order, env)
		}
	}
	got, _, err := st.GetOffsiteTarget(extra.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SortOrder != 2 {
		t.Fatalf("a refused update moved the target to sort order %d", got.SortOrder)
	}
}

// The off-site wizard stores the credential set on the primary through this
// route and sends the row back with its sort order 0.
func TestUpdateOffsiteTargetKeepsEditingThePrimary(t *testing.T) {
	h, st := newCRUDHandler(t)
	primary, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Primary", Repo: "s3:c", Enabled: true, SortOrder: 0})
	if err != nil {
		t.Fatal(err)
	}
	env := putOffsiteTarget(t, h, primary.ID, `{"domain":"containers","name":"Primary","repo":"s3:c","credsRef":"cs1","enabled":true,"sortOrder":0}`)
	if env["ok"] != true {
		t.Fatalf("editing the primary was refused: %v", env)
	}
	if got := env["target"].(map[string]any); got["credsRef"] != "cs1" || got["sortOrder"] != float64(0) {
		t.Fatalf("the primary did not keep its edit and its place: %v", got)
	}
}

// A body without a sort order leaves the target where it is instead of moving
// it onto the primary's 0.
func TestUpdateOffsiteTargetWithoutSortOrderKeepsItsPlace(t *testing.T) {
	h, st := newCRUDHandler(t)
	extra, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Second", Repo: "s3:c2", Enabled: true, SortOrder: 2})
	if err != nil {
		t.Fatal(err)
	}
	env := putOffsiteTarget(t, h, extra.ID, `{"domain":"containers","name":"Renamed","repo":"s3:c2","enabled":true}`)
	if env["ok"] != true {
		t.Fatalf("update not ok: %v", env)
	}
	if got := env["target"].(map[string]any); got["name"] != "Renamed" || got["sortOrder"] != float64(2) {
		t.Fatalf("want the new name at sort order 2, got %v", got)
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
