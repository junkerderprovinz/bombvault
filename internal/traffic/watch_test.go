package traffic

import (
	"testing"
	"time"
)

var t0 = time.Unix(1_700_000_000, 0)

// feed records one sample every 10s from t0, each adding the given CPU
// seconds and bytes sent.
func feed(w *Watch, name string, n int, cpuSec float64, tx uint64) time.Time {
	var cpu, sent uint64
	at := t0
	for i := range n {
		at = t0.Add(time.Duration(i) * 10 * time.Second)
		w.Record(name, Sample{At: at, Running: true, CPUNanos: cpu, TxBytes: sent, HasNet: true})
		cpu += uint64(cpuSec * 1e9)
		sent += tx
	}
	return at
}

func TestRatesComeFromTheDifferenceOfTwoSamples(t *testing.T) {
	w := NewWatch(time.Hour, 30*time.Second)
	w.Record("app", Sample{At: t0, Running: true, CPUNanos: 0, TxBytes: 0, HasNet: true})
	w.Record("app", Sample{At: t0.Add(10 * time.Second), Running: true, CPUNanos: 5e9, TxBytes: 10_000, RxBytes: 5_000, HasNet: true})
	r := w.rates["app"]
	if len(r) != 1 {
		t.Fatalf("got %d rates, want 1", len(r))
	}
	if r[0].CPUPct != 50 || r[0].TxBps != 1000 || r[0].RxBps != 500 {
		t.Fatalf("rate = %+v, want 50%% CPU, 1000 B/s out, 500 B/s in", r[0])
	}
}

func TestARestartedContainerGivesNoRate(t *testing.T) {
	w := NewWatch(time.Hour, 30*time.Second)
	w.Record("app", Sample{At: t0, Running: true, CPUNanos: 9e9, TxBytes: 1e9, HasNet: true})
	w.Record("app", Sample{At: t0.Add(10 * time.Second), Running: true, CPUNanos: 1e8, TxBytes: 10, HasNet: true})
	if len(w.rates["app"]) != 0 {
		t.Fatalf("a counter reset produced a rate: %+v", w.rates["app"])
	}
}

func TestStreamingHoldsUntilTheQuietTimeHasPassed(t *testing.T) {
	w := NewWatch(time.Hour, 30*time.Second)
	last := feed(w, "plex", 4, 0.1, 5_000_000)
	rule := StreamRule{Servers: []string{"plex"}, Threshold: 250_000, Hold: 5 * time.Minute}
	if who, on := w.Streaming(rule, last); !on || who != "plex" {
		t.Fatalf("Streaming = %q, %v; want plex, true", who, on)
	}
	if _, on := w.Streaming(rule, last.Add(4*time.Minute)); !on {
		t.Fatal("the stream ended before the hold time passed")
	}
	if _, on := w.Streaming(rule, last.Add(6*time.Minute)); on {
		t.Fatal("the stream still counts after the hold time")
	}
}

func TestTrafficBelowTheThresholdIsNoStream(t *testing.T) {
	w := NewWatch(time.Hour, 30*time.Second)
	last := feed(w, "plex", 4, 0.1, 100_000)
	if _, on := w.Streaming(StreamRule{Servers: []string{"plex"}, Threshold: 250_000, Hold: time.Minute}, last); on {
		t.Fatal("10 kB/s counted as a stream")
	}
}

func TestHostNetworkedServerNeverStreams(t *testing.T) {
	w := NewWatch(time.Hour, 30*time.Second)
	w.Record("plex", Sample{At: t0, Running: true})
	w.Record("plex", Sample{At: t0.Add(10 * time.Second), Running: true, TxBytes: 1e9})
	if _, on := w.Streaming(StreamRule{Servers: []string{"plex"}, Threshold: 1, Hold: time.Minute}, t0.Add(10*time.Second)); on {
		t.Fatal("a container without network counters was seen streaming")
	}
}

