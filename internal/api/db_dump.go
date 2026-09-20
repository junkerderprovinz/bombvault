package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/dbdump"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// dbDumpIdentityPrefix starts the identity tag of a dump snapshot, which gives
// a dump its own retention series next to the container's volume snapshots.
const dbDumpIdentityPrefix = "dbdump:"

func dbDumpIdentity(name string) string { return dbDumpIdentityPrefix + name }

// dbDumpStdinPath is the path the dump stream is stored under inside the
// snapshot. A restore reads the file back by exactly this name.
func dbDumpStdinPath(name string) string { return "/dbdump/" + name + ".sql" }

const (
	dbDumpDefaultMaxRuntime = 6 * time.Hour
	dbDumpMinMaxRuntime     = time.Hour
	dbDumpMaxMaxRuntime     = 48 * time.Hour
)

// dbDumpMaxRuntime is the time limit a dump gets inside its container,
// DB_DUMP_MAX_HOURS in hours. Read once per process so an invalid value warns
// at the first backup instead of every night.
var dbDumpMaxRuntime = sync.OnceValue(func() time.Duration {
	return dbDumpMaxRuntimeFrom(os.Getenv("DB_DUMP_MAX_HOURS"), backupHardCap())
})

// dbDumpMaxRuntimeFrom reads the configured limit and keeps it an hour below
// the whole backup's cap, so a dump is cut by its own limit and reported as
// such rather than being killed with the backup around it.
func dbDumpMaxRuntimeFrom(raw string, backupCap time.Duration) time.Duration {
	max := dbDumpDefaultMaxRuntime
	if raw = strings.TrimSpace(raw); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || time.Duration(n)*time.Hour < dbDumpMinMaxRuntime || time.Duration(n)*time.Hour > dbDumpMaxMaxRuntime {
			log.Printf("api: invalid DB_DUMP_MAX_HOURS=%q (want 1..48 hours), using %v", raw, max) //nolint:gosec // G706: %q-quoted
		} else {
			max = time.Duration(n) * time.Hour
		}
	}
	if backupCap > 0 && max >= backupCap {
		max = backupCap - time.Hour
		if max < dbDumpMinMaxRuntime {
			max = dbDumpMinMaxRuntime
		}
		log.Printf("api: database dump limit shortened to %v so it stays below BACKUP_MAX_HOURS", max)
	}
	return max
}

// dbDumpPlanFor decides whether this backup dumps the container's database,
// and with which engine. It returns nil for everything that is not a
// recognised, running, dump-enabled database.
func (s *Service) dbDumpPlanFor(settings store.Settings, tg store.Target, name string, in model.Inspect) *backup.DBDumpPlan {
	if !settings.DBDumpsEnabled || tg.DBDumpOff || !in.Running {
		return nil
	}
	image := in.Config.Image
	if image == "" {
		image = in.Image
	}
	chosen, _ := dbdump.ParseEngine(tg.DBDumpEngine)
	rec := dbdump.Resolve(image, envNames(in.Config.Env), in.Config.Labels, chosen)
	if !rec.Default {
		return nil
	}
	return &backup.DBDumpPlan{
		Engine:     string(rec.Engine),
		Identity:   dbDumpIdentity(name),
		StdinPath:  dbDumpStdinPath(name),
		Image:      dbDumpImageTagValue(name, in.Config.Image),
		MaxRuntime: dbDumpMaxRuntime(),
	}
}

// maxTagValue is how long a tag value may be before restic and the listings
// built on it become unwieldy.
const maxTagValue = 255

// dbDumpImageTagValue returns the image reference to tag the snapshot with, or
// "" when it could not be one.
func dbDumpImageTagValue(name, image string) string {
	if image == "" {
		return ""
	}
	if tagValueError(image) != nil || len(image) > maxTagValue {
		log.Printf("api: database dump of %q: image reference is not usable as a tag, leaving it off the snapshot", name) //nolint:gosec // G706: name is %q-quoted
		return ""
	}
	return image
}

