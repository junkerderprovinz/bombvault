// Package restic provides argv builders and execution helpers for the restic
// backup CLI.  All operations run restic as an external process; no cgo or
// native bindings are used.
package restic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// remoteRepoRe matches a restic remote-backend repo location (vs. a local path).
// rclone covers cloud backends (B2/S3/Drive/…); the others are restic's native
// remote backends. A local repo is a plain filesystem path with no scheme.
var remoteRepoRe = regexp.MustCompile(`^(rclone|sftp|rest|s3|b2|azure|gs|swift):`)

// IsRemoteRepo reports whether loc is a restic remote-backend location (not a
// local filesystem path). Used to skip path-containment resolution and to inject
// the rclone config.
func IsRemoteRepo(loc string) bool { return remoteRepoRe.MatchString(loc) }

// schemeLikeRe matches a leading "word:" scheme. Used to spot a repo location
// that looks like a remote backend but lacks a recognized prefix — e.g. a user
// typing "BackBlaze:bucket" (an rclone remote name) instead of the required
// "rclone:BackBlaze:bucket".
var schemeLikeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// LooksLikeUnprefixedRemote reports whether loc has a scheme-like "word:" prefix
// but is NOT a recognized restic remote — the common mistake of omitting the
// rclone:/s3:/… prefix, which would otherwise be silently treated as a local
// path named after the string.
func LooksLikeUnprefixedRemote(loc string) bool {
	return !remoteRepoRe.MatchString(loc) && schemeLikeRe.MatchString(loc)
}

// Mode describes how the restic repository is secured.
type Mode struct {
	// Encrypted, when true, means the repo uses a password passed via the
	// RESTIC_PASSWORD environment variable (never in argv).
	Encrypted bool
	// Password is the restic repository password.  Only used when Encrypted is
	// true.  Must never appear in argv.
	Password string
	// Env is extra environment passed to the restic process ("KEY=VALUE"), used
	// for backend credentials (S3 AWS_*, REST RESTIC_REST_*). Never in argv.
	Env []string
	// NoLock, when true, adds --no-lock to every lock-taking operation built for
	// this mode (cat config, restore), so the operation NEVER writes a lock file
	// into the repository. It is set only for a foreign read-only session (open
	// another instance's repo without mutating it, #61); the settings-driven
	// backup/restore paths leave it false and lock normally. Snapshot listings are
	// always lock-free regardless (SnapshotsArgs hard-codes --no-lock).
	NoLock bool
}

// Summary holds the fields we extract from restic's --json backup summary line.
type Summary struct {
	SnapshotID   string  `json:"snapshot_id"`
	FilesNew     int     `json:"files_new"`
	FilesChanged int     `json:"files_changed"`
	BytesAdded   float64 `json:"data_added"`
}

// Snapshot holds a subset of the restic snapshot JSON.
type Snapshot struct {
	ID       string   `json:"id"`
	Time     string   `json:"time"`
	Paths    []string `json:"paths"`
	Tags     []string `json:"tags"`
	Hostname string   `json:"hostname"`
}

// StatsResult holds the fields we extract from `restic stats --json`. Which
// fields restic populates depends on the --mode:
//   - mode "raw-data":     TotalSize = physical (deduplicated + compressed)
//     repository size; BlobCount = total stored blobs.
//   - mode "restore-size": TotalSize = logical restore size; FileCount = total
//     files that would be restored.
//
// A field not populated for the chosen mode stays zero.
type StatsResult struct {
	TotalSize int64 `json:"total_size"`
	BlobCount int64 `json:"total_blob_count"`
	FileCount int64 `json:"total_file_count"`
}

// DiffResult holds the summary we extract from `restic diff --json`: its final
// "statistics" object. The per-file "change" lines are ignored — the headline
// counts (files added/removed/changed) and the added/removed byte totals are
// what the UI shows.
type DiffResult struct {
	AddedFiles   int   `json:"addedFiles"`
	RemovedFiles int   `json:"removedFiles"`
	ChangedFiles int   `json:"changedFiles"`
	AddedBytes   int64 `json:"addedBytes"`
	RemovedBytes int64 `json:"removedBytes"`
}

// Restic is the adapter for calling the restic CLI.
type Restic struct {
	// Bin is the path (or name) of the restic binary.  Defaults to "restic".
	Bin string
	// RcloneConfig is the path to the rclone config file. When set AND the file
	// exists, RCLONE_CONFIG is exported for every restic run so rclone-backed
	// (off-site) repos authenticate. Ignored for local repos.
	RcloneConfig string
}

// bin returns the binary to invoke, defaulting to "restic".
func (r Restic) bin() string {
	if r.Bin != "" {
		return r.Bin
	}
	return "restic"
}

// ---- argv builders ---------------------------------------------------------

// insecureFlag is the restic flag for password-less repos (requires restic ≥0.17).
const insecureFlag = "--insecure-no-password"

// backupHost is a fixed restic --host for every backup. restic otherwise defaults
// host to the machine hostname, which inside a container is a random short
// container ID that changes whenever the container is recreated (e.g. on update).
// Pinning it keeps a target's snapshots under one host so retention can group and
// prune them together — see ForgetPolicyArgs.
const backupHost = "bombvault"

// repoFlag returns the common leading args for every restic subcommand.
func repoFlag(repo string) []string {
	return []string{"-r", repo}
}

// Limits caps restic's transfer bandwidth in KiB/s. A zero value omits that cap
// (restic's default = unlimited). Used for off-site replication so backups don't
// saturate the WAN.
type Limits struct {
	UploadKBps   int
	DownloadKBps int
}

