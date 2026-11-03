package qcow2_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/qcow2"
)

// image writes a version 3 qcow2 header that names backing, recording
// format for it when that is set.
func image(t *testing.T, file, backing, format string) {
	t.Helper()
	b := make([]byte, 512)
	be := binary.BigEndian
	be.PutUint32(b[0:], 0x514649fb)
	be.PutUint32(b[4:], 3)
	be.PutUint32(b[20:], 16)
	be.PutUint32(b[100:], 104)
	off := 104
	if format != "" {
		be.PutUint32(b[off:], 0xe2792aca)
		be.PutUint32(b[off+4:], uint32(len(format))) //nolint:gosec // G115: a short test string
		copy(b[off+8:], format)
		off += 8 + (len(format)+7)&^7
	}
	off += 8
	if backing != "" {
		be.PutUint64(b[8:], uint64(off))           //nolint:gosec // G115: a small offset
		be.PutUint32(b[16:], uint32(len(backing))) //nolint:gosec // G115: a short test string
		copy(b[off:], backing)
	}
	write(t, file, b)
}

func write(t *testing.T, file string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// host is a test's Host Data mount: /mnt on the VM host, root here.
type host struct{ root string }

func newHost(t *testing.T) host { return host{root: t.TempDir()} }

func (h host) local(p string) (string, bool) {
	rest, ok := strings.CutPrefix(p, "/mnt/")
	if !ok {
		return "", false
	}
	return filepath.Join(h.root, filepath.FromSlash(rest)), true
}

func (h host) at(p string) string {
	l, _ := h.local(p)
	return l
}

func TestChainFollowsRelativeAndAbsoluteNamesToTheBase(t *testing.T) {
	h := newHost(t)
	image(t, h.at("/mnt/user/vm/win/base.qcow2"), "", "")
	image(t, h.at("/mnt/user/vm/win/os.qcow2"), "/mnt/user/vm/win/base.qcow2", "qcow2")
	image(t, h.at("/mnt/user/vm/win/final.qcow2"), "os.qcow2", "qcow2")
	image(t, h.at("/mnt/user/vm/win/active.qcow2"), "final.qcow2", "")

	links, err := qcow2.Chain("/mnt/user/vm/win/active.qcow2", h.local)
	if err != nil {
		t.Fatal(err)
	}
	want := []qcow2.Link{
		{Image: h.at("/mnt/user/vm/win/active.qcow2"), File: h.at("/mnt/user/vm/win/final.qcow2"), Format: "qcow2"},
		{Image: h.at("/mnt/user/vm/win/final.qcow2"), File: h.at("/mnt/user/vm/win/os.qcow2"), Format: "qcow2"},
		{Image: h.at("/mnt/user/vm/win/os.qcow2"), File: h.at("/mnt/user/vm/win/base.qcow2"), Format: "qcow2"},
	}
	if !slices.Equal(links, want) {
		t.Fatalf("chain = %+v, want %+v", links, want)
	}
}

func TestChainResolvesARelativeNameAgainstTheImagesOwnFolder(t *testing.T) {
	h := newHost(t)
	image(t, h.at("/mnt/user/templates/debian.qcow2"), "", "")
	image(t, h.at("/mnt/user/vm/deb/vdisk1.qcow2"), "../../templates/debian.qcow2", "qcow2")

	links, err := qcow2.Chain("/mnt/user/vm/deb/vdisk1.qcow2", h.local)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].File != h.at("/mnt/user/templates/debian.qcow2") {
		t.Fatalf("chain = %+v", links)
	}
}

func TestChainOfAnImageWithoutBackingFileIsEmpty(t *testing.T) {
	h := newHost(t)
	image(t, h.at("/mnt/user/vm/plain/vdisk1.qcow2"), "", "")
	links, err := qcow2.Chain("/mnt/user/vm/plain/vdisk1.qcow2", h.local)
	if err != nil || len(links) != 0 {
		t.Fatalf("chain = %+v, %v", links, err)
	}
}

func TestChainEndsAtARawBackingFile(t *testing.T) {
	h := newHost(t)
	write(t, h.at("/mnt/user/vm/r/base.img"), bytes.Repeat([]byte{0}, 512))
	image(t, h.at("/mnt/user/vm/r/top.qcow2"), "base.img", "")

	links, err := qcow2.Chain("/mnt/user/vm/r/top.qcow2", h.local)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Format != "raw" {
		t.Fatalf("chain = %+v", links)
	}
}

