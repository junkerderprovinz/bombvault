package api

import (
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"filippo.io/age"

	"github.com/junkerderprovinz/bombvault/internal/ageseal"
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

// The describing tags of a dump snapshot. None of them is an identity: they say
// what was dumped and which backup run the dump belongs to.
const (
	dbDumpEngineTagPrefix  = "dbengine:"
	dbDumpImageTagPrefix   = "dbimage:"
	dbDumpVersionTagPrefix = "dbversion:"
	dbDumpNameTagPrefix    = "dbname:"
	dbDumpRunTagPrefix     = "bvrun:"
)

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

// dumpMounts is a container's mount table as the recognition reads it.
func dumpMounts(in model.Inspect) []dbdump.Mount {
	mounts := make([]dbdump.Mount, 0, len(in.Mounts))
	for _, m := range in.Mounts {
		mounts = append(mounts, dbdump.Mount{Source: m.Source, Destination: m.Destination})
	}
	return mounts
}

// dbDataCoverage locates the engine's data directory on the host and reports
// which of those a files backup makes of it. toContainer translates a host path
// into this process's view; stackDir, effective and folderSetPaths are already
// in it.
func dbDataCoverage(in model.Inspect, e dbdump.Engine, toContainer func(string) (string, bool), stackDir string, effective, folderSetPaths []string) string {
	host, found := dbdump.DataMount(e, in.Config.Env, dumpMounts(in))
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
		tags = append(tags, dbDumpVersionTagPrefix+p.Version)
	}
	for _, db := range p.Databases {
		tags = append(tags, dbDumpNameTagPrefix+db)
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
	a.svc.publishDBDumpStage(a.progressKey, "backup", stage, a.startedAt, bytes)
}

// publishDBDumpStage marks a container's progress entry with the dump step that
// is running and how many bytes of the stream have moved.
func (s *Service) publishDBDumpStage(key, phase, stage string, startedAt, bytes int64) {
	if s.progress == nil {
		return
	}
	s.progress.Publish(progress.Event{
		Key:       key,
		Phase:     phase,
		Stage:     stage,
		Bytes:     bytes,
		Active:    true,
		StartedAt: startedAt,
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

// take is what the round's summary appends, empty when every dump went
// through. It empties the tally, so rounds that overlap and share it name each
// failure once.
func (t *dbDumpTally) take() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	all := t.failed
	t.failed = nil
	if len(all) == 0 {
		return ""
	}
	names, tail := all, ""
	if len(names) > maxListedFailures {
		names, tail = names[:maxListedFailures], fmt.Sprintf(", +%d more", len(names)-maxListedFailures)
	}
	return fmt.Sprintf("%d database dumps failed: %s%s", len(all), strings.Join(names, ", "), tail)
}

// ScheduledRounds hands the scheduler's per-domain rounds the context their
// items and their summary share, which is where a failed dump is tallied in
// summary mode. Rounds of one domain that overlap, such as a per-item schedule
// firing during the domain run, share one tally.
type ScheduledRounds struct {
	mu   sync.Mutex
	open map[string]*scheduledRound
}

type scheduledRound struct {
	ctx  context.Context
	open int
}

func NewScheduledRounds() *ScheduledRounds {
	return &ScheduledRounds{open: map[string]*scheduledRound{}}
}

// Begin opens a round of domain.
func (r *ScheduledRounds) Begin(domain string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if round, ok := r.open[domain]; ok {
		round.open++
		return
	}
	r.open[domain] = &scheduledRound{ctx: withDBDumpTally(context.Background()), open: 1}
}

// Context is the base context for an item of domain's open round.
func (r *ScheduledRounds) Context(domain string) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	if round, ok := r.open[domain]; ok {
		return round.ctx
	}
	return context.Background()
}

// End closes a round of domain and returns the context its summary goes out
// with.
func (r *ScheduledRounds) End(domain string) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	round, ok := r.open[domain]
	if !ok {
		return context.Background()
	}
	if round.open--; round.open == 0 {
		delete(r.open, domain)
	}
	return round.ctx
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
	if s.unraidGate(c.Unraid) && (!notify.MessagesSuppressed(ctx) || !c.ScheduledSummary) {
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

// DBDumpView is one database dump as the container's dump list shows it.
type DBDumpView struct {
	ID        string   `json:"id"`
	Time      string   `json:"time"`
	Engine    string   `json:"engine"`
	Image     string   `json:"image"`
	Version   string   `json:"version"`
	Databases []string `json:"databases"`
	Bytes     int64    `json:"bytes"`
	// Damaged marks a snapshot a failed dump left behind: it can be deleted and
	// nothing else.
	Damaged bool `json:"damaged"`
	// PairedSnapshotID is the local volume snapshot taken in the same backup.
	// The client matches it against a snapshot's original, else its id, so the
	// pairing also shows on an off-site source, where every snapshot was copied
	// under a new id.
	PairedSnapshotID string `json:"pairedSnapshotId,omitempty"`
}

// dbDumpSnapshot is a dump as the list shows it plus what only the download
// needs: the paths restic stored, which say whether the snapshot really holds
// this container's stream.
type dbDumpSnapshot struct {
	view  DBDumpView
	paths []string
	runID string
}

// dbDumpSource is the repository a container's dumps were resolved to, carried
// along so a download does not resolve it a second time.
type dbDumpSource struct {
	repo string
	mode restic.Mode
}

// DBDumps lists a container's database dumps in the given source, newest first.
func (s *Service) DBDumps(ctx context.Context, name, source string) ([]DBDumpView, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	dumps, _, err := s.dbDumpsOf(ctx, settings, name, source)
	if err != nil {
		return nil, err
	}
	out := make([]DBDumpView, 0, len(dumps))
	for _, d := range dumps {
		out = append(out, d.view)
	}
	return out, nil
}

func (s *Service) dbDumpsOf(ctx context.Context, settings store.Settings, name, source string) ([]dbDumpSnapshot, dbDumpSource, error) {
	if !validResourceName(name) {
		return nil, dbDumpSource{}, errors.New("invalid container name")
	}
	if source != "local" && !isOffsiteSource(source) {
		return nil, dbDumpSource{}, errors.New("invalid source (must be local or offsite)")
	}
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return nil, dbDumpSource{}, err
	}
	src := dbDumpSource{repo: repo, mode: s.repoModeFor(settings, "containers", source, repo)}
	snaps, err := s.snapshotsOwnedBy(ctx, repo, src.mode, s.containerDumpIdentity(name))
	if err != nil {
		return nil, dbDumpSource{}, err
	}
	damaged, err := s.damagedDBDumpSnapshots(name)
	if err != nil {
		return nil, dbDumpSource{}, err
	}

	dumps := make([]dbDumpSnapshot, 0, len(snaps))
	runIDs := make([]string, 0, len(snaps))
	// restic reports oldest first and the list reads newest first.
	for i := len(snaps) - 1; i >= 0; i-- {
		sn := snaps[i]
		view, runID := dbDumpViewOf(sn)
		view.Damaged = damaged[sn.ID] || (sn.Original != "" && damaged[sn.Original])
		dumps = append(dumps, dbDumpSnapshot{view: view, paths: sn.Paths, runID: runID})
		if runID != "" {
			runIDs = append(runIDs, runID)
		}
	}

	paired, err := s.store.BackupSnapshotsOfRuns(runIDs)
	if err != nil {
		return nil, dbDumpSource{}, err
	}
	for i := range dumps {
		dumps[i].view.PairedSnapshotID = paired[dumps[i].runID]
	}
	return dumps, src, nil
}

// dbDumpViewOf reads a dump snapshot's tags into its row and returns the id of
// the backup run the dump was taken for.
func dbDumpViewOf(sn restic.Snapshot) (DBDumpView, string) {
	v := DBDumpView{ID: sn.ID, Time: sn.Time, Databases: []string{}}
	if sn.Summary != nil {
		v.Bytes = int64(sn.Summary.TotalBytesProcessed) //nolint:gosec // G115: one dump stream, nowhere near MaxInt64
	}
	var runID string
	for _, t := range sn.Tags {
		switch {
		case strings.HasPrefix(t, dbDumpEngineTagPrefix):
			v.Engine = strings.TrimPrefix(t, dbDumpEngineTagPrefix)
		case strings.HasPrefix(t, dbDumpImageTagPrefix):
			v.Image = strings.TrimPrefix(t, dbDumpImageTagPrefix)
		case strings.HasPrefix(t, dbDumpVersionTagPrefix):
			v.Version = strings.TrimPrefix(t, dbDumpVersionTagPrefix)
		case strings.HasPrefix(t, dbDumpNameTagPrefix):
			v.Databases = append(v.Databases, strings.TrimPrefix(t, dbDumpNameTagPrefix))
		case strings.HasPrefix(t, dbDumpRunTagPrefix):
			runID = strings.TrimPrefix(t, dbDumpRunTagPrefix)
		}
	}
	return v, runID
}

// damagedDBDumpSnapshots are the snapshots this container's failed dumps left
// behind. A container without a target row has no runs and so no leftovers.
func (s *Service) damagedDBDumpSnapshots(name string) (map[string]bool, error) {
	tg, err := s.store.GetTargetByContainer(name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read this container's dump runs: %w", err)
	}
	return s.store.FailedDBDumpSnapshots(tg.ID)
}

// What a picked dump can be refused for, as sentinels so an import can name
// the reason the page translates.
var (
	errDBDumpDamaged = errors.New("damaged and can only be deleted")
	errNotADBDump    = errors.New("not a database dump of this container")
	errDBDumpPath    = errors.New("does not hold this container's dump")
)

// dbDumpFor picks one of a container's dumps and refuses what neither a
// download nor a save may touch: a snapshot that is not one of its dumps, one a
// failed dump left behind, one that does not hold this container's stream.
func (s *Service) dbDumpFor(ctx context.Context, settings store.Settings, name, source, snapshotID string) (dbDumpSnapshot, dbDumpSource, error) {
	if !backup.ValidSnapshotID(snapshotID) {
		return dbDumpSnapshot{}, dbDumpSource{}, backup.ErrInvalidSnapshotID
	}
	dumps, src, err := s.dbDumpsOf(ctx, settings, name, source)
	if err != nil {
		return dbDumpSnapshot{}, dbDumpSource{}, err
	}
	for _, d := range dumps {
		if d.view.ID != snapshotID {
			continue
		}
		switch {
		case d.view.Damaged:
			return dbDumpSnapshot{}, dbDumpSource{}, fmt.Errorf("snapshot %s is %w", shortID(snapshotID), errDBDumpDamaged)
		case !slices.Contains(d.paths, dbDumpStdinPath(name)):
			return dbDumpSnapshot{}, dbDumpSource{}, fmt.Errorf("snapshot %s %w", shortID(snapshotID), errDBDumpPath)
		}
		return d, src, nil
	}
	return dbDumpSnapshot{}, dbDumpSource{}, fmt.Errorf("snapshot %s is %w", shortID(snapshotID), errNotADBDump)
}

// DBDumpDownloadName names a dump on its way out of BombVault, so two dumps of
// one container never collide in a download folder.
func DBDumpDownloadName(name string, v DBDumpView, gz, sealed bool) string {
	out := name
	if at, err := time.Parse(time.RFC3339, v.Time); err == nil {
		out += "-" + at.Format("20060102-150405")
	}
	out += "-" + shortID(v.ID) + ".sql"
	if gz {
		out += ".gz"
	}
	if sealed {
		out += ".age"
	}
	return out
}

// DownloadDBDump streams one dump to w, gzip-compressed in the stream when
// asked and age-sealed around that when export encryption is on. onResolved is
// called with the dump once every refusal has passed, so the handler can name
// the attachment before the first byte leaves. check runs the refusals and
// returns without streaming, which is what the browser asks before it starts a
// native download.
func (s *Service) DownloadDBDump(ctx context.Context, name, source, snapshotID string, gz, check bool, onResolved func(v DBDumpView, sealed bool), w io.Writer) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	// The recipients are resolved before anything else: with sealing on but
	// broken, a plaintext stream must never start under a sealed name.
	recipients, sealed, err := s.exportRecipients(settings)
	if err != nil {
		return err
	}
	dump, src, err := s.dbDumpFor(ctx, settings, name, source, snapshotID)
	if err != nil {
		return err
	}
	if check {
		return nil
	}
	if onResolved != nil {
		onResolved(dump.view, sealed)
	}

	dst := w
	var ageW *sealOnFirstWrite
	if sealed {
		ageW = &sealOnFirstWrite{dst: dst, recipients: recipients}
		dst = ageW
	}
	var gzW *gzip.Writer
	if gz {
		gzW = gzip.NewWriter(dst)
		dst = gzW
	}

	log.Printf("api: database dump of %q: streaming snapshot %s to a download", name, shortID(snapshotID)) //nolint:gosec // G706: name is %q-quoted
	if err := s.engine.DumpRaw(ctx, src.repo, snapshotID, dbDumpStdinPath(name), dst, src.mode); err != nil {
		return err
	}
	if gzW != nil {
		if err := gzW.Close(); err != nil {
			return err
		}
	}
	if ageW != nil {
		return ageW.Close()
	}
	return nil
}

// sealOnFirstWrite starts the age stream with the first byte restic delivers.
// age writes its header at once, and a header sent before restic has read a
// byte turns a dump that cannot be read into a download that looks finished.
type sealOnFirstWrite struct {
	dst        io.Writer
	recipients []age.Recipient
	w          io.WriteCloser
}

func (s *sealOnFirstWrite) open() error {
	if s.w != nil {
		return nil
	}
	w, err := ageseal.WrapWriter(s.dst, s.recipients)
	s.w = w
	return err
}

func (s *sealOnFirstWrite) Write(p []byte) (int, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.w.Write(p)
}

// Close seals an empty dump too, so it still decrypts to nothing.
func (s *sealOnFirstWrite) Close() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.w.Close()
}

