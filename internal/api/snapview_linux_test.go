//go:build linux

package api

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestEnteringAViewWithoutTheChrootCapabilityIsRefusedAsEPERM(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root holds SYS_CHROOT")
	}
	if err := enterView(t.TempDir()); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("enterView = %v, want an error that carries EPERM", err)
	}
}
