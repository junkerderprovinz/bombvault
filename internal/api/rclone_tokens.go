package api

import (
	"os"
	"strings"
)

// rcloneRefreshedKeys are the options rclone writes back into its config file
// on its own: an OAuth token after each refresh, and the session of iCloud
// Drive. Box hands out a refresh token that works once, so the stored copy is
// dead after rclone's first refresh.
var rcloneRefreshedKeys = []string{"token", "cookies", "trust_token"}

// withRefreshedTokens takes into stored the values rclone has written into
// disk since stored was saved. Only a remote of the same name and type in
// both is touched, and only the options rclone refreshes.
func withRefreshedTokens(stored, disk string) string {
	if strings.TrimSpace(disk) == "" {
		return stored
	}
	fresh := map[string]map[string]string{}
	for _, b := range splitRcloneSections(disk) {
		if b.name != "" {
			fresh[b.name] = sectionValues(b.text)
		}
	}
	blocks := splitRcloneSections(stored)
	changed := false
	for i, b := range blocks {
		newer, ok := fresh[b.name]
		if b.name == "" || !ok {
			continue
		}
		own := sectionValues(b.text)
		if own["type"] != newer["type"] {
			continue
		}
		for _, key := range rcloneRefreshedKeys {
			if v := newer[key]; v != "" && v != own[key] {
				blocks[i].text = setSectionValue(blocks[i].text, key, v)
				changed = true
			}
		}
	}
	if !changed {
		return stored
	}
	var out strings.Builder
	for _, b := range blocks {
		out.WriteString(b.text)
		if !strings.HasSuffix(b.text, "\n") {
			out.WriteString("\n")
		}
	}
	return out.String()
}

// sectionValues reads the options of one section, its type included.
func sectionValues(text string) map[string]string {
	out := map[string]string{}
	for line := range strings.SplitSeq(text, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if key = strings.TrimSpace(key); ok && settingKeyRe.MatchString(key) {
			out[key] = strings.TrimSpace(value)
		}
	}
	return out
}

// setSectionValue puts key = value into a section, in place of the line that
// holds key or after the others.
func setSectionValue(text, key, value string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		if k, _, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == key {
			lines[i] = key + " = " + value
			return strings.Join(lines, "\n") + "\n" + trailingBlank(text)
		}
	}
	lines = append(lines, key+" = "+value)
	return strings.Join(lines, "\n") + "\n" + trailingBlank(text)
}

// trailingBlank is the blank line a section ends with before the next one.
func trailingBlank(text string) string {
	if strings.HasSuffix(text, "\n\n") {
		return "\n"
	}
	return ""
}

// rcloneConfOnDisk is the config file rclone reads and writes, "" when there
// is none.
func (s *Service) rcloneConfOnDisk() string {
	b, err := os.ReadFile(s.rcloneConfPath())
	if err != nil {
		return ""
	}
	return string(b)
}
