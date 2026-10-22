package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"database/sql"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Pulling fetches snapshots from another instance's repository into this one.
// It is off-site replication with the ends swapped, the same restic copy, so a
// box can fetch a neighbour's backups without the neighbour configuring
// anything or even being awake.
//
// The source belongs to somebody else and is only read: RepoOpens and Copy's
// source argument are the only engine calls that reach it, never EnsureRepo,
// Unlock, Forget or Prune. A lock on it usually means its owner is backing up.
//
// The two ends have different passwords, because each instance derives its own
// from its APP_KEY, and restic.Mode.From carries the source's. The source's
// password arrives over the pairing group; its APP_KEY never does.

// pullOpen resolves a pull source's location and opens it read-only, returning
// the location and the mode to read it with. It follows receiverOpen and adds
// what the foreign-restore path has on top: no ambient credentials and no
// rclone location.
func (s *Service) pullOpen(ctx context.Context, ps store.PullSource, settings store.Settings) (string, restic.Mode, error) {
	loc := strings.TrimSpace(ps.Repo)
	if loc == "" {
		return "", restic.Mode{}, errors.New("missing repository location")
	}
	if isRcloneLocation(loc) {
		// rclone reads its remotes from this instance's config file, so an
		// operator-supplied rclone location would authenticate with our secrets.
		return "", restic.Mode{}, errors.New("an rclone: location cannot be a pull source, because rclone would use THIS instance's remotes to reach it. Use the repository's own address (rest:, s3:, sftp: or b2:) with its own credentials")
	}
	// As in receiverOpen: an absolute path inside the host mount is used as is,
	// anything else goes through resolveRepo, whose error suggests the relative
	// path to type instead.
	var repo string
	mountRoot := path.Clean(filepath.ToSlash(s.cfg.HostMountRoot))
	inMount := mountRoot != "." && mountRoot != "/" &&
		strings.HasPrefix(path.Clean(filepath.ToSlash(loc)), mountRoot+"/")
	if !restic.IsRemoteRepo(loc) && inMount {
		repo = loc
	} else {
		resolved, err := s.resolveRepo(loc)
		if err != nil {
			return "", restic.Mode{}, err
		}
		repo = resolved
	}

	password, err := s.openResticPassword(ps.ResticPasswordEnc)
	if err != nil {
		return "", restic.Mode{}, err
	}

	// The source's own backend credentials, never this box's. Without a
	// credential set Env stays empty; unlike an off-site target, which is ours,
	// a source does not borrow the shared ones, not even when its own is gone.
	var env []string
	if ref := strings.TrimSpace(ps.CredsRef); ref != "" {
		c, err := s.savedSignIn(settings, ref)
		if errors.Is(err, errUnknownSignIn) {
			return "", restic.Mode{}, errors.New("the saved sign-in this source used is gone, pick another one for it")
		}
		if err != nil {
			return "", restic.Mode{}, err
		}
		env = cloudEnv(c)
	}

	base := restic.Mode{NoLock: true, NoAmbientCreds: true, Env: env}
	encMode := base
	encMode.Encrypted = true
	encMode.Password = password
	mode, err := s.openOthersRepo(ctx, repo, encMode, base, "pull source")
	if err != nil {
		return "", restic.Mode{}, err
	}
	return repo, mode, nil
}

// pulledRepo is the repository a source's snapshots land in: one of its own
// inside the domain's location. restic copy keeps each snapshot's host and
// tags, so in the domain's own repository a neighbour's container:plex or flash
// would be the same item as this box's: retention would age the two as one and
// a restore could pick the neighbour's.
func pulledRepo(domainRepo, sourceID string) string {
	if restic.IsRemoteRepo(domainRepo) {
		return strings.TrimRight(domainRepo, "/") + "/pulled/" + sourceID
	}
	return filepath.Join(domainRepo, "pulled", sourceID)
}

// errPullBusy refuses a second pull of a source while one runs. Both would
// copy the same snapshots, and each takes the other's locks on dest for stale.
var errPullBusy = errors.New("this source is being pulled already")

func (s *Service) claimPull(id string) bool {
	s.pullMu.Lock()
	defer s.pullMu.Unlock()
	if s.pulling[id] {
		return false
	}
	if s.pulling == nil {
		s.pulling = map[string]bool{}
	}
	s.pulling[id] = true
	return true
}

func (s *Service) releasePull(id string) {
	s.pullMu.Lock()
	delete(s.pulling, id)
	s.pullMu.Unlock()
}