// limitFlags returns restic's global --limit-upload / --limit-download flags for
// the non-zero caps in l (KiB/s). These are GLOBAL flags, so they must be placed
// before the subcommand (right after the repo flag). A zero cap is omitted; an
// all-zero Limits yields no flags (unlimited, the default).
func limitFlags(l Limits) []string {
	var args []string
	if l.UploadKBps > 0 {
		args = append(args, "--limit-upload", strconv.Itoa(l.UploadKBps))
	}
	if l.DownloadKBps > 0 {
		args = append(args, "--limit-download", strconv.Itoa(l.DownloadKBps))
	}
	return args
}

// InitArgs returns the argv slice (without the binary name) for `restic init`.
func InitArgs(repo string, m Mode) []string {
	args := append(repoFlag(repo), "init")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	return args
}

// CatConfigArgs returns the argv slice for `restic cat config`. Reading the
// config is the minimal operation that opens AND decrypts a repository, so it is
// the cheapest probe for both "does this repo exist?" and "does mode m open it?"
// (a wrong encryption mode / password fails to decrypt the config).
func CatConfigArgs(repo string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "cat", "config")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	if m.NoLock {
		args = append(args, "--no-lock") // read-only foreign probe must not lock the repo
	}
	return args
}

// BackupArgs returns the argv slice for `restic backup`.
// Tags are added with --tag; each exclude is added with --exclude (restic matches
// a bare name like ".git" by basename at any depth); paths are placed after --
// (arg-injection guard).
func BackupArgs(repo string, paths []string, tags []string, m Mode, excludes ...string) []string {
	args := repoFlag(repo)
	args = append(args, "backup")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--json")
	// Pin a stable host (see backupHost) so a target's snapshots stay in one
	// restic group across container recreations; otherwise retention silently
	// stops collapsing snapshots after an update.
	args = append(args, "--host", backupHost)
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}
	for _, ex := range excludes {
		args = append(args, "--exclude", ex)
	}
	args = append(args, "--")
	args = append(args, paths...)
	return args
}

// DumpZipArgs returns the argv slice for streaming a snapshot subtree as a zip to
// stdout: `restic dump -a zip <snapshot>:<subfolder> /`. Rooting the archive at
// subfolder puts that folder's CONTENTS at the zip root (so a flash zip has
// bzimage/, config/ … at top level, ready for the Unraid USB creator). The
// snapshot selector + path go after -- (arg-injection guard); callers also
// validate the id against the repo's snapshot list.
func DumpZipArgs(repo, snapshotID, subfolder string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "dump", "-a", "zip")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--", snapshotID+":"+subfolder, "/")
	return args
}

// CopyArgs returns the argv for replicating snapshots from srcRepo into destRepo:
// `restic -r <dest> copy --from-repo <src>`. With no ids it copies every source
// snapshot not already in dest (idempotent — restic skips ones already copied).
// For an unencrypted pair both ends use --insecure-no-password / --from-…. Any
// snapshot ids go after -- (arg-injection guard). lim caps the transfer rate via
// restic's global --limit-upload / --limit-download (zero = unlimited), placed
// before the subcommand as restic requires for global flags.
func CopyArgs(destRepo, srcRepo string, snapshotIDs []string, lim Limits, m Mode) []string {
	args := repoFlag(destRepo)
	args = append(args, limitFlags(lim)...)
	args = append(args, "copy", "--from-repo", srcRepo)
	if !m.Encrypted {
		args = append(args, insecureFlag, "--from-insecure-no-password")
	}
	if len(snapshotIDs) > 0 {
		args = append(args, "--")
		args = append(args, snapshotIDs...)
	}
	return args
}

// RestoreSubtreeToArgs returns the argv slice for restoring the subtree at
// subtreePath out of a snapshot INTO target:
// `restic restore <id>:<subtreePath> --target <target>`. The `<id>:<subtreePath>`
// selector roots the restore at subtreePath, so that subtree's CONTENTS land
// directly in target — restic does NOT recreate subtreePath's absolute path
// components under target. (A bare `restore <id> --target X --include /` DOES nest
// the whole absolute /host/user/… path under X, which is issue #62.) subtreePath
// must be one of the snapshot's own backed-up paths; callers take it from the
// SNAPSHOT's Paths (not a recomputed value), so the selector can't miss after a
// HostMountRoot change. The selector goes after -- (arg-injection guard); callers
// also validate the id.
func RestoreSubtreeToArgs(repo, snapshotID, subtreePath, target string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "restore")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	if m.NoLock {
		args = append(args, "--no-lock") // a foreign restore only READS the source repo
	}
	args = append(args, "--json")
	args = append(args, "--target", target)
	args = append(args, "--", snapshotID+":"+subtreePath)
	return args
}

// RestorePathArgs returns the argv slice for restoring a single path back to its
// own location as a subtree: `restic restore <id>:<path> --target <path>`. This
// restores the path's contents to origin WITHOUT restic walking/reconciling the
// shared parent directories (which fails on a populated appdata share). It is the
// RestoreSubtreeToArgs special case where the target IS the subtree path.
func RestorePathArgs(repo, snapshotID, p string, m Mode) []string {
	return RestoreSubtreeToArgs(repo, snapshotID, p, p, m)
}

// FileEntry is one node from `restic ls` (a file or directory in a snapshot).
type FileEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "file" | "dir" | ...
	Size int64  `json:"size"`
}

// LsArgs returns the argv slice for `restic ls --json <snapshotID>` (snapshot id
// after -- as an arg-injection guard; callers also validate it as hex).
func LsArgs(repo, snapshotID string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "ls", "--json")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--", snapshotID)
	return args
}

// RestoreIncludeArgs returns the argv for restoring ONLY includePath out of a
// snapshot, to target. With target "/" the file is written back to its original
// absolute location (in-place file-level restore). The snapshot id is the
// arg-injection-guarded positional.
func RestoreIncludeArgs(repo, snapshotID, includePath, target string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "restore")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	if m.NoLock {
		args = append(args, "--no-lock") // a foreign restore only READS the source repo
	}
	args = append(args, "--json", "--target", target, "--include", includePath, "--", snapshotID)
	return args
}

