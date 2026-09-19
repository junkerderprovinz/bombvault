package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// renameSuggestVirsh serves a fixed VM list, or listErr, and a domain XML or
// error per name. dumpCalls records every name DumpXMLInactive was asked for,
// so a test can prove the gate skipped virsh.
type renameSuggestVirsh struct {
	fakeVirsh
	vms       []virshcli.VMInfo
	listErr   error
	xmlByName map[string]string
	errByName map[string]error
	dumpCalls []string
}

func (v *renameSuggestVirsh) List(context.Context) ([]virshcli.VMInfo, error) {
	return v.vms, v.listErr
}

func (v *renameSuggestVirsh) DumpXMLInactive(_ context.Context, name string) (string, error) {
	v.dumpCalls = append(v.dumpCalls, name)
	if err, ok := v.errByName[name]; ok {
		return "", err
	}
	return v.xmlByName[name], nil
}

// DumpXML serves the same XML as the live definition, for a test that backs
// the VM up.
func (v *renameSuggestVirsh) DumpXML(_ context.Context, name string) (string, error) {
	if err, ok := v.errByName[name]; ok {
		return "", err
	}
	return v.xmlByName[name], nil
}

// vmRenameTestDomainXML builds a domain XML that carries only the given
// <uuid>.
func vmRenameTestDomainXML(uuid string) string {
	return "<domain type='kvm'><uuid>" + uuid + "</uuid></domain>"
}

const testVMUUID = "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33"

// newRenameSuggestTestService wires a Service with the VMs domain enabled.
func newRenameSuggestTestService(t *testing.T, st *store.Repo, v *renameSuggestVirsh) *api.Service {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	s := mustSettings(t, st)
	s.VMsEnabled = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	return api.NewService(cfg, st, &fakeServiceDocker{}, v, &fakeResticEngine{})
}

// vmView finds the row for the given libvirt name, failing the test if it's
// not there.
func vmView(t *testing.T, views []api.VMView, libvirtName string) api.VMView {
	t.Helper()
	for _, v := range views {
		if v.LibvirtName == libvirtName {
			return v
		}
	}
	t.Fatalf("no VMView with LibvirtName %q in %+v", libvirtName, views)
	return api.VMView{}
}

// A live domain with no backups of its own, whose inactive domain XML carries
// the UUID of a not-installed target, gets that entry offered as its rename
// source.
func TestListVMsSuggestsRenameFromLibvirtUUID(t *testing.T) {
	st := newMemStore(t)
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Method: "graceful", UUID: testVMUUID}); err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{
		vms:       []virshcli.VMInfo{{Name: "win11", State: "running"}},
		xmlByName: map[string]string{"win11": vmRenameTestDomainXML(testVMUUID)},
	}
	svc := newRenameSuggestTestService(t, st, v)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	got := vmView(t, views, "win11")
	if got.RenameFrom != "windows-11" {
		t.Fatalf("RenameFrom = %q, want %q", got.RenameFrom, "windows-11")
	}
	if got.RenameReason != "libvirt-uuid" { // reasonLibvirtUUID
		t.Fatalf("RenameReason = %q, want %q", got.RenameReason, "libvirt-uuid")
	}
}

// A live VM with a backup under its own name has its own history and is not a
// rename candidate, even when its UUID matches a not-installed entry (as in
// TestListContainersSuppressesRenameWhenLiveHasOwnRunRecord).
//
// "unrelated" is a second live VM without backups. It keeps the suggestion
// pass's gate open, so the test reaches the per-row filter instead of passing
// because win11 alone closed the gate.
func TestListVMsSuppressesRenameWhenLiveHasOwnBackup(t *testing.T) {
	st := newMemStore(t)
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Method: "graceful", UUID: testVMUUID}); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	runID, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(runID, "success", "deadbeef", 1024, ""); err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{
		vms: []virshcli.VMInfo{
			{Name: "win11", State: "running"},
			{Name: "unrelated", State: "running"}, // no backups: keeps the suggestion pass' gate open
		},
		xmlByName: map[string]string{
			"win11":     vmRenameTestDomainXML(testVMUUID),
			"unrelated": vmRenameTestDomainXML("ffffffff-ffff-ffff-ffff-ffffffffffff"),
		},
	}
	svc := newRenameSuggestTestService(t, st, v)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	got := vmView(t, views, "win11")
	if got.RenameFrom != "" || got.RenameReason != "" {
		t.Fatalf("win11 (has its own backup) must not carry a rename suggestion, got RenameFrom=%q RenameReason=%q", got.RenameFrom, got.RenameReason)
	}
}

