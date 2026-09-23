package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// receiverHandlerFixture builds a Handler over a real in-memory store and the
// real restic engine. The receiver endpoints use only h.store, h.svc and h.cfg.
func receiverHandlerFixture(t *testing.T, appKey string) (*Handler, *store.Repo) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	// A received repo has to live under the host mount like any repo path, and
	// t.TempDir() is inside the OS temp root.
	cfg := config.Config{AppKey: appKey, HostMountRoot: os.TempDir()}
	svc := &Service{cfg: cfg, store: st, engine: restic.Restic{Bin: "restic"}}
	return &Handler{cfg: cfg, store: st, svc: svc}, st
}

func decodeResp(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode response: %v (%s)", err, w.Body.String())
	}
	return m
}

func postJSONReq(t *testing.T, target string, body any) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return jsonReq(http.MethodPost, target, bytes.NewReader(b))
}

// None of these refusals needs restic, and none may persist a row.
func TestReceiverCreateValidation(t *testing.T) {
	h, st := receiverHandlerFixture(t, strings.Repeat("ab", 32))

	w := httptest.NewRecorder()
	h.handleCreateReceiverRepo(w, postJSONReq(t, "/api/receiver/repos", map[string]any{
		"repo": "rest:https://box:8000/vault", "appKey": "not-hex",
	}))
	if resp := decodeResp(t, w); resp["ok"] != false || !strings.Contains(resp["error"].(string), "64 lowercase hex") {
		t.Fatalf("bad app key must be rejected: %v", resp)
	}

	w = httptest.NewRecorder()
	h.handleCreateReceiverRepo(w, postJSONReq(t, "/api/receiver/repos", map[string]any{
		"repo": "   ", "appKey": strings.Repeat("cd", 32),
	}))
	if resp := decodeResp(t, w); resp["ok"] != false || !strings.Contains(resp["error"].(string), "repo must not be empty") {
		t.Fatalf("empty repo must be rejected: %v", resp)
	}

	// The probe refuses a location that does not open, with or without restic
	// installed.
	w = httptest.NewRecorder()
	h.handleCreateReceiverRepo(w, postJSONReq(t, "/api/receiver/repos", map[string]any{
		"repo": filepath.Join(t.TempDir(), "not-a-repo"), "appKey": strings.Repeat("cd", 32),
	}))
	if resp := decodeResp(t, w); resp["ok"] != false {
		t.Fatalf("an unopenable repo must be rejected: %v", resp)
	}

	if all, _ := st.ListReceivedRepos(); len(all) != 0 {
		t.Fatalf("no rejected create may persist a row, got %d", len(all))
	}
}

func TestReceiverCreateAndCheckNow(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	appKey := strings.Repeat("ab", 32)
	sendingKey := strings.Repeat("cd", 32)
	repo := seedReceivedRepo(t, sendingKey)
	h, st := receiverHandlerFixture(t, appKey)

	w := httptest.NewRecorder()
	h.handleCreateReceiverRepo(w, postJSONReq(t, "/api/receiver/repos", map[string]any{
		"name": "Off-site A", "repo": repo, "appKey": sendingKey, "checkCadence": "daily 04:00",
	}))
	resp := decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("create against a real repo must succeed: %v", resp)
	}
	repoView, _ := resp["repo"].(map[string]any)
	id, _ := repoView["id"].(string)
	if id == "" {
		t.Fatalf("create must return a repo id: %v", resp)
	}
	if repoView["hasAppKey"] != true {
		t.Fatalf("view must report a stored key: %v", repoView)
	}
	if _, leaked := repoView["appKey"]; leaked {
		t.Fatal("the view must NEVER carry the app key")
	}
	if repoView["lastCheckOk"] != nil {
		t.Fatalf("a fresh repo must have a null lastCheckOk: %v", repoView["lastCheckOk"])
	}

	w = httptest.NewRecorder()
	cr := jsonReq(http.MethodPost, "/api/receiver/repos/"+id+"/check", nil)
	cr.SetPathValue("id", id)
	h.handleReceiverCheck(w, cr)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("check-now envelope must be ok: %v", resp)
	}
	result, _ := resp["result"].(map[string]any)
	if result["ok"] != true {
		t.Fatalf("check on a healthy repo must pass: %v", result)
	}
	got, _, _ := st.GetReceivedRepo(id)
	if !got.LastCheckOK.Valid || !got.LastCheckOK.Bool || got.LastCheckAt == 0 {
		t.Fatalf("check-now must persist the verdict: %+v", got)
	}
}

func TestReceiverDeleteRemovesRowOnly(t *testing.T) {
	h, st := receiverHandlerFixture(t, strings.Repeat("ab", 32))

	// A real on-disk location with a sentinel file the delete must not touch.
	repoDir := t.TempDir()
	sentinel := filepath.Join(repoDir, "config")
	if err := os.WriteFile(sentinel, []byte("do not delete"), 0o644); err != nil { //nolint:gosec // G306: test file
		t.Fatal(err)
	}
	created, err := st.CreateReceivedRepo(store.ReceivedRepo{Name: "A", Repo: repoDir, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	// Seed a dead-man episode so the delete's cleanup path is exercised.
	if err := st.UpsertReceivedAlertState(store.ReceivedAlertState{ReceivedRepoID: created.ID, Source: "container:web\x00tower", NotifiedAt: 1, BasedOn: 1}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	dr := jsonReq(http.MethodDelete, "/api/receiver/repos/"+created.ID, nil)
	dr.SetPathValue("id", created.ID)
	h.handleDeleteReceiverRepo(w, dr)
	if resp := decodeResp(t, w); resp["ok"] != true {
		t.Fatalf("delete must succeed: %v", resp)
	}

	if _, ok, _ := st.GetReceivedRepo(created.ID); ok {
		t.Fatal("the row must be gone after delete")
	}
	if _, ok, _ := st.GetReceivedAlertState(created.ID, "container:web\x00tower"); ok {
		t.Fatal("the dead-man episode state must be purged with the repo")
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("delete must NOT touch the received repository on disk: %v", err)
	}
}

// Without restic the row lists as unreachable, which does not matter here.
func TestReceiverListReturnsStatusNoKey(t *testing.T) {
	h, st := receiverHandlerFixture(t, strings.Repeat("ab", 32))
	if _, err := st.CreateReceivedRepo(store.ReceivedRepo{Name: "A", Repo: "rest:https://box/vault", AppKeyEnc: []byte("ciphertext"), Enabled: true}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.handleListReceiverRepos(w, httptest.NewRequest(http.MethodGet, "/api/receiver/repos", nil).WithContext(context.Background()))
	resp := decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("list must be ok: %v", resp)
	}
	repos, _ := resp["repos"].([]any)
	if len(repos) != 1 {
		t.Fatalf("want 1 repo, got %d", len(repos))
	}
	row, _ := repos[0].(map[string]any)
	if row["hasAppKey"] != true {
		t.Fatalf("a stored key must surface as hasAppKey: %v", row)
	}
	if _, leaked := row["appKey"]; leaked {
		t.Fatal("the list must NEVER carry the app key")
	}
	if _, ok := row["lastReceived"]; !ok {
		t.Fatalf("the list status must carry lastReceived: %v", row)
	}
}
