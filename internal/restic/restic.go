// Package restic provides argv builders and execution helpers for the restic
// backup CLI.  All operations run restic as an external process; no cgo or
// native bindings are used.
package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
)

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
	args = append(args, "--target", target)
	args = append(args, "--", snapshotID)
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
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Log the full stderr for server-side diagnostics, and surface a concise,
		// path-scrubbed reason to the caller so the UI shows WHY restic failed
		// (e.g. "repository is already locked") instead of a generic message.
		sub := subcommand(args)
		log.Printf("restic %s stderr: %s", sub, stderr.String())
		if reason := lastReason(stderr.String()); reason != "" {
			return nil, fmt.Errorf("restic %s failed: %s", sub, reason)
		}
		return nil, fmt.Errorf("restic %s failed", sub)
	}
	return stdout.Bytes(), nil
}

// reasonPathRe matches absolute-path-like tokens so they can be stripped from a
// surfaced restic error (defense-in-depth: repo/appdata paths must not leak).
var reasonPathRe = regexp.MustCompile(`(/[^\s:"']+)+`)

// lastReason returns the last non-empty line of stderr with absolute paths
// scrubbed and the length capped — a concise failure cause for the UI.
func lastReason(stderr string) string {
	reason := ""
	for _, line := range strings.Split(stderr, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			reason = s
		}
	}
	reason = reasonPathRe.ReplaceAllString(reason, "[path]")
	if len(reason) > 200 {
		reason = reason[:200]
	}
	return reason
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

// Forget removes the given snapshots from the repo, optionally pruning the freed
// data. A nil/empty ID list is a no-op (nothing to forget).
func (r Restic) Forget(ctx context.Context, repo string, snapshotIDs []string, prune bool, m Mode) error {
	if len(snapshotIDs) == 0 {
		return nil
	}
	_, err := r.run(ctx, ForgetArgs(repo, snapshotIDs, prune, m), m)
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
