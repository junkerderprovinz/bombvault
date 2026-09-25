package api

import (
	"strings"
	"testing"
)

func TestThePauseNoticeSendsTheReaderToTheDomainsRow(t *testing.T) {
	for reason, why := range pauseReasons {
		msg := placementPausedMessage("vms", reason)
		if !strings.Contains(msg, why) || !strings.Contains(msg, "the row of vms under Settings > Storage > Domains") {
			t.Errorf("pause notice for %s = %q", reason, msg)
		}
	}
}

func TestAPathOntoANamedRepositoryIsRefusedWithWhereItIsSetUp(t *testing.T) {
	f := newPlacementFixture(t)
	f.namedRepo("Old NAS", "oldnas")
	cur, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(cur)
	v.ContainersPath = "oldnas"
	if msg := f.h.rejectSettingsPathOnNamedRepo(v, cur); !strings.Contains(msg, `"Old NAS", which Settings, Storage lists at its place or under Without a place`) {
		t.Fatalf("refusal = %q", msg)
	}
}
