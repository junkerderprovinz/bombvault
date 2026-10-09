package zfsrepl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// fakeHost is a pool owner that understands the argv the zfs builders emit.
// Streams are a header line, a payload and a trailer, so a cut stream is
// visible to the receive the way an incomplete one is to zfs.
type fakeHost struct {
	mu    sync.Mutex
	world *fakeWorld
	ds    map[string]*fakeDS
	txg   uint64
	calls [][]string

	// payload is how many bytes a stream of a dataset carries.
	payload map[string]int
	// cutAfter makes the send of a dataset die after that many payload bytes.
	cutAfter map[string]int
	// failSendWait makes a complete send of a dataset report a failure anyway.
	failSendWait map[string]bool
	// refuseReceive makes a receive into a dataset fail with this stderr after
	// reading only the header, which leaves the send blocked on its pipe.
	refuseReceive map[string]string
}

// fakeWorld hands out guids that are unique across both hosts, as they are in
// practice.
type fakeWorld struct {
	mu   sync.Mutex
	guid uint64
}

func (w *fakeWorld) next() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.guid += 1000
	return w.guid
}

type fakeDS struct {
	typ       string
	encrypted bool
	snaps     []fakePoint
	marks     []fakePoint
	holds     map[string]bool
	token     string
	modified  bool
	props     map[string]string
}

type fakePoint struct {
	name string
	guid uint64
	txg  uint64
}

func newFakeHost(w *fakeWorld) *fakeHost {
	return &fakeHost{
		world:         w,
		ds:            map[string]*fakeDS{},
		payload:       map[string]int{},
		cutAfter:      map[string]int{},
		failSendWait:  map[string]bool{},
		refuseReceive: map[string]string{},
	}
}

func (h *fakeHost) add(name, typ string, encrypted bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ds[name] = &fakeDS{typ: typ, encrypted: encrypted, holds: map[string]bool{}, props: map[string]string{}}
}

// snapshot takes a snapshot outside the engine, the way a user or another tool
// would.
func (h *fakeHost) snapshot(dataset, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.txg++
	d := h.ds[dataset]
	d.snaps = append(d.snaps, fakePoint{name, h.world.next(), h.txg})
}

func (h *fakeHost) dropSnapshot(dataset, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	d := h.ds[dataset]
	d.snaps = without(d.snaps, name)
	delete(d.holds, name)
}

func (h *fakeHost) get(name string) *fakeDS {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ds[name]
}

func (h *fakeHost) snapNames(dataset string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	d := h.ds[dataset]
	if d == nil {
		return nil
	}
	var out []string
	for _, p := range d.snaps {
		out = append(out, p.name)
	}
	return out
}

func (h *fakeHost) markNames(dataset string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	for _, p := range h.ds[dataset].marks {
		out = append(out, p.name)
	}
	return out
}

func (h *fakeHost) record(args []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, append([]string(nil), args...))
}

// callsOf returns the recorded argv whose subcommand is sub.
func (h *fakeHost) callsOf(sub string) [][]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out [][]string
	for _, c := range h.calls {
		if len(c) > 1 && c[1] == sub {
			out = append(out, c)
		}
	}
	return out
}

func (h *fakeHost) resetCalls() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = nil
}

type exitStatus int

func (e exitStatus) Error() string { return "exit status " + strconv.Itoa(int(e)) }
func (e exitStatus) ExitCode() int { return int(e) }

func fail(args []string, exit int, format string, a ...any) error {
	stderr := fmt.Sprintf(format, a...)
	return &zfs.CmdError{Args: args, Stderr: stderr, Code: zfs.Classify(stderr, exitStatus(exit)), Err: exitStatus(exit)}
}

func missing(args []string, name string) error {
	return fail(args, 1, "cannot open '%s': dataset does not exist", name)
}

