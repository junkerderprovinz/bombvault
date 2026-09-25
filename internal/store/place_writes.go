package store

import "errors"

// ErrPlaceDomainUnavailable refuses a row of a domain the place has no folder for.
var ErrPlaceDomainUnavailable = errors.New("this place has no folder for that domain")
