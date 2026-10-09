package zfs

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// The leading record of every send stream is a DRR_BEGIN dmu_replay_record:
// type and payload length, then magic, version info, creation time, objset
// type, flags, toguid, fromguid and the 256 byte name of the snapshot it
// carries, all in the byte order of the sending host.
const (
	streamBeginLen   = 312
	streamMagic      = 0x2F5bacbac
	streamSubstream  = 1
	streamCompound   = 2
	streamRecordEnd  = 5
	streamObjsetZvol = 3
)

// StreamBegin is what a send stream says about itself before any data.
type StreamBegin struct {
	// Snapshot is the full name the stream was sent from, dataset@snap.
	Snapshot string
	ToGUID   uint64
	// FromGUID is the base of an incremental, 0 for a full stream.
	FromGUID uint64
	Volume   bool
}

// ReadStreamBegin reads the leading record of a send stream without consuming
// it, so the stream goes on to zfs receive whole. zfs send -p wraps its one
// dataset in a compound stream, a begin record with the property list, an end
// record and then the substream, whose begin record is the one read here. A
// compound stream whose list lacks not_recursive is what zfs send -R makes,
// and is refused: one of those can create and destroy datasets beside the one
// it names.
func ReadStreamBegin(r *bufio.Reader) (StreamBegin, error) {
	b, err := r.Peek(streamBeginLen)
	if err != nil {
		return StreamBegin{}, fmt.Errorf("zfs stream: the leading record is short: %w", err)
	}
	var order binary.ByteOrder = binary.LittleEndian
	if order.Uint64(b[8:]) != streamMagic {
		order = binary.BigEndian
	}
	if !isBegin(order, b) {
		return StreamBegin{}, errors.New("zfs stream: does not start with a send stream begin record")
	}
	switch order.Uint64(b[16:]) & 3 {
	case streamSubstream:
		return parseBegin(order, b), nil
	case streamCompound:
	default:
		return StreamBegin{}, errors.New("zfs stream: unknown stream type")
	}

	list := int(order.Uint32(b[4:]))
	need := 3*streamBeginLen + list
	if need > r.Size() {
		return StreamBegin{}, fmt.Errorf("zfs stream: the property list of %d bytes is too large", list)
	}
	if b, err = r.Peek(need); err != nil {
		return StreamBegin{}, fmt.Errorf("zfs stream: the compound stream is short: %w", err)
	}
	single, err := nvlistHas(b[streamBeginLen:streamBeginLen+list], "not_recursive")
	if err != nil {
		return StreamBegin{}, err
	}
	if !single {
		return StreamBegin{}, errors.New("zfs stream: a recursive stream carries more than one dataset")
	}
	if order.Uint32(b[streamBeginLen+list:]) != streamRecordEnd {
		return StreamBegin{}, errors.New("zfs stream: no end record after the property list")
	}
	sub := b[2*streamBeginLen+list:]
	if !isBegin(order, sub) || order.Uint64(sub[16:])&3 != streamSubstream {
		return StreamBegin{}, errors.New("zfs stream: no substream after the property list")
	}
	return parseBegin(order, sub), nil
}

// The record types of a send stream that SnapshotStream tells apart.
const (
	recordBegin         = 0
	recordObject        = 1
	recordFreeObjects   = 2
	recordWrite         = 3
	recordFree          = 4
	recordWriteByRef    = 6
	recordSpill         = 7
	recordWriteEmbedded = 8
	recordObjectRange   = 9
	recordRedact        = 10
)

// The places in a stream SnapshotStream can be at. A compound stream is its
// begin record with the property list, an end record, the substream and a
// last end record; a plain stream is the substream alone.
const (
	atStart = iota
	inList
	beforeSub
	inSub
	afterSub
	atEnd
)

// SnapshotStream passes a send stream through record by record and refuses
// anything beyond the one snapshot ReadStreamBegin read: a second substream
// in a compound stream, or bytes after the stream's end. zfs receive takes
// every substream of a compound stream with the flags chosen for the first, so
// a second one could roll back the very snapshots the first was checked
// against.
type SnapshotStream struct {
	r        io.Reader
	order    binary.ByteOrder
	head     [streamBeginLen]byte
	pending  []byte
	left     uint64
	at       int
	compound bool
	refused  error
}

// NewSnapshotStream wraps the stream ReadStreamBegin accepted.
func NewSnapshotStream(r io.Reader) *SnapshotStream { return &SnapshotStream{r: r} }

// Refused is why the stream was cut off, nil while it is one snapshot.
func (s *SnapshotStream) Refused() error { return s.refused }

