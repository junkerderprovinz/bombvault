package traffic

import (
	"testing"
	"time"
)

func TestAContainerSendingAboveTheRateWithinTheWindowIsSending(t *testing.T) {
	w := NewWatch(time.Hour, 30*time.Second)
	last := feed(w, "game", 40, 0, 2_000_000)
	sending, measured := w.Sending("game", 100*1024, 5*time.Minute, last)
	if !sending || !measured {
		t.Fatalf("sending = %v, measured = %v; want a busy container", sending, measured)
	}
}

func TestAQuietContainerIsNotSending(t *testing.T) {
	w := NewWatch(time.Hour, 30*time.Second)
	last := feed(w, "game", 40, 0, 10_000)
	if sending, measured := w.Sending("game", 100*1024, 5*time.Minute, last); sending || !measured {
		t.Fatalf("sending = %v, measured = %v; want a quiet container", sending, measured)
	}
}

func TestABurstOlderThanTheWindowDoesNotCount(t *testing.T) {
	w := NewWatch(time.Hour, 30*time.Second)
	at := t0
	var sent uint64
	for i := range 60 {
		at = t0.Add(time.Duration(i) * 10 * time.Second)
		w.Record("game", Sample{At: at, Running: true, TxBytes: sent, HasNet: true})
		if i < 10 {
			sent += 50_000_000
		} else {
			sent += 1_000
		}
	}
	if sending, _ := w.Sending("game", 100*1024, 5*time.Minute, at); sending {
		t.Fatal("a burst eight minutes back counted for a five minute window")
	}
	if sending, _ := w.Sending("game", 100*1024, 9*time.Minute, at); !sending {
		t.Fatal("the burst was missed by a window that covers it")
	}
}

func TestAShortMeasurementIsNotEnough(t *testing.T) {
	w := NewWatch(time.Hour, 30*time.Second)
	last := feed(w, "game", 6, 0, 10)
	if _, measured := w.Sending("game", 100*1024, 5*time.Minute, last); measured {
		t.Fatal("a minute of readings counted as five")
	}
	if _, measured := w.Sending("never-read", 100*1024, time.Minute, last); measured {
		t.Fatal("a container never read counted as measured")
	}
}

func TestAStoppedContainerSendsNothing(t *testing.T) {
	w := NewWatch(time.Hour, 30*time.Second)
	w.Record("game", Sample{At: t0})
	if sending, measured := w.Sending("game", 1, time.Minute, t0); sending || !measured {
		t.Fatalf("sending = %v, measured = %v", sending, measured)
	}
}
