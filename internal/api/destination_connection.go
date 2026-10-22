package api

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/remotes"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// connectionEdit changes how a saved destination is reached. Settings holds
// the fields the wizard asked for and is read as a patch: a field left out
// stays as stored, and an empty one is removed unless its value is one the
// server does not show, such as a password, which only a new value replaces.
// Dir is the bucket or folder; nil keeps it.
type connectionEdit struct {
	Settings map[string]string `json:"settings"`
	Dir      *string           `json:"dir"`
}

var (
	errDestinationHoldsBackups = errors.New("this destination holds backups, so its address, bucket and folder stay where they are: putting them somewhere else is a move")
	errBackupsNotThere         = errors.New("the backups this destination holds do not open through the changed connection, so it was not saved")
	errNoConnectionForm        = errors.New("this destination was not set up with a provider, so it has no connection to edit")
)

// EditDestinationConnection changes the connection of a destination after the
// same checks its creation ran. One that holds backups keeps its location, and
// every repository it holds has to open through the changed connection before
// anything is written. The warnings say what the change meant for the direct
// repositories of its domain targets.
func (s *Service) EditDestinationConnection(ctx context.Context, id string, e connectionEdit) (store.OffsiteTarget, []saveWarning, error) {
	d, ok, err := s.store.GetDestination(id)
	if err != nil {
		return store.OffsiteTarget{}, nil, err
	}
	if !ok {
		return store.OffsiteTarget{}, nil, store.ErrNotDestination
	}
	p, ok := remotes.FindProvider(d.Provider)
	if !ok {
		return store.OffsiteTarget{}, nil, errNoConnectionForm
	}
	held, err := s.store.DestinationTargets(d.ID)
	if err != nil {
		return store.OffsiteTarget{}, nil, err
	}
	warnings := []saveWarning{}
	switch p.Route {
	case remotes.RouteRclone:
		d, err = s.editRemoteConnection(ctx, d, p, held, e)
	case remotes.RouteS3, remotes.RouteRest:
		d, warnings, err = s.editSignInConnection(ctx, d, p, held, e)
	case remotes.RoutePath:
		d, err = s.editPathConnection(ctx, d, p, held, e)
	}
	if err != nil {
		return store.OffsiteTarget{}, nil, err
	}
	return d, warnings, nil
}

// editPathConnection moves a folder destination nothing copies to yet. The
// folder is all there is to its connection.
func (s *Service) editPathConnection(ctx context.Context, d store.OffsiteTarget, p remotes.Provider, held []store.OffsiteTarget, e connectionEdit) (store.OffsiteTarget, error) {
	repo := strings.TrimRight(strings.TrimSpace(e.Settings["path"]), "/")
	if repo == "" || repo == d.Repo {
		return d, nil
	}
	if len(held) > 0 {
		return store.OffsiteTarget{}, errDestinationHoldsBackups
	}
	if err := s.destinationClash(repo, d.ID); err != nil {
		return store.OffsiteTarget{}, err
	}
	if _, err := s.CheckDraft(ctx, draftRequest{Provider: p.ID, Settings: map[string]string{"path": repo}}); err != nil {
		return store.OffsiteTarget{}, err
	}
	d.Repo = repo
	return s.store.SaveDestination(d)
}

