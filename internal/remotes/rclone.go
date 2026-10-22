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

	mu       sync.Mutex
	backends map[string]Backend
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
		return nil, errors.New(explain(lastError(errBuf.String(), err)))
	}
	return out.Bytes(), nil
}

// callPrefixRe is what rclone puts in front of a failure: the remote, which
// for a draft is a name nobody chose, and the call inside rclone that failed.
var callPrefixRe = regexp.MustCompile(`^(?:Failed to create file system for "[^"]*": |NewFs: |error in ListJSON: )`)

// hints say what to do about the failures people meet setting a remote up,
// which rclone words for someone who edits rclone.conf by hand. Without keep
// rclone's sentence is left out, because it points at commands BombVault
// does not have.
var hints = []struct {
	match, hint string
	keep        bool
}{
	{"unable to authenticate", "the server did not accept BombVault's SSH key. Add the public key the form shows to the authorized keys of this user on the server, then test again", true},
	{"Non-anonymous sessions must use encryption", "the server only takes encrypted logins. Under More settings set explicit_tls to true, or tls to true for a server that expects FTPS on port 990", true},
	{"the remote url looks incorrect", "this is not the address Nextcloud keeps files under. Use https://cloud.example.com/remote.php/dav/files/USER/ with your Nextcloud user name in place of USER", false},
	{"AuthorizationHeaderMalformed", "the server signs for another region than the form names, and an empty region means us-east-1. Fill in the region the server is set up with: for Garage it is s3_region in garage.toml, garage unless it was changed", true},
}

// explain takes rclone's prefixes off a failure and puts a hint in front of
// one the form can be changed to get past.
func explain(msg string) string {
	for next := callPrefixRe.ReplaceAllString(msg, ""); next != msg; next = callPrefixRe.ReplaceAllString(msg, "") {
		msg = next
	}
	for _, h := range hints {
		if !strings.Contains(msg, h.match) {
			continue
		}
		if !h.keep {
			return h.hint
		}
		return h.hint + " (" + msg + ")"
	}
	return msg
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

// Backends returns how rclone describes the backends the providers use. The
// first answer is kept, since the binary does not change underneath; a failed
// one is not, so the next caller asks again.
func (r *Rclone) Backends(ctx context.Context) (map[string]Backend, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.backends != nil {
		return r.backends, nil
	}
	// Detached from the caller: the page that asks first may be closed while
	// rclone answers, and the list serves every caller after it.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), Timeout)
	defer cancel()
	out, err := r.run(ctx, nil, "", "config", "providers")
	if err != nil {
		return nil, fmt.Errorf("rclone config providers: %w", err)
	}
	backends, err := ParseBackends(out)
	if err != nil {
		return nil, err
	}
	r.backends = backends
	return backends, nil
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

// Free returns the free space of a remote in bytes. ok is false when the
// backend does not report it, which most object stores do not.
func (r *Rclone) Free(ctx context.Context, remote string, d *Draft) (free int64, ok bool) {
	env, remote, err := r.target(ctx, remote, d)
	if err != nil {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := r.run(ctx, env, "", "about", "--json", "--retries", "1", "--low-level-retries", "1", "--", Join(remote, ""))
	if err != nil {
		return 0, false
	}
	var about struct{ Free *int64 }
	if json.Unmarshal(out, &about) != nil || about.Free == nil {
		return 0, false
	}
	return *about.Free, true
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
