package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ModeFor builds the restic Mode from the encryption setting: with encryption
// on the password is derived from APP_KEY, otherwise the repo has none.
func (s *Service) ModeFor(settings store.Settings) restic.Mode {
	// Decode the cloud creds once for both the backend-credential env vars and
	// the off-site S3 storage class, which share the same encrypted blob. A decode
	// failure logs and yields a zero CloudCreds, so the restic op fails clearly on
	// auth rather than panicking.
	c, err := s.decodeCloud(settings)
	if err != nil {
		log.Printf("api: cloud creds decode failed (ignoring): %v", err)
	}
	m := restic.Mode{Env: cloudEnv(c), StorageClass: c.S3StorageClass}
	if settings.EncryptionEnabled {
		m.Encrypted = true
		m.Password = restickey.Derive(s.cfg.AppKey)
	}
	return m
}

// resolveRepo turns a configured repo location into the value passed to
// restic -r. A restic remote backend (rclone:…, s3:…, sftp:…, used off-site)
// is passed verbatim; a local location is resolved as a relative subpath
// under the host mount root, rejecting traversal. A rejection comes back as
// operator guidance naming the relative-path convention and the value to
// enter instead (see repoPathError) rather than the raw paths sentinel (#138).
func (s *Service) resolveRepo(loc string) (string, error) {
	if restic.IsRemoteRepo(loc) {
		return loc, nil
	}
	repo, err := paths.Resolve(s.cfg.HostMountRoot, loc)
	if err != nil {
		return "", s.repoPathError(loc, err)
	}
	return repo, nil
}

// DiscoverSource reports which repository the discovery pass for a domain
// reads, so the Recovery wizard can name it instead of leaving the user to
// guess (#196). The wizard asks for an off-site repository in one step and
// then reads the domain's primary path in the next, and on a fresh install
// with nothing local left those are rarely the same place. Naming the folder
// does not make the wizard read the off-site copy, but it turns "my backups
// are gone" into "it looked in the wrong place".
func (s *Service) DiscoverSource(domain string) string {
	settings, err := s.store.GetSettings()
	if err != nil {
		return ""
	}
	var repo string
	switch domain {
	case "vms":
		repo, err = s.vmsRepoPath(settings)
	case "files":
		repo, err = s.filesRepoPath(settings)
	default:
		repo, err = s.containersRepoPath(settings)
	}
	if err != nil {
		return ""
	}
	return repo
}

// containersRepoPath resolves the restic repo for the containers domain.
func (s *Service) containersRepoPath(settings store.Settings) (string, error) {
	return s.resolveRepo(settings.ContainersPath)
}

// vmsRepoPath resolves the restic repo for the vms domain.
func (s *Service) vmsRepoPath(settings store.Settings) (string, error) {
	return s.resolveRepo(settings.VMsPath)
}

// flashRepoPath resolves the restic repo for the flash domain.
func (s *Service) flashRepoPath(settings store.Settings) (string, error) {
	return s.resolveRepo(settings.FlashPath)
}

// configRepoPath resolves the restic repo for the config self-backup domain.
func (s *Service) configRepoPath(settings store.Settings) (string, error) {
	return s.resolveRepo(settings.ConfigPath)
}

// filesRepoPath resolves the restic repo for the files domain.
func (s *Service) filesRepoPath(settings store.Settings) (string, error) {
	return s.resolveRepo(settings.FilesPath)
}

// fileSetRepoPath resolves the restic repo for one file set (#204): its own
// repository if it has one, otherwise the Folders domain repository. A domain
// path could already be a restic remote, but that moved every folder set at
// once; this moves one, for large static folders that only need a copy on a
// B2 or NAS share.
//
// The override goes through the same resolveRepo as the domain path, so the
// same string shapes work and the same containment rules apply: a relative
// subpath is resolved under the host mount root, a raw remote ("b2:…",
// "s3:…", "sftp:…", "rest:…", "rclone:…") is handed to restic verbatim.
//
// PruneDomain and CheckDomain still operate on the domain repository. A set
// in its own repository is backed up and restored there and its retention
// runs with it (applyRetention takes the repo it is handed), but a
// whole-domain prune or integrity check does not reach into it. The UI
// discloses that gap; closing it means teaching those two to iterate
// repositories.
func (s *Service) fileSetRepoPath(settings store.Settings, set store.FileSet) (string, error) {
	return s.itemRepoPath(set.Repo, func() (string, error) { return s.filesRepoPath(settings) })
}

// itemRepoPath is the one place a per-item repository override (#204) turns
// into a location, shared by containers, VMs and folder sets.
//
// The stored value is a named repository's id, not a location. Locations are
// written down once in Settings and picked per item, so ten containers do not
// mean typing the same bucket path ten times, and a location can be corrected
// in one place instead of in every item that copied it.
//
// An id that no longer resolves is an error, never a quiet fallback to the
// domain repository. Falling back would send the next backup somewhere else
// and look exactly like a working backup until somebody went looking for a
// snapshot that is in the other repo.
func (s *Service) itemRepoPath(repoID string, domainFallback func() (string, error)) (string, error) {
	id := strings.TrimSpace(repoID)
	if id == "" {
		return domainFallback()
	}
	named, err := s.store.GetNamedRepo(id)
	if err != nil {
		return "", fmt.Errorf("this item points at a repository that no longer exists; pick one again in Settings or clear the field")
	}
	if !named.Enabled {
		return "", fmt.Errorf("the repository %q is switched off, so nothing can be backed up to it", named.Name)
	}
	return s.resolveRepo(named.Repo)
}

// containerRepoPath resolves a container's per-item repository override (#204),
// falling back to the Containers domain repository.
func (s *Service) containerRepoPath(settings store.Settings, tg store.Target) (string, error) {
	return s.itemRepoPath(tg.Repo, func() (string, error) { return s.containersRepoPath(settings) })
}