// envNames returns the variable names of a container's environment, dropping
// every value: recognition reads names only and a value can be a password.
func envNames(env []string) []string {
	names := make([]string, 0, len(env))
	for _, e := range env {
		name, _, _ := strings.Cut(e, "=")
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// The data coverage values say what a files backup of a database container is
// worth: a copy taken while the server was stopped, one taken while it was
// running, none at all, or a data directory that could not be located.
const (
	dbCoverageStopped = "stopped"
	dbCoverageLive    = "live"
	dbCoverageNone    = "none"
	dbCoverageUnknown = "unknown"
)

// dbDataCoverage locates the engine's data directory on the host and reports
// which of those a files backup makes of it. toContainer translates a host path
// into this process's view; stackDir, effective and folderSetPaths are already
// in it.
func dbDataCoverage(in model.Inspect, e dbdump.Engine, toContainer func(string) (string, bool), stackDir string, effective, folderSetPaths []string) string {
	mounts := make([]dbdump.Mount, 0, len(in.Mounts))
	for _, m := range in.Mounts {
		mounts = append(mounts, dbdump.Mount{Source: m.Source, Destination: m.Destination})
	}
	host, found := dbdump.DataMount(e, in.Config.Env, mounts)
	if !found {
		return dbCoverageUnknown
	}
	dir, reachable := toContainer(host)
	if !reachable {
		return dbCoverageNone
	}
	if isUnderAny(dir, effective) {
		return dbCoverageStopped
	}
	// An empty stackDir would match every absolute path, so it never reaches
	// isUnderAny.
	if isUnderAny(dir, folderSetPaths) || (stackDir != "" && isUnderAny(dir, []string{stackDir})) {
		return dbCoverageLive
	}
	return dbCoverageNone
}

// dbDumpRow is one container's database picture as the Containers page reads
// it. It carries no value out of the container's environment: the variables are
// read for their names inside dbDumpRows and dropped there.
type dbDumpRow struct {
	Engine      string
	Suggested   string
	Tier        string
	LabelOff    bool
	Coverage    string
	HookOverlap bool
}

// dumpToolRe finds a dump tool in a stored pre-hook, so the card can say that
// this container would be dumped twice.
var dumpToolRe = regexp.MustCompile(`\b(pg_dump|pg_dumpall|mysqldump|mariadb-dump)\b`)

// dbDumpRows describes the database containers among the listed rows, keyed by
// name. Only a row that can be a database at all is inspected: a host runs
// hundreds of containers and the page asks for all of them at once.
func (s *Service) dbDumpRows(ctx context.Context, infos []dockercli.ContainerInfo, byName map[string]store.Target) map[string]dbDumpRow {
	out := map[string]dbDumpRow{}
	folderSets := sync.OnceValue(s.folderSetPaths)

	for _, c := range infos {
		tg := byName[c.Name]
		if !mayHoldADatabase(c, tg) {
			continue
		}
		chosen, _ := dbdump.ParseEngine(tg.DBDumpEngine)
		row := dbDumpRow{HookOverlap: dumpToolRe.MatchString(tg.PreHook)}

		in, err := s.docker.Inspect(ctx, c.Name)
		if err != nil {
			log.Printf("api: list containers: inspecting %q for its database failed, its data coverage is unknown: %v", c.Name, err) //nolint:gosec // G706: name is %q-quoted
			rec := dbdump.Resolve(c.Image, nil, c.Labels, chosen)
			if rec.Tier == dbdump.TierNone {
				continue
			}
			row.Engine, row.Suggested, row.Tier = string(rec.Engine), string(rec.Suggested), string(rec.Tier)
			row.LabelOff, row.Coverage = rec.LabelOff, dbCoverageUnknown
			out[c.Name] = row
			continue
		}

		image := in.Config.Image
		if image == "" {
			image = in.Image
		}
		rec := dbdump.Resolve(image, envNames(in.Config.Env), in.Config.Labels, chosen)
		if rec.Tier == dbdump.TierNone {
			continue
		}
		// A container the user has not decided about yet is located by the
		// guessed engine: the coverage line is what tells them whether the dump
		// would be the only consistent copy.
		engine := rec.Engine
		if engine == dbdump.EngineNone {
			engine = rec.Suggested
		}
		effective, _ := s.effectiveBackupPathsWithSelection(c.Name, in)
		_, stackDir, _ := s.stackDirFor(in)

		row.Engine, row.Suggested, row.Tier = string(rec.Engine), string(rec.Suggested), string(rec.Tier)
		row.LabelOff = rec.LabelOff
		row.Coverage = dbDataCoverage(in, engine, s.toContainerPath, stackDir, effective, folderSets())
		out[c.Name] = row
	}
	return out
}

// dbDumpRepoWords are the words a repository path carries when an image might
// be a database server.
var dbDumpRepoWords = []string{"postgres", "mysql", "mariadb"}

// mayHoldADatabase decides from the list row alone whether a container is worth
// an inspect. An image that is a bare digest, which is what a container carries
// after its image was replaced, is inspected because the row says nothing at
// all about it.
func mayHoldADatabase(c dockercli.ContainerInfo, tg store.Target) bool {
	if tg.DBDumpEngine != "" || strings.HasPrefix(c.Image, "sha256:") {
		return true
	}
	if decision, _ := dbdump.LabelDecisionFor(c.Labels); decision != dbdump.LabelDefault {
		return true
	}
	if dbdump.EngineFor(c.Image) != dbdump.EngineNone {
		return true
	}
	repo := dbdump.CanonicalRepo(c.Image)
	for _, word := range dbDumpRepoWords {
		if strings.Contains(repo, word) {
			return true
		}
	}
	return false
}

// folderSetPaths are the folder sets' roots in this process's view, the second
// way a database's data directory ends up in a backup taken while the server
// runs.
func (s *Service) folderSetPaths() []string {
	sets, err := s.store.ListFileSets()
	if err != nil {
		log.Printf("api: database dumps: listing the folder sets failed, their paths do not count towards data coverage: %v", err)
		return nil
	}
	out := make([]string, 0, len(sets))
	for _, set := range sets {
		if resolved, rErr := paths.Resolve(s.cfg.HostMountRoot, set.Path); rErr == nil {
			out = append(out, resolved)
		}
	}
	return out
}

// SetDBDumpOff opts a container out of the automatic database dump, or back in.
func (s *Service) SetDBDumpOff(_ context.Context, name string, off bool) error {
	return s.store.SetDBDumpOff(name, off)
}

// errUnknownDBDumpEngine is the refusal the PATCH route answers with 400: an
// engine outside the three the dump scripts speak is a client mistake, not a
// state this instance could reach.
var errUnknownDBDumpEngine = errors.New("unknown database engine")

// SetDBDumpEngine records the engine a container that only looks like a
// database is dumped with. An empty engine leaves it to the image again.
func (s *Service) SetDBDumpEngine(_ context.Context, name, engine string) error {
	if _, ok := dbdump.ParseEngine(engine); engine != "" && !ok {
		return fmt.Errorf("%w %q", errUnknownDBDumpEngine, engine)
	}
	return s.store.SetDBDumpEngine(name, engine)
}

// imageBinaryPath is where the binary sits in BombVault's own image, the
// answer when this process cannot say where it was started from.
const imageBinaryPath = "/usr/local/bin/bombvault"

// helperBinaryPath resolves this binary's own path, which restic runs again as
// the dump helper.
func helperBinaryPath() string {
	exe, err := os.Executable()
	if err != nil {
		return imageBinaryPath
	}
	if resolved, rErr := filepath.EvalSymlinks(exe); rErr == nil {
		return resolved
	}
	return exe
}

// dbDumpHelperArgv is the command restic runs to get the dump on its stdin.
func (s *Service) dbDumpHelperArgv(container string, p backup.DBDumpPlan) []string {
	return []string{
		s.dbDumpHelper, "dbdump-stream",
		"--container", container,
		"--engine", p.Engine,
		"--max-seconds", strconv.Itoa(int(p.MaxRuntime / time.Second)),
	}
}

const (
	// dbDumpContextGrace sits above the helper's own deadline: restic can spend
	// minutes waiting for a repository lock and loading the index before the
	// helper is even started.
	dbDumpContextGrace = 30 * time.Minute
	// dbDumpProbeTimeout bounds the read-only exec that reports the server
	// version and the database names.
	dbDumpProbeTimeout = 30 * time.Second
	// dbDumpOrphanStopTimeout bounds the exec that signals a dump left running
	// inside the container.
	dbDumpOrphanStopTimeout = 10 * time.Second
	// dbDumpForgetTimeout bounds removing a snapshot that must not exist. The
	// domain lock is already held, so this waits for nothing but restic.
	dbDumpForgetTimeout = 2 * time.Minute
	// dbDumpProgressEvery throttles the byte counter on its way to the card.
	dbDumpProgressEvery = time.Second
)

// dbDumpScopeOneDatabase is the scope the dump script reports when the
// credentials in the container reach a single database rather than the server.
const dbDumpScopeOneDatabase = "database"

// dbDumpAdapter takes one dump: it runs restic against the helper command,
// turns what comes back into exactly one run reason, and leaves neither a
// snapshot nor a dump process behind that the reason does not account for.
type dbDumpAdapter struct {
	svc         *Service
	engine      ResticEngine
	docker      dockercli.Docker
	mode        restic.Mode
	container   string
	progressKey string
	startedAt   int64
	// guard arms the dump-scoped stall guard; nil means armStallGuard. A field
	// so a test can trip it in milliseconds, where the real one counts hours.
	guard func(ctx context.Context, cancel context.CancelFunc, label string) context.Context
}

var _ backup.DBDumper = (*dbDumpAdapter)(nil)

func (a *dbDumpAdapter) Dump(ctx context.Context, req backup.DBDumpRequest) (backup.DBDumpResult, error) {
	engine, _ := dbdump.ParseEngine(req.Plan.Engine)
	tags := append(req.Tags, a.probeTags(ctx, engine)...)

	dumpCtx, cancel := context.WithTimeout(ctx, req.Plan.MaxRuntime+dbDumpContextGrace)
	defer cancel()
	arm := a.guard
	if arm == nil {
		arm = armStallGuard
	}
	// The guard replaces the watcher the backup armed on ctx, so a stalled dump
	// cancels the dump and nothing else; the publisher chains after it.
	dumpCtx = arm(dumpCtx, cancel, "database dump")
	dumpCtx = restic.WithAddedWatcher(dumpCtx, a.publishBytes())

	sum, lines, err := a.engine.BackupFromCommand(dumpCtx, req.Repo, req.Plan.StdinPath, tags,
		a.svc.dbDumpHelperArgv(a.container, req.Plan), a.mode)

	res, dumpErr := a.decide(ctx, dumpCtx, req, sum, lines, err)
	if dumpErr != nil {
		a.stopOrphan(ctx, lines)
	}
	a.publishStage("", 0)
	return res, dumpErr
}

// decide names the one outcome of a finished dump call. The first match wins:
// a user's cancel must never be booked as a time limit, and a snapshot restic
// wrote is the truth about the dump unless the helper contradicts it.
func (a *dbDumpAdapter) decide(ctx, dumpCtx context.Context, req backup.DBDumpRequest, sum restic.Summary, lines []string, err error) (backup.DBDumpResult, error) {
	res, hasRes := dbdump.ParseResult(lines)
	fail := func(reason string) (backup.DBDumpResult, error) {
		return backup.DBDumpResult{}, &backup.DBDumpError{Reason: reason}
	}

	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		return fail(store.ReasonCancelled)
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return fail(store.ReasonDBDumpBackupCap)
	case errors.Is(dumpCtx.Err(), context.Canceled):
		return fail(store.ReasonDBDumpStalled)
	case errors.Is(dumpCtx.Err(), context.DeadlineExceeded):
		return fail(store.ReasonDBDumpTimeout)
	}

	var partial *restic.CommandSnapshotPartialError
	if errors.As(err, &partial) {
		return a.forget(ctx, req.Repo, partial.SnapshotID, store.ReasonDBDumpIncomplete)
	}
	if err == nil && sum.SnapshotID != "" {
		switch {
		case hasRes && !res.OK:
			return a.forget(ctx, req.Repo, sum.SnapshotID, dbDumpReasonText(res.Reason, res.Detail))
		case hasRes && sum.TotalBytesProcessed != uint64(res.Bytes): //nolint:gosec // G115: ParseResult refuses a negative byte count
			return a.forget(ctx, req.Repo, sum.SnapshotID, store.ReasonDBDumpMismatch)
		}
		return a.success(sum, res, hasRes, lines), nil
	}

	switch {
	case hasRes && !res.OK:
		return fail(dbDumpReasonText(res.Reason, res.Detail))
	case hasRes && res.OK:
		return fail(store.ReasonDBDumpRepository + ": " + scrubError(err))
	case err != nil:
		return fail(store.ReasonDBDumpHelper + ": " + scrubError(err))
	}
	return fail(store.ReasonDBDumpHelper)
}

// success reads the finished dump. restic's snapshot is authoritative: a
// missing result line costs the byte count the helper would have confirmed,
// not the dump.
func (a *dbDumpAdapter) success(sum restic.Summary, res dbdump.Result, hasRes bool, lines []string) backup.DBDumpResult {
	out := backup.DBDumpResult{SnapshotID: sum.SnapshotID, Bytes: int64(sum.TotalBytesProcessed)} //nolint:gosec // G115: a dump stream is gigabytes at most, nowhere near MaxInt64
	scope := dbdump.ParseScope(lines)
	if hasRes {
		out.Bytes = res.Bytes
		if res.Scope != "" {
			scope = res.Scope
		}
	} else {
		log.Printf("api: database dump of %q: restic wrote snapshot %s but the helper reported no result", a.container, shortID(sum.SnapshotID)) //nolint:gosec // G706: name is %q-quoted
	}
	if scope == dbDumpScopeOneDatabase {
		out.Note = store.NoteDBDumpOneDatabase
	}
	return out
}

// forget removes a snapshot that must not be trusted: an incomplete stream, a
// byte mismatch. A snapshot that cannot be removed becomes the leftover reason
// instead, so the dump list can mark exactly it.
func (a *dbDumpAdapter) forget(ctx context.Context, repo, snapshotID, reason string) (backup.DBDumpResult, error) {
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dbDumpForgetTimeout)
	defer cancel()
	if err := a.engine.Forget(fctx, repo, []string{snapshotID}, false, a.mode); err != nil {
		log.Printf("api: database dump of %q: %s, and snapshot %s could not be removed: %v", a.container, reason, shortID(snapshotID), err) //nolint:gosec // G706: name is %q-quoted
		return backup.DBDumpResult{}, &backup.DBDumpError{
			Reason:     store.ReasonDBDumpLeftover + ": " + shortID(snapshotID),
			SnapshotID: snapshotID,
		}
	}
	return backup.DBDumpResult{}, &backup.DBDumpError{Reason: reason}
}

