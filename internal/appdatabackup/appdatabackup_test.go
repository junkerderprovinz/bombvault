package appdatabackup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

type entry struct {
	name string
	body string
	dir  bool
	link string
}

// writeArchive builds an archive the way the plugin does: tar -c -P over the
// absolute host paths of a container's volumes, compressed by extension.
func writeArchive(t *testing.T, file string, entries []entry) {
	t.Helper()
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	mod := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: 0o644, ModTime: mod, Typeflag: tar.TypeReg, Size: int64(len(e.body))}
		switch {
		case e.dir:
			h.Typeflag, h.Mode, h.Size = tar.TypeDir, 0o755, 0
		case e.link != "":
			h.Typeflag, h.Linkname, h.Size = tar.TypeLink, e.link, 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	switch {
	case strings.HasSuffix(file, ".gz"):
		gz := gzip.NewWriter(&out)
		_, _ = gz.Write(raw.Bytes())
		_ = gz.Close()
	case strings.HasSuffix(file, ".zst"):
		zw, _ := zstd.NewWriter(&out)
		_, _ = zw.Write(raw.Bytes())
		_ = zw.Close()
	default:
		out = raw
	}
	if err := os.WriteFile(file, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mkBackup(t *testing.T, root, folder string, files map[string][]entry, extra ...string) string {
	t.Helper()
	dir := filepath.Join(root, folder)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, entries := range files {
		writeArchive(t, filepath.Join(dir, name), entries)
	}
	for _, name := range extra {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("<Container/>"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestScanFindsEveryContainerArchiveOfEveryRun(t *testing.T) {
	root := t.TempDir()
	plex := []entry{{name: "/mnt/user/appdata/plex/Preferences.xml", body: "x"}}
	mkBackup(t, root, "ab_20250304_010203", map[string][]entry{
		"plex.tar.zst":        plex,
		"sonarr.tar.gz":       plex,
		"mariadb.tar":         plex,
		"extra_files.tar.zst": plex,
	}, "my-plex.xml", "config.json", "backup.log", "vm_meta.tgz")
	mkBackup(t, root, "ab_20250101_000000", map[string][]entry{"plex.tar": plex})
	if err := os.MkdirAll(filepath.Join(root, "not_a_backup"), 0o750); err != nil {
		t.Fatal(err)
	}

	got, err := Scan(root, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Folder != "ab_20250101_000000" || got[1].Folder != "ab_20250304_010203" {
		t.Fatalf("backups = %+v, want the two runs oldest first", got)
	}
	if !got[1].Time.Equal(time.Date(2025, 3, 4, 1, 2, 3, 0, time.UTC)) {
		t.Fatalf("time = %v", got[1].Time)
	}
	var names []string
	for _, a := range got[1].Archives {
		names = append(names, a.Container+":"+a.File)
	}
	if strings.Join(names, " ") != "mariadb:mariadb.tar plex:plex.tar.zst sonarr:sonarr.tar.gz" {
		t.Fatalf("archives = %v, want the containers without extra_files", names)
	}
	if xml, ok := got[1].Template("plex"); !ok || xml != "<Container/>" {
		t.Fatalf("template = %q %v", xml, ok)
	}
	if _, ok := got[1].Template("sonarr"); ok {
		t.Fatal("a template that was not saved was found")
	}
}

func TestScanAcceptsASingleBackupFolder(t *testing.T) {
	dir := mkBackup(t, t.TempDir(), "ab_20250304_010203", map[string][]entry{"plex.tar": {{name: "/mnt/a", body: "x"}}})
	got, err := Scan(dir, time.UTC)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v %v", got, err)
	}
}

func hostToStaging(p string) (string, bool) {
	rest, ok := strings.CutPrefix(p, "/mnt/")
	if !ok {
		return "", false
	}
	return "/host/user/" + rest, true
}

func TestExtractMapsHostPathsForEveryCompression(t *testing.T) {
	for _, ext := range []string{".tar", ".tar.gz", ".tar.zst"} {
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "plex"+ext)
			writeArchive(t, file, []entry{
				{name: "/mnt/user/appdata/plex/", dir: true},
				{name: "/mnt/user/appdata/plex/Library/db.sqlite", body: "database"},
				{name: "/mnt/user/appdata/plex/Library/copy.sqlite", link: "/mnt/user/appdata/plex/Library/db.sqlite"},
				{name: "/boot/config/elsewhere.cfg", body: "outside"},
			})
			dest := filepath.Join(dir, "staging")
			var read int64
			st, err := Extract(context.Background(), file, dest, hostToStaging, func(n int64) { read = n })
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(dest, "host", "user", "user", "appdata", "plex", "Library", "db.sqlite")) //nolint:gosec // G304: inside the test's temp folder
			if err != nil || string(got) != "database" {
				t.Fatalf("db = %q %v", got, err)
			}
			b, err := os.ReadFile(filepath.Join(dest, "host", "user", "user", "appdata", "plex", "Library", "copy.sqlite")) //nolint:gosec // G304: inside the test's temp folder
			if err != nil || string(b) != "database" {
				t.Fatalf("hard link = %q %v", b, err)
			}
			if st.Files != 2 || st.Skipped != 1 {
				t.Fatalf("stats = %+v, want two files and the path outside the mount skipped", st)
			}
			info, _ := os.Stat(file)
			if read != info.Size() {
				t.Fatalf("progress ended at %d of %d bytes", read, info.Size())
			}
			fi, err := os.Stat(filepath.Join(dest, "host", "user", "user", "appdata", "plex", "Library", "db.sqlite"))
			if err != nil || !fi.ModTime().Equal(time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)) {
				t.Fatalf("time = %v %v", fi, err)
			}
		})
	}
}

func TestExtractCannotLeaveTheStagingFolder(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "evil.tar")
	writeArchive(t, file, []entry{
		{name: "/mnt/../../../../escaped.txt", body: "no"},
		{name: "../../mnt/user/x/../../../../escaped2.txt", body: "no"},
		{name: "/mnt/user/ok.txt", body: "yes"},
	})
	dest := filepath.Join(dir, "staging")
	if _, err := Extract(context.Background(), file, dest, hostToStaging, nil); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(dir, "escaped.txt"), filepath.Join(filepath.Dir(dir), "escaped.txt"), filepath.Join(dir, "escaped2.txt")} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("%s was written outside the staging folder", p)
		}
	}
	err := filepath.WalkDir(dest, func(p string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasPrefix(p, dest) {
			t.Errorf("%s is outside %s", p, dest)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestExtractOfAnArchiveWithNothingToMapFails(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "odd.tar")
	writeArchive(t, file, []entry{{name: "/opt/app/data.bin", body: "x"}})
	_, err := Extract(context.Background(), file, filepath.Join(dir, "s"), hostToStaging, nil)
	if !errors.Is(err, ErrOutsideSource) {
		t.Fatalf("err = %v, want ErrOutsideSource", err)
	}
}