// With every live VM already backed up, the gate closes and suggestVMRenames
// returns before touching virsh, which keeps the VM page cheap. dumpCalls
// proves it, not only the missing suggestion.
func TestListVMsRenameSuggestionSkippedWhenAllLiveHaveOwnBackups(t *testing.T) {
	st := newMemStore(t)
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Method: "graceful", UUID: testVMUUID}); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	runID, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(runID, "success", "deadbeef", 1024, ""); err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{
		vms:       []virshcli.VMInfo{{Name: "win11", State: "running"}},
		xmlByName: map[string]string{"win11": vmRenameTestDomainXML(testVMUUID)},
	}
	svc := newRenameSuggestTestService(t, st, v)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	got := vmView(t, views, "win11")
	if got.RenameFrom != "" || got.RenameReason != "" {
		t.Fatalf("win11 must carry no suggestion when every live VM already has backups, got RenameFrom=%q RenameReason=%q", got.RenameFrom, got.RenameReason)
	}
	if len(v.dumpCalls) != 0 {
		t.Fatalf("the gate must skip virsh entirely here, got DumpXMLInactive calls: %v", v.dumpCalls)
	}
}

// win11 has no backups of its own, but there is no not-installed VM target to
// match against, so suggestVMRenames returns before touching virsh.
func TestListVMsRenameSuggestionSkippedWithNoOrphanUUID(t *testing.T) {
	st := newMemStore(t)
	v := &renameSuggestVirsh{
		vms: []virshcli.VMInfo{{Name: "win11", State: "running"}},
	}
	svc := newRenameSuggestTestService(t, st, v)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	got := vmView(t, views, "win11")
	if got.RenameFrom != "" || got.RenameReason != "" {
		t.Fatalf("win11 must carry no suggestion with no not-installed target to match, got RenameFrom=%q RenameReason=%q", got.RenameFrom, got.RenameReason)
	}
	if len(v.dumpCalls) != 0 {
		t.Fatalf("the gate must skip virsh entirely here, got DumpXMLInactive calls: %v", v.dumpCalls)
	}
}

// A DumpXMLInactive error for one live candidate drops that candidate from
// the match and does not fail ListVMs.
func TestListVMsRenameSuggestionSkipsCandidateOnVirshError(t *testing.T) {
	st := newMemStore(t)
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Method: "graceful", UUID: testVMUUID}); err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{
		vms:       []virshcli.VMInfo{{Name: "win11", State: "running"}},
		errByName: map[string]error{"win11": errors.New("ssh: connection reset")},
	}
	svc := newRenameSuggestTestService(t, st, v)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs must not fail on a virsh error for one candidate: %v", err)
	}
	got := vmView(t, views, "win11")
	if got.RenameFrom != "" || got.RenameReason != "" {
		t.Fatalf("win11 must carry no suggestion when its XML dump failed, got RenameFrom=%q RenameReason=%q", got.RenameFrom, got.RenameReason)
	}
}

