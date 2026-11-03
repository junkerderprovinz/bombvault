package api_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/platform"
)

// writeQCOW2 puts a qcow2 header naming backing at rel below the Host Data
// mount root, which the host knows as /mnt.
func writeQCOW2(t *testing.T, root, rel, backing string) string {
	t.Helper()
	b := make([]byte, 512)
	be := binary.BigEndian
	be.PutUint32(b[0:], 0x514649fb)
	be.PutUint32(b[4:], 3)
	be.PutUint32(b[20:], 16)
	be.PutUint32(b[100:], 104)
	if backing != "" {
		be.PutUint64(b[8:], 112)
		be.PutUint32(b[16:], uint32(len(backing))) //nolint:gosec // G115: a short test string
		copy(b[112:], backing)
	}
	file := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return root + "/" + rel
}

func qcow2DomainXML(sources ...string) string {
	var b strings.Builder
	b.WriteString("<domain type='kvm'><name>lab</name><devices>")
	for i, s := range sources {
		b.WriteString("<disk type='file' device='disk'><driver name='qemu' type='qcow2'/><source file='" + s + "'/><target dev='vd" + string(rune('a'+i)) + "'/></disk>")
	}
	b.WriteString("</devices></domain>")
	return b.String()
}

type chainRig struct {
	svc    *api.Service
	engine *fakeResticEngine
	root   string
	def    func() map[string]any
}

func newChainRig(t *testing.T, xml string) *chainRig {
	t.Helper()
	st := newMemStore(t)
	settings := mustSettings(t, st)
	settings.VMsPath = "backups/vms"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	engine := &fakeResticEngine{}
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: root, HostSourceRoot: "/mnt"}
	r := &chainRig{svc: api.NewService(cfg, st, &fakeServiceDocker{}, zvolTPMVirsh{domainXML: xml}, engine), engine: engine, root: root}
	r.def = func() map[string]any {
		t.Helper()
		tg, err := st.GetVMTargetByName("lab")
		if err != nil {
			t.Fatal(err)
		}
		var def map[string]any
		if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
			t.Fatal(err)
		}
		return def
	}
	return r
}

func TestBackupVMTakesTheWholeBackingChainOfEveryDisk(t *testing.T) {
	r := newChainRig(t, qcow2DomainXML("/mnt/user/vm/lab/vdisk1.final.qcow2", "/mnt/user/data/lab/vdisk2.qcow2"))
	base1 := writeQCOW2(t, r.root, "user/vm/lab/vdisk1.qcow2", "")
	os1 := writeQCOW2(t, r.root, "user/vm/lab/vdisk1.os.qcow2", "/mnt/user/vm/lab/vdisk1.qcow2")
	top1 := writeQCOW2(t, r.root, "user/vm/lab/vdisk1.final.qcow2", "vdisk1.os.qcow2")
	base2 := writeQCOW2(t, r.root, "user/data/base/vdisk2.qcow2", "")
	top2 := writeQCOW2(t, r.root, "user/data/lab/vdisk2.qcow2", "../base/vdisk2.qcow2")

	if _, err := r.svc.BackupVM(context.Background(), "lab"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	if want := []string{top1, top2, os1, base1, base2}; !slices.Equal(r.engine.lastPaths, want) {
		t.Fatalf("backed up %q, want %q", r.engine.lastPaths, want)
	}
	links, _ := r.def()["backing"].([]any)
	if len(links) != 3 {
		t.Fatalf("the definition records %d links of the chains, want 3: %v", len(links), links)
	}
	if got := links[2].(map[string]any); got["image"] != top2 || got["file"] != base2 {
		t.Fatalf("the second disk's link = %v", got)
	}
}

func TestBackupVMOfAQCOW2DiskWithoutChainTakesTheDiskAlone(t *testing.T) {
	r := newChainRig(t, qcow2DomainXML("/mnt/user/vm/lab/vdisk1.qcow2"))
	disk := writeQCOW2(t, r.root, "user/vm/lab/vdisk1.qcow2", "")

	if _, err := r.svc.BackupVM(context.Background(), "lab"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	if !slices.Equal(r.engine.lastPaths, []string{disk}) {
		t.Fatalf("backed up %q, want only the disk", r.engine.lastPaths)
	}
	def := r.def()
	for _, key := range []string{"backing", "snapshot_db"} {
		if _, ok := def[key]; ok {
			t.Fatalf("the definition of a disk without chain has %q: %v", key, def)
		}
	}
}

func TestBackupVMWhoseChainCannotBeFollowedFailsBeforeReadingAnything(t *testing.T) {
	r := newChainRig(t, qcow2DomainXML("/mnt/user/vm/lab/vdisk1.final.qcow2"))
	writeQCOW2(t, r.root, "user/vm/lab/vdisk1.final.qcow2", "vdisk1.os.qcow2")

	_, err := r.svc.BackupVM(context.Background(), "lab")

	if err == nil || !strings.Contains(err.Error(), "disk vda: /mnt/user/vm/lab/vdisk1.final.qcow2 reads through to /mnt/user/vm/lab/vdisk1.os.qcow2, which is missing") {
		t.Fatalf("err = %v, want the missing backing file named", err)
	}
	if len(r.engine.backedUp) != 0 {
		t.Fatalf("restic backed up %q", r.engine.lastPaths)
	}
}

func TestBackupVMOnUnraidKeepsTheSnapshotListOfAChainedVM(t *testing.T) {
	r := newChainRig(t, qcow2DomainXML("/mnt/user/vm/lab/vdisk1.final.qcow2"))
	writeQCOW2(t, r.root, "user/vm/lab/vdisk1.qcow2", "")
	writeQCOW2(t, r.root, "user/vm/lab/vdisk1.final.qcow2", "vdisk1.qcow2")
	r.svc.SetPlatform(platform.Unraid{})
	r.svc.SetHostSSH(&zvolTPMSSH{files: map[string][]byte{
		"/etc/libvirt/qemu/snapshotdb/lab/snapshots.db": []byte(`{"final":{}}`),
	}})

	if _, err := r.svc.BackupVM(context.Background(), "lab"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	// JSON carries the bytes as base64.
	if got := r.def()["snapshot_db"]; got != "eyJmaW5hbCI6e319" {
		t.Fatalf("snapshot_db = %v", got)
	}
}