func (h *fakeHost) Run(_ context.Context, args []string) (string, error) {
	h.record(args)
	h.mu.Lock()
	defer h.mu.Unlock()
	last := args[len(args)-1]
	switch args[1] {
	case "list":
		if args[4] == "-r" {
			return h.tree(args, last)
		}
		return h.points(args, last)
	case "get":
		d := h.ds[last]
		if d == nil {
			return "", missing(args, last)
		}
		enc, tok := "off", "-"
		if d.encrypted {
			enc = "aes-256-gcm"
		}
		if d.token != "" {
			tok = d.token
		}
		return "type\t" + d.typ + "\nencryption\t" + enc + "\nreceive_resume_token\t" + tok + "\n", nil
	case "snapshot":
		root, snap, _ := strings.Cut(last, "@")
		if h.ds[root] == nil {
			return "", missing(args, root)
		}
		h.txg++
		for name, d := range h.ds {
			if name == root || zfs.DescendantOf(name, root) {
				if hasPoint(d.snaps, snap) {
					return "", fail(args, 1, "cannot create snapshot '%s@%s': dataset already exists", name, snap)
				}
				d.snaps = append(d.snaps, fakePoint{snap, h.world.next(), h.txg})
			}
		}
		return "", nil
	case "create":
		if h.ds[last] != nil {
			return "", nil
		}
		parent := last[:strings.LastIndexByte(last, '/')]
		if h.ds[parent] == nil {
			return "", fail(args, 1, "cannot create '%s': parent does not exist", last)
		}
		d := &fakeDS{typ: "filesystem", holds: map[string]bool{}, props: map[string]string{}}
		for i, a := range args {
			if a == "-o" {
				k, v, _ := strings.Cut(args[i+1], "=")
				d.props[k] = v
			}
		}
		h.ds[last] = d
		return "", nil
	case "bookmark":
		ds, snap, _ := strings.Cut(args[2], "@")
		d := h.ds[ds]
		p, ok := point(d.snaps, snap)
		if !ok {
			return "", fail(args, 1, "cannot create bookmark '%s': snapshot does not exist", args[3])
		}
		if hasPoint(d.marks, snap) {
			return "", fail(args, 1, "cannot create bookmark '%s': bookmark exists", args[3])
		}
		d.marks = append(d.marks, p)
		return "", nil
	case "hold", "release":
		ds, snap, _ := strings.Cut(last, "@")
		d := h.ds[ds]
		if d == nil || !hasPoint(d.snaps, snap) {
			return "", fail(args, 1, "cannot %s snapshot '%s': dataset does not exist", args[1], last)
		}
		if args[1] == "hold" {
			if d.holds[snap] {
				return "", fail(args, 1, "cannot hold snapshot '%s': tag already exists on this dataset", last)
			}
			d.holds[snap] = true
			return "", nil
		}
		if !d.holds[snap] {
			return "", fail(args, 1, "cannot release hold from snapshot '%s': no such tag on this dataset", last)
		}
		delete(d.holds, snap)
		return "", nil
	case "destroy":
		return "", h.destroy(args)
	case "receive":
		return "", h.abort(args, last)
	case "send":
		return h.estimate(args)
	}
	return "", fail(args, 2, "fake: unknown command %q", args)
}

func (h *fakeHost) tree(args []string, root string) (string, error) {
	if h.ds[root] == nil {
		return "", missing(args, root)
	}
	var names []string
	for name := range h.ds {
		if name == root || zfs.DescendantOf(name, root) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		d := h.ds[name]
		enc, mp := "off", "/mnt/"+name
		if d.encrypted {
			enc = "aes-256-gcm"
		}
		if d.typ == "volume" {
			mp = "-"
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\tyes\ton\t%s\tavailable\thidden\t1024\t1024\n", name, d.typ, mp, enc)
	}
	return b.String(), nil
}

func (h *fakeHost) points(args []string, name string) (string, error) {
	d := h.ds[name]
	if d == nil {
		return "", missing(args, name)
	}
	type line struct {
		text string
		txg  uint64
	}
	var lines []line
	for _, p := range d.snaps {
		lines = append(lines, line{fmt.Sprintf("%s@%s\t%d\t%d", name, p.name, p.guid, p.txg), p.txg})
	}
	for _, p := range d.marks {
		lines = append(lines, line{fmt.Sprintf("%s#%s\t%d\t%d", name, p.name, p.guid, p.txg), p.txg})
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].txg < lines[j].txg })
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.text + "\n")
	}
	return b.String(), nil
}

