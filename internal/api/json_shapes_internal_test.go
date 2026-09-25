package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The page reads these fields with .length and [key] and never checks for
// null, so an empty value has to reach it as [] or {}.

func marshalled(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func wantInJSON(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("want %s in %s", want, got)
	}
}

func TestSummaryListsNoUnmeasuredVolumeAsAnEmptyArray(t *testing.T) {
	var none *anomalyEngine
	wantInJSON(t, marshalled(t, none.summary()), `"unmeasuredVolumes":[]`)

	f := newEngineFixture(t)
	wantInJSON(t, marshalled(t, f.e.summary()), `"unmeasuredVolumes":[]`)

	f.pass(t)
	wantInJSON(t, marshalled(t, f.e.summary()), `"unmeasuredVolumes":[]`)
}

func TestItemsTabWithNothingWatchedServesAnEmptyList(t *testing.T) {
	var none *anomalyEngine
	wantInJSON(t, marshalled(t, map[string]any{"items": none.itemViews()}), `"items":[]`)

	f := newEngineFixture(t)
	wantInJSON(t, marshalled(t, map[string]any{"items": f.e.itemViews()}), `"items":[]`)

	f.pass(t)
	wantInJSON(t, marshalled(t, map[string]any{"items": f.e.itemViews()}), `"items":[]`)
}

func TestFindingWithoutDetailsServesAnEmptyObject(t *testing.T) {
	view := anomalyViewOf(store.Anomaly{ID: "a1", Metric: "failure_streak"}, nil, nil, nil, store.Settings{})
	wantInJSON(t, marshalled(t, view), `"details":{}`)
}

func TestZFSRunWithoutMembersServesAnEmptyList(t *testing.T) {
	s := &Service{store: newTestStore(t)}
	detail, err := s.ZFSRunDetail(context.Background(), "no-such-run")
	if err != nil {
		t.Fatal(err)
	}
	wantInJSON(t, marshalled(t, detail), `"members":[]`)
}

func TestMissingPropagationWithoutNestedDatasetsNamesAnEmptyList(t *testing.T) {
	const unpropagated = `
30 1 0:28 / / rw,relatime - overlay overlay rw
40 30 0:29 /mnt /host/user rw,relatime - rootfs rootfs rw
`
	s, _, host := zfsProbeFixture(t, config.Config{LibvirtHost: "tower", LibvirtSSHUser: "root", LibvirtSSHPort: "22"})
	zfsMountFixture(t, zfsTestRecords(t, unpropagated), host, nil)

	res := s.ZFSConnectionTest(context.Background())
	if res.Propagation != "propagation-missing" {
		t.Fatalf("propagation = %q, want propagation-missing", res.Propagation)
	}
	wantInJSON(t, marshalled(t, res), `"unpropagated":[]`)
}
