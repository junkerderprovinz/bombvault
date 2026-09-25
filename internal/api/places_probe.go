package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
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

// ProbeRequest is a connection test before a place is added or changed.
type ProbeRequest struct {
	Provider string            `json:"provider"`
	Fields   map[string]string `json:"fields"`
	PlaceID  string            `json:"placeId,omitempty"`
}

// probePlaceID names the rclone remote a WebDAV place is probed through before
// it has an id of its own.
const probePlaceID = "probe"

// placeProbe is one probe on its way: the provider, the form as the probe
// completes it, the credentials, and the address once it is known.
type placeProbe struct {
	provider places.Provider
	fields   map[string]string
	creds    places.Creds
	placeID  string
	base     string
	folders  places.Folders
}

// ProbePlace tests a place before it is added and changes nothing: no
// repository is created and no row is written. A place that cannot be reached
// comes back as a result with OK false and a code; an error is left for a
// request that names no known provider or leaves a required field empty.
func (s *Service) ProbePlace(ctx context.Context, req ProbeRequest) (places.ProbeResult, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return places.ProbeResult{}, fmt.Errorf("read settings: %w", err)
	}
	pr, err := s.newPlaceProbe(req)
	if err != nil {
		return places.ProbeResult{}, err
	}
	var res places.ProbeResult
	pr.base, err = places.Base(pr.provider, pr.fields, pr.placeID)
	res.Fields = publicFields(pr.provider, pr.fields)
	if err != nil {
		return failedProbe(res, err)
	}
	mode, err := s.probeMode(settings, pr)
	if err != nil {
		return res, err
	}
	if pr.provider.Kind == places.KindLocal {
		if err := s.localPlaceReady(pr); err != nil {
			return failedProbe(res, err)
		}
	}
	found := s.probeFolders(ctx, pr.base, pr.folders, mode)
	found.Fields, found.Buckets, found.Facts = res.Fields, res.Buckets, res.Facts
	return found, nil
}

func (s *Service) newPlaceProbe(req ProbeRequest) (placeProbe, error) {
	p, ok := places.ProviderByID(req.Provider)
	if !ok {
		return placeProbe{}, fmt.Errorf("unknown provider %q", req.Provider)
	}
	fields := maps.Clone(req.Fields)
	if fields == nil {
		fields = map[string]string{}
	}
	return placeProbe{
		provider: p, fields: fields, creds: places.CredsFromFields(p, fields),
		placeID: probePlaceID, folders: places.DefaultFolders(),
	}, nil
}

// probeMode opens a probed place the way its rows will be opened: with this
// instance's repository password and the environment of the place's kind.
func (s *Service) probeMode(settings store.Settings, pr placeProbe) (restic.Mode, error) {
	env, err := places.Env(pr.provider.Kind, pr.creds, pr.placeID)
	if err != nil {
		return restic.Mode{}, err
	}
	mode := s.ModeFor(settings)
	mode.Env = env
	return mode, nil
}

// localPlaceReady refuses a local place BombVault cannot write to, and a NAS
// share with nothing mounted: its empty mount point takes a test write as
// readily as a backup, and the backup would fill the container instead.
func (s *Service) localPlaceReady(pr placeProbe) error {
	loc, err := s.resolveRepo(pr.base)
	if err != nil {
		return err
	}
	if slices.Contains(pr.provider.PickRoots, "remotes") && !s.destinationMounted(loc) {
		return fmt.Errorf("%w: nothing is mounted at this share; mount it on the server first", errPlaceProbeFailed)
	}
	if err := writableDir(loc); err != nil {
		return fmt.Errorf("%w: BombVault cannot write here: %v", errPlaceProbeFailed, err)
	}
	return nil
}

// writableDir creates and removes a throwaway file in dir, or in its deepest
// existing parent while dir is still to be created, which is where the first
// backup has to write.
func writableDir(dir string) error {
	for {
		if _, err := os.Stat(dir); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	f, err := os.CreateTemp(dir, ".bombvault-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

// publicFields is the form as the probe completed it, without its secrets, so
// the add request can send back what the probe filled in.
func publicFields(p places.Provider, fields map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range fields {
		hidden := slices.ContainsFunc(p.Fields, func(f places.Field) bool { return f.Key == key && f.Secret })
		if value != "" && !hidden {
			out[key] = value
		}
	}
	return out
}

// failedProbe turns a failure into the answer: a probe that reached no verdict
// is a result with OK false, anything else stays the request's error.
func failedProbe(res places.ProbeResult, err error) (places.ProbeResult, error) {
	if !errors.Is(err, errPlaceProbeFailed) {
		return res, err
	}
	res.OK, res.Code, res.Error = false, placementCode(err), scrubError(err)
	return res, nil
}