func TestIdleNeedsTheWholeQuietTimeMeasured(t *testing.T) {
	w := NewWatch(time.Hour, 30*time.Second)
	rule := IdleRule{CPUPct: 10, NetBps: 125_000, Quiet: 3 * time.Minute}
	if idle, why := w.Idle("app", rule, t0); idle || why != BusyMeasuring {
		t.Fatalf("unmeasured container: Idle = %v, %q", idle, why)
	}
	last := feed(w, "app", 6, 0.01, 1000)
	if idle, why := w.Idle("app", rule, last); idle || why != BusyMeasuring {
		t.Fatalf("50s of samples: Idle = %v, %q; want measuring", idle, why)
	}
	w = NewWatch(time.Hour, 30*time.Second)
	last = feed(w, "app", 20, 0.01, 1000)
	if idle, why := w.Idle("app", rule, last); !idle {
		t.Fatalf("quiet for 190s: Idle = %v, %q", idle, why)
	}
}

func TestBusyCPUOrNetworkInTheQuietTimeIsNotIdle(t *testing.T) {
	rule := IdleRule{CPUPct: 10, NetBps: 125_000, Quiet: 3 * time.Minute}
	w := NewWatch(time.Hour, 30*time.Second)
	last := feed(w, "db", 20, 5, 0)
	if idle, why := w.Idle("db", rule, last); idle || why != BusyCPU {
		t.Fatalf("50%% CPU: Idle = %v, %q", idle, why)
	}
	w = NewWatch(time.Hour, 30*time.Second)
	last = feed(w, "nc", 20, 0.01, 10_000_000)
	if idle, why := w.Idle("nc", rule, last); idle || why != BusyNetwork {
		t.Fatalf("1 MB/s: Idle = %v, %q", idle, why)
	}
}

func TestOldActivityOutsideTheQuietTimeDoesNotCount(t *testing.T) {
	rule := IdleRule{CPUPct: 10, NetBps: 125_000, Quiet: time.Minute}
	w := NewWatch(time.Hour, 30*time.Second)
	w.Record("db", Sample{At: t0, Running: true, HasNet: true})
	w.Record("db", Sample{At: t0.Add(10 * time.Second), Running: true, CPUNanos: 9e9, HasNet: true})
	at := t0.Add(10 * time.Second)
	for i := 1; i <= 12; i++ {
		at = t0.Add(time.Duration(10+10*i) * time.Second)
		w.Record("db", Sample{At: at, Running: true, CPUNanos: 9e9, HasNet: true})
	}
	if idle, why := w.Idle("db", rule, at); !idle {
		t.Fatalf("busy two minutes ago only: Idle = %v, %q", idle, why)
	}
}

func TestStoppedOrUnreadableContainersCountAsIdle(t *testing.T) {
	rule := IdleRule{CPUPct: 10, NetBps: 125_000, Quiet: time.Hour}
	w := NewWatch(time.Hour, 30*time.Second)
	w.Record("off", Sample{At: t0})
	if idle, _ := w.Idle("off", rule, t0); !idle {
		t.Fatal("a stopped container is not idle")
	}
	w.RecordFailure("gone", t0)
	if idle, _ := w.Idle("gone", rule, t0); !idle {
		t.Fatal("a container that could not be read is not idle")
	}
}

func TestMediaServerIsIdleWhenNothingStreams(t *testing.T) {
	stream := &StreamRule{Threshold: 250_000, Hold: 2 * time.Minute}
	rule := IdleRule{CPUPct: 1, NetBps: 1, Quiet: time.Hour, Stream: stream}
	w := NewWatch(time.Hour, 30*time.Second)
	last := feed(w, "plex", 13, 3, 1_000)
	if idle, why := w.Idle("plex", rule, last); !idle {
		t.Fatalf("busy CPU but no stream: Idle = %v, %q", idle, why)
	}
	w = NewWatch(time.Hour, 30*time.Second)
	last = feed(w, "plex", 4, 0, 10_000_000)
	if idle, why := w.Idle("plex", rule, last); idle || why != BusyStreaming {
		t.Fatalf("streaming: Idle = %v, %q", idle, why)
	}
}

