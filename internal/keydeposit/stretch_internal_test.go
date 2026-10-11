package keydeposit

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestOnlySoManyCodesWaitToBeStretched(t *testing.T) {
	salt := []byte("0123456789abcdef")
	// The one stretch at a time is held here, so every call below waits.
	stretching <- struct{}{}
	var done sync.WaitGroup
	for range maxWaiting + 1 {
		done.Go(func() {
			if _, err := Recipient("01234-56789-ABCDE-FGHJK", salt); err != nil {
				t.Errorf("a code that waited its turn: %v", err)
			}
		})
	}
	deadline := time.Now().Add(10 * time.Second)
	for waiting.Load() < maxWaiting+1 {
		if time.Now().After(deadline) {
			t.Fatal("the calls never queued")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := Recipient("01234-56789-ABCDE-FGHJK", salt); !errors.Is(err, ErrBusy) {
		t.Fatalf("one code more than may wait: %v, want %v", err, ErrBusy)
	}
	<-stretching
	done.Wait()
	if waiting.Load() != 0 {
		t.Fatalf("%d codes are still counted as waiting", waiting.Load())
	}
}
