package zfs

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// The heads in testdata are the first records of real streams from OpenZFS
// 2.4.3: zfs send -c -L -e -p of a filesystem and of a volume, and zfs send -R.
func TestReadStreamBeginReadsTheDatasetInsideASendPStream(t *testing.T) {
	for _, tc := range []struct {
		file string
		want StreamBegin
	}{
		{"send-p.head", StreamBegin{Snapshot: "bvzsrc/peer/c1@bvzhdr1", ToGUID: 4884498687262333136}},
		{"send-p-volume.head", StreamBegin{Snapshot: "bvzsrc/data/vol@bombvault-replica-20261009181412", ToGUID: 8031173462771466140, Volume: true}},
	} {
		head, err := os.ReadFile(filepath.Join("testdata", tc.file))
		if err != nil {
			t.Fatal(err)
		}
		r := bufio.NewReader(bytes.NewReader(head))
		got, err := ReadStreamBegin(r)
		if err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		if got != tc.want {
			t.Fatalf("%s: ReadStreamBegin = %+v, want %+v", tc.file, got, tc.want)
		}
		if rest, _ := io.ReadAll(r); !bytes.Equal(rest, head) {
			t.Fatalf("%s: reading the head consumed part of the stream", tc.file)
		}
	}
}

func TestReadStreamBeginRefusesARecursiveStream(t *testing.T) {
	head, err := os.ReadFile(filepath.Join("testdata", "send-R.head"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ReadStreamBegin(bufio.NewReader(bytes.NewReader(head))); err == nil {
		t.Fatalf("ReadStreamBegin accepted a zfs send -R stream: %+v", got)
	}
}

// compoundStream wraps a substream the way zfs send -p does, with a property
// list that holds the given top-level names.
//
//nolint:gosec // G115: fixture lengths are small
func compoundStream(order binary.ByteOrder, names []string, sub []byte) []byte {
	nv := []byte{1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	for _, n := range names {
		padded := (len(n) + 3) &^ 3
		pair := make([]byte, 20+padded+4)
		binary.BigEndian.PutUint32(pair[0:], uint32(len(pair)))
		binary.BigEndian.PutUint32(pair[4:], uint32(len(pair)))
		binary.BigEndian.PutUint32(pair[8:], uint32(len(n)))
		copy(pair[12:], n)
		binary.BigEndian.PutUint32(pair[12+padded:], 1)
		nv = append(nv, pair...)
	}
	nv = append(nv, make([]byte, 8)...)
	outer := beginRecord(order, 2, 0, 0, 0, "cache/appdata@"+replicaSnap)
	order.PutUint32(outer[4:], uint32(len(nv)))
	end := make([]byte, streamBeginLen)
	order.PutUint32(end, 5)
	return slices.Concat(outer, nv, end, sub)
}

func TestReadStreamBeginTakesTheGUIDsFromTheSubstream(t *testing.T) {
	sub := beginRecord(binary.BigEndian, streamSubstream, 2, 11, 10, "cache/appdata@"+replicaSnap)
	stream := compoundStream(binary.BigEndian, []string{"tosnap", "not_recursive", "fss"}, sub)
	got, err := ReadStreamBegin(bufio.NewReader(bytes.NewReader(stream)))
	if err != nil || got.ToGUID != 11 || got.FromGUID != 10 {
		t.Fatalf("ReadStreamBegin = %+v, %v; want the substream's guids 11 from 10", got, err)
	}
}

func TestReadStreamBeginRefusesACompoundStreamItCannotFollow(t *testing.T) {
	sub := beginRecord(binary.LittleEndian, streamSubstream, 2, 7, 0, "cache@"+replicaSnap)
	recursive := compoundStream(binary.LittleEndian, []string{"tosnap", "fss"}, sub)
	noSub := compoundStream(binary.LittleEndian, []string{"not_recursive"}, bytes.Repeat([]byte("x"), streamBeginLen))
	nested := compoundStream(binary.LittleEndian, []string{"not_recursive"}, compoundStream(binary.LittleEndian, nil, sub))
	cut := compoundStream(binary.LittleEndian, []string{"not_recursive"}, sub)
	cut = cut[:len(cut)-1]
	for name, stream := range map[string][]byte{
		"recursive": recursive, "no substream": noSub, "nested": nested, "cut": cut,
	} {
		if got, err := ReadStreamBegin(bufio.NewReader(bytes.NewReader(stream))); err == nil {
			t.Errorf("%s: ReadStreamBegin accepted it: %+v", name, got)
		}
	}
}

// readStream reads one of the whole streams in testdata, sent from OpenZFS
// 2.4.3 with the flags its name gives.
func readStream(t *testing.T, name string) []byte {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name+".stream.gz")) //nolint:gosec // G304: a fixture the test names
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // a read-only fixture
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSnapshotStreamPassesEveryRecordOfOneSnapshot(t *testing.T) {
	// send-p and send-p-volume are zfs send -c -L -e -p, send-w-p a raw
	// stream of an encrypted dataset, send-plain one without any flag and
	// send-t its resumption.
	for _, name := range []string{"send-p", "send-p-volume", "send-w-p", "send-plain", "send-t"} {
		stream := readStream(t, name)
		s := NewSnapshotStream(bytes.NewReader(stream))
		got, err := io.ReadAll(s)
		if err != nil || s.Refused() != nil {
			t.Errorf("%s: %v", name, err)
		}
		if !bytes.Equal(got, stream) {
			t.Errorf("%s: passed %d of %d bytes", name, len(got), len(stream))
		}
	}
}

func TestSnapshotStreamStopsBeforeASecondSnapshot(t *testing.T) {
	// zfs send -p -I from the first of three snapshots: one substream to the
	// second snapshot, then one to the third.
	stream := readStream(t, "send-p-I")
	s := NewSnapshotStream(bytes.NewReader(stream))
	got, err := io.ReadAll(s)
	if err == nil || s.Refused() == nil {
		t.Fatal("a stream of two snapshots passed")
	}
	rest := stream[len(got):]
	if !bytes.HasPrefix(stream, got) || len(rest) < streamBeginLen || !isBegin(binary.LittleEndian, rest) {
		t.Fatalf("passed %d bytes, want everything before the second begin record", len(got))
	}
	if b := parseBegin(binary.LittleEndian, rest); !strings.HasSuffix(b.Snapshot, "@bombvault-replica-20261011030000") {
		t.Fatalf("stopped before %s, want the substream to the third snapshot", b.Snapshot)
	}
}

func TestSnapshotStreamRefusesWhatFollowsTheEnd(t *testing.T) {
	for _, name := range []string{"send-p", "send-plain"} {
		stream := append(readStream(t, name), "more"...)
		s := NewSnapshotStream(bytes.NewReader(stream))
		if _, err := io.ReadAll(s); err == nil || s.Refused() == nil {
			t.Errorf("%s: bytes after the end passed", name)
		}
	}
}

func TestSnapshotStreamPassesACutStreamOnWithoutRefusing(t *testing.T) {
	stream := readStream(t, "send-p")
	cut := stream[:len(stream)/2]
	s := NewSnapshotStream(bytes.NewReader(cut))
	got, err := io.ReadAll(s)
	if !errors.Is(err, io.ErrUnexpectedEOF) || s.Refused() != nil {
		t.Fatalf("a cut stream ended with %v, refused %v; want it short and not refused", err, s.Refused())
	}
	if !bytes.HasPrefix(cut, got) || len(cut)-len(got) >= streamBeginLen {
		t.Fatalf("passed %d of the %d bytes that came", len(got), len(cut))
	}
}
