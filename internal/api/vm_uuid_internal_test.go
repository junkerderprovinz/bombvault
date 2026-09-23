package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// newVMUUIDTestService builds a Service on an in-memory store. vmUUID only
// uses the store, so docker, virsh and the engine stay nil.
func newVMUUIDTestService(t *testing.T) (*Service, *store.Repo) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	svc := NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}, st, nil, nil, nil)
	return svc, st
}

const vmUUIDTestDomainXML = `
<domain type='kvm'>
  <uuid>4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33</uuid>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/cache/vms/Win/vdisk1.img'/>
      <target dev='vda'/>
    </disk>
  </devices>
</domain>`

// A VM backed up by an older release has an empty UUID column although its
// stored DomainXML carries the domain's <uuid>. The lazy backfill resolves it
// and persists it, so rename detection works for that VM before its next
// backup.
func TestVMUUIDBackfillsFromSavedDomainXML(t *testing.T) {
	svc, st := newVMUUIDTestService(t)
	defJSON, err := json.Marshal(vmDefinition{DomainXML: vmUUIDTestDomainXML})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "win10", Method: "graceful", Definition: string(defJSON)}); err != nil {
		t.Fatal(err)
	}
	tg, err := st.GetVMTargetByName("win10")
	if err != nil {
		t.Fatal(err)
	}
	if tg.UUID != "" {
		t.Fatalf("fixture is wrong: UUID must start empty, got %q", tg.UUID)
	}

	got := svc.vmUUID(tg)
	if got != "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33" {
		t.Fatalf("vmUUID = %q", got)
	}

	after, err := st.GetVMTargetByName("win10")
	if err != nil {
		t.Fatal(err)
	}
	if after.UUID != "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33" {
		t.Fatalf("backfill was not persisted: stored UUID = %q", after.UUID)
	}
}

// Once a row's UUID column is set, vmUUID trusts it and does not parse
// DomainXML again. The Definition handed in parses to a different UUID, so a
// re-parse would return and store the wrong one.
func TestVMUUIDSecondCallDoesNotReparse(t *testing.T) {
	svc, st := newVMUUIDTestService(t)
	defJSON, err := json.Marshal(vmDefinition{DomainXML: vmUUIDTestDomainXML})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "win10", Method: "graceful", Definition: string(defJSON)}); err != nil {
		t.Fatal(err)
	}
	tg, err := st.GetVMTargetByName("win10")
	if err != nil {
		t.Fatal(err)
	}
	first := svc.vmUUID(tg)
	if first == "" {
		t.Fatal("fixture: first backfill must succeed")
	}
	tg2, err := st.GetVMTargetByName("win10")
	if err != nil {
		t.Fatal(err)
	}
	if tg2.UUID != first {
		t.Fatalf("fixture: stored UUID must equal the backfilled value, got %q", tg2.UUID)
	}

	// Point the in-memory copy's Definition at a different UUID.
	otherXML := strings.Replace(vmUUIDTestDomainXML, "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33", "ffffffff-ffff-ffff-ffff-ffffffffffff", 1)
	otherDefJSON, err := json.Marshal(vmDefinition{DomainXML: otherXML})
	if err != nil {
		t.Fatal(err)
	}
	tg2.Definition = string(otherDefJSON)

	got := svc.vmUUID(tg2)
	if got != first {
		t.Fatalf("vmUUID re-derived instead of trusting the stored value: got %q, want %q", got, first)
	}
	after, err := st.GetVMTargetByName("win10")
	if err != nil {
		t.Fatal(err)
	}
	if after.UUID != first {
		t.Fatalf("the stored UUID must not have been overwritten: got %q, want %q", after.UUID, first)
	}
}

// A row without a stored Definition, such as a VM never backed up or a target
// created some other way, yields "" and writes nothing.
func TestVMUUIDNoUsableDefinitionStaysEmpty(t *testing.T) {
	svc, st := newVMUUIDTestService(t)
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "fresh", Method: "graceful"}); err != nil {
		t.Fatal(err)
	}
	tg, err := st.GetVMTargetByName("fresh")
	if err != nil {
		t.Fatal(err)
	}

	if got := svc.vmUUID(tg); got != "" {
		t.Fatalf("vmUUID = %q, want empty for a target with no Definition", got)
	}
	after, err := st.GetVMTargetByName("fresh")
	if err != nil {
		t.Fatal(err)
	}
	if after.UUID != "" {
		t.Fatalf("nothing must be written when there is nothing to backfill: got %q", after.UUID)
	}
}

// TestVMUUIDUnparseableDefinitionStaysEmpty covers the other two "no usable
// XML" cases: a domain XML with no <uuid> element, and a Definition that is
// not valid JSON at all. Neither may reach the caller as an error or write
// anything to the store.
func TestVMUUIDUnparseableDefinitionStaysEmpty(t *testing.T) {
	svc, st := newVMUUIDTestService(t)

	const noUUIDXML = `<domain type='kvm'><devices></devices></domain>`
	defJSON, err := json.Marshal(vmDefinition{DomainXML: noUUIDXML})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "no-uuid-xml", Method: "graceful", Definition: string(defJSON)}); err != nil {
		t.Fatal(err)
	}
	tg, err := st.GetVMTargetByName("no-uuid-xml")
	if err != nil {
		t.Fatal(err)
	}
	if got := svc.vmUUID(tg); got != "" {
		t.Fatalf("vmUUID = %q, want empty for a domain XML with no <uuid>", got)
	}

	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "garbage-json", Method: "graceful", Definition: "not json"}); err != nil {
		t.Fatal(err)
	}
	tg2, err := st.GetVMTargetByName("garbage-json")
	if err != nil {
		t.Fatal(err)
	}
	if got := svc.vmUUID(tg2); got != "" {
		t.Fatalf("vmUUID = %q, want empty for an unparseable Definition", got)
	}

	for _, name := range []string{"no-uuid-xml", "garbage-json"} {
		after, err := st.GetVMTargetByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if after.UUID != "" {
			t.Fatalf("%s: nothing must be written, got %q", name, after.UUID)
		}
	}
}
