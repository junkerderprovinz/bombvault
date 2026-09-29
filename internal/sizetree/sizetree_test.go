package sizetree

import (
	"fmt"
	"strings"
	"testing"
)

type node struct {
	path, typ string
	size      int64
}

// listing is what restic ls hands over for a small appdata tree, parents
// first.
var listing = []node{
	{"/host", "dir", 0},
	{"/host/user", "dir", 0},
	{"/host/user/appdata", "dir", 0},
	{"/host/user/appdata/plex", "dir", 0},
	{"/host/user/appdata/plex/Library", "dir", 0},
	{"/host/user/appdata/plex/Library/cache", "dir", 0},
	{"/host/user/appdata/plex/Library/cache/a.bin", "file", 700},
	{"/host/user/appdata/plex/Library/cache/b.bin", "file", 100},
	{"/host/user/appdata/plex/Library/db.sqlite", "file", 1000},
	{"/host/user/appdata/plex/Preferences.xml", "file", 5},
}

func build(changed map[string]struct{}) Tree {
	b := NewBuilder("/host/user/appdata/plex", changed)
	for _, n := range listing {
		b.Add(n.path, n.typ, n.size)
	}
	return b.Tree()
}

func TestTreeAddsFoldersUpAndMarksWhatTheBackupAdded(t *testing.T) {
	tr := build(map[string]struct{}{"/host/user/appdata/plex/Library/db.sqlite": {}})
	root := tr.Nodes[""]
	if root.Size != 1805 || root.Files != 4 || root.Added != 1000 {
		t.Fatalf("root %+v", root)
	}
	if root.Children[0].Name != "Library" || !root.Children[0].Dir || root.Children[0].Size != 1800 {
		t.Fatalf("children %+v", root.Children)
	}
	lib := tr.Nodes["Library"]
	if lib.Children[0].Name != "db.sqlite" || lib.Children[0].Added != 1000 || lib.Children[1].Name != "cache" {
		t.Fatalf("Library %+v", lib.Children)
	}
	if cache := tr.Nodes["Library/cache"]; cache.Size != 800 || cache.Added != 0 {
		t.Fatalf("cache %+v", cache)
	}
}

func TestAFirstBackupCountsEverythingAsAdded(t *testing.T) {
	if root := build(nil).Nodes[""]; root.Added != root.Size {
		t.Fatalf("root %+v", root)
	}
}

func TestTreeOpensWhereTheDataBranches(t *testing.T) {
	b := NewBuilder("/host/user", nil)
	b.Add("/host/user/appdata", "dir", 0)
	b.Add("/host/user/appdata/app", "dir", 0)
	b.Add("/host/user/appdata/app/x", "file", 1)
	b.Add("/host/user/appdata/app/sub", "dir", 0)
	b.Add("/host/user/appdata/app/sub/y", "file", 2)
	tr := b.Tree()
	if tr.Root != "/host/user/appdata/app" {
		t.Fatalf("root %q", tr.Root)
	}
	if _, ok := tr.Nodes["sub"]; !ok {
		t.Fatalf("nodes %v", tr.Nodes)
	}
}

func TestASingleFileBackedUpAsTheRootGetsItsFolder(t *testing.T) {
	b := NewBuilder("/domains/vm/vdisk1.img", nil)
	b.Add("/domains", "dir", 0)
	b.Add("/domains/vm", "dir", 0)
	b.Add("/domains/vm/vdisk1.img", "file", 42)
	tr := b.Tree()
	if tr.Root != "/domains/vm" || tr.Nodes[""].Size != 42 || tr.Nodes[""].Children[0].Name != "vdisk1.img" {
		t.Fatalf("tree %+v", tr)
	}
}

func TestAFolderListsItsLargestEntriesAndSumsTheRest(t *testing.T) {
	b := NewBuilder("/r", nil)
	for i := range 30 {
		b.Add(fmt.Sprintf("/r/d%02d", i), "dir", 0)
		b.Add(fmt.Sprintf("/r/d%02d/f", i), "file", int64(100+i))
	}
	for i := range 8 {
		b.Add(fmt.Sprintf("/r/small%d", i), "file", 1)
	}
	root := b.Tree().Nodes[""]
	if len(root.Children) != PerFolder || root.Children[0].Name != "d29" {
		t.Fatalf("children %+v", root.Children)
	}
	var sum int64
	for _, c := range root.Children {
		sum += c.Size
	}
	if root.Other == nil || sum+root.Other.Size != root.Size || root.Other.Count != 18 {
		t.Fatalf("other %+v, listed %d of %d", root.Other, sum, root.Size)
	}
}

func TestDeepFilesCountIntoTheDeepestKeptFolder(t *testing.T) {
	b := NewBuilder("/r", nil)
	p := "/r"
	for i := range MaxDepth + 3 {
		p += fmt.Sprintf("/l%d", i)
		b.Add(p, "dir", 0)
	}
	b.Add(p+"/deep.bin", "file", 9)
	tr := b.Tree()
	if !strings.HasSuffix(tr.Root, fmt.Sprintf("/l%d", MaxDepth-1)) || len(tr.Nodes) != 1 {
		t.Fatalf("root %q with %d nodes", tr.Root, len(tr.Nodes))
	}
	if n := tr.Nodes[""]; n.Size != 9 || n.Other == nil || n.Other.Size != 9 {
		t.Fatalf("node %+v", n)
	}
}

func TestTheTreeStopsAtMaxNodes(t *testing.T) {
	b := NewBuilder("/r", nil)
	for i := range 20 {
		b.Add(fmt.Sprintf("/r/a%d", i), "dir", 0)
		for j := range 20 {
			b.Add(fmt.Sprintf("/r/a%d/b%d", i, j), "dir", 0)
			for k := range 20 {
				d := fmt.Sprintf("/r/a%d/b%d/c%d", i, j, k)
				b.Add(d, "dir", 0)
				b.Add(d+"/f", "file", int64(i*400+j*20+k+1))
			}
		}
	}
	if n := len(b.Tree().Nodes); n != MaxNodes {
		t.Fatalf("%d nodes", n)
	}
}

func TestCommonRootIsTheDeepestSharedFolder(t *testing.T) {
	for _, c := range []struct {
		in   []string
		want string
	}{
		{[]string{"/a/b/c"}, "/a/b/c"},
		{[]string{"/a/b/c", "/a/b/d"}, "/a/b"},
		{[]string{"/a/bc", "/a/b"}, "/a"},
		{[]string{"/x", "/y"}, "/"},
	} {
		if got := CommonRoot(c.in); got != c.want {
			t.Errorf("%v: %q, want %q", c.in, got, c.want)
		}
	}
}
