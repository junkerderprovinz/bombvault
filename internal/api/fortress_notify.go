package api

import (
	"context"
	"encoding/json"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	kit "github.com/junkerderprovinz/parleyport/kit/notify"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/bombvault/internal/notify"
)

const (
	// actionNotifyDeliver is how a TollRoll takes a message to send it.
	actionNotifyDeliver = "notify.deliver"
	// capabilityNotifyHub is what a TollRoll lists in its status while it
	// sends for the other apps.
	capabilityNotifyHub = "notify.hub"
)

// hubWait bounds one hand-over, so a TollRoll that does not answer leaves
// the message to BombVault's own channels in time.
const hubWait = 10 * time.Second

// hubLook is how long what a TollRoll's status said holds, and hubLookWait
// how long reading it again may take before a message.
const (
	hubLook     = time.Minute
	hubLookWait = 3 * time.Second
)

// hubAsker is the part of a fortress the hand-over needs.
type hubAsker interface {
	Members() []fortress.Member
	Act(ctx context.Context, id, action string, body []byte) ([]byte, error)
	Status(ctx context.Context, id, lang string) (status.Report, error)
}

// hubLooks remembers, per member, whether its status listed the hub and
// when it was read.
type hubLooks struct {
	mu   sync.Mutex
	seen map[string]hubSeen
}

type hubSeen struct {
	hub bool
	at  time.Time
}

// hubs lists the TollRolls that take messages now, in the order the fortress
// lists them. A status older than hubLook is read again.
func (l *hubLooks) hubs(ctx context.Context, f hubAsker, now time.Time) []string {
	var out []string
	for _, m := range f.Members() {
		if m.App.ID != "tollroll" || !m.Online() || !slices.Contains(m.Actions, actionNotifyDeliver) {
			continue
		}
		l.mu.Lock()
		s, ok := l.seen[m.ID]
		l.mu.Unlock()
		if !ok || now.Sub(s.at) > hubLook {
			s = hubSeen{at: now}
			lctx, cancel := context.WithTimeout(ctx, hubLookWait)
			if rep, err := f.Status(lctx, m.ID, "en"); err == nil {
				s.hub = slices.Contains(rep.Capabilities, capabilityNotifyHub)
			}
			cancel()
			l.mu.Lock()
			if l.seen == nil {
				l.seen = map[string]hubSeen{}
			}
			l.seen[m.ID] = s
			l.mu.Unlock()
		}
		if s.hub {
			out = append(out, m.ID)
		}
	}
	return out
}

// hubMessage is the body of notify.deliver.
type hubMessage struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Body     string `json:"body,omitempty"`
	Link     string `json:"link,omitempty"`
	Key      string `json:"key,omitempty"`
}

// deliverToHubs offers e to each hub in turn and reports whether one took
// it. Any answer but {"taken":true} passes it on.
func deliverToHubs(ctx context.Context, f hubAsker, hubs []string, e kit.Event) bool {
	body, err := json.Marshal(hubMessage{Kind: e.Kind, Severity: string(e.Severity), Title: e.Title, Body: e.Body, Link: e.Link, Key: e.Key})
	if err != nil {
		return false
	}
	for _, id := range hubs {
		actx, cancel := context.WithTimeout(ctx, hubWait)
		out, err := f.Act(actx, id, actionNotifyDeliver, body)
		cancel()
		if err != nil {
			log.Printf("notify: TollRoll %s did not take the message, sending it here: %v", id, err)
			continue
		}
		var answer struct {
			Taken bool `json:"taken"`
		}
		if json.Unmarshal(out, &answer) == nil && answer.Taken {
			return true
		}
	}
	return false
}

// fortressHub hands BombVault's messages to a TollRoll of the fortress.
type fortressHub struct {
	f    hubAsker
	hubs []string
}

func (h fortressHub) Deliver(ctx context.Context, e kit.Event) bool {
	return deliverToHubs(ctx, h.f, h.hubs, e)
}

// notifyHub is the hub messages go to first, nil outside a fortress or while
// no TollRoll of it takes messages.
func (s *Service) notifyHub() notify.Hub {
	f := s.fortressNow()
	if f == nil {
		return nil
	}
	hubs := s.hubLooks.hubs(context.Background(), f, time.Now())
	if len(hubs) == 0 {
		return nil
	}
	return fortressHub{f: f, hubs: hubs}
}