// RestoreSubtreeIncludeArgs returns the argv for restoring ONLY includePath out
// of a snapshot's subtree INTO target:
// `restic restore <id>:<subtreePath> --target <target> --include <includePath>`.
// The `<id>:<subtreePath>` selector roots the restore at subtreePath, so
// includePath is matched RELATIVE to that subtree and the matched contents land
// directly under target — NOT nested under the absolute /host/user/… path (issue
// #62). This is the selective (pick-some-files) counterpart of RestoreSubtreeToArgs:
// it powers the files-domain "restore selected files into a folder" flow, where
// subtreePath is the SNAPSHOT's own backed-up root (Paths[0]) and includePath is a
// selected path made relative to it (so it always starts with "/"). subtreePath is
// the arg-injection-guarded positional (after --); includePath is a flag value.
func RestoreSubtreeIncludeArgs(repo, snapshotID, subtreePath, includePath, target string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "restore")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	if m.NoLock {
		args = append(args, "--no-lock") // a foreign restore only READS the source repo
	}
	args = append(args, "--json", "--target", target, "--include", includePath, "--", snapshotID+":"+subtreePath)
	return args
}

// CheckArgs returns the argv slice for `restic check` (verifies repository
// structure + metadata integrity).
func CheckArgs(repo string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "check")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	return args
}

// CheckDataArgs returns the argv slice for a restore-readiness "drill":
// `restic check --read-data-subset=<pct>%`. Unlike a plain CheckArgs (metadata
// only), this actually reads back a random subset of the real pack data and
// re-verifies it, proving the backup is genuinely restorable — without needing a
// scratch disk to restore to. subsetPercent is clamped to 1..100.
func CheckDataArgs(repo string, subsetPercent int, m Mode) []string {
	if subsetPercent < 1 {
		subsetPercent = 1
	} else if subsetPercent > 100 {
		subsetPercent = 100
	}
	args := repoFlag(repo)
	args = append(args, "check", fmt.Sprintf("--read-data-subset=%d%%", subsetPercent))
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	return args
}

// SnapshotsArgs returns the argv slice for `restic snapshots --no-lock --json`.
// Listing snapshots is strictly read-only, so it takes --no-lock: it must never
// collide with the exclusive lock a concurrent backup/forget/prune holds (which
// would surface "repository is already locked exclusively" and make the
// latest-backup-times / restore listing read blank). Worst case is a marginally
// stale listing, never corruption, and the writer is never blocked (#57).
func SnapshotsArgs(repo string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "snapshots")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--no-lock", "--json")
	return args
}

// StatsArgs returns the argv slice for `restic stats --json --mode <mode>`.
// mode is restic's --mode value: "raw-data" reports the physical
// (deduplicated + compressed) repository size and blob count; "restore-size"
// reports the logical size and file count of the restored data. The mode is a
// fixed caller-chosen literal, never user input.
func StatsArgs(repo, mode string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "stats", "--json", "--mode", mode)
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	return args
}

// StatsRestoreSizeArgs returns the argv for `restic stats --mode restore-size
// --json <snapshotID>` — the logical restore size + file count of ONE snapshot
// (vs. StatsArgs, which is repo-wide). The snapshot id goes after -- (arg-injection
// guard); callers validate it as hex + scope it to the target first. Used by the
// DR drill to compare restic's own accounting against an on-disk walk of the
// restored sandbox.
func StatsRestoreSizeArgs(repo, snapshotID string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "stats", "--mode", "restore-size", "--json")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--", snapshotID)
	return args
}

// DiffArgs returns the argv slice for `restic diff --json <snap1> <snap2>`. The
// two snapshot ids go after -- (arg-injection guard); callers also validate them
// as hex AND confirm both belong to the target before invoking. restic diff
// streams one JSON object per line (many "change" lines then a final
// "statistics" object), parsed by Diff.
func DiffArgs(repo, snap1, snap2 string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "diff", "--json")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--", snap1, snap2)
	return args
}

// TagAddArgs returns the argv slice for `restic tag --add <tag> <snapID>`. One
// --add per tag is emitted; the snapshot id goes after -- (arg-injection guard),
// and callers validate the id as hex + scope it to the target first. restic tags
// are comma-separated, so callers must reject commas/control chars in tags.
func TagAddArgs(repo, snapID string, tags []string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "tag")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	for _, tag := range tags {
		args = append(args, "--add", tag)
	}
	args = append(args, "--", snapID)
	return args
}

// ForgetArgs returns the argv slice for `restic forget` of specific snapshot IDs,
// optionally pruning the freed data. IDs are placed after -- (arg-injection guard).
func ForgetArgs(repo string, snapshotIDs []string, prune bool, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "forget")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	if prune {
		args = append(args, "--prune")
	}
	args = append(args, "--")
	args = append(args, snapshotIDs...)
	return args
}

// RetentionPolicy is a restic forget keep-policy. A count of 0 omits that
// dimension. When no dimension is set the policy is inert (Any reports false).
type RetentionPolicy struct {
	KeepLast    int
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int
}

// Any reports whether at least one keep dimension is set, i.e. retention is on.
func (p RetentionPolicy) Any() bool {
	return p.KeepLast > 0 || p.KeepDaily > 0 || p.KeepWeekly > 0 || p.KeepMonthly > 0
}

