package api

import (
	"errors"
	"fmt"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

var (
	errPlaceLocationEstablished = errors.New("backups lie at the old address, and the new one does not hold the same repository")
	errPlaceHomeDomain          = errors.New("a domain keeps its backups at this place")
	errPlaceIsRepository        = errors.New("this place is itself a repository, so it takes no second role and no folder below it")
	errPlaceAddressTaken        = errors.New("a repository of this domain already lies at that address")
	errPlaceOff                 = errors.New("this place is switched off")
)

// writePlace writes a place and, when edit is set and changes them, the
// credential sets edit returns, in one WritePlace. The sets are read under
// credSetsMu, which every other writer of them takes too.
func (s *Service) writePlace(w store.PlaceWrite, edit func([]CloudCredSet) []CloudCredSet) (store.Place, error) {
	s.credSetsMu.Lock()
	defer s.credSetsMu.Unlock()
	if edit != nil {
		settings, err := s.store.GetSettings()
		if err != nil {
			return store.Place{}, err
		}
		sets, err := s.decodeCloudCredSets(settings)
		if err != nil {
			return store.Place{}, fmt.Errorf("read the credential sets: %w", err)
		}
		if next := edit(slices.Clone(sets)); !slices.Equal(next, sets) {
			enc, err := s.encodeCloudCredSets(next)
			if err != nil {
				return store.Place{}, err
			}
			w.CredSetsBlob = []byte(enc)
		}
	}
	return s.store.WritePlace(w)
}