// validateItemRepoID refuses a per-item repository choice (#204) at the HTTP
// boundary instead of at the next backup, where it would surface as a restic
// error inside a run record nobody is watching. Empty is always valid and
// means "use the domain repository".
//
// It checks what itemRepoPath checks at use time (the repository exists and
// is switched on), so a choice that passes here still works when the backup
// runs. DeleteNamedRepoIfUnused counts users and deletes in one transaction,
// so a repository cannot disappear in between.
//
// A direct repository serves only the domain of its target.
func (s *Service) validateItemRepoID(domain, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	named, err := s.store.GetNamedRepo(id)
	if err != nil {
		return errors.New("no such repository; pick one from the list in Settings")
	}
	if !named.Enabled {
		return fmt.Errorf("the repository %q is switched off", named.Name)
	}
	if named.CompanionOf != "" {
		target, _, err := s.store.GetOffsiteTarget(named.CompanionOf)
		if err != nil {
			return fmt.Errorf("read the target of %q: %w", named.Name, err)
		}
		if target.Domain != domain {
			return errForeignDomain
		}
	}
	if _, err := s.resolveRepo(named.Repo); err != nil {
		return fmt.Errorf("the repository %q does not resolve to a usable location: %w", named.Name, err)
	}
	return nil
}

// containerRepoForName is repoFor for one container, with the signature the
// call sites already have (a name and a source), so a per-item repository
// reaches them without each one reading the store itself.
//
// An off-site source still resolves to the domain's off-site target, as in
// fileSetRepoFor: off-site copies are configured per domain, and where an
// item keeps its primary says nothing about where its replica lives.
//
// A container without a stored row has no override and takes the domain
// repository; that is the first backup, not an error. Every other store error
// is returned: treating a locked database as "no row" would send the backup
// to the domain repository while the item's snapshots sit in its own, a green
// run with a history split across two places. itemRepoPath refuses that
// case, and this function sits in front of it.
func (s *Service) containerRepoForName(settings store.Settings, name, source string) (string, error) {
	if isOffsiteSource(source) {
		return s.repoFor(settings, "containers", source)
	}
	tg, err := s.store.GetTargetByContainer(name)
	if errors.Is(err, sql.ErrNoRows) {
		return s.containersRepoPath(settings)
	}
	if err != nil {
		return "", fmt.Errorf("read this container's repository: %w", err)
	}
	return s.containerRepoPath(settings, tg)
}

// vmRepoForName is containerRepoForName's twin for the VMs domain, including the
// no-row-is-not-an-error rule.
func (s *Service) vmRepoForName(settings store.Settings, name, source string) (string, error) {
	if isOffsiteSource(source) {
		return s.repoFor(settings, "vms", source)
	}
	vm, err := s.store.GetVMTargetByName(name)
	if errors.Is(err, sql.ErrNoRows) {
		return s.vmsRepoPath(settings)
	}
	if err != nil {
		return "", fmt.Errorf("read this VM's repository: %w", err)
	}
	return s.vmRepoPath(settings, vm)
}

// vmRepoPath resolves a VM's per-item repository override (#204), falling back
// to the VMs domain repository.
func (s *Service) vmRepoPath(settings store.Settings, vm store.VMTarget) (string, error) {
	return s.itemRepoPath(vm.Repo, func() (string, error) { return s.vmsRepoPath(settings) })
}

// fileSetRepoFor is fileSetRepoPath with a source, mirroring repoFor: an
// off-site source still resolves to the domain's off-site target, because an
// off-site copy is configured per domain and a per-set override says nothing
// about where its replica lives. Only the primary is per set.
func (s *Service) fileSetRepoFor(settings store.Settings, set store.FileSet, source string) (string, error) {
	if isOffsiteSource(source) {
		return s.repoFor(settings, "files", source)
	}
	return s.fileSetRepoPath(settings, set)
}

// EnsureRepo makes sure the restic repo at repo is ready to use with the
// configured encryption mode. It is idempotent and reconciles the mode:
//
//   - opens with mode                  → exists and consistent; nothing to do
//   - opens only with the opposite mode → the Encryption setting was toggled
//     against an existing repo; return a clear, actionable error instead of
//     letting every later restic call fail cryptically
//   - opens with neither mode          → not initialised yet, so create it
//
// The probe is `restic cat config` (cheap, needs no lock). Telling a real mode
// mismatch apart from a not-yet-created repo is what stops a flipped Encryption
// setting from silently breaking backups (issue #14).
func (s *Service) EnsureRepo(ctx context.Context, repo string, mode restic.Mode) error {
	// Fast path: the repo opens with the configured mode, so it exists and its
	// encryption mode matches. This is the common case on every backup after
	// the first.
	if s.engine.RepoOpens(ctx, repo, mode) {
		s.markRepoEstablished(repo) // remember it for the not-mounted guard (#55)
		return nil
	}
	// It did not open. Probe the opposite encryption mode (same backend creds):
	// if that opens it, the repo exists but was created under the other mode
	// because the user toggled the Encryption setting. Fail fast with an
	// actionable message rather than running Init (which would log "config
	// already exists") and failing every later backup against the mismatched
	// repo.
	if s.engine.RepoOpens(ctx, repo, s.oppositeMode(mode)) {
		return fmt.Errorf("this backup repository was created %s, but the Encryption setting is now %s, so "+
			"restic cannot open it after the change. Set Encryption back to %s, or point this backup at a "+
			"new, empty repository location",
			encryptionWord(!mode.Encrypted), enabledWord(mode.Encrypted), enabledWord(!mode.Encrypted))
	}
	// Opens with neither mode: not initialised (a brand-new location) or its
	// backing store vanished. Local repos need their directory; remote backends
	// do not.
	if !restic.IsRemoteRepo(repo) {
		// #55/#120: a repo was established here before but its `config` is now
		// missing. Two very different causes:
		//   - The backing store vanished, typically a remote share that mounts
		//     after the container started and is invisible right now.
		//     Re-initialising would write an empty repo that shadows the real
		//     backups once the share reappears, so refuse and surface "not
		//     mounted" (#55).
		//   - The destination is present, writable and mounted, but this repo is
		//     legitimately not there yet, e.g. a phantom marker from a pre-mount
		//     init on a UD disk that mounted over it later (#120). The established
		//     marker is permanent (nothing deletes it, so a restart cannot clear
		//     it), so the healthy disk has to be recognised and the repo
		//     re-established on it.
		// destinationMounted (kernel mount table, not a stat or write probe) tells
		// the two apart: an unmounted mountpoint dir is often still writable, which
		// is exactly the #55 case.
		if localRepoMissing(repo) && s.repoEstablished(repo) {
			if !s.destinationMounted(repo) {
				return ErrBackupPathNotMounted // #55: backing store not mounted
			}
			// #120: stale or phantom marker on a live disk. Drop it and fall through
			// to EnsureDir+Init so the repo is re-established on the mounted disk.
			s.clearRepoEstablished(repo)
		}
		// Genuine first run (marker unset): create the repo dir chain. The marker
		// guard above prevents re-initialising over an established but now
		// unmounted repo, so MkdirAll here is safe.
		if err := paths.EnsureDir(repo); err != nil {
			return fmt.Errorf("ensure repo dir: %w", err)
		}
	}
	if err := s.engine.Init(ctx, repo, mode); err != nil {
		// Tolerate a race / pre-existing repo: the scrubbed adapter error may not
		// name the cause, so re-probe with the configured mode before failing.
		if s.engine.RepoOpens(ctx, repo, mode) {
			s.markRepoEstablished(repo)
			return nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "already") {
			return fmt.Errorf("init repo: %w", err)
		}
	}
	// Mark established only when the repo verifiably opens now (a real config
	// was written), never on a no-op init, so the not-mounted guard can only
	// trip on a location that held a repo.
	if s.engine.RepoOpens(ctx, repo, mode) {
		s.markRepoEstablished(repo)
	}
	return nil
}

