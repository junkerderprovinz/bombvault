package api

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
)

const actionUpdatesManaged = "updates.managed"

// shipLogWait bounds asking ShipLog which containers it updates, so a ShipLog
// that does not answer leaves the update to BombVault.
const shipLogWait = 30 * time.Second

// shipLogAsker is the part of a fortress that asks ShipLog.
type shipLogAsker interface {
	Members() []fortress.Member
	Act(ctx context.Context, id, action string, body []byte) ([]byte, error)
}

// shipLogManaged asks ShipLog on device which containers it updates itself,
// by their lower-case names. Its answer holds for the moment it is given, so
// every update step asks again.
func shipLogManaged(ctx context.Context, f shipLogAsker, device string) map[string]bool {
	here := deviceHost(device)
	managed := map[string]bool{}
	if here == "" {
		return managed
	}
	for _, m := range f.Members() {
		if m.App.ID != "shiplog" || deviceHost(m.Device) != here {
			continue
		}
		out, err := f.Act(ctx, m.ID, actionUpdatesManaged, []byte("{}"))
		if err != nil {
			log.Printf("fortress: ask ShipLog which containers it updates: %v", err)
			continue
		}
		var answer struct {
			AutoUpdate bool     `json:"autoUpdate"`
			Containers []string `json:"containers"`
		}
		if json.Unmarshal(out, &answer) != nil || !answer.AutoUpdate {
			continue
		}
		for _, c := range answer.Containers {
			managed[strings.ToLower(c)] = true
		}
	}
	return managed
}

// shipLogUpdatesContainer reports whether ShipLog on device updates container
// name, which it backs up through BombVault first.
func shipLogUpdatesContainer(ctx context.Context, f shipLogAsker, device, name string) bool {
	return shipLogManaged(ctx, f, device)[strings.ToLower(name)]
}

// deviceHost is a device name the way ShipLog compares it: without case and
// without a domain, so "Tower" is "tower.lan".
func deviceHost(device string) string {
	host, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(device)), ".")
	return host
}

// shipLogContainers is shipLogManaged for this server's fortress, empty
// outside one.
func (s *Service) shipLogContainers(ctx context.Context) map[string]bool {
	s.house.mu.Lock()
	f, device := s.house.f, s.house.device
	s.house.mu.Unlock()
	if f == nil {
		return map[string]bool{}
	}
	ctx, cancel := context.WithTimeout(ctx, shipLogWait)
	defer cancel()
	return shipLogManaged(ctx, f, device)
}
