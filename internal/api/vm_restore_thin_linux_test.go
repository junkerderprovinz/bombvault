package api

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// allocated is how many bytes of file take room on disk.
func allocated(t *testing.T, file string) int64 {
	t.Helper()
	var st syscall.Stat_t
	if err := syscall.Stat(file, &st); err != nil {
		t.Fatal(err)
	}
	return st.Blocks * 512
}

func TestRestoreVMKeepsAThinDiskThin(t *testing.T) {
	if _, err := exec.LookPath("fallocate"); err != nil {
		t.Skip("no fallocate")
	}
	r := newRestoreRig(t)
	const size, room = 64 << 20, 16 << 20
	disk := filepath.Join(r.root, "domains/lab/vdisk1.img")
	r.write(t, map[string]string{"domains/lab/vdisk1.img": "thin"})
	if err := os.Truncate(disk, size); err != nil {
		t.Fatal(err)
	}
	r.virsh.name, r.virsh.state, r.virsh.xml = "lab", "shut off", labDomain("raw", "/mnt/user/domains/lab/vdisk1.img")
	tg, snap := r.backupLab(t)

	if run := r.restoreLab(t, tg, snap, RestoreOptions{}); run.Status != "success" {
		t.Fatalf("run = %+v", run)
	}
	if got := allocated(t, disk); got > room {
		t.Fatalf("restored in place, the disk takes %d bytes", got)
	}
	if run := r.restoreLab(t, tg, snap, RestoreOptions{Beside: true}); run.Status != "success" {
		t.Fatalf("run = %+v", run)
	}
	if got := allocated(t, filepath.Join(r.root, "domains/lab-restored/vdisk1.img")); got > room {
		t.Fatalf("restored beside, the copy takes %d bytes", got)
	}
}
