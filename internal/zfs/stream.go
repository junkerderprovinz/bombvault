package zfs

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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
