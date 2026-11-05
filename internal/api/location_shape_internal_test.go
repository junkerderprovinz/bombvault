package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const optionLikeLocation = "sftp:backup@-badhost:/srv/repo"

func refusedForItsShape(t *testing.T, what string, w *httptest.ResponseRecorder) {
	t.Helper()
	resp := decodeResp(t, w)
	if msg, _ := resp["error"].(string); resp["ok"] != false || !strings.Contains(msg, "start with a dash") {
		t.Errorf("%s = %v, want the location refused for its dash", what, resp)
	}
}

func TestAMemberCannotOfferAHostThatReadsAsAnOption(t *testing.T) {
	h, st := meshHandlerFixture(t, strings.Repeat("a", 64))
	const member = "0123456789abcdef0123456789abcdef"
	call := signedOfferCall(t, h.svc, pinnedSeed(t, st, member), member, meshOfferRequest{FromName: "tower-a", Repo: optionLikeLocation, RESTUser: "bv", RESTPassword: "invented"}, time.Now().UnixMilli())

	status, out := h.svc.servePeer(context.Background(), call)

	if status != http.StatusOK || !strings.Contains(string(out), "start with a dash") {
		t.Errorf("offer answered %d %s, want the location refused", status, out)
	}
	if offers, _ := st.ListMeshOffers(); len(offers) != 0 {
		t.Errorf("the offer was stored: %+v", offers)
	}
}

func TestAStoredOfferOfSuchAHostIsNotAccepted(t *testing.T) {
	appKey := strings.Repeat("a", 64)
	h, st := meshHandlerFixture(t, appKey)
	enc, err := secret.Encrypt(appKey, []byte("invented"))
	if err != nil {
		t.Fatal(err)
	}
	offer, err := st.CreateMeshOffer(store.MeshOffer{From: "tower-a", Repo: optionLikeLocation, RESTUser: "bv", RESTPasswordEnc: enc})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := postJSONReq(t, "/api/fleet/mesh-offers/"+offer.ID+"/accept", map[string]any{"domain": "containers"})
	r.SetPathValue("id", offer.ID)
	h.handleAcceptMeshOffer(w, r)

	refusedForItsShape(t, "accept", w)
	if targets, _ := st.OffsiteTargetsForDomain("containers"); len(targets) != 0 {
		t.Errorf("the offer became a target: %+v", targets)
	}
}

func TestEveryRouteThatStoresALocationRefusesSuchAHost(t *testing.T) {
	h, st := hiddenLocationHandler(t)

	saved, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	setting := toView(saved)
	setting.FilesOffsite = optionLikeLocation
	w := httptest.NewRecorder()
	h.handlePutSettings(w, postJSONReq(t, "/api/settings", setting))
	refusedForItsShape(t, "a settings save", w)

	w = httptest.NewRecorder()
	h.handleCreateOffsiteTarget(w, postJSONReq(t, "/api/offsite/targets", map[string]any{"domain": "containers", "repo": optionLikeLocation}))
	refusedForItsShape(t, "a new target", w)

	w = httptest.NewRecorder()
	h.handleCreateNamedRepo(w, postJSONReq(t, "/api/repos", map[string]any{"name": "Box", "repo": optionLikeLocation}))
	refusedForItsShape(t, "a new repository", w)

	w = httptest.NewRecorder()
	h.handleNewTargetPreview(w, httptest.NewRequest(http.MethodGet, "/api/placement/new-target-preview?domain=containers&as=target&repo="+optionLikeLocation, nil))
	refusedForItsShape(t, "the question before a save", w)

	target, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Box", Repo: "s3:box/bucket", Enabled: true, SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	moved := postJSONReq(t, "/api/offsite/targets/"+target.ID, map[string]any{"domain": "containers", "repo": optionLikeLocation})
	moved.SetPathValue("id", target.ID)
	h.handleUpdateOffsiteTarget(w, moved)
	refusedForItsShape(t, "a moved target", w)
}

func TestASettingsFileWithSuchAHostIsRefused(t *testing.T) {
	file := func(edit func(*settingsExport)) settingsExport {
		exp := settingsExport{SchemaVersion: settingsExportSchema}
		edit(&exp)
		return exp
	}
	for name, exp := range map[string]settingsExport{
		"settings": file(func(e *settingsExport) { e.Settings.ContainersPath = optionLikeLocation }),
		"target": file(func(e *settingsExport) {
			e.OffsiteTargets = []offsiteTargetView{{Domain: "vms", Repo: optionLikeLocation}}
		}),
		"repository":  file(func(e *settingsExport) { e.NamedRepos = []offsiteTargetView{{Name: "Box", Repo: optionLikeLocation}} }),
		"destination": file(func(e *settingsExport) { e.Destinations = []offsiteTargetView{{Name: "Box", Repo: optionLikeLocation}} }),
	} {
		if err := validateExport(exp, "/host", "/mnt"); err == nil || !strings.Contains(err.Error(), "start with a dash") {
			t.Errorf("%s: import check = %v, want the location refused", name, err)
		}
	}
}

func TestPullSourceAndReceivedRepoRefuseSuchAHost(t *testing.T) {
	s, _ := newSyncTestService(t)
	_, _, err := s.pullOpen(context.Background(), store.PullSource{Repo: optionLikeLocation}, store.Settings{})
	if !errors.Is(err, restic.ErrLocationReadsAsOption) {
		t.Errorf("pull source: %v, want the location refused", err)
	}
	_, _, err = s.receiverOpen(context.Background(), store.ReceivedRepo{Repo: optionLikeLocation})
	if !errors.Is(err, restic.ErrLocationReadsAsOption) {
		t.Errorf("received repository: %v, want the location refused", err)
	}
}

func TestSettingsSaveKeepsWorkingWithSuchAHostAlreadyStored(t *testing.T) {
	h, st := hiddenLocationHandler(t)
	saved, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	saved.FilesOffsite = optionLikeLocation
	if err := st.UpdateSettings(saved); err != nil {
		t.Fatal(err)
	}
	unrelated := toView(saved)
	unrelated.InstanceName = "tower"

	w := httptest.NewRecorder()
	h.handlePutSettings(w, postJSONReq(t, "/api/settings", unrelated))

	if resp := decodeResp(t, w); resp["ok"] != true {
		t.Errorf("an unrelated save was refused: %v", resp)
	}
}
