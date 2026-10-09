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
// it, so the stream goes on to zfs receive whole. It refuses a compound
// stream, which is what zfs send -R makes: one of those can create and
// destroy datasets beside the one it names.
func ReadStreamBegin(r *bufio.Reader) (StreamBegin, error) {
	b, err := r.Peek(streamBeginLen)
	if err != nil {
		return StreamBegin{}, fmt.Errorf("zfs stream: the leading record is short: %w", err)
	}
	var order binary.ByteOrder = binary.LittleEndian
	if order.Uint64(b[8:]) != streamMagic {
		order = binary.BigEndian
		if order.Uint64(b[8:]) != streamMagic {
			return StreamBegin{}, errors.New("zfs stream: no send stream magic")
		}
	}
	if order.Uint32(b[0:]) != 0 {
		return StreamBegin{}, errors.New("zfs stream: does not start with a begin record")
	}
	if order.Uint64(b[16:])&3 != streamSubstream {
		return StreamBegin{}, errors.New("zfs stream: a compound stream carries more than one dataset")
	}
	name, _, _ := bytes.Cut(b[56:streamBeginLen], []byte{0})
	return StreamBegin{
		Snapshot: string(name),
		ToGUID:   order.Uint64(b[40:]),
		FromGUID: order.Uint64(b[48:]),
		Volume:   order.Uint32(b[32:]) == streamObjsetZvol,
	}, nil
}
