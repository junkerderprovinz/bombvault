package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// vmTakeoverDef mirrors the JSON of a stored VM definition.
type vmTakeoverDef struct {
	DomainXML     string   `json:"domain_xml"`
	DiskPaths     []string `json:"disk_paths"`
	NVRAMHostPath string   `json:"nvram_host_path"`
	NVRAMBytes    []byte   `json:"nvram_bytes,omitempty"`
	TPMBytes      []byte   `json:"tpm_bytes,omitempty"`
	Method        string   `json:"method"`
}

// unraidVMXML is the persistent XML of a VM called name, whose disk and NVRAM
// sit in places named after it, as Unraid lays them out.
func unraidVMXML(name string) string {
	return "<domain type='kvm'><name>" + name + "</name><uuid>" + testVMUUID + "</uuid>" +
		"<os><nvram>/etc/libvirt/qemu/nvram/" + name + "_VARS.fd</nvram></os>" +
		"<devices><disk type='file' device='disk'><source file='/mnt/user/domains/" + name + "/vdisk1.img'/><target dev='hdc'/></disk></devices></domain>"
}

const vmTakeoverOffsite = "rest:http://192.168.1.9:8000/vms-offsite"

// otherVMUUID is the libvirt UUID of a VM that is not the entry's.
const otherVMUUID = "ffffffff-ffff-ffff-ffff-ffffffffffff"

// vmTakeover is an entry backed up as oldName before its VM was renamed to
// newName, with the VMs repository established and the DR drill pinned to it.
type vmTakeover struct {
	cfg              config.Config
	svc              *api.Service
	db               *sql.DB
	st               *store.Repo
	eng              *fakeResticEngine
	virsh            *renameSuggestVirsh
	root, repo       string
	oldName, newName string
	id, oldDef       string
}

func newVMTakeover(t *testing.T, snaps ...restic.Snapshot) *vmTakeover {
	t.Helper()
	return newVMTakeoverNamed(t, "windows-11", "win11", snaps...)
}

func newVMTakeoverNamed(t *testing.T, oldName, newName string, snaps ...restic.Snapshot) *vmTakeover {
	t.Helper()
	f := &vmTakeover{root: filepath.ToSlash(t.TempDir()), oldName: oldName, newName: newName}
	f.cfg = config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: f.root, HostSourceRoot: "/mnt"}
	f.db, f.st = openStore(t)
	s := mustSettings(t, f.st)
	s.VMsPath = "backups/vms"
	s.VMsEnabled = true
	s.DRDrillTargetVM = oldName
	if err := f.st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	f.repo = establishLocalRepo(t, f.root, s.VMsPath)
	def, err := json.Marshal(vmTakeoverDef{
		DomainXML:     unraidVMXML(oldName),
		DiskPaths:     []string{f.root + "/user/domains/" + oldName + "/vdisk1.img"},
		NVRAMHostPath: "/etc/libvirt/qemu/nvram/" + oldName + "_VARS.fd",
		NVRAMBytes:    []byte("uefi-vars"),
		TPMBytes:      []byte("tpm-state"),
		Method:        "graceful",
	})
	if err != nil {
		t.Fatal(err)
	}
	f.oldDef = string(def)
	tg, err := f.st.UpsertVMTarget(store.VMTarget{Name: oldName, Definition: f.oldDef})
	if err != nil {
		t.Fatal(err)
	}
	f.id = tg.ID
	f.virsh = &renameSuggestVirsh{
		vms:       []virshcli.VMInfo{{Name: newName, State: "shut off"}},
		xmlByName: map[string]string{newName: unraidVMXML(newName)},
		errByName: map[string]error{},
	}
	f.eng = &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{f.repo: snaps}}
	f.svc = api.NewService(f.cfg, f.st, &fakeServiceDocker{}, f.virsh, f.eng)
	return f
}

// withOffsite gives the VMs domain an off-site copy holding snaps.
func (f *vmTakeover) withOffsite(t *testing.T, snaps ...restic.Snapshot) {
	t.Helper()
	s := mustSettings(t, f.st)
	s.VMsOffsite = vmTakeoverOffsite
	if err := f.st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	f.eng.snapsByRepo[vmTakeoverOffsite] = snaps
}

func (f *vmTakeover) router() http.Handler {
	sched := schedule.New(func(string) error { return nil }, f.st.ListTargets)
	return api.NewHandler(f.cfg, f.st, &fakeServiceDocker{}, f.svc, sched, spike.DefaultProbes()).Router()
}

func (f *vmTakeover) definition(t *testing.T, name string) vmTakeoverDef {
	t.Helper()
	tg, err := f.st.GetVMTargetByName(name)
	if err != nil {
		t.Fatalf("no entry on %q: %v", name, err)
	}
	var def vmTakeoverDef
	if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
		t.Fatalf("definition of %q: %v", name, err)
	}
	return def
}

// vmState lists every VM row with its definition, every VM alias with the
// definition it keeps, and the DR-drill pin, so a test can tell that a refused
// call wrote nothing.
func vmState(t *testing.T, st *store.Repo) string {
	t.Helper()
	rows, err := st.ListVMTargets()
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("row %s=%s %q", r.ID, r.Name, r.Definition))
	}
	aliases, err := st.ListAliases("vm")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range aliases {
		full, err := st.AliasByOldName("vm", a.OldName)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, fmt.Sprintf("alias %s->%s@%d %q", a.OldName, a.TargetID, a.LinkedAt, full.PrevDefinition))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\ndrill " + mustSettings(t, st).DRDrillTargetVM
}

