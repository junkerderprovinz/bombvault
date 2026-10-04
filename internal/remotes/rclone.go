package remotes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Timeout bounds one call to a remote, so a dead backend answers in the form
// instead of holding the request.
const Timeout = 30 * time.Second

// DraftName is the remote a draft is reachable under, for one call only.
const DraftName = "BVDRAFT"

// NameRe is what a remote name may look like. rclone addresses a remote as
// name:path, so a colon or a slash makes the location unparseable, and a space
// or a bracket breaks the section header.
var NameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Rclone runs the rclone binary against BombVault's own config file.
type Rclone struct {
	// Bin is the binary to run, "rclone" when empty.
	Bin    string
	Config string

	once     sync.Once
	backends map[string]Backend
	loadErr  error
}

// Draft is a remote described by a form and not saved yet.
type Draft struct {
	Backend  string
	Settings map[string]string
}

func (r *Rclone) command(ctx context.Context, env []string, args ...string) *exec.Cmd {
	bin := r.Bin
	if bin == "" {
		bin = "rclone"
	}
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // G204: fixed subcommands; secrets travel in the environment or on stdin
	cmd.Env = append(append(os.Environ(), "RCLONE_CONFIG="+r.Config), env...)
	return cmd
}

func (r *Rclone) run(ctx context.Context, env []string, stdin string, args ...string) ([]byte, error) {
	cmd := r.command(ctx, env, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("the remote did not answer in time")
		}
		return nil, errors.New(lastError(errBuf.String(), err))
	}
	return out.Bytes(), nil
}

// logPrefix is the time and level rclone puts in front of each line.
var logPrefix = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} [A-Z]+\s*:\s*`)

// lastError picks the line of rclone's output that says what went wrong, and
// of a summary that counts errors, the last error it names.
func lastError(stderr string, err error) string {
	var last string
	for line := range strings.SplitSeq(strings.TrimSpace(stderr), "\n") {
		if line = strings.TrimSpace(logPrefix.ReplaceAllString(strings.TrimSpace(line), "")); line != "" {
			last = line
		}
	}
	if _, after, ok := strings.Cut(last, "last error was: "); ok {
		last = after
	}
	if last == "" {
		return err.Error()
	}
	return last
}

// Backends returns how rclone describes the backends the providers use. It
// asks the binary once per process; the binary does not change underneath.
func (r *Rclone) Backends(ctx context.Context) (map[string]Backend, error) {
	r.once.Do(func() {
		out, err := r.run(ctx, nil, "", "config", "providers")
		if err != nil {
			r.loadErr = fmt.Errorf("rclone config providers: %w", err)
			return
		}
		r.backends, r.loadErr = ParseBackends(out)
	})
	return r.backends, r.loadErr
}

// Obscure turns a password into the form rclone reads. It asks rclone
// itself, since the format is rclone's and a wrong one fails to sign in
// while looking right. The password goes in on stdin, never as an argument.
func (r *Rclone) Obscure(ctx context.Context, plain string) (string, error) {
	if strings.TrimSpace(plain) == "" {
		return "", errors.New("the password is empty")
	}
	out, err := r.run(ctx, nil, plain, "obscure", "-")
	if err != nil {
		return "", fmt.Errorf("could not prepare the password with rclone: %w", err)
	}
	obscured := strings.TrimSpace(string(out))
	if obscured == "" {
		return "", errors.New("rclone returned an empty password")
	}
	return obscured, nil
}

// Settings returns a draft's settings as rclone stores them: every password
// obscured, empty values left out.
func (r *Rclone) Settings(ctx context.Context, d Draft) (map[string]string, error) {
	backends, err := r.Backends(ctx)
	if err != nil {
		return nil, err
	}
	b, ok := backends[d.Backend]
	if !ok {
		return nil, fmt.Errorf("rclone has no backend %q", d.Backend)
	}
	out := make(map[string]string, len(d.Settings))
	for k, v := range d.Settings {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if PasswordOption(b, k) {
			if v, err = r.Obscure(ctx, v); err != nil {
				return nil, err
			}
		}
		out[k] = v
	}
	return out, nil
}

// draftEnv defines the draft as the remote DraftName in the environment of
// one call.
func (r *Rclone) draftEnv(ctx context.Context, d Draft) ([]string, error) {
	settings, err := r.Settings(ctx, d)
	if err != nil {
		return nil, err
	}
	prefix := "RCLONE_CONFIG_" + DraftName + "_"
	env := []string{prefix + "TYPE=" + d.Backend}
	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, prefix+strings.ToUpper(k)+"="+settings[k])
	}
	return env, nil
}

// Folders lists the folders in dir of a saved remote, or of a draft when d is
// not nil. At the root of a bucket store they are the buckets.
func (r *Rclone) Folders(ctx context.Context, remote string, d *Draft, dir string) ([]string, error) {
	env, remote, err := r.target(ctx, remote, d)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	// One attempt: a form waits on the answer, and a wrong password does not
	// get righter by retrying.
	out, err := r.run(ctx, env, "", "lsjson", "--dirs-only", "--max-depth", "1",
		"--retries", "1", "--low-level-retries", "1", "--", Join(remote, dir))
	if err != nil {
		return nil, err
	}
	var entries []struct{ Name string }
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("read rclone's listing: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	return names, nil
}

// MakeFolder creates child in dir and returns the new folder's path.
func (r *Rclone) MakeFolder(ctx context.Context, remote string, d *Draft, dir, child string) (string, error) {
	child = strings.Trim(strings.TrimSpace(child), "/")
	if child == "" || strings.ContainsAny(child, "/\\") || child == "." || child == ".." {
		return "", errors.New("a folder name may not be empty or contain a slash")
	}
	env, remote, err := r.target(ctx, remote, d)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	created := path.Join(dir, child)
	if _, err := r.run(ctx, env, "", "mkdir", "--", Join(remote, created)); err != nil {
		return "", err
	}
	return created, nil
}

// Check reaches a remote by listing its root.
func (r *Rclone) Check(ctx context.Context, remote string, d *Draft) error {
	_, err := r.Folders(ctx, remote, d, "")
	return err
}

func (r *Rclone) target(ctx context.Context, remote string, d *Draft) ([]string, string, error) {
	if d == nil {
		if !NameRe.MatchString(remote) {
			return nil, "", errors.New("not a remote name")
		}
		return nil, remote, nil
	}
	env, err := r.draftEnv(ctx, *d)
	return env, DraftName, err
}

// Join puts a remote and a path together the way rclone reads them.
func Join(remote, dir string) string {
	return remote + ":" + strings.Trim(dir, "/")
}