func (h *fakeHost) destroy(args []string) error {
	target := args[len(args)-1]
	if ds, mark, ok := strings.Cut(target, "#"); ok {
		d := h.ds[ds]
		if d == nil || !hasPoint(d.marks, mark) {
			return fail(args, 1, "bookmark '%s' does not exist.", target)
		}
		d.marks = without(d.marks, mark)
		return nil
	}
	ds, snap, _ := strings.Cut(target, "@")
	names := []string{ds}
	if args[2] == "-r" {
		for name := range h.ds {
			if zfs.DescendantOf(name, ds) {
				names = append(names, name)
			}
		}
	}
	found := false
	for _, name := range names {
		d := h.ds[name]
		if d == nil || !hasPoint(d.snaps, snap) {
			continue
		}
		if d.holds[snap] {
			return fail(args, 1, "cannot destroy snapshot %s@%s: dataset is busy", name, snap)
		}
		found = true
	}
	if !found {
		return fail(args, 1, "could not find any snapshots to destroy; check snapshot names.")
	}
	for _, name := range names {
		if d := h.ds[name]; d != nil {
			d.snaps = without(d.snaps, snap)
		}
	}
	return nil
}

func (h *fakeHost) abort(args []string, target string) error {
	d := h.ds[target]
	if d == nil || d.token == "" {
		return fail(args, 1, "'%s' does not have any resumable receive state to abort", target)
	}
	if len(d.snaps) == 0 {
		delete(h.ds, target)
		return nil
	}
	d.token = ""
	return nil
}

// streamHeader is what a fake stream and a fake resume token carry.
type streamHeader struct {
	Dataset  string
	Snap     string
	GUID     uint64
	FromGUID uint64
	Raw      bool
	Volume   bool
	Size     int
	Offset   int
}

const streamTrailer = "\nEND\n"

func encodeToken(hd streamHeader) string {
	raw, _ := json.Marshal(hd)
	return fmt.Sprintf("1-%x-%x-%s", crc32.ChecksumIEEE(raw), len(raw), hex.EncodeToString(raw))
}

func decodeToken(tok string) (streamHeader, bool) {
	parts := strings.Split(tok, "-")
	var hd streamHeader
	if len(parts) != 4 {
		return hd, false
	}
	raw, err := hex.DecodeString(parts[3])
	if err != nil || json.Unmarshal(raw, &hd) != nil {
		return hd, false
	}
	return hd, true
}

// header resolves a send argv against this host, for a stream and for its
// estimate alike.
func (h *fakeHost) header(args []string) (streamHeader, error) {
	var hd streamHeader
	if i := index(args, "-t"); i >= 0 {
		tok, ok := decodeToken(args[i+1])
		if !ok {
			return hd, fail(args, 255, "cannot resume send: kernel modules must be upgraded to receive this stream")
		}
		d := h.ds[tok.Dataset]
		if d == nil {
			return hd, fail(args, 255, "cannot resume send: '%s' used in the initial send no longer exists", tok.Dataset)
		}
		p, ok := point(d.snaps, tok.Snap)
		if !ok || p.guid != tok.GUID {
			return hd, fail(args, 255, "cannot resume send: '%s@%s' used in the initial send no longer exists", tok.Dataset, tok.Snap)
		}
		return tok, nil
	}
	ds, snap, _ := strings.Cut(args[len(args)-1], "@")
	d := h.ds[ds]
	if d == nil {
		return hd, missing(args, ds)
	}
	p, ok := point(d.snaps, snap)
	if !ok {
		return hd, fail(args, 1, "cannot open '%s@%s': dataset does not exist", ds, snap)
	}
	hd = streamHeader{Dataset: ds, Snap: snap, GUID: p.guid, Raw: index(args, "-w") >= 0, Volume: d.typ == "volume", Size: h.payloadOf(ds)}
	if hd.Raw != d.encrypted && d.encrypted {
		return hd, fail(args, 1, "cannot send %s@%s: encrypted dataset %s may not be sent with properties without the raw flag", ds, snap, ds)
	}
	if i := index(args, "-i"); i >= 0 {
		from := args[i+1]
		pool := d.snaps
		if from[0] == '#' {
			pool = d.marks
		}
		fp, ok := point(pool, from[1:])
		if !ok {
			return hd, fail(args, 1, "cannot send '%s': incremental source (%s%s) does not exist", ds, ds, from)
		}
		hd.FromGUID = fp.guid
	}
	return hd, nil
}

