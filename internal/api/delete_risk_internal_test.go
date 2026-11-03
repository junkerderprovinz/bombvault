package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTheStatusSaysWhenTheCheckOfDeleteProtectionIsOff(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "OneDrive", "rclone:onedrive:bv/containers")
	f.settings(func(s *store.Settings) { s.TamperTestSchedule = "daily 04:00" })
	if f.domainStatus("containers").TamperOff {
		t.Error("a scheduled check reads as off")
	}
	f.settings(func(s *store.Settings) { s.TamperTestSchedule = "off" })
	if !f.domainStatus("containers").TamperOff {
		t.Error("a check switched off reads as on")
	}
}
