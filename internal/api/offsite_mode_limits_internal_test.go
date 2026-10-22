package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Every restic run at a target, its keep-policy's forget and prune included,
// goes through this mode, so it carries the target's caps.
func TestATargetsModeCarriesItsBandwidthCaps(t *testing.T) {
	f := newPlacementFixture(t)
	target := store.OffsiteTarget{Domain: "containers", Repo: "b2:bucket:containers", LimitUpload: 500, LimitDownload: 800}

	mode := f.svc.offsiteModeForTarget(settingsOf(t, f.svc), target)

	if want := (restic.Limits{UploadKBps: 500, DownloadKBps: 800}); mode.Limits != want {
		t.Fatalf("limits = %+v, want %+v", mode.Limits, want)
	}
}