// ErrBackupPathNotMounted is returned when a local backup repo BombVault
// previously established is now unreadable, typically a remote share (e.g.
// under /mnt/remotes) that mounts after the container started and stays
// invisible to it until the mount is restored. BombVault refuses to
// re-initialise an empty repo over it, which would shadow the real backups,
// and surfaces this instead of a misleading "no backups" (#55).
var ErrBackupPathNotMounted = errors.New("backup path is not mounted yet: a remote backup share may mount late at boot; it recovers once the mount is available (restart BombVault if it persists)")

// markRepoEstablished records a successfully created/opened local repo so a later
// open-failure can be told apart from a fresh location (#55). Remote repos have
// no local backing store to vanish, so they are not tracked. Best-effort.
func (s *Service) markRepoEstablished(repo string) {
	if restic.IsRemoteRepo(repo) {
		return
	}
	if err := s.store.MarkRepoEstablished(repo); err != nil {
		log.Printf("api: mark repo established: %v", err)
	}
}

// clearRepoEstablished removes the established marker for a local repo, used when
// the destination is confirmed mounted but the repo legitimately is not there yet
// (a stale/phantom pre-mount marker, #120). Remote repos have no marker. Best-effort.
func (s *Service) clearRepoEstablished(repo string) {
	if restic.IsRemoteRepo(repo) {
		return
	}
	if err := s.store.ClearRepoEstablished(repo); err != nil {
		log.Printf("api: clear repo established: %v", err)
	}
}

// repoEstablishment is the three-way answer to "was this local repository
// ever established?". reposThatExist, offsiteReplicationSources and
// discoverNamesAcrossRepos decide whether to report a missing repository
// and need to tell "it was never created, so nothing is missing" from "the
// store could not say". A bool cannot carry that: repoEstablished's
// false-on-error is the right default for EnsureRepo, where false means "go
// ahead and init", and the wrong one here, where it means "stay silent
// about a repository that may hold every backup".
type repoEstablishment int

const (
	repoNeverEstablished repoEstablishment = iota
	repoWasEstablished
	repoEstablishmentUnknown
)

// repoEstablishmentOf answers the question with its uncertainty intact. Remote
// repositories have no local backing store to vanish, so they are never tracked
// and answer "never".
func (s *Service) repoEstablishmentOf(repo string) repoEstablishment {
	if restic.IsRemoteRepo(repo) {
		return repoNeverEstablished
	}
	ok, err := s.store.IsRepoEstablished(repo)
	if err != nil {
		// One retry before giving up. The usual failure is a local SQLite read
		// losing a race with a concurrent write, which the next attempt almost
		// always wins; without the retry a nightly verify, prune, unlock and drill
		// would all report failure over a repository that never held anything,
		// because "could not tell" is reported rather than swallowed.
		if ok, err = s.store.IsRepoEstablished(repo); err != nil {
			log.Printf("api: is repo established (twice): %v", err)
			return repoEstablishmentUnknown
		}
	}
	if ok {
		return repoWasEstablished
	}
	return repoNeverEstablished
}

// repoEstablished reports whether a local repo destination was previously
// established. It is false for remote repos and on any store error, the
// permissive default EnsureRepo wants. A caller deciding whether to report
// a missing repository wants repoEstablishmentOf instead.
func (s *Service) repoEstablished(repo string) bool {
	return s.repoEstablishmentOf(repo) == repoWasEstablished
}

// oppositeMode returns mode with its encryption flag flipped, preserving backend
// credentials (Env). The encrypted variant carries the APP_KEY-derived repo
// password so a probe can actually open an encrypted repo; the unencrypted
// variant clears it.
func (s *Service) oppositeMode(mode restic.Mode) restic.Mode {
	o := mode
	o.Encrypted = !mode.Encrypted
	if o.Encrypted {
		o.Password = restickey.Derive(s.cfg.AppKey)
	} else {
		o.Password = ""
	}
	return o
}

// enabledWord renders an Encryption setting state in the UI's wording.
func enabledWord(encrypted bool) string {
	if encrypted {
		return "enabled"
	}
	return "disabled"
}

// encryptionWord renders a repository's actual encryption mode.
func encryptionWord(encrypted bool) string {
	if encrypted {
		return "encrypted"
	}
	return "unencrypted"
}

