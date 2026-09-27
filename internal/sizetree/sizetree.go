// Package sizetree adds up a snapshot listing into the sizes of its folders,
// with what the latest backup brought in, and keeps the part of the tree a
// person would drill into.
package sizetree

import (
	"container/heap"
	"path"
	"sort"
	"strings"
)

// Limits of a tree. Files deeper than MaxDepth below the root count into the
// folder at that depth, so the work stays bounded on a deep tree.
const (
	MaxDepth   = 8
	MaxNodes   = 2000
	PerFolder  = 20
	filesKept  = 5
	MaxChanged = 500_000
)

// Entry is one child of a folder in a stored tree.
type Entry struct {
	Name  string `json:"name"`
	Dir   bool   `json:"dir,omitempty"`
	Size  int64  `json:"size"`
	Files int64  `json:"files"`
	Added int64  `json:"added"`
}

// Rest sums the children a folder lists no further.
type Rest struct {
	Count int   `json:"count"`
	Size  int64 `json:"size"`
	Files int64 `json:"files"`
	Added int64 `json:"added"`
}

// Node is one folder the tree can show. Children is nil for a folder that
// was not expanded.
type Node struct {
	Size     int64   `json:"size"`
	Files    int64   `json:"files"`
	Added    int64   `json:"added"`
	Children []Entry `json:"children,omitempty"`
	Other    *Rest   `json:"other,omitempty"`
}

// Tree is a snapshot's folders by path relative to Root, "" being Root.
type Tree struct {
	Root  string          `json:"root"`
	Nodes map[string]Node `json:"nodes"`
}

type folder struct {
	size, files, added int64
	subdirs            []string
	top                []Entry
}

// Builder takes a snapshot listing entry by entry.
type Builder struct {
	root    string
	changed map[string]struct{}
	folders map[string]*folder
}

// NewBuilder adds up the snapshot below root. changed holds the absolute
// paths of the files the latest backup added or changed; nil when there was
// no earlier backup to compare with, so every file counts as added.
func NewBuilder(root string, changed map[string]struct{}) *Builder {
	return &Builder{root: path.Clean(root), changed: changed, folders: map[string]*folder{"": {}}}
}

// CommonRoot is the deepest folder that holds every path.
func CommonRoot(paths []string) string {
	if len(paths) == 0 {
		return "/"
	}
	root := path.Clean(paths[0])
	for _, p := range paths[1:] {
		p = path.Clean(p)
		for root != "/" && p != root && !strings.HasPrefix(p, root+"/") {
			root = path.Dir(root)
		}
	}
	return root
}

// rel is p relative to the root and its depth, or false for a path outside.
func (b *Builder) rel(p string) (string, int, bool) {
	p = path.Clean(p)
	if p == b.root {
		return "", 0, true
	}
	prefix := b.root + "/"
	if b.root == "/" {
		prefix = "/"
	}
	if !strings.HasPrefix(p, prefix) {
		return "", 0, false
	}
	r := strings.TrimPrefix(p, prefix)
	return r, strings.Count(r, "/") + 1, true
}

// capDepth cuts a relative path to at most MaxDepth parts.
func capDepth(r string, depth int) string {
	for ; depth > MaxDepth; depth-- {
		r = path.Dir(r)
	}
	return r
}

func parent(r string) string {
	if d := path.Dir(r); d != "." {
		return d
	}
	return ""
}

func (b *Builder) folder(r string) *folder {
	f, ok := b.folders[r]
	if !ok {
		f = &folder{}
		b.folders[r] = f
		if r != "" {
			p := b.folder(parent(r))
			p.subdirs = append(p.subdirs, r)
		}
	}
	return f
}

// Add takes one node of the listing. restic lists a folder before what it
// holds, which is how a single file backed up as the root is recognised.
func (b *Builder) Add(p, typ string, size int64) {
	r, depth, ok := b.rel(p)
	if !ok {
		return
	}
	if r == "" {
		if typ != "dir" {
			b.root = path.Dir(b.root)
			b.Add(p, typ, size)
		}
		return
	}
	if typ == "dir" {
		if depth <= MaxDepth {
			b.folder(r)
		}
		return
	}
	added := int64(0)
	if b.changed == nil {
		added = size
	} else if _, ok := b.changed[path.Clean(p)]; ok {
		added = size
	}
	f := b.folder(capDepth(parent(r), depth-1))
	f.size += size
	f.files++
	f.added += added
	if depth-1 <= MaxDepth {
		f.keepFile(Entry{Name: path.Base(r), Size: size, Files: 1, Added: added})
	}
}

