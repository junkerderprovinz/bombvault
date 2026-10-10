package api_test

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// recordProgress collects every event the service publishes until stop is
// called, which returns them in order.
func recordProgress(t *testing.T, svc *api.Service) (stop func() []progress.Event) {
	t.Helper()
	prog := progress.NewStore()
	svc.SetProgress(prog)
	ch, cancel := prog.Subscribe()
	var (
		wg     sync.WaitGroup
		events []progress.Event
	)
	wg.Go(func() {
		for e := range ch {
			events = append(events, e)
		}
	})
	return func() []progress.Event {
		cancel()
		wg.Wait()
		return events
	}
}

func eventsUnder(events []progress.Event, key string) []progress.Event {
	return slices.DeleteFunc(slices.Clone(events), func(e progress.Event) bool { return e.Key != key })
}

func TestBackupEverythingPublishesItsTotalUpFrontAndCountsEveryItem(t *testing.T) {
	svc, _, _, _ := everythingTestService(t, &fakeResticEngine{})
	stop := recordProgress(t, svc)

	if _, err := svc.BackupEverything(context.Background()); err != nil {
		t.Fatal(err)
	}

	pass := eventsUnder(stop(), progress.EverythingKey)
	if len(pass) == 0 {
		t.Fatal("the pass published nothing under its own key")
	}
	first, last := pass[0], pass[len(pass)-1]
	// One container, the flash drive and the self-backup.
	if !first.Active || first.Done != 0 || first.Total != 3 || first.Unit != "items" || first.Phase != "backup" {
		t.Fatalf("first event = %+v, want an active one with 0 of 3 items", first)
	}
	if last.Active || last.Done != 3 || last.Total != 3 || last.Percent != 100 {
		t.Fatalf("last event = %+v, want the end with 3 of 3 items", last)
	}
	var done []int64
	for _, e := range pass[:len(pass)-1] {
		done = append(done, e.Done)
	}
	if want := []int64{0, 1, 2, 3}; !slices.Equal(done, want) {
		t.Fatalf("done counts = %v, want %v", done, want)
	}
}

func TestBackupEverythingLeavesOutOfItsTotalWhatItSkips(t *testing.T) {
	svc, _, _, tg := everythingTestService(t, &fakeResticEngine{})
	stop := recordProgress(t, svc)

	if _, err := svc.BackupEverything(api.WithEverythingSkips(context.Background(), []string{tg.ID})); err != nil {
		t.Fatal(err)
	}

	pass := eventsUnder(stop(), progress.EverythingKey)
	if len(pass) == 0 || pass[0].Total != 2 {
		t.Fatalf("pass events = %+v, want a total of 2 from the first one", pass)
	}
}

func TestBackupOfARunningContainerShowsItsStepsOnTheProgressStream(t *testing.T) {
	svc, _, docker, tg := everythingTestService(t, &fakeResticEngine{})
	docker.inspect = model.Inspect{Name: tg.ContainerName, Running: true}
	stop := recordProgress(t, svc)

	if _, err := svc.Backup(context.Background(), tg.ContainerName); err != nil {
		t.Fatal(err)
	}

	var steps []string
	var committedAt string
	for _, e := range eventsUnder(stop(), "container:"+tg.ContainerName) {
		if len(steps) == 0 || steps[len(steps)-1] != e.Step {
			steps = append(steps, e.Step)
		}
		if e.Committed && committedAt == "" {
			committedAt = e.Step
		}
	}
	// The last entry is the event that ends the run, which names no step.
	want := []string{"", progress.StepStop, progress.StepRead, progress.StepStart, ""}
	if !slices.Equal(steps, want) {
		t.Fatalf("steps = %q, want %q", steps, want)
	}
	if committedAt != progress.StepRead {
		t.Fatalf("the restore point was reported written during %q, want it before the start", committedAt)
	}
}
