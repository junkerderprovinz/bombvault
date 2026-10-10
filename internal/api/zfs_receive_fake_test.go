package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/junkerderprovinz/bombvault/internal/sshconn"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// fakePool is a host that owns ZFS pools and answers the argv the zfs builders
// emit, over the two transports an instance reaches its host by: the commands
// a zfs.SSHHost runs and the streams of a HostSSH. A stream is a begin record,
// one write and an end record, so a cut one shows.
type fakePool struct {
	HostSSH
	guids *atomic.Uint64

	mu    sync.Mutex
	txg   uint64
	ds    map[string]*fakeDataset
	calls [][]string
	// free is what each pool of ten terabytes has left, half of it where it
	// is not set.
	free map[string]int64
}

type fakeDataset struct {
	typ   string
	snaps []fakeSnap
	marks []fakeSnap
	holds map[string]bool
	props map[string]string
}

type fakeSnap struct {
	name      string
	guid, txg uint64
}

func newFakePool(guids *atomic.Uint64, datasets ...string) *fakePool {
	p := &fakePool{guids: guids, ds: map[string]*fakeDataset{}, free: map[string]int64{}}
	for _, d := range datasets {
		p.ds[d] = newFakeDataset("filesystem")
	}
	return p
}

func newFakeDataset(typ string) *fakeDataset {
	return &fakeDataset{typ: typ, holds: map[string]bool{}, props: map[string]string{}}
}

func (p *fakePool) dataset(name string) *fakeDataset {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ds[name]
}

func (p *fakePool) snapNames(name string) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	if d := p.ds[name]; d != nil {
		for _, s := range d.snaps {
			out = append(out, s.name)
		}
	}
	return out
}

func (p *fakePool) callsOf(sub string) [][]string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out [][]string
	for _, c := range p.calls {
		if len(c) > 1 && c[1] == sub {
			out = append(out, c)
		}
	}
	return out
}

type fakeExit int

func (e fakeExit) Error() string { return fmt.Sprintf("exit status %d", int(e)) }
func (e fakeExit) ExitCode() int { return int(e) }

func (p *fakePool) RunCapture(_ context.Context, args ...string) (string, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, slices.Clone(args))
	out, stderr := p.run(args)
	if stderr != "" {
		return "", stderr, fakeExit(1)
	}
	return out, "", nil
}

func missingDataset(name string) string { return "cannot open '" + name + "': dataset does not exist" }

func (p *fakePool) run(args []string) (string, string) {
	last := args[len(args)-1]
	switch args[1] {
	case "list":
		if slices.Equal(args, zfs.PoolsArgs()) {
			return p.pools(), ""
		}
		if slices.Contains(args, "-r") {
			return p.tree(last)
		}
		return p.points(last)
	case "get":
		d := p.ds[last]
		if d == nil {
			return "", missingDataset(last)
		}
		if slices.Contains(args, "local") {
			if v, ok := d.props[zfs.SourceProperty]; ok {
				return v + "\n", ""
			}
			return "-\n", ""
		}
		return "type\t" + d.typ + "\nencryption\toff\nreceive_resume_token\t-\n", ""
	case "snapshot":
		root, snap, _ := strings.Cut(last, "@")
		if p.ds[root] == nil {
			return "", missingDataset(root)
		}
		p.txg++
		for name, d := range p.ds {
			if name == root || zfs.DescendantOf(name, root) {
				d.snaps = append(d.snaps, fakeSnap{snap, p.guids.Add(1000), p.txg})
			}
		}
		return "", ""
	case "create":
		if p.ds[last] != nil {
			return "", ""
		}
		parent := last[:strings.LastIndexByte(last, '/')]
		if p.ds[parent] == nil {
			return "", "cannot create '" + last + "': parent does not exist"
		}
		d := newFakeDataset("filesystem")
		for i, a := range args {
			if a == "-o" {
				k, v, _ := strings.Cut(args[i+1], "=")
				d.props[k] = v
			}
		}
		p.ds[last] = d
		return "", ""
	case "bookmark":
		ds, snap, _ := strings.Cut(args[2], "@")
		i := slices.IndexFunc(p.ds[ds].snaps, func(s fakeSnap) bool { return s.name == snap })
		if slices.ContainsFunc(p.ds[ds].marks, func(s fakeSnap) bool { return s.name == snap }) {
			return "", "cannot create bookmark: bookmark exists"
		}
		p.ds[ds].marks = append(p.ds[ds].marks, p.ds[ds].snaps[i])
		return "", ""
	case "hold", "release":
		ds, snap, _ := strings.Cut(last, "@")
		d := p.ds[ds]
		if args[1] == "hold" {
			d.holds[snap] = true
			return "", ""
		}
		if !d.holds[snap] {
			return "", "cannot release hold: no such tag on this dataset"
		}
		delete(d.holds, snap)
		return "", ""
	case "destroy":
		ds, snap, _ := strings.Cut(last, "@")
		d := p.ds[ds]
		if d == nil || !slices.ContainsFunc(d.snaps, func(s fakeSnap) bool { return s.name == snap }) {
			return "", "could not find any snapshots to destroy; check snapshot names."
		}
		if d.holds[snap] {
			return "", "cannot destroy snapshot: dataset is busy"
		}
		d.snaps = slices.DeleteFunc(d.snaps, func(s fakeSnap) bool { return s.name == snap })
		return "", ""
	case "send":
		return "size\t4096\n", ""
	}
	return "", fmt.Sprintf("fake: unknown command %q", args)
}

