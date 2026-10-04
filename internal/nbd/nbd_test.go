package nbd

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
)

// fakeServer answers the parts of the protocol the client uses, over one
// end of a net.Pipe.
type fakeServer struct {
	data []byte
	// extents per context id; a block status answer returns at most two of
	// them so the client has to ask again.
	extents map[uint32][]Extent
	names   map[string]uint32
	failAt  int64
}

//nolint:gosec // G115: fixture sizes and offsets are small
func (s *fakeServer) serve(t *testing.T, conn net.Conn) {
	t.Helper()
	defer func() { _ = conn.Close() }()
	w := func(v any) { _ = binary.Write(conn, binary.BigEndian, v) }
	w(uint64(magicInit))
	w(uint64(magicOpt))
	w(uint16(flagFixedNewstyle | flagNoZeroes))
	var cflags uint32
	if binary.Read(conn, binary.BigEndian, &cflags) != nil {
		return
	}
	reply := func(opt, typ uint32, data []byte) {
		w(uint64(magicOptReply))
		w(opt)
		w(typ)
		w(uint32(len(data)))
		if len(data) > 0 {
			_, _ = conn.Write(data)
		}
	}
	for {
		var o struct {
			Magic  uint64
			Opt    uint32
			Length uint32
		}
		if binary.Read(conn, binary.BigEndian, &o) != nil {
			return
		}
		body := make([]byte, o.Length)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		switch o.Opt {
		case optStructuredReply:
			reply(o.Opt, repAck, nil)
		case optSetMetaContext:
			n := binary.BigEndian.Uint32(body[:4])
			rest := body[4+n:]
			count := binary.BigEndian.Uint32(rest[:4])
			rest = rest[4:]
			for range count {
				l := binary.BigEndian.Uint32(rest[:4])
				name := string(rest[4 : 4+l])
				rest = rest[4+l:]
				if id, ok := s.names[name]; ok {
					reply(o.Opt, repMetaContext, append(binary.BigEndian.AppendUint32(nil, id), name...))
				}
			}
			reply(o.Opt, repAck, nil)
		case optGo:
			info := binary.BigEndian.AppendUint16(nil, infoExport)
			info = binary.BigEndian.AppendUint64(info, uint64(len(s.data)))
			info = binary.BigEndian.AppendUint16(info, 0)
			reply(o.Opt, repInfo, info)
			reply(o.Opt, repAck, nil)
			s.transmit(conn)
			return
		}
	}
}

//nolint:gosec // G115: fixture sizes and offsets are small
func (s *fakeServer) transmit(conn net.Conn) {
	w := func(v any) { _ = binary.Write(conn, binary.BigEndian, v) }
	chunkOut := func(flags, typ uint16, cookie uint64, data []byte) {
		w(uint32(magicStructure))
		w(flags)
		w(typ)
		w(cookie)
		w(uint32(len(data)))
		if len(data) > 0 {
			_, _ = conn.Write(data)
		}
	}
	for {
		var r struct {
			Magic  uint32
			Flags  uint16
			Type   uint16
			Cookie uint64
			Offset uint64
			Length uint32
		}
		if binary.Read(conn, binary.BigEndian, &r) != nil || r.Type == cmdDisc {
			return
		}
		off, end := int64(r.Offset), int64(r.Offset)+int64(r.Length)
		switch r.Type {
		case cmdRead:
			if s.failAt > 0 && off <= s.failAt && s.failAt < end {
				msg := "boom"
				e := binary.BigEndian.AppendUint32(nil, 5)
				e = binary.BigEndian.AppendUint16(e, uint16(len(msg)))
				chunkOut(replyFlagDone, replyErrBit|1, r.Cookie, append(e, msg...))
				continue
			}
			// Answer in 4 KiB pieces, zero pieces as holes, last one first so
			// the client cannot rely on order.
			var pieces [][2]int64
			for p := off; p < end; p += 4096 {
				pieces = append(pieces, [2]int64{p, min(p+4096, end)})
			}
			for i := len(pieces) - 1; i >= 0; i-- {
				flags := uint16(0)
				if i == 0 {
					flags = replyFlagDone
				}
				a, b := pieces[i][0], pieces[i][1]
				if bytes.Count(s.data[a:b], []byte{0}) == int(b-a) {
					h := binary.BigEndian.AppendUint64(nil, uint64(a))
					chunkOut(flags, replyOffsetHole, r.Cookie, binary.BigEndian.AppendUint32(h, uint32(b-a)))
					continue
				}
				chunkOut(flags, replyOffsetData, r.Cookie, append(binary.BigEndian.AppendUint64(nil, uint64(a)), s.data[a:b]...))
			}
		case cmdBlockStatus:
			for id, exts := range s.extents {
				payload := binary.BigEndian.AppendUint32(nil, id)
				sent := 0
				for _, e := range exts {
					if e.Offset+e.Length <= off || sent == 2 {
						continue
					}
					start := max(e.Offset, off)
					payload = binary.BigEndian.AppendUint32(payload, uint32(e.Offset+e.Length-start))
					payload = binary.BigEndian.AppendUint32(payload, e.Flags)
					sent++
				}
				chunkOut(0, replyBlockStatus, r.Cookie, payload)
			}
			chunkOut(replyFlagDone, replyNone, r.Cookie, nil)
		}
	}
}

