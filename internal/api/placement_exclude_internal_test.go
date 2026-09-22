package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (f *placementFixture) skipOf(domain, identity string) []string {
	f.t.Helper()
	rule, found, err := f.st.CopyRuleFor(domain, identity)
	if err != nil || !found {
		f.t.Fatalf("rule of %s: found %v, %v", identity, found, err)
	}
	return slices.Sorted(slices.Values(rule.Skip))
}

func sortedIDs(ids ...string) []string {
	return slices.Sorted(slices.Values(ids))
}

// brokenRule leaves a rule the placement read cannot decode, so an exclusion
// fails only once its target has been written.
func (f *placementFixture) brokenRule(domain, identity string) {
	f.t.Helper()
	if _, err := f.db.Exec(`INSERT INTO offsite_copy_rules (domain, identity, skip) VALUES (?, ?, 'not json')`,
		domain, identity); err != nil {
		f.t.Fatal(err)
	}
}

// meshOffer leaves a pending offer of a peer's rest-server for containers.
func (f *placementFixture) meshOffer() store.MeshOffer {
	f.t.Helper()
	enc, err := secret.Encrypt(f.h.cfg.AppKey, []byte("peer-password"))
	if err != nil {
		f.t.Fatal(err)
	}
	offer, err := f.st.CreateMeshOffer(store.MeshOffer{
		From: "tower-a", SuggestedDomain: "containers",
		Repo: "rest:http://192.0.2.10:8000/bv/containers", RESTUser: "bv", RESTPasswordEnc: enc,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return offer
}

// credSetsChangeWhenATargetIsWritten stands in for another request that saves
// the whole credential set list while this one sits between reading that list
// and taking its own set back.
func (f *placementFixture) credSetsChangeWhenATargetIsWritten(sets []CloudCredSet) {
	f.t.Helper()
	blob, err := json.Marshal(sets)
	if err != nil {
		f.t.Fatal(err)
	}
	enc, err := secret.Encrypt(f.h.cfg.AppKey, blob)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.db.Exec(`CREATE TABLE other_request (sets TEXT)`); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO other_request (sets) VALUES (?)`,
		base64.StdEncoding.EncodeToString(enc)); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.db.Exec(`CREATE TRIGGER other_request_saves AFTER INSERT ON offsite_targets
		BEGIN UPDATE settings SET cloud_cred_sets = (SELECT sets FROM other_request); END`); err != nil {
		f.t.Fatal(err)
	}
}

func TestAnExclusionAddsTheNewTargetToWhatEachItemSkips(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.setDefault("containers", "", b2.ID)
	f.rule("containers", "container:plex", b2.ID)
	f.rule("containers", "container:radarr", store.SkipAll)
	wasabi := f.target("containers", "Wasabi", "s3:https://s3.wasabi.example/bv")

	res := f.do(http.MethodPost, "/api/placement/exclude", map[string]any{
		"domain": "containers", "targetId": wasabi.ID,
		"identities": []string{"container:plex", "container:nginx", "container:radarr"}, "default": true,
	})
	if res["ok"] != true {
		t.Fatalf("exclude = %v", res)
	}
	want := sortedIDs(b2.ID, wasabi.ID)
	if got := f.skipOf("containers", "container:plex"); !reflect.DeepEqual(got, want) {
		t.Errorf("plex = %v, want %v", got, want)
	}
	if got := f.skipOf("containers", "container:nginx"); !reflect.DeepEqual(got, want) {
		t.Errorf("nginx = %v, want the default's skip plus Wasabi", got)
	}
	if got := f.skipOf("containers", "container:radarr"); !reflect.DeepEqual(got, []string{store.SkipAll}) {
		t.Errorf("radarr = %v, want [*] untouched", got)
	}
	d, _, err := f.st.PlacementDefaultFor("containers")
	if err != nil || !reflect.DeepEqual(slices.Sorted(slices.Values(d.Skip)), want) {
		t.Errorf("default skip = %v, %v, want %v", d.Skip, err, want)
	}
}

func TestTheFieldMeansTheRowTheOffsiteFieldEdits(t *testing.T) {
	f := newPlacementFixture(t)
	field := f.fieldTarget("containers", "b2:bucket:containers")
	res := f.do(http.MethodPost, "/api/placement/exclude", map[string]any{
		"domain": "containers", "field": true, "identities": []string{"container:plex"}, "default": false,
	})
	if res["ok"] != true {
		t.Fatalf("exclude = %v", res)
	}
	if got := f.skipOf("containers", "container:plex"); !reflect.DeepEqual(got, []string{field.ID}) {
		t.Fatalf("plex = %v, want the field row", got)
	}
}

func TestAnExclusionTheQuestionCouldNotHaveOfferedIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	vmTarget := f.target("vms", "B2 VMs", "b2:bucket:vms")
	cases := []struct {
		name string
		body map[string]any
		code string
	}{
		{"target and field at once", map[string]any{"domain": "containers", "targetId": b2.ID, "field": true, "identities": []string{"container:plex"}}, "invalid-placement"},
		{"a target of another domain", map[string]any{"domain": "containers", "targetId": vmTarget.ID, "identities": []string{"container:plex"}}, "unknown-target"},
		{"a project folder", map[string]any{"domain": "containers", "targetId": b2.ID, "identities": []string{"stack:immich"}}, "stack-rule"},
		{"an item of another domain", map[string]any{"domain": "containers", "targetId": b2.ID, "identities": []string{"vm:win11"}}, "invalid-placement"},
		{"a domain without a bar", map[string]any{"domain": "flash", "targetId": b2.ID, "identities": []string{}}, "invalid-placement"},
	}
	for _, c := range cases {
		if res := f.do(http.MethodPost, "/api/placement/exclude", c.body); res["code"] != c.code {
			t.Errorf("%s: %v, want %s", c.name, res, c.code)
		}
	}
	if rules, err := f.st.ListCopyRules(); err != nil || len(rules) != 0 {
		t.Fatalf("rules = %v, %v, want none written", rules, err)
	}
}

func TestAddingATargetTakesTheAnswerAlong(t *testing.T) {
	f := newPlacementFixture(t)
	res := f.do(http.MethodPost, "/api/offsite/targets", map[string]any{
		"domain": "containers", "name": "B2", "repo": "b2:bucket:containers", "enabled": true,
		"alsoExclude": map[string]any{"identities": []string{"container:plex"}, "default": false},
	})
	target, _ := res["target"].(map[string]any)
	id, _ := target["id"].(string)
	if res["ok"] != true || id == "" {
		t.Fatalf("create = %v", res)
	}
	if got := f.skipOf("containers", "container:plex"); !reflect.DeepEqual(got, []string{id}) {
		t.Fatalf("plex = %v, want the new target", got)
	}
}

func TestAnAnswerThatCannotBeWrittenCreatesNoTarget(t *testing.T) {
	f := newPlacementFixture(t)
	res := f.do(http.MethodPost, "/api/offsite/targets", map[string]any{
		"domain": "containers", "name": "B2", "repo": "b2:bucket:containers", "enabled": true,
		"alsoExclude": map[string]any{"identities": []string{"stack:immich"}, "default": false},
	})
	if res["code"] != "stack-rule" {
		t.Fatalf("create = %v, want stack-rule", res)
	}
	if targets, err := f.st.OffsiteTargetsForDomain("containers"); err != nil || len(targets) != 0 {
		t.Fatalf("targets = %v, %v, want none", targets, err)
	}
}

func TestAnExclusionThatFailsAfterTheWriteLeavesNoTargetBehind(t *testing.T) {
	f := newPlacementFixture(t)
	f.brokenRule("containers", "container:nginx")
	res := f.do(http.MethodPost, "/api/offsite/targets", map[string]any{
		"domain": "containers", "name": "B2", "repo": "b2:bucket:containers", "enabled": true,
		"alsoExclude": map[string]any{"identities": []string{"container:plex"}, "default": false},
	})
	if res["ok"] != false {
		t.Fatalf("create = %v, want a refusal", res)
	}
	if targets, err := f.st.OffsiteTargetsForDomain("containers"); err != nil || len(targets) != 0 {
		t.Fatalf("targets = %v, %v, want none, so pressing Save again does not add a second one", targets, err)
	}
}

func TestMovingATargetTakesTheAnswerOnlyWithANewLocation(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	body := map[string]any{
		"domain": "containers", "name": "B2", "repo": "b2:bucket:containers", "enabled": true,
		"alsoExclude": map[string]any{"identities": []string{"container:plex"}, "default": false},
	}
	if res := f.do(http.MethodPut, "/api/offsite/targets/"+b2.ID, body); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	if _, found, _ := f.st.CopyRuleFor("containers", "container:plex"); found {
		t.Fatal("a PUT on the same location wrote the exclusion")
	}
	body["repo"] = "b2:other:containers"
	if res := f.do(http.MethodPut, "/api/offsite/targets/"+b2.ID, body); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	if got := f.skipOf("containers", "container:plex"); !reflect.DeepEqual(got, []string{b2.ID}) {
		t.Fatalf("plex = %v, want B2 after the move", got)
	}
}

func TestAMovedTargetSaysItWasSavedWhenOnlyTheExclusionFailed(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.brokenRule("containers", "container:nginx")
	res := f.do(http.MethodPut, "/api/offsite/targets/"+b2.ID, map[string]any{
		"domain": "containers", "name": "B2", "repo": "b2:other:containers", "enabled": true,
		"alsoExclude": map[string]any{"identities": []string{"container:plex"}, "default": false},
	})
	msg, _ := res["error"].(string)
	if res["ok"] != false || !strings.Contains(msg, "the target was saved") {
		t.Fatalf("PUT = %v, want a refusal that says the target itself was saved", res)
	}
	stored, ok, err := f.st.GetOffsiteTarget(b2.ID)
	if err != nil || !ok || stored.Repo != "b2:other:containers" {
		t.Fatalf("target = %v, %v, %v, want the new location kept", stored, ok, err)
	}
}

func TestAcceptingAMeshOfferTakesTheAnswerAlong(t *testing.T) {
	f := newPlacementFixture(t)
	offer := f.meshOffer()
	res := f.do(http.MethodPost, "/api/fleet/mesh-offers/"+offer.ID+"/accept", map[string]any{
		"domain": "containers", "alsoExclude": map[string]any{"identities": []string{"container:plex"}, "default": false},
	})
	target, _ := res["target"].(map[string]any)
	id, _ := target["id"].(string)
	if res["ok"] != true || id == "" {
		t.Fatalf("accept = %v", res)
	}
	if got := f.skipOf("containers", "container:plex"); !reflect.DeepEqual(got, []string{id}) {
		t.Fatalf("plex = %v, want the mesh target", got)
	}
}

func TestAFailedExclusionLeavesAnAcceptedOfferAsItWas(t *testing.T) {
	f := newPlacementFixture(t)
	offer := f.meshOffer()
	kept := CloudCredSet{ID: "cs-1", Name: "Wasabi", CloudCreds: CloudCreds{RESTUser: "bv", RESTPassword: "keep-me"}}
	if err := f.svc.SetCloudCredSets([]CloudCredSet{kept}); err != nil {
		t.Fatal(err)
	}
	f.brokenRule("containers", "container:nginx")

	res := f.do(http.MethodPost, "/api/fleet/mesh-offers/"+offer.ID+"/accept", map[string]any{
		"domain": "containers", "alsoExclude": map[string]any{"identities": []string{"container:plex"}, "default": false},
	})
	if res["ok"] != false {
		t.Fatalf("accept = %v, want a refusal", res)
	}
	if targets, tErr := f.st.OffsiteTargetsForDomain("containers"); tErr != nil || len(targets) != 0 {
		t.Fatalf("targets = %v, %v, want none, so accepting again does not add a second one", targets, tErr)
	}
	settings, sErr := f.st.GetSettings()
	if sErr != nil {
		t.Fatal(sErr)
	}
	sets, dErr := f.svc.decodeCloudCredSets(settings)
	if dErr != nil || len(sets) != 1 || sets[0] != kept {
		t.Fatalf("credential sets = %v, %v, want only %v", sets, dErr, kept)
	}
	again, _, gErr := f.st.GetMeshOffer(offer.ID)
	if gErr != nil || again.Status != "pending" {
		t.Fatalf("offer status = %q, %v, want pending", again.Status, gErr)
	}
}

func TestTakingBackAMeshCredentialSetLeavesTheSetsAnotherRequestWrote(t *testing.T) {
	f := newPlacementFixture(t)
	offer := f.meshOffer()
	kept := CloudCredSet{ID: "cs-1", Name: "Wasabi", CloudCreds: CloudCreds{RESTUser: "bv", RESTPassword: "keep-me"}}
	if err := f.svc.SetCloudCredSets([]CloudCredSet{kept}); err != nil {
		t.Fatal(err)
	}
	added := CloudCredSet{ID: "cs-2", Name: "Backblaze", CloudCreds: CloudCreds{RESTUser: "bv", RESTPassword: "mine"}}
	f.credSetsChangeWhenATargetIsWritten([]CloudCredSet{kept, added})
	f.brokenRule("containers", "container:nginx")

	res := f.do(http.MethodPost, "/api/fleet/mesh-offers/"+offer.ID+"/accept", map[string]any{
		"domain": "containers", "alsoExclude": map[string]any{"identities": []string{"container:plex"}, "default": false},
	})
	if res["ok"] != false {
		t.Fatalf("accept = %v, want a refusal", res)
	}
	settings, sErr := f.st.GetSettings()
	if sErr != nil {
		t.Fatal(sErr)
	}
	want := []CloudCredSet{kept, added}
	sets, dErr := f.svc.decodeCloudCredSets(settings)
	if dErr != nil || !reflect.DeepEqual(sets, want) {
		t.Fatalf("credential sets = %v, %v, want %v", sets, dErr, want)
	}
}