// probeTags reads the server version and the database names right before the
// dump. Everything about the probe is optional: it only adds tags.
func (a *dbDumpAdapter) probeTags(ctx context.Context, engine dbdump.Engine) []string {
	argv, err := dbdump.ProbeArgv(engine)
	if err != nil {
		log.Printf("api: database dump of %q: %v", a.container, err) //nolint:gosec // G706: name is %q-quoted
		return nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, dbDumpProbeTimeout)
	defer cancel()
	out, _, err := a.docker.ExecOutput(probeCtx, a.container, argv, 16<<10)
	if err != nil {
		log.Printf("api: database dump of %q: the probe did not answer, dumping without a version or database tags: %v", a.container, err) //nolint:gosec // G706: name is %q-quoted
		return nil
	}
	p := dbdump.ParseProbe(out)
	if p.Dropped > 0 {
		log.Printf("api: database dump of %q: %d database names are not on the snapshot", a.container, p.Dropped) //nolint:gosec // G706: name is %q-quoted
	}
	var tags []string
	if p.Version != "" {
		tags = append(tags, "dbversion:"+p.Version)
	}
	for _, db := range p.Databases {
		tags = append(tags, "dbname:"+db)
	}
	return tags
}

// stopOrphan signals a dump still running inside the container after restic
// left without a snapshot. Docker keeps an exec alive when its attach closes,
// so nothing else ends that transaction.
func (a *dbDumpAdapter) stopOrphan(ctx context.Context, lines []string) {
	pid, ok := dbdump.ParsePID(lines)
	if !ok {
		return
	}
	argv, err := dbdump.OrphanStopArgv(pid)
	if err != nil {
		return
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dbDumpOrphanStopTimeout)
	defer cancel()
	if err := a.docker.Exec(stopCtx, a.container, argv); err != nil {
		log.Printf("api: database dump of %q: the dump inside the container could not be stopped and may run until its time limit: %v", a.container, err) //nolint:gosec // G706: name is %q-quoted
	}
}

