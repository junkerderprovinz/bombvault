package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// errPlaceProbeFailed marks a probe that reached no verdict about a place or a
// folder: the key was refused, the endpoint did not answer, or a local folder
// cannot take a repository.
var errPlaceProbeFailed = errors.New("the connection test failed")

// folderProbe is what one address holds.
type folderProbe struct {
	state   places.FolderState
	repoID  string
	problem *places.ProbeError
}

func probeProblem(err error) folderProbe {
	return folderProbe{state: places.FolderFailed, problem: &places.ProbeError{Code: placementCode(err), Message: scrubError(err)}}
}

// probeFolders opens the address of every folder of a place read-only and
// records what each holds. Folders that share an address are opened once.
func (s *Service) probeFolders(ctx context.Context, base string, folders places.Folders, mode restic.Mode) places.ProbeResult {
	res := places.ProbeResult{OK: true, Base: base, Folders: map[string]places.FolderState{}}
	seen := map[string]folderProbe{}
	for _, domain := range places.Domains {
		addr, ok := places.Address(base, folders, domain, "")
		if !ok {
			continue
		}
		fp, done := seen[addr]
		if !done {
			fp = s.probeFolder(ctx, addr, mode)
			seen[addr] = fp
		}
		res.Folders[domain] = fp.state
		if fp.repoID != "" {
			if res.RepoIDs == nil {
				res.RepoIDs = map[string]string{}
			}
			res.RepoIDs[domain] = fp.repoID
		}
		if fp.problem != nil {
			if res.Errors == nil {
				res.Errors = map[string]places.ProbeError{}
			}
			res.Errors[domain] = *fp.problem
			res.OK = false
		}
	}
	return res
}

// probeFolder reports what one address holds. A local folder is judged by what
// lies in it, so one holding other files is refused before restic runs there;
// a remote one is opened the way probeOffsiteRepo opens a target.
func (s *Service) probeFolder(ctx context.Context, addr string, mode restic.Mode) folderProbe {
	loc, err := s.resolveRepo(addr)
	if err != nil {
		return probeProblem(err)
	}
	if !restic.IsRemoteRepo(loc) {
		entries, err := os.ReadDir(loc)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return folderProbe{state: places.FolderAbsent}
		case err != nil:
			return probeProblem(err)
		case len(entries) == 0:
			return folderProbe{state: places.FolderEmpty}
		}
		if _, err := os.Stat(filepath.Join(loc, "config")); err != nil {
			return probeProblem(fmt.Errorf("%w: this folder holds other files and no restic repository", errPlaceProbeFailed))
		}
	}
	_, initialized, err := s.probeLocation(ctx, loc, mode)
	switch {
	case err != nil:
		return probeProblem(err)
	case !initialized:
		return folderProbe{state: places.FolderEmpty}
	}
	id, err := s.repoIDIn(ctx, loc, mode)
	if err != nil {
		return probeProblem(err)
	}
	return folderProbe{state: places.FolderRepository, repoID: id}
}

// repoIDIn reads the id of the repository at loc in whichever encryption mode
// opens it, as probeOffsiteRepo accepts either.
func (s *Service) repoIDIn(ctx context.Context, loc string, mode restic.Mode) (string, error) {
	read := func(m restic.Mode) (string, error) {
		pctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return s.engine.RepoID(pctx, loc, m)
	}
	id, err := read(mode)
	if err == nil {
		return id, nil
	}
	if id, oErr := read(s.oppositeMode(mode)); oErr == nil {
		return id, nil
	}
	return "", err
}
