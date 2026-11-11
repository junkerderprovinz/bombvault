package api

import (
	"strings"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
)

// shipLogUpdatesOn reports whether ShipLog runs on device among members. It
// then updates the containers there and asks for a backup first, so two apps
// never update the same container.
func shipLogUpdatesOn(members []fortress.Member, device string) bool {
	here := deviceHost(device)
	if here == "" {
		return false
	}
	for _, m := range members {
		if m.App.ID == "shiplog" && deviceHost(m.Device) == here {
			return true
		}
	}
	return false
}

// deviceHost is a device name the way ShipLog compares it: without case and
// without a domain, so "Tower" is "tower.lan".
func deviceHost(device string) string {
	host, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(device)), ".")
	return host
}

// shipLogUpdates reports whether ShipLog updates the containers on this
// server.
func (s *Service) shipLogUpdates() bool {
	s.house.mu.Lock()
	f, device := s.house.f, s.house.device
	s.house.mu.Unlock()
	return f != nil && shipLogUpdatesOn(f.Members(), device)
}
