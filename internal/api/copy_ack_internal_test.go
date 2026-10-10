package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// copyRanAt records a copy to a target that began at at and ended as ok says.
func copyRanAt(t *testing.T, st *store.Repo, target store.OffsiteTarget, at int64, ok bool) {
	t.Helper()
	id, err := st.RecordOffsiteRunForTarget(target.Domain, target.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishOffsiteRun(id, ok, ""); err != nil {
		t.Fatal(err)
	}
}

func ack(t *testing.T, h *Handler, body map[string]any) (int, map[string]any) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.handleAckRuns(rec, jsonReq(http.MethodPost, "/api/runs/ack", bytes.NewReader(b)))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	return rec.Code, out
}

func TestAFailedCopyIsAcknowledgedUntilTheNextOneFails(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	h := &Handler{store: st, svc: svc}
	dest, derived := mustDestination(t, st, store.OffsiteTarget{Name: "Box", Repo: "rclone:box:bv"}, "containers", "vms")
	copyRanAt(t, st, derived[0], 1000, true)
	copyRanAt(t, st, derived[0], 2000, false)
	copyRanAt(t, st, derived[0], 3000, false)
	copyRanAt(t, st, derived[1], 2500, false)
	lastCopy := func(domain string) *copyState {
		return sectionFor(t, locationByID(t, svc, "destination:"+dest.ID), domain, useCopy).LastCopy
	}
	if got := lastCopy("containers"); got == nil || got.Acknowledged {
		t.Fatalf("before the acknowledgement: %+v", got)
	}

	if code, res := ack(t, h, map[string]any{"copyTargets": []string{derived[0].ID}}); code != http.StatusOK || res["count"] != float64(2) {
		t.Fatalf("answer = %d %v, want both failed copies counted", code, res)
	}

	if got := lastCopy("containers"); got == nil || !got.Acknowledged || got.OK || got.FailingSince != 2000 {
		t.Fatalf("containers = %+v, want it still failing since 2000 and acknowledged", got)
	}
	if got := lastCopy("vms"); got == nil || got.Acknowledged {
		t.Fatalf("vms = %+v, want the copy to another target left as it was", got)
	}

	copyRanAt(t, st, derived[0], 4000, false)
	if got := lastCopy("containers"); got == nil || got.Acknowledged {
		t.Fatalf("after another failure: %+v, want it to count again", got)
	}
}

func TestAcknowledgingEverythingTakesTheFailedCopiesAlong(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	h := &Handler{store: st, svc: svc}
	dest, derived := mustDestination(t, st, store.OffsiteTarget{Name: "Box", Repo: "rclone:box:bv"}, "containers", "vms")
	copyRanAt(t, st, derived[0], 2000, false)
	copyRanAt(t, st, derived[1], 2500, false)
	copyRanAt(t, st, derived[1], 3500, true)

	if code, res := ack(t, h, map[string]any{"all": true}); code != http.StatusOK || res["count"] != float64(2) {
		t.Fatalf("answer = %d %v, want the two failed copies counted", code, res)
	}

	loc := locationByID(t, svc, "destination:"+dest.ID)
	if got := sectionFor(t, loc, "containers", useCopy).LastCopy; got == nil || !got.Acknowledged {
		t.Fatalf("containers = %+v, want its failed copy acknowledged", got)
	}
	if _, res := ack(t, h, map[string]any{"all": true}); res["count"] != float64(0) {
		t.Fatalf("a second acknowledgement changed %v rows", res["count"])
	}
}

func TestAnAcknowledgementRefusesATargetIDThatIsNone(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	h := &Handler{store: st, svc: svc}

	if code, _ := ack(t, h, map[string]any{"copyTargets": []string{"1 OR 1=1"}}); code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
}
