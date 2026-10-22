package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestATargetMovedInTheEditorDropsTheOldPlacesTamperVerdict(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bkt:containers")
	if err := f.st.RecordTamperTestForTarget("containers", b2.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	v := offsiteTargetToView(b2)
	v.Repo = "b2:bkt:containers/"
	if res := f.do("PUT", "/api/offsite/targets/"+b2.ID, v); res["ok"] != true {
		t.Fatalf("answer = %v", res)
	}
	if _, found, _ := f.st.LatestTamperTestForTarget("containers", b2.ID); !found {
		t.Fatal("another spelling of the same place dropped the verdict")
	}
	v.Repo = "b2:fresh:containers"
	if res := f.do("PUT", "/api/offsite/targets/"+b2.ID, v); res["ok"] != true {
		t.Fatalf("answer = %v", res)
	}
	if _, found, _ := f.st.LatestTamperTestForTarget("containers", b2.ID); found {
		t.Fatal("the fresh bucket shows the old place's append-only verdict")
	}
}

func TestAPrimaryMovedInTheSettingsDropsTheOldPlacesTamperVerdict(t *testing.T) {
	s := primaryCredsService(t)
	settle := func(repo string) store.OffsiteTarget {
		t.Helper()
		settings, err := s.store.MutateSettings(func(m *store.Settings) error {
			m.ContainersOffsite = repo
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.syncPrimaryOffsiteTarget("containers", settings); err != nil {
			t.Fatal(err)
		}
		primary, ok, err := s.store.FieldOffsiteTarget("containers")
		if err != nil || !ok {
			t.Fatalf("primary = %+v, %v, %v", primary, ok, err)
		}
		return primary
	}
	before := settle("rest:http://box:8000/containers")
	if err := s.store.RecordTamperTestForTarget("containers", before.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if again := settle("rest:http://box:8000/containers"); again.ID != before.ID {
		t.Fatalf("a save without a change made a new primary %s", again.ID)
	}
	if _, found, _ := s.store.LatestTamperTestForTarget("containers", before.ID); !found {
		t.Fatal("a save that kept the place dropped the verdict")
	}
	after := settle("s3:https://s3.example.com/fresh/containers")
	if after.ID != before.ID {
		t.Fatalf("the primary changed id from %s to %s", before.ID, after.ID)
	}
	if _, found, _ := s.store.LatestTamperTestForTarget("containers", after.ID); found {
		t.Fatal("the new bucket shows the old place's append-only verdict")
	}
}
