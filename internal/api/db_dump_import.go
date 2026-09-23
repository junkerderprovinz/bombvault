package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/compose"
	"github.com/junkerderprovinz/bombvault/internal/dbdump"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The reason ids a refused import carries to the dump list, which turns each
// one into its own sentence.
const (
	importRefusedBusy           = "busy"
	importRefusedNotRunning     = "notRunning"
	importRefusedEngineMismatch = "engineMismatch"
	importRefusedNoDataMount    = "noDataMount"
	importRefusedVersion        = "version"
	importRefusedDamaged        = "damaged"
	importRefusedNotADump       = "notADump"
)

const (
	// dbImportStopTimeout is the stop timeout a backup gives a container.
	dbImportStopTimeout = 30 * time.Second
	dbImportStderrTail  = 64 << 10
	dbImportStamp       = "20060102-150405"
)

// dbImportReadyEvery and dbImportReadyFor bound the wait for the freshly
// initialised server. Without the cap the containers domain lock would be held
// by a server that never comes up. Variables so a test can give up in
// milliseconds.
var (
	dbImportReadyEvery = 2 * time.Second
	dbImportReadyFor   = 5 * time.Minute
)

// importRefusal is an import the server declines before it touches anything.
// The page picks the sentence from the id; a version mismatch fills it with the
// two major versions.
type importRefusal struct {
	code   string
	msg    string
	server int
	dump   int
}

func (e *importRefusal) Error() string { return e.msg }

func refuseImport(code, format string, a ...any) error {
	return &importRefusal{code: code, msg: fmt.Sprintf(format, a...)}
}

// importRefusalFor maps a dump that cannot be picked onto the id the page
// translates.
func importRefusalFor(err error) error {
	switch {
	case errors.Is(err, errDBDumpDamaged):
		return &importRefusal{code: importRefusedDamaged, msg: err.Error()}
	case errors.Is(err, errNotADBDump), errors.Is(err, errDBDumpPath), errors.Is(err, backup.ErrInvalidSnapshotID):
		return &importRefusal{code: importRefusedNotADump, msg: err.Error()}
	}
	return err
}

// errDBImportFolders tags an import outcome whose data folder paths are the
// actionable part of the message, like errRestoreDestination: they are the
// user's own folders and the message is worthless without them.
var errDBImportFolders = errors.New("database import outcome")

type dbImportErr struct {
	msg string
	// dataBack says the container runs on its previous data folder again.
	dataBack bool
}

func (e *dbImportErr) Error() string { return e.msg }

func (e *dbImportErr) Is(target error) bool { return target == errDBImportFolders }

// dbImportDetailMax caps the cause behind an import outcome. The run reason is
// not cut as a whole, because the folder paths ahead of the cause are the part
// the user needs.
const dbImportDetailMax = 300

func importDetail(s string) string {
	if len(s) <= dbImportDetailMax {
		return s
	}
	end := dbImportDetailMax
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end]
}

// importToolTail keeps the end of what the import tool wrote, where the line
// that made it exit is, and starts at a whole line where one fits.
func importToolTail(s string) string {
	s = strings.TrimRight(s, "\n")
	if len(s) <= dbImportDetailMax {
		return s
	}
	s = s[len(s)-dbImportDetailMax:]
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	for len(s) > 0 && !utf8.RuneStart(s[0]) {
		s = s[1:]
	}
	return s
}

func importPrepareFailure(cause error) error {
	return &dbImportErr{msg: store.ReasonDBImportPrepare + ": " + importDetail(cause.Error()), dataBack: true}
}

func importRollbackFailure(fresh, kept string, cause error) error {
	return &dbImportErr{msg: fmt.Sprintf("%s: the previous data folder is at %s and the fresh one at %s; %s",
		store.ReasonDBImportRollback, kept, fresh, importDetail(cause.Error()))}
}

// importToolFailure takes a detail its caller has already bounded, because
// only the caller knows which end of it matters.
func importToolFailure(kept, detail string) error {
	return &dbImportErr{msg: fmt.Sprintf("%s: the previous data folder is kept at %s; %s",
		store.ReasonDBImportFailed, kept, detail)}
}

