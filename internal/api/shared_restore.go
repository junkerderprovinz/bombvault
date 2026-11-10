package api

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// sharedOtherServer is another server's part of a shared repository, as the
// Restore page offers it. Name is empty for a host no member this server
// knows writes under.
type sharedOtherServer struct {
	Repo    string `json:"repo"`
	Host    string `json:"host"`
	Name    string `json:"name,omitempty"`
	Entries int    `json:"entries"`
	Newest  int64  `json:"newest,omitempty"`
	Error   string `json:"error,omitempty"`
}

var (
	errSharedRepoUnknown = errors.New("this server takes no part in that shared repository")
	errSharedHost        = errors.New("that is not a host a server of the group writes under")
	errSharedOwnHost     = errors.New("these are this server's own backups, which every entry restores from its Backups tab")
	errSharedUnreachable = errors.New("this server has no copy target in that shared repository, so it cannot reach it")
)

// SharedOtherServers lists the other servers whose backups lie in the shared
// repositories this server takes part in. They are read from the repository,
// so a server that joined after this one is there as well. When a repository
// cannot be read, the members this server knows of stand in for it with the
// reason.
func (s *Service) SharedOtherServers(ctx context.Context) ([]sharedOtherServer, error) {
	repos, err := s.store.ListSharedRepos()
	if err != nil {
		return nil, err
	}
	own := s.resticHost()
	out := []sharedOtherServer{}
	for _, sr := range repos {
		lctx, cancel := context.WithTimeout(ctx, offsiteListTimeout)
		found, err := s.sharedOthersIn(lctx, sr, own)
		cancel()
		if err != nil {
			for _, m := range sr.Members {
				if m.Host != own {
					out = append(out, sharedOtherServer{Repo: sr.ID, Host: m.Host, Name: m.Name, Error: scrubError(err)})
				}
			}
			continue
		}
		out = append(out, found...)
	}
	return out, nil
}

func (s *Service) sharedOthersIn(ctx context.Context, sr store.SharedRepo, own string) ([]sharedOtherServer, error) {
	repo, mode, err := s.sharedReach(sr)
	if err != nil {
		return nil, err
	}
	snaps, err := s.engine.SnapshotsOfEveryHost(ctx, repo, mode)
	if err != nil {
		return nil, err
	}
	byHost := map[string][]restic.Snapshot{}
	for _, sn := range snaps {
		if sn.Hostname != own {
			byHost[sn.Hostname] = append(byHost[sn.Hostname], sn)
		}
	}
	out := make([]sharedOtherServer, 0, len(byHost))
	for host, list := range byHost {
		v := sharedOtherServer{Repo: sr.ID, Host: host, Entries: len(identityTags(list))}
		if i := slices.IndexFunc(sr.Members, func(m store.SharedMember) bool { return m.Host == host }); i >= 0 {
			v.Name = sr.Members[i].Name
		}
		for _, sn := range list {
			if ts, err := time.Parse(time.RFC3339Nano, sn.Time); err == nil && ts.Unix() > v.Newest {
				v.Newest = ts.Unix()
			}
		}
		out = append(out, v)
	}
	// The servers this one knows by name first, then the hosts nobody claims.
	slices.SortFunc(out, func(a, b sharedOtherServer) int {
		return cmp.Or(compareBool(a.Name == "", b.Name == ""), strings.Compare(a.Name, b.Name), strings.Compare(a.Host, b.Host))
	})
	return out, nil
}

// sharedReach is where and how this server reads sr: as its owner in the
// folder, as a member the way its copies go there. A local folder without a
// target is one a recovery kit named after the server that held it was set
// up anew.
func (s *Service) sharedReach(sr store.SharedRepo) (string, restic.Mode, error) {
	return s.sharedReachAt(sr, "")
}

// sharedReachAt is sharedReach for a member that reads sr at the location at
// instead, with the login of its copies there. An empty at is sharedReach.
func (s *Service) sharedReachAt(sr store.SharedRepo, at string) (string, restic.Mode, error) {
	if s.ownsSharedRepo(sr) {
		dest, mode, _, err := s.sharedAccess(sr)
		return dest, mode, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", restic.Mode{}, err
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		return "", restic.Mode{}, err
	}
	if i := slices.IndexFunc(targets, func(t store.OffsiteTarget) bool { return sameRepoLocation(t.Repo, sr.Repo) }); i >= 0 {
		repo, err := s.resolveRepo(cmp.Or(at, targets[i].Repo))
		if err != nil {
			return "", restic.Mode{}, err
		}
		return repo, s.offsiteModeForTarget(settings, targets[i]), nil
	}
	if restic.IsRemoteRepo(sr.Repo) {
		return "", restic.Mode{}, errSharedUnreachable
	}
	repo, err := s.resolveRepo(sr.Repo)
	if err != nil {
		return "", restic.Mode{}, err
	}
	mode := s.ModeFor(settings)
	mode.Sharing = restic.SharingMember
	return repo, mode, nil
}

// OpenSharedOther opens the backups host wrote into the shared repository id
// as a read-only session. This server's key opens them, since every key of a
// restic repository opens the same master key, but the definitions of their
// containers and VMs are sealed with the other server's key, so those come
// back as files.
func (s *Service) OpenSharedOther(ctx context.Context, id, host string) (string, ForeignInventory, error) {
	if !sharedHostRe.MatchString(host) {
		return "", ForeignInventory{}, errSharedHost
	}
	if host == s.resticHost() {
		return "", ForeignInventory{}, errSharedOwnHost
	}
	repos, err := s.store.ListSharedRepos()
	if err != nil {
		return "", ForeignInventory{}, err
	}
	i := slices.IndexFunc(repos, func(sr store.SharedRepo) bool { return sr.ID == id })
	if i < 0 {
		return "", ForeignInventory{}, errSharedRepoUnknown
	}
	repo, mode, err := s.sharedReach(repos[i])
	if err != nil {
		return "", ForeignInventory{}, err
	}
	key, err := s.backupKey()
	if err != nil {
		return "", ForeignInventory{}, err
	}
	// A shared repository lists the snapshots of the host a mode names as its
	// own, so the session reads as that host. It never writes.
	mode.Host, mode.Sharing, mode.NoLock = host, restic.SharingMember, true
	inv, err := s.foreignInventory(ctx, repo, mode)
	if err != nil {
		return "", ForeignInventory{}, err
	}
	session, err := s.openOwnRepoSession(repo, key, mode)
	if err != nil {
		return "", ForeignInventory{}, err
	}
	s.markForeignDefinitions(foreignSession{id: session, repo: repo, key: key, mode: mode}, &inv)
	return session, inv, nil
}

// handleSharedOthers serves GET /api/shared-repo/others.
func (h *Handler) handleSharedOthers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.svc.SharedOtherServers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"servers": servers}))
}

// handleOpenSharedOther serves POST /api/shared-repo/others/open with the
// body {repo, host}.
func (h *Handler) handleOpenSharedOther(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo string `json:"repo"`
		Host string `json:"host"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	session, inv, err := h.svc.OpenSharedOther(r.Context(), body.Repo, body.Host)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"session": session, "inventory": inv}))
}
