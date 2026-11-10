package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// errNotSharedOwner refuses the upkeep of a shared repository to a server
// that does not hold it. Only the owner forgets and prunes there.
var errNotSharedOwner = errors.New("only the server that holds a shared repository keeps it")

// sharedMembers is how the owner's upkeep reaches the members of a shared
// repository: through the group, and for its own host by itself.
type sharedMembers interface {
	// heldBy names the entries whose old backups m holds on to.
	heldBy(ctx context.Context, m store.SharedMember) ([]string, error)
	// quiet asks m to start no copy to the repository before until. busy is
	// the answer that a copy is still running there, and then m promised
	// nothing.
	quiet(ctx context.Context, m store.SharedMember, until time.Time) (busy bool, err error)
	// release tells m that the prune is over.
	release(ctx context.Context, m store.SharedMember)
	// tellSpace hands m what the owner measured.
	tellSpace(ctx context.Context, m store.SharedMember, space store.SharedSpace)
}

// sharedForgetEvery is how often the owner forgets outside a prune window.
// A forget by id is quick, and what it lets go only frees space at the next
// prune.
const sharedForgetEvery = 24 * time.Hour

// sharedBusyPoll is how often the owner asks a member that is still copying
// again, and sharedBusyWait how long it waits for one at most before it moves
// the prune to the next window.
var (
	sharedBusyPoll = time.Minute
	sharedBusyWait = 30 * time.Minute
)

// keepSharedRepo runs the owner's upkeep of sr that is due at now: a forget
// pass once a day and when the prune window opens, and in that window the
// announcement, the prune, the check and the measurement. pruneNow opens a
// window at now.
func (s *Service) keepSharedRepo(ctx context.Context, sr store.SharedRepo, members sharedMembers, now time.Time, pruneNow bool) error {
	if !s.ownsSharedRepo(sr) {
		return errNotSharedOwner
	}
	u, err := s.store.SharedUpkeepOf(sr.ID)
	if err != nil {
		return err
	}
	window, prune := sharedWindow(u, now)
	if pruneNow {
		window, prune = now, true
	}
	forget := prune || u.Forget == nil || now.Sub(time.Unix(u.Forget.At, 0)) >= sharedForgetEvery
	if forget {
		if err := s.store.NoteSharedForget(sr.ID, s.forgetShared(ctx, sr, u, members, now)); err != nil {
			return err
		}
	}
	if !prune {
		// A new repository is measured with its first forget rather than a
		// week later in its first window.
		if forget && u.Space == nil {
			_, err = s.measureShared(ctx, sr, members, now)
		}
		return err
	}
	report := s.pruneShared(ctx, sr, u, members, window.Add(sharedHours(u)))
	if err := s.store.NoteSharedPrune(sr.ID, window.Unix(), report); err != nil {
		return err
	}
	if report.Error != "" || len(report.Postponed) > 0 {
		s.tellSharedWindow(ctx, sr, members, now)
		return nil
	}
	_, err = s.measureShared(ctx, sr, members, time.Now())
	return err
}

// nextSharedWindow is when the next prune window of u opens after now, 0 when
// none does.
func nextSharedWindow(u store.SharedUpkeep, now time.Time) int64 {
	cad, err := schedule.ParseCadence(u.PruneCadence)
	if err != nil {
		return 0
	}
	next, ok := cad.NextDue(time.Time{}, now)
	if !ok {
		return 0
	}
	return next.Unix()
}

// tellSharedWindow hands the members the last numbers of sr with the next
// window after now, for a pass that measured nothing and for new rules.
func (s *Service) tellSharedWindow(ctx context.Context, sr store.SharedRepo, members sharedMembers, now time.Time) {
	u, err := s.store.SharedUpkeepOf(sr.ID)
	if err != nil {
		log.Printf("api: shared repository: could not read its upkeep, so the members do not hear the next window: %v", err)
		return
	}
	space := store.SharedSpace{Hosts: []store.SharedHostSpace{}}
	if u.Space != nil {
		space = *u.Space
	}
	space.NextWindow = nextSharedWindow(u, now)
	for _, m := range sr.Members {
		members.tellSpace(ctx, m, space)
	}
}

func sharedHours(u store.SharedUpkeep) time.Duration {
	return time.Duration(max(u.PruneHours, 1)) * time.Hour
}

// sharedWindow is the prune window of u open at now, if the owner has not
// pruned in it yet.
func sharedWindow(u store.SharedUpkeep, now time.Time) (time.Time, bool) {
	cad, err := schedule.ParseCadence(u.PruneCadence)
	if err != nil {
		return time.Time{}, false
	}
	start, ok := cad.LastFire(now)
	return start, ok && now.Before(start.Add(sharedHours(u))) && start.Unix() > u.PrunedWindow
}

