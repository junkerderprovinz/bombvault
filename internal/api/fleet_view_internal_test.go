package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAMemberNeverPolledHasAnEmptyScorecard(t *testing.T) {
	b, err := json.Marshal(fleetPeerToView(store.FleetPeer{MemberID: "m"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"lastPollDomains":[]`) {
		t.Fatalf("view %s, want an empty scorecard list", b)
	}
}