func TestChainThatCannotBeFollowedToItsEndFails(t *testing.T) {
	for name, tc := range map[string]struct {
		arrange func(h host)
		want    string
	}{
		"a backing file is missing": {
			arrange: func(h host) {
				image(t, h.at("/mnt/user/vm/x/final.qcow2"), "os.qcow2", "qcow2")
				image(t, h.at("/mnt/user/vm/x/active.qcow2"), "final.qcow2", "qcow2")
			},
			want: "/mnt/user/vm/x/final.qcow2 reads through to /mnt/user/vm/x/os.qcow2, which is missing",
		},
		"the chain loops": {
			arrange: func(h host) {
				image(t, h.at("/mnt/user/vm/x/a.qcow2"), "/mnt/user/vm/x/b.qcow2", "qcow2")
				image(t, h.at("/mnt/user/vm/x/b.qcow2"), "c.qcow2", "qcow2")
				image(t, h.at("/mnt/user/vm/x/c.qcow2"), "a.qcow2", "qcow2")
				image(t, h.at("/mnt/user/vm/x/active.qcow2"), "a.qcow2", "qcow2")
			},
			want: "loops back to /mnt/user/vm/x/a.qcow2",
		},
		"a backing file lies outside the mount": {
			arrange: func(h host) {
				image(t, h.at("/mnt/user/vm/x/active.qcow2"), "/var/lib/libvirt/images/base.qcow2", "qcow2")
			},
			want: "/var/lib/libvirt/images/base.qcow2, which lies outside the Host Data mount",
		},
		"the backing file is a network block device": {
			arrange: func(h host) {
				image(t, h.at("/mnt/user/vm/x/active.qcow2"), "nbd://nas:10809/base", "raw")
			},
			want: `names its backing file as "nbd://nas:10809/base"`,
		},
		"a backing file is in a format with chains of its own": {
			arrange: func(h host) {
				write(t, h.at("/mnt/user/vm/x/base.vmdk"), []byte("KDMV"))
				image(t, h.at("/mnt/user/vm/x/active.qcow2"), "base.vmdk", "vmdk")
			},
			want: "a vmdk image",
		},
		"a header is cut short": {
			arrange: func(h host) {
				write(t, h.at("/mnt/user/vm/x/os.qcow2"), []byte{0x51, 0x46, 0x49, 0xfb, 0, 0, 0, 3})
				image(t, h.at("/mnt/user/vm/x/active.qcow2"), "os.qcow2", "qcow2")
			},
			want: "read the header of /mnt/user/vm/x/os.qcow2: the qcow2 header is cut short",
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHost(t)
			tc.arrange(h)
			links, err := qcow2.Chain("/mnt/user/vm/x/active.qcow2", h.local)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("chain = %+v, err = %v, want an error saying %q", links, err, tc.want)
			}
		})
	}
}

func TestReadHeaderTellsAFileThatIsNoQCOW2(t *testing.T) {
	h := newHost(t)
	write(t, h.at("/mnt/user/vm/r/vdisk1.img"), []byte("a raw disk"))
	if _, err := qcow2.ReadHeader(h.at("/mnt/user/vm/r/vdisk1.img")); !errors.Is(err, qcow2.ErrNotQCOW2) {
		t.Fatalf("err = %v, want ErrNotQCOW2", err)
	}
}

func TestReadHeaderOfAVersion2ImageWithTheNameRightAfterTheHeader(t *testing.T) {
	b := make([]byte, 512)
	be := binary.BigEndian
	be.PutUint32(b[0:], 0x514649fb)
	be.PutUint32(b[4:], 2)
	be.PutUint64(b[8:], 72)
	be.PutUint32(b[16:], 9)
	be.PutUint32(b[20:], 16)
	copy(b[72:], "base.img2")
	h := newHost(t)
	write(t, h.at("/mnt/user/vm/old/top.qcow2"), b)

	hd, err := qcow2.ReadHeader(h.at("/mnt/user/vm/old/top.qcow2"))
	if err != nil || hd.BackingFile != "base.img2" || hd.BackingFormat != "" {
		t.Fatalf("header = %+v, %v", hd, err)
	}
}

func TestRepointLeadsARestoredImageToTheRestoredBackingFile(t *testing.T) {
	for name, tc := range map[string]struct {
		stored, host, backing, want string
	}{
		"an absolute name gets the new path": {
			stored: "/mnt/user/vm/win/final.qcow2", host: "/mnt/user/vm/win-restored/active.qcow2",
			backing: "/mnt/user/vm/win-restored/final.qcow2", want: "/mnt/user/vm/win-restored/final.qcow2",
		},
		"a relative name stays relative": {
			stored: "../../templates/debian.qcow2", host: "/mnt/user/vm-restored/deb/vdisk1.qcow2",
			backing: "/mnt/user/templates-restored/debian.qcow2", want: "../../templates-restored/debian.qcow2",
		},
		"a name that leads there already is kept": {
			stored: "final.qcow2", host: "/mnt/user/vm/win-restored/active.qcow2",
			backing: "/mnt/user/vm/win-restored/final.qcow2", want: "final.qcow2",
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHost(t)
			file := h.at(tc.host)
			image(t, file, tc.stored, "qcow2")
			if err := qcow2.Repoint(file, tc.host, tc.backing); err != nil {
				t.Fatal(err)
			}
			hd, err := qcow2.ReadHeader(file)
			if err != nil || hd.BackingFile != tc.want || hd.BackingFormat != "qcow2" {
				t.Fatalf("header = %+v, %v, want backing file %q", hd, err, tc.want)
			}
		})
	}
}

func TestSetBackingFileWithAShorterNameLeavesNothingOfTheOldOne(t *testing.T) {
	h := newHost(t)
	file := h.at("/mnt/user/vm/x/active.qcow2")
	image(t, file, "/mnt/user/vm/a-rather-long-folder-name/final.qcow2", "qcow2")
	if err := qcow2.SetBackingFile(file, "final.qcow2"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(file) //nolint:gosec // G304: the test's own file
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("long-folder")) {
		t.Fatal("the old name is still in the header")
	}
	if hd, err := qcow2.ReadHeader(file); err != nil || hd.BackingFile != "final.qcow2" {
		t.Fatalf("header = %+v, %v", hd, err)
	}
}

func TestSetBackingFileRefusesANameTooLongForQemu(t *testing.T) {
	h := newHost(t)
	file := h.at("/mnt/user/vm/x/active.qcow2")
	image(t, file, "final.qcow2", "qcow2")
	if err := qcow2.SetBackingFile(file, "/"+strings.Repeat("a", 1100)); err == nil {
		t.Fatal("a 1101 byte name was written")
	}
}