// PullFromSource runs one pull and records its verdict. It returns how many
// snapshots the copy was asked to carry. A pull refused because another one
// runs records nothing.
func (s *Service) PullFromSource(ctx context.Context, ps store.PullSource) (int, error) {
	if !s.claimPull(ps.ID) {
		return 0, errPullBusy
	}
	defer s.releasePull(ps.ID)
	n, err := s.pullFromSource(ctx, ps)
	ok := sql.NullBool{Bool: err == nil, Valid: true}
	msg := ""
	if err != nil {
		msg = scrubError(err)
	}
	if rErr := s.store.UpdatePullSourceResult(ps.ID, time.Now().Unix(), ok, msg, n); rErr != nil {
		log.Printf("api: pull source %s: could not record the result: %v", ps.ID, rErr) //nolint:gosec // G706: ps.ID is an opaque store-generated id
	}
	return n, err
}

func (s *Service) pullFromSource(ctx context.Context, ps store.PullSource) (int, error) {
	domain := strings.TrimSpace(ps.Domain)
	if domain == "" {
		return 0, errors.New("this pull source names no domain, so there is nowhere for its snapshots to land")
	}
	// Earlier pulls are moved first, so a source that is out of reach or no
	// longer lets this box fetch still has them taken out.
	s.settleEarlierPulls(ctx, ps, false)
	if err := s.fetcherRoleOpen(ps.MemberID, domain); err != nil {
		return 0, err
	}
	settings, domainRepo, err := s.domainRepoSource(domain, "local")
	if err != nil {
		return 0, fmt.Errorf("resolve the local %s repository: %w", domain, err)
	}
	dest := pulledRepo(domainRepo, ps.ID)
	src, srcMode, err := s.pullOpen(ctx, ps, settings)
	if err != nil {
		return 0, err
	}

	// From here on only dest is written. The source appears twice more: in the
	// listing, which is read-only by mode, and as Copy's source argument.
	destMode := s.primaryModeFor(settings, domain, dest)
	if err := s.EnsureRepo(ctx, dest, destMode); err != nil {
		return 0, fmt.Errorf("ensure the local %s repository: %w", domain, err)
	}
	// This pull is the only writer of dest, so a lock left behind is stale.
	// listSnapshots leaves the source's locks alone because its mode sets NoLock.
	// A remote dest is unlocked only when a lock stops the copy, since every
	// call to it is billed.
	if !restic.IsRemoteRepo(dest) {
		s.unlockStale(ctx, dest, destMode)
	}

	srcSnaps, err := s.listSnapshots(ctx, src, srcMode)
	if err != nil {
		return 0, fmt.Errorf("list the source's snapshots: %w", err)
	}
	if len(srcSnaps) == 0 {
		return 0, nil
	}
	pending := 0
	if dstSnaps, dErr := s.listSnapshots(ctx, dest, destMode); dErr != nil {
		log.Printf("api: pull %s: could not estimate how much is pending (continuing): %v", domain, dErr) //nolint:gosec // G706: domain comes from a fixed set
	} else {
		pending = len(restic.PendingCopyIDs(srcSnaps, dstSnaps))
	}
	if pending == 0 {
		// Nothing new is the ordinary outcome between two backups, not a failure.
		s.movePullLeftovers(ctx, ps, settings, domain, domainRepo, dest, destMode, true)
		return 0, nil
	}

	// The copy carries the destination's mode with the source's credentials
	// attached. No snapshot ids are passed: restic compares full metadata and
	// decides what moves, so pending is only for display.
	copyDest, copySrc, env, err := copyAccess(dest, destMode.Env, src, srcMode.Env)
	if err != nil {
		return 0, err
	}
	copyMode := destMode
	copyMode.From = &restic.From{Encrypted: srcMode.Encrypted, Password: srcMode.Password, NoLock: true}
	copyMode.Env = env
	// Only a destination behind an rclone remote reads this box's rclone config.
	copyMode.NoAmbientCreds = repoScheme(dest) != "rclone"
	lim := restic.Limits{DownloadKBps: ps.LimitDownload, UploadKBps: ps.LimitUpload}
	if err := s.retryAfterUnlock(ctx, dest, destMode, func() error { return s.engine.Copy(ctx, copyDest, copySrc, nil, lim, copyMode) }); err != nil {
		return 0, fmt.Errorf("copy from the source: %w", err)
	}
	s.movePullLeftovers(ctx, ps, settings, domain, domainRepo, dest, destMode, true)
	return pending, nil
}

