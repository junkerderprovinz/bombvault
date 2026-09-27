package api

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// errPlaceProbeFailed marks a probe that reached no verdict about a place or a
// folder: the key was refused, the endpoint did not answer, or a local folder
// cannot take a repository.
var errPlaceProbeFailed = errors.New("the connection test failed")

// errRESTPathDeep refuses a rest-server path of more than one folder. The
// server creates repositories at most two levels deep, and a place keeps each
// domain in a folder below its path, so the first copy there would fail.
var errRESTPathDeep = fmt.Errorf("%w: rest-server creates repositories at most two levels deep, so the folder can only be one name", errPlaceProbeFailed)

// restPathDeep reports whether a rest-server form's path leaves no room for a
// domain's folder below it.
func restPathDeep(fields map[string]string) bool {
	return strings.Contains(strings.Trim(strings.TrimSpace(fields["path"]), "/"), "/")
}

// folderProbe is what one address holds.
type folderProbe struct {
	state   places.FolderState
	repoID  string
	problem *places.ProbeError
}

func probeProblem(err error) folderProbe {
	return folderProbe{state: places.FolderFailed, problem: &places.ProbeError{Code: placementCode(err), Message: probeReason(err)}}
}

// probeReason is why a probe failed, without the words of errPlaceProbeFailed,
// which the code and every sentence shown around the reason already say.
func probeReason(err error) string {
	return strings.TrimPrefix(scrubError(err), errPlaceProbeFailed.Error()+": ")
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
			return probeProblem(fmt.Errorf("%w: BombVault cannot read this folder: %v", errPlaceProbeFailed, err))
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

// ProbePlace tests a new or stored place: no repository is created and no row
// is written. A stored place, named by PlaceID, is probed at its own address
// with its stored secrets, each replaced by a value the form holds, so a
// changed key can be tested before it is saved. A place that cannot be reached
// is a result with OK false; the error is for a request that names no known
// provider or place, or leaves a required field empty.
func (s *Service) ProbePlace(ctx context.Context, req ProbeRequest) (places.ProbeResult, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return places.ProbeResult{}, fmt.Errorf("read settings: %w", err)
	}
	pr, err := s.newPlaceProbe(req)
	if err != nil {
		return places.ProbeResult{}, err
	}
	fresh := pr.base == ""
	var res places.ProbeResult
	if fresh {
		switch {
		case pr.provider.ID == "b2":
			err = probeB2(ctx, &pr, &res)
		case pr.provider.Kind == places.KindS3:
			err = probeS3(ctx, &pr, &res)
		case pr.provider.Kind == places.KindAzure && strings.TrimSpace(pr.fields["container"]) == "":
			err = probeAzure(ctx, &pr, &res)
		}
		if err == nil {
			pr.base, err = places.Base(pr.provider, pr.fields, pr.placeID)
		}
	}
	res.Fields = publicFields(pr.provider, pr.fields)
	var missing places.MissingField
	switch {
	case errors.As(err, &missing) && ((pr.provider.Kind == places.KindS3 && missing == "bucket") ||
		(pr.provider.Kind == places.KindAzure && missing == "container")):
		// The bucket or container is chosen from what the probe listed, or typed.
		res.OK = true
		return res, nil
	case err != nil:
		return failedProbe(res, err)
	}
	// A new WebDAV place is reached through a remote named after the id it
	// gets when it is added, so the address probed here is not the one it will
	// have, and the answer names none.
	shown := pr.base
	if fresh && pr.provider.Kind == places.KindWebDAV {
		shown = ""
	}
	mode := s.probeMode(settings, pr)
	if pr.provider.Kind == places.KindLocal {
		if err := s.localPlaceReady(pr); err != nil {
			return failedProbe(res, err)
		}
	}
	// Folders below a repository would be repositories inside it, so a new
	// place whose address holds one already is offered as that repository.
	if fresh {
		if at := s.probeFolder(ctx, pr.base, mode); at.state == places.FolderRepository {
			res.OK, res.Base = true, shown
			res.RepoIDs = map[string]string{"": at.repoID}
			res.Facts = append(res.Facts, places.ProbeFact{Key: places.FactBaseIsRepository})
			return res, nil
		}
		if pr.provider.Kind == places.KindREST && restPathDeep(pr.fields) {
			return failedProbe(res, errRESTPathDeep)
		}
	}
	found := s.probeFolders(ctx, pr.base, pr.folders, mode)
	found.Base, found.Fields, found.Buckets, found.Facts = shown, res.Fields, res.Buckets, res.Facts
	return found, nil
}