func (h *fakeHost) payloadOf(ds string) int {
	if n, ok := h.payload[ds]; ok {
		return n
	}
	return 4096
}

func (h *fakeHost) estimate(args []string) (string, error) {
	hd, err := h.header(args)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("full\t%s@%s\t%d\nsize\t%d\n", hd.Dataset, hd.Snap, hd.Size-hd.Offset, hd.Size-hd.Offset), nil
}

func (h *fakeHost) Send(ctx context.Context, args []string) (io.ReadCloser, func() error, error) {
	h.record(args)
	h.mu.Lock()
	hd, err := h.header(args)
	cut, cuts := h.cutAfter[hd.Dataset]
	failWait := h.failSendWait[hd.Dataset]
	h.mu.Unlock()

	pr, pw := io.Pipe()
	done := make(chan error, 1)
	if err != nil {
		_ = pw.Close()
		done <- err
		return pr, func() error { return <-done }, nil
	}
	stop := context.AfterFunc(ctx, func() { _ = pw.CloseWithError(ctx.Err()) })
	go func() {
		defer stop()
		head, _ := json.Marshal(hd)
		if _, err := pw.Write(append(append([]byte("BVSTREAM "), head...), '\n')); err != nil {
			done <- fail(args, 255, "killed")
			return
		}
		chunk := bytes.Repeat([]byte("x"), 512)
		for sent := hd.Offset; sent < hd.Size; {
			n := min(len(chunk), hd.Size-sent)
			if cuts && sent+n > cut {
				n = cut - sent
				_, _ = pw.Write(chunk[:n])
				_ = pw.CloseWithError(errors.New("connection reset"))
				done <- fail(args, 255, "client_loop: send disconnect: Broken pipe")
				return
			}
			if _, err := pw.Write(chunk[:n]); err != nil {
				done <- fail(args, 255, "killed")
				return
			}
			sent += n
		}
		if _, err := pw.Write([]byte(streamTrailer)); err != nil {
			done <- fail(args, 255, "killed")
			return
		}
		_ = pw.Close()
		if failWait {
			done <- fail(args, 1, "warning: cannot send '%s@%s': Input/output error", hd.Dataset, hd.Snap)
			return
		}
		done <- nil
	}()
	return pr, func() error { return <-done }, nil
}

func (h *fakeHost) Receive(_ context.Context, args []string, stream io.Reader) error {
	h.record(args)
	br := bufio.NewReader(stream)
	line, err := br.ReadString('\n')
	if err != nil {
		return fail(args, 1, "cannot receive: failed to read from stream")
	}
	var hd streamHeader
	if json.Unmarshal([]byte(strings.TrimPrefix(line, "BVSTREAM ")), &hd) != nil {
		return fail(args, 1, "cannot receive: invalid stream (bad magic number)")
	}
	target := args[len(args)-1]
	force, resumable := index(args, "-F") >= 0, index(args, "-s") >= 0

	h.mu.Lock()
	if stderr, ok := h.refuseReceive[target]; ok {
		h.mu.Unlock()
		return fail(args, 1, "%s", stderr)
	}
	d, err := h.checkReceive(args, target, hd, force)
	h.mu.Unlock()
	if err != nil {
		return err
	}

	body, readErr := io.ReadAll(br)
	got := len(body) - len(streamTrailer)
	complete := readErr == nil && bytes.HasSuffix(body, []byte(streamTrailer)) && hd.Offset+got == hd.Size

	h.mu.Lock()
	defer h.mu.Unlock()
	if !complete {
		if !resumable {
			return fail(args, 1, "cannot receive new filesystem stream: checksum mismatch or incomplete stream")
		}
		if d == nil {
			d = &fakeDS{typ: "filesystem", encrypted: hd.Raw, holds: map[string]bool{}, props: map[string]string{}}
			if hd.Volume {
				d.typ = "volume"
			}
			h.ds[target] = d
		}
		resume := hd
		resume.Offset = hd.Offset + max(len(body), 0)
		if bytes.HasSuffix(body, []byte(streamTrailer)) {
			resume.Offset -= len(streamTrailer)
		}
		d.token = encodeToken(resume)
		return fail(args, 1, "cannot receive incremental stream: checksum mismatch or incomplete stream.\nPartially received snapshot is saved.\nA resuming stream can be generated on the sending system by running:\n    zfs send -t %s", d.token)
	}

	if d == nil {
		d = &fakeDS{typ: "filesystem", encrypted: hd.Raw, holds: map[string]bool{}, props: map[string]string{}}
		if hd.Volume {
			d.typ = "volume"
		}
		h.ds[target] = d
	}
	for i, a := range args {
		if a == "-o" {
			k, v, _ := strings.Cut(args[i+1], "=")
			d.props[k] = v
		}
	}
	h.txg++
	d.snaps = append(d.snaps, fakePoint{hd.Snap, hd.GUID, h.txg})
	d.token = ""
	d.modified = false
	return nil
}

