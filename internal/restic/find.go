package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"strings"
	"time"
	"unicode"
)

// ErrFileNamePattern refuses a search text that cannot be a file name.
var ErrFileNamePattern = errors.New("search for a file name of 1 to 255 characters, without a slash or control characters")

// maxFileNameLen is the longest name ext4, XFS, ZFS and Btrfs allow.
const maxFileNameLen = 255

// FileNamePattern turns what a person typed into the glob restic find and
// MatchesName take. Plain text matches anywhere in a name. A typed * or ?
// stays a wildcard and the pattern then has to match the whole name. Every
// other glob character is taken literally, and so is a leading "!", which
// restic would read as a negation. The pattern is lower case because the
// search ignores case.
func FileNamePattern(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > maxFileNameLen || strings.ContainsAny(text, "/") || strings.ContainsFunc(text, unicode.IsControl) {
		return "", ErrFileNamePattern
	}
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch r {
		case '\\', '[', ']', '!':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	if strings.ContainsAny(text, "*?") {
		return b.String(), nil
	}
	return "*" + b.String() + "*", nil
}

// MatchesName reports whether the last part of p matches pattern. restic find
// also reports everything inside a matching folder, which this tells apart.
func MatchesName(pattern, p string) bool {
	ok, err := path.Match(pattern, strings.ToLower(path.Base(p)))
	return ok && err == nil
}

// FoundFile is one match restic find reports.
type FoundFile struct {
	Path    string    `json:"path"`
	Type    string    `json:"type"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`
}

// FindArgs returns the argv for searching the given snapshots for pattern,
// ignoring case. Without --snapshot restic would search every snapshot of the
// repository, so the caller passes at least one. find only reads, and it
// takes no lock so a search never waits for a backup or holds up a prune.
func FindArgs(repo string, snapshotIDs []string, pattern string, m Mode) []string {
	args := repoFlag(repo)
	args = append(args, "find", "--no-lock", "--json", "--ignore-case")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	for _, id := range snapshotIDs {
		args = append(args, "--snapshot="+id)
	}
	return append(args, "--", pattern)
}

// Find searches the given snapshots for files and folders whose name matches
// pattern, a FileNamePattern. onSnapshot gets the matches of one snapshot at a
// time, newest snapshot first, and only for a snapshot that has any.
func (r Restic) Find(ctx context.Context, repo string, snapshotIDs []string, pattern string, m Mode, onSnapshot func(snapshotID string, files []FoundFile)) error {
	args := FindArgs(repo, snapshotIDs, pattern, m)
	if err := checkRepoArgs(args); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, r.bin(), args...) //nolint:gosec // G204: argv is constructed by typed builders in this package and the pattern follows --
	configureProcGroup(cmd)
	cmd.Env = r.authEnv(m)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("restic find: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return runError(args, stderr.String())
	}
	decodeErr := decodeFind(stdout, pattern, onSnapshot)
	if decodeErr != nil {
		// restic blocks on a full pipe, and Wait would wait for it forever.
		_, _ = io.Copy(io.Discard, stdout)
	}
	if err := cmd.Wait(); err != nil {
		return ctxCancelErr(ctx, args, runError(args, stderr.String()))
	}
	if decodeErr != nil {
		return fmt.Errorf("restic find: read result: %w", decodeErr)
	}
	return nil
}

// decodeFind reads restic find's JSON as it arrives. The whole result is one
// array on one line, and a folder that matches brings every path below it, so
// each match is decoded alone and only the name matches are kept.
func decodeFind(rd io.Reader, pattern string, onSnapshot func(snapshotID string, files []FoundFile)) error {
	dec := json.NewDecoder(rd)
	if err := expectDelim(dec, '['); err != nil {
		return err
	}
	for dec.More() {
		id, files, err := decodeFindSnapshot(dec, pattern)
		if err != nil {
			return err
		}
		if len(files) > 0 {
			onSnapshot(id, files)
		}
	}
	return expectDelim(dec, ']')
}

// decodeFindSnapshot reads one snapshot's object, whose id comes after its
// matches.
func decodeFindSnapshot(dec *json.Decoder, pattern string) (string, []FoundFile, error) {
	if err := expectDelim(dec, '{'); err != nil {
		return "", nil, err
	}
	var id string
	var files []FoundFile
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return "", nil, err
		}
		switch key {
		case "matches":
			if err := expectDelim(dec, '['); err != nil {
				return "", nil, err
			}
			for dec.More() {
				var f FoundFile
				if err := dec.Decode(&f); err != nil {
					return "", nil, err
				}
				if MatchesName(pattern, f.Path) {
					files = append(files, f)
				}
			}
			if err := expectDelim(dec, ']'); err != nil {
				return "", nil, err
			}
		case "snapshot":
			if err := dec.Decode(&id); err != nil {
				return "", nil, err
			}
		default:
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return "", nil, err
			}
		}
	}
	return id, files, expectDelim(dec, '}')
}

func expectDelim(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok != want {
		return fmt.Errorf("got %v where %v was expected", tok, want)
	}
	return nil
}