// keepFile holds on to the largest few files of a folder, which is all a
// folder's list can show next to its subfolders.
func (f *folder) keepFile(e Entry) {
	if len(f.top) == filesKept && e.Size <= f.top[filesKept-1].Size {
		return
	}
	i := sort.Search(len(f.top), func(i int) bool { return f.top[i].Size < e.Size })
	f.top = append(f.top, Entry{})
	copy(f.top[i+1:], f.top[i:])
	f.top[i] = e
	if len(f.top) > filesKept {
		f.top = f.top[:filesKept]
	}
}

// Tree adds the folders up and keeps the largest ones, up to MaxNodes. A
// chain of lone folders at the top is skipped, so the tree opens where the
// data starts to branch.
func (b *Builder) Tree() Tree {
	dirs := make([]string, 0, len(b.folders))
	for r := range b.folders {
		dirs = append(dirs, r)
	}
	sort.Slice(dirs, func(i, j int) bool { return depthOf(dirs[i]) > depthOf(dirs[j]) })
	for _, r := range dirs {
		if r == "" {
			continue
		}
		f, p := b.folders[r], b.folders[parent(r)]
		p.size += f.size
		p.files += f.files
		p.added += f.added
	}
	start := ""
	for {
		f := b.folders[start]
		if len(f.subdirs) != 1 || len(f.top) > 0 {
			break
		}
		start = f.subdirs[0]
	}
	root := b.root
	if start != "" {
		root = path.Join(b.root, start)
	}
	t := Tree{Root: root, Nodes: map[string]Node{}}
	pq := &queue{{rel: start, size: b.folders[start].size}}
	for pq.Len() > 0 && len(t.Nodes) < MaxNodes {
		it := heap.Pop(pq).(queued)
		f := b.folders[it.rel]
		n := Node{Size: f.size, Files: f.files, Added: f.added, Children: []Entry{}}
		var entries []Entry
		for _, d := range f.subdirs {
			c := b.folders[d]
			entries = append(entries, Entry{Name: path.Base(d), Dir: true, Size: c.size, Files: c.files, Added: c.added})
		}
		entries = append(entries, f.top...)
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].Size > entries[j].Size })
		var size, files, added int64
		for i, e := range entries {
			size, files, added = size+e.Size, files+e.Files, added+e.Added
			if i < PerFolder {
				n.Children = append(n.Children, e)
				if e.Dir {
					heap.Push(pq, queued{rel: join(it.rel, e.Name), size: e.Size})
				}
				continue
			}
			n.Other = n.Other.plus(Rest{Count: 1, Size: e.Size, Files: e.Files, Added: e.Added})
		}
		// The files a folder did not keep go to its rest, so the rows always
		// add up to the folder.
		if left := f.files - files; left > 0 {
			n.Other = n.Other.plus(Rest{Count: int(left), Size: f.size - size, Files: left, Added: f.added - added})
		}
		t.Nodes[relFrom(it.rel, start)] = n
	}
	return t
}

func depthOf(r string) int {
	if r == "" {
		return 0
	}
	return strings.Count(r, "/") + 1
}

func join(a, b string) string {
	if a == "" {
		return b
	}
	return a + "/" + b
}

// relFrom is r relative to start, the folder the tree opens at.
func relFrom(r, start string) string {
	if start == "" || r == start {
		return strings.TrimPrefix(r, start)
	}
	return strings.TrimPrefix(r, start+"/")
}

func (r *Rest) plus(o Rest) *Rest {
	if r == nil {
		return &o
	}
	return &Rest{Count: r.Count + o.Count, Size: r.Size + o.Size, Files: r.Files + o.Files, Added: r.Added + o.Added}
}

type queued struct {
	rel  string
	size int64
}

type queue []queued

func (q queue) Len() int           { return len(q) }
func (q queue) Less(i, j int) bool { return q[i].size > q[j].size }
func (q queue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x any)        { *q = append(*q, x.(queued)) }
func (q *queue) Pop() any {
	old := *q
	it := old[len(old)-1]
	*q = old[:len(old)-1]
	return it
}
