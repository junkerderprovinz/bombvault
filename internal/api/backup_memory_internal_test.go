package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func TestBackupMemoryLimitIsSavedClampedAndApplied(t *testing.T) {
	t.Cleanup(func() { restic.SetMemLimitMB(0) })
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)

	for _, c := range []struct {
		name  string
		send  int
		saved int
	}{
		{"a limit", 2048, 2048},
		{"a negative value", -10, 0},
		{"a value past the cap", maxBackupMemoryMB * 4, maxBackupMemoryMB},
		{"zero", 0, 0},
	} {
		v.BackupMemoryMB = c.send
		if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
			t.Fatalf("%s: save failed: %v", c.name, res)
		}
		got, err := f.st.GetSettings()
		if err != nil {
			t.Fatal(err)
		}
		if got.BackupMemoryMB != c.saved {
			t.Errorf("%s: saved %d, want %d", c.name, got.BackupMemoryMB, c.saved)
		}
		if shown := toView(got).BackupMemoryMB; shown != c.saved {
			t.Errorf("%s: shown %d, want %d", c.name, shown, c.saved)
		}
	}

	v.BackupMemoryMB = 768
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("save failed: %v", res)
	}
	if got := restic.MemLimitMB(); got != 768 {
		t.Errorf("restic runs with a %d MiB limit, want 768", got)
	}
}