// ForgetPolicyArgs returns the argv for `restic forget --keep-* --prune`. Only
// the set dimensions are emitted. restic's default grouping (host+paths) applies
// the policy per group, so each container/VM/flash target keeps its own history.
func ForgetPolicyArgs(repo string, p RetentionPolicy, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "forget")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	// Group by paths only, NOT restic's default host+paths. A target's snapshots
	// share their paths but may carry different hosts (older snapshots predate the
	// stable --host, or were taken under a now-gone container hostname). Grouping
	// by host would apply the keep-policy separately per container incarnation, so
	// snapshots from before an update would never be pruned — which is why manual
	// and automatic prune appeared to "do nothing". Grouping by paths collapses a
	// target's whole history into one group regardless of host.
	args = append(args, "--group-by", "paths")
	if p.KeepLast > 0 {
		args = append(args, "--keep-last", strconv.Itoa(p.KeepLast))
	}
	if p.KeepDaily > 0 {
		args = append(args, "--keep-daily", strconv.Itoa(p.KeepDaily))
	}
	if p.KeepWeekly > 0 {
		args = append(args, "--keep-weekly", strconv.Itoa(p.KeepWeekly))
	}
	if p.KeepMonthly > 0 {
		args = append(args, "--keep-monthly", strconv.Itoa(p.KeepMonthly))
	}
	args = append(args, "--prune")
	return args
}

// UnlockArgs returns the argv slice for `restic unlock`. removeAll adds
// --remove-all, which clears ALL locks (not just stale ones) — safe because
// BombVault is the sole writer to its repos and serialises its operations.
func UnlockArgs(repo string, removeAll bool, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "unlock")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	if removeAll {
		args = append(args, "--remove-all")
	}
	return args
}

// PruneArgs returns the argv slice for `restic prune` (reclaims space from
// forgotten snapshots).
func PruneArgs(repo string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "prune")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	return args
}

// ---- execution helper ------------------------------------------------------

// authEnv returns the process environment plus restic auth + backend credentials
// for mode m: the repo password (or insecure-no-password), the rclone config when
// the file exists, and any backend creds (S3 AWS_*, REST RESTIC_REST_*) in m.Env.
// Credentials travel via env, never argv/logs. Shared by run() and DumpZip.
func (r Restic) authEnv(m Mode) []string {
	env := os.Environ()
	if m.Encrypted {
		env = append(env, "RESTIC_PASSWORD="+m.Password)
	} else {
		env = append(env, "RESTIC_INSECURE_NO_PASSWORD=true")
	}
	if r.RcloneConfig != "" {
		if _, statErr := os.Stat(r.RcloneConfig); statErr == nil {
			env = append(env, "RCLONE_CONFIG="+r.RcloneConfig)
		}
	}
	env = append(env, m.Env...)
	return env
}

// run executes restic with the given args and mode.  The password is injected
// via env (RESTIC_PASSWORD or RESTIC_INSECURE_NO_PASSWORD=true), never via
// argv.  On failure, full stderr is logged server-side but only a scrubbed
// error is returned to the caller.
func (r Restic) run(ctx context.Context, args []string, m Mode) ([]byte, error) {
	cmd := exec.CommandContext(ctx, r.bin(), args...) //nolint:gosec // G204: argv is constructed by typed builders in this package; no user input reaches here
	env := r.authEnv(m)
	var out []byte
	var err error
	// When a progress sink is present (backup/restore), stream stdout so each
	// --json "status" line's percentage reaches the UI live; otherwise capture
	// the full output in one shot (the default for snapshots/ls/check/forget).
	if sink := progress.SinkFrom(ctx); sink != nil {
		// restic emits periodic --json "status" progress only when stdout is a
		// TTY or RESTIC_PROGRESS_FPS is set. Our stdout is a pipe, so without this
		// restic prints only the final summary and the bar would never fill.
		cmd.Env = append(env, "RESTIC_PROGRESS_FPS=3")
		out, err = runStreaming(cmd, args, sink)
	} else {
		cmd.Env = env
		out, err = runBuffered(cmd, args)
	}
	return out, ctxCancelErr(ctx, args, err)
}

// ctxCancelErr re-wraps a command failure that was actually caused by ctx being
// cancelled or hitting its deadline. exec.CommandContext kills the child on
// cancel, which cmd.Run/Wait reports as a generic *ExitError ("signal: killed")
// with NO context error in its chain; runError would then scrub that into a
// plain "restic <sub> failed", and every finish site — which tells a user
// cancel from a real failure via errors.Is(err, context.Canceled) — would
// misrecord the cancel as "failed" (which is exactly what defeated the cancel
// feature in production). When ctx is done we therefore return an error WRAPPING
// ctx.Err() so errors.Is holds. A deadline stays context.DeadlineExceeded (NOT
// Canceled), so a restore wedged past its 48h cap is still recorded "failed" —
// correct. When ctx is not done the original (scrubbed) error is returned as-is.
func ctxCancelErr(ctx context.Context, args []string, err error) error {
	if err != nil && ctx.Err() != nil {
		return fmt.Errorf("restic %s cancelled: %w", subcommand(args), ctx.Err())
	}
	return err
}

// runBuffered runs restic capturing all stdout into a buffer.
func runBuffered(cmd *exec.Cmd, args []string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, runError(args, stderr.String())
	}
	return stdout.Bytes(), nil
}

// runToWriter runs restic streaming stdout straight into w (for `dump` zip
// downloads), capturing stderr so a scrubbed reason is returned on failure.
func runToWriter(cmd *exec.Cmd, args []string, w io.Writer) error {
	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return runError(args, stderr.String())
	}
	return nil
}

