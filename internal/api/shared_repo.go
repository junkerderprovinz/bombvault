package api

import (
	"log"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// sharingAt is the part this server has in the repository restic reaches at
// repo. A shared repository is only ever a copy target, so the modes of off-site
// targets are the ones that ask. When the store cannot say, the answer is a
// member's part: it clears no lock and lists and forgets only under this
// server's host, which is safe in any repository.
func (s *Service) sharingAt(repo string) restic.Sharing {
	sr, ok, err := s.store.SharedRepoAt(repo)
	if err != nil {
		log.Printf("api: could not read whether %s is shared, treating it as shared: %v", scrubRepoLocation(repo), err) //nolint:gosec // G706: credentials are scrubbed from the location
		return restic.SharingMember
	}
	if !ok {
		return restic.SharingNone
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		log.Printf("api: could not read this server's group id, treating it as a member of a shared repository: %v", err)
		return restic.SharingMember
	}
	return sr.Sharing(g.InstanceID)
}
