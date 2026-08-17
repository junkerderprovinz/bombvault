package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/platform"
)

// TestForeignDestBaseDefaultsThroughPlatformSeam exercises
// foreignContainerDestBase/foreignVMDestBase through their REAL call chain —
// an empty target AND an explicitly emptied settings.RestoreFolder (the
// migrated default is the non-empty "user/bombvault/restore", so reaching
// the platform fallback for real requires clearing it, exactly as an
// operator would via the restore-folder setting) — not an injected destBase,
// which is how every other foreign-restore test reaches these functions.
// Proves the platform.Platform seam these two functions were rewired onto is
// actually live at the exact spot those tests bypass. Covers both the
// nil-platform default (must reproduce Unraid{}'s historical literal via
// platformFn) and an explicitly injected Generic{} (the identity default),
// so a future wiring bug in either direction — falling through to the wrong
// platform, or not consulting platformFn() at all — would fail here.
func TestForeignDestBaseDefaultsThroughPlatformSeam(t *testing.T) {
	s := newForeignTestService(t, nil)
	s.cfg.HostMountRoot = "/host/user"

	settings, err := s.store.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	settings.RestoreFolder = ""
	if err := s.store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	cases := []struct {
		name string
		plat platform.Platform // nil leaves s.platform unset (default Unraid{})
		call func(target string) (string, error)
		want string
	}{
		{"container/nil-platform-defaults-to-Unraid", nil, s.foreignContainerDestBase, "/host/user/user/appdata"},
		{"container/Generic-is-identity", platform.Generic{}, s.foreignContainerDestBase, "/host/user"},
		{"vm/nil-platform-defaults-to-Unraid", nil, s.foreignVMDestBase, "/host/user/user/domains"},
		{"vm/Generic-is-identity", platform.Generic{}, s.foreignVMDestBase, "/host/user"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s.SetPlatform(tc.plat)
			t.Cleanup(func() { s.SetPlatform(nil) })
			got, err := tc.call("")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("= %q, want %q", got, tc.want)
			}
		})
	}
}
