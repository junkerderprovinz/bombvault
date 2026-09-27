package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
)

// Building an rclone remote from a form instead of from a pasted INI file.
//
// SMB and WebDAV are the two destinations asked for most often, and BombVault
// could technically already reach both: rclone speaks them, restic speaks
// rclone. What stood in the way was that the operator had to write the INI
// section by hand.
//
// The documented alternative is worse than it looks. "Mount the share on Unraid
// and point a backup path at it" puts the repository on a CIFS mount, which
// restic's own documentation explicitly advises against: file locking and
// rename semantics over CIFS have historically corrupted repositories. Going
// through rclone skips the mount entirely, so this is not only more convenient,
// it is the sounder of the two routes.
//
// NFS is left out. rclone has no NFS backend and restic has no NFS
// backend, so for NFS the host mount remains the only way, and saying so beats
// offering a form that cannot work.
const (
	rcloneTypeSMB    = "smb"
	rcloneTypeWebDAV = "webdav"
)

// rcloneRemote is one destination as the form describes it.
type rcloneRemote struct {
	// Name becomes the remote's INI section and the prefix of every repository
	// location built from it ("nas:backups/containers").
	Name string
	Type string

	// SMB.
	Host string
	// Share is carried for the caller's convenience when it builds the
	// repository location. It stays out of the config on purpose: an
	// rclone SMB remote addresses the share as the first path segment, so a
	// "share =" key would make every path double up.
	Share string

	// WebDAV.
	URL string
	// Vendor tunes rclone's WebDAV dialect ("nextcloud", "owncloud",
	// "sharepoint", "other"). Left empty it falls back to "other", which costs
	// modification times on a Nextcloud server.
	Vendor string

	User string
	// ObscuredPass is the password in rclone's own obscured form, never the
	// plaintext. rclone reads nothing else, so a plaintext value here would not
	// merely be insecure, it would not work.
	ObscuredPass string
}

// rcloneNameRe is what a remote name may look like. rclone addresses a remote
// as "name:path", so a colon or a slash in the name makes the location
// unparseable, and a space or a bracket breaks the INI section header.
var rcloneNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func (r rcloneRemote) validate() error {
	if !rcloneNameRe.MatchString(r.Name) {
		return errors.New("the remote name may contain only letters, digits, dashes and underscores")
	}
	if r.ObscuredPass == "" {
		// Guarded here rather than trusted from the caller: a plaintext
		// password reaching the config file is the failure this whole type
		// exists to prevent.
		return errors.New("the password was not obscured")
	}
	switch r.Type {
	case rcloneTypeSMB:
		if strings.TrimSpace(r.Host) == "" {
			return errors.New("an SMB remote needs a host")
		}
	case rcloneTypeWebDAV:
		if strings.TrimSpace(r.URL) == "" {
			return errors.New("a WebDAV remote needs a URL")
		}
	default:
		return fmt.Errorf("unsupported remote type %q", r.Type)
	}
	if strings.TrimSpace(r.User) == "" {
		return errors.New("the remote needs a user name")
	}
	return nil
}

// rcloneSection renders the INI section for one remote.
func rcloneSection(r rcloneRemote) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s]\n", r.Name)
	fmt.Fprintf(&b, "type = %s\n", r.Type)
	switch r.Type {
	case rcloneTypeSMB:
		fmt.Fprintf(&b, "host = %s\n", r.Host)
	case rcloneTypeWebDAV:
		fmt.Fprintf(&b, "url = %s\n", r.URL)
		vendor := r.Vendor
		if vendor == "" {
			vendor = "other"
		}
		fmt.Fprintf(&b, "vendor = %s\n", vendor)
	}
	fmt.Fprintf(&b, "user = %s\n", r.User)
	fmt.Fprintf(&b, "pass = %s\n", r.ObscuredPass)
	return b.String()
}

// sectionHeaderRe matches an INI section header at the start of a line.
var sectionHeaderRe = regexp.MustCompile(`(?m)^\[([^\]]+)\]`)

// appendRcloneSection adds r to conf, replacing any section of the same name.
//
// Replacing rather than appending a duplicate matters: rclone reads the first
// section with a given name, so a second one would be silently ignored and an
// operator correcting a password would see no change at all.
//
// Every other section is copied through untouched. Someone with ten working
// cloud remotes must not have them rewritten because they added a share.
func appendRcloneSection(conf string, r rcloneRemote) (string, error) {
	if err := r.validate(); err != nil {
		return "", err
	}

	var kept []string
	for _, block := range splitRcloneSections(conf) {
		if strings.EqualFold(block.name, r.Name) {
			continue // replaced below
		}
		kept = append(kept, strings.TrimRight(block.text, "\n"))
	}
	kept = append(kept, strings.TrimRight(rcloneSection(r), "\n"))
	return strings.Join(kept, "\n\n") + "\n", nil
}

