package api

import (
	"context"
	"strings"
	"testing"
)

// A zvol cannot be copied into a folder, so the refusal says so whichever
// backup was picked, rather than sending the user to look for another one.
func TestABesideRestoreOfAZvolVMIsRefusedWhicheverBackupIsPicked(t *testing.T) {
	s := vmRestoreSvc(t, &foreignRecordingEngine{})
	tg := vmTargetJSON(t, "zvolvm", vmZvolDomainXML, []string{"/host/user/pool/domains/zvolvm/zvolvm.qcow2"}, "")
	if _, err := s.store.UpsertVMTarget(tg); err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range []string{"latest", "deadbeef12345678"} {
		_, err := s.prepareRestoreVMBeside(context.Background(), "zvolvm", snapshot, "local")
		if !isDestinationRefusal(err) || !strings.Contains(err.Error(), "the ZFS volume tank/vms/zvolvm/disk1") || !strings.HasSuffix(err.Error(), ".") {
			t.Fatalf("%s: err = %v, want the refusal naming the ZFS volume", snapshot, err)
		}
	}
}
