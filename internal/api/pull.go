package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path"
	"path/filepath"
	"strings"
	"time"

	"database/sql"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/secret"
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
// from its APP_KEY, and restic.Mode.From carries the source's. Getting that
// wrong reads as "wrong APP_KEY" for a correctly typed key.

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

	keyBytes, err := secret.Decrypt(s.cfg.AppKey, ps.AppKeyEnc)
	if err != nil {
		return "", restic.Mode{}, errors.New("could not decrypt the stored APP_KEY for this pull source")
	}
	sourceKey := string(keyBytes)
	// restickey.Derive panics on non-hex input, so a corrupted row has to be
	// caught here.
	if !foreignKeyRe.MatchString(sourceKey) {
		return "", restic.Mode{}, errors.New("the stored APP_KEY is not 64 lowercase hex characters")
	}

	// The source's own backend credentials, never this box's. Without a
	// credential set Env stays empty; unlike an off-site target, which is ours,
	// a source does not borrow the shared ones.
	var env []string
	if ref := strings.TrimSpace(ps.CredsRef); ref != "" {
		if c, cErr := s.decodeCloudFor(settings, ref); cErr == nil {
			env = cloudEnv(c)
		} else {
			log.Printf("api: pull source %s: credential set decode failed (continuing without it): %v", ps.ID, cErr) //nolint:gosec // G706: ps.ID is an opaque store-generated id
		}
	}

	base := restic.Mode{NoLock: true, NoAmbientCreds: true, Env: env}
	encMode := base
	encMode.Encrypted = true
	encMode.Password = restickey.Derive(sourceKey)
	switch {
	case s.engine.RepoOpens(ctx, repo, encMode):
		return repo, encMode, nil
	case s.engine.RepoOpens(ctx, repo, base):
		return repo, base, nil
	default:
		return "", restic.Mode{}, errors.New("could not open the pull source: wrong APP_KEY, or the location is not a BombVault or restic repository")
	}
}

// PullFromSource runs one pull and records its verdict. It returns how many
// snapshots the copy was asked to carry.
func (s *Service) PullFromSource(ctx context.Context, ps store.PullSource) (int, error) {
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
	settings, dest, err := s.domainRepoSource(domain, "local")
	if err != nil {
		return 0, fmt.Errorf("resolve the local %s repository: %w", domain, err)
	}
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
	// BombVault is the only writer of dest, so a lock left behind is stale.
	// listSnapshots leaves the source's locks alone because its mode sets NoLock.
	s.unlockStale(ctx, dest, destMode)

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
		return 0, nil
	}

	// The copy carries the destination's mode with the source's credentials
	// attached. No snapshot ids are passed: restic compares full metadata and
	// decides what moves, so pending is only for display.
	copyMode := destMode
	copyMode.From = &restic.From{Encrypted: srcMode.Encrypted, Password: srcMode.Password}
	copyMode.Env = append(append([]string{}, destMode.Env...), srcMode.Env...)
	lim := restic.Limits{DownloadKBps: ps.LimitDownload, UploadKBps: ps.LimitUpload}
	if err := s.engine.Copy(ctx, dest, src, nil, lim, copyMode); err != nil {
		return 0, fmt.Errorf("copy from the source: %w", err)
	}
	return pending, nil
}

// RunPulls is the scheduled sweep: every enabled source whose cadence says it is
// due gets pulled once. A source that fails does not stop the others.
func (s *Service) RunPulls(ctx context.Context) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	if !settings.PullEnabled {
		return nil
	}
	sources, err := s.store.ListPullSources()
	if err != nil {
		return fmt.Errorf("list pull sources: %w", err)
	}
	now := time.Now()
	for _, ps := range sources {
		if !ps.Enabled {
			continue
		}
		period := cadencePeriodSeconds(ps.Cadence)
		if period <= 0 {
			continue // 'off', or a cadence with no period: only the button runs it
		}
		// schedule.PeriodDue rather than "now minus last exceeds the period",
		// which drifts: each late start pushes the next run later, until a daily
		// job runs every other day.
		if !schedule.PeriodDue(time.Unix(ps.LastPullAt, 0), now, period) {
			continue
		}
		if _, pErr := s.PullFromSource(ctx, ps); pErr != nil {
			log.Printf("api: pull source %s failed: %v", ps.ID, pErr) //nolint:gosec // G706: ps.ID is an opaque store-generated id
		}
	}
	return nil
}

// pullProbe opens a source read-only and discards the result. The Test button
// runs it, and create and update run it before saving, so a mistyped location
// or key is refused on the form.
func (s *Service) pullProbe(ctx context.Context, ps store.PullSource) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	_, _, err = s.pullOpen(ctx, ps, settings)
	return err
}