// runStreaming runs restic and scans its --json stdout line by line, forwarding
// each "status" line's percent_done to the sink while still accumulating the
// full output so a trailing summary line can be parsed afterwards.
func runStreaming(cmd *exec.Cmd, args []string, sink progress.Sink) ([]byte, error) {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("restic %s: stdout pipe: %w", subcommand(args), err)
	}
	if err := cmd.Start(); err != nil {
		return nil, runError(args, stderr.String())
	}

	var out bytes.Buffer
	sc := bufio.NewScanner(stdout)
	// Status/summary lines are normally small, but a status line embeds the
	// current file path, which can be very long. Allow up to 16 MiB so a giant
	// path can't overflow the scanner and make a successful backup look failed
	// (the trailing summary line would be lost after a token-too-long abort).
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
	for sc.Scan() {
		line := sc.Bytes()
		out.Write(line)
		out.WriteByte('\n')
		if pct, ok := statusPercent(line); ok {
			sink(pct)
		}
	}
	// A scanner error (e.g. an over-long line) is logged, not fatal: still wait
	// for the process and let the caller parse whatever output was captured.
	if scErr := sc.Err(); scErr != nil {
		log.Printf("restic %s: stdout scan: %v", subcommand(args), scErr)
		// Drain the rest of stdout so restic doesn't block writing to a full pipe,
		// which would hang cmd.Wait below.
		_, _ = io.Copy(io.Discard, stdout)
	}
	if err := cmd.Wait(); err != nil {
		return nil, runError(args, stderr.String())
	}
	return out.Bytes(), nil
}

// ErrRestoreMetadataOnly tags a restore failure whose ONLY errors were per-file
// ownership/permission/metadata errors on the RESTORE TARGET: every file's data
// was extracted, but restic could not set its ownership/permissions/timestamps/
// xattrs. That is the norm on an Unraid /mnt/user (FUSE shfs) share, which refuses
// chown/xattr even for root — the same reason the flash domain streams a zip
// instead of restoring to disk (see DumpZip). The files restore treats it as
// success-with-warning (RestoreMetadataWarning) rather than a hard failure;
// genuine failures (missing snapshot, no space, unreachable repo, corruption)
// never carry it. Detect it with errors.Is(err, ErrRestoreMetadataOnly).
var ErrRestoreMetadataOnly = errors.New("restore completed but file ownership/metadata could not be set on the target")

// RestoreMetadataWarning is the user-facing reason recorded on a restore that
// finished success-with-warning because the target (an Unraid /mnt/user FUSE
// share) refused the ownership/metadata restore. All file data is present.
const RestoreMetadataWarning = "restored; could not set original ownership on the share, which is normal on /mnt/user"

// metadataOnlyRestoreErr wraps a restore failure as metadata-only. It keeps the
// ordinary scrubbed failure message (so callers that merely DISPLAY the error —
// e.g. the container to-path restore — behave exactly as before) while satisfying
// errors.Is(err, ErrRestoreMetadataOnly) for the files restore.
type metadataOnlyRestoreErr struct{ msg string }

func (e *metadataOnlyRestoreErr) Error() string { return e.msg }

func (e *metadataOnlyRestoreErr) Is(target error) bool { return target == ErrRestoreMetadataOnly }

// runError logs the full stderr server-side and returns a concise, path-scrubbed
// reason to the caller so the UI shows WHY restic failed (e.g. "repository is
// already locked") instead of a generic message.
func runError(args []string, stderr string) error {
	sub := subcommand(args)
	log.Printf("restic %s stderr: %s", sub, stderr)
	msg := fmt.Sprintf("restic %s failed", sub)
	if reason := lastReason(stderr); reason != "" {
		msg = fmt.Sprintf("restic %s failed: %s", sub, reason)
	}
	// A restore whose ONLY errors were per-file ownership/metadata permission
	// errors on the target (all data extracted — normal on an Unraid /mnt/user FUSE
	// share) is tagged ErrRestoreMetadataOnly so the files restore can finish
	// success-with-warning. The message text is unchanged, so every other restore
	// caller (container to-path, DR drill, config self-restore) still records a
	// plain failure.
	if sub == "restore" && isMetadataOnlyRestoreFailure(stderr) {
		return &metadataOnlyRestoreErr{msg: msg}
	}
	return errors.New(msg)
}

// isMetadataOnlyRestoreFailure reports whether a failed restore's stderr shows
// ONLY per-file ownership/permission/metadata errors on the target — i.e. restic
// extracted every file's data but could not set its ownership/permissions on the
// share (the norm on an Unraid /mnt/user FUSE mount). It is deliberately
// conservative: it requires at least one such per-file permission error and treats
// ANY other content — a Fatal that is not restic's error-count tally, a data/
// space/I-O/read-only/connection/repo error, restic's corruption trailer, or a
// per-file error that is NOT a permission error — as a genuine failure (returns
// false), so a real problem is never masked as success.
func isMetadataOnlyRestoreFailure(stderr string) bool {
	sawPermErr := false
	for _, raw := range strings.Split(stderr, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := strings.ToLower(line)
		switch {
		case isPerFilePermError(low):
			sawPermErr = true
		case errorCountRe.MatchString(low):
			// restic's "There were N errors" tally (often prefixed "Fatal:") — the
			// expected companion to the per-file errors; it names no cause of its own.
		default:
			return false // anything else means we cannot prove it was metadata-only
		}
	}
	return sawPermErr
}

// isPerFilePermError reports whether a lower-cased stderr line is one of restic's
// per-file "ignoring error for <path>: … operation not permitted / permission
// denied" lines — the shape restic emits when it cannot set a restored file's
// ownership/permissions/metadata on the target (observed as thousands of "Lchown:
// operation not permitted" lines restoring onto /mnt/user).
func isPerFilePermError(low string) bool {
	if !strings.Contains(low, "ignoring error") {
		return false
	}
	return strings.Contains(low, "operation not permitted") ||
		strings.Contains(low, "permission denied")
}

// statusPercent extracts the 0..100 completion percentage from a restic --json
// "status" line. Returns ok=false for any other line (summary, errors, non-JSON).
func statusPercent(line []byte) (float64, bool) {
	var s struct {
		MessageType string  `json:"message_type"`
		PercentDone float64 `json:"percent_done"`
	}
	if json.Unmarshal(line, &s) != nil || s.MessageType != "status" {
		return 0, false
	}
	pct := s.PercentDone * 100
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100 // restic can briefly report >100% when total_bytes is underestimated mid-scan
	}
	return pct, true
}