// editSignInConnection changes the credential set and the address of an S3 or
// rest-server destination.
func (s *Service) editSignInConnection(ctx context.Context, d store.OffsiteTarget, p remotes.Provider, held []store.OffsiteTarget, e connectionEdit) (store.OffsiteTarget, []saveWarning, error) {
	before, err := s.store.GetSettings()
	if err != nil {
		return store.OffsiteTarget{}, nil, err
	}
	creds, err := s.decodeCloudFor(before, d.CredsRef)
	if err != nil {
		return store.OffsiteTarget{}, nil, err
	}
	was := creds
	repo := d.Repo
	draft := draftRequest{Provider: p.ID}
	if p.Route == remotes.RouteRest {
		creds.RESTUser = patched(e.Settings, "user", creds.RESTUser)
		creds.RESTPassword = cmp.Or(e.Settings["pass"], creds.RESTPassword)
		if strings.TrimSpace(e.Settings["url"]) != "" {
			addr, err := restAddress(e.Settings["url"])
			if err != nil {
				return store.OffsiteTarget{}, nil, err
			}
			repo = "rest:" + addr
		}
		draft.Settings = map[string]string{"url": strings.TrimPrefix(repo, "rest:"), "user": creds.RESTUser, "pass": creds.RESTPassword}
	} else {
		creds.S3KeyID = patched(e.Settings, "access_key_id", creds.S3KeyID)
		creds.S3Secret = cmp.Or(e.Settings["secret_access_key"], creds.S3Secret)
		creds.S3Region = patched(e.Settings, "region", creds.S3Region)
		endpoint := s3Host(d.Repo)
		dir := strings.TrimPrefix(strings.TrimPrefix(d.Repo, "s3:"), endpoint+"/")
		if e.Dir != nil {
			dir = strings.Trim(strings.TrimSpace(*e.Dir), "/")
		}
		if dir == "" {
			return store.OffsiteTarget{}, nil, errors.New("choose a bucket first")
		}
		if typed, sent := e.Settings["endpoint"]; sent || (p.ID == "aws" && e.Settings["region"] != "") {
			if endpoint, err = s3Endpoint(p, map[string]string{"endpoint": typed, "region": creds.S3Region}); err != nil {
				return store.OffsiteTarget{}, nil, err
			}
		}
		repo = "s3:" + endpoint + "/" + dir
		draft.Dir = dir
		draft.Settings = map[string]string{
			"endpoint": endpoint, "region": creds.S3Region,
			"access_key_id": creds.S3KeyID, "secret_access_key": creds.S3Secret,
		}
	}

	if repo != d.Repo {
		if err := s.destinationClash(repo, d.ID); err != nil {
			return store.OffsiteTarget{}, nil, err
		}
	}
	switch {
	case len(held) == 0:
		if _, err := s.CheckDraft(ctx, draft); err != nil {
			return store.OffsiteTarget{}, nil, err
		}
	case repo != d.Repo:
		return store.OffsiteTarget{}, nil, errDestinationHoldsBackups
	default:
		if err := s.heldOpen(ctx, held, func(t store.OffsiteTarget) (string, restic.Mode) {
			mode := s.offsiteModeForTarget(before, t)
			mode.Env = cloudEnv(creds)
			return t.Repo, mode
		}); err != nil {
			return store.OffsiteTarget{}, nil, err
		}
	}

	shared, err := s.signInSharedBeyond(before, d)
	switch {
	case err != nil:
		return store.OffsiteTarget{}, nil, err
	case d.CredsRef != "" && creds == was:
		// An unchanged sign-in needs no write, and a shared one no copy.
	case shared:
		// Only this destination's repositories were tried with the change, so
		// the others keep the sign-in as it was.
		if d.CredsRef, err = s.addCredSet(d.Name, creds); err != nil {
			return store.OffsiteTarget{}, nil, err
		}
	default:
		if err := s.editCloudCredSets(func(sets []CloudCredSet) []CloudCredSet {
			for i := range sets {
				if sets[i].ID == d.CredsRef {
					sets[i].CloudCreds = creds
				}
			}
			return sets
		}); err != nil {
			return store.OffsiteTarget{}, nil, err
		}
	}
	d.Repo = repo
	saved, err := s.store.SaveDestination(d)
	if err != nil {
		return store.OffsiteTarget{}, nil, err
	}
	warnings := s.directCredsWarnings(ctx, before, func(_, target store.OffsiteTarget) bool { return target.DestinationID == d.ID })
	return saved, warnings, nil
}

