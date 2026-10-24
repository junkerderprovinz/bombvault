package store_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAContainerWithoutAPolicyGetsTheDefault(t *testing.T) {
	r := newRepo(t)
	p, err := r.DeferPolicyOf("t1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, store.DefaultDeferPolicy("t1")) {
		t.Fatalf("policy = %+v", p)
	}
	if p.RetryMin != 30 || p.MaxHours != 12 || p.AtDeadline != store.AtDeadlineGiveUp || !p.FailOpen {
		t.Fatalf("the default is %+v", p)
	}
}

func TestAPolicyIsStoredWhole(t *testing.T) {
	r := newRepo(t)
	p := store.DeferPolicy{
		TargetID: "t1", RetryMin: 15, WindowFrom: "22:00", WindowTo: "06:00", MaxHours: 20,
		AtDeadline: store.AtDeadlineBackUp, Preset: "jellyfin", APIKeyEnc: []byte{1, 2, 3},
		PausedMin: 45, NetKiB: 300, NetMin: 10,
	}
	if err := r.SetDeferPolicy(p); err != nil {
		t.Fatal(err)
	}
	got, err := r.DeferPolicyOf("t1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.APIKeyEnc, p.APIKeyEnc) {
		t.Fatalf("key = %v", got.APIKeyEnc)
	}
	got.APIKeyEnc, p.APIKeyEnc = nil, nil
	if !reflect.DeepEqual(got, p) {
		t.Fatalf("got %+v, want %+v", got, p)
	}
	all, err := r.DeferPolicies()
	if err != nil || len(all) != 1 || all["t1"].Preset != "jellyfin" {
		t.Fatalf("all = %+v, %v", all, err)
	}
}

func TestAWaitingBackupIsKeptUntilItIsForgotten(t *testing.T) {
	r := newRepo(t)
	d := store.Deferral{TargetID: "t1", Container: "plex", Since: 100, NextTry: 1900, Tries: 1, Reason: "busy"}
	if err := r.SaveDeferral(d); err != nil {
		t.Fatal(err)
	}
	d.Tries, d.NextTry = 2, 3700
	if err := r.SaveDeferral(d); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.GetDeferral("t1")
	if err != nil || !ok || got != d {
		t.Fatalf("got %+v, %v, %v", got, ok, err)
	}
	list, err := r.ListDeferrals()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if err := r.DeleteDeferral("t1"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.GetDeferral("t1"); ok {
		t.Fatal("a forgotten wait is still there")
	}
}

func TestADeferredRunCarriesItsNextTry(t *testing.T) {
	r := newRepo(t)
	tg, err := r.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(id, "deferred", "", 0, "busy"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetRunRetryAt(id, 12345); err != nil {
		t.Fatal(err)
	}
	run, err := r.LastRunForTarget(tg.ID)
	if err != nil || run == nil || run.RetryAt == nil || *run.RetryAt != 12345 {
		t.Fatalf("run = %+v, %v", run, err)
	}
}
