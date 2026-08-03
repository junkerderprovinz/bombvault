package api

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestBackupHardCap pins the configurable backup timeout (BACKUP_MAX_HOURS):
// a raised 48h default, an explicit N hours, the 0=unlimited sentinel, and an
// invalid value falling back to the default.
func TestBackupHardCap(t *testing.T) {
	cases := []struct {
		name string
		env  string
		set  bool
		want time.Duration
	}{
		{"unset defaults to 48h", "", false, 48 * time.Hour},
		{"empty defaults to 48h", "", true, 48 * time.Hour},
		{"whitespace defaults to 48h", "   ", true, 48 * time.Hour},
		{"explicit 6 hours", "6", true, 6 * time.Hour},
		{"explicit 72 hours", "72", true, 72 * time.Hour},
		{"zero disables the cap", "0", true, 0},
		{"negative falls back to 48h", "-3", true, 48 * time.Hour},
		{"garbage falls back to 48h", "banana", true, 48 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv("BACKUP_MAX_HOURS", tc.env)
			} else {
				// Register restoration via t.Setenv, then fully remove it to
				// exercise the "unset" branch distinctly from "empty".
				t.Setenv("BACKUP_MAX_HOURS", "")
				if err := os.Unsetenv("BACKUP_MAX_HOURS"); err != nil {
					t.Fatal(err)
				}
			}
			if got := backupHardCap(); got != tc.want {
				t.Fatalf("backupHardCap()=%v, want %v", got, tc.want)
			}
		})
	}
}

// TestBackupHoldCtxCap verifies the hold context has a deadline for a finite cap
// and none for the unlimited (0) cap, while always being cancellable.
func TestBackupHoldCtxCap(t *testing.T) {
	t.Run("finite cap sets a deadline", func(t *testing.T) {
		t.Setenv("BACKUP_MAX_HOURS", "24")
		ctx, cancel := backupHoldCtx(context.Background())
		defer cancel()
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("finite cap must set a context deadline")
		}
	})
	t.Run("unlimited cap sets no deadline", func(t *testing.T) {
		t.Setenv("BACKUP_MAX_HOURS", "0")
		ctx, cancel := backupHoldCtx(context.Background())
		defer cancel()
		if d, ok := ctx.Deadline(); ok {
			t.Fatalf("unlimited cap must not set a deadline, got %v", d)
		}
	})
}

// TestDrillWaitCap ties the scheduled-drill wait to the backup cap and bounds the
// unlimited case at a safe (non-overflowing) 100 years.
func TestDrillWaitCap(t *testing.T) {
	t.Setenv("BACKUP_MAX_HOURS", "10")
	if got := drillWaitCap(); got != 10*time.Hour {
		t.Fatalf("drillWaitCap()=%v, want 10h", got)
	}
	t.Setenv("BACKUP_MAX_HOURS", "0")
	got := drillWaitCap()
	if got != 100*365*24*time.Hour {
		t.Fatalf("unlimited drillWaitCap()=%v, want 100y", got)
	}
	// Must not overflow time.Time.Add into the past (the whole point of the bound).
	if !time.Now().Add(got).After(time.Now()) {
		t.Fatal("drillWaitCap for unlimited overflowed time.Time.Add")
	}
}