// domainReposForOp is domainRepoSource for an operation that has to reach
// all of a domain's data rather than one repository: check, unlock, prune,
// and finding the repository a given snapshot lives in. With named
// repositories (#204), resolving the domain's own repository alone would
// give a green integrity check over a repository the item's data is not in,
// an unlock that leaves the stuck lock in place, a prune that never
// reclaims the space retention freed, and a delete button that reports "no
// matching ID" for a snapshot the list beside it shows.
//
// An off-site source still answers with exactly one repository: an
// off-site copy is configured per domain, and a per-item override says
// nothing about it.
//
// The third return value lists the repositories that belong to this domain
// and could not be included. A caller must report them (skippedError);
// covering three repositories out of four and calling it success is the
// failure these callers exist to avoid.
func (s *Service) domainReposForOp(domain, source string) (store.Settings, []domainRepoRef, []repoSkip, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.Settings{}, nil, nil, fmt.Errorf("read settings: %w", err)
	}
	if isOffsiteSource(source) {
		repo, rErr := s.repoFor(settings, domain, source)
		if rErr != nil {
			return settings, nil, nil, rErr
		}
		// An off-site destination is neither the domain's own repository nor a
		// named one; the zero Own/Named is exactly right.
		return settings, []domainRepoRef{{Loc: repo}}, nil, nil
	}
	repos, skipped, err := s.domainReposInUse(settings, domain)
	if err != nil {
		return settings, nil, nil, err
	}
	return settings, repos, skipped, nil
}

// domainReposInUse returns every repository a domain's items write to: the
// domain's own, plus each distinct named repository (#204) one of its
// items points at. The order is deterministic (the domain repo first, then
// the named ones in the order Settings lists them), so a caller that stops
// at the first hit prefers the domain repo.
//
// A named repository that is switched off or no longer resolves is skipped
// rather than failing the call, but it is reported: every skip comes back
// in the second return value, and an operation that covers less of a
// domain than the domain has must say so rather than report success over
// the remainder. Callers such as verify, prune, unlock, the restorability
// drill, snapshot delete, discovery and off-site replication never reach
// itemRepoPath, so this is where a broken override gets noticed. A
// switched-off repository is the ordinary case: the interface invites it
// when a share dies.
func (s *Service) domainReposInUse(settings store.Settings, domain string) ([]domainRepoRef, []repoSkip, error) {
	own, err := s.repoFor(settings, domain, "local")
	if err != nil {
		return nil, nil, err
	}
	out := []domainRepoRef{ownRef(own)}
	var skipped []repoSkip
	ids := map[string]bool{}
	// A store read that fails is itself a skip: the answer is no longer "the
	// domain has one repository", it is "I could not find out", and those two
	// must not look the same to the caller.
	switch domain {
	case "containers":
		tgs, tErr := s.store.ListTargets()
		if tErr != nil {
			return out, []repoSkip{{Name: "this domain's items", Reason: "their list could not be read", Unreachable: true}}, nil //nolint:nilerr // reported as a skip instead
		}
		for _, t := range tgs {
			if id := strings.TrimSpace(t.Repo); id != "" {
				ids[id] = true
			}
		}
	case "vms":
		vms, vErr := s.store.ListVMTargets()
		if vErr != nil {
			return out, []repoSkip{{Name: "this domain's items", Reason: "their list could not be read", Unreachable: true}}, nil //nolint:nilerr // see above
		}
		for _, v := range vms {
			if id := strings.TrimSpace(v.Repo); id != "" {
				ids[id] = true
			}
		}
	case "files":
		sets, fErr := s.store.ListFileSets()
		if fErr != nil {
			return out, []repoSkip{{Name: "this domain's items", Reason: "their list could not be read", Unreachable: true}}, nil //nolint:nilerr // see above
		}
		for _, f := range sets {
			if id := strings.TrimSpace(f.Repo); id != "" {
				ids[id] = true
			}
		}
	}
	named, err := s.store.ListNamedRepos()
	if err != nil {
		return out, []repoSkip{{Name: "the named repositories", Reason: "their list could not be read", Unreachable: true}}, nil //nolint:nilerr // see above
	}
	for _, n := range named {
		if !ids[n.ID] {
			continue // nothing in this domain points at it
		}
		if !n.Enabled {
			skipped = append(skipped, repoSkip{Name: scrubSafeName(n.Name), Reason: "switched off"})
			continue
		}
		loc, rErr := s.resolveRepo(n.Repo)
		if rErr != nil {
			skipped = append(skipped, repoSkip{Name: scrubSafeName(n.Name), Reason: "its location does not resolve", Unreachable: true})
			continue
		}
		if sameRepoLocation(loc, own) {
			continue // the same place as the domain's own, already in the list
		}
		out = append(out, namedRef(loc, n))
	}
	return out, skipped, nil
}

// domainTagPrefix is the identity-tag prefix every snapshot of a domain carries
// ("container:<name>", "vm:<name>", "fileset:<name>"). Empty for flash and
// config, which have no per-item repositories and therefore cannot share one.
func domainTagPrefix(domain string) string {
	switch domain {
	case "containers":
		return "container:"
	case "vms":
		return "vm:"
	case "files":
		return "fileset:"
	}
	return ""
}

// domainRepoRef is one repository of a domain together with what it is.
// Deciding that by slice position ("index 0 is the domain's own
// repository") breaks as soon as a caller reshapes the slice: a filter
// that drops a never-created repository from the head would promote a
// named repository into the place that means "the domain's own", and the
// off-site copy would stop narrowing to this domain's snapshots. With the
// identity carried along, reshaping a list cannot change what an element
// means.
type domainRepoRef struct {
	// Loc is the resolved location, the string restic is handed.
	Loc string
	// Own marks the domain's own repository (Settings.<Domain>Path).
	Own bool
	// Named is the named repository's row (#204). Zero value when Own.
	Named store.OffsiteTarget
	// CountOnly marks a source no item is copied from. It is listed, so a
	// target's keep-policy knows its items still exist here, and never copied.
	CountOnly bool
}

