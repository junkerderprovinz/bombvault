package backup_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/qcow2"
)

// restoredQCOW2 stands for a qcow2 image restic has just restored: a header
// naming backing, written to a file of the test.
func restoredQCOW2(t *testing.T, backing string) string {
	t.Helper()
	b := make([]byte, 512)
	be := binary.BigEndian
	be.PutUint32(b[0:], 0x514649fb)
	be.PutUint32(b[4:], 3)
	be.PutUint32(b[20:], 16)
	be.PutUint32(b[100:], 104)
	be.PutUint64(b[8:], 112)
	be.PutUint32(b[16:], uint32(len(backing))) //nolint:gosec // G115: a short test string
	copy(b[112:], backing)
	file := filepath.Join(t.TempDir(), "active.qcow2")
	if err := os.WriteFile(file, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func backingFileOf(t *testing.T, file string) string {
	t.Helper()
	h, err := qcow2.ReadHeader(file)
	if err != nil {
		t.Fatal(err)
	}
	return h.BackingFile
}

func TestRestoreVMBesideLeadsTheCopiedChainThroughTheCopies(t *testing.T) {
	img := restoredQCOW2(t, "/mnt/user/vm/win/final.qcow2")
	r, runs := &fakeRestic{}, &fakeRuns{}

	err := backup.RestoreVMBeside(t.Context(), backup.BesideVMDeps{
		RepoPath:   "/repo",
		SnapshotID: "abcdef12",
		Dirs:       []backup.VMRestoreDir{{Subtree: "/host/vm/win", Target: "/host/vm/win-restored"}},
		MakeDirs:   func() error { return nil },
		Backing: []backup.VMBackingLink{{
			Image: img, ImageHost: "/mnt/user/vm/win-restored/active.qcow2", Backing: "/mnt/user/vm/win-restored/final.qcow2",
		}},
		Restic: r,
		Runs:   runs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := backingFileOf(t, img); got != "/mnt/user/vm/win-restored/final.qcow2" {
		t.Fatalf("the copy reads through to %q, the original's file", got)
	}
}

func TestRestoreVMInPlaceKeepsAChainThatLeadsWhereItWasBackedUp(t *testing.T) {
	img := restoredQCOW2(t, "/mnt/user/vm/win/final.qcow2")
	before, err := os.ReadFile(img) //nolint:gosec // G304: the test's own file
	if err != nil {
		t.Fatal(err)
	}
	vm, r, runs := &fakeVM{stateVal: "shut off"}, &fakeRestic{}, &fakeRuns{}
	d := sampleVMRestoreDeps(t, vm, r, runs)
	d.Backing = []backup.VMBackingLink{{Image: img, ImageHost: "/mnt/user/vm/win/active.qcow2", Backing: "/mnt/user/vm/win/final.qcow2"}}

	if err := backup.RestoreVM(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(img); !bytes.Equal(after, before) { //nolint:gosec // G304: the test's own file
		t.Fatal("a restore in place rewrote the image's header")
	}
}

func TestRestoreVMThatCannotRepointTheChainDefinesNoVM(t *testing.T) {
	notQCOW2 := filepath.Join(t.TempDir(), "active.qcow2")
	if err := os.WriteFile(notQCOW2, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	vm, r, runs := &fakeVM{stateVal: "shut off"}, &fakeRestic{}, &fakeRuns{}
	d := sampleVMRestoreDeps(t, vm, r, runs)
	d.Backing = []backup.VMBackingLink{{Image: notQCOW2, ImageHost: "/mnt/pool/win/active.qcow2", Backing: "/mnt/pool/win/final.qcow2"}}

	err := backup.RestoreVM(t.Context(), d)

	if err == nil || !strings.Contains(err.Error(), "point /mnt/pool/win/active.qcow2 at its backing file") {
		t.Fatalf("err = %v", err)
	}
	if slices.ContainsFunc(vm.log, func(e string) bool { return strings.HasPrefix(e, "define:") }) {
		t.Fatalf("a VM was defined on a chain that leads into the original's files: %v", vm.log)
	}
}
