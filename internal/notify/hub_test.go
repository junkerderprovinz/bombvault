package notify_test

import (
	"context"
	"testing"

	kit "github.com/junkerderprovinz/parleyport/kit/notify"

	"github.com/junkerderprovinz/bombvault/internal/notify"
)

type fakeHub struct {
	takes bool
	got   []kit.Event
}

func (h *fakeHub) Deliver(_ context.Context, e kit.Event) bool {
	h.got = append(h.got, e)
	return h.takes
}

func TestAHubThatTakesTheMessageSendsItInsteadOfTheChannels(t *testing.T) {
	wh, hc := newService(t), newService(t)
	hub := &fakeHub{takes: true}
	c := notify.Config{Channels: []kit.Channel{webhook("hook", wh.URL, kit.OnAlways), checks(hc.URL, nil)}}.WithHub(hub)

	if !notify.Send(context.Background(), c, "container", notify.Event{Title: "BombVault: backup FAILED", Message: "broke", Key: "container:nextcloud"}) {
		t.Fatal("a message the hub took counts as not delivered")
	}
	if n := len(wh.got()); n != 0 {
		t.Errorf("the webhook got %d messages the hub took", n)
	}
	if n := len(hc.got()); n != 1 {
		t.Errorf("the health check got %d pings, want its own one", n)
	}
	if len(hub.got) != 1 {
		t.Fatalf("the hub got %d messages", len(hub.got))
	}
	e := hub.got[0]
	if e.Title != "backup FAILED" || e.Severity != kit.SeverityFailure || e.Kind != "container.failed" || e.Key != "container:nextcloud" || e.Body != "broke" {
		t.Errorf("the hub got %+v", e)
	}
}

func TestAHubThatTurnsTheMessageDownLeavesItToTheChannels(t *testing.T) {
	wh := newService(t)
	hub := &fakeHub{}
	c := notify.Config{Channels: []kit.Channel{webhook("hook", wh.URL, kit.OnAlways)}}.WithHub(hub)

	if !notify.Send(context.Background(), c, "container", notify.Event{Title: "BombVault: backup FAILED", Message: "broke"}) {
		t.Fatal("the webhook did not take the message the hub turned down")
	}
	if len(hub.got) != 1 || len(wh.got()) != 1 {
		t.Errorf("hub got %d, webhook %d, want one each", len(hub.got), len(wh.got()))
	}
}

func TestAHubAloneIsEnoughToSend(t *testing.T) {
	c := notify.Config{}.WithHub(&fakeHub{takes: true})
	if !c.Active() || !c.TakesMessages() {
		t.Error("a config with only a hub counts as off")
	}
	if !c.Carries(notify.Event{Title: "BombVault", OK: true}) {
		t.Error("a hub does not carry a success, which closes what a failure opened")
	}
}

func TestAFoldedItemDoesNotReachTheHub(t *testing.T) {
	hub := &fakeHub{takes: true}
	c := notify.Config{ScheduledSummary: true}.WithHub(hub)
	notify.Send(notify.WithMessagesSuppressed(context.Background()), c, "container", notify.Event{Title: "BombVault", OK: true, ItemResult: true})
	if len(hub.got) != 0 {
		t.Errorf("the hub got %d messages the summary folds", len(hub.got))
	}
}
