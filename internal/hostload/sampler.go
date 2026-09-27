package hostload

import (
	"sort"
	"sync"
	"time"
)

// Summary is how busy things were over one run, as shares from 0 to 1.
// CPU is nil when /proc/stat was unreadable, UploadBps when /proc/net/dev was.
type Summary struct {
	Samples   int        `json:"samples"`
	CPU       *float64   `json:"cpu,omitempty"`
	Disks     []DiskLoad `json:"disks,omitempty"`
	UploadBps *float64   `json:"uploadBps,omitempty"`
}

// DiskLoad is one device's share of the run it spent busy. Label and Role
// are filled in by the caller that knows the paths.
type DiskLoad struct {
	Name  string  `json:"name"`
	Label string  `json:"label,omitempty"`
	Role  string  `json:"role,omitempty"`
	Busy  float64 `json:"busy"`
}

// topDisks is how many devices a summary keeps, the busiest first.
const topDisks = 3

type window struct {
	samples   int
	cpu       float64
	cpuN      int
	disks     map[string]float64
	sent      float64
	netSecs   float64
	startedAt time.Time
}

// Sampler reads the counters every few seconds while at least one run is
// open and adds each interval to every open run. Nothing is read while no
// run is open.
type Sampler struct {
	proc  string
	every time.Duration

	mu      sync.Mutex
	windows map[string]*window
	prev    Counters
	prevAt  time.Time
	ticking bool
}

// NewSampler samples the counters below proc every interval.
func NewSampler(proc string, every time.Duration) *Sampler {
	return &Sampler{proc: proc, every: every, windows: map[string]*window{}}
}

// maxWindowAge drops a run that never reported its end, so a lost finish
// cannot keep the sampler running for good.
const maxWindowAge = 72 * time.Hour

// Begin opens a run.
func (s *Sampler) Begin(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.windows[id] = &window{disks: map[string]float64{}, startedAt: time.Now()}
	if !s.ticking {
		s.prev, s.prevAt = Read(s.proc), time.Now()
		s.ticking = true
		go s.loop()
	}
}

// End closes a run and returns what it measured, or false for a run that
// was never opened or ended before the first sample.
func (s *Sampler) End(id string) (Summary, bool) {
	s.mu.Lock()
	w, ok := s.windows[id]
	delete(s.windows, id)
	s.mu.Unlock()
	if !ok || w.samples == 0 {
		return Summary{}, false
	}
	return w.summary(), true
}

func (s *Sampler) loop() {
	t := time.NewTicker(s.every)
	defer t.Stop()
	for range t.C {
		cur, at := Read(s.proc), time.Now()
		s.mu.Lock()
		for id, w := range s.windows {
			if at.Sub(w.startedAt) > maxWindowAge {
				delete(s.windows, id)
			}
		}
		if len(s.windows) == 0 {
			s.ticking = false
			s.mu.Unlock()
			return
		}
		s.addInterval(s.prev, cur, at.Sub(s.prevAt))
		s.prev, s.prevAt = cur, at
		s.mu.Unlock()
	}
}

// addInterval adds one interval to every open run. A counter that went
// backwards belongs to a device that was replaced and is skipped.
func (s *Sampler) addInterval(a, b Counters, dt time.Duration) {
	ms := float64(dt.Milliseconds())
	if ms <= 0 {
		return
	}
	cpu, haveCPU := 0.0, b.CPUTotal > a.CPUTotal && b.CPUBusy >= a.CPUBusy
	if haveCPU {
		cpu = float64(b.CPUBusy-a.CPUBusy) / float64(b.CPUTotal-a.CPUTotal)
	}
	for _, w := range s.windows {
		w.samples++
		if haveCPU {
			w.cpu += cpu
			w.cpuN++
		}
		for name, t := range b.DiskTicks {
			if p, ok := a.DiskTicks[name]; ok && t >= p {
				w.disks[name] += min(float64(t-p)/ms, 1)
			}
		}
		if a.HasNet && b.HasNet && b.TxBytes >= a.TxBytes {
			w.sent += float64(b.TxBytes - a.TxBytes)
			w.netSecs += dt.Seconds()
		}
	}
}

func (w *window) summary() Summary {
	sum := Summary{Samples: w.samples}
	if w.cpuN > 0 {
		cpu := w.cpu / float64(w.cpuN)
		sum.CPU = &cpu
	}
	if w.netSecs > 0 {
		bps := w.sent / w.netSecs
		sum.UploadBps = &bps
	}
	for name, busy := range w.disks {
		sum.Disks = append(sum.Disks, DiskLoad{Name: name, Busy: busy / float64(w.samples)})
	}
	sort.Slice(sum.Disks, func(i, j int) bool {
		if sum.Disks[i].Busy != sum.Disks[j].Busy {
			return sum.Disks[i].Busy > sum.Disks[j].Busy
		}
		return sum.Disks[i].Name < sum.Disks[j].Name
	})
	if len(sum.Disks) > topDisks {
		sum.Disks = sum.Disks[:topDisks]
	}
	return sum
}
