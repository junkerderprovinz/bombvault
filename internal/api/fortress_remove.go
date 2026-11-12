package api

import (
	"cmp"
	"errors"
)

// fortressRemoved is a member this instance took out of the fortress since
// the phrase last changed. Until it changes, the phrase lets it back in.
type fortressRemoved struct {
	App    string `json:"app"`
	Name   string `json:"name"`
	Device string `json:"device"`
}

var errNoMember = errors.New("no other member of the fortress has that id")

// RemoveFortressMember takes another member out of the fortress, here and at
// every member in reach, and keeps its name until the phrase changes. This
// instance is no other member, so it leaves instead.
func (s *Service) RemoveFortressMember(id string) error {
	s.house.door.Lock()
	defer s.house.door.Unlock()
	f := s.fortressNow()
	if f == nil {
		return errNoFortress
	}
	for _, m := range f.Members() {
		if m.ID != id {
			continue
		}
		if err := f.Remove(id); err != nil {
			return err
		}
		s.house.mu.Lock()
		s.house.removed = append(s.house.removed, fortressRemoved{App: m.App.ID, Name: cmp.Or(m.App.Name, m.App.ID), Device: m.Device})
		s.house.mu.Unlock()
		return nil
	}
	return errNoMember
}

// forgetRemoved drops the members taken out so far, once the phrase that
// would let them back in is gone.
func (s *Service) forgetRemoved() {
	s.house.mu.Lock()
	s.house.removed = nil
	s.house.mu.Unlock()
}