func (s *SnapshotStream) Read(p []byte) (int, error) {
	if s.refused != nil {
		return 0, s.refused
	}
	if len(s.pending) == 0 && s.left == 0 {
		if err := s.next(); err != nil {
			return 0, err
		}
	}
	if len(s.pending) > 0 {
		n := copy(p, s.pending)
		s.pending = s.pending[n:]
		return n, nil
	}
	if uint64(len(p)) > s.left {
		p = p[:s.left]
	}
	n, err := s.r.Read(p)
	s.left -= uint64(n) //nolint:gosec // G115: Read never returns a negative count
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

// next reads the header of the next record and decides whether it may pass.
func (s *SnapshotStream) next() error {
	if s.at == atEnd {
		var b [1]byte
		if n, _ := s.r.Read(b[:]); n > 0 {
			return s.refuse("there is more after the end of the stream")
		}
		return io.EOF
	}
	if _, err := io.ReadFull(s.r, s.head[:]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	h := s.head[:]
	if s.order == nil {
		s.order = binary.LittleEndian
		if s.order.Uint64(h[8:]) != streamMagic {
			s.order = binary.BigEndian
		}
	}
	o := s.order
	typ := o.Uint32(h)
	switch {
	case typ == recordBegin && (s.at == atStart || s.at == beforeSub):
		if !isBegin(o, h) {
			return s.refuse("a begin record without the stream magic")
		}
		switch hdr := o.Uint64(h[16:]) & 3; {
		case hdr == streamCompound && s.at == atStart:
			s.at, s.compound = inList, true
		case hdr == streamSubstream:
			s.at = inSub
		default:
			return s.refuse("a begin record that does not start one substream")
		}
		s.left = uint64(o.Uint32(h[4:]))
	case typ == recordBegin:
		return s.refuse("the stream carries a second snapshot")
	case typ == streamRecordEnd && s.at == inList:
		s.at = beforeSub
	case typ == streamRecordEnd && s.at == inSub && s.compound:
		s.at = afterSub
	case typ == streamRecordEnd && (s.at == inSub || s.at == afterSub):
		s.at = atEnd
	case s.at == inSub:
		size, ok := payloadSize(o, typ, h)
		if !ok {
			return s.refuse(fmt.Sprintf("a record of unknown type %d", typ))
		}
		s.left = size
	default:
		return s.refuse(fmt.Sprintf("a record of type %d outside the substream", typ))
	}
	s.pending = h
	return nil
}

func (s *SnapshotStream) refuse(why string) error {
	s.refused = errors.New("zfs stream: " + why)
	return s.refused
}

// payloadSize is how many bytes follow a record header inside a substream,
// computed the way zfs receive does for each record type.
func payloadSize(o binary.ByteOrder, typ uint32, h []byte) (uint64, bool) {
	round8 := func(n uint64) uint64 { return (n + 7) &^ 7 }
	switch typ {
	case recordObject:
		if raw := o.Uint32(h[36:]); raw != 0 {
			return uint64(raw), true
		}
		return round8(uint64(o.Uint32(h[28:]))), true
	case recordWrite:
		if h[50] != 0 {
			return o.Uint64(h[96:]), true
		}
		return o.Uint64(h[32:]), true
	case recordSpill:
		if c := o.Uint64(h[40:]); c != 0 {
			return c, true
		}
		return o.Uint64(h[16:]), true
	case recordWriteEmbedded:
		return round8(uint64(o.Uint32(h[52:]))), true
	case recordFreeObjects, recordFree, recordWriteByRef, recordObjectRange, recordRedact:
		return 0, true
	}
	return 0, false
}

func isBegin(order binary.ByteOrder, b []byte) bool {
	return order.Uint32(b[0:]) == 0 && order.Uint64(b[8:]) == streamMagic
}

func parseBegin(order binary.ByteOrder, b []byte) StreamBegin {
	name, _, _ := bytes.Cut(b[56:streamBeginLen], []byte{0})
	return StreamBegin{
		Snapshot: string(name),
		ToGUID:   order.Uint64(b[40:]),
		FromGUID: order.Uint64(b[48:]),
		Volume:   order.Uint32(b[32:]) == streamObjsetZvol,
	}
}

// nvlistHas reports whether an XDR packed nvlist holds a pair called name at
// its top level. Every pair starts with its encoded size, so the walk steps
// over the values without decoding them.
func nvlistHas(p []byte, name string) (bool, error) {
	short := errors.New("zfs stream: the property list is cut short")
	// The 4 byte header says XDR with a 1, then come the version and flags.
	if len(p) < 12 || p[0] != 1 {
		return false, errors.New("zfs stream: the property list is not XDR encoded")
	}
	for off := 12; ; {
		if off+8 > len(p) {
			return false, short
		}
		size := int(binary.BigEndian.Uint32(p[off:]))
		if size == 0 {
			return false, nil
		}
		if size < 12 || size > len(p)-off {
			return false, short
		}
		n := int(binary.BigEndian.Uint32(p[off+8:]))
		if n > size-12 {
			return false, short
		}
		if string(p[off+12:off+12+n]) == name {
			return true, nil
		}
		off += size
	}
}