// A retired VM name that is a live domain again is reported on the row of the
// entry that owns the alias, not on the live domain's own row.
// "retired-alias" is a second alias of the same entry whose old name is not
// live, so the live check has to tell the two apart.
func TestListVMsAliasConflictReportsOwnerRow(t *testing.T) {
	st := newMemStore(t)
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "current-name", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAlias("vm", "old-name", tg.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAlias("vm", "retired-alias", tg.ID); err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{vms: []virshcli.VMInfo{
		{Name: "current-name", State: "running"},
		{Name: "old-name", State: "running"}, // a different domain reusing the retired name
		// "retired-alias" is not live.
	}}
	svc := newRenameSuggestTestService(t, st, v)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	owner := vmView(t, views, "current-name")
	if want := []string{"old-name"}; !reflect.DeepEqual(owner.AliasConflicts, want) {
		t.Fatalf("current-name AliasConflicts = %q, want %q", owner.AliasConflicts, want)
	}
	reused := vmView(t, views, "old-name")
	if len(reused.AliasConflicts) != 0 {
		t.Fatalf("old-name (the domain reusing the retired name) must not itself carry the conflict, got %q", reused.AliasConflicts)
	}
}

// A retired alias whose old name is not a live domain reports no conflict
// anywhere.
func TestListVMsAliasConflictNoneWhenRetiredNameNotLive(t *testing.T) {
	st := newMemStore(t)
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "current-name", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAlias("vm", "retired-alias", tg.ID); err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{vms: []virshcli.VMInfo{
		{Name: "current-name", State: "running"},
	}}
	svc := newRenameSuggestTestService(t, st, v)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	owner := vmView(t, views, "current-name")
	if len(owner.AliasConflicts) != 0 {
		t.Fatalf("AliasConflicts = %q, want none (retired-alias is not live again)", owner.AliasConflicts)
	}
}

// When an entry has more than one alias that is live again, each is reported,
// alphabetically.
func TestListVMsListsEveryLiveFormerNameAsAConflict(t *testing.T) {
	st := newMemStore(t)
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "current-name", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAlias("vm", "zulu-old", tg.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAlias("vm", "alpha-old", tg.ID); err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{vms: []virshcli.VMInfo{
		{Name: "current-name", State: "running"},
		{Name: "zulu-old", State: "running"},
		{Name: "alpha-old", State: "running"},
	}}
	svc := newRenameSuggestTestService(t, st, v)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	owner := vmView(t, views, "current-name")
	if want := []string{"alpha-old", "zulu-old"}; !reflect.DeepEqual(owner.AliasConflicts, want) {
		t.Fatalf("AliasConflicts = %q, want %q", owner.AliasConflicts, want)
	}
}

// blockDiskDomainXML is a domain whose only disk is block-backed, the TrueNAS
// 25.10 zvol shape.
const blockDiskDomainXML = `<domain type='kvm'>
  <devices>
    <disk type='block' device='disk'>
      <source dev='/dev/zvol/tank/vms/windows-11/disk0'/>
      <target dev='vda'/>
    </disk>
  </devices>
</domain>`

// vmDefFixture carries the JSON tag of the unexported vmDefinition's
// DomainXML field, which this external test package cannot reference.
type vmDefFixture struct {
	DomainXML string `json:"domain_xml"`
}

// A not-installed target whose stored definition has a block-device (zvol)
// disk is not offered as a rename source, even when its UUID matches a live
// domain without backups. zvol tags are not aliased, so a takeover would
// carry the VM's history forward but leave its disk's behind. The takeover
// route refuses the same shape.
func TestListVMsRenameSuggestionSkipsBlockDiskCandidate(t *testing.T) {
	st := newMemStore(t)
	def, err := json.Marshal(vmDefFixture{DomainXML: blockDiskDomainXML})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Method: "graceful", UUID: testVMUUID, Definition: string(def)}); err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{
		vms:       []virshcli.VMInfo{{Name: "win11", State: "running"}},
		xmlByName: map[string]string{"win11": vmRenameTestDomainXML(testVMUUID)},
	}
	svc := newRenameSuggestTestService(t, st, v)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	got := vmView(t, views, "win11")
	if got.RenameFrom != "" || got.RenameReason != "" {
		t.Fatalf("win11 must not be offered windows-11 as a rename source (its stored definition has a block disk), got RenameFrom=%q RenameReason=%q", got.RenameFrom, got.RenameReason)
	}
}

// fileBackedDomainXML is a file-backed (vdisk) domain, the shape a backed-up
// VM's stored definition has. The other positive tests in this file leave
// Definition empty for brevity.
const fileBackedDomainXML = `<domain type='kvm'>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/user/domains/windows-11/vdisk1.qcow2'/>
      <target dev='vda'/>
    </disk>
  </devices>
</domain>`

// A not-installed target with a file-backed stored definition and a matching
// UUID is still offered. The other positive tests carry an empty Definition
// and never reach the block-disk check, so this one keeps that check from
// treating every definition as block-backed.
func TestListVMsRenameSuggestionOffersFileBackedCandidate(t *testing.T) {
	st := newMemStore(t)
	def, err := json.Marshal(vmDefFixture{DomainXML: fileBackedDomainXML})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Method: "graceful", UUID: testVMUUID, Definition: string(def)}); err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{
		vms:       []virshcli.VMInfo{{Name: "win11", State: "running"}},
		xmlByName: map[string]string{"win11": vmRenameTestDomainXML(testVMUUID)},
	}
	svc := newRenameSuggestTestService(t, st, v)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	got := vmView(t, views, "win11")
	if got.RenameFrom != "windows-11" {
		t.Fatalf("RenameFrom = %q, want %q (a realistic file-backed stored definition must not disable rename suggestions)", got.RenameFrom, "windows-11")
	}
	if got.RenameReason != "libvirt-uuid" {
		t.Fatalf("RenameReason = %q, want %q", got.RenameReason, "libvirt-uuid")
	}
}

// A stored definition that is not valid JSON is an unknown disk layout, not a
// known-safe one, so the target is not offered even though its UUID matches.
func TestListVMsRenameSuggestionSkipsUnparseableJSONDefinition(t *testing.T) {
	st := newMemStore(t)
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Method: "graceful", UUID: testVMUUID, Definition: "{not valid json"}); err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{
		vms:       []virshcli.VMInfo{{Name: "win11", State: "running"}},
		xmlByName: map[string]string{"win11": vmRenameTestDomainXML(testVMUUID)},
	}
	svc := newRenameSuggestTestService(t, st, v)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	got := vmView(t, views, "win11")
	if got.RenameFrom != "" || got.RenameReason != "" {
		t.Fatalf("win11 must not be offered windows-11 when its stored definition is not valid JSON (fail closed: unknown disk layout), got RenameFrom=%q RenameReason=%q", got.RenameFrom, got.RenameReason)
	}
}