// publishBytes sends the stream's byte counter to the container's card,
// throttled, so the bar moves while a dump runs.
func (a *dbDumpAdapter) publishBytes() restic.ProgressWatcher {
	var last time.Time
	return func(p restic.Progress) {
		if now := time.Now(); now.Sub(last) >= dbDumpProgressEvery {
			last = now
			a.publishStage("dbdump", int64(p.BytesDone)) //nolint:gosec // G115: restic's byte counter of one dump, nowhere near MaxInt64
		}
	}
}

// publishStage goes straight to the progress store rather than through the
// context's sink, which carries the surrounding backup's percentage.
func (a *dbDumpAdapter) publishStage(stage string, bytes int64) {
	if a.svc.progress == nil {
		return
	}
	a.svc.progress.Publish(progress.Event{
		Key:       a.progressKey,
		Phase:     "backup",
		Stage:     stage,
		Bytes:     bytes,
		Active:    true,
		StartedAt: a.startedAt,
	})
}

// dbDumpReasonText maps a helper reason id to the one run reason it stands
// for, with the tool's own message behind it when there is one.
func dbDumpReasonText(reasonID, detail string) string {
	reason := store.ReasonDBDumpHelper
	switch reasonID {
	case dbdump.ReasonAuth:
		reason = store.ReasonDBDumpAuth
	case dbdump.ReasonPrivileges:
		reason = store.ReasonDBDumpPrivileges
	case dbdump.ReasonUnreachable:
		reason = store.ReasonDBDumpUnreachable
	case dbdump.ReasonNoClient:
		reason = store.ReasonDBDumpNoClient
	case dbdump.ReasonSecret:
		reason = store.ReasonDBDumpSecret
	case dbdump.ReasonNoCredentials:
		reason = store.ReasonDBDumpNoCredentials
	case dbdump.ReasonNotRunning:
		reason = store.ReasonDBDumpNotRunning
	case dbdump.ReasonNeedsUpgrade:
		reason = store.ReasonDBDumpNeedsUpgrade
	case dbdump.ReasonTimeout:
		reason = store.ReasonDBDumpTimeout
	case dbdump.ReasonEmpty:
		reason = store.ReasonDBDumpEmpty
	case dbdump.ReasonIncomplete:
		reason = store.ReasonDBDumpIncomplete
	case dbdump.ReasonTool:
		reason = store.ReasonDBDumpTool
	case dbdump.ReasonDocker:
		reason = store.ReasonDBDumpDocker
	case dbdump.ReasonWrite:
		reason = store.ReasonDBDumpRepository
	case dbdump.ReasonCancelled:
		reason = store.ReasonCancelled
	}
	if detail != "" {
		return reason + ": " + detail
	}
	return reason
}