// reasonPathRe matches absolute-path-like tokens so they can be stripped from a
// surfaced restic error (defense-in-depth: repo/appdata paths must not leak).
var reasonPathRe = regexp.MustCompile(`(/[^\s:"']+)+`)

// lastReason returns the most informative line of stderr, with absolute paths
// scrubbed and the length capped — a concise failure cause for the UI.
//
// restic often ends its output with a generic boilerplate trailer ("Corrupted
// blobs are either caused by hardware issues or software bugs. Please open an
// issue …"), which is useless to the user. We prefer a line that names the
// actual cause (a "Fatal:" line, "unable to save", "hash mismatch", etc.) and
// only fall back to the literal last line when nothing better is present.
func lastReason(stderr string) string {
	var lines []string
	for _, line := range strings.Split(stderr, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			lines = append(lines, s)
		}
	}
	if len(lines) == 0 {
		return ""
	}

	reason := lines[len(lines)-1] // default: the literal last line
	// Prefer a line that names the real cause (scan from the end).
	for i := len(lines) - 1; i >= 0; i-- {
		if isInformativeReason(lines[i]) {
			reason = lines[i]
			break
		}
	}
	// If we still landed on boilerplate, take the last non-boilerplate line.
	if isBoilerplateReason(reason) {
		for i := len(lines) - 1; i >= 0; i-- {
			if !isBoilerplateReason(lines[i]) {
				reason = lines[i]
				break
			}
		}
	}

	// restic's restore summary "There were N errors" names a count but not a
	// cause. When that is all we have, append the first concrete per-item error
	// (e.g. an "ignoring error for <file>: …" line) so the UI shows WHY a restore
	// partially failed instead of just a number.
	if errorCountRe.MatchString(reason) {
		if sample := firstItemError(lines); sample != "" {
			reason += " (e.g. " + sample + ")"
		}
	}

	reason = reasonPathRe.ReplaceAllString(reason, "[path]")
	if len(reason) > 200 {
		reason = reason[:200]
	}
	return reason
}

// errorCountRe matches restic's count-only restore summary ("There were N
// errors"), which says how many items failed but not why.
var errorCountRe = regexp.MustCompile(`(?i)there were \d+ errors`)

// firstItemError returns the first stderr line that names a concrete per-item
// failure (the kind restic prints once per file during a partial restore), so it
// can be surfaced alongside the count-only summary.
func firstItemError(lines []string) string {
	for _, l := range lines {
		ll := strings.ToLower(l)
		if strings.Contains(ll, "ignoring error") ||
			strings.Contains(ll, "operation not permitted") ||
			strings.Contains(ll, "permission denied") {
			return l
		}
	}
	return ""
}

// isInformativeReason reports whether a stderr line names an actual failure
// cause (worth surfacing over restic's generic trailer).
func isInformativeReason(line string) bool {
	l := strings.ToLower(line)
	return strings.HasPrefix(l, "fatal:") ||
		strings.HasPrefix(l, "error") ||
		strings.Contains(l, "unable to") ||
		strings.Contains(l, "data corruption") ||
		strings.Contains(l, "hash mismatch")
}

// isBoilerplateReason reports whether a stderr line is restic's generic
// "open an issue" trailer that carries no actionable detail.
func isBoilerplateReason(line string) bool {
	l := strings.ToLower(line)
	return strings.Contains(l, "please open an issue") ||
		strings.Contains(l, "corrupted blobs are either") ||
		strings.Contains(l, "for further troubleshooting")
}

// subcommand extracts the subcommand name from an args slice for use in error
// messages (first arg that is not a flag or -r value).
func subcommand(args []string) string {
	skip := false
	for _, a := range args {
		if skip {
			skip = false
			continue
		}
		if a == "-r" {
			skip = true
			continue
		}
		if len(a) > 0 && a[0] != '-' {
			return a
		}
	}
	return "unknown"
}

// ---- high-level operations -------------------------------------------------

// Init initialises a new restic repository.
func (r Restic) Init(ctx context.Context, repo string, m Mode) error {
	_, err := r.run(ctx, InitArgs(repo, m), m)
	return err
}

// RepoOpens reports whether the repository at repo can be opened (and its config
// decrypted) using mode m. It runs `restic cat config` — the cheapest read that
// exercises the repo key — and treats any error (repo missing, wrong encryption
// mode/password, backend unreachable) as "does not open". It needs no lock, so a
// concurrently-locked repo still reports true. Used to reconcile the configured
// encryption mode against the mode the repo was actually created with.
func (r Restic) RepoOpens(ctx context.Context, repo string, m Mode) bool {
	_, err := r.run(ctx, CatConfigArgs(repo, m), m)
	return err == nil
}

// Backup backs up paths into the repo, tagging each snapshot with tags, and
// returns the parsed backup summary.
func (r Restic) Backup(ctx context.Context, repo string, paths []string, tags []string, m Mode, excludes ...string) (Summary, error) {
	out, err := r.run(ctx, BackupArgs(repo, paths, tags, m, excludes...), m)
	if err != nil {
		return Summary{}, err
	}
	return ParseBackupSummary(out)
}

// DumpZip streams the snapshot subtree rooted at subfolder as a zip into w
// (restic dump -a zip). Used for flash restore: a zip carries no filesystem
// metadata, so it sidesteps the per-file ownership/permission errors a to-disk
// restore hits on an Unraid /mnt/user (FUSE) share, and the file drops straight
// into the Unraid USB creator.
func (r Restic) DumpZip(ctx context.Context, repo, snapshotID, subfolder string, w io.Writer, m Mode) error {
	args := DumpZipArgs(repo, snapshotID, subfolder, m)
	cmd := exec.CommandContext(ctx, r.bin(), args...) //nolint:gosec // G204: argv from typed builders; snapshot id validated against the repo by the caller
	cmd.Env = r.authEnv(m)
	// A client disconnect / user cancel of the download cancels ctx and kills the
	// child; re-wrap so DownloadFlashZip's errors.Is(err, context.Canceled) holds
	// and records "cancelled" instead of "failed" (same root cause as run()).
	return ctxCancelErr(ctx, args, runToWriter(cmd, args, w))
}