// ownsSharedRepo reports whether this server holds sr. A group state that
// cannot be read answers no, which leaves the repository alone.
func (s *Service) ownsSharedRepo(sr store.SharedRepo) bool {
	g, err := s.store.GetGroupState()
	if err != nil {
		log.Printf("api: could not read this server's group id, so the shared repository is not kept this time: %v", err)
		return false
	}
	return sr.Sharing(g.InstanceID) == restic.SharingOwner
}

// sharedAccess is where and how the owner reaches sr: the folder, and its
// own mode with the part of an owner.
func (s *Service) sharedAccess(sr store.SharedRepo) (string, restic.Mode, store.Settings, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", restic.Mode{}, settings, err
	}
	dest, err := s.resolveRepo(sr.Repo)
	if err != nil {
		return "", restic.Mode{}, settings, err
	}
	mode := s.ModeFor(settings)
	mode.Sharing = restic.SharingOwner
	return dest, mode, settings, nil
}

// forgetShared ages the history of every member of sr by the owner's rules,
// entry by entry and host by host, and forgets what they let go in one call,
// so the exclusive lock is held once and briefly. The rules are asked without
// a lock while the members go on copying, which does no harm: a snapshot that
// lands in between can only make another old one expendable, and that one
// goes next time. A host no member writes under is never aged, and neither is
// a member that cannot say which of its entries it holds. A server that left
// can say nothing any more and ages by the rules alone, as its removal said.
func (s *Service) forgetShared(ctx context.Context, sr store.SharedRepo, u store.SharedUpkeep, members sharedMembers, now time.Time) store.SharedForgetReport {
	report := store.SharedForgetReport{At: now.Unix()}
	dest, mode, settings, err := s.sharedAccess(sr)
	if err != nil {
		report.Error = err.Error()
		return report
	}
	snaps, err := s.engine.SnapshotsOfEveryHost(ctx, dest, mode)
	if err != nil {
		report.Error = scrubError(err)
		return report
	}
	former, err := s.store.ListSharedFormer(sr.ID)
	if err != nil {
		report.Error = err.Error()
		return report
	}
	byHost := map[string][]restic.Snapshot{}
	for _, sn := range snaps {
		byHost[sn.Hostname] = append(byHost[sn.Hostname], sn)
	}
	for host := range byHost {
		if !slices.ContainsFunc(sr.Members, func(m store.SharedMember) bool { return m.Host == host }) &&
			!slices.ContainsFunc(former, func(f store.SharedFormer) bool { return f.Host == host }) {
			report.Unknown = append(report.Unknown, host)
		}
	}
	slices.Sort(report.Unknown)

	var ids []string
	var errs []error
	for _, m := range sr.Members {
		own := byHost[m.Host]
		if len(own) == 0 {
			continue
		}
		held, err := members.heldBy(ctx, m)
		if err != nil {
			log.Printf("api: shared repository: %s could not say which of its entries it holds, so its history stays as it is: %v", m.Name, err) //nolint:gosec // G706: the name is the member's own
			report.Skipped = append(report.Skipped, m.Name)
			continue
		}
		gone, err := s.sharedForgets(ctx, dest, mode, settings, u, m, own, held)
		ids = append(ids, gone...)
		errs = append(errs, err)
	}
	for _, f := range former {
		own := byHost[f.Host]
		if len(own) == 0 {
			continue
		}
		if _, err := s.formerMember(sr, f.Host); err != nil {
			continue
		}
		gone, err := s.sharedForgets(ctx, dest, mode, settings, u, store.SharedMember{Host: f.Host, Name: f.Name}, own, nil)
		ids = append(ids, gone...)
		errs = append(errs, err)
	}
	if len(ids) > 0 {
		err := s.retryAfterUnlock(ctx, dest, mode, func() error { return s.engine.Forget(ctx, dest, ids, false, mode) })
		if err == nil {
			report.Forgotten = len(ids)
		}
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		report.Error = scrubError(err)
	}
	return report
}