// signInSharedBeyond reports whether the sign-in of d is used by more than d
// and its domain targets. The default is: every place that names no sign-in
// signs in with it.
func (s *Service) signInSharedBeyond(settings store.Settings, d store.OffsiteTarget) (bool, error) {
	if d.CredsRef == "" {
		return true, nil
	}
	sets, err := s.decodeCloudCredSets(settings)
	if err != nil {
		return false, err
	}
	if i := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == d.CredsRef }); i >= 0 && sets[i].Default {
		return true, nil
	}
	return s.store.CredsRefSharedBeyond(d.CredsRef, d.ID)
}

// patched returns the value a patch sets for key, or stored when the patch
// leaves the key out.
func patched(patch map[string]string, key, stored string) string {
	if v, sent := patch[key]; sent {
		return strings.TrimSpace(v)
	}
	return stored
}

// editRemoteConnection changes the options of a destination's rclone remote
// and the folder it sits in. The changed remote is tried under a name of its
// own first, so a copy that runs meanwhile keeps the working one.
func (s *Service) editRemoteConnection(ctx context.Context, d store.OffsiteTarget, p remotes.Provider, held []store.OffsiteTarget, e connectionEdit) (store.OffsiteTarget, error) {
	remote, dir, _ := strings.Cut(strings.TrimPrefix(d.Repo, "rclone:"), ":")
	if e.Dir != nil {
		dir = strings.Trim(strings.TrimSpace(*e.Dir), "/")
	}
	if atShares(p, dir) {
		return store.OffsiteTarget{}, errPickShare
	}
	repo := "rclone:" + remotes.Join(remote, dir)
	if repo != d.Repo && len(held) > 0 {
		return store.OffsiteTarget{}, errDestinationHoldsBackups
	}
	if repo != d.Repo {
		if err := s.destinationClash(repo, d.ID); err != nil {
			return store.OffsiteTarget{}, err
		}
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	stored, err := s.savedRemote(remote)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	draft, err := s.draftFor(p, e.Settings)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	options, err := s.layOver(ctx, stored, draft)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	unchanged := maps.Equal(options, stored)
	pinned := remotes.Draft{Backend: draft.Backend, Stored: options}
	if _, err := pinHostKey(ctx, &pinned); err != nil {
		return store.OffsiteTarget{}, err
	}
	maps.Copy(options, pinned.Settings)

	var check string
	if err := s.editRcloneConf(func(conf string) (string, error) {
		check = freeRemoteName(conf, remote+"-check")
		section, err := settingsSection(check, draft.Backend, options)
		if err != nil {
			return "", err
		}
		return replaceRcloneSection(conf, check, section), nil
	}); err != nil {
		return store.OffsiteTarget{}, err
	}
	defer s.removeRcloneRemote(check)

	if len(held) == 0 {
		_, err = s.remotes().Folders(ctx, check, nil, dir)
	} else {
		err = s.heldOpen(ctx, held, func(t store.OffsiteTarget) (string, restic.Mode) {
			return "rclone:" + check + strings.TrimPrefix(t.Repo, "rclone:"+remote), s.offsiteModeForTarget(settings, t)
		})
	}
	// The check may have refreshed the sign-in under its own name, and Box or
	// Jottacloud void the token it refreshed from. A refused change keeps that
	// refresh when it tried the remote as it was, so the sign-in is the same.
	if err != nil {
		if unchanged {
			_ = s.editRcloneConf(func(conf string) (string, error) {
				return carrySignIn(conf, check, remote), nil
			})
		}
		return store.OffsiteTarget{}, err
	}

	if err := s.editRcloneConf(func(conf string) (string, error) {
		if checked, ok := remoteOptions(conf, check); ok {
			for _, key := range rcloneSignInKeys {
				if v := checked[key]; v != "" {
					options[key] = v
				}
			}
		}
		section, err := settingsSection(remote, draft.Backend, options)
		if err != nil {
			return "", err
		}
		return replaceRcloneSection(conf, remote, section), nil
	}); err != nil {
		return store.OffsiteTarget{}, err
	}
	d.Repo = repo
	return s.store.SaveDestination(d)
}

// savedRemote returns the options of a remote of the stored rclone config,
// with the sign-in rclone refreshed since.
func (s *Service) savedRemote(remote string) (map[string]string, error) {
	conf, err := s.currentRcloneConf()
	if err != nil {
		return nil, err
	}
	options, found := remoteOptions(conf, remote)
	if !found {
		return nil, fmt.Errorf("the rclone config has no remote %s", remote)
	}
	return options, nil
}

// layOver returns a remote's stored options with a form's fields laid over
// them, as the config keeps them. A field the server shows and the form left
// empty drops its option; a secret left empty keeps the stored one. A pinned
// host key belongs to the host it was read from, so a new host or port drops
// it.
func (s *Service) layOver(ctx context.Context, stored map[string]string, draft remotes.Draft) (map[string]string, error) {
	backends, err := s.remotes().Backends(ctx)
	if err != nil {
		return nil, err
	}
	set, err := s.remotes().Settings(ctx, draft)
	if err != nil {
		return nil, err
	}
	options := maps.Clone(stored)
	for name, value := range draft.Settings {
		if strings.TrimSpace(value) == "" && remotes.Shown(backends[draft.Backend], name) {
			delete(options, name)
		}
	}
	maps.Copy(options, set)
	if options["host"] != stored["host"] || options["port"] != stored["port"] {
		delete(options, "host_keys")
	}
	return options, nil
}

// remoteOptions returns the options of one remote of an rclone config as they
// are stored, without its type.
func remoteOptions(conf, remote string) (map[string]string, bool) {
	for _, block := range splitRcloneSections(conf) {
		if block.name != remote {
			continue
		}
		options := map[string]string{}
		for line := range strings.SplitSeq(block.text, "\n") {
			key, value, ok := strings.Cut(line, "=")
			if key = strings.TrimSpace(key); !ok || key == "type" || !settingKeyRe.MatchString(key) {
				continue
			}
			options[key] = strings.TrimSpace(value)
		}
		return options, true
	}
	return nil, false
}

// heldOpen checks that every repository a destination holds is reached through
// a changed connection, and that one a copy has arrived in is still a
// repository there. at gives where a domain target's repository is looked for
// and how it is signed into.
func (s *Service) heldOpen(ctx context.Context, held []store.OffsiteTarget, at func(store.OffsiteTarget) (string, restic.Mode)) error {
	for _, t := range held {
		repo, mode := at(t)
		reachable, initialized, err := s.probeOffsiteRepo(ctx, repo, mode)
		// The backups are reached either way; their encryption mode is not
		// what the connection changes.
		if _, other := errors.AsType[*otherEncryptionErr](err); other {
			err = nil
		}
		if err != nil || !reachable {
			return fmt.Errorf("%w: %s is not reached (%s)", errBackupsNotThere, t.Domain, scrubError(err))
		}
		if initialized {
			continue
		}
		_, copied, err := s.store.LatestSuccessfulOffsiteRunForTarget(t.Domain, t.ID)
		if err != nil {
			return err
		}
		if copied {
			return fmt.Errorf("%w: there is no repository for %s", errBackupsNotThere, t.Domain)
		}
	}
	return nil
}

// handleUpdateDestinationConnection changes how a saved destination is
// reached: its sign-in, the options of its remote and, while nothing is stored
// there yet, its address and folder.
// PUT /api/offsite/destinations/{id}/connection
func (h *Handler) handleUpdateDestinationConnection(w http.ResponseWriter, r *http.Request) {
	var e connectionEdit
	if !decodeBody(w, r, &e) {
		return
	}
	d, warnings, err := h.svc.EditDestinationConnection(r.Context(), r.PathValue("id"), e)
	if errors.Is(err, store.ErrNotDestination) {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	v, err := h.svc.destinationView(d)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"destination": v, "warnings": warnings}))
}
