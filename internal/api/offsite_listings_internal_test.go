package api

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAPassListsATargetOnceAndOnceMoreAfterItsKeepPolicy(t *testing.T) {
	for _, c := range []struct {
		name     string
		keepLast int
		want     int
	}{
		{"without a keep-policy", 0, 1},
		{"with a keep-policy", 3, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newPlacementFixture(t)
			keepLast(t, f, f.target("containers", "B2", "b2:bucket:containers"), c.keepLast)
			nas := f.namedRepo("NAS", "nas")
			f.container("nginx", nas.ID)
			f.replicated("containers")
			f.hold(f.domainPath("containers"), snap("a1", 100, "container:plex"))
			f.hold(f.root+"/nas", snap("n1", 100, "container:nginx"))

			if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
				t.Fatal(err)
			}
			if n := f.eng.lists["b2:bucket:containers"]; n != c.want {
				t.Fatalf("B2 was listed %d times, want %d", n, c.want)
			}
		})
	}
}

func TestAnIdlePassListsEachPlaceOnce(t *testing.T) {
	f := newPlacementFixture(t)
	photosOnTheNAS(t, f)
	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatal(err)
	}
	if n := f.eng.lists["b2:bucket:files"]; n != 2 {
		t.Fatalf("the aging pass listed B2 %d times, want 2: before and after the keep-policy", n)
	}
	f.eng.lists = map[string]int{}
	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatal(err)
	}
	if b2, nas := f.eng.lists["b2:bucket:files"], f.eng.lists[f.root+"/nas"]; b2 != 1 || nas != 1 {
		t.Fatalf("the idle pass listed B2 %d and the NAS %d times, want once each", b2, nas)
	}
}

func TestARemoteDomainPathUnderRulesIsListedOnce(t *testing.T) {
	f := newPlacementFixture(t)
	settings := settingsOf(t, f.svc)
	settings.ContainersPath = "s3:https://s3.example/primary"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.target("containers", "B2", "b2:bucket:containers")
	f.replicated("containers")
	f.rule("containers", "container:plex", store.SkipAll)
	f.hold("s3:https://s3.example/primary", snap("a1", 100, "container:nginx"), snap("p1", 100, "container:plex"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if b2, s3 := f.eng.lists["b2:bucket:containers"], f.eng.lists["s3:https://s3.example/primary"]; b2 != 1 || s3 != 1 {
		t.Fatalf("B2 listed %d, the domain path %d times, want once each", b2, s3)
	}
}
