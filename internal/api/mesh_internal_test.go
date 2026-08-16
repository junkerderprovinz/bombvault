package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// meshHandlerFixture mirrors receiverHandlerFixture/drDrillService: a Handler
// wired to a real (temp) store, sharing one app secret key. Enough for the
// mesh endpoints, which use only h.store, h.svc and h.cfg.
func meshHandlerFixture(t *testing.T, appKey string) (*Handler, *store.Repo) {
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
	cfg := config.Config{AppKey: appKey}
	svc := &Service{cfg: cfg, store: st, engine: restic.Restic{Bin: "restic"}}
	return &Handler{cfg: cfg, store: st, svc: svc}, st
}

func TestFleetMeshOfferReceive(t *testing.T) {
	appKey := strings.Repeat("a", 64)
	h, st := meshHandlerFixture(t, appKey)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.FleetToken = "correct-token"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(meshOfferRequest{
		FromName: "tower-a", SuggestedDomain: "containers",
		Repo:     "rest:http://192.168.1.50:8000/bombvault-containers/containers",
		RESTUser: "bombvault-containers", RESTPassword: "s3cr3t",
	})

	// Wrong token -> 403, nothing persisted.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/fleet/mesh-offer", strings.NewReader(string(body)))
	r.Header.Set("X-Fleet-Token", "wrong")
	h.handleFleetMeshOfferReceive(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("wrong token: want 403, got %d", w.Code)
	}
	if all, _ := st.ListMeshOffers(); len(all) != 0 {
		t.Fatalf("a refused offer must persist nothing, got %d rows", len(all))
	}

	// Correct token -> stored as pending, password encrypted at rest.
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/api/fleet/mesh-offer", strings.NewReader(string(body)))
	r.Header.Set("X-Fleet-Token", "correct-token")
	h.handleFleetMeshOfferReceive(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("correct token: want 200, got %d body=%s", w.Code, w.Body.String())
	}
	all, err := st.ListMeshOffers()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("want exactly 1 stored offer, got %d", len(all))
	}
	got := all[0]
	if got.From != "tower-a" || got.SuggestedDomain != "containers" || got.RESTUser != "bombvault-containers" || got.Status != "pending" {
		t.Fatalf("stored offer mismatch: %+v", got)
	}
	if strings.Contains(string(got.RESTPasswordEnc), "s3cr3t") {
		t.Fatal("stored rest_password_enc must not contain the plaintext password")
	}
	dec, err := secret.Decrypt(appKey, got.RESTPasswordEnc)
	if err != nil || string(dec) != "s3cr3t" {
		t.Fatalf("stored password did not decrypt back to the original: dec=%q err=%v", dec, err)
	}
}

func TestAcceptMeshOffer(t *testing.T) {
	appKey := strings.Repeat("b", 64)
	h, st := meshHandlerFixture(t, appKey)

	enc, err := secret.Encrypt(appKey, []byte("peer-password"))
	if err != nil {
		t.Fatal(err)
	}
	offer, err := st.CreateMeshOffer(store.MeshOffer{
		From: "tower-a", SuggestedDomain: "containers",
		Repo:     "rest:http://192.168.1.50:8000/bombvault-containers/containers",
		RESTUser: "bombvault-containers", RESTPasswordEnc: enc,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Invalid domain -> rejected, offer stays pending.
	w := httptest.NewRecorder()
	r := postJSONReq(t, "/api/fleet/mesh-offers/"+offer.ID+"/accept", map[string]any{"domain": "not-a-domain"})
	r.SetPathValue("id", offer.ID)
	h.handleAcceptMeshOffer(w, r)
	if resp := decodeResp(t, w); resp["ok"] != false {
		t.Fatalf("invalid domain must be rejected: %v", resp)
	}

	// Valid accept -> creates a CloudCredSet + OffsiteTarget, marks accepted.
	w = httptest.NewRecorder()
	r = postJSONReq(t, "/api/fleet/mesh-offers/"+offer.ID+"/accept", map[string]any{"domain": "containers"})
	r.SetPathValue("id", offer.ID)
	h.handleAcceptMeshOffer(w, r)
	resp := decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("accept must succeed: %v", resp)
	}

	updated, ok, err := st.GetMeshOffer(offer.ID)
	if err != nil || !ok {
		t.Fatalf("GetMeshOffer: ok=%v err=%v", ok, err)
	}
	if updated.Status != "accepted" {
		t.Fatalf("want status 'accepted', got %q", updated.Status)
	}

	targets, err := st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("want exactly 1 created off-site target, got %d", len(targets))
	}
	tg := targets[0]
	if tg.Domain != "containers" || tg.Repo != offer.Repo || tg.CredsRef == "" || !tg.Enabled {
		t.Fatalf("created target mismatch: %+v", tg)
	}

	sets, err := h.svc.CloudCredSets()
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0].ID != tg.CredsRef || sets[0].RESTUser != "bombvault-containers" {
		t.Fatalf("created credential set mismatch: %+v (target CredsRef=%q)", sets, tg.CredsRef)
	}

	// Re-accepting an already-decided offer is rejected, no duplicate target.
	w = httptest.NewRecorder()
	r = postJSONReq(t, "/api/fleet/mesh-offers/"+offer.ID+"/accept", map[string]any{"domain": "containers"})
	r.SetPathValue("id", offer.ID)
	h.handleAcceptMeshOffer(w, r)
	if resp := decodeResp(t, w); resp["ok"] != false {
		t.Fatalf("re-accepting a decided offer must be rejected: %v", resp)
	}
	if targets, _ := st.ListOffsiteTargets(); len(targets) != 1 {
		t.Fatalf("a rejected re-accept must not create a second target, got %d", len(targets))
	}
}

