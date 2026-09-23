package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/platform"
)

// With no target and an empty restore folder setting, foreign restores fall
// back to the platform's default folder. The other foreign-restore tests
// inject destBase and never reach this path.
func TestForeignDestBaseFallsBackToPlatformDefault(t *testing.T) {
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
		plat platform.Platform // nil means the default, Unraid
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
