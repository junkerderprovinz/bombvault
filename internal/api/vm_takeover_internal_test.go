package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// Takeover and unlink both hold the VMs domain lock for the whole call, so no
// backup can land under either name between a check and the rename.
func TestVMTakeoverAndUnlinkRefuseABusyVMsDomainBeforeAskingAnything(t *testing.T) {
	st := newVMTagsTestStore(t)
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-10"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "windows-11", tg.ID, linkedAt2024); err != nil {
		t.Fatal(err)
	}
	v := &countingVMTagsVirsh{listOut: []virshcli.VMInfo{{Name: "win10"}}}
	dir := t.TempDir()
	svc := NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}, st, nil, v, nil)
	unlock, ok := svc.tryLockDomainFor("vms", "backup")
	if !ok {
		t.Fatal("the VMs domain lock must be free at the start")
	}
	defer unlock()

	if err := svc.TakeOverVM(context.Background(), "windows-10", "win10"); !errors.Is(err, errDomainBusy) {
		t.Fatalf("TakeOverVM = %v, want the domain-busy refusal", err)
	}
	if err := svc.UnlinkVMAlias(context.Background(), "windows-11"); !errors.Is(err, errDomainBusy) {
		t.Fatalf("UnlinkVMAlias = %v, want the domain-busy refusal", err)
	}
	if v.listCalls != 0 {
		t.Fatalf("virsh was asked %d times while the domain was busy", v.listCalls)
	}
	if got, err := st.GetVMTargetByName("win11"); err != nil || got.ID != tg.ID {
		t.Fatalf("entry on win11 = %+v, %v; want it untouched", got, err)
	}
}