// Copy replicates snapshots from srcRepo into destRepo (restic copy) for off-site
// backup. When encrypted both repos share the APP_KEY-derived password: the dest
// via RESTIC_PASSWORD (authEnv), the source via RESTIC_FROM_PASSWORD — never argv.
// destRepo must already exist (the caller EnsureRepo's it first). lim caps the
// transfer bandwidth (zero = unlimited) so replication doesn't saturate the WAN.
func (r Restic) Copy(ctx context.Context, destRepo, srcRepo string, snapshotIDs []string, lim Limits, m Mode) error {
	args := CopyArgs(destRepo, srcRepo, snapshotIDs, lim, m)
	cmd := exec.CommandContext(ctx, r.bin(), args...) //nolint:gosec // G204: argv from typed builders; repos are operator-configured
	env := r.authEnv(m)
	if m.Encrypted {
		env = append(env, "RESTIC_FROM_PASSWORD="+m.Password)
	}
	cmd.Env = env
	_, err := runBuffered(cmd, args)
	return err
}

// RestorePath restores a single backed-up path (p) back to its own location as a
// subtree, so restic never reconciles the shared parent directory.
func (r Restic) RestorePath(ctx context.Context, repo, snapshotID, p string, m Mode) error {
	_, err := r.run(ctx, RestorePathArgs(repo, snapshotID, p, m), m)
	return err
}

// RestoreSubtreeTo restores the subtree at subtreePath from a snapshot INTO
// target, so target receives that subtree's contents directly (no absolute-path
// nesting — see RestoreSubtreeToArgs). subtreePath must be one of the snapshot's
// own backed-up paths (Paths[0]).
func (r Restic) RestoreSubtreeTo(ctx context.Context, repo, snapshotID, subtreePath, target string, m Mode) error {
	_, err := r.run(ctx, RestoreSubtreeToArgs(repo, snapshotID, subtreePath, target, m), m)
	return err
}

// RestoreSubtreeInclude restores ONLY includePath (relative to subtreePath) from a
// snapshot INTO target, rooting the restore at the snapshot's subtree so the
// matched contents land directly under target with no absolute-path nesting (see
// RestoreSubtreeIncludeArgs). Used by the files-domain selective restore: each
// selected path becomes an includePath relative to the set's backed-up root.
func (r Restic) RestoreSubtreeInclude(ctx context.Context, repo, snapshotID, subtreePath, includePath, target string, m Mode) error {
	_, err := r.run(ctx, RestoreSubtreeIncludeArgs(repo, snapshotID, subtreePath, includePath, target, m), m)
	return err
}

// Snapshots lists snapshots in the repository and returns them as a slice.
func (r Restic) Snapshots(ctx context.Context, repo string, m Mode) ([]Snapshot, error) {
	out, err := r.run(ctx, SnapshotsArgs(repo, m), m)
	if err != nil {
		return nil, err
	}
	var snaps []Snapshot
	if err := json.Unmarshal(out, &snaps); err != nil {
		return nil, fmt.Errorf("restic snapshots: parse JSON: %w", err)
	}
	return snaps, nil
}

// Stats returns repository statistics for the chosen --mode (see StatsArgs).
// restic stats --json emits a single JSON object, so it is parsed in one shot
// (the buffered run, like Snapshots — not the streaming progress path).
func (r Restic) Stats(ctx context.Context, repo, mode string, m Mode) (StatsResult, error) {
	out, err := r.run(ctx, StatsArgs(repo, mode, m), m)
	if err != nil {
		return StatsResult{}, err
	}
	var res StatsResult
	if err := json.Unmarshal(out, &res); err != nil {
		return StatsResult{}, fmt.Errorf("restic stats: parse JSON: %w", err)
	}
	return res, nil
}

// StatsRestoreSize returns the logical restore size (total bytes) and file count
// of ONE snapshot via `restic stats --mode restore-size --json <snap>`. It is the
// per-snapshot counterpart of Stats (which is repo-wide): the DR drill compares
// these against an on-disk walk of the restored sandbox to prove the restore was
// complete. restic stats --json emits a single JSON object (buffered parse).
func (r Restic) StatsRestoreSize(ctx context.Context, repo, snapshotID string, m Mode) (int, int64, error) {
	out, err := r.run(ctx, StatsRestoreSizeArgs(repo, snapshotID, m), m)
	if err != nil {
		return 0, 0, err
	}
	var res StatsResult
	if err := json.Unmarshal(out, &res); err != nil {
		return 0, 0, fmt.Errorf("restic stats: parse JSON: %w", err)
	}
	return int(res.FileCount), res.TotalSize, nil
}

// Diff compares two snapshots (`restic diff --json snap1 snap2`) and returns the
// parsed summary. restic streams one JSON object per line: many "change" lines
// then a final "statistics" object — we keep only the statistics, mapping its
// added/removed file counts + byte totals and its changed_files count into a
// DiffResult. Buffered (like Snapshots/Stats), not the streaming progress path.
func (r Restic) Diff(ctx context.Context, repo, snap1, snap2 string, m Mode) (DiffResult, error) {
	out, err := r.run(ctx, DiffArgs(repo, snap1, snap2, m), m)
	if err != nil {
		return DiffResult{}, err
	}
	return parseDiffStatistics(out)
}

// TagAdd adds tags to a snapshot (`restic tag --add`). An empty tag list is a
// no-op (nothing to add).
func (r Restic) TagAdd(ctx context.Context, repo, snapID string, tags []string, m Mode) error {
	if len(tags) == 0 {
		return nil
	}
	_, err := r.run(ctx, TagAddArgs(repo, snapID, tags, m), m)
	return err
}