// ownRef and namedRef are the two ways a reference is created, so nobody has to
// remember which fields go together.
func ownRef(loc string) domainRepoRef { return domainRepoRef{Loc: loc, Own: true} }

func namedRef(loc string, row store.OffsiteTarget) domainRepoRef {
	return domainRepoRef{Loc: loc, Named: row}
}

// refFor builds a reference for a location whose identity the caller does
// not already know: the post-backup replication hook, which gets whatever
// repository the item it just backed up uses.
func (s *Service) refFor(settings store.Settings, domain, loc string) domainRepoRef {
	if named, ok := s.namedRepoForLocation(loc); ok {
		return namedRef(loc, named)
	}
	// A domain repository that does not resolve leaves the location unidentified,
	// and an unidentified direct repository loses its tag and its exclusion from
	// the copies, so the failure is said rather than swallowed by the comparison.
	if own, err := s.repoFor(settings, domain, "local"); err != nil {
		log.Printf("api: %s: the domain repository does not resolve, so %s could not be identified: %v", domain, shortRepoName(loc), err) //nolint:gosec // G706: the domain is a fixed literal and the location is shortened
	} else if sameRepoLocation(own, loc) {
		return ownRef(loc)
	}
	// Neither the domain's own nor a known named row: an off-site destination
	// or a location resolved from a source string. Not Own and no named row,
	// which is what the consumers need to know.
	return domainRepoRef{Loc: loc}
}

// repoModeFor builds the restic mode for one repository, choosing the
// builder by source. Every operation that has already resolved which
// repository it is about to open uses it: the maintenance ones (verify,
// unlock, prune, drill, snapshot delete) and the readers (snapshot lists,
// file listings, diffs, restore plans) alike.
//
// An off-site location is described by its own target row (its credential
// set, storage class and caps), which offsiteModeForTarget reads.
// primaryModeFor describes a domain's primary repository; applied to an
// off-site copy it would authenticate somebody else's bucket with the
// primary's keys and impose the primary's bandwidth caps on a link they
// were never set for. Readers of an item's named repository likewise need
// that row's credentials, storage class and caps, or an archive the writer
// could write could not be opened again from the interface.
func (s *Service) repoModeFor(settings store.Settings, domain, source, repo string) restic.Mode {
	if isOffsiteSource(source) {
		if t, err := s.offsiteTargetForSource(settings, domain, source); err == nil {
			return s.offsiteModeForTarget(settings, t)
		}
		return s.ModeFor(settings)
	}
	return s.primaryModeFor(settings, domain, repo)
}

// repoSkip names one repository an operation could not cover, and why, so
// the operation can say it covered less than the whole domain instead of
// reporting success over what was left.
type repoSkip struct {
	Name   string
	Reason string
	// Note marks a skip the operator cannot act on: a permanent consequence of
	// how the instance is configured rather than something that went wrong. It
	// is still reported (skipNames carries it into the answer), but
	// skippedError leaves it out, so it never becomes the operation's error.
	//
	// The error is the failure channel: it stamps the run row red and fires a
	// "FAILED" notification. A shared named repository that can only ever get
	// the stale-lock clear is true, worth saying and unchangeable; delivered as
	// an error, a supported configuration would report failure every night and
	// teach the operator to ignore the one message that matters.
	Note bool
	// Unreachable marks a skip where this box could not open the repository at
	// all, so nothing here can say whether its contents still exist.
	//
	// It is separate from Note because "should this fail the operation" and
	// "could this repository's data be gone" are different questions. A
	// repository the operator switched off is a failure for an operation asked
	// to cover the whole domain (its items are not being backed up), and it is
	// not unreachable: it sits there with its data intact.
	//
	// The off-site retention gate is the one consumer: it must not age a
	// destination that may be the last copy of something. Keyed on "did
	// anything fail" instead, a domain with one switched-off repository would
	// never have its off-site destination aged again on an install where the
	// post-backup hook is the only replication.
	Unreachable bool
	// Ref is the repository the skip is about, when there is one, so a pass that
	// reads the placement later can tell whether anything is copied from it.
	Ref domainRepoRef
}

// skipNames renders a skip list for a JSON response: the repository's own
// name and why it could not be covered, one readable line each. Never a
// location: a path would be redacted on the way out and means nothing to a
// reader anyway.
func skipNames(skipped []repoSkip) []string {
	if len(skipped) == 0 {
		return nil
	}
	out := make([]string, 0, len(skipped))
	for _, s := range skipped {
		out = append(out, fmt.Sprintf("%s (%s)", s.Name, s.Reason))
	}
	return out
}

// actionableSkips drops the Notes and keeps the skips that say something
// went wrong: the ones an operator can act on, and the only ones that may
// fail an operation, colour a pill or hold back a destructive maintenance
// step. Every consumer of a skip list that decides rather than displays
// goes through here, so "permanent and correct" means the same thing in
// all of them.
func actionableSkips(skipped []repoSkip) []repoSkip {
	out := make([]repoSkip, 0, len(skipped))
	for _, s := range skipped {
		if s.Note {
			continue // reported, but not a failure; see repoSkip.Note
		}
		out = append(out, s)
	}
	return out
}

// unreachableSkips keeps the skips where this box could not open the
// repository, so nothing it saw can say whether that repository's contents
// still exist.
//
// The off-site retention gate is the one consumer, and it needs this
// question rather than actionableSkips'. The two differ on the case that
// matters: a repository the operator switched off is a failure for an
// operation asked to cover the whole domain, but it is not unreachable; it
// sits there with its data, so aging the off-site destination cannot leave
// its items without a copy.
func unreachableSkips(skipped []repoSkip) []repoSkip {
	out := make([]repoSkip, 0, len(skipped))
	for _, s := range skipped {
		if s.Unreachable {
			out = append(out, s)
		}
	}
	return out
}