// forgetDBDumpSeries applies the retention policy to a container's dumps as
// their own series and without pruning: the container's own pass right after
// it reclaims the space of both in one go. A renamed container's dumps age
// with it, under the same tags its volume snapshots are forgotten by.
func (s *Service) forgetDBDumpSeries(ctx context.Context, repo string, settings store.Settings, mode restic.Mode, name string) {
	p := s.retentionPolicy(settings)
	if !p.Any() || s.primaryIsImmutable("containers", repo) {
		return
	}
	tags, ok := s.retentionTagsFor(ctx, repo, mode, s.containerDumpIdentity(name))
	if !ok {
		return
	}
	if err := s.forgetWithLockHeal(ctx, repo, p, mode, tags, false); err != nil {
		log.Printf("api: retention of the database dumps of %q failed (the backup is safe): %v", name, err) //nolint:gosec // G706: name is %q-quoted
	}
}

// dbDumpTally collects the dumps that failed during one scheduled round, so
// the round's single summary can name them.
type dbDumpTally struct {
	mu     sync.Mutex
	failed []string
}

type dbDumpTallyKey struct{}

// withDBDumpTally marks ctx as one scheduled round. It belongs where the round
// suppresses the per-item messages, and the same ctx has to reach
// ScheduledNotifyResult.
func withDBDumpTally(ctx context.Context) context.Context {
	return context.WithValue(ctx, dbDumpTallyKey{}, &dbDumpTally{})
}