var (
	vmPre  = restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}}
	vmMid  = restic.Snapshot{ID: "mid1", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:win11", "p2"}}
	vmPost = restic.Snapshot{ID: "post1", Time: "2099-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}}
)

// The entry answers to the new libvirt name with the renamed VM's disk
// folder, keeps its firmware state, keeps the definition it had for an
// unlink, and brings its old backups and the DR-drill pin along.
func TestTakeOverVMMovesTheEntryOntoTheRenamedVM(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	ctx := context.Background()

	if err := f.svc.TakeOverVM(ctx, "windows-11", "win11"); err != nil {
		t.Fatalf("TakeOverVM: %v", err)
	}
	tg, err := f.st.GetVMTargetByName("win11")
	if err != nil || tg.ID != f.id {
		t.Fatalf("entry on win11 = %+v, %v; want the same row", tg, err)
	}
	def := f.definition(t, "win11")
	if def.DomainXML != unraidVMXML("win11") {
		t.Fatalf("DomainXML = %q, want the renamed VM's", def.DomainXML)
	}
	if want := f.root + "/user/domains/win11/vdisk1.img"; len(def.DiskPaths) != 1 || def.DiskPaths[0] != want {
		t.Fatalf("DiskPaths = %v, want [%s]", def.DiskPaths, want)
	}
	if def.NVRAMHostPath != "/etc/libvirt/qemu/nvram/win11_VARS.fd" {
		t.Fatalf("NVRAMHostPath = %q, want the renamed VM's", def.NVRAMHostPath)
	}
	if string(def.NVRAMBytes) != "uefi-vars" || string(def.TPMBytes) != "tpm-state" || def.Method != "graceful" {
		t.Fatalf("captured state = %q %q %q, want it kept", def.NVRAMBytes, def.TPMBytes, def.Method)
	}
	a, err := f.st.AliasByOldName("vm", "windows-11")
	if err != nil || a.TargetID != f.id || a.PrevDefinition != f.oldDef {
		t.Fatalf("alias = %+v, %v; want windows-11 linked with its old definition", a, err)
	}
	if pin := mustSettings(t, f.st).DRDrillTargetVM; pin != "win11" {
		t.Fatalf("DR-drill pin = %q, want win11", pin)
	}
	snaps, err := f.svc.SnapshotsVM(ctx, "win11", "")
	if err != nil || joined(snapshotIDs(snaps)) != "pre1" {
		t.Fatalf("SnapshotsVM(win11) = %v, %v; want the old name's pre1", snapshotIDs(snaps), err)
	}
}

// Only a DR-drill pin on the old name follows the entry.
func TestTakeOverVMLeavesADRDrillPinOnAnotherVMAlone(t *testing.T) {
	f := newVMTakeover(t)
	s := mustSettings(t, f.st)
	s.DRDrillTargetVM = "ubuntu"
	if err := f.st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.TakeOverVM(context.Background(), "windows-11", "win11"); err != nil {
		t.Fatalf("TakeOverVM: %v", err)
	}
	if pin := mustSettings(t, f.st).DRDrillTargetVM; pin != "ubuntu" {
		t.Fatalf("DR-drill pin = %q, want ubuntu", pin)
	}
}

// The captured NVRAM and TPM state belong to one VM, so they follow the entry
// only when the renamed VM has the same libvirt UUID. Otherwise the next
// backup captures the right ones.
func TestTakeOverVMKeepsFirmwareStateOnlyForTheSameVM(t *testing.T) {
	withoutUUID := func(xml string) string { return strings.Replace(xml, "<uuid>"+testVMUUID+"</uuid>", "", 1) }
	for _, c := range []struct {
		name  string
		setup func(t *testing.T, f *vmTakeover)
		keep  bool
	}{
		{name: "the same UUID", keep: true},
		{name: "the same UUID from the stored column", keep: true,
			setup: func(t *testing.T, f *vmTakeover) {
				def, err := json.Marshal(vmTakeoverDef{
					DomainXML:     withoutUUID(unraidVMXML("windows-11")),
					DiskPaths:     []string{f.root + "/user/domains/windows-11/vdisk1.img"},
					NVRAMHostPath: "/etc/libvirt/qemu/nvram/windows-11_VARS.fd",
					NVRAMBytes:    []byte("uefi-vars"),
					TPMBytes:      []byte("tpm-state"),
					Method:        "graceful",
				})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Definition: string(def), UUID: testVMUUID}); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "the same UUID in the definition while the column names another VM", keep: true,
			setup: func(t *testing.T, f *vmTakeover) {
				if err := f.st.SetVMUUID("windows-11", otherVMUUID); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "another UUID on the renamed VM",
			setup: func(t *testing.T, f *vmTakeover) {
				f.virsh.xmlByName["win11"] = strings.Replace(unraidVMXML("win11"), testVMUUID, otherVMUUID, 1)
			}},
		{name: "no UUID on the renamed VM",
			setup: func(t *testing.T, f *vmTakeover) { f.virsh.xmlByName["win11"] = withoutUUID(unraidVMXML("win11")) }},
		{name: "no UUID on either side",
			setup: func(t *testing.T, f *vmTakeover) {
				def, err := json.Marshal(vmTakeoverDef{
					DomainXML:  withoutUUID(unraidVMXML("windows-11")),
					NVRAMBytes: []byte("uefi-vars"),
					TPMBytes:   []byte("tpm-state"),
					Method:     "graceful",
				})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Definition: string(def)}); err != nil {
					t.Fatal(err)
				}
				f.virsh.xmlByName["win11"] = withoutUUID(unraidVMXML("win11"))
			}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newVMTakeover(t)
			if c.setup != nil {
				c.setup(t, f)
			}
			if err := f.svc.TakeOverVM(context.Background(), "windows-11", "win11"); err != nil {
				t.Fatalf("TakeOverVM: %v", err)
			}
			def := f.definition(t, "win11")
			kept := string(def.NVRAMBytes) == "uefi-vars" && string(def.TPMBytes) == "tpm-state"
			dropped := len(def.NVRAMBytes) == 0 && len(def.TPMBytes) == 0
			if c.keep && !kept || !c.keep && !dropped {
				t.Fatalf("firmware state = %q %q, want it kept %v", def.NVRAMBytes, def.TPMBytes, c.keep)
			}
			if want := f.root + "/user/domains/win11/vdisk1.img"; len(def.DiskPaths) != 1 || def.DiskPaths[0] != want || def.Method != "graceful" {
				t.Fatalf("definition = %+v, want the renamed VM's disk and the kept method", def)
			}
		})
	}
}

