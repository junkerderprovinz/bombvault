package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOnePlaceHasOneNameUnderEverySpelling(t *testing.T) {
	root := filepath.ToSlash(t.TempDir())
	places := places{
		root: pathElements(filepath.ToSlash(realPath(root))),
		remotes: rcloneOptions("[box]\ntype = box\nroot_folder_id = 0\ntoken = {\"a\":1}\n\n" +
			"[box2]\ntype = box\nroot_folder_id = 0\ntoken = {\"b\":2}\n\n" +
			"[other]\ntype = box\nroot_folder_id = 7\n\n" +
			"[short]\ntype = alias\nremote = box:bv\n\n" +
			"[both]\ntype = union\nupstreams = box:a other:b\n"),
	}
	name := func(loc string) place {
		t.Helper()
		p, ok := places.of(loc)
		if !ok {
			t.Fatalf("%s has no name", loc)
		}
		return p
	}

	for _, pair := range [][2]string{
		{root + "/user/backups/vms", root + "/user0/backups/vms"},
		{root + "/user/backups/vms", root + "/disk3/backups/vms"},
		{root + "/cache/backups/vms", root + "/disk1/backups/vms"},
		{"rclone:box:bv/vms", "rclone:box2:bv/vms"},
		{"rclone:short:vms", "rclone:box:bv/vms"},
		{"rest:https://bv:one@box:8000/bv", "rest:https://box:8000/bv"},
	} {
		if !name(pair[0]).same(name(pair[1])) {
			t.Errorf("%s and %s name two places", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]string{
		{root + "/user/backups/vms", root + "/user/backups/files"},
		{"rclone:box:bv/vms", "rclone:other:bv/vms"},
		{"rclone:box:bv/vms", "rclone:box:bv/files"},
	} {
		if name(pair[0]).same(name(pair[1])) {
			t.Errorf("%s and %s name one place", pair[0], pair[1])
		}
	}
	if !name("rclone:box:bv").overlaps(name("rclone:box2:bv/vms")) {
		t.Error("a folder of a remote does not lie inside the remote's folder under its second name")
	}
	for _, loc := range []string{"rclone:unknown:bv", "rclone:both:bv"} {
		if _, ok := places.of(loc); ok {
			t.Errorf("%s is named although nothing says where it lies", loc)
		}
	}
}

func TestADirectoryAndASymlinkToItAreOnePlace(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "backups", "vms")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, "backups"), link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	var places places
	a, _ := places.of(dir)
	b, _ := places.of(filepath.Join(link, "vms"))
	if !a.same(b) {
		t.Fatalf("%s and its symlinked spelling name two places: %+v and %+v", dir, a, b)
	}
}