func dbDumpTallyFrom(ctx context.Context) *dbDumpTally {
	t, _ := ctx.Value(dbDumpTallyKey{}).(*dbDumpTally)
	return t
}

func (t *dbDumpTally) add(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failed = append(t.failed, name)
}

// line is what the round's summary appends, empty when every dump went
// through.
func (t *dbDumpTally) line() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.failed) == 0 {
		return ""
	}
	names := t.failed
	tail := ""
	if len(names) > maxListedFailures {
		names, tail = names[:maxListedFailures], fmt.Sprintf(", +%d more", len(names)-maxListedFailures)
	}
	return fmt.Sprintf("%d database dumps failed: %s%s", len(t.failed), strings.Join(names, ", "), tail)
}

// notifyDBDumpFailed reports a failed dump once the backup around it has its
// own result, so the message can say whether the files made it. A dump that
// keeps failing for the same reason is reported once; the run rows and the
// badge carry the repeats.
func (s *Service) notifyDBDumpFailed(ctx context.Context, targetID, name string, o backup.DBDumpOutcome, backupOK bool) {
	if o.Status != "failed" || dbDumpReasonHead(o.Reason) == store.ReasonCancelled {
		return
	}
	c, err := s.NotifyConfig()
	if err != nil || (c.On != "always" && c.On != "failure") {
		return
	}
	if s.dbDumpFailedBefore(targetID, o) {
		return
	}
	if tally := dbDumpTallyFrom(ctx); tally != nil && c.ScheduledSummary && notify.MessagesSuppressed(ctx) {
		tally.add(name)
		return
	}

	outcome := "The files backup of the container succeeded."
	if !backupOK {
		outcome = "The files backup of the container failed as well; see its own message."
	}
	msg := fmt.Sprintf("Database dump of container %q failed: %s. %s", name, dbDumpFailureSentence(o.Reason), outcome)
	// The detail can quote row data, so it stays in the run row: a webhook, a
	// chat room and a mailbox are not where a database error belongs.
	notify.Send(notify.WithHealthchecksSuppressed(ctx), c, "containers",
		notify.Event{Title: "BombVault: database dump FAILED", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: database dump FAILED", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// dbDumpFailedBefore reports whether the target's previous dump run already
// failed for the same reason.
func (s *Service) dbDumpFailedBefore(targetID string, o backup.DBDumpOutcome) bool {
	runs, err := s.store.RecentRunsOfKind(targetID, "dbdump", 2)
	if err != nil {
		log.Printf("api: database dump: read the previous run: %v", err)
		return false
	}
	for _, r := range runs {
		if r.ID == o.RunID {
			continue
		}
		return r.Status == "failed" && dbDumpReasonHead(r.Error) == dbDumpReasonHead(o.Reason)
	}
	return false
}

// dbDumpReasonConstants are the reasons a dump run can carry. A stored reason
// is one of them, optionally followed by the tool's message.
var dbDumpReasonConstants = []string{
	store.ReasonDBDumpAuth, store.ReasonDBDumpPrivileges, store.ReasonDBDumpUnreachable,
	store.ReasonDBDumpNoClient, store.ReasonDBDumpSecret, store.ReasonDBDumpNoCredentials,
	store.ReasonDBDumpNotRunning, store.ReasonDBDumpNeedsUpgrade, store.ReasonDBDumpTimeout,
	store.ReasonDBDumpBackupCap, store.ReasonDBDumpStalled, store.ReasonDBDumpEmpty,
	store.ReasonDBDumpIncomplete, store.ReasonDBDumpTool, store.ReasonDBDumpDocker,
	store.ReasonDBDumpRepository, store.ReasonDBDumpHelper, store.ReasonDBDumpMismatch,
	store.ReasonDBDumpLeftover, store.ReasonCancelled,
}

// dbDumpReasonHead returns the constant a stored reason starts with, so two
// failures compare on what went wrong rather than on the tool's message.
func dbDumpReasonHead(reason string) string {
	for _, known := range dbDumpReasonConstants {
		if reason == known || strings.HasPrefix(reason, known+": ") {
			return known
		}
	}
	return reason
}

// dbDumpFailureSentence is the reason as a notification says it, without the
// head every dump failure shares and without the tool's own message.
func dbDumpFailureSentence(reason string) string {
	return strings.TrimPrefix(dbDumpReasonHead(reason), "database dump failed: ")
}
