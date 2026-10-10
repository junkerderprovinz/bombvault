package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

func TestEverythingPassCorrectsItsTotalWhenADomainFindsFewerItems(t *testing.T) {
	prog := progress.NewStore()
	p := &everythingPass{svc: &Service{progress: prog}, planned: map[string]int{"containers": 3, "flash": 1}, total: 4}

	p.settle("containers", 1)
	p.itemDone()

	snap := prog.Snapshot()
	if len(snap) != 1 || snap[0].Done != 1 || snap[0].Total != 2 || snap[0].Percent != 50 {
		t.Fatalf("progress = %+v, want 1 of 2 items at 50 percent", snap)
	}
}

func TestEverythingPassWithNothingToDoEndsAtAHundredPercent(t *testing.T) {
	prog := progress.NewStore()
	ch, cancel := prog.Subscribe()
	defer cancel()
	p := &everythingPass{svc: &Service{progress: prog}, planned: map[string]int{}}

	p.publish(true)
	p.end()

	if first := <-ch; !first.Active || first.Percent != 0 || first.Total != 0 {
		t.Fatalf("first event = %+v, want an active one at 0 percent", first)
	}
	if last := <-ch; last.Active || last.Percent != 100 {
		t.Fatalf("last event = %+v, want the end at 100 percent", last)
	}
	if snap := prog.Snapshot(); len(snap) != 0 {
		t.Fatalf("the pass left %+v behind", snap)
	}
}
