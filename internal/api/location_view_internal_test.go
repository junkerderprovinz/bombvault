package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const (
	locationPassword = "inventedpw"
	signedLocation   = "rest:https://bob:" + locationPassword + "@box.example:8000/repo"
	hiddenLocation   = "rest:https://[redacted]@box.example:8000/repo"
	hiddenMoved      = "rest:https://[redacted]@box.example:8000/moved"
	signedMoved      = "rest:https://bob:" + locationPassword + "@box.example:8000/moved"
	hiddenElsewhere  = "rest:https://[redacted]@other.example:8000/repo"
)

func hiddenLocationHandler(t *testing.T) (*Handler, *store.Repo) {
	t.Helper()
	h, st := receiverHandlerFixture(t, strings.Repeat("ab", 32))
	h.scheduler = schedule.New(func(string) error { return nil }, st.ListTargets)
	return h, st
}

func getBody(t *testing.T, handler http.HandlerFunc, target string, pathValues ...string) string {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, target, nil)
	for i := 0; i+1 < len(pathValues); i += 2 {
		r.SetPathValue(pathValues[i], pathValues[i+1])
	}
	w := httptest.NewRecorder()
	handler(w, r)
	return w.Body.String()
}

func TestNoAnswerToAnUnpairedCallerHoldsALocationPassword(t *testing.T) {
	h, st := hiddenLocationHandler(t)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersPath = signedLocation
	s.FlashZipExportPath = "rest:https://zip:" + locationPassword + "@box.example:8000/zip"
	s.VMsOffsite = "rest:https://vm:" + locationPassword + "@box.example:8000/vms"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	target, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "files", Name: "Box", Repo: "rest:https://t:" + locationPassword + "@box.example:8000/t", Enabled: true, SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "Named", Repo: "rest:https://n:" + locationPassword + "@box.example:8000/n", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateCompanionRepo(target.ID, "", "rest:https://d:"+locationPassword+"@box.example:8000/d"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePullSource(store.PullSource{Name: "Pull", Repo: "rest:https://p:" + locationPassword + "@box.example:8000/p", Domain: "containers"}); err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(h.cfg.AppKey, []byte("sealed"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateMeshOffer(store.MeshOffer{From: "tower", Repo: "rest:https://m:" + locationPassword + "@box.example:8000/m", RESTPasswordEnc: enc}); err != nil {
		t.Fatal(err)
	}

	for name, body := range map[string]string{
		"settings": getBody(t, h.handleGetSettings, "/api/settings"),
		"targets":  getBody(t, h.handleListOffsiteTargets, "/api/offsite/targets"),
		"repos":    getBody(t, h.handleListNamedRepos, "/api/repos"),
		"pull":     getBody(t, h.handleListPullSources, "/api/pull/sources"),
		"direct":   getBody(t, h.handleGetDirectRepo, "/api/offsite/targets/x/direct", "id", target.ID),
		"offers":   getBody(t, h.handleListMeshOffers, "/api/fleet/mesh-offers"),
	} {
		if strings.Contains(body, locationPassword) {
			t.Errorf("%s hands out the password: %s", name, body)
		}
		if !strings.Contains(body, "[redacted]@box.example") {
			t.Errorf("%s no longer shows the location: %s", name, body)
		}
	}
}

func TestAReceivedRepoViewHidesItsPassword(t *testing.T) {
	v := receivedRepoToView(store.ReceivedRepo{Name: "Recv", Repo: signedLocation})
	if v.Repo != hiddenLocation {
		t.Errorf("repo = %q, want %q", v.Repo, hiddenLocation)
	}
}

func TestTheAnswerToASavedTargetHidesItsPassword(t *testing.T) {
	h, _ := hiddenLocationHandler(t)
	w := httptest.NewRecorder()
	h.handleCreateOffsiteTarget(w, postJSONReq(t, "/api/offsite/targets", map[string]any{"domain": "containers", "repo": signedLocation}))
	if strings.Contains(w.Body.String(), locationPassword) {
		t.Errorf("the answer hands out the password: %s", w.Body)
	}
}

func TestEditingAHiddenTargetLocationMovesItAndKeepsThePassword(t *testing.T) {
	h, st := hiddenLocationHandler(t)
	target, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Box", Repo: signedLocation, Enabled: true, SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	put := func(repo string) map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		r := postJSONReq(t, "/api/offsite/targets/"+target.ID, map[string]any{"domain": "containers", "name": "Box", "repo": repo, "enabled": true})
		r.SetPathValue("id", target.ID)
		h.handleUpdateOffsiteTarget(w, r)
		return decodeResp(t, w)
	}
	stored := func() string {
		t.Helper()
		got, _, err := st.GetOffsiteTarget(target.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got.Repo
	}

	if resp := put(hiddenLocation); resp["ok"] != true {
		t.Fatalf("saving the location as shown: %v", resp)
	}
	if got := stored(); got != signedLocation {
		t.Fatalf("after a save of the location as shown: %q", got)
	}
	if resp := put(hiddenMoved); resp["ok"] != true {
		t.Fatalf("moving it: %v", resp)
	}
	if got := stored(); got != signedMoved {
		t.Fatalf("after the move: %q, want %q", got, signedMoved)
	}
	if resp := put(hiddenElsewhere); resp["ok"] != false || !strings.Contains(resp["error"].(string), "hidden") {
		t.Fatalf("taking the hidden password to another server: %v", resp)
	}
	if got := stored(); got != signedMoved {
		t.Fatalf("a refused edit changed the location: %q", got)
	}
}

func TestEditingAHiddenSettingsLocationKeepsThePassword(t *testing.T) {
	h, st := hiddenLocationHandler(t)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersPath = signedLocation
	s.ContainersOffsite = "rest:https://off:" + locationPassword + "@off.example:8000/repo"
	s.FlashZipExportPath = "rest:https://zip:" + locationPassword + "@zip.example:8000/zip"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	var shown struct {
		Settings settingsView `json:"settings"`
	}
	if err := json.Unmarshal([]byte(getBody(t, h.handleGetSettings, "/api/settings")), &shown); err != nil {
		t.Fatal(err)
	}
	shown.Settings.ContainersPath = hiddenMoved
	shown.Settings.ContainersOffsite = strings.Replace(shown.Settings.ContainersOffsite, "/repo", "/moved", 1)

	w := httptest.NewRecorder()
	h.handlePutSettings(w, postJSONReq(t, "/api/settings", shown.Settings))
	if resp := decodeResp(t, w); resp["ok"] != true {
		t.Fatalf("save: %v", resp)
	}
	after, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if after.ContainersPath != signedMoved {
		t.Errorf("own path = %q, want %q", after.ContainersPath, signedMoved)
	}
	if want := "rest:https://off:" + locationPassword + "@off.example:8000/moved"; after.ContainersOffsite != want {
		t.Errorf("off-site = %q, want %q", after.ContainersOffsite, want)
	}
	if after.FlashZipExportPath != s.FlashZipExportPath {
		t.Errorf("flash ZIP path = %q, want it kept", after.FlashZipExportPath)
	}

	shown.Settings.ContainersPath = hiddenElsewhere
	w = httptest.NewRecorder()
	h.handlePutSettings(w, postJSONReq(t, "/api/settings", shown.Settings))
	if resp := decodeResp(t, w); resp["ok"] != false || !strings.Contains(resp["error"].(string), "hidden") {
		t.Errorf("taking the hidden password to another server: %v", resp)
	}
}

func TestEditingAHiddenRepositoryLocationKeepsThePassword(t *testing.T) {
	h, st := hiddenLocationHandler(t)
	row, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "Named", Repo: signedLocation, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := postJSONReq(t, "/api/repos/"+row.ID, map[string]any{"repo": hiddenMoved})
	r.SetPathValue("id", row.ID)
	h.handleUpdateNamedRepo(w, r)
	if resp := decodeResp(t, w); resp["ok"] != true {
		t.Fatalf("patch: %v", resp)
	}
	got, err := st.GetNamedRepo(row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Repo != signedMoved {
		t.Errorf("repo = %q, want %q", got.Repo, signedMoved)
	}
}

func TestSavingAReceivedRepoAsShownKeepsItsPassword(t *testing.T) {
	h, st := hiddenLocationHandler(t)
	row, err := st.CreateReceivedRepo(store.ReceivedRepo{Name: "Recv", Repo: signedLocation})
	if err != nil {
		t.Fatal(err)
	}
	built, _, msg := h.buildReceivedRepo(t.Context(), receivedRepoInput{Name: "Recv", Repo: hiddenMoved, CheckCadence: "off"}, row, false)
	if msg != "" || built.Repo != signedMoved {
		t.Errorf("built %q, %q, want %q", built.Repo, msg, signedMoved)
	}
	if _, _, msg := h.buildReceivedRepo(t.Context(), receivedRepoInput{Name: "Recv", Repo: hiddenElsewhere, CheckCadence: "off"}, row, false); !strings.Contains(msg, "hidden") {
		t.Errorf("another server: %q", msg)
	}
}

func TestSavingAPullSourceAsShownKeepsItsPassword(t *testing.T) {
	h, _ := hiddenLocationHandler(t)
	row := store.PullSource{Name: "Pull", Repo: signedLocation, Domain: "containers"}
	built, _, msg := h.buildPullSource(t.Context(), pullSourceInput{Name: "Pull", Repo: hiddenMoved, Domain: "containers", Cadence: "off"}, row, false)
	if msg != "" || built.Repo != signedMoved {
		t.Errorf("built %q, %q, want %q", built.Repo, msg, signedMoved)
	}
}

func TestTheQuestionBeforeASaveReadsAHiddenPassword(t *testing.T) {
	h, st := hiddenLocationHandler(t)
	target, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Box", Repo: signedLocation, Enabled: true, SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	loc, err := h.askedLocation("containers", target.ID, hiddenMoved, "target")
	if err != nil || loc != signedMoved {
		t.Errorf("askedLocation = %q, %v, want %q", loc, err, signedMoved)
	}
	if _, err := h.askedLocation("containers", target.ID, hiddenElsewhere, "target"); err == nil {
		t.Error("asked about another server with the hidden password, want a refusal")
	}
}

func TestADirectRepositorySuggestionHidesThePasswordAndComesBackWithIt(t *testing.T) {
	h, st := hiddenLocationHandler(t)
	target, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Box", Repo: signedLocation, Enabled: true, SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	body := getBody(t, h.handleGetDirectRepo, "/api/offsite/targets/x/direct", "id", target.ID)
	if strings.Contains(body, locationPassword) {
		t.Fatalf("the suggestion hands out the password: %s", body)
	}
	if got, err := unhideLocation(hiddenLocation+"-direct", target.Repo); err != nil || got != signedLocation+"-direct" {
		t.Errorf("unhideLocation = %q, %v", got, err)
	}
}
