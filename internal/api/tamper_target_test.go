package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestATamperTestOfOneTargetLeavesTheOthersAlone(t *testing.T) {
	var refusing, accepting []string
	guarded := httptest.NewServer(deleteRecorder(http.StatusForbidden, &refusing))
	defer guarded.Close()
	open := httptest.NewServer(deleteRecorder(http.StatusOK, &accepting))
	defer open.Close()

	svc, st := tamperService(t, "", &fakeHostSSH{})
	first, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Guarded", Repo: "rest:" + guarded.URL, Enabled: true, Immutable: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Open", Repo: "rest:" + open.URL, Enabled: true, Immutable: true})
	if err != nil {
		t.Fatal(err)
	}

	v, err := svc.RunTamperTestForTarget(context.Background(), first.ID)
	if err != nil || !v.Testable || !v.Protected {
		t.Fatalf("verdict = %+v, %v, want the guarded target protected", v, err)
	}
	if len(refusing) != 2 || len(accepting) != 0 {
		t.Fatalf("probes: %d at the tested target, %d at the other, want 2 and 0", len(refusing), len(accepting))
	}
	if got, found, err := st.LatestTamperTestForTarget("containers", first.ID); err != nil || !found || !got.Protected {
		t.Errorf("stored verdict = %+v, found=%v err=%v", got, found, err)
	}
	if _, found, _ := st.LatestTamperTestForTarget("containers", second.ID); found {
		t.Error("the target that was not tested has a verdict")
	}
	if run := latestTamperRun(t, st); run.Status != "success" || run.TargetID != "containers" || run.FinishedAt == nil {
		t.Errorf("tamper run = %+v, want a finished success under the domain", run)
	}

	if v, err = svc.RunTamperTestForTarget(context.Background(), second.ID); err != nil || !v.Testable || v.Protected || v.Detail == "" {
		t.Fatalf("verdict = %+v, %v, want the open target unprotected with a reason", v, err)
	}
	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(runs, func(r store.Run) bool { return r.Kind == "tamper" && r.Status == "failed" }) {
		t.Errorf("runs = %+v, want a failed tamper run for the open target", runs)
	}
}

func TestATargetsTamperTestShowsOnItsLocation(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(deleteRecorder(http.StatusForbidden, &seen))
	defer srv.Close()

	svc, st := tamperService(t, "", &fakeHostSSH{})
	dest, derived := mustDestination(t, st, store.OffsiteTarget{Name: "Box", Repo: "rest:" + srv.URL, Immutable: true}, "containers", "vms")
	if _, err := svc.RunTamperTestForTarget(context.Background(), derived[1].ID); err != nil {
		t.Fatal(err)
	}

	loc := locationByID(t, svc, "destination:"+dest.ID)
	if got := sectionFor(t, loc, "vms", useCopy).LastTamper; got == nil || !got.Protected || got.At == 0 {
		t.Errorf("vms = %+v, want the test that just ran", got)
	}
	if got := sectionFor(t, loc, "containers", useCopy).LastTamper; got != nil {
		t.Errorf("containers = %+v, want no test", got)
	}
	if got := loc.Protection.LastTamper; got == nil || !got.Protected {
		t.Errorf("location = %+v, want the one test there is", got)
	}
}

func TestATargetIsProbedWithItsOwnCredentials(t *testing.T) {
	var probedAs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, _ := r.BasicAuth()
		probedAs = append(probedAs, user)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	svc, st := tamperService(t, "", &fakeHostSSH{})
	if err := svc.SetCloudCredSets([]CloudCredSet{
		{ID: "garage", Name: "Garage", CloudCreds: CloudCreds{RESTUser: "named", RESTPassword: "namedpw"}},
	}); err != nil {
		t.Fatal(err)
	}
	target, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "Garage", Repo: "rest:" + srv.URL, Enabled: true, CredsRef: "garage"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RunTamperTestForTarget(context.Background(), target.ID); err != nil {
		t.Fatal(err)
	}
	if len(probedAs) != 2 || probedAs[0] != "named" || probedAs[1] != "named" {
		t.Fatalf("probes signed in as %v, want the target's own login twice", probedAs)
	}
}

func TestATamperTestOfATargetThatCannotBeProbedRecordsNothing(t *testing.T) {
	svc, st := tamperService(t, "", &fakeHostSSH{})
	target, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "flash", Name: "Bucket", Repo: "s3:https://s3.example.com/bucket", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{store: st, svc: svc}
	call := func(id string) (*httptest.ResponseRecorder, map[string]any) {
		req := jsonReq(http.MethodPost, "/api/offsite/targets/"+id+"/tamper-test", nil)
		req.SetPathValue("id", id)
		rec := httptest.NewRecorder()
		h.handleTamperTestOffsiteTarget(rec, req)
		return rec, decodeEnvelope(t, rec)
	}

	if _, env := call(target.ID); env["ok"] != true || env["testable"] != false {
		t.Fatalf("answer for an S3 target = %v, want it not testable", env)
	}
	if _, found, _ := st.LatestTamperTestForTarget("flash", target.ID); found {
		t.Error("a verdict was recorded for a target nothing could probe")
	}
	if run := latestTamperRun(t, st); run.Status != statusSkipped {
		t.Errorf("tamper run = %+v, want it skipped", run)
	}
	if rec, env := call("0000"); rec.Code != http.StatusNotFound || env["ok"] != false {
		t.Errorf("answer for an unknown target = %d %v, want 404", rec.Code, env)
	}
}