// sharedForgets is what the rules let go of the snapshots own of member m,
// leaving out the entries it holds. Only ids restic listed under m's host
// count, so nothing of another server can slip in.
func (s *Service) sharedForgets(ctx context.Context, dest string, mode restic.Mode, settings store.Settings, u store.SharedUpkeep, m store.SharedMember, own []restic.Snapshot, held []string) ([]string, error) {
	holds := heldIdentityTags{}
	for _, t := range held {
		holds[t] = struct{}{}
	}
	listed := map[string]bool{}
	for _, sn := range own {
		listed[sn.ID] = true
	}
	hostMode := mode
	hostMode.Host = m.Host
	var ids []string
	var errs []error
	for _, tag := range identityTags(own) {
		p := sharedRule(settings, u, tag)
		if holds.holds(tag) || !p.Any() {
			continue
		}
		groups, err := s.engine.ForgetPreview(ctx, dest, p, hostMode, tag)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s of %s: %w", tag, m.Name, err))
			continue
		}
		for _, g := range groups {
			for _, sn := range g.Remove {
				if listed[sn.ID] && !slices.Contains(ids, sn.ID) {
					ids = append(ids, sn.ID)
				}
			}
		}
	}
	return ids, errors.Join(errs...)
}

// sharedRule is the keep-policy an entry with the identity tag ages by in a
// shared repository: the card of the repository once the owner saved it, the
// owner's off-site rules for the entry's section until then. Like on every
// copy target, a copy of a direct repository's snapshot ages with the rest.
func sharedRule(settings store.Settings, u store.SharedUpkeep, tag string) restic.RetentionPolicy {
	k := u.Keep
	if !u.KeepSet {
		k = offsiteKeep(settings, identityDomain(tag))
	}
	p := keepPolicy(k)
	p.Direct = true
	return p
}

// pruneShared tells the members that the owner prunes until until, waits for
// the ones still copying, then prunes and checks. A member that does not
// answer holds nothing up: its copy waits at restic's lock and gives up
// without touching it. One still copying when the wait is over moves the
// prune to the next window, since its copy would hold the prune's lock off
// for as long as it runs.
func (s *Service) pruneShared(ctx context.Context, sr store.SharedRepo, u store.SharedUpkeep, members sharedMembers, until time.Time) store.SharedPruneReport {
	report := store.SharedPruneReport{At: time.Now().Unix()}
	dest, mode, _, err := s.sharedAccess(sr)
	if err != nil {
		report.Error = err.Error()
		return report
	}
	quiet, busy := quietMembers(ctx, sr.Members, members, until)
	defer func() {
		for _, m := range quiet {
			members.release(context.WithoutCancel(ctx), m)
		}
	}()
	for _, m := range busy {
		report.Postponed = append(report.Postponed, m.Name)
	}
	if len(busy) > 0 {
		log.Printf("api: shared repository: %s still copying, so the prune waits for the next window", strings.Join(report.Postponed, ", ")) //nolint:gosec // G706: the names are the members' own
		return report
	}
	mode.MaxUnused = u.MaxUnused
	started := time.Now()
	err = s.retryAfterUnlock(ctx, dest, mode, func() error { return s.engine.Prune(ctx, dest, mode) })
	report.Seconds = int64(time.Since(started).Seconds())
	if err != nil {
		report.Error = scrubError(err)
		return report
	}
	report.Checked = true
	if err := s.retryAfterUnlock(ctx, dest, mode, func() error { return s.engine.CheckData(ctx, dest, max(u.CheckSubset, 1), mode) }); err != nil {
		report.CheckError = scrubError(err)
	}
	return report
}

// quietMembers asks every member to hold back its copies until until, and
// asks the busy ones again until they are done or sharedBusyWait is over. It
// returns the members that agreed and the ones still busy.
func quietMembers(ctx context.Context, all []store.SharedMember, members sharedMembers, until time.Time) (quiet, busy []store.SharedMember) {
	ask := func(list []store.SharedMember) []store.SharedMember {
		var still []store.SharedMember
		for _, m := range list {
			b, err := members.quiet(ctx, m, until)
			switch {
			case err != nil:
				log.Printf("api: shared repository: %s did not hear about the prune: %v", m.Name, err) //nolint:gosec // G706: the name is the member's own
			case b:
				still = append(still, m)
			default:
				quiet = append(quiet, m)
			}
		}
		return still
	}
	busy = ask(all)
	if len(busy) == 0 {
		return quiet, nil
	}
	deadline := time.NewTimer(sharedBusyWait)
	defer deadline.Stop()
	poll := time.NewTicker(sharedBusyPoll)
	defer poll.Stop()
	for len(busy) > 0 {
		select {
		case <-ctx.Done():
			return quiet, busy
		case <-deadline.C:
			return quiet, busy
		case <-poll.C:
			busy = ask(busy)
		}
	}
	return quiet, nil
}