// skippedError turns a skip list into the error an operation returns after
// doing what it could. The work is not abandoned (checking three
// repositories out of four beats checking none), but the outcome is not a
// success either: a run record must not say "success" about a domain half
// of whose data was never opened.
//
// `what` is a noun phrase naming the operation ("this prune", "this
// unlock"), because it is the subject of the sentence below.
func skippedError(what string, skipped []repoSkip) error {
	real := actionableSkips(skipped)
	if len(real) == 0 {
		return nil
	}
	parts := make([]string, 0, len(real))
	for _, s := range real {
		parts = append(parts, fmt.Sprintf("%s (%s)", s.Name, s.Reason))
	}
	return fmt.Errorf("%s covered only part of this domain: %s", what, strings.Join(parts, ", "))
}

// nothingCoveredError is skippedError's counterpart for an operation that
// covered nothing. It is a separate sentence, because "covered only part"
// is false there, and it is the message a single-repository domain whose
// repository went away is most likely to see.
func nothingCoveredError(skipped []repoSkip) error {
	real := actionableSkips(skipped)
	if len(real) == 0 {
		return nil
	}
	parts := make([]string, 0, len(real))
	for _, s := range real {
		parts = append(parts, fmt.Sprintf("%s (%s)", s.Name, s.Reason))
	}
	return fmt.Errorf("nothing in this domain could be opened: %s", strings.Join(parts, ", "))
}

// refName names a repository for a message: a named repository by its name, the
// domain's own by a shortened location. A name is what the operator recognises,
// and unlike a path it survives the error scrubber.
func (s *Service) refName(r domainRepoRef) string {
	if !r.Own && strings.TrimSpace(r.Named.Name) != "" {
		return scrubSafeName(r.Named.Name)
	}
	return shortRepoName(r.Loc)
}