// settleEarlierPulls moves what pulls of ps left in the domain's own
// repository before the source had one of its own, without reaching the
// source. Before the source's next copy, a leftover that belongs to one of
// several sources of the domain cannot be told apart yet, so it waits for
// that copy.
func (s *Service) settleEarlierPulls(ctx context.Context, ps store.PullSource, final bool) {
	if moved, err := s.store.PullLeftoversMoved(ps.ID); err != nil || moved {
		return
	}
	domain := strings.TrimSpace(ps.Domain)
	settings, domainRepo, err := s.domainRepoSource(domain, "local")
	if err != nil {
		log.Printf("api: pull %s: earlier pulls stay in the %s repository for now: %v", ps.ID, domain, err) //nolint:gosec // G706: ps.ID is an opaque store-generated id, domain comes from a fixed set
		return
	}
	dest := pulledRepo(domainRepo, ps.ID)
	destMode := s.primaryModeFor(settings, domain, dest)
	if err := s.EnsureRepo(ctx, dest, destMode); err != nil {
		log.Printf("api: pull %s: earlier pulls stay in the %s repository for now: %v", ps.ID, domain, err) //nolint:gosec // G706: see above
		return
	}
	s.movePullLeftovers(ctx, ps, settings, domain, domainRepo, dest, destMode, final)
}

// movePullLeftovers takes the snapshots that pulls wrote into the domain's own
// repository, before a source had one of its own, out of it. One whose copy
// is in the source's repository here is forgotten. One the source has since
// let go of is copied there first when the source is the domain's only one,
// since nothing else can tell whose it is. Until then retention ages them with
// this box's own and the restore lists offer them as this box's. A failure is
// logged and tried again on the next pull. final is set once the source's
// copy has run, after which nothing more of it can land.
func (s *Service) movePullLeftovers(ctx context.Context, ps store.PullSource, settings store.Settings, domain, domainRepo, dest string, destMode restic.Mode, final bool) {
	moved, err := s.store.PullLeftoversMoved(ps.ID)
	if err != nil || moved {
		return
	}
	fail := func(what string, err error) {
		log.Printf("api: pull %s: could not move earlier pulls out of the %s repository, %s: %v", ps.ID, domain, what, err) //nolint:gosec // G706: ps.ID is an opaque store-generated id, domain comes from a fixed set
	}
	own, err := s.store.RunSnapshotIDs()
	if err != nil {
		fail("reading this box's runs", err)
		return
	}
	sources, err := s.store.ListPullSources()
	if err != nil {
		fail("reading the pull sources", err)
		return
	}
	alone := !slices.ContainsFunc(sources, func(o store.PullSource) bool {
		return o.ID != ps.ID && strings.TrimSpace(o.Domain) == domain
	})
	primaryMode := s.primaryModeFor(settings, domain, domainRepo)
	primary, err := s.listSnapshots(ctx, domainRepo, primaryMode)
	if err != nil {
		fail("listing it", err)
		return
	}
	landed, err := s.listSnapshots(ctx, dest, destMode)
	if err != nil {
		fail("listing the source's repository here", err)
		return
	}
	forget, unplaced := pullLeftovers(primary, landed, own)
	if alone && len(unplaced) > 0 {
		copyMode := destMode
		copyMode.From = &restic.From{Encrypted: primaryMode.Encrypted, Password: primaryMode.Password, NoLock: true}
		copyDest, copySrc, env, err := copyAccess(dest, destMode.Env, domainRepo, primaryMode.Env)
		if err != nil {
			fail("reaching both repositories", err)
			return
		}
		copyMode.Env = env
		copyMode.NoAmbientCreds = repoScheme(dest) != "rclone"
		if err := s.retryAfterUnlock(ctx, dest, destMode, func() error {
			return s.engine.Copy(ctx, copyDest, copySrc, unplaced, restic.Limits{}, copyMode)
		}); err != nil {
			fail("copying them to the source's repository", err)
			return
		}
		forget, unplaced = append(forget, unplaced...), nil
	}
	if len(forget) > 0 {
		if err := s.retryAfterUnlock(ctx, domainRepo, primaryMode, func() error {
			return s.engine.Forget(ctx, domainRepo, forget, false, primaryMode)
		}); err != nil {
			fail("forgetting them", err)
			return
		}
		log.Printf("api: pull %s: moved %d earlier pulled snapshots out of the %s repository", ps.ID, len(forget), domain) //nolint:gosec // G706: see above
	}
	if len(unplaced) > 0 && !final {
		return
	}
	if err := s.store.SetPullLeftoversMoved(ps.ID, true); err != nil {
		fail("recording it", err)
	}
}

