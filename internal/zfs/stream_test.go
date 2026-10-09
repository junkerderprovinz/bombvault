package zfs

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

// beginRecord builds the leading record a send stream starts with.
func beginRecord(order binary.ByteOrder, hdrtype uint64, objset uint32, to, from uint64, name string) []byte {
	b := make([]byte, streamBeginLen)
	order.PutUint64(b[8:], streamMagic)
	order.PutUint64(b[16:], 17<<2|hdrtype)
	order.PutUint32(b[32:], objset)
	order.PutUint64(b[40:], to)
	order.PutUint64(b[48:], from)
	copy(b[56:], name)
	return b
}

func TestReadStreamBeginLeavesTheStreamWhole(t *testing.T) {
	stream := append(beginRecord(binary.LittleEndian, streamSubstream, 2, 7, 5, "cache/appdata@"+replicaSnap), "payload"...)
	r := bufio.NewReader(bytes.NewReader(stream))
	got, err := ReadStreamBegin(r)
	if err != nil {
		t.Fatal(err)
	}
	if got != (StreamBegin{Snapshot: "cache/appdata@" + replicaSnap, ToGUID: 7, FromGUID: 5}) {
		t.Fatalf("ReadStreamBegin = %+v", got)
	}
	rest, _ := io.ReadAll(r)
	if !bytes.Equal(rest, stream) {
		t.Fatal("reading the leading record consumed part of the stream")
	}
}

func TestReadStreamBeginReadsABigEndianVolume(t *testing.T) {
	stream := beginRecord(binary.BigEndian, streamSubstream, streamObjsetZvol, 9, 0, "tank/vm@"+replicaSnap)
	got, err := ReadStreamBegin(bufio.NewReader(bytes.NewReader(stream)))
	if err != nil || !got.Volume || got.FromGUID != 0 || got.ToGUID != 9 {
		t.Fatalf("ReadStreamBegin = %+v, %v; want a full volume stream", got, err)
	}
}

func TestReadStreamBeginRefusesWhatIsNotOneDataset(t *testing.T) {
	compound := beginRecord(binary.LittleEndian, 2, 2, 7, 0, "cache@"+replicaSnap)
	garbage := bytes.Repeat([]byte("x"), streamBeginLen)
	notBegin := beginRecord(binary.LittleEndian, streamSubstream, 2, 7, 0, "cache@"+replicaSnap)
	binary.LittleEndian.PutUint32(notBegin, 7)
	for name, stream := range map[string][]byte{
		"compound": compound, "no magic": garbage, "not a begin record": notBegin, "short": compound[:100],
	} {
		if _, err := ReadStreamBegin(bufio.NewReader(bytes.NewReader(stream))); err == nil {
			t.Errorf("%s: ReadStreamBegin accepted it", name)
		}
	}
}