// Ls lists the files/dirs in a snapshot. restic ls --json emits one JSON object
// per line: a leading snapshot-metadata object then one "node" per path. We keep
// only nodes that carry a path.
func (r Restic) Ls(ctx context.Context, repo, snapshotID string, m Mode) ([]FileEntry, error) {
	out, err := r.run(ctx, LsArgs(repo, snapshotID, m), m)
	if err != nil {
		return nil, err
	}
	var entries []FileEntry
	for _, line := range bytes.Split(out, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var e FileEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip non-node lines
		}
		if e.Path != "" && e.Path != "/" {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// RestoreInclude restores ONLY includePath from a snapshot to target. With
// target "/" this writes the file back to its original absolute location.
func (r Restic) RestoreInclude(ctx context.Context, repo, snapshotID, includePath, target string, m Mode) error {
	_, err := r.run(ctx, RestoreIncludeArgs(repo, snapshotID, includePath, target, m), m)
	return err
}

// Forget removes the given snapshots from the repo, optionally pruning the freed
// data. A nil/empty ID list is a no-op (nothing to forget).
func (r Restic) Forget(ctx context.Context, repo string, snapshotIDs []string, prune bool, m Mode) error {
	if len(snapshotIDs) == 0 {
		return nil
	}
	_, err := r.run(ctx, ForgetArgs(repo, snapshotIDs, prune, m), m)
	return err
}

// Check verifies the repository's structure and metadata integrity
// (`restic check`). It returns nil when the repo is healthy, or a scrubbed error
// describing the problem.
func (r Restic) Check(ctx context.Context, repo string, m Mode) error {
	_, err := r.run(ctx, CheckArgs(repo, m), m)
	return err
}

// CheckData runs a restore-readiness drill: `restic check
// --read-data-subset=<pct>%`, which reads back and re-verifies a random subset of
// the real pack data (not just metadata), proving the backup is restorable. It
// returns nil when the checked data is intact, or a scrubbed error describing the
// corruption. subsetPercent is clamped to 1..100 by CheckDataArgs.
func (r Restic) CheckData(ctx context.Context, repo string, subsetPercent int, m Mode) error {
	_, err := r.run(ctx, CheckDataArgs(repo, subsetPercent, m), m)
	return err
}

// ForgetPolicy applies a keep-policy to the repo and prunes the freed data. An
// inert policy (no keep dimension set) is a no-op, so retention stays off until
// the user configures it.
func (r Restic) ForgetPolicy(ctx context.Context, repo string, p RetentionPolicy, m Mode) error {
	if !p.Any() {
		return nil
	}
	_, err := r.run(ctx, ForgetPolicyArgs(repo, p, m), m)
	return err
}

// Unlock removes locks from the repo (`restic unlock`). removeAll clears ALL
// locks, not just stale ones — safe because BombVault is the sole writer.
func (r Restic) Unlock(ctx context.Context, repo string, removeAll bool, m Mode) error {
	_, err := r.run(ctx, UnlockArgs(repo, removeAll, m), m)
	return err
}

// Prune reclaims repository space freed by forgotten snapshots (`restic prune`).
func (r Restic) Prune(ctx context.Context, repo string, m Mode) error {
	_, err := r.run(ctx, PruneArgs(repo, m), m)
	return err
}

// ---- JSON parsing ----------------------------------------------------------

// backupJSONLine is used to detect the summary line in restic --json backup
// output (which may also contain status lines with different message_type values).
type backupJSONLine struct {
	MessageType string `json:"message_type"`
	Summary
}

// diffStat mirrors restic's DiffStat object (the "added"/"removed" sub-objects
// of a diff statistics line). Field names match restic cmd/restic/cmd_diff.go
// exactly. We use only Files + Bytes; the rest are kept for completeness.
type diffStat struct {
	Files     int   `json:"files"`
	Dirs      int   `json:"dirs"`
	Others    int   `json:"others"`
	DataBlobs int   `json:"data_blobs"`
	TreeBlobs int   `json:"tree_blobs"`
	Bytes     int64 `json:"bytes"`
}

// diffStatistics mirrors restic's diff statistics JSON line
// (message_type == "statistics"), per restic cmd/restic/cmd_diff.go.
type diffStatistics struct {
	MessageType  string   `json:"message_type"`
	ChangedFiles int      `json:"changed_files"`
	Added        diffStat `json:"added"`
	Removed      diffStat `json:"removed"`
}

// parseDiffStatistics scans lines of restic --json diff output for the final
// "statistics" object and maps it into a DiffResult. Per-file "change" lines are
// ignored — only the summary statistics are surfaced.
func parseDiffStatistics(data []byte) (DiffResult, error) {
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var s diffStatistics
		if err := json.Unmarshal(line, &s); err != nil {
			continue // skip non-JSON / per-file change lines
		}
		if s.MessageType == "statistics" {
			return DiffResult{
				AddedFiles:   s.Added.Files,
				RemovedFiles: s.Removed.Files,
				ChangedFiles: s.ChangedFiles,
				AddedBytes:   int64(s.Added.Bytes),
				RemovedBytes: int64(s.Removed.Bytes),
			}, nil
		}
	}
	return DiffResult{}, fmt.Errorf("restic diff: no statistics line in output")
}

// ParseBackupSummary scans lines of restic --json backup output for the
// summary line (message_type == "summary") and returns the parsed Summary.
func ParseBackupSummary(data []byte) (Summary, error) {
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var l backupJSONLine
		if err := json.Unmarshal(line, &l); err != nil {
			continue // skip non-JSON lines
		}
		if l.MessageType == "summary" {
			return l.Summary, nil
		}
	}
	return Summary{}, fmt.Errorf("restic backup: no summary line in output")
}