// pullLeftovers sorts the snapshots of the domain's own repository that an
// earlier pull brought in: copies (restic gives every copy an original) this
// box never recorded making itself. forget are those the source's repository
// here holds by now, unplaced the rest. Mutual pulls can put a snapshot of
// this box into the source, and the run history keeps that one.
func pullLeftovers(primary, landed []restic.Snapshot, own map[string]bool) (forget, unplaced []string) {
	here := make(map[string]bool, len(landed))
	for _, sn := range landed {
		here[restic.Identity(sn)] = true
	}
	for _, sn := range primary {
		if sn.Original == "" || own[sn.ID] || own[sn.Original] {
			continue
		}
		if here[restic.Identity(sn)] {
			forget = append(forget, sn.ID)
		} else {
			unplaced = append(unplaced, sn.ID)
		}
	}
	return forget, unplaced
}

// copyAccess fits the sign-ins of both ends of a restic copy into its one
// process. Each end brings only what its own backend reads. restic hands the
// S3 and rest-server variables to both repositories, so when both ends are S3
// or both are rest-servers and they do not share a sign-in, each signs in
// through its own address, and neither login reaches the other server.
func copyAccess(dest string, destEnv []string, src string, srcEnv []string) (copyDest, copySrc string, env []string, err error) {
	destEnv, srcEnv = ownBackendEnv(dest, destEnv), ownBackendEnv(src, srcEnv)
	if joined, ok := bothSignIns(dest, destEnv, src, srcEnv); ok {
		return dest, src, joined, nil
	}
	c := credsInEnv(srcEnv)
	if repoScheme(src) == "s3" {
		// An rclone remote without keys reads the bucket anonymously.
		var rcloneEnv []string
		if copySrc, rcloneEnv, err = s3ThroughRclone(src, c); err != nil {
			return "", "", nil, err
		}
		return dest, copySrc, append(destEnv, rcloneEnv...), nil
	}
	d := credsInEnv(destEnv)
	if copyDest, err = restWithLogin(dest, d.RESTUser, d.RESTPassword); err != nil {
		return "", "", nil, err
	}
	if copySrc, err = restWithLogin(src, c.RESTUser, c.RESTPassword); err != nil {
		return "", "", nil, err
	}
	env = slices.DeleteFunc(destEnv, func(kv string) bool { return strings.HasPrefix(kv, backendEnvPrefix["rest"]) })
	return copyDest, copySrc, env, nil
}

// RunPulls is the scheduled sweep: every enabled source whose cadence says it is
// due gets pulled once. A source that fails does not stop the others. One
// that is not pulled still has its earlier pulls moved.
func (s *Service) RunPulls(ctx context.Context) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	sources, err := s.store.ListPullSources()
	if err != nil {
		return fmt.Errorf("list pull sources: %w", err)
	}
	now := time.Now()
	for _, ps := range sources {
		period := cadencePeriodSeconds(ps.Cadence)
		// schedule.PeriodDue rather than "now minus last exceeds the period",
		// which drifts: each late start pushes the next run later, until a daily
		// job runs every other day. A period of 0 is 'off', or a cadence with
		// no period: only the button runs it.
		if !settings.PullEnabled || !ps.Enabled || period <= 0 || !schedule.PeriodDue(time.Unix(ps.LastPullAt, 0), now, period) {
			if s.claimPull(ps.ID) {
				s.settleEarlierPulls(ctx, ps, false)
				s.releasePull(ps.ID)
			}
			continue
		}
		if _, pErr := s.PullFromSource(ctx, ps); pErr != nil && !errors.Is(pErr, errPullBusy) {
			log.Printf("api: pull source %s failed: %v", ps.ID, pErr) //nolint:gosec // G706: ps.ID is an opaque store-generated id
		}
	}
	return nil
}

// pullProbe opens a source read-only and discards the result. The Test button
// runs it, and create and update run it before saving, so a mistyped location
// is refused on the form.
func (s *Service) pullProbe(ctx context.Context, ps store.PullSource) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	_, _, err = s.pullOpen(ctx, ps, settings)
	return err
}