// scrubSafeName makes an operator's free-text repository name survive the
// error scrubber. The name goes into skip lists, verify wrappers and run
// rows, all of which leave through scrubError, whose absolute-path regex
// redacts any slash-led token, so a repository called "NAS/cold" would
// reach the screen as "NAS[path]", matching nothing in the picker.
//
// Slashes become a middle dot rather than being dropped, because the name
// has to stay recognisable: "NAS · cold" still reads as what was typed.
// shortRepoName returns a single path segment for the same reason.
func scrubSafeName(name string) string {
	name = strings.TrimSpace(name)
	if !strings.ContainsAny(name, `/\`) {
		return name
	}
	r := strings.NewReplacer("/", " · ", `\`, " · ")
	return strings.Join(strings.Fields(r.Replace(name)), " ")
}

// ContainerPath returns the resolved absolute containers backup path, used
// by the path-writable probe. Returns "" if it cannot be resolved.
func (s *Service) ContainerPath() string {
	settings, err := s.store.GetSettings()
	if err != nil {
		return ""
	}
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return ""
	}
	return repo
}

// repoFor resolves the restic repo of a domain ("containers", "vms", "flash",
// "config" or "files") and source. An off-site source is the target
// offsiteTargetForSource resolves and fails where that refuses; anything else
// ("" or "local") is the domain's own repo.
func (s *Service) repoFor(settings store.Settings, domain, source string) (string, error) {
	if isOffsiteSource(source) {
		target, err := s.offsiteTargetForSource(settings, domain, source)
		if err != nil {
			return "", err
		}
		return s.resolveRepo(target.Repo)
	}
	switch domain {
	case "containers":
		return s.containersRepoPath(settings)
	case "vms":
		return s.vmsRepoPath(settings)
	case "flash":
		return s.flashRepoPath(settings)
	case "config":
		return s.configRepoPath(settings)
	case "files":
		return s.filesRepoPath(settings)
	default:
		return "", fmt.Errorf("unknown domain %q", domain)
	}
}

// domainRepoSource resolves one repository for a domain and source
// ("local"|"offsite"), returning the settings alongside it so callers
// don't re-read them.
//
// An operation that has to reach all of a domain's data wants
// domainReposForOp instead; this one is for the paths that address a
// single repository.
func (s *Service) domainRepoSource(domain, source string) (store.Settings, string, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.Settings{}, "", fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, domain, source)
	return settings, repo, err
}

// localRepoMissing reports whether a local repo has not been initialised
// yet (no `config` marker). It is always false for a remote repo
// (rest:/s3:/b2:/…), which has no local marker to stat; its emptiness is
// decided by listing it. The off-site view (often a remote repo) therefore
// must not use a local config check, or it would always look empty even
// when snapshots exist.
func localRepoMissing(repo string) bool {
	if restic.IsRemoteRepo(repo) {
		return false
	}
	_, statErr := os.Stat(filepath.Join(repo, "config")) //nolint:gosec // G703: repo is an operator-configured location validated under the mount root on save; source only selects which configured location
	return errors.Is(statErr, fs.ErrNotExist)
}

// requireExistingRepo returns a friendly error (notYet) when a local repo has not
// been initialised yet. Remote repos are assumed to exist (no cheap local check).
func (s *Service) requireExistingRepo(repo, notYet string) error {
	if restic.IsRemoteRepo(repo) {
		return nil
	}
	if _, statErr := os.Stat(filepath.Join(repo, "config")); errors.Is(statErr, fs.ErrNotExist) { //nolint:gosec // G703: repo is an operator-configured location (settings path or its off-site sibling), validated under the mount root on save; source only selects which configured location, never a raw path
		return errors.New(notYet)
	}
	return nil
}

// reposThatExist narrows a domain's repositories (domainReposForOp) to the
// ones that are there, and fails with the caller's "not yet" sentence only
// when none of them is.
//
// With named repositories (#204), "the repository is missing" and "there
// is nothing to work on" are different sentences: a domain whose items
// all sit on named repositories may have no domain repository on disk at
// all, and refusing the whole operation over that would leave those
// repositories unverified, unpruned and locked.
// A repository that is missing while others are present is reported as a
// skip, for the same reason a switched-off one is: "the share was not
// mounted when the nightly verify ran" and "this repository was never
// created" are different facts, and only the second one is harmless.
func (s *Service) reposThatExist(repos []domainRepoRef, notYet string) ([]domainRepoRef, []repoSkip, error) {
	out := make([]domainRepoRef, 0, len(repos))
	var skipped []repoSkip
	var first error
	for _, r := range repos {
		if err := s.requireExistingRepo(r.Loc, notYet); err != nil {
			if first == nil {
				first = err
			}
			// Never created is silent; was there and is gone is reported, and so is
			// "don't know".
			//
			// An empty folder holds no backups, so a repository that was never created
			// is not a skip: pointing one container at a named repository before the
			// domain was ever backed up leaves the domain's own repository permanently
			// "not present", and verify, prune, unlock and the drill would report an
			// incomplete pass every night over repositories that are all fine.
			//
			// An unknown answer goes to the reported side, matching the other two
			// unknown rules in this file (an unreadable in-use count counts as in use;
			// an unreadable domain list counts as shared). Silence is the answer that
			// loses information, so it is not the one an error gets.
			switch s.repoEstablishmentOf(r.Loc) {
			case repoWasEstablished:
				skipped = append(skipped, repoSkip{Name: s.refName(r), Reason: "it was there before and is not reachable now", Unreachable: true})
			case repoEstablishmentUnknown:
				skipped = append(skipped, repoSkip{Name: s.refName(r), Reason: "it is not reachable now, and whether it ever held backups could not be read", Unreachable: true})
			case repoNeverEstablished:
			}
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		if first == nil {
			first = errors.New(notYet)
		}
		// The skip list travels with the error: "nothing to verify yet" and "the
		// one repository this domain has went away" are different messages, and
		// only the second one asks somebody to act; without it an all-named domain
		// would be told it has no backups at all.
		//
		// nothingCoveredError, not skippedError: nothing was covered here, and
		// `notYet` is a whole sentence rather than a noun phrase, so the other
		// template would read "no backups to verify yet covered only part of this
		// domain: …".
		if sErr := nothingCoveredError(skipped); sErr != nil {
			return nil, skipped, sErr
		}
		return nil, skipped, first
	}
	return out, skipped, nil
}

// shortRepoName names a repository in a message without printing a full
// host path: the last segment, or the remote's scheme, is what a reader
// recognises, and the error scrubber would redact the whole path anyway.
//
// It is slash-free because scrubError's absolute-path regex redacts any
// slash-led token, so "folder backups/cold" would arrive as "folder
// backups[path]"; that is why this returns one segment and not two.
func shortRepoName(repo string) string {
	if restic.IsRemoteRepo(repo) {
		if i := strings.IndexByte(repo, ':'); i > 0 {
			return repo[:i] + " remote"
		}
		return "a remote repository"
	}
	parts := strings.Split(strings.Trim(filepath.ToSlash(repo), "/"), "/")
	if n := len(parts); n > 0 && parts[n-1] != "" {
		return "folder " + parts[n-1]
	}
	return "a repository"
}

// isLockErr reports whether a restic error is a repository-lock conflict. It
// matches restic's specific lock-conflict phrasing ("unable to create lock" /
// "already locked") rather than the bare word "locked", so an unrelated error
// that merely mentions a lock doesn't trigger a needless unlock + retry.
func isLockErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unable to create lock") || strings.Contains(msg, "already locked")
}

// isRepoUninitialized reports whether a restic error means the backend was
// reached and holds no repository yet, which a remote off-site repo is until its
// first replication. restic says so as "repository does not exist". It prefixes
// "unable to open config file" onto every failure to reach the backend as well,
// a name that does not resolve or a 401 included, so that phrase alone counts
// only when the backend named a missing object, such as a bucket restic init
// will create, and no transport failure is named with it.
func isRepoUninitialized(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "repository does not exist") {
		return true
	}
	return strings.Contains(msg, "unable to open config file") &&
		containsAny(msg, repoAbsenceMarkers) && !containsAny(msg, transportFailureMarkers)
}

// unlockStale clears stale locks, best-effort (plain restic unlock: only
// locks from dead processes or old enough, never an active concurrent
// lock). Logged, never fatal.
func (s *Service) unlockStale(ctx context.Context, repo string, mode restic.Mode) {
	if err := s.engine.Unlock(ctx, repo, false, mode); err != nil {
		log.Printf("api: stale-unlock failed (continuing): %v", err)
	}
}

// listSnapshots lists snapshots, self-healing a stale-lock conflict: on a
// lock error it clears stale locks and retries once, so an interrupted run
// that left a lock behind does not make the backups list fail to load.
//
// The self-heal is skipped for a read-only caller. `restic unlock` writes:
// it deletes lock files in the repository. Mode.NoLock is how a caller
// declares "I never write to this repository", and the two surfaces that
// set it mean somebody else's repository: the foreign restore session and
// the receiver dashboard, both of which say so on screen. Retrying through
// an unlock would break that promise against a box whose owner never
// agreed to it, and only occasionally, since lock errors are rare and the
// repair looks like the read succeeding.
//
// A lock error on a foreign repository is also not BombVault's to repair.
// It usually means the far instance is running a backup right now, and
// the right answer is to report that rather than clear the marker it set.
func (s *Service) listSnapshots(ctx context.Context, repo string, mode restic.Mode) ([]restic.Snapshot, error) {
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if isLockErr(err) && !mode.NoLock {
		s.unlockStale(ctx, repo, mode)
		snaps, err = s.engine.Snapshots(ctx, repo, mode)
	}
	// A remote off-site repo that has not been replicated or initialised yet
	// has no snapshots: restic reports "repository does not exist", which must
	// read as "no backups yet", not a fatal error (#117). Local repos are
	// short-circuited upstream by localRepoMissing, so this only affects
	// remotes; genuine auth or connectivity errors do not match
	// isRepoUninitialized and still propagate.
	if err != nil && restic.IsRemoteRepo(repo) && isRepoUninitialized(err) {
		return nil, nil
	}
	return snaps, err
}

// lsSelfHeal lists a snapshot's files (restic ls), self-healing a
// stale-lock conflict like listSnapshots: on a lock error it clears stale
// locks and retries once. `ls` only takes a shared lock, but a stale
// exclusive lock left by an interrupted write elsewhere in the repo (an
// off-site replication or check that didn't finish cleanly, a killed
// backup) blocks it all the same. restic never notices staleness on its
// own, and nothing else touches this repo until the next scheduled backup
// runs its own unlockStale (see #29), so "Select files" would fail with a
// bare "Failed to load files" (#129) while the backups list right above it
// kept working.
func (s *Service) lsSelfHeal(ctx context.Context, repo, snapshotID string, mode restic.Mode) ([]restic.FileEntry, error) {
	entries, err := s.engine.Ls(ctx, repo, snapshotID, mode)
	if isLockErr(err) {
		s.unlockStale(ctx, repo, mode)
		entries, err = s.engine.Ls(ctx, repo, snapshotID, mode)
	}
	return entries, err
}

// pathsPresentInSnapshot answers, for each candidate, whether that exact path is
// a node in the snapshot's tree. One `restic ls` for the whole set.
//
// It exists for mapRestorePaths' pass 2, which produces a selector out of
// a stored path whose ancestor was recorded. An ancestor proves the path
// lies under a backed-up root and nothing more: a --exclude at backup time
// (derived from the selection's own exclusion branches) leaves a hole in
// that root, and a folder created after the snapshot was taken was never
// in it at all. Handing restic such a selector fails the restore, and on
// the container route that failure lands after the container has been
// stopped and removed.
//
// Called only when pass 2 actually narrowed something, which is the uncommon
// case: a selection that still matches the chosen snapshot resolves entirely in
// pass 1 and never reaches here. That keeps a full listing off the normal path.
func (s *Service) pathsPresentInSnapshot(ctx context.Context, repo, snapshotID string, mode restic.Mode, candidates []string) (map[string]bool, error) {
	present := make(map[string]bool, len(candidates))
	if len(candidates) == 0 {
		return present, nil
	}
	entries, err := s.lsSelfHeal(ctx, repo, snapshotID, mode)
	if err != nil {
		return nil, err
	}
	want := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		want[path.Clean(c)] = true
	}
	for _, e := range entries {
		if p := path.Clean(e.Path); want[p] {
			present[p] = true
		}
	}
	// Answer under the caller's own spelling too, so a candidate that was not
	// path.Clean to begin with still looks itself up.
	for _, c := range candidates {
		if present[path.Clean(c)] {
			present[c] = true
		}
	}
	return present, nil
}

// UnlockDomain removes locks from a domain's repo (restic unlock
// --remove-all). BombVault is the sole writer and serialises its
// operations, so a leftover lock is always safe to clear; this is the
// manual counterpart to the automatic stale-lock cleanup before each
// backup.
//
// It covers every repository this domain's items write to (#204). The
// button exists for the case where something is stuck, and a lock on an
// item's named repository is as stuck as one on the domain's own;
// unlocking only the latter would report success and change nothing. Every
// repository is attempted even if one fails, so a single unreachable
// location cannot leave the others locked; the first failure is what the
// caller hears about.
//
// The skip list is returned, not only folded into the error. A shared
// repository can only get the stale-lock clear, a permanent and correct
// limitation (a Note, in repoSkip's terms, so it must not stamp the run
// red), but the operator still has to be told, because the one repository
// they pressed the button for may be exactly the one that kept its lock.
//
// Rendered lines rather than the skips themselves: the caller shows them,
// the same lines the three discovery endpoints send, and a repoSkip is
// this package's own bookkeeping.
func (s *Service) UnlockDomain(ctx context.Context, domain, source string) ([]string, error) {
	settings, repos, skipped, err := s.domainReposForOp(domain, source)
	if err != nil {
		return nil, err
	}
	repos, missing, err := s.reposThatExist(repos, "no repository to unlock yet")
	if err != nil {
		return nil, err
	}
	skipped = append(skipped, missing...)
	unlock, ok := s.tryLockDomainFor(domain, "unlock")
	if !ok {
		return nil, errDomainBusy
	}
	defer unlock()
	// Per repository, like the other three (see CheckDomain).
	ctx, cancel := context.WithTimeout(ctx, time.Duration(len(repos))*2*time.Minute)
	defer cancel()
	// removeAll (a forced removal, live locks included) is only safe where
	// this process is provably the only writer, and the in-process
	// serialisation is keyed by domain. A named repository can be shared
	// between domains (nothing scopes one to a single domain, and the same
	// picker offers it to all three), so forcing on a shared repository would
	// yank the lock out from under another domain's running backup. There a
	// stale clear is the right tool: it removes a lock restic itself deems
	// dead and leaves a live one alone, as every other site in this file does.
	var firstErr error
	for _, r := range repos {
		rMode := s.repoModeFor(settings, domain, source, r.Loc)
		if s.repoSharedWithAnotherDomain(settings, domain, r) {
			log.Printf("api: unlock %s: %s is shared with another domain; clearing only stale locks there", domain, s.refName(r)) //nolint:gosec // G706: domain is a fixed literal, the name is the row's own
			// Reported, not silently downgraded. `restic unlock` without --remove-all
			// removes only what restic calls stale, and a lock left by a previous
			// container incarnation is not stale until it is old enough, so the
			// button that exists for exactly that case can come back green having
			// changed nothing. restic cannot be asked afterwards whether the lock is
			// gone, so the skip list says which repository got the weaker
			// treatment and why.
			if uErr := s.engine.Unlock(ctx, r.Loc, false, rMode); uErr != nil && firstErr == nil {
				firstErr = fmt.Errorf("clearing stale locks on %s: %w", s.refName(r), uErr)
			}
			skipped = append(skipped, repoSkip{Name: s.refName(r), Reason: "another domain writes to it too, so only stale locks were cleared there", Note: true})
			continue
		}
		if uErr := s.engine.Unlock(ctx, r.Loc, true, rMode); uErr != nil && firstErr == nil {
			firstErr = fmt.Errorf("unlocking %s: %w", s.refName(r), uErr)
		}
	}
	if firstErr == nil {
		firstErr = skippedError("this unlock", skipped)
	}
	return skipNames(skipped), firstErr
}
