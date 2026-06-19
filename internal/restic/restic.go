// Package restic provides argv builders and execution helpers for the restic
// backup CLI.  All operations run restic as an external process; no cgo or
// native bindings are used.
package restic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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

// Mode describes how the restic repository is secured.
type Mode struct {
	// Encrypted, when true, means the repo uses a password passed via the
	// RESTIC_PASSWORD environment variable (never in argv).
	Encrypted bool
	// Password is the restic repository password.  Only used when Encrypted is
	// true.  Must never appear in argv.
	Password string
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

// repoFlag returns the common leading args for every restic subcommand.
func repoFlag(repo string) []string {
	return []string{"-r", repo}
}

// InitArgs returns the argv slice (without the binary name) for `restic init`.
func InitArgs(repo string, m Mode) []string {
	args := append(repoFlag(repo), "init")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	return args
}

// BackupArgs returns the argv slice for `restic backup`.
// Tags are added with --tag; paths are placed after -- (arg-injection guard).
func BackupArgs(repo string, paths []string, tags []string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "backup")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--json")
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}
	args = append(args, "--")
	args = append(args, paths...)
	return args
}

// RestoreArgs returns the argv slice for `restic restore`.
// snapshotID is placed after -- (arg-injection guard).
func RestoreArgs(repo string, snapshotID string, target string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "restore")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--json")
	args = append(args, "--target", target)
	args = append(args, "--", snapshotID)
	return args
}

// RestorePathArgs returns the argv slice for restoring a single path back to its
// own location as a subtree: `restic restore <id>:<path> --target <path>`. This
// restores the path's contents to origin WITHOUT restic walking/reconciling the
// shared parent directories (which fails on a populated appdata share). The
// snapshot selector goes after -- (arg-injection guard).
func RestorePathArgs(repo, snapshotID, p string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "restore")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--json")
	args = append(args, "--target", p)
	args = append(args, "--", snapshotID+":"+p)
	return args
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
	args = append(args, "--json", "--target", target, "--include", includePath, "--", snapshotID)
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

// SnapshotsArgs returns the argv slice for `restic snapshots --json`.
func SnapshotsArgs(repo string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "snapshots")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--json")
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

// ---- execution helper ------------------------------------------------------

// run executes restic with the given args and mode.  The password is injected
// via env (RESTIC_PASSWORD or RESTIC_INSECURE_NO_PASSWORD=true), never via
// argv.  On failure, full stderr is logged server-side but only a scrubbed
// error is returned to the caller.
func (r Restic) run(ctx context.Context, args []string, m Mode) ([]byte, error) {
	cmd := exec.CommandContext(ctx, r.bin(), args...) //nolint:gosec // G204: argv is constructed by typed builders in this package; no user input reaches here

	// Inject auth env — password never in argv.
	env := cmd.Environ()
	if m.Encrypted {
		env = append(env, "RESTIC_PASSWORD="+m.Password)
	} else {
		env = append(env, "RESTIC_INSECURE_NO_PASSWORD=true")
	}
	// Point restic→rclone at the managed config for off-site repos (only when it
	// exists; harmless for local repos).
	if r.RcloneConfig != "" {
		if _, statErr := os.Stat(r.RcloneConfig); statErr == nil {
			env = append(env, "RCLONE_CONFIG="+r.RcloneConfig)
		}
	}
	// When a progress sink is present (backup/restore), stream stdout so each
	// --json "status" line's percentage reaches the UI live; otherwise capture
	// the full output in one shot (the default for snapshots/ls/check/forget).
	if sink := progress.SinkFrom(ctx); sink != nil {
		// restic emits periodic --json "status" progress only when stdout is a
		// TTY or RESTIC_PROGRESS_FPS is set. Our stdout is a pipe, so without this
		// restic prints only the final summary and the bar would never fill.
		cmd.Env = append(env, "RESTIC_PROGRESS_FPS=3")
		return runStreaming(cmd, args, sink)
	}
	cmd.Env = env
	return runBuffered(cmd, args)
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
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20) // status/summary lines are small; 1 MiB is ample
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

// runError logs the full stderr server-side and returns a concise, path-scrubbed
// reason to the caller so the UI shows WHY restic failed (e.g. "repository is
// already locked") instead of a generic message.
func runError(args []string, stderr string) error {
	sub := subcommand(args)
	log.Printf("restic %s stderr: %s", sub, stderr)
	if reason := lastReason(stderr); reason != "" {
		return fmt.Errorf("restic %s failed: %s", sub, reason)
	}
	return fmt.Errorf("restic %s failed", sub)
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

	reason = reasonPathRe.ReplaceAllString(reason, "[path]")
	if len(reason) > 200 {
		reason = reason[:200]
	}
	return reason
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

// Backup backs up paths into the repo, tagging each snapshot with tags, and
// returns the parsed backup summary.
func (r Restic) Backup(ctx context.Context, repo string, paths []string, tags []string, m Mode) (Summary, error) {
	out, err := r.run(ctx, BackupArgs(repo, paths, tags, m), m)
	if err != nil {
		return Summary{}, err
	}
	return ParseBackupSummary(out)
}

// Restore restores the snapshot identified by snapshotID into target directory.
func (r Restic) Restore(ctx context.Context, repo string, snapshotID string, target string, m Mode) error {
	_, err := r.run(ctx, RestoreArgs(repo, snapshotID, target, m), m)
	return err
}

// RestorePath restores a single backed-up path (p) back to its own location as a
// subtree, so restic never reconciles the shared parent directory.
func (r Restic) RestorePath(ctx context.Context, repo, snapshotID, p string, m Mode) error {
	_, err := r.run(ctx, RestorePathArgs(repo, snapshotID, p, m), m)
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

// ---- JSON parsing ----------------------------------------------------------

// backupJSONLine is used to detect the summary line in restic --json backup
// output (which may also contain status lines with different message_type values).
type backupJSONLine struct {
	MessageType string `json:"message_type"`
	Summary
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
