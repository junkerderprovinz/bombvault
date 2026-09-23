package backup

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const kindDBDump = "dbdump"

// DBDumpPlan is the service's decision to dump this container's database.
type DBDumpPlan struct {
	// Engine is "postgres", "mysql" or "mariadb".
	Engine string
	// Identity is the snapshot's identity tag, "dbdump:<ref>".
	Identity string
	// StdinPath is the path the stream is stored under, "/dbdump/<ref>.sql".
	StdinPath string
	// Image is the container's Config.Image at backup time, empty when it is not
	// a valid tag value.
	Image string
	// MaxRuntime bounds the dump inside the container.
	MaxRuntime time.Duration
}

// DBDumpRequest is one dump of one container into one repository.
type DBDumpRequest struct {
	Repo string
	Plan DBDumpPlan
	Tags []string
}

// DBDumpResult describes the snapshot a finished dump left behind. Note is a
// success note for the run, empty when the dump has nothing to add.
type DBDumpResult struct {
	SnapshotID string
	Bytes      int64
	Note       string
}

// DBDumpError is the failure a DBDumper reports: Reason is the run's error
// text, one of the store's dump reasons with an optional scrubbed detail after
// a ": ". SnapshotID is set only for store.ReasonDBDumpLeftover, where restic
// wrote a snapshot that could not be removed again.
type DBDumpError struct {
	Reason     string
	SnapshotID string
}

func (e *DBDumpError) Error() string { return e.Reason }

// DBDumper streams one dump into the repository as its own snapshot.
type DBDumper interface {
	Dump(ctx context.Context, req DBDumpRequest) (DBDumpResult, error)
}

// DBDumpOutcome is what the dump run was recorded as, handed back to the caller
// so it can word a notification once the backup's own result is known.
type DBDumpOutcome struct {
	RunID      string
	Status     string
	Reason     string
	SnapshotID string
	Bytes      int64
}

// dbDumpTags pairs the dump snapshot with the backup run it belongs to, so a
// restore can find the dump taken minutes before a volume snapshot. The former
// names go on it for the same reason the volume snapshot carries them: a
// container whose data sits under its compose working directory leaves dumps
// and nothing else, and discovery has to read the link out of them.
func dbDumpTags(p DBDumpPlan, backupRunID string, formerNames []string) []string {
	tags := []string{p.Identity, "p1", "dbengine:" + p.Engine, "bvrun:" + backupRunID}
	if p.Image != "" {
		tags = append(tags, "dbimage:"+p.Image)
	}
	return withFormerNames(tags, formerNames)
}

// runDBDump takes the dump and records it as a run of its own, so a failed dump
// is visible without failing the backup around it. The note it returns goes on
// the backup run and is empty unless the dump was skipped: a dump snapshot with
// no run row would be invisible everywhere the dumps are listed.
func runDBDump(ctx context.Context, d BackupDeps, backupRunID string) (backupNote string) {
	runID, err := d.Runs.Start(d.TargetID, kindDBDump)
	if err != nil {
		log.Printf("backup: record database dump run for %q failed (dump skipped): %v", d.ContainerRef, err)
		return store.NoteDBDumpNotRecorded
	}

	res, dumpErr := d.DBDumper.Dump(ctx, DBDumpRequest{
		Repo: d.RepoPath,
		Plan: *d.DBDump,
		Tags: dbDumpTags(*d.DBDump, backupRunID, d.FormerNames),
	})

	out := DBDumpOutcome{
		RunID:      runID,
		Status:     statusSuccess,
		Reason:     res.Note,
		SnapshotID: res.SnapshotID,
		Bytes:      res.Bytes,
	}
	if dumpErr != nil {
		out = DBDumpOutcome{RunID: runID, Status: statusFailed}
		var dumpFail *DBDumpError
		if errors.As(dumpErr, &dumpFail) {
			out.Reason = dumpFail.Reason
			out.SnapshotID = dumpFail.SnapshotID
		} else {
			out.Reason = truncateErr(dumpErr)
		}
	}

	if err := d.Runs.Finish(runID, out.Status, out.SnapshotID, out.Bytes, out.Reason); err != nil {
		log.Printf("backup: record database dump result for %q failed: %v", d.ContainerRef, err)
	}
	if d.OnDBDumpDone != nil {
		d.OnDBDumpDone(out)
	}
	return ""
}