// A saved dump belongs to Unraid's nobody:users like everything else on a
// share, so it opens over SMB without a detour through the console.
const (
	dbDumpFileUID  = 99
	dbDumpFileGID  = 100
	dbDumpFileMode = 0o640
)

func (s *Service) dbDumpChownFn() func(string, int, int) error {
	if s.dbDumpChown != nil {
		return s.dbDumpChown
	}
	return os.Chown
}

// dbDumpSavePlan is everything StartSaveDBDumpToPath resolved while the request
// was still open, so the detached run only has to write.
type dbDumpSavePlan struct {
	src   dbDumpSource
	dump  DBDumpView
	path  string
	final string
	gz    bool
}

// StartSaveDBDumpToPath writes one dump into a folder on the server. It runs
// under its own run kind, so a failed save does not colour the container's
// backup history; validation happens before it returns and the writing runs
// detached, like a restore into a folder. It returns the file the dump will
// appear as.
func (s *Service) StartSaveDBDumpToPath(ctx context.Context, name, source, snapshotID, targetSubPath string, gz bool) (string, bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return "", false, nil
	}
	plan, err := s.prepareSaveDBDump(ctx, name, source, snapshotID, targetSubPath, gz)
	if err != nil {
		s.batchActive.Store(false)
		return "", false, err
	}
	bctx := context.WithoutCancel(ctx)
	rkey := "container:" + name
	go func() {
		var runID string
		defer s.recoverOperation("save database dump: "+name, nil, func(msg string) {
			s.finishRestoreRun(runID, "", errors.New(msg))
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(rkey, cancel)
		defer s.unregisterCancel(rkey)
		runID = s.beginDBDumpRun(name, "dbdumpsave", "save database dump")
		pctx, startedAt := s.progBegin(rctx, rkey, "restore")
		serr := s.saveDBDump(pctx, plan, rkey, startedAt)
		s.progEnd(rkey, "restore", serr == nil, startedAt)
		s.finishRestoreRun(runID, plan.dump.ID, serr)
		if serr != nil {
			log.Printf("api: save database dump of %q failed: %v", name, serr) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return plan.final, true, nil
}

func (s *Service) prepareSaveDBDump(ctx context.Context, name, source, snapshotID, targetSubPath string, gz bool) (dbDumpSavePlan, error) {
	target, err := paths.Resolve(s.cfg.HostMountRoot, targetSubPath)
	if err != nil {
		return dbDumpSavePlan{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return dbDumpSavePlan{}, fmt.Errorf("read settings: %w", err)
	}
	dump, src, err := s.dbDumpFor(ctx, settings, name, source, snapshotID)
	if err != nil {
		return dbDumpSavePlan{}, err
	}
	if err := paths.EnsureDir(target); err != nil {
		return dbDumpSavePlan{}, fmt.Errorf("create target folder: %w", err)
	}
	final := filepath.Join(target, DBDumpDownloadName(name, dump.view, gz, false))
	if _, err := os.Stat(final); err == nil {
		return dbDumpSavePlan{}, fmt.Errorf("%s already exists in that folder", filepath.Base(final))
	}
	return dbDumpSavePlan{src: src, dump: dump.view, path: dbDumpStdinPath(name), final: final, gz: gz}, nil
}

// saveDBDump writes the dump beside its final name and publishes it with a
// rename, so a half-written file is never mistaken for a dump.
func (s *Service) saveDBDump(ctx context.Context, plan dbDumpSavePlan, key string, startedAt int64) error {
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()

	partial := plan.final + ".partial"
	f, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, dbDumpFileMode) //nolint:gosec // G304: a name built from the container and the snapshot, inside a folder paths.Resolve contained
	if err != nil {
		return err
	}
	if cErr := s.dbDumpChownFn()(partial, dbDumpFileUID, dbDumpFileGID); cErr != nil {
		log.Printf("api: save database dump: %s keeps this process's ownership: %v", filepath.Base(partial), cErr) //nolint:gosec // G706: a name this process built
	}

	err = s.streamDBDumpInto(ctx, plan, f, key, startedAt)
	if cErr := f.Close(); err == nil {
		err = cErr
	}
	if err == nil {
		err = os.Rename(partial, plan.final)
	}
	if err != nil {
		if rErr := os.Remove(partial); rErr != nil && !errors.Is(rErr, os.ErrNotExist) {
			log.Printf("api: save database dump: the unfinished file could not be removed: %v", rErr)
		}
		return err
	}
	return nil
}

func (s *Service) streamDBDumpInto(ctx context.Context, plan dbDumpSavePlan, f *os.File, key string, startedAt int64) error {
	counted := &countingWriter{w: f, publish: func(n int64) {
		s.publishDBDumpStage(key, "restore", "dbdumpsave", startedAt, n)
	}}
	var dst io.Writer = counted
	var gzW *gzip.Writer
	if plan.gz {
		gzW = gzip.NewWriter(counted)
		dst = gzW
	}
	if err := s.engine.DumpRaw(ctx, plan.src.repo, plan.dump.ID, plan.path, dst, plan.src.mode); err != nil {
		return err
	}
	if gzW != nil {
		if err := gzW.Close(); err != nil {
			return err
		}
	}
	return f.Sync()
}

// countingWriter reports how far a stream has come, throttled, so a save of a
// multi-gigabyte dump shows movement on the container's card.
type countingWriter struct {
	w       io.Writer
	publish func(bytes int64)
	written int64
	last    time.Time
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.written += int64(n)
	if now := time.Now(); now.Sub(c.last) >= dbDumpProgressEvery {
		c.last = now
		c.publish(c.written)
	}
	return n, err
}

// beginDBDumpRun opens a run row of its own kind for work on a dump, so the
// run history says a dump was saved or imported instead of counting it as a
// restore. what names the work in the log.
func (s *Service) beginDBDumpRun(name, kind, what string) string {
	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		log.Printf("api: %s: no target row for %q, the outcome stays out of the run history: %v", what, name, err) //nolint:gosec // G706: name is %q-quoted
		return ""
	}
	runID, err := runsAdapter{st: s.store, ctx: context.Background()}.Start(tg.ID, kind)
	if err != nil {
		log.Printf("api: %s: record the run start for %q failed: %v", what, name, err) //nolint:gosec // G706: name is %q-quoted
		return ""
	}
	return runID
}
