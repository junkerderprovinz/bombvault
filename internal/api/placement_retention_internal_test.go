package api

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func localKeepLast(t *testing.T, f *placementFixture, keepLast int) store.Settings {
	t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = keepLast
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	return settings
}

func TestEveryRepositoryAgesByTheLocalPolicy(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	settings := localKeepLast(t, f, 3)
	for _, ref := range []domainRepoRef{ownRef(f.domainPath("containers")), namedRef(f.root+"/nas", nas)} {
		if got := f.svc.retentionPolicyForRef(settings, ref); got != (restic.RetentionPolicy{KeepLast: 3}) {
			t.Errorf("retentionPolicyForRef(%s) = %+v, want the local keep-last 3", ref.Loc, got)
		}
	}
}

func TestAnItemOnANamedRepositoryAgesByTheLocalPolicy(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	settings := localKeepLast(t, f, 3)
	f.svc.applyRetention(context.Background(), f.root+"/nas", settings, restic.Mode{}, "container:nginx", "containers")
	want := []forgetCall{{Repo: f.root + "/nas", Tags: []string{"container:nginx"}, Policy: restic.RetentionPolicy{KeepLast: 3}, Prune: true}}
	if !reflect.DeepEqual(f.eng.forgets, want) {
		t.Fatalf("forgets = %+v, want %+v", f.eng.forgets, want)
	}
}

func TestAManualPruneAgesEveryRepositoryOfTheDomain(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	localKeepLast(t, f, 3)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:plex"))
	f.hold(f.root+"/nas", snap("b1", 100, "container:nginx"))
	if err := f.svc.pruneDomain(context.Background(), "containers", "local", true); err != nil {
		t.Fatalf("pruneDomain: %v", err)
	}
	var aged []string
	for _, c := range f.eng.forgets {
		if c.Policy != (restic.RetentionPolicy{KeepLast: 3}) {
			t.Errorf("%s forgot with %+v, want the local keep-last 3", c.Repo, c.Policy)
		}
		aged = append(aged, c.Repo+" "+strings.Join(c.Tags, ","))
	}
	want := []string{f.domainPath("containers") + " container:plex", f.root + "/nas container:nginx"}
	if slices.Sort(aged); !slices.Equal(aged, want) {
		t.Fatalf("aged %v, want %v", aged, want)
	}
	if len(f.eng.prunes) != 2 {
		t.Fatalf("prunes = %v, want both repositories", f.eng.prunes)
	}
}

func TestTheBatchedPruneRunsOnlyWhenARepositoryAges(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	f.svc.PruneAfterBulk(context.Background(), "containers")
	if len(f.eng.prunes) != 0 {
		t.Fatalf("without a policy the batched prune ran on %v", f.eng.prunes)
	}
	localKeepLast(t, f, 3)
	f.svc.PruneAfterBulk(context.Background(), "containers")
	if len(f.eng.prunes) != 2 {
		t.Fatalf("with a policy the batched prune reached %v, want both repositories", f.eng.prunes)
	}
}