// A row with no backups and no settings, as a click on the renamed VM leaves
// behind, makes way.
func TestTakeOverVMRemovesAnEmptyRowOnTheNewName(t *testing.T) {
	f := newVMTakeover(t)
	if _, err := f.st.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.TakeOverVM(context.Background(), "windows-11", "win11"); err != nil {
		t.Fatalf("TakeOverVM: %v", err)
	}
	if tg, err := f.st.GetVMTargetByName("win11"); err != nil || tg.ID != f.id {
		t.Fatalf("entry on win11 = %+v, %v; want the taken-over row", tg, err)
	}
}

// Every refusal comes before the first write, so the rows, the aliases and
// the DR-drill pin stay as they were.
func TestTakeOverVMRefusesBeforeWritingAnything(t *testing.T) {
	storedDef := func(t *testing.T, f *vmTakeover, def string) {
		t.Helper()
		if _, err := f.st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Definition: def}); err != nil {
			t.Fatal(err)
		}
	}
	jsonDef := func(t *testing.T, domainXML string) string {
		t.Helper()
		b, err := json.Marshal(vmTakeoverDef{DomainXML: domainXML})
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	rowOnNewName := func(t *testing.T, f *vmTakeover) {
		t.Helper()
		if _, err := f.st.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
			t.Fatal(err)
		}
	}
	otherVM := restic.Snapshot{ID: "other1", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:win11", "p2"}}
	ownRepo := func(t *testing.T, f *vmTakeover) string {
		t.Helper()
		named, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "Cold", Repo: "backups/cold", Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := f.st.SetVMRepo("windows-11", named.ID); err != nil {
			t.Fatal(err)
		}
		return establishLocalRepo(t, f.root, "backups/cold")
	}

	for _, c := range []struct {
		name     string
		setup    func(t *testing.T, f *vmTakeover)
		old, new string
		want     string
	}{
		{name: "the same name on both sides", old: "windows-11", new: "windows-11", want: "itself"},
		{name: "an old name that is not a VM name", old: "-windows-11", new: "win11", want: "invalid VM name",
			setup: func(t *testing.T, f *vmTakeover) {
				if _, err := f.st.UpsertVMTarget(store.VMTarget{Name: "-windows-11", Definition: f.oldDef}); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "an old name with a comma", old: "windows,11", new: "win11", want: "comma",
			setup: func(t *testing.T, f *vmTakeover) {
				if _, err := f.st.UpsertVMTarget(store.VMTarget{Name: "windows,11", Definition: f.oldDef}); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "a new name that is not a VM name", old: "windows-11", new: "-win11", want: "invalid VM name",
			setup: func(t *testing.T, f *vmTakeover) {
				f.virsh.vms = []virshcli.VMInfo{{Name: "-win11"}}
				f.virsh.xmlByName["-win11"] = unraidVMXML("win11")
			}},
		{name: "the new name is not defined", old: "windows-11", new: "win11", want: "not defined",
			setup: func(t *testing.T, f *vmTakeover) { f.virsh.vms = nil }},
		{name: "the old name is defined again", old: "windows-11", new: "win11", want: "defined on the host again",
			setup: func(t *testing.T, f *vmTakeover) {
				f.virsh.vms = append(f.virsh.vms, virshcli.VMInfo{Name: "windows-11"})
			}},
		{name: "the old name has no entry", old: "windows-10", new: "win11", want: "no entry"},
		{name: "the new name is another entry's former name", old: "windows-11", new: "win11", want: `"win11-gaming"`,
			setup: func(t *testing.T, f *vmTakeover) {
				other, err := f.st.UpsertVMTarget(store.VMTarget{Name: "win11-gaming"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.st.AddAliasAt("vm", "win11", other.ID, unixOf(t, linkTime)); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "the old name is another entry's former name", old: "windows-11", new: "win11", want: `"win10"`,
			setup: func(t *testing.T, f *vmTakeover) {
				other, err := f.st.UpsertVMTarget(store.VMTarget{Name: "win10"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.st.AddAliasAt("vm", "windows-11", other.ID, unixOf(t, linkTime)); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "another VM's backup under the new name", old: "windows-11", new: "win11", want: "already has backups",
			setup: func(t *testing.T, f *vmTakeover) { f.eng.snapsByRepo[f.repo] = []restic.Snapshot{vmPre, otherVM} }},
		{name: "another VM's backup under the new name naming a former name of that VM", old: "windows-11", new: "win11", want: "already has backups",
			setup: func(t *testing.T, f *vmTakeover) {
				marked := restic.Snapshot{ID: "other2", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:win11", "p2", "formerly:win10"}}
				f.eng.snapsByRepo[f.repo] = []restic.Snapshot{vmPre, marked}
			}},
		{name: "another VM's backup in the entry's own repository", old: "windows-11", new: "win11", want: "already has backups",
			setup: func(t *testing.T, f *vmTakeover) { f.eng.snapsByRepo[ownRepo(t, f)] = []restic.Snapshot{otherVM} }},
		{name: "another VM's backup where the new name resolves, the entry keeping its own repository", old: "windows-11", new: "win11", want: "already has backups",
			setup: func(t *testing.T, f *vmTakeover) {
				ownRepo(t, f)
				f.eng.snapsByRepo[f.repo] = []restic.Snapshot{vmPre, otherVM}
			}},
		{name: "another VM's backup under the new name, off-site only", old: "windows-11", new: "win11", want: `off-site target "Primary"`,
			setup: func(t *testing.T, f *vmTakeover) { f.withOffsite(t, vmPre, otherVM) }},
		{name: "the new name's backups cannot be listed", old: "windows-11", new: "win11", want: "could not be read",
			setup: func(t *testing.T, f *vmTakeover) {
				f.eng.snapsErrFor = map[string]error{f.repo: errors.New("repository unreadable")}
			}},
		{name: "the off-site copy cannot be listed", old: "windows-11", new: "win11", want: `off-site target "Primary"`,
			setup: func(t *testing.T, f *vmTakeover) {
				f.withOffsite(t)
				f.eng.snapsErrFor = map[string]error{vmTakeoverOffsite: errors.New("connection refused")}
			}},
		{name: "a scheduled row on the new name", old: "windows-11", new: "win11", want: "scheduled",
			setup: func(t *testing.T, f *vmTakeover) {
				rowOnNewName(t, f)
				if err := f.st.SetVMInclude("win11", true); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "a row on the new name with its own backup method", old: "windows-11", new: "win11", want: "backup method",
			setup: func(t *testing.T, f *vmTakeover) {
				rowOnNewName(t, f)
				if err := f.st.SetVMMethod("win11", "live"); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "a row on the new name with its own schedule", old: "windows-11", new: "win11", want: "schedule cadence",
			setup: func(t *testing.T, f *vmTakeover) {
				rowOnNewName(t, f)
				if err := f.st.SetVMScheduleCadence("win11", "daily 03:00"); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "a row on the new name with a backup order", old: "windows-11", new: "win11", want: "backup order",
			setup: func(t *testing.T, f *vmTakeover) {
				rowOnNewName(t, f)
				if err := f.st.SetVMBackupOrders([]store.VMOrder{{VM: "win11", Order: 1}}); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "a row on the new name with its own repository", old: "windows-11", new: "win11", want: "repository",
			setup: func(t *testing.T, f *vmTakeover) {
				named, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "Cold", Repo: "backups/cold", Enabled: true})
				if err != nil {
					t.Fatal(err)
				}
				if err := f.st.SetVMRepo("win11", named.ID); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "a row on the new name with a backup of its own", old: "windows-11", new: "win11", want: "own entry with backups",
			setup: func(t *testing.T, f *vmTakeover) {
				rowOnNewName(t, f)
				tg, err := f.st.GetVMTargetByName("win11")
				if err != nil {
					t.Fatal(err)
				}
				runID, err := f.st.StartRun(tg.ID, "backup")
				if err != nil {
					t.Fatal(err)
				}
				if err := f.st.FinishRun(runID, "success", "deadbeef", 1024, ""); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "a stored definition with a block-device disk", old: "windows-11", new: "win11", want: "zvol",
			setup: func(t *testing.T, f *vmTakeover) { storedDef(t, f, jsonDef(t, blockDiskDomainXML)) }},
		{name: "a stored definition that is not JSON", old: "windows-11", new: "win11", want: "disk layout is unknown",
			setup: func(t *testing.T, f *vmTakeover) { storedDef(t, f, "{not valid json") }},
		{name: "a stored definition that does not decode", old: "windows-11", new: "win11", want: "disk layout is unknown",
			setup: func(t *testing.T, f *vmTakeover) {
				storedDef(t, f, `{"domain_xml":"<domain type='kvm'/>","disk_paths":"not-a-list"}`)
			}},
		{name: "a stored domain XML that does not parse", old: "windows-11", new: "win11", want: "disk layout is unknown",
			setup: func(t *testing.T, f *vmTakeover) { storedDef(t, f, jsonDef(t, "<not-well-formed")) }},
		{name: "the renamed VM's definition cannot be read", old: "windows-11", new: "win11", want: "could not be read",
			setup: func(t *testing.T, f *vmTakeover) { f.virsh.errByName["win11"] = errors.New("ssh: connection reset") }},
		{name: "the renamed VM's definition does not parse", old: "windows-11", new: "win11", want: "does not parse",
			setup: func(t *testing.T, f *vmTakeover) { f.virsh.xmlByName["win11"] = "<not-well-formed" }},
		{name: "the renamed VM's disk is outside the host mount", old: "windows-11", new: "win11", want: "not under the host mount",
			setup: func(t *testing.T, f *vmTakeover) {
				f.virsh.xmlByName["win11"] = strings.Replace(unraidVMXML("win11"), "/mnt/user/domains", "/srv/vms", 1)
			}},
		{name: "the renamed VM has no file disk", old: "windows-11", new: "win11", want: "no disk paths",
			setup: func(t *testing.T, f *vmTakeover) { f.virsh.xmlByName["win11"] = vmRenameTestDomainXML(testVMUUID) }},
		{name: "the renamed VM is left on a snapshot overlay", old: "windows-11", new: "win11", want: `"Delete all backups" on "win11"`,
			setup: func(t *testing.T, f *vmTakeover) {
				f.virsh.xmlByName["win11"] = strings.Replace(unraidVMXML("win11"), "vdisk1.img", "vdisk1.bombvault-tmp", 1)
			}},
		{name: "an empty row on the new name and a block-device disk", old: "windows-11", new: "win11", want: "zvol",
			setup: func(t *testing.T, f *vmTakeover) {
				rowOnNewName(t, f)
				storedDef(t, f, jsonDef(t, blockDiskDomainXML))
			}},
		{name: "an empty row on the new name and a renamed VM that cannot be read", old: "windows-11", new: "win11", want: "could not be read",
			setup: func(t *testing.T, f *vmTakeover) {
				rowOnNewName(t, f)
				f.virsh.errByName["win11"] = errors.New("ssh: connection reset")
			}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newVMTakeover(t, vmPre)
			if c.setup != nil {
				c.setup(t, f)
			}
			before := vmState(t, f.st)
			err := f.svc.TakeOverVM(context.Background(), c.old, c.new)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("TakeOverVM(%q, %q) = %v, want a refusal containing %q", c.old, c.new, err, c.want)
			}
			if after := vmState(t, f.st); after != before {
				t.Fatalf("a refused takeover changed the store:\nbefore\n%s\nafter\n%s", before, after)
			}
		})
	}
}

// The row on win11 may own backups under a former name, so it is not removed
// as empty while that alias list cannot be read, and both rows stay.
func TestTakeOverVMRefusesWhenTheNewNamesFormerNamesCannotBeRead(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	b, err := f.st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec("INSERT INTO target_aliases (id, domain, old_name, target_id, linked_at) VALUES ('bad', 'vm', 'win11-old', ?, 'garbage')", b.ID); err != nil {
		t.Fatal(err)
	}

	err = f.svc.TakeOverVM(context.Background(), "windows-11", "win11")
	if err == nil || !strings.Contains(err.Error(), "backups could not be checked") {
		t.Fatalf("TakeOverVM = %v, want the backups check to refuse", err)
	}
	if tg, err := f.st.GetVMTargetByName("win11"); err != nil || tg.ID != b.ID {
		t.Fatalf("the row on win11 must stay: %+v, %v", tg, err)
	}
	if tg, err := f.st.GetVMTargetByName("windows-11"); err != nil || tg.ID != f.id {
		t.Fatalf("the entry must keep its name: %+v, %v", tg, err)
	}
}

// renamedBack is an entry taken over from windows-11 to win11 whose VM was
// then renamed back to windows-11, with Unraid's XML for it changed on the way.
func renamedBack(t *testing.T, snaps ...restic.Snapshot) (*vmTakeover, string) {
	t.Helper()
	f := newVMTakeover(t, vmPre)
	if err := f.svc.TakeOverVM(context.Background(), "windows-11", "win11"); err != nil {
		t.Fatalf("first takeover: %v", err)
	}
	tg, err := f.st.GetVMTargetByName("win11")
	if err != nil {
		t.Fatal(err)
	}
	f.eng.snapsByRepo[f.repo] = snaps
	f.virsh.vms = []virshcli.VMInfo{{Name: "windows-11"}}
	f.virsh.xmlByName["windows-11"] = strings.Replace(unraidVMXML("windows-11"), "hdc", "sda", 1)
	return f, tg.Definition
}

// The entry returns to what it was under windows-11, and the backups under
// both names stay its own.
func TestTakeOverVMRenamedBackRestoresTheDefinitionItHadThere(t *testing.T) {
	f, asWin11 := renamedBack(t, vmPre, vmMid)
	ctx := context.Background()

	if err := f.svc.TakeOverVM(ctx, "win11", "windows-11"); err != nil {
		t.Fatalf("rename back: %v", err)
	}
	tg, err := f.st.GetVMTargetByName("windows-11")
	if err != nil || tg.ID != f.id || tg.Definition != f.oldDef {
		t.Fatalf("entry on windows-11 = %+v, %v; want the same row with its definition from before", tg, err)
	}
	if _, err := f.st.AliasByOldName("vm", "windows-11"); err == nil {
		t.Fatal("windows-11 is the entry's name again, its alias must be gone")
	}
	if a, err := f.st.AliasByOldName("vm", "win11"); err != nil || a.TargetID != f.id || a.PrevDefinition != asWin11 {
		t.Fatalf("win11 alias = %+v, %v; want it linked with the definition it had there", a, err)
	}
	snaps, err := f.svc.SnapshotsVM(ctx, "windows-11", "")
	if err != nil || joined(snapshotIDs(snaps)) != "mid1,pre1" {
		t.Fatalf("SnapshotsVM(windows-11) = %v, %v; want both histories", snapshotIDs(snaps), err)
	}
	if pin := mustSettings(t, f.st).DRDrillTargetVM; pin != "windows-11" {
		t.Fatalf("DR-drill pin = %q, want windows-11", pin)
	}
}

// Dropping the alias lifts the time bound on windows-11, so a later VM's
// backup under it refuses, as does a listing that fails, and an alias that
// kept no definition cannot be restored.
func TestTakeOverVMRenamedBackRefusals(t *testing.T) {
	t.Run("a backup under the old name from after the link", func(t *testing.T) {
		f, _ := renamedBack(t, vmPre, vmMid, vmPost)
		before := vmState(t, f.st)
		if err := f.svc.TakeOverVM(context.Background(), "win11", "windows-11"); err == nil || !strings.Contains(err.Error(), "another VM") {
			t.Fatalf("rename back = %v, want a refusal naming another VM", err)
		}
		if after := vmState(t, f.st); after != before {
			t.Fatalf("a refused rename back changed the store:\nbefore\n%s\nafter\n%s", before, after)
		}
	})
	t.Run("the listing fails", func(t *testing.T) {
		f, _ := renamedBack(t, vmPre, vmMid)
		f.eng.snapsErrFor = map[string]error{f.repo: errors.New("repository unreadable")}
		before := vmState(t, f.st)
		if err := f.svc.TakeOverVM(context.Background(), "win11", "windows-11"); err == nil {
			t.Fatal("renaming back must be refused when windows-11's backups cannot be read")
		}
		if after := vmState(t, f.st); after != before {
			t.Fatalf("a refused rename back changed the store:\nbefore\n%s\nafter\n%s", before, after)
		}
	})
	t.Run("the alias kept no definition", func(t *testing.T) {
		f := newVMTakeoverNamed(t, "win11", "windows-11", vmPre)
		if _, err := f.st.AddAliasAt("vm", "windows-11", f.id, unixOf(t, linkTime)); err != nil {
			t.Fatal(err)
		}
		before := vmState(t, f.st)
		if err := f.svc.TakeOverVM(context.Background(), "win11", "windows-11"); err == nil || !strings.Contains(err.Error(), "not kept") {
			t.Fatalf("rename back = %v, want a refusal because no definition was kept", err)
		}
		if after := vmState(t, f.st); after != before {
			t.Fatalf("a refused rename back changed the store:\nbefore\n%s\nafter\n%s", before, after)
		}
	})
	for _, c := range []struct {
		name  string
		setup func(f *vmTakeover)
		want  string
	}{
		{"a VM with another UUID under the old name", func(f *vmTakeover) {
			f.virsh.xmlByName["windows-11"] = strings.Replace(f.virsh.xmlByName["windows-11"], testVMUUID, otherVMUUID, 1)
		}, "rename that VM"},
		{"the VM under the old name cannot be read", func(f *vmTakeover) {
			f.virsh.errByName["windows-11"] = errors.New("ssh: connection reset")
		}, "VM defined under that name could not be read"},
		{"both names defined", func(f *vmTakeover) {
			f.virsh.vms = append(f.virsh.vms, virshcli.VMInfo{Name: "win11"})
		}, "defined on the host again"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, _ := renamedBack(t, vmPre, vmMid)
			c.setup(f)
			before := vmState(t, f.st)
			if err := f.svc.TakeOverVM(context.Background(), "win11", "windows-11"); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("rename back = %v, want a refusal containing %q", err, c.want)
			}
			if after := vmState(t, f.st); after != before {
				t.Fatalf("a refused rename back changed the store:\nbefore\n%s\nafter\n%s", before, after)
			}
		})
	}
}

// The unlinked entry points at its own old disks again, not at the live VM's,
// and the DR-drill pin goes back with it.
func TestUnlinkVMAliasPutsBackTheDefinitionFromBeforeTheTakeover(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	ctx := context.Background()
	if err := f.svc.TakeOverVM(ctx, "windows-11", "win11"); err != nil {
		t.Fatalf("TakeOverVM: %v", err)
	}

	if err := f.svc.UnlinkVMAlias(ctx, "windows-11"); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}
	tg, err := f.st.GetVMTargetByName("windows-11")
	if err != nil || tg.ID != f.id || tg.Definition != f.oldDef {
		t.Fatalf("entry on windows-11 = %+v, %v; want the same row with its definition from before", tg, err)
	}
	if _, err := f.st.AliasByOldName("vm", "windows-11"); err == nil {
		t.Fatal("the alias must be gone after the unlink")
	}
	if pin := mustSettings(t, f.st).DRDrillTargetVM; pin != "windows-11" {
		t.Fatalf("DR-drill pin = %q, want windows-11", pin)
	}
}

// The backups taken under win11 while the entry answered to it name
// windows-11 as a former name, so taking win11 over again counts them as the
// entry's own.
func TestTakeOverVMAgainAfterAnUnlinkKeepsTheBackupsMadeInBetween(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	ctx := context.Background()
	if err := f.svc.TakeOverVM(ctx, "windows-11", "win11"); err != nil {
		t.Fatalf("first takeover: %v", err)
	}
	if _, err := f.svc.BackupVM(ctx, "win11"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}
	between := restic.Snapshot{ID: "between1", Time: time.Now().UTC().Format(time.RFC3339), Tags: f.eng.lastTags}
	f.eng.snapsByRepo[f.repo] = append(f.eng.snapsByRepo[f.repo], between)
	if err := f.svc.UnlinkVMAlias(ctx, "windows-11"); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}

	if err := f.svc.TakeOverVM(ctx, "windows-11", "win11"); err != nil {
		t.Fatalf("second takeover: %v", err)
	}
	snaps, err := f.svc.SnapshotsVM(ctx, "win11", "")
	if err != nil || joined(snapshotIDs(snaps)) != "between1,pre1" {
		t.Fatalf("SnapshotsVM(win11) = %v, %v; want the backup made in between and pre1", snapshotIDs(snaps), err)
	}
}

// A backup under win11 that names win10, which the entry left before
// windows-11, is the entry's own too.
func TestTakeOverVMCountsABackupNamingAnOlderFormerNameAsItsOwn(t *testing.T) {
	own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:win11", "p2", "formerly:win10"}}
	f := newVMTakeover(t, vmPre, own)
	if _, err := f.st.AddAliasAt("vm", "win10", f.id, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.TakeOverVM(context.Background(), "windows-11", "win11"); err != nil {
		t.Fatalf("TakeOverVM: %v", err)
	}
}

// An unlink that could make a later VM's backups the entry's own, or could
// not put the old definition back, changes nothing.
func TestUnlinkVMAliasRefusesBeforeWritingAnything(t *testing.T) {
	takenOver := func(t *testing.T, snaps ...restic.Snapshot) *vmTakeover {
		t.Helper()
		f := newVMTakeover(t, snaps...)
		if err := f.svc.TakeOverVM(context.Background(), "windows-11", "win11"); err != nil {
			t.Fatalf("TakeOverVM: %v", err)
		}
		return f
	}
	for _, c := range []struct {
		name  string
		setup func(t *testing.T) *vmTakeover
		old   string
		want  string
	}{
		{name: "a backup under the old name from after the link", old: "windows-11", want: "stays linked",
			setup: func(t *testing.T) *vmTakeover { return takenOver(t, vmPre, vmPost) }},
		{name: "the listing fails", old: "windows-11", want: "could not be read",
			setup: func(t *testing.T) *vmTakeover {
				f := takenOver(t, vmPre)
				f.eng.snapsErrFor = map[string]error{f.repo: errors.New("repository unreadable")}
				return f
			}},
		{name: "the off-site copy holds a later backup", old: "windows-11", want: "stays linked",
			setup: func(t *testing.T) *vmTakeover {
				f := takenOver(t, vmPre)
				f.withOffsite(t, vmPre, vmPost)
				return f
			}},
		{name: "the alias kept no definition", old: "windows-11", want: "not kept",
			setup: func(t *testing.T) *vmTakeover {
				f := newVMTakeoverNamed(t, "win11", "win11-new", vmPre)
				if _, err := f.st.AddAliasAt("vm", "windows-11", f.id, unixOf(t, linkTime)); err != nil {
					t.Fatal(err)
				}
				return f
			}},
		{name: "another VM defined under the old name next to the entry's", old: "windows-11", want: "rename that VM",
			setup: func(t *testing.T) *vmTakeover {
				f := takenOver(t, vmPre)
				f.virsh.vms = append(f.virsh.vms, virshcli.VMInfo{Name: "windows-11"})
				f.virsh.xmlByName["windows-11"] = unraidVMXML("windows-11")
				return f
			}},
		{name: "a VM with another UUID under the old name", old: "windows-11", want: "rename that VM",
			setup: func(t *testing.T) *vmTakeover {
				f := takenOver(t, vmPre)
				f.virsh.vms = []virshcli.VMInfo{{Name: "windows-11"}}
				f.virsh.xmlByName["windows-11"] = strings.Replace(unraidVMXML("windows-11"), testVMUUID, otherVMUUID, 1)
				return f
			}},
		{name: "the VM under the old name cannot be read", old: "windows-11", want: "VM defined under that name could not be read",
			setup: func(t *testing.T) *vmTakeover {
				f := takenOver(t, vmPre)
				f.virsh.vms = []virshcli.VMInfo{{Name: "windows-11"}}
				f.virsh.errByName["windows-11"] = errors.New("ssh: connection reset")
				return f
			}},
		{name: "the VMs cannot be listed", old: "windows-11", want: "can be listed",
			setup: func(t *testing.T) *vmTakeover {
				f := takenOver(t, vmPre)
				f.virsh.listErr = errors.New("ssh: connection reset")
				return f
			}},
		{name: "a name that was never taken over", old: "windows-10", want: "not a taken-over name",
			setup: func(t *testing.T) *vmTakeover { return takenOver(t, vmPre) }},
		{name: "an old name that is not a VM name", old: "-windows-11", want: "invalid VM name",
			setup: func(t *testing.T) *vmTakeover {
				f := newVMTakeoverNamed(t, "-windows-11", "win11", vmPre)
				if err := f.st.RenameVMTargetWithAlias("-windows-11", "win11", f.oldDef, ""); err != nil {
					t.Fatal(err)
				}
				return f
			}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := c.setup(t)
			before := vmState(t, f.st)
			err := f.svc.UnlinkVMAlias(context.Background(), c.old)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("UnlinkVMAlias(%q) = %v, want a refusal containing %q", c.old, err, c.want)
			}
			if after := vmState(t, f.st); after != before {
				t.Fatalf("a refused unlink changed the store:\nbefore\n%s\nafter\n%s", before, after)
			}
		})
	}
}

// A VM renamed back to its old name in libvirt is still the entry's VM, so
// the unlink follows it there.
func TestUnlinkVMAliasFollowsTheVMRenamedBackUnderItsOldName(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	ctx := context.Background()
	if err := f.svc.TakeOverVM(ctx, "windows-11", "win11"); err != nil {
		t.Fatalf("TakeOverVM: %v", err)
	}
	f.virsh.vms = []virshcli.VMInfo{{Name: "windows-11"}}
	f.virsh.xmlByName["windows-11"] = unraidVMXML("windows-11")

	if err := f.svc.UnlinkVMAlias(ctx, "windows-11"); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}
	if tg, err := f.st.GetVMTargetByName("windows-11"); err != nil || tg.ID != f.id {
		t.Fatalf("entry on windows-11 = %+v, %v; want the same row", tg, err)
	}
}

// The uuid column names the VM the stored definition belongs to, which the
// rename suggestion and a later takeover's firmware check go by. A backup
// refreshes it, and a takeover, an unlink and a rename back store the one of
// the definition they write.
func TestTheStoredVMUUIDFollowsTheDefinition(t *testing.T) {
	ctx := context.Background()
	uuidOn := func(t *testing.T, f *vmTakeover, name string) string {
		t.Helper()
		tg, err := f.st.GetVMTargetByName(name)
		if err != nil {
			t.Fatalf("no entry on %q: %v", name, err)
		}
		return tg.UUID
	}
	// linkedToAnotherVM takes the entry over onto a win11 with another UUID.
	linkedToAnotherVM := func(t *testing.T) *vmTakeover {
		t.Helper()
		f := newVMTakeover(t, vmPre)
		f.virsh.xmlByName["win11"] = strings.Replace(unraidVMXML("win11"), testVMUUID, otherVMUUID, 1)
		if err := f.svc.TakeOverVM(ctx, "windows-11", "win11"); err != nil {
			t.Fatalf("TakeOverVM: %v", err)
		}
		return f
	}

	t.Run("a backup", func(t *testing.T) {
		f := newVMTakeover(t)
		if err := f.st.SetVMUUID("windows-11", otherVMUUID); err != nil {
			t.Fatal(err)
		}
		f.virsh.vms = []virshcli.VMInfo{{Name: "windows-11"}}
		f.virsh.xmlByName["windows-11"] = unraidVMXML("windows-11")
		if _, err := f.svc.BackupVM(ctx, "windows-11"); err != nil {
			t.Fatalf("BackupVM: %v", err)
		}
		if got := uuidOn(t, f, "windows-11"); got != testVMUUID {
			t.Fatalf("uuid = %q, want the backed-up VM's %q", got, testVMUUID)
		}
	})
	t.Run("a takeover", func(t *testing.T) {
		f := linkedToAnotherVM(t)
		if got := uuidOn(t, f, "win11"); got != otherVMUUID {
			t.Fatalf("uuid = %q, want win11's %q", got, otherVMUUID)
		}
	})
	t.Run("an unlink", func(t *testing.T) {
		f := linkedToAnotherVM(t)
		if err := f.st.SetVMUUID("win11", otherVMUUID); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.UnlinkVMAlias(ctx, "windows-11"); err != nil {
			t.Fatalf("UnlinkVMAlias: %v", err)
		}
		if got := uuidOn(t, f, "windows-11"); got != testVMUUID {
			t.Fatalf("uuid = %q, want the one of the definition from before, %q", got, testVMUUID)
		}
	})
	t.Run("a rename back", func(t *testing.T) {
		f := linkedToAnotherVM(t)
		if err := f.st.SetVMUUID("win11", otherVMUUID); err != nil {
			t.Fatal(err)
		}
		f.virsh.vms = []virshcli.VMInfo{{Name: "windows-11"}}
		f.virsh.xmlByName["windows-11"] = unraidVMXML("windows-11")
		if err := f.svc.TakeOverVM(ctx, "win11", "windows-11"); err != nil {
			t.Fatalf("rename back: %v", err)
		}
		if got := uuidOn(t, f, "windows-11"); got != testVMUUID {
			t.Fatalf("uuid = %q, want the one of the definition from before, %q", got, testVMUUID)
		}
	})
}

// On TrueNAS the list shows a display name, but the route takes the libvirt
// name, and only that one.
func TestTakeOverVMRouteKeysOnTheLibvirtName(t *testing.T) {
	f := newVMTakeoverNamed(t, "1_windows-11", "1_win11")
	f.virsh.vms = []virshcli.VMInfo{{Name: "1_win11", State: "shut off", FriendlyName: "win11"}}
	f.svc.SetPlatform(platform.TrueNAS{})
	h := f.router()

	if _, m := doJSON(t, h, http.MethodPost, "/api/vms/win11/takeover", `{"from":"1_windows-11"}`); m["ok"] == true {
		t.Fatalf("a takeover onto the display name must be refused, got %v", m)
	}
	w, m := doJSON(t, h, http.MethodPost, "/api/vms/1_win11/takeover", `{"from":"1_windows-11"}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("takeover = %d %v", w.Code, m)
	}
	if tg, err := f.st.GetVMTargetByName("1_win11"); err != nil || tg.ID != f.id {
		t.Fatalf("entry on 1_win11 = %+v, %v", tg, err)
	}
}

// Unraid's own VM names carry spaces, and an unsafe "from" is refused at the
// boundary.
func TestTakeOverVMRouteTakesNamesWithSpaces(t *testing.T) {
	f := newVMTakeoverNamed(t, "Windows 11", "Win 11")
	h := f.router()

	w, m := doJSON(t, h, http.MethodPost, "/api/vms/Win%2011/takeover", `{"from":"../Windows 11"}`)
	if w.Code != http.StatusBadRequest || m["ok"] == true {
		t.Fatalf("an unsafe from must be refused with 400, got %d %v", w.Code, m)
	}
	w, m = doJSON(t, h, http.MethodPost, "/api/vms/Win%2011/takeover", `{"from":"Windows 11"}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("takeover = %d %v", w.Code, m)
	}
	if tg, err := f.st.GetVMTargetByName("Win 11"); err != nil || tg.ID != f.id {
		t.Fatalf("entry on Win 11 = %+v, %v", tg, err)
	}
}

// DELETE /api/vms/{name}/alias/{old} moves the entry back, and an unsafe old
// name is refused at the boundary.
func TestUnlinkVMAliasRoute(t *testing.T) {
	f := newVMTakeoverNamed(t, "Windows 11", "Win 11")
	if err := f.svc.TakeOverVM(context.Background(), "Windows 11", "Win 11"); err != nil {
		t.Fatalf("TakeOverVM: %v", err)
	}
	h := f.router()

	w, m := doJSON(t, h, http.MethodDelete, "/api/vms/Win%2011/alias/-Windows%2011", "")
	if w.Code != http.StatusBadRequest || m["ok"] == true {
		t.Fatalf("an unsafe old name must be refused with 400, got %d %v", w.Code, m)
	}
	w, m = doJSON(t, h, http.MethodDelete, "/api/vms/Win%2011/alias/Windows%2011", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("unlink = %d %v", w.Code, m)
	}
	if tg, err := f.st.GetVMTargetByName("Windows 11"); err != nil || tg.ID != f.id || tg.Definition != f.oldDef {
		t.Fatalf("entry on Windows 11 = %+v, %v; want it back with its old definition", tg, err)
	}
}

// Every former name of an entry that is a live VM again is a conflict, listed
// alphabetically, and a row without one carries an empty list.
func TestListVMsRouteListsEveryLiveFormerNameAsAConflict(t *testing.T) {
	f := newVMTakeover(t)
	if err := f.svc.TakeOverVM(context.Background(), "windows-11", "win11"); err != nil {
		t.Fatalf("TakeOverVM: %v", err)
	}
	if _, err := f.st.AddAliasAt("vm", "zz-old", f.id, 1); err != nil {
		t.Fatal(err)
	}
	f.virsh.vms = append(f.virsh.vms, virshcli.VMInfo{Name: "windows-11"}, virshcli.VMInfo{Name: "zz-old"})

	w, m := doJSON(t, f.router(), http.MethodGet, "/api/vms", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("list failed: %d %v", w.Code, m)
	}
	got := map[string]any{}
	for _, row := range m["vms"].([]any) {
		r := row.(map[string]any)
		got[r["libvirtName"].(string)] = r["aliasConflicts"]
	}
	want := map[string]any{
		"win11":      []any{"windows-11", "zz-old"},
		"windows-11": []any{},
		"zz-old":     []any{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("aliasConflicts = %#v, want %#v", got, want)
	}
}

func TestListVMsRouteListsFormerNamesInLinkOrder(t *testing.T) {
	f := newVMTakeover(t)
	if err := f.svc.TakeOverVM(context.Background(), "windows-11", "win11"); err != nil {
		t.Fatalf("TakeOverVM: %v", err)
	}
	if _, err := f.st.AddAliasAt("vm", "zz-first", f.id, 1); err != nil {
		t.Fatal(err)
	}
	gone, err := f.st.UpsertVMTarget(store.VMTarget{Name: "gone", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.AddAliasAt("vm", "gone-old", gone.ID, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.UpsertVMTarget(store.VMTarget{Name: "tracked", Method: "graceful"}); err != nil {
		t.Fatal(err)
	}
	f.virsh.vms = append(f.virsh.vms,
		virshcli.VMInfo{Name: "tracked", State: "running"},
		virshcli.VMInfo{Name: "fresh", State: "running"},
	)

	w, m := doJSON(t, f.router(), http.MethodGet, "/api/vms", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("list failed: %d %v", w.Code, m)
	}
	got := map[string]any{}
	for _, row := range m["vms"].([]any) {
		r := row.(map[string]any)
		got[r["libvirtName"].(string)] = r["aliases"]
	}
	want := map[string]any{
		"win11":   []any{"zz-first", "windows-11"},
		"tracked": []any{},
		"fresh":   []any{},
		"gone":    []any{"gone-old"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("aliases = %#v, want %#v", got, want)
	}
}