func (p *fakePool) tree(root string) (string, string) {
	if p.ds[root] == nil {
		return "", missingDataset(root)
	}
	var names []string
	for name := range p.ds {
		if name == root || zfs.DescendantOf(name, root) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "%s\t%s\t/mnt/%s\tyes\ton\toff\t-\thidden\t1024\t1024\n", name, p.ds[name].typ, name)
	}
	return b.String(), ""
}

func (p *fakePool) pools() string {
	var b strings.Builder
	for name := range p.ds {
		if strings.Contains(name, "/") {
			continue
		}
		free, ok := p.free[name]
		if !ok {
			free = 5 << 40
		}
		fmt.Fprintf(&b, "%s\t%d\t%d\n", name, 10<<40-free, free)
	}
	return b.String()
}

func (p *fakePool) points(name string) (string, string) {
	d := p.ds[name]
	if d == nil {
		return "", missingDataset(name)
	}
	var b strings.Builder
	for _, s := range d.snaps {
		fmt.Fprintf(&b, "%s@%s\t%d\t%d\n", name, s.name, s.guid, s.txg)
	}
	for _, s := range d.marks {
		fmt.Fprintf(&b, "%s#%s\t%d\t%d\n", name, s.name, s.guid, s.txg)
	}
	return b.String(), ""
}

// streamRecord is the leading record of a stream of snapshot name with the
// two guids, as a little-endian host writes it.
func streamRecord(name string, to, from uint64, volume bool) []byte {
	b := make([]byte, 312)
	binary.LittleEndian.PutUint64(b[8:], 0x2F5bacbac)
	binary.LittleEndian.PutUint64(b[16:], 17<<2|1)
	objset := uint32(2)
	if volume {
		objset = 3
	}
	binary.LittleEndian.PutUint32(b[32:], objset)
	binary.LittleEndian.PutUint64(b[40:], to)
	binary.LittleEndian.PutUint64(b[48:], from)
	copy(b[56:], name)
	return b
}

// fakeRecord is a little-endian record header of type typ whose write size,
// where it has one, is size.
func fakeRecord(typ uint32, size uint64) []byte {
	b := make([]byte, 312)
	binary.LittleEndian.PutUint32(b, typ)
	binary.LittleEndian.PutUint64(b[32:], size)
	return b
}

// fakeStream is the whole stream that begin starts.
func fakeStream(begin []byte) []byte {
	return slices.Concat(begin, fakeRecord(3, 4096), bytes.Repeat([]byte("x"), 4096), fakeRecord(5, 0))
}