// dbImportPlan is what an import resolved while the request was still open: the
// container by name and by id, the dump, the engine, the data folder in this
// process's view and the role a PostgreSQL dump always recreates.
type dbImportPlan struct {
	name    string
	id      string
	src     dbDumpSource
	dump    DBDumpView
	engine  dbdump.Engine
	dataDir string
	pgUser  string
}

// StartImportDBDump imports one dump into a freshly initialised database. Every
// refusal is answered before the first container call; from then on the work
// runs detached under its own run kind, like a restore, and the container's
// previous data folder is set aside rather than deleted. It offers no cancel:
// from the moment that folder moves there is nothing safe to stop.
func (s *Service) StartImportDBDump(ctx context.Context, name, source, snapshotID string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	plan, err := s.prepareImportDBDump(ctx, name, source, snapshotID)
	if err != nil {
		s.batchActive.Store(false)
		return false, err
	}
	bctx := context.WithoutCancel(ctx)
	rkey := "container:" + name
	go func() {
		var runID string
		defer s.recoverOperation("import database dump: "+name, nil, func(msg string) {
			s.finishDBImportRun(runID, "", "", errors.New(msg))
		})
		defer s.batchActive.Store(false)
		ictx, cancel := context.WithTimeout(bctx, restoreTimeout)
		defer cancel()
		runID = s.beginDBDumpRun(name, "dbimport", "import database dump")
		pctx, startedAt := s.progBegin(ictx, rkey, "restore")
		note, ierr := s.importDBDump(pctx, plan, rkey, startedAt)
		s.progEnd(rkey, "restore", ierr == nil, startedAt)
		s.finishDBImportRun(runID, plan.dump.ID, note, ierr)
		if ierr != nil {
			log.Printf("api: import database dump into %q failed: %s", name, shareableRunError("dbimport", ierr.Error())) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return true, nil
}

func (s *Service) prepareImportDBDump(ctx context.Context, name, source, snapshotID string) (dbImportPlan, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return dbImportPlan{}, fmt.Errorf("read settings: %w", err)
	}
	dump, src, err := s.dbDumpFor(ctx, settings, name, source, snapshotID)
	if err != nil {
		return dbImportPlan{}, importRefusalFor(err)
	}
	in, err := s.inspectNamed(ctx, name)
	if err != nil {
		return dbImportPlan{}, err
	}
	if !in.Running {
		return dbImportPlan{}, refuseImport(importRefusedNotRunning, "container %q is not running", name)
	}

	engine, err := s.importEngineFor(name, in)
	if err != nil {
		return dbImportPlan{}, err
	}
	if engine == dbdump.EngineNone || string(engine) != dump.view.Engine {
		return dbImportPlan{}, refuseImport(importRefusedEngineMismatch,
			"this container runs %q and the dump comes from %q", engine, dump.view.Engine)
	}
	dataDir, ok := s.databaseDataDir(in, engine)
	if !ok {
		return dbImportPlan{}, refuseImport(importRefusedNoDataMount,
			"the data folder of %q cannot be reached under the host mount", name)
	}
	server := s.databaseServerVersion(ctx, name, in.ID, engine)
	if !dbdump.VersionMajorOK(engine, dump.view.Version, server) {
		return dbImportPlan{}, &importRefusal{
			code:   importRefusedVersion,
			msg:    fmt.Sprintf("the server runs version %s and the dump comes from version %s", server, dump.view.Version),
			server: versionMajor(server),
			dump:   versionMajor(dump.view.Version),
		}
	}
	return dbImportPlan{
		name:    name,
		id:      in.ID,
		src:     src,
		dump:    dump.view,
		engine:  engine,
		dataDir: dataDir,
		pgUser:  postgresRole(in.Config.Env),
	}, nil
}

// importEngineFor is the engine an import speaks to: the recognised one, else
// the guess, because asking for an import is the user's word that this
// container is a database.
func (s *Service) importEngineFor(name string, in model.Inspect) (dbdump.Engine, error) {
	tg, err := s.store.GetTargetByContainer(name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return dbdump.EngineNone, fmt.Errorf("read this container's settings: %w", err)
	}
	chosen, _ := dbdump.ParseEngine(tg.DBDumpEngine)
	image := in.Config.Image
	if image == "" {
		image = in.Image
	}
	rec := dbdump.Resolve(image, envNames(in.Config.Env), in.Config.Labels, chosen)
	if rec.Engine != dbdump.EngineNone {
		return rec.Engine, nil
	}
	return rec.Suggested, nil
}

// databaseDataDir locates the engine's data directory in this process's view.
func (s *Service) databaseDataDir(in model.Inspect, e dbdump.Engine) (string, bool) {
	host, found := dbdump.DataMount(e, in.Config.Env, dumpMounts(in))
	if !found {
		return "", false
	}
	return s.toContainerPath(host)
}

// databaseServerVersion asks the running server which version it is. An answer
// that does not come is not a reason to refuse the import.
func (s *Service) databaseServerVersion(ctx context.Context, name, id string, e dbdump.Engine) string {
	argv, err := dbdump.ProbeArgv(e)
	if err != nil {
		return ""
	}
	pctx, cancel := context.WithTimeout(ctx, dbDumpProbeTimeout)
	defer cancel()
	out, _, err := s.docker.ExecOutput(pctx, id, argv, 16<<10)
	if err != nil {
		log.Printf("api: import database dump into %q: the probe did not answer, importing without a version check: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return ""
	}
	return dbdump.ParseProbe(out).Version
}

func versionMajor(version string) int {
	head, _, _ := strings.Cut(version, ".")
	major, _ := strconv.Atoi(head)
	return major
}

// postgresRole is the role a PostgreSQL dump recreates, so the one collision it
// always produces against a fresh cluster is not counted as an error.
func postgresRole(env []string) string {
	for _, e := range env {
		if name, value, ok := strings.Cut(e, "="); ok && name == "POSTGRES_USER" && value != "" {
			return value
		}
	}
	return "postgres"
}

// importDBDump runs the import under the containers restore lock and returns
// the note its run row carries.
func (s *Service) importDBDump(ctx context.Context, plan dbImportPlan, key string, startedAt int64) (string, error) {
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()
	s.publishDBDumpStage(key, "restore", "dbimport", startedAt, 0)

	stopped := s.stopImportDependents(ctx, plan.name)
	var note string
	kept, err := s.freshDataDirFor(ctx, plan)
	if err == nil {
		note, err = s.feedDBImport(ctx, plan, kept, key, startedAt)
	}
	var ierr *dbImportErr
	if errors.As(err, &ierr) && !ierr.dataBack {
		// An app started on a half-imported or empty database runs its
		// migrations there and takes writes that are lost once the kept folder
		// goes back.
		if len(stopped) > 0 {
			ierr.msg += "; " + store.ImportTailAppsStopped + ": " + dependentNames(stopped)
		}
		return note, err
	}
	if down := s.startImportDependents(context.WithoutCancel(ctx), stopped); len(down) > 0 {
		const suffix = "; " + store.ImportTailAppsDown + ": "
		if ierr != nil {
			ierr.msg += suffix + dependentNames(down)
		} else {
			note += suffix + dependentNames(down)
		}
	}
	return note, err
}

// importDependent is a container the import stopped. It is stopped and started
// by id, so a name that meanwhile belongs to nothing cannot reach another
// container.
type importDependent struct {
	name      string
	id        string
	service   string
	dependsOn []string
}

func dependentNames(deps []importDependent) string {
	names := make([]string, len(deps))
	for i, d := range deps {
		names[i] = d.name
	}
	return strings.Join(names, ", ")
}

// stopImportDependents stops the running containers the database's backup
// stops too. An app left running reconnects to the fresh database and can
// create its own schema there before the dump arrives, and the dump then
// collides with it.
func (s *Service) stopImportDependents(ctx context.Context, name string) []importDependent {
	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("api: import database dump into %q: read the containers to stop: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		}
		return nil
	}
	var stopped []importDependent
	for _, dep := range tg.StopContainers {
		di, err := s.inspectNamed(ctx, dep)
		if err != nil {
			log.Printf("api: import database dump into %q: inspect %q: %v (leaving as-is)", name, dep, err) //nolint:gosec // G706: names are %q-quoted
			continue
		}
		if !di.Running {
			continue
		}
		if err := s.docker.Stop(ctx, di.ID, dbImportStopTimeout); err != nil {
			log.Printf("api: import database dump into %q: stop %q failed (continuing): %v", name, dep, err) //nolint:gosec // G706: names are %q-quoted
			continue
		}
		stopped = append(stopped, importDependent{
			name:      dep,
			id:        di.ID,
			service:   composeService(di.Config.Labels),
			dependsOn: parseDependsOn(di.Config.Labels),
		})
	}
	return stopped
}

// startImportDependents brings the stopped containers back in depends_on
// order and returns those that did not start.
func (s *Service) startImportDependents(ctx context.Context, deps []importDependent) []importDependent {
	services := make([]string, len(deps))
	dependsOn := make([][]string, len(deps))
	for i, dep := range deps {
		services[i], dependsOn[i] = dep.service, dep.dependsOn
	}
	var down []importDependent
	for _, i := range compose.StartOrder(services, dependsOn) {
		if err := s.docker.Start(ctx, deps[i].id); err != nil {
			log.Printf("api: import database dump: start %q again failed: %v", deps[i].name, err) //nolint:gosec // G706: name is %q-quoted
			down = append(down, deps[i])
		}
	}
	return down
}

// freshDataDirFor stops the container, sets its data folder aside and lets the
// image initialise an empty one in its place. It returns the folder it kept.
func (s *Service) freshDataDirFor(ctx context.Context, plan dbImportPlan) (string, error) {
	if err := s.docker.Stop(ctx, plan.id, dbImportStopTimeout); err != nil {
		return "", importPrepareFailure(fmt.Errorf("stop the container: %w", err))
	}
	info, err := os.Stat(plan.dataDir)
	if err != nil {
		return "", s.rollbackImport(ctx, plan, "", fmt.Errorf("read the data folder: %w", err))
	}
	kept := plan.dataDir + ".bombvault-before-import-" + time.Now().Format(dbImportStamp)
	if err := os.Rename(plan.dataDir, kept); err != nil {
		return "", s.rollbackImport(ctx, plan, "", fmt.Errorf("set the data folder aside: %w", err))
	}
	if err := recreateDataDir(plan.dataDir, info, s.dbDumpChownFn()); err != nil {
		return "", s.rollbackImport(ctx, plan, kept, err)
	}
	log.Printf("api: import database dump into %q: the previous data folder is kept at %s", plan.name, kept) //nolint:gosec // G706: name is %q-quoted
	if err := s.docker.Start(ctx, plan.id); err != nil {
		return "", s.rollbackImport(ctx, plan, kept, fmt.Errorf("start the container: %w", err))
	}
	if err := s.waitDatabaseReady(ctx, plan); err != nil {
		return "", s.rollbackImport(ctx, plan, kept, err)
	}
	return kept, nil
}

// recreateDataDir puts an empty directory where the old one was, with the
// permissions and the owner the server expects to find.
func recreateDataDir(dir string, old os.FileInfo, chown func(*os.File, int, int) error) error {
	mode := old.Mode().Perm()
	if err := os.Mkdir(dir, mode); err != nil {
		return err
	}
	d, err := openCreatedDir(dir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	// The umask of this process would otherwise narrow what the server gets.
	if err := d.Chmod(mode); err != nil {
		return err
	}
	uid, gid, ok := dirOwner(old)
	if !ok {
		return nil
	}
	return chown(d, uid, gid)
}

// rollbackImport puts the container back the way it was after a step before the
// import itself failed. kept is the folder the data was moved to, empty when
// the move had not happened yet.
func (s *Service) rollbackImport(ctx context.Context, plan dbImportPlan, kept string, cause error) error {
	if kept != "" {
		// Moving the folders under a server that still runs would leave it
		// writing into the one set aside while the run claims the old data is back.
		if err := s.docker.Stop(ctx, plan.id, dbImportStopTimeout); err != nil {
			return importRollbackFailure(s.toHostPath(plan.dataDir), s.toHostPath(kept), fmt.Errorf("%w; the container could not be stopped for the rollback: %w", cause, err))
		}
		failed := plan.dataDir + ".bombvault-import-failed-" + time.Now().Format(dbImportStamp)
		if err := os.Rename(plan.dataDir, failed); err != nil && !errors.Is(err, os.ErrNotExist) {
			return importRollbackFailure(s.toHostPath(plan.dataDir), s.toHostPath(kept), cause)
		}
		if err := os.Rename(kept, plan.dataDir); err != nil {
			return importRollbackFailure(s.toHostPath(failed), s.toHostPath(kept), cause)
		}
	}
	if err := s.docker.Start(ctx, plan.id); err != nil {
		log.Printf("api: import database dump into %q: the container could not be started again: %v", plan.name, err) //nolint:gosec // G706: name is %q-quoted
	}
	return importPrepareFailure(cause)
}

// waitDatabaseReady polls the engine's own readiness check until the freshly
// initialised server answers.
func (s *Service) waitDatabaseReady(ctx context.Context, plan dbImportPlan) error {
	argv, err := dbdump.ReadyArgv(plan.engine)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(dbImportReadyFor)
	for {
		if s.databaseAnswers(ctx, plan.id, argv) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the database was not ready within %v", dbImportReadyFor)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(dbImportReadyEvery):
		}
	}
}

func (s *Service) databaseAnswers(ctx context.Context, name string, argv []string) bool {
	rctx, cancel := context.WithTimeout(ctx, dbDumpProbeTimeout)
	defer cancel()
	_, exit, err := s.docker.ExecOutput(rctx, name, argv, 1<<10)
	return err == nil && exit == 0
}

// feedDBImport streams the dump into the engine's client and reads the outcome
// off its stderr.
func (s *Service) feedDBImport(ctx context.Context, plan dbImportPlan, keptHere, key string, startedAt int64) (string, error) {
	// The user looks for the kept folder on the host, not in this process.
	kept := s.toHostPath(keptHere)
	argv, err := dbdump.ImportArgv(plan.engine)
	if err != nil {
		return "", importToolFailure(kept, err.Error())
	}
	pr, pw := io.Pipe()
	counted := &countingWriter{w: pw, publish: func(n int64) {
		s.publishDBDumpStage(key, "restore", "dbimport", startedAt, n)
	}}
	read := make(chan error, 1)
	go func() {
		err := s.engine.DumpRaw(ctx, plan.src.repo, plan.dump.ID, dbDumpStdinPath(plan.name), counted, plan.src.mode)
		_ = pw.CloseWithError(err)
		read <- err
	}()

	tail, exit, err := s.docker.ExecStdin(ctx, plan.id, argv, pr, dbImportStderrTail)
	// Closing the read side releases the dump stream when the client gives up
	// before the last byte.
	_ = pr.Close()
	// A client handed a script that just stops can exit 0 on it, so a dump
	// that could not be read fails the import whatever the client said.
	readErr := <-read
	switch {
	case err != nil:
		return "", importToolFailure(kept, importDetail(err.Error()))
	case exit != 0:
		return "", importToolFailure(kept, fmt.Sprintf("exit %d: %s", exit, importToolTail(tail)))
	case readErr != nil:
		return "", importToolFailure(kept, importDetail("read the dump: "+readErr.Error()))
	}
	if n := dbdump.CountImportErrors(plan.engine, tail, plan.pgUser); n > 0 {
		return fmt.Sprintf("%s: %d errors, the previous data folder is kept at %s", store.NoteDBImportErrors, n, kept), nil
	}
	return store.NoteDBImportKeptOld + ": " + kept, nil
}

// finishDBImportRun closes the import's run row: a success carries the note
// that names the kept data folder, a failure the reason it ended with.
func (s *Service) finishDBImportRun(runID, snapshotID, note string, ierr error) {
	if runID == "" {
		return
	}
	status, text := "success", note
	if ierr != nil {
		status, snapshotID, text = "failed", "", truncateRunErr(ierr)
	}
	if err := (runsAdapter{st: s.store, ctx: context.Background()}).Finish(runID, status, backup.Summary{SnapshotID: snapshotID}, text); err != nil {
		log.Printf("api: import database dump: record the run result failed: %v", err)
	}
}
