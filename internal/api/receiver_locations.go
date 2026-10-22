package api

import (
	"log"
	"net/url"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// restAddress is a rest-server location without the rest: prefix, a login in
// the URL or a trailing slash, so two spellings of one place compare equal.
func restAddress(loc string) string {
	loc = strings.TrimPrefix(strings.TrimSpace(loc), "rest:")
	if u, err := url.Parse(loc); err == nil && u.Host != "" {
		u.User = nil
		loc = u.String()
	}
	return strings.TrimRight(loc, "/")
}

// restLiesAt reports whether the location repo is base or lies below it.
func restLiesAt(repo, base string) bool {
	r, b := restAddress(repo), restAddress(base)
	return b != "" && (r == b || strings.HasPrefix(r, b+"/"))
}

// learnReceiverLocations remembers the storage locations that lie on a
// receiver that just answered, so they are still found once its member is out
// of reach or its address has changed.
func (s *Service) learnReceiverLocations(found []groupReceiverView) {
	dests, err := s.store.ListDestinations()
	if err != nil {
		log.Printf("receiver: list the storage locations: %v", err)
		return
	}
	for _, v := range found {
		if v.URL == "" {
			continue
		}
		for _, d := range dests {
			if !restLiesAt(d.Repo, v.URL) {
				continue
			}
			if err := s.store.NoteReceiverLocation(v.MemberID, restAddress(d.Repo)); err != nil {
				log.Printf("receiver: %v", err)
			}
		}
	}
}

// receiverDestinations returns the storage locations this instance knows to
// lie on a member's receiving server, whether the member is in reach or not.
func (s *Service) receiverDestinations(memberID string) ([]store.OffsiteTarget, error) {
	locs, err := s.store.ReceiverLocations()
	if err != nil || len(locs[memberID]) == 0 {
		return nil, err
	}
	dests, err := s.store.ListDestinations()
	if err != nil {
		return nil, err
	}
	var out []store.OffsiteTarget
	for _, d := range dests {
		if slices.ContainsFunc(locs[memberID], func(at string) bool { return restLiesAt(d.Repo, at) }) {
			out = append(out, d)
		}
	}
	return out, nil
}

// destinationRef names a storage location in an answer.
type destinationRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// switchOffReceiverDestinations switches off the storage locations on a
// member's receiving server once the Receiver role is withdrawn. The member
// drops this instance's login when it hears, and every copy sent there after
// that would fail on it.
func (s *Service) switchOffReceiverDestinations(memberID string) ([]destinationRef, error) {
	dests, err := s.receiverDestinations(memberID)
	if err != nil {
		return nil, err
	}
	off := []destinationRef{}
	for _, d := range dests {
		if !d.Enabled {
			continue
		}
		d.Enabled = false
		if _, err := s.store.SaveDestination(d); err != nil {
			return off, err
		}
		off = append(off, destinationRef{ID: d.ID, Name: d.Name})
	}
	return off, nil
}
