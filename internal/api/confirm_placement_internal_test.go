package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConfirmPlacementResumesAPausedDomain(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))

	// The first pass finds unreplicated history and pauses.
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if !pausedDefault(t, f, "containers") {
		t.Fatal("setup: the domain did not pause")
	}

	if res := f.do(http.MethodPost, "/api/placement/containers/confirm", map[string]any{}); res["ok"] != true {
		t.Fatalf("confirm = %v, want ok", res)
	}
	if pausedDefault(t, f, "containers") {
		t.Fatal("the domain is still paused after confirm")
	}
	f.eng.copies = nil
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if len(f.eng.copies) != 1 {
		t.Fatalf("copies = %+v, want one copy after the confirmation", f.eng.copies)
	}
}

func TestConfirmingAHealthyDomainDoesNotSilenceALaterFirstListingPause(t *testing.T) {
	f := newPlacementFixture(t)
	if res := f.do(http.MethodPost, "/api/placement/containers/confirm", map[string]any{}); res["ok"] != true {
		t.Fatalf("confirm = %v, want ok", res)
	}

	b2 := f.target("containers", "B2", "b2:bucket:containers")
	now := time.Now().Unix()
	f.hold(f.domainPath("containers"), snap("a9", now, "container:nginx"))
	f.hold("b2:bucket:containers", copied("b1", "a1", now-86400, "container:nginx"))

	for range 2 {
		if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
			t.Fatal(err)
		}
	}
	if !pausedDefault(t, f, "containers") {
		t.Fatal("confirming a domain that was never paused silenced its later first-listing pause")
	}
	if _, listed, err := f.st.TargetObservationFor("containers", b2.ID); err != nil || !listed {
		t.Fatalf("the listing that found it was not recorded (listed=%v err=%v)", listed, err)
	}
}

func TestConfirmPlacementRefusesAnUnknownDomain(t *testing.T) {
	f := newPlacementFixture(t)
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, jsonReq(http.MethodPost, "/api/placement/flash/confirm", strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("confirm flash = %d, want 400", rec.Code)
	}
}

func TestConfirmPlacementAcceptsAnEmptyBody(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if !pausedDefault(t, f, "containers") {
		t.Fatal("setup: the domain did not pause")
	}

	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, jsonReq(http.MethodPost, "/api/placement/containers/confirm", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm with no body = %d: %s", rec.Code, rec.Body.String())
	}
	if pausedDefault(t, f, "containers") {
		t.Fatal("the domain is still paused after a bodyless confirm")
	}
}

func TestConfirmPlacementIsIdempotentOnADomainNeverPaused(t *testing.T) {
	f := newPlacementFixture(t)
	if res := f.do(http.MethodPost, "/api/placement/containers/confirm", map[string]any{}); res["ok"] != true {
		t.Fatalf("confirm = %v, want ok", res)
	}
	if pausedDefault(t, f, "containers") {
		t.Fatal("confirming an unpaused domain paused it")
	}
	if res := f.do(http.MethodPost, "/api/placement/containers/confirm", map[string]any{}); res["ok"] != true {
		t.Fatalf("second confirm = %v, want ok", res)
	}
}