// checkReceive is what zfs decides from the stream's header before it reads
// any data.
func (h *fakeHost) checkReceive(args []string, target string, hd streamHeader, force bool) (*fakeDS, error) {
	d := h.ds[target]
	if hd.Offset > 0 {
		if d == nil || d.token == "" {
			return nil, fail(args, 1, "cannot receive resume stream: destination '%s' does not have a resumable receive state", target)
		}
		return d, nil
	}
	if d != nil && d.token != "" {
		return nil, fail(args, 1, "cannot receive: destination %s contains partially-complete state from \"zfs receive -s\".", target)
	}
	if hd.Volume && index(args, "canmount=noauto") >= 0 {
		return nil, fail(args, 1, "cannot receive: 'canmount' does not apply to datasets of this type")
	}
	if hd.FromGUID == 0 {
		if d != nil {
			if !force {
				return nil, fail(args, 1, "cannot receive new filesystem stream: destination '%s' exists\nmust specify -F to overwrite it", target)
			}
			if len(d.snaps) > 0 {
				return nil, fail(args, 1, "cannot receive new filesystem stream: destination has snapshots (eg. %s@%s)\nmust destroy them to overwrite it", target, d.snaps[0].name)
			}
			return d, nil
		}
		parent := target[:strings.LastIndexByte(target, '/')]
		if h.ds[parent] == nil {
			return nil, fail(args, 1, "cannot receive new filesystem stream: dataset does not exist")
		}
		return nil, nil
	}
	if d == nil {
		return nil, fail(args, 1, "cannot receive incremental stream: destination '%s' does not exist", target)
	}
	if hd.Raw && !d.encrypted {
		return nil, fail(args, 1, "cannot receive incremental stream: raw receive on top of existing unencrypted dataset")
	}
	from := -1
	for i, p := range d.snaps {
		if p.guid == hd.FromGUID {
			from = i
		}
	}
	if from < 0 {
		return nil, fail(args, 1, "cannot receive incremental stream: most recent snapshot of %s does not\nmatch incremental source", target)
	}
	if force {
		d.snaps = d.snaps[:from+1]
		d.modified = false
		return d, nil
	}
	if d.modified || (hd.Raw && from != len(d.snaps)-1) {
		return nil, fail(args, 1, "cannot receive incremental stream: destination %s has been modified\nsince most recent snapshot", target)
	}
	return d, nil
}

func index(args []string, flag string) int {
	for i, a := range args {
		if a == flag {
			return i
		}
	}
	return -1
}

func point(pts []fakePoint, name string) (fakePoint, bool) {
	for _, p := range pts {
		if p.name == name {
			return p, true
		}
	}
	return fakePoint{}, false
}

func hasPoint(pts []fakePoint, name string) bool {
	_, ok := point(pts, name)
	return ok
}

func without(pts []fakePoint, name string) []fakePoint {
	var out []fakePoint
	for _, p := range pts {
		if p.name != name {
			out = append(out, p)
		}
	}
	return out
}
