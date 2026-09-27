package api_test

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Naming an additional target's repository in a domain's off-site field makes
// that target the primary, which then follows the off-site settings. A target
// with settings of its own would lose them without a word, and an append-only
// one would start pruning, so the save is refused.
func TestSettingsSaveRefusesAdoptingATargetWithItsOwnSettings(t *testing.T) {
	h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	extra, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Vault", Repo: "s3:vault", Immutable: true,
		RetentionKeepMonthly: 24, Enabled: true, SortOrder: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	body := `{"containersPath": "backups/c", "containersSchedule": "daily 02:30", "containersOffsite": "s3:vault"}`
	if _, m := doJSON(t, h, http.MethodPut, "/api/settings", body); m["ok"] == true {
		t.Fatalf("the save took over a target with its own settings: %v", m)
	}

	got, _, err := st.GetOffsiteTarget(extra.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Immutable || got.RetentionKeepMonthly != 24 || got.SortOrder != 1 {
		t.Fatalf("the refused save changed the target: %+v", got)
	}
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.ContainersOffsite != "" {
		t.Fatalf("the refused save stored the off-site repository %q", s.ContainersOffsite)
	}
}

// A target that already matches the off-site settings loses nothing, so it
// becomes the primary instead of a second copy on the same repository.
func TestSettingsSaveAdoptsATargetThatMatchesTheOffsiteSettings(t *testing.T) {
	h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	extra, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Peer", Repo: "s3:peer", Enabled: true, SortOrder: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	body := `{"containersPath": "backups/c", "containersSchedule": "daily 02:30", "containersOffsite": "s3:peer"}`
	if _, m := doJSON(t, h, http.MethodPut, "/api/settings", body); m["ok"] != true {
		t.Fatalf("save refused: %v", m)
	}

	targets, err := st.OffsiteTargetsForDomain("containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].ID != extra.ID || targets[0].SortOrder != 0 {
		t.Fatalf("want the target adopted as primary, got %+v", targets)
	}
}