func TestRetainForgetsContainersNoLongerWatched(t *testing.T) {
	w := NewWatch(time.Hour, 30*time.Second)
	feed(w, "a", 3, 0, 0)
	feed(w, "b", 3, 0, 0)
	w.Retain([]string{"a"})
	if _, ok := w.last["b"]; ok {
		t.Fatal("b is still kept")
	}
	if _, ok := w.last["a"]; !ok {
		t.Fatal("a was dropped")
	}
}

func TestAMediaServerIsMeasuredForTheHoldBeforeItCountsAsIdle(t *testing.T) {
	stream := &StreamRule{Threshold: 250_000, Hold: 2 * time.Minute}
	rule := IdleRule{CPUPct: 10, NetBps: 125_000, Quiet: time.Minute, Stream: stream}
	w := NewWatch(time.Hour, 30*time.Second)
	w.Record("plex", Sample{At: t0, Running: true, HasNet: true})
	if idle, why := w.Idle("plex", rule, t0); idle || why != BusyMeasuring {
		t.Fatalf("one sample: Idle = %v, %q; want measuring", idle, why)
	}
	last := feed(w, "plex", 12, 0.01, 1_000)
	if idle, why := w.Idle("plex", rule, last); idle || why != BusyMeasuring {
		t.Fatalf("110s of samples: Idle = %v, %q; want measuring", idle, why)
	}
	// A pause longer than the gap starts the series again.
	resumed := last.Add(time.Minute)
	w.Record("plex", Sample{At: resumed, Running: true, HasNet: true})
	if idle, why := w.Idle("plex", rule, resumed); idle || why != BusyMeasuring {
		t.Fatalf("after a pause: Idle = %v, %q; want measuring", idle, why)
	}
	w = NewWatch(time.Hour, 30*time.Second)
	last = feed(w, "plex", 13, 0.01, 1_000)
	if idle, why := w.Idle("plex", rule, last); !idle {
		t.Fatalf("quiet for the whole hold: Idle = %v, %q", idle, why)
	}
}

// hostFeed records samples without network counters, the way Docker reports a
// container on the host network.
func hostFeed(w *Watch, name string, n int, cpuSec float64) time.Time {
	var cpu uint64
	at := t0
	for i := range n {
		at = t0.Add(time.Duration(i) * 10 * time.Second)
		w.Record(name, Sample{At: at, Running: true, CPUNanos: cpu})
		cpu += uint64(cpuSec * 1e9)
	}
	return at
}

func TestAMediaServerOnTheHostNetworkIsIdleByItsCPU(t *testing.T) {
	stream := &StreamRule{Threshold: 250_000, Hold: 2 * time.Minute}
	rule := IdleRule{CPUPct: 10, NetBps: 125_000, Quiet: 3 * time.Minute, Stream: stream}
	w := NewWatch(time.Hour, 30*time.Second)
	last := hostFeed(w, "plex", 20, 5)
	if idle, why := w.Idle("plex", rule, last); idle || why != BusyCPU {
		t.Fatalf("50%% CPU on the host network: Idle = %v, %q; want cpu", idle, why)
	}
	w = NewWatch(time.Hour, 30*time.Second)
	last = hostFeed(w, "plex", 10, 0.01)
	if idle, why := w.Idle("plex", rule, last); idle || why != BusyMeasuring {
		t.Fatalf("90s on the host network: Idle = %v, %q; want measuring", idle, why)
	}
	w = NewWatch(time.Hour, 30*time.Second)
	last = hostFeed(w, "plex", 20, 0.01)
	if idle, why := w.Idle("plex", rule, last); !idle {
		t.Fatalf("quiet on the host network: Idle = %v, %q", idle, why)
	}
}