// The other parse failure the block-disk check fails closed on: valid JSON,
// but a DomainXML that virshcli.ParseDomain rejects.
func TestListVMsRenameSuggestionSkipsUnparseableDomainXML(t *testing.T) {
	st := newMemStore(t)
	def, err := json.Marshal(vmDefFixture{DomainXML: "<not-well-formed"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Method: "graceful", UUID: testVMUUID, Definition: string(def)}); err != nil {
		t.Fatal(err)
	}
	v := &renameSuggestVirsh{
		vms:       []virshcli.VMInfo{{Name: "win11", State: "running"}},
		xmlByName: map[string]string{"win11": vmRenameTestDomainXML(testVMUUID)},
	}
	svc := newRenameSuggestTestService(t, st, v)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	got := vmView(t, views, "win11")
	if got.RenameFrom != "" || got.RenameReason != "" {
		t.Fatalf("win11 must not be offered windows-11 when its stored DomainXML does not parse (fail closed: unknown disk layout), got RenameFrom=%q RenameReason=%q", got.RenameFrom, got.RenameReason)
	}
}

// On TrueNAS the rename suggestion and the alias-conflict check key on the
// raw libvirt name, not the friendly one. Every live domain here has a
// friendly name that differs from its raw name (the TrueNAS 25.10 "{id}_"
// shape):
//
//   - "1_win11" has no backups and shares its UUID with the not-installed
//     target "1_windows-11", so it is offered that name.
//   - "1_ubuntu" is a different domain that reuses a retired alias of
//     "1_ubuntu-current".
//   - "1_ubuntu-current" is live itself. An orphan row only ever sees the raw
//     store name, so only a live owner row shows which name the conflict
//     lookup keys on.
func TestListVMsTrueNASRenameSuggestionAndAliasConflictKeyOnLibvirtName(t *testing.T) {
	st := newMemStore(t)
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "1_windows-11", Method: "graceful", UUID: testVMUUID}); err != nil {
		t.Fatal(err)
	}
	ubuntuCurrent, err := st.UpsertVMTarget(store.VMTarget{Name: "1_ubuntu-current", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAlias("vm", "1_ubuntu", ubuntuCurrent.ID); err != nil {
		t.Fatal(err)
	}

	v := &renameSuggestVirsh{
		vms: []virshcli.VMInfo{
			{Name: "1_win11", State: "running", FriendlyName: "windows 11 display"},
			{Name: "1_ubuntu", State: "running", FriendlyName: "ubuntu display"},
			{Name: "1_ubuntu-current", State: "running", FriendlyName: "ubuntu current display"},
		},
		xmlByName: map[string]string{
			"1_win11":          vmRenameTestDomainXML(testVMUUID),
			"1_ubuntu":         vmRenameTestDomainXML("ffffffff-ffff-ffff-ffff-ffffffffffff"),
			"1_ubuntu-current": vmRenameTestDomainXML("11111111-1111-1111-1111-111111111111"),
		},
	}
	svc := newRenameSuggestTestService(t, st, v)
	svc.SetPlatform(platform.TrueNAS{})

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}

	renamed := vmView(t, views, "1_win11")
	if renamed.Name != "windows 11 display" {
		t.Fatalf("Name = %q, want the resolved friendly name", renamed.Name)
	}
	if renamed.RenameFrom != "1_windows-11" {
		t.Fatalf("RenameFrom = %q, want the raw libvirt old name %q, not a friendly one", renamed.RenameFrom, "1_windows-11")
	}
	if renamed.RenameReason != "libvirt-uuid" {
		t.Fatalf("RenameReason = %q, want %q", renamed.RenameReason, "libvirt-uuid")
	}

	// The domain reusing the retired name carries no conflict itself. It is
	// reported on the owner's row.
	reused := vmView(t, views, "1_ubuntu")
	if reused.Name != "ubuntu display" {
		t.Fatalf("Name = %q, want the resolved friendly name", reused.Name)
	}
	if len(reused.AliasConflicts) != 0 {
		t.Fatalf("the domain reusing the retired name must not itself carry the conflict, got %q", reused.AliasConflicts)
	}

	// The owner row is live, so its conflict proves the lookup keys on the
	// libvirt name and not the friendly one.
	owner := vmView(t, views, "1_ubuntu-current")
	if owner.Name != "ubuntu current display" {
		t.Fatalf("Name = %q, want the resolved friendly name", owner.Name)
	}
	if want := []string{"1_ubuntu"}; !reflect.DeepEqual(owner.AliasConflicts, want) {
		t.Fatalf("AliasConflicts = %q, want the raw libvirt name %q, not a friendly one", owner.AliasConflicts, want)
	}
}
