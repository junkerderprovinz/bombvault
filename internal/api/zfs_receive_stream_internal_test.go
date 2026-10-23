package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// largeStream is a full stream of sixteen MiB in blocks of 128 KiB.
func largeStream() []byte {
	parts := [][]byte{streamRecord("cache/appdata@bombvault-replica-20261010030000", 13, 0, false)}
	block := bytes.Repeat([]byte("x"), 128<<10)
	for range 128 {
		parts = append(parts, fakeRecord(3, uint64(len(block))), block)
	}
	return slices.Concat(append(parts, fakeRecord(5, 0))...)
}

// copyingPool takes a receive's stdin the way exec does, with io.Copy, and
// notes the largest write the receive got. pause, when set, runs after the
// first write, before the receive reads on.
type copyingPool struct {
	*fakePool
	largest int
	pause   func()
}

func (p *copyingPool) RunWithStdin(ctx context.Context, rd io.Reader, args ...string) error {
	var got bytes.Buffer
	if _, err := io.Copy(&pausingWriter{p: p, w: &got}, rd); err != nil {
		return err
	}
	return p.fakePool.RunWithStdin(ctx, &got, args...)
}

type pausingWriter struct {
	p      *copyingPool
	w      io.Writer
	writes int
}

func (w *pausingWriter) Write(b []byte) (int, error) {
	w.p.largest = max(w.p.largest, len(b))
	w.writes++
	if w.writes == 1 && w.p.pause != nil {
		w.p.pause()
	}
	return w.w.Write(b)
}

// countingBody is a request body that counts what was read of it.
type countingBody struct {
	r io.Reader
	n atomic.Int64
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.n.Add(int64(n))
	return n, err
}

// putIntoSlot allows a slot on in for one item and puts body into it.
func putIntoSlot(t *testing.T, in *instance, pool *copyingPool, body io.Reader) string {
	t.Helper()
	in.svc.SetZFSHost(zfs.NewSSHHost(pool))
	in.svc.SetHostSSH(pool)
	switchReceiver(t, in, true)
	id := askSlot(t, in, "0b0b0b0b", "cache/appdata")
	if out := allowSlot(t, in, id); out["ok"] != true {
		t.Fatalf("allow = %v", out)
	}
	slot, _, _ := in.st.GetZFSReceiveSlot(id)
	token, err := secret.Decrypt(in.appKey, slot.TokenEnc)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/zfs/receive/"+id+"/members/cache%2Fappdata", body)
	req.Header.Set("Authorization", "Bearer "+string(token))
	w := httptest.NewRecorder()
	in.router.ServeHTTP(w, req)
	return w.Body.String()
}

func TestASlotHandsTheReceiveTheStreamInLargeWrites(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	pool := &copyingPool{fakePool: newFakePool(&atomic.Uint64{}, "tank")}
	if got := putIntoSlot(t, in, pool, bytes.NewReader(largeStream())); !strings.Contains(got, `"ok":true`) {
		t.Fatalf("receive = %s", got)
	}
	if pool.largest < 1<<20 {
		t.Errorf("the largest write into the receive was %d bytes, want a MiB", pool.largest)
	}
}

func TestASlotReadsTheStreamAheadOfAReceiveThatPauses(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	stream := largeStream()
	body := &countingBody{r: bytes.NewReader(stream)}
	var ahead int64
	pool := &copyingPool{fakePool: newFakePool(&atomic.Uint64{}, "tank")}
	pool.pause = func() {
		// A receive that writes out a transaction group takes nothing for a
		// while, and the source should not stall behind it.
		for deadline := time.Now().Add(5 * time.Second); body.n.Load() < int64(len(stream)) && time.Now().Before(deadline); {
			time.Sleep(10 * time.Millisecond)
		}
		ahead = body.n.Load()
	}
	if got := putIntoSlot(t, in, pool, body); !strings.Contains(got, `"ok":true`) {
		t.Fatalf("receive = %s", got)
	}
	if ahead < int64(len(stream)) {
		t.Errorf("%d of %d bytes were read while the receive paused", ahead, len(stream))
	}
}
