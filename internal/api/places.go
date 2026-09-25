package api

import "errors"

var (
	errPlaceLocationEstablished = errors.New("backups lie at the old address, and the new one does not hold the same repository")
	errPlaceHomeDomain          = errors.New("a domain keeps its backups at this place")
	errPlaceIsRepository        = errors.New("this place is itself a repository, so it takes no second role and no folder below it")
	errPlaceAddressTaken        = errors.New("a repository of this domain already lies at that address")
	errPlaceOff                 = errors.New("this place is switched off")
)
