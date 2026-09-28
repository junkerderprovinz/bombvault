package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// meshHandlerFixture returns a Handler on an in-memory store whose Service
// shares appKey.
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
	svc := NewService(cfg, st, nil, nil, restic.Restic{Bin: "restic"})
	return &Handler{cfg: cfg, store: st, svc: svc}, st
}

func TestMemberMeshOfferIsStoredPendingWithItsPasswordSealed(t *testing.T) {
	appKey := strings.Repeat("a", 64)
	h, st := meshHandlerFixture(t, appKey)

	body, _ := json.Marshal(meshOfferRequest{
		FromName: "tower-a", SuggestedDomain: "containers",
		Repo:     "rest:http://192.168.1.50:8000/bombvault-containers/containers",
		RESTUser: "bombvault-containers", RESTPassword: "s3cr3t",
	})
	status, out := h.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodPost, Path: "/api/group/peer/mesh-offer", Body: body})
	if status != http.StatusOK {
		t.Fatalf("offer: want 200, got %d body=%s", status, out)
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

	w := httptest.NewRecorder()
	r := postJSONReq(t, "/api/fleet/mesh-offers/"+offer.ID+"/accept", map[string]any{"domain": "not-a-domain"})
	r.SetPathValue("id", offer.ID)
	h.handleAcceptMeshOffer(w, r)
	if resp := decodeResp(t, w); resp["ok"] != false {
		t.Fatalf("invalid domain must be rejected: %v", resp)
	} else if msg, _ := resp["error"].(string); !strings.Contains(msg, "zfs") {
		t.Fatalf("the error leaves out a domain the offer can go to: %q", msg)
	}

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

// A target taken over from a mesh offer is an additional target, so a settings
// save on a domain without an off-site repo of its own keeps it.
func TestAcceptedMeshTargetSurvivesSettingsSave(t *testing.T) {
	appKey := strings.Repeat("d", 64)
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
	w := httptest.NewRecorder()
	r := postJSONReq(t, "/api/fleet/mesh-offers/"+offer.ID+"/accept", map[string]any{"domain": "containers"})
	r.SetPathValue("id", offer.ID)
	h.handleAcceptMeshOffer(w, r)
	if resp := decodeResp(t, w); resp["ok"] != true {
		t.Fatalf("accept must succeed: %v", resp)
	}

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	h.svc.syncAllPrimaryOffsiteTargets(settings)

	targets, err := st.OffsiteTargetsForDomain("containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Repo != offer.Repo || targets[0].SortOrder == 0 {
		t.Fatalf("mesh target should survive a settings save outside sort order 0, got %+v", targets)
	}
}

// A mesh target left at sort order 0 moves behind the domain's other targets,
// so the next settings save keeps it. The primary on the repo the settings
// name stays in place.
func TestMeshTargetAtSortOrderZeroMovesBehindThePrimary(t *testing.T) {
	h, st := meshHandlerFixture(t, strings.Repeat("e", 64))

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersOffsite = "s3:containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	accept := func(domain, repo string) store.OffsiteTarget {
		t.Helper()
		offer, err := st.CreateMeshOffer(store.MeshOffer{From: "tower-a", Repo: repo})
		if err != nil {
			t.Fatal(err)
		}
		if err := st.UpdateMeshOfferStatus(offer.ID, "accepted"); err != nil {
			t.Fatal(err)
		}
		row, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
			Domain: domain, Name: "mesh: tower-a", Repo: repo, Enabled: true, SortOrder: 0,
		})
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	primary, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: "s3:containers", Enabled: true, SortOrder: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	extra, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Second copy", Repo: "s3:containers-extra", Enabled: true, SortOrder: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	containersMesh := accept("containers", "rest:http://tower-a:8000/containers")
	vmsMesh := accept("vms", "rest:http://tower-a:8000/vms")

	moved, err := h.svc.MoveTargetsOffPrimarySlot()
	if err != nil {
		t.Fatal(err)
	}
	h.svc.syncAllPrimaryOffsiteTargets(settings)

	vms, err := st.OffsiteTargetsForDomain("vms")
	if err != nil {
		t.Fatal(err)
	}
	if len(vms) != 1 || vms[0].ID != vmsMesh.ID || vms[0].SortOrder == 0 {
		t.Fatalf("vms mesh target should survive a settings save outside sort order 0, got %+v", vms)
	}
	containers, err := st.OffsiteTargetsForDomain("containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(containers) != 3 ||
		containers[0].ID != primary.ID || containers[0].SortOrder != 0 ||
		containers[1].ID != extra.ID ||
		containers[2].ID != containersMesh.ID || containers[2].SortOrder != 4 {
		t.Fatalf("want primary, additional target, then the mesh target at 4, got %+v", containers)
	}
	if moved != 2 {
		t.Fatalf("moved = %d, want 2", moved)
	}
	if again, err := h.svc.MoveTargetsOffPrimarySlot(); err != nil || again != 0 {
		t.Fatalf("a second run moved %d (err %v), want 0", again, err)
	}
}

// The startup repair also covers mesh targets accepted into the ZFS domain.
func TestZFSMeshTargetAtSortOrderZeroMovesOffIt(t *testing.T) {
	h, st := meshHandlerFixture(t, strings.Repeat("f", 64))

	repo := "rest:http://tower-a:8000/zfs"
	offer, err := st.CreateMeshOffer(store.MeshOffer{From: "tower-a", Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateMeshOfferStatus(offer.ID, "accepted"); err != nil {
		t.Fatal(err)
	}
	mesh, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: zfsDomain, Name: "mesh: tower-a", Repo: repo, Enabled: true, SortOrder: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	moved, err := h.svc.MoveTargetsOffPrimarySlot()
	if err != nil {
		t.Fatal(err)
	}
	if moved != 1 {
		t.Fatalf("moved = %d, want 1", moved)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	h.svc.syncAllPrimaryOffsiteTargets(settings)

	targets, err := st.OffsiteTargetsForDomain(zfsDomain)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].ID != mesh.ID || targets[0].SortOrder != 1 {
		t.Fatalf("zfs mesh target should sit at sort order 1 after a settings save, got %+v", targets)
	}
}

func TestDeclineMeshOffer(t *testing.T) {
	h, st := meshHandlerFixture(t, strings.Repeat("c", 64))
	offer, err := st.CreateMeshOffer(store.MeshOffer{From: "tower-a", Repo: "rest:http://x:8000/y"})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := jsonReq(http.MethodPost, "/api/fleet/mesh-offers/"+offer.ID+"/decline", nil)
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

	w = httptest.NewRecorder()
	r = jsonReq(http.MethodPost, "/api/fleet/mesh-offers/does-not-exist/decline", nil)
	r.SetPathValue("id", "does-not-exist")
	h.handleDeclineMeshOffer(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown id: want 404, got %d", w.Code)
	}
}