func (s *Service) newPlaceProbe(req ProbeRequest) (placeProbe, error) {
	providerID := req.Provider
	var place store.Place
	if req.PlaceID != "" {
		var err error
		if place, err = s.store.GetPlace(req.PlaceID); err != nil {
			return placeProbe{}, err
		}
		providerID = place.Provider
	}
	p, ok := places.ProviderByID(providerID)
	if !ok {
		return placeProbe{}, fmt.Errorf("unknown provider %q", providerID)
	}
	fields := maps.Clone(req.Fields)
	if fields == nil {
		fields = map[string]string{}
	}
	pr := placeProbe{
		provider: p, fields: fields, creds: places.CredsFromFields(p, fields),
		placeID: probePlaceID, folders: places.DefaultFolders(),
	}
	if req.PlaceID == "" {
		return pr, nil
	}
	stored, err := s.placeCreds(place)
	if err != nil {
		return placeProbe{}, err
	}
	pr.creds = overlayCreds(stored, pr.creds)
	pr.placeID, pr.base, pr.folders = place.ID, place.Base, places.Folders(place.Folders)
	return pr, nil
}

// overlayCreds takes each value typed into the form over the stored one. A
// new WebDAV user without a new URL gets the stored URL moved to their files.
func overlayCreds(stored, typed places.Creds) places.Creds {
	return places.Creds{
		S3KeyID:        cmp.Or(typed.S3KeyID, stored.S3KeyID),
		S3Secret:       cmp.Or(typed.S3Secret, stored.S3Secret),
		S3Region:       cmp.Or(typed.S3Region, stored.S3Region),
		S3StorageClass: cmp.Or(typed.S3StorageClass, stored.S3StorageClass),
		RESTUser:       cmp.Or(typed.RESTUser, stored.RESTUser),
		RESTPassword:   cmp.Or(typed.RESTPassword, stored.RESTPassword),
		WebDAVURL:      cmp.Or(typed.WebDAVURL, places.WebDAVURLForUser(stored.WebDAVURL, stored.WebDAVUser, typed.WebDAVUser)),
		WebDAVVendor:   cmp.Or(typed.WebDAVVendor, stored.WebDAVVendor),
		WebDAVUser:     cmp.Or(typed.WebDAVUser, stored.WebDAVUser),
		WebDAVPass:     cmp.Or(typed.WebDAVPass, stored.WebDAVPass),
		AzureAccount:   cmp.Or(typed.AzureAccount, stored.AzureAccount),
		AzureKey:       cmp.Or(typed.AzureKey, stored.AzureKey),
	}
}

// probeMode opens a probed place the way its rows will be opened: with this
// instance's repository password and the environment of the place's kind.
func (s *Service) probeMode(settings store.Settings, pr placeProbe) restic.Mode {
	mode := s.ModeFor(settings)
	mode.Env = places.Env(pr.provider.Kind, pr.creds, pr.placeID)
	return mode
}

// localPlaceReady refuses a local place BombVault cannot write to, and a NAS
// share with nothing mounted, whether a device tile or the address names it:
// its empty mount point takes a test write as readily as a backup, and the
// backup would never reach the NAS.
func (s *Service) localPlaceReady(pr placeProbe) error {
	loc, err := s.resolveRepo(pr.base)
	if err != nil {
		return err
	}
	share := slices.Contains(pr.provider.PickRoots, "remotes") || places.UnderRemotes(pr.base)
	if share && !s.destinationMounted(loc) {
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
	res.OK, res.Code, res.Error = false, placementCode(err), probeReason(err)
	return res, nil
}

// probeRefusal is a failed probe as the place-probe-failed refusal carrying
// it; any other error stays as it is.
func probeRefusal(err error) error {
	res, err := failedProbe(places.ProbeResult{}, err)
	if err != nil {
		return err
	}
	return &probeFailedErr{result: res}
}