func (p *fakePool) StreamCommand(ctx context.Context, args ...string) (io.ReadCloser, func() error, error) {
	p.mu.Lock()
	p.calls = append(p.calls, slices.Clone(args))
	record, stderr := p.sendRecord(args)
	p.mu.Unlock()

	pr, pw := io.Pipe()
	done := make(chan error, 1)
	if stderr != "" {
		_ = pw.Close()
		done <- &sshconn.RemoteError{Stderr: stderr, Err: fakeExit(1)}
		return pr, func() error { return <-done }, nil
	}
	stop := context.AfterFunc(ctx, func() { _ = pw.CloseWithError(ctx.Err()) })
	go func() {
		defer stop()
		if _, err := pw.Write(fakeStream(record)); err != nil {
			done <- &sshconn.RemoteError{Stderr: "killed", Err: fakeExit(255)}
			return
		}
		_ = pw.Close()
		done <- nil
	}()
	return pr, func() error { return <-done }, nil
}

func (p *fakePool) sendRecord(args []string) ([]byte, string) {
	full := args[len(args)-1]
	ds, snap, _ := strings.Cut(full, "@")
	d := p.ds[ds]
	if d == nil {
		return nil, missingDataset(ds)
	}
	i := slices.IndexFunc(d.snaps, func(s fakeSnap) bool { return s.name == snap })
	if i < 0 {
		return nil, missingDataset(full)
	}
	var from uint64
	if j := slices.Index(args, "-i"); j >= 0 {
		base, pool := args[j+1][1:], d.snaps
		if args[j+1][0] == '#' {
			pool = d.marks
		}
		k := slices.IndexFunc(pool, func(s fakeSnap) bool { return s.name == base })
		if k < 0 {
			return nil, "incremental source (" + args[j+1] + ") does not exist"
		}
		from = pool[k].guid
	}
	return streamRecord(full, d.snaps[i].guid, from, d.typ == "volume"), ""
}

func (p *fakePool) RunWithStdin(_ context.Context, rd io.Reader, args ...string) error {
	p.mu.Lock()
	p.calls = append(p.calls, slices.Clone(args))
	p.mu.Unlock()
	br := bufio.NewReader(rd)
	begin, err := zfs.ReadStreamBegin(br)
	if err != nil {
		return &sshconn.RemoteError{Stderr: "cannot receive: invalid stream (bad magic number)", Err: fakeExit(1)}
	}
	body, err := io.ReadAll(br)
	if err != nil || !bytes.HasSuffix(body, fakeRecord(5, 0)) {
		return &sshconn.RemoteError{Stderr: "cannot receive: checksum mismatch or incomplete stream", Err: fakeExit(1)}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if stderr := p.receive(args, begin); stderr != "" {
		return &sshconn.RemoteError{Stderr: stderr, Err: fakeExit(1)}
	}
	return nil
}

func (p *fakePool) receive(args []string, b zfs.StreamBegin) string {
	target := args[len(args)-1]
	force := slices.Contains(args, "-F")
	d := p.ds[target]
	switch {
	case b.FromGUID == 0 && d != nil && !force:
		return "cannot receive new filesystem stream: destination '" + target + "' exists\nmust specify -F to overwrite it"
	case b.FromGUID == 0 && d != nil && len(d.snaps) > 0:
		return "cannot receive new filesystem stream: destination has snapshots"
	case b.FromGUID == 0:
		if p.ds[target[:strings.LastIndexByte(target, '/')]] == nil {
			return "cannot receive new filesystem stream: dataset does not exist"
		}
		typ := "filesystem"
		if b.Volume {
			typ = "volume"
		}
		d = newFakeDataset(typ)
		p.ds[target] = d
	case d == nil:
		return "cannot receive incremental stream: destination '" + target + "' does not exist"
	default:
		i := slices.IndexFunc(d.snaps, func(s fakeSnap) bool { return s.guid == b.FromGUID })
		switch {
		case i < 0:
			return "cannot receive incremental stream: most recent snapshot of " + target + " does not\nmatch incremental source"
		case force:
			d.snaps = d.snaps[:i+1]
		case i != len(d.snaps)-1:
			return "cannot receive incremental stream: destination " + target + " has been modified\nsince most recent snapshot"
		}
	}
	for i, a := range args {
		if a == "-o" {
			k, v, _ := strings.Cut(args[i+1], "=")
			d.props[k] = v
		}
	}
	_, snap, _ := strings.Cut(b.Snapshot, "@")
	p.txg++
	d.snaps = append(d.snaps, fakeSnap{snap, b.ToGUID, p.txg})
	return ""
}