func TestDeclineMeshOffer(t *testing.T) {
	h, st := meshHandlerFixture(t, strings.Repeat("c", 64))
	offer, err := st.CreateMeshOffer(store.MeshOffer{From: "tower-a", Repo: "rest:http://x:8000/y"})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/fleet/mesh-offers/"+offer.ID+"/decline", nil)
	r.SetPathValue("id", offer.ID)
	h.handleDeclineMeshOffer(w, r)
	if resp := decodeResp(t, w); resp["ok"] != true {
		t.Fatalf("decline must succeed: %v", resp)
	}

	updated, ok, err := st.GetMeshOffer(offer.ID)
	if err != nil || !ok {
		t.Fatalf("GetMeshOffer: ok=%v err=%v", ok, err)
	}
	if updated.Status != "declined" {
		t.Fatalf("want status 'declined', got %q", updated.Status)
	}

	// Unknown id -> 404.
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/api/fleet/mesh-offers/does-not-exist/decline", nil)
	r.SetPathValue("id", "does-not-exist")
	h.handleDeclineMeshOffer(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown id: want 404, got %d", w.Code)
	}
}

// TestProposeMeshOffer pins the sending side end to end: a fake peer server
// verifies it received the right token and a well-formed offer, built from
// the admin-provided base URL rather than the deploy-snippet's generic
// placeholder.
func TestProposeMeshOffer(t *testing.T) {
	var gotToken string
	var gotOffer meshOfferRequest
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Fleet-Token")
		_ = json.NewDecoder(r.Body).Decode(&gotOffer)
		w.WriteHeader(http.StatusOK)
	}))
	defer peer.Close()

	appKey := strings.Repeat("d", 64)
	h, st := meshHandlerFixture(t, appKey)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.InstanceName = "tower-a"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	enc, err := secret.Encrypt(appKey, []byte("peer-token-for-b"))
	if err != nil {
		t.Fatal(err)
	}
	fp, err := st.CreateFleetPeer(store.FleetPeer{Name: "tower-b", URL: peer.URL, TokenEnc: enc, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := postJSONReq(t, "/api/fleet/peers/"+fp.ID+"/mesh-offer", map[string]any{
		"domain": "containers", "baseUrl": "http://192.168.1.9:8000",
	})
	r.SetPathValue("id", fp.ID)
	h.handleProposeMeshOffer(w, r)
	resp := decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("propose must succeed: %v", resp)
	}

	if gotToken != "peer-token-for-b" { //nolint:gosec // G101: a test-fixture literal, not a real credential
		t.Fatalf("peer received token %q, want the decrypted stored peer token", gotToken)
	}
	if gotOffer.FromName != "tower-a" {
		t.Fatalf("offer fromName = %q, want this instance's InstanceName", gotOffer.FromName)
	}
	if gotOffer.SuggestedDomain != "containers" {
		t.Fatalf("offer suggestedDomain = %q, want %q", gotOffer.SuggestedDomain, "containers")
	}
	if !strings.HasPrefix(gotOffer.Repo, "rest:http://192.168.1.9:8000/bombvault-containers/containers") {
		t.Fatalf("offer repo = %q, want it built from the provided baseUrl", gotOffer.Repo)
	}
	if gotOffer.RESTUser != "bombvault-containers" || gotOffer.RESTPassword == "" {
		t.Fatalf("offer credentials incomplete: %+v", gotOffer)
	}
}