type rcloneBlock struct {
	name string
	text string
}

// splitRcloneSections cuts a config into its sections. Anything before the
// first header (comments, stray lines) is kept as an unnamed block so a
// round-trip never drops it.
func splitRcloneSections(conf string) []rcloneBlock {
	if strings.TrimSpace(conf) == "" {
		return nil
	}
	idx := sectionHeaderRe.FindAllStringSubmatchIndex(conf, -1)
	if len(idx) == 0 {
		return []rcloneBlock{{name: "", text: conf}}
	}
	var out []rcloneBlock
	if lead := strings.TrimSpace(conf[:idx[0][0]]); lead != "" {
		out = append(out, rcloneBlock{name: "", text: conf[:idx[0][0]]})
	}
	for i, m := range idx {
		end := len(conf)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		out = append(out, rcloneBlock{name: conf[m[2]:m[3]], text: conf[m[0]:end]})
	}
	return out
}

// rcloneObscure turns a plaintext password into rclone's obscured form by
// asking rclone itself.
//
// Reimplementing the transform in Go would be a handful of lines, and it would
// be the wrong handful: it is rclone's format, it can change, and a silently
// wrong obscure produces a config that looks right and fails to authenticate.
// Asking the binary that will read it back cannot drift.
//
// The password goes in on STDIN, never as an argument. An argument is visible
// in the process list to every user on the host, which for a backup
// destination's credentials is exactly the leak this feature must not add.
func rcloneObscure(ctx context.Context, plain string) (string, error) {
	if strings.TrimSpace(plain) == "" {
		return "", errors.New("the password is empty")
	}
	cmd := exec.CommandContext(ctx, "rclone", "obscure", "-") //nolint:gosec // G204: fixed argv; the secret travels on stdin
	cmd.Stdin = strings.NewReader(plain)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		// rclone's own message is surfaced, never the password. The most common
		// cause by far is that rclone is not installed, which the message says
		// plainly enough to act on.
		detail := strings.TrimSpace(errBuf.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("could not prepare the password with rclone: %s", detail)
	}
	obscured := strings.TrimSpace(out.String())
	if obscured == "" {
		return "", errors.New("rclone returned an empty password")
	}
	return obscured, nil
}

// AddRcloneRemote stores a destination described by a form.
//
// The plaintext password exists for exactly as long as it takes to obscure it:
// it is never written to the settings, never logged, and never returned.
func (s *Service) AddRcloneRemote(ctx context.Context, r rcloneRemote, plainPassword string) error {
	obscured, err := rcloneObscure(ctx, plainPassword)
	if err != nil {
		return err
	}
	r.ObscuredPass = obscured

	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	current, err := s.decodeRcloneConf(settings)
	if err != nil {
		return err
	}
	next, err := appendRcloneSection(current, r)
	if err != nil {
		return err
	}
	return s.SetRcloneConf(next)
}

// handleAddRcloneRemote stores an SMB or WebDAV destination from a form.
// POST /api/offsite/rclone-remote
//
// Behind requireAuthForSecrets: the request body carries a live password for a
// storage backend, and a route that accepts one must not be open in
// trusted-LAN mode the way the read API is by design.
//
// The answer never echoes the password back, not even on failure. The most
// likely failure by far is a typo in it.
func (h *Handler) handleAddRcloneRemote(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "adding a storage destination") {
		return
	}
	var body struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Host     string `json:"host"`
		Share    string `json:"share"`
		URL      string `json:"url"`
		Vendor   string `json:"vendor"`
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &body) {
		return
	}

	remote := rcloneRemote{
		Name: strings.TrimSpace(body.Name), Type: body.Type,
		Host: strings.TrimSpace(body.Host), Share: strings.TrimSpace(body.Share),
		URL: strings.TrimSpace(body.URL), Vendor: body.Vendor,
		User: strings.TrimSpace(body.User),
	}
	if err := h.svc.AddRcloneRemote(r.Context(), remote, body.Password); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	// The location the operator now pastes into a backup path. Built here
	// rather than left to them, because "name:share/path" is exactly the shape
	// people get wrong, and getting it wrong produces a repository in an
	// unexpected place rather than an error.
	location := remote.Name + ":"
	if remote.Type == rcloneTypeSMB && remote.Share != "" {
		location += remote.Share
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"location": "rclone:" + location}))
}
