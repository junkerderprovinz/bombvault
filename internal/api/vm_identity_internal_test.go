package api

import (
	"context"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// countingVMTagsVirsh counts List calls, so a test can prove vmIdentity never
// reaches virsh.
type countingVMTagsVirsh struct {
	listOut   []virshcli.VMInfo
	listCalls int
}

var _ virshcli.Virsh = (*countingVMTagsVirsh)(nil)

func (v *countingVMTagsVirsh) List(context.Context) ([]virshcli.VMInfo, error) {
	v.listCalls++
	return v.listOut, nil
}
func (v *countingVMTagsVirsh) State(context.Context, string) (string, error) { return "", nil }
func (v *countingVMTagsVirsh) DumpXML(context.Context, string) (string, error) {
	return "<domain/>", nil
}
func (v *countingVMTagsVirsh) DumpXMLInactive(context.Context, string) (string, error) {
	return "<domain/>", nil
}
func (v *countingVMTagsVirsh) Shutdown(context.Context, string) error         { return nil }
func (v *countingVMTagsVirsh) Destroy(context.Context, string) error          { return nil }
func (v *countingVMTagsVirsh) Start(context.Context, string) error            { return nil }
func (v *countingVMTagsVirsh) Define(context.Context, string) error           { return nil }
func (v *countingVMTagsVirsh) Undefine(context.Context, string) error         { return nil }
func (v *countingVMTagsVirsh) Autostart(context.Context, string, bool) error  { return nil }
func (v *countingVMTagsVirsh) IsActive(context.Context, string) (bool, error) { return false, nil }
func (v *countingVMTagsVirsh) SnapshotCreateDiskOnly(context.Context, string, string, bool, []string) error {
	return nil
}
func (v *countingVMTagsVirsh) BlockCommitActivePivot(context.Context, string, string) error {
	return nil
}
func (v *countingVMTagsVirsh) GuestAgentPing(context.Context, string) bool { return false }

// newVMTagsTestStore opens an in-memory store, migrated, with the VMs domain
// enabled, so a test can show that even an enabled domain with a live old
// name makes vmIdentity ask virsh nothing.
func newVMTagsTestStore(t *testing.T) *store.Repo {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.VMsEnabled = true
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	return st
}

// A VM with no aliases lists only its own tag.
func TestVMIdentityNoAliasIsOwnTagOnly(t *testing.T) {
	st := newVMTagsTestStore(t)
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
		t.Fatal(err)
	}
	s := &Service{store: st, virsh: &countingVMTagsVirsh{}}

	got := s.vmIdentity("win11").listTags()
	if len(got) != 1 || got[0] != "vm:win11" {
		t.Fatalf("listTags = %v, want exactly [%q]", got, "vm:win11")
	}
}

// The alias carries its linked_at, and nothing asks virsh whether the old
// name is a live domain again. Liveness is the wrong guard: it lets another
// machine's snapshots into retention once the reused name is removed, virsh
// fails, or the VMs domain is switched off.
func TestVMIdentityCarriesLinkTimeAndNeverAsksVirsh(t *testing.T) {
	st := newVMTagsTestStore(t)
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "windows-11", tg.ID, linkedAt2024); err != nil {
		t.Fatal(err)
	}
	v := &countingVMTagsVirsh{listOut: []virshcli.VMInfo{{Name: "windows-11", State: "running"}}}
	s := &Service{store: st, virsh: v}

	id := s.vmIdentity("win11")
	if id.tag != "vm:win11" || len(id.aliases) != 1 || id.aliases[0].tag != "vm:windows-11" ||
		!id.aliases[0].linkedAt.Equal(time.Unix(linkedAt2024, 0)) {
		t.Fatalf("vmIdentity = %+v, want vm:win11 with vm:windows-11 linked at %d", id, linkedAt2024)
	}
	if v.listCalls != 0 {
		t.Fatalf("vmIdentity must not call virsh, got %d List calls", v.listCalls)
	}
}
