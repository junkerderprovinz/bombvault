package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/remotes"
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

func (r rcloneRemote) validate() error {
	if !remotes.NameRe.MatchString(r.Name) {
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
	return replaceRcloneSection(conf, r.Name, rcloneSection(r)), nil
}

// replaceRcloneSection puts section in place of the one called name, or
// after the others when there is none. An empty section removes it.
func replaceRcloneSection(conf, name, section string) string {
	var kept []string
	for _, block := range splitRcloneSections(conf) {
		if strings.EqualFold(block.name, name) {
			continue
		}
		kept = append(kept, strings.TrimRight(block.text, "\n"))
	}
	if section != "" {
		kept = append(kept, strings.TrimRight(section, "\n"))
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, "\n\n") + "\n"
}

// settingKeyRe is what an rclone option name looks like.
var settingKeyRe = regexp.MustCompile(`^[a-z0-9_]+$`)

// settingsSection renders a remote of any backend. A key or value that could
// end the line is refused: it would add settings, or a whole remote, nobody
// typed.
func settingsSection(name, backend string, settings map[string]string) (string, error) {
	if !remotes.NameRe.MatchString(name) {
		return "", errors.New("the remote name may contain only letters, digits, dashes and underscores")
	}
	if strings.ContainsAny(backend, "\r\n") {
		return "", errors.New("the backend name is not valid")
	}
	keys := make([]string, 0, len(settings))
	for k, v := range settings {
		if !settingKeyRe.MatchString(k) || k == "type" {
			return "", fmt.Errorf("%q is not a setting rclone takes", k)
		}
		if strings.ContainsAny(v, "\r\n") {
			return "", fmt.Errorf("the value of %s may not contain a line break", k)
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	fmt.Fprintf(&b, "[%s]\ntype = %s\n", name, backend)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s = %s\n", k, settings[k])
	}
	return b.String(), nil
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

// AddRcloneRemote stores a destination described by a form.
//
// The plaintext password exists for exactly as long as it takes to obscure it:
// it is never written to the settings, never logged, and never returned.
func (s *Service) AddRcloneRemote(ctx context.Context, r rcloneRemote, plainPassword string) error {
	obscured, err := s.remotes().Obscure(ctx, plainPassword)
	if err != nil {
		return err
	}
	r.ObscuredPass = obscured
	return s.editRcloneConf(func(conf string) (string, error) {
		return appendRcloneSection(conf, r)
	})
}

// handleAddRcloneRemote stores an SMB or WebDAV destination from a form.
// POST /api/offsite/rclone-remote
//
// The answer never echoes the password back, not even on failure. The most
// likely failure by far is a typo in it.
func (h *Handler) handleAddRcloneRemote(w http.ResponseWriter, r *http.Request) {
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
