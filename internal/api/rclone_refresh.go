package api

import (
	"os"
	"strings"
)

// rcloneSignInKeys are the options rclone writes into its config file on its
// own: a refreshed OAuth token, and the sessions Mega, Proton Drive and iCloud
// Drive keep between runs. Box and Jottacloud void a refresh token once it is
// used, so the copy in the settings has to follow what rclone wrote.
var rcloneSignInKeys = []string{
	"token",
	"session_id", "master_key",
	"client_uid", "client_access_token", "client_refresh_token", "client_salted_key_pass",
	"trust_token", "cookies",
}

// withRcloneRefreshes takes into conf the sign-in keys rclone wrote into
// onDisk, for every remote both have with the same type. A remote only one of
// them has is left as conf has it.
func withRcloneRefreshes(conf, onDisk string) string {
	if strings.TrimSpace(onDisk) == "" {
		return conf
	}
	blocks := splitRcloneSections(conf)
	changed := false
	for i, b := range blocks {
		if b.name == "" {
			continue
		}
		disk, ok := remoteOptions(onDisk, b.name)
		if !ok || remoteType(onDisk, b.name) != remoteType(conf, b.name) {
			continue
		}
		stored, _ := remoteOptions(conf, b.name)
		for _, key := range rcloneSignInKeys {
			if v := disk[key]; v != "" && v != stored[key] {
				blocks[i].text = setSectionOption(blocks[i].text, key, v)
				changed = true
			}
		}
	}
	if !changed {
		return conf
	}
	var b strings.Builder
	for _, block := range blocks {
		b.WriteString(block.text)
	}
	return b.String()
}

// carrySignIn copies the sign-in keys of the remote from onto the remote to,
// for a remote that was reached under another name and refreshed its sign-in
// there.
func carrySignIn(conf, from, to string) string {
	src, ok := remoteOptions(conf, from)
	if !ok {
		return conf
	}
	blocks := splitRcloneSections(conf)
	var b strings.Builder
	for _, block := range blocks {
		if block.name == to {
			for _, key := range rcloneSignInKeys {
				if v := src[key]; v != "" {
					block.text = setSectionOption(block.text, key, v)
				}
			}
		}
		b.WriteString(block.text)
	}
	return b.String()
}

// remoteType is the backend of one remote of an rclone config.
func remoteType(conf, remote string) string {
	for _, block := range splitRcloneSections(conf) {
		if block.name != remote {
			continue
		}
		for line := range strings.SplitSeq(block.text, "\n") {
			if key, value, ok := strings.Cut(line, "="); ok && strings.TrimSpace(key) == "type" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

// setSectionOption sets key in one section's text, in the line that holds it
// or in a new one after the others.
func setSectionOption(text, key, value string) string {
	body := strings.TrimRight(text, "\n")
	tail := text[len(body):]
	if tail == "" {
		tail = "\n"
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if k, _, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == key {
			lines[i] = key + " = " + value
			return strings.Join(lines, "\n") + tail
		}
	}
	return body + "\n" + key + " = " + value + tail
}

// rcloneFile is the rclone config as rclone last left it on disk.
func (s *Service) rcloneFile() string {
	raw, err := os.ReadFile(s.rcloneConfPath())
	if err != nil {
		return ""
	}
	return string(raw)
}

// currentRcloneConf returns the stored rclone config with what rclone wrote
// back into its file taken in first, for a copy that has to sign in later.
func (s *Service) currentRcloneConf() (string, error) {
	var out string
	err := s.editRcloneConf(func(conf string) (string, error) {
		out = conf
		return conf, nil
	})
	return out, err
}