// measureShared measures what sr holds as a whole and what the snapshots of
// each member use of it, keeps the numbers and hands them to the members. It
// takes no lock, so it runs beside their copies.
func (s *Service) measureShared(ctx context.Context, sr store.SharedRepo, members sharedMembers, now time.Time) (store.SharedSpace, error) {
	dest, mode, _, err := s.sharedAccess(sr)
	if err != nil {
		return store.SharedSpace{}, err
	}
	whole, err := s.engine.StatsOfEveryHost(ctx, dest, "raw-data", mode)
	if err != nil {
		return store.SharedSpace{}, fmt.Errorf("measure the shared repository: %w", err)
	}
	u, err := s.store.SharedUpkeepOf(sr.ID)
	if err != nil {
		return store.SharedSpace{}, err
	}
	space := store.SharedSpace{At: now.Unix(), Total: whole.TotalSize, Hosts: []store.SharedHostSpace{}, NextWindow: nextSharedWindow(u, now)}
	for _, m := range sr.Members {
		hostMode := mode
		hostMode.Host = m.Host
		st, err := s.engine.Stats(ctx, dest, "raw-data", hostMode)
		if err != nil {
			return store.SharedSpace{}, fmt.Errorf("measure what %s uses: %w", m.Name, err)
		}
		space.Hosts = append(space.Hosts, store.SharedHostSpace{Host: m.Host, Name: m.Name, Bytes: st.TotalSize})
	}
	if err := s.store.NoteSharedSpace(sr.ID, space); err != nil {
		return store.SharedSpace{}, err
	}
	for _, m := range sr.Members {
		members.tellSpace(ctx, m, space)
	}
	return space, nil
}

// errSharedUpkeepBusy answers a request to keep a shared repository while an
// upkeep runs already.
var errSharedUpkeepBusy = errors.New("the upkeep of the shared repositories is running already")

// RunSharedUpkeep runs what is due in the upkeep of every shared repository
// this server holds, and takes back the one-time keys nobody used in time.
// The scheduler calls it every minute; a pass that is still running, such as
// a long prune, lets the tick go by.
func (s *Service) RunSharedUpkeep(ctx context.Context) error {
	s.tellSharedHoldersSoon()
	s.clearProofKeysSoon(ctx)
	return s.runSharedUpkeep(ctx, s.groupMembers(), time.Now())
}

func (s *Service) runSharedUpkeep(ctx context.Context, members sharedMembers, now time.Time) error {
	// Taking a login back needs no turn, so a long prune does not stretch
	// the day a one-time key is good for.
	expired := s.expireSharedJoins(ctx, now)
	if !s.sharedUpkeep.take() {
		return expired
	}
	defer s.sharedUpkeep.done()
	owned, err := s.ownedSharedRepos()
	if err != nil {
		return errors.Join(expired, err)
	}
	errs := []error{expired}
	for _, sr := range owned {
		errs = append(errs, s.keepSharedRepo(ctx, sr, members, now, false))
	}
	errs = append(errs, s.dropExpiredKeys(ctx))
	return errors.Join(errs...)
}

// runSharedUpkeepNow forgets, prunes, checks and measures the shared
// repository id at once, as a person asked.
func (s *Service) runSharedUpkeepNow(ctx context.Context, id string, members sharedMembers) error {
	if !s.sharedUpkeep.take() {
		return errSharedUpkeepBusy
	}
	defer s.sharedUpkeep.done()
	sr, err := s.ownedSharedRepo(id)
	if err != nil {
		return err
	}
	return s.keepSharedRepo(ctx, sr, members, time.Now(), true)
}

// NextSharedUpkeep is when the next pass over a shared repository this server
// holds is due: the daily forget or the next prune window, whichever comes
// first.
func (s *Service) NextSharedUpkeep(now time.Time) (time.Time, bool) {
	owned, err := s.ownedSharedRepos()
	if err != nil {
		return time.Time{}, false
	}
	var soonest time.Time
	for _, sr := range owned {
		u, err := s.store.SharedUpkeepOf(sr.ID)
		if err != nil {
			continue
		}
		next := now
		if u.Forget != nil {
			next = time.Unix(u.Forget.At, 0).Add(sharedForgetEvery)
		}
		if cad, err := schedule.ParseCadence(u.PruneCadence); err == nil {
			if w, ok := cad.NextDue(time.Time{}, now); ok && w.Before(next) {
				next = w
			}
		}
		if soonest.IsZero() || next.Before(soonest) {
			soonest = next
		}
	}
	return soonest, !soonest.IsZero()
}

// ownedSharedRepos lists the shared repositories this server holds.
func (s *Service) ownedSharedRepos() ([]store.SharedRepo, error) {
	all, err := s.store.ListSharedRepos()
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(sr store.SharedRepo) bool { return !s.ownsSharedRepo(sr) }), nil
}
