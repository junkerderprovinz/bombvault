package api

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// storeShared writes c as an install from before sign-ins were one list held
// it.
func storeShared(t *testing.T, s *Service, c CloudCreds) {
	t.Helper()
	blob, err := s.sealSharedBlob(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.MutateSettings(func(st *store.Settings) error {
		st.CloudConf = blob
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// storeSets writes sets without the rules SetCloudCredSets applies, as an
// older install left them.
func storeSets(t *testing.T, s *Service, sets []CloudCredSet) {
	t.Helper()
	enc, err := s.encodeCloudCredSets(sets)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.MutateSettings(func(st *store.Settings) error {
		st.CloudCredSets = enc
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func savedSets(t *testing.T, s *Service) []CloudCredSet {
	t.Helper()
	sets, err := s.decodeCloudCredSets(settingsOf(t, s))
	if err != nil {
		t.Fatal(err)
	}
	return sets
}

func TestTheSharedSignInTakesTheDefaultMarkOfASavedOneWithTheSameKeys(t *testing.T) {
	s := unraidNotifyService(t, nil)
	hetzner := CloudCreds{S3KeyID: "HETZ", S3Secret: "hs", S3Region: "fsn1"}
	storeSets(t, s, []CloudCredSet{
		{ID: "hetzner", Name: "Hetzner S3", CloudCreds: hetzner},
		{ID: "twin", Name: "Hetzner again", CloudCreds: hetzner},
		{ID: "wasabi", Name: "Wasabi", CloudCreds: CloudCreds{S3KeyID: "WAS", S3Secret: "ws"}},
	})
	storeShared(t, s, hetzner)
	twinUser, err := s.store.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "Copy", Repo: "s3:host/b/vms", CredsRef: "twin", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.ConvertSharedSignIns(); err != nil {
		t.Fatal(err)
	}

	sets := savedSets(t, s)
	if len(sets) != 2 || sets[0].ID != "hetzner" || !sets[0].Default || sets[1].Default {
		t.Fatalf("sets = %+v, want Hetzner S3 as the default and Wasabi beside it", sets)
	}
	if settingsOf(t, s).CloudConf != "" {
		t.Fatal("the shared blob stays after the conversion")
	}
	got, _, err := s.store.GetOffsiteTarget(twinUser.ID)
	if err != nil || got.CredsRef != "hetzner" {
		t.Fatalf("a copy on the merged twin signs in with %q (%v), want hetzner", got.CredsRef, err)
	}
	note, show, err := s.store.SignInsNoteToShow()
	if err != nil || !show || note.SetID != "hetzner" || !note.Merged || note.Duplicates != 1 {
		t.Fatalf("note = %+v shown=%v (%v)", note, show, err)
	}
}

func TestASharedSignInWithoutATwinBecomesTheDefaultSignIn(t *testing.T) {
	s := unraidNotifyService(t, nil)
	storeShared(t, s, CloudCreds{RESTUser: "bv", RESTPassword: "pw"})

	if err := s.ConvertSharedSignIns(); err != nil {
		t.Fatal(err)
	}

	sets := savedSets(t, s)
	if len(sets) != 1 || sets[0].Name != "Default sign-in" || !sets[0].Default || sets[0].RESTPassword != "pw" {
		t.Fatalf("sets = %+v", sets)
	}
	env := strings.Join(s.ModeFor(settingsOf(t, s)).Env, "\n")
	if !strings.Contains(env, "RESTIC_REST_PASSWORD=pw") {
		t.Fatalf("a location naming no sign-in signs in with %v, want the default", env)
	}
}

func TestTheConversionRunsOnce(t *testing.T) {
	s := unraidNotifyService(t, nil)
	storeShared(t, s, CloudCreds{RESTUser: "bv", RESTPassword: "pw"})
	if err := s.ConvertSharedSignIns(); err != nil {
		t.Fatal(err)
	}
	storeShared(t, s, CloudCreds{RESTUser: "later", RESTPassword: "x"})
	if err := s.ConvertSharedSignIns(); err != nil {
		t.Fatal(err)
	}
	if got := savedSets(t, s); len(got) != 1 {
		t.Fatalf("a second start converted again: %+v", got)
	}
}

func TestAnInstallWithNothingToConvertShowsNoNote(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.ConvertSharedSignIns(); err != nil {
		t.Fatal(err)
	}
	if _, show, err := s.store.SignInsNoteToShow(); err != nil || show {
		t.Fatalf("shown=%v (%v), want no note", show, err)
	}
}

func TestATypedOwnFolderInALocationsFolderFollowsTheLocation(t *testing.T) {
	s := unraidNotifyService(t, nil)
	storeShared(t, s, CloudCreds{S3KeyID: "K", S3Secret: "S"})
	d, err := s.store.SaveDestination(store.OffsiteTarget{Name: "Garage", Repo: "s3:https://garage/bv"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.MutateSettings(func(st *store.Settings) error {
		st.ContainersPath = "s3:https://garage/bv/containers"
		st.VMsPath = "s3:https://elsewhere/bv/vms"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.ConvertSharedSignIns(); err != nil {
		t.Fatal(err)
	}

	row, ok, err := s.store.PrimaryRemoteTarget("containers")
	if err != nil || !ok || row.DestinationID != d.ID {
		t.Fatalf("containers row = %+v found=%v (%v), want it linked to Garage", row, ok, err)
	}
	if _, ok, _ := s.store.PrimaryRemoteTarget("vms"); ok {
		t.Fatal("an own folder outside every location got linked")
	}
	note, _, _ := s.store.SignInsNoteToShow()
	if !slices.Equal(note.Linked, []string{"containers"}) {
		t.Fatalf("note names %v as linked", note.Linked)
	}
}

func TestAnOwnFolderThatSignsInDifferentlyStaysTyped(t *testing.T) {
	s := unraidNotifyService(t, nil)
	storeShared(t, s, CloudCreds{S3KeyID: "SHARED", S3Secret: "S"})
	storeSets(t, s, []CloudCredSet{{ID: "other", Name: "Other", CloudCreds: CloudCreds{S3KeyID: "OTHER", S3Secret: "O"}}})
	if _, err := s.store.SaveDestination(store.OffsiteTarget{Name: "Garage", Repo: "s3:https://garage/bv", CredsRef: "other"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.MutateSettings(func(st *store.Settings) error {
		st.ContainersPath = "s3:https://garage/bv/containers"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.ConvertSharedSignIns(); err != nil {
		t.Fatal(err)
	}
	if row, ok, _ := s.store.PrimaryRemoteTarget("containers"); ok && row.DestinationID != "" {
		t.Fatal("linking would have changed the keys the own folder signs in with")
	}
}

func TestTheDefaultStaysWhenASavedListLeavesItOut(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCreds(CloudCreds{S3KeyID: "DEF", S3Secret: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCloudCredSets([]CloudCredSet{{ID: "b", Name: "B", CloudCreds: CloudCreds{S3KeyID: "B"}}}); err != nil {
		t.Fatal(err)
	}
	creds, _ := s.decodeCloud(settingsOf(t, s))
	if creds.S3KeyID != "DEF" || creds.S3Secret != "d" {
		t.Fatalf("default = %+v, want it kept", creds)
	}

	if err := s.SetCloudCredSets([]CloudCredSet{{ID: "b", Name: "B", Default: true, CloudCreds: CloudCreds{S3KeyID: "B"}}}); err != nil {
		t.Fatal(err)
	}
	if sets := savedSets(t, s); len(sets) != 1 || !sets[0].Default {
		t.Fatalf("sets = %+v, want B alone as the default once it took the mark", sets)
	}
}

func TestASavedListCannotLeaveOutASignInARowStillUses(t *testing.T) {
	s := unraidNotifyService(t, nil)
	storeSets(t, s, []CloudCredSet{
		{ID: "own", Name: "Own folder", CloudCreds: CloudCreds{S3KeyID: "OWN", S3Secret: "o"}},
		{ID: "spare", Name: "Spare", CloudCreds: CloudCreds{S3KeyID: "SPARE", S3Secret: "s"}},
	})
	if _, err := s.store.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RolePrimary, Domain: "containers", Repo: "s3:host/b/containers", CredsRef: "own", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	err := s.SetCloudCredSets([]CloudCredSet{{ID: "spare", Name: "Spare", CloudCreds: CloudCreds{S3KeyID: "SPARE"}}})
	if err == nil || !strings.Contains(err.Error(), "Own folder") {
		t.Fatalf("err = %v, want the save refused naming Own folder", err)
	}
	if sets := savedSets(t, s); len(sets) != 2 {
		t.Fatalf("sets = %+v, want both kept", sets)
	}

	if err := s.SetCloudCredSets([]CloudCredSet{{ID: "own", Name: "Own folder", CloudCreds: CloudCreds{S3KeyID: "OWN"}}}); err != nil {
		t.Fatalf("leaving out the unused one: %v", err)
	}
	if sets := savedSets(t, s); len(sets) != 1 || sets[0].ID != "own" || sets[0].S3Secret != "o" {
		t.Fatalf("sets = %+v, want Own folder alone with its secret", sets)
	}
}

func TestOnlyOneSignInCanBeTheDefault(t *testing.T) {
	s := unraidNotifyService(t, nil)
	err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "a", Name: "A", Default: true},
		{ID: "b", Name: "B", Default: true},
	})
	if err == nil {
		t.Fatal("two defaults were accepted")
	}
}

func TestAnEmptyOrUnknownSignInNameUsesTheDefault(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "def", Name: "Def", Default: true, CloudCreds: CloudCreds{RESTUser: "def", RESTPassword: "1"}},
		{ID: "other", Name: "Other", CloudCreds: CloudCreds{RESTUser: "other", RESTPassword: "2"}},
	}); err != nil {
		t.Fatal(err)
	}
	settings := settingsOf(t, s)
	for _, ref := range []string{"", "gone"} {
		c, err := s.decodeCloudFor(settings, ref)
		if err != nil || c.RESTUser != "def" {
			t.Fatalf("ref %q signs in as %q (%v), want the default", ref, c.RESTUser, err)
		}
	}
	if c, _ := s.decodeCloudFor(settings, "other"); c.RESTUser != "other" {
		t.Fatalf("a named sign-in resolved to %q", c.RESTUser)
	}
}

func TestASettingsFileCarriesTheSavedSignInsAndTheDefault(t *testing.T) {
	from := unraidNotifyService(t, nil)
	if err := from.SetCloudCredSets([]CloudCredSet{
		{ID: "def", Name: "Def", Default: true, CloudCreds: CloudCreds{S3KeyID: "D", S3Secret: "ds"}},
		{ID: "nas", Name: "NAS", CloudCreds: CloudCreds{RESTUser: "u", RESTPassword: "p"}},
	}); err != nil {
		t.Fatal(err)
	}
	creds, err := (&Handler{store: from.store, svc: from}).collectCredentials(settingsOf(t, from))
	if err != nil {
		t.Fatal(err)
	}
	if creds.Cloud.S3KeyID != "D" {
		t.Fatalf("the file's cloud block is %+v, want the default for older readers", creds.Cloud)
	}

	to := unraidNotifyService(t, nil)
	if err := (&Handler{store: to.store, svc: to}).applyImportedCredentials(*creds); err != nil {
		t.Fatal(err)
	}
	sets := savedSets(t, to)
	if len(sets) != 2 || !sets[0].Default || sets[0].S3Secret != "ds" || sets[1].RESTPassword != "p" {
		t.Fatalf("imported sets = %+v", sets)
	}
}

func TestAnOldSettingsFilesSharedCredentialsBecomeTheDefault(t *testing.T) {
	s := unraidNotifyService(t, nil)
	twin := CloudCreds{S3KeyID: "T", S3Secret: "t"}
	if err := s.SetCloudCredSets([]CloudCredSet{{ID: "twin", Name: "Twin", CloudCreds: twin}}); err != nil {
		t.Fatal(err)
	}
	if err := (&Handler{store: s.store, svc: s}).applyImportedCredentials(exportCredentials{Cloud: twin}); err != nil {
		t.Fatal(err)
	}
	sets := savedSets(t, s)
	if len(sets) != 1 || !sets[0].Default {
		t.Fatalf("sets = %+v, want the twin marked default and nothing added", sets)
	}

	if err := (&Handler{store: s.store, svc: s}).applyImportedCredentials(exportCredentials{Cloud: CloudCreds{RESTUser: "new"}}); err != nil {
		t.Fatal(err)
	}
	sets = savedSets(t, s)
	if len(sets) != 2 || sets[0].Default || !sets[1].Default || sets[1].Name != "Default sign-in" {
		t.Fatalf("sets = %+v, want a new Default sign-in carrying the mark", sets)
	}
}

func TestAWizardSignsInWithASavedSignInItNames(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{{ID: "nas", Name: "NAS", CloudCreds: CloudCreds{RESTUser: "u", RESTPassword: "p"}}}); err != nil {
		t.Fatal(err)
	}
	req := draftRequest{Provider: "rest", CredsRef: "nas", Settings: map[string]string{"url": "http://nas:8000/bv"}}
	if err := s.signInFromSaved(&req); err != nil {
		t.Fatal(err)
	}
	if req.Settings["user"] != "u" || req.Settings["pass"] != "p" {
		t.Fatalf("draft settings = %v", req.Settings)
	}
	d, err := s.SaveNewDestination(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if d.CredsRef != "nas" || len(savedSets(t, s)) != 1 {
		t.Fatalf("destination signs in with %q and %d sets exist, want the saved one reused", d.CredsRef, len(savedSets(t, s)))
	}

	gone := draftRequest{Provider: "rest", CredsRef: "gone"}
	if err := s.signInFromSaved(&gone); err == nil {
		t.Fatal("a draft naming a missing sign-in was filled")
	}
}

func TestMakingASignInTheDefaultMovesTheMarkAndTheRuns(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "old", Name: "Old", Default: true, CloudCreds: CloudCreds{S3KeyID: "OLD", S3Secret: "os"}},
		{ID: "new", Name: "New", CloudCreds: CloudCreds{S3KeyID: "NEW", S3Secret: "ns"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.MakeDefaultSignIn("new"); err != nil {
		t.Fatal(err)
	}
	sets := savedSets(t, s)
	if len(sets) != 2 || sets[0].Default || !sets[1].Default || sets[1].S3Secret != "ns" {
		t.Fatalf("sets = %+v, want New alone as the default with its secret", sets)
	}
	settings := settingsOf(t, s)
	if env := strings.Join(s.ModeFor(settings).Env, "\n"); !strings.Contains(env, "AWS_ACCESS_KEY_ID=NEW") {
		t.Fatalf("a run signs in with %v, want the new default", env)
	}
	if c, _ := s.decodeCloudFor(settings, ""); c.S3KeyID != "NEW" {
		t.Fatalf("a place that names no sign-in uses %q", c.S3KeyID)
	}
	if c, _ := s.decodeCloudFor(settings, "old"); c.S3KeyID != "OLD" {
		t.Fatalf("a place that names the old default uses %q", c.S3KeyID)
	}
}

func TestAnUnknownSignInCannotBecomeTheDefault(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{{ID: "a", Name: "A", Default: true, CloudCreds: CloudCreds{RESTUser: "a"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.MakeDefaultSignIn("gone"); err == nil {
		t.Fatal("an unknown sign-in became the default")
	}
	if sets := savedSets(t, s); !sets[0].Default {
		t.Fatalf("sets = %+v, want A kept as the default", sets)
	}
}

func TestTheDefaultCanBeMovedOverTheAPI(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "a", Name: "A", Default: true, CloudCreds: CloudCreds{RESTUser: "a", RESTPassword: "1"}},
		{ID: "b", Name: "B", CloudCreds: CloudCreds{RESTUser: "b", RESTPassword: "2"}},
	}); err != nil {
		t.Fatal(err)
	}
	if res := f.do("POST", "/api/cloud/creds-sets/b/default", nil); res["ok"] != true {
		t.Fatalf("answer = %v", res)
	}
	if c, _ := f.svc.CloudConfig(); c.RESTUser != "b" {
		t.Fatalf("the default signs in as %q, want b", c.RESTUser)
	}
	if res := f.do("POST", "/api/cloud/creds-sets/gone/default", nil); res["ok"] != false {
		t.Fatalf("an unknown sign-in was accepted: %v", res)
	}
}

func TestTheDefaultSignInGoesOnlyAsTheLastOne(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "a", Name: "A", Default: true, CloudCreds: CloudCreds{RESTUser: "a"}},
		{ID: "b", Name: "B", CloudCreds: CloudCreds{RESTUser: "b"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveSignIn("a"); !errors.Is(err, errDefaultSignInStays) {
		t.Fatalf("the default went while another sign-in was left: %v", err)
	}
	if err := s.RemoveSignIn("b"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveSignIn("a"); err != nil {
		t.Fatal(err)
	}
	if sets := savedSets(t, s); len(sets) != 0 {
		t.Fatalf("sets = %+v, want none", sets)
	}
	if err := s.RemoveSignIn("a"); !errors.Is(err, errUnknownSignIn) {
		t.Fatalf("a gone sign-in was removed again: %v", err)
	}
}

func TestASignInAStorageLocationUsesStays(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, opensWithPassword("old"))
	d, _ := restDestination(t, svc, st, "vms")
	err := svc.RemoveSignIn(d.CredsRef)
	if err == nil || !strings.Contains(err.Error(), "Box still signs in with it") {
		t.Fatalf("err = %v, want the location named", err)
	}
	if got := signInOf(t, svc, d); got.RESTPassword != "old" {
		t.Fatalf("the sign-in of Box is %+v", got)
	}
}

func TestASignInCanBeRemovedOverTheAPI(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "a", Name: "A", Default: true, CloudCreds: CloudCreds{RESTUser: "a", RESTPassword: "1"}},
		{ID: "b", Name: "B", CloudCreds: CloudCreds{RESTUser: "b", RESTPassword: "2"}},
	}); err != nil {
		t.Fatal(err)
	}
	if res := f.do("DELETE", "/api/cloud/creds-sets/b", nil); res["ok"] != true {
		t.Fatalf("answer = %v", res)
	}
	if sets := savedSets(t, f.svc); len(sets) != 1 || sets[0].ID != "a" {
		t.Fatalf("sets = %+v, want A alone", sets)
	}
	if res := f.do("DELETE", "/api/cloud/creds-sets/a", nil); res["ok"] != true {
		t.Fatalf("the last sign-in was kept: %v", res)
	}
}

func TestASignInIsTestedWithTheEditedValuesWithoutSavingThem(t *testing.T) {
	svc, st, eng := newConnectionSvc(t, opensWithPassword("new"))
	d, _ := restDestination(t, svc, st, "vms", "flash")
	if d.CredsRef == "" {
		t.Fatal("the destination saved no sign-in of its own")
	}

	checks, err := svc.TestSignIn(t.Context(), CloudCredSet{ID: d.CredsRef, CloudCreds: CloudCreds{RESTUser: "bv", RESTPassword: "new"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || checks[0].ID != "destination:"+d.ID || !checks[0].Reachable || !checks[0].Initialized {
		t.Fatalf("checks = %+v, want the destination reached", checks)
	}
	if len(eng.probed) != 1 || !strings.HasPrefix(eng.probed[0], "rest:https://box:8000/bv/") {
		t.Fatalf("probed %v, want the folder of one of the destination's sections", eng.probed)
	}
	if got := signInOf(t, svc, d); got.RESTPassword != "old" {
		t.Fatalf("the test saved the password %q", got.RESTPassword)
	}

	checks, err = svc.TestSignIn(t.Context(), CloudCredSet{ID: d.CredsRef, CloudCreds: CloudCreds{RESTUser: "bv"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || checks[0].Reachable || checks[0].Error == "" {
		t.Fatalf("checks = %+v, want the stored password tried and refused", checks)
	}
}

func TestASignInNothingUsesHasNothingToTest(t *testing.T) {
	svc, _, eng := newConnectionSvc(t, opensWithPassword("p"))
	if err := svc.SetCloudCredSets([]CloudCredSet{{ID: "spare", Name: "Spare", CloudCreds: CloudCreds{RESTUser: "u", RESTPassword: "p"}}}); err != nil {
		t.Fatal(err)
	}
	checks, err := svc.TestSignIn(t.Context(), CloudCredSet{ID: "spare"})
	if err != nil || len(checks) != 0 || len(eng.probed) != 0 {
		t.Fatalf("checks = %+v (%v), probed %v, want nothing tried", checks, err, eng.probed)
	}
	if _, err := svc.TestSignIn(t.Context(), CloudCredSet{ID: "gone"}); !errors.Is(err, errUnknownSignIn) {
		t.Fatalf("an unknown sign-in was tested: %v", err)
	}
}

func TestASignInCanBeTestedOverTheAPI(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "a", Name: "A", CloudCreds: CloudCreds{RESTUser: "a", RESTPassword: "1"}}}); err != nil {
		t.Fatal(err)
	}
	res := f.do("POST", "/api/cloud/creds-sets/a/test", map[string]any{"restUser": "a"})
	if locations, ok := res["locations"].([]any); res["ok"] != true || !ok || len(locations) != 0 {
		t.Fatalf("answer = %v", res)
	}
}

// envRecorder opens every repository and keeps the environment each open
// went out with.
type envRecorder struct {
	ResticEngine
	envs [][]string
}

func (e *envRecorder) RepoOpens(_ context.Context, _ string, m restic.Mode) bool {
	e.envs = append(e.envs, m.Env)
	return true
}

func pullSourceSigningInWith(t *testing.T, s *Service, ref string) store.PullSource {
	t.Helper()
	if err := s.SetCloudCredSets([]CloudCredSet{
		{ID: "ours", Name: "Ours", Default: true, CloudCreds: CloudCreds{RESTUser: "ours", RESTPassword: "our secret"}},
		{ID: "theirs", Name: "Theirs", CloudCreds: CloudCreds{RESTUser: "theirs", RESTPassword: "their secret"}},
	}); err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, []byte("their restic password"))
	if err != nil {
		t.Fatal(err)
	}
	ps, err := s.store.CreatePullSource(store.PullSource{
		Name: "Tower next door", Repo: "rest:http://192.168.1.9:8000/containers", ResticPasswordEnc: enc,
		CredsRef: ref, Domain: "containers", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

func TestASignInAPullSourceUsesStaysAndNamesTheSource(t *testing.T) {
	s := unraidNotifyService(t, nil)
	pullSourceSigningInWith(t, s, "theirs")
	err := s.RemoveSignIn("theirs")
	if err == nil || !strings.Contains(err.Error(), "Tower next door still signs in with it") {
		t.Fatalf("err = %v, want the pull source named", err)
	}
	users, err := s.signInUsers("ours")
	if err != nil {
		t.Fatal(err)
	}
	if len(users["theirs"]) != 1 || users["theirs"][0].Name != "Tower next door" {
		t.Fatalf("users of Theirs = %+v, want the pull source", users["theirs"])
	}
}

func TestASignInARowTheListLeavesOutUsesStays(t *testing.T) {
	s := unraidNotifyService(t, nil)
	pullSourceSigningInWith(t, s, "")
	if _, err := s.store.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{Name: "Left behind", Repo: "rest:http://192.168.1.9:8000/x", CredsRef: "theirs"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveSignIn("theirs"); err == nil || !strings.Contains(err.Error(), "Theirs is still in use") {
		t.Fatalf("err = %v, want the sign-in kept", err)
	}
}

func TestAPullSourceWhoseSignInIsGoneDoesNotBorrowTheDefault(t *testing.T) {
	s := unraidNotifyService(t, nil)
	eng := &envRecorder{}
	s.engine = eng
	ps := pullSourceSigningInWith(t, s, "theirs")
	storeSets(t, s, []CloudCredSet{{ID: "ours", Name: "Ours", Default: true, CloudCreds: CloudCreds{RESTUser: "ours", RESTPassword: "our secret"}}})
	_, _, err := s.pullOpen(context.Background(), ps, settingsOf(t, s))
	if err == nil || !strings.Contains(err.Error(), "sign-in this source used is gone") {
		t.Fatalf("err = %v, want the missing sign-in named", err)
	}
	for _, env := range eng.envs {
		if slices.Contains(env, "RESTIC_REST_PASSWORD=our secret") {
			t.Fatal("the pull source was opened with this instance's default sign-in")
		}
	}
}
