package store

import (
	"errors"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

// ErrPlaceDomainUnavailable refuses a row of a domain the place has no folder for.
var ErrPlaceDomainUnavailable = errors.New("this place has no folder for that domain")

// PlaceAddress is where a row of the domain with the address ending lies at
// the place. A row without a domain is the place's own repository at its
// base; false means the place has no folder for the domain.
func PlaceAddress(p Place, domain, suffix string) (string, bool) {
	return places.Address(p.Base, p.Folders, domain, suffix)
}
