package zfsrepl

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

type largestWrite struct {
	bytes.Buffer
	largest int
}

func (w *largestWrite) Write(p []byte) (int, error) {
	w.largest = max(w.largest, len(p))
	return w.Buffer.Write(p)
}

func TestTheRelayReadsTheSendAheadOfAReceiveThatPauses(t *testing.T) {
	payload := bytes.Repeat([]byte("zfs stream "), 800_000)
	pr, pw := io.Pipe()
	sent := make(chan error, 1)
	go func() {
		_, err := pw.Write(payload)
		_ = pw.Close()
		sent <- err
	}()
	var reported int64
	r := newRelay(pr, func(done int64) { reported = done })
	defer r.stop()

	select {
	case err := <-sent:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the send stalled while the receive had not read yet")
	}

	var got largestWrite
	if _, err := io.Copy(&got, r); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), payload) {
		t.Fatalf("the receive got %d bytes, want the %d sent", got.Len(), len(payload))
	}
	if got.largest < relayChunk {
		t.Errorf("the largest write was %d bytes, want whole chunks of %d", got.largest, relayChunk)
	}
	if reported != int64(len(payload)) || r.n.Load() != int64(len(payload)) {
		t.Errorf("counted %d, reported %d, want %d", r.n.Load(), reported, len(payload))
	}
}

func TestTheRelayPassesOnAFailedRead(t *testing.T) {
	broken := errors.New("connection reset")
	r := newRelay(io.MultiReader(bytes.NewReader([]byte("start")), errReader{broken}), nil)
	defer r.stop()
	if _, err := io.ReadAll(r); !errors.Is(err, broken) {
		t.Fatalf("err = %v, want the read error", err)
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }
