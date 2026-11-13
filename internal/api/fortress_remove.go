package api

import (
	"encoding/json"
	"errors"

	"github.com/junkerderprovinz/bombvault/internal/secret"
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
		list, err := s.fortressRemovedList()
		if err != nil {
			return err
		}
		list = append(list, fortressRemoved{App: m.App.ID, Name: appName(m.App), Device: m.Device})
		plain, err := json.Marshal(list)
		if err != nil {
			return err
		}
		sealed, err := s.sealFortress(plain)
		if err != nil {
			return err
		}
		return s.store.SetFortressRemoved(sealed)
	}
	return errNoMember
}

// fortressRemovedList is the members this instance took out since the
// phrase last changed.
func (s *Service) fortressRemovedList() ([]fortressRemoved, error) {
	m, err := s.store.FortressMembership()
	if err != nil {
		return nil, err
	}
	list := []fortressRemoved{}
	if len(m.RemovedEnc) == 0 {
		return list, nil
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, m.RemovedEnc)
	if err != nil {
		return nil, err
	}
	return list, json.Unmarshal(plain, &list)
}