func dialFake(t *testing.T, s *fakeServer, contexts []string) *Client {
	t.Helper()
	cli, srv := net.Pipe()
	go s.serve(t, srv)
	c, err := Handshake(context.Background(), cli, "vda", contexts)
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestReadReturnsDataAndZerosForHoles(t *testing.T) {
	data := make([]byte, 64<<10)
	for i := 8 << 10; i < 20<<10; i++ {
		data[i] = byte(i) //nolint:gosec // G115: wraps on purpose
	}
	c := dialFake(t, &fakeServer{data: data}, nil)
	if c.Size() != int64(len(data)) {
		t.Fatalf("size = %d, want %d", c.Size(), len(data))
	}
	got := make([]byte, 30<<10)
	for i := range got {
		got[i] = 0xff
	}
	if _, err := c.ReadAt(got, 2<<10); err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, data[2<<10:32<<10]) {
		t.Fatal("read returned other bytes than the export holds")
	}
}

func TestReadReportsServerError(t *testing.T) {
	c := dialFake(t, &fakeServer{data: make([]byte, 16<<10), failAt: 5000}, nil)
	if _, err := c.ReadAt(make([]byte, 8<<10), 0); err == nil {
		t.Fatal("a failed read came back without an error")
	}
}

func TestReadOutsideExportIsRefused(t *testing.T) {
	c := dialFake(t, &fakeServer{data: make([]byte, 4096)}, nil)
	if _, err := c.ReadAt(make([]byte, 4096), 1); err == nil {
		t.Fatal("a read past the end was sent")
	}
}

func TestBlockStatusCoversRangeAcrossShortAnswers(t *testing.T) {
	s := &fakeServer{
		data:  make([]byte, 1<<20),
		names: map[string]uint32{"qemu:dirty-bitmap:backup-vda": 7},
		extents: map[uint32][]Extent{7: {
			{Offset: 0, Length: 64 << 10, Flags: 0},
			{Offset: 64 << 10, Length: 128 << 10, Flags: StateDirty},
			{Offset: 192 << 10, Length: 512 << 10, Flags: 0},
			{Offset: 704 << 10, Length: 320 << 10, Flags: StateDirty},
		}},
	}
	c := dialFake(t, s, []string{"qemu:dirty-bitmap:backup-vda", "qemu:allocation-depth"})
	if c.HasContext("qemu:allocation-depth") {
		t.Fatal("a context the server did not offer counts as negotiated")
	}
	exts, err := c.BlockStatus("qemu:dirty-bitmap:backup-vda", 0, 1<<20)
	if err != nil {
		t.Fatalf("block status: %v", err)
	}
	var dirty []Extent
	var pos int64
	for _, e := range exts {
		if e.Offset != pos {
			t.Fatalf("extent at %d, want %d", e.Offset, pos)
		}
		pos += e.Length
		if e.Flags&StateDirty != 0 {
			dirty = append(dirty, e)
		}
	}
	if pos != 1<<20 {
		t.Fatalf("extents cover %d bytes, want %d", pos, 1<<20)
	}
	want := []Extent{{64 << 10, 128 << 10, StateDirty}, {704 << 10, 320 << 10, StateDirty}}
	if len(dirty) != 2 || dirty[0] != want[0] || dirty[1] != want[1] {
		t.Fatalf("dirty = %v, want %v", dirty, want)
	}
}

func TestBlockStatusNeedsNegotiatedContext(t *testing.T) {
	c := dialFake(t, &fakeServer{data: make([]byte, 4096)}, nil)
	if _, err := c.BlockStatus(AllocationContext, 0, 4096); err == nil {
		t.Fatal("block status ran for a context that was never agreed")
	}
}
