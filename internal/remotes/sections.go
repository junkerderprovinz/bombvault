package remotes

import (
	"regexp"
	"strings"
)

// Section is one remote of an rclone config with its secrets left out.
type Section struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Options []SectionOption `json:"options"`
}

// SectionOption is one setting of a remote. Hidden means the config holds a
// value that is not shown.
type SectionOption struct {
	Name   string `json:"name"`
	Value  string `json:"value,omitempty"`
	Hidden bool   `json:"hidden,omitempty"`
}

var sectionHeaderRe = regexp.MustCompile(`^\[([^\]]+)\]$`)

// secretNameRe matches option names that can carry a secret although rclone
// does not mark them: webdav's headers take an Authorization line, and a
// command or an environment can have a password in it.
var secretNameRe = regexp.MustCompile(`pass|secret|token|key|auth|cred|cookie|header|command|env|session|sign|cert|pem`)

// userinfoRe matches the sign-in part of a URL.
var userinfoRe = regexp.MustCompile(`://[^/\s@]+@`)

// Redact reads the remotes of an rclone config and keeps the value of an
// option only where rclone's description of the backend lists the option,
// does not mark it as a secret, and its name does not look like one. A remote
// whose backend is not in backends shows the names of its options and no
// value.
func Redact(conf string, backends map[string]Backend) []Section {
	sections := []Section{}
	var raw []SectionOption
	flush := func() {
		if len(sections) == 0 {
			return
		}
		s := &sections[len(sections)-1]
		backend, described := backends[s.Type]
		for _, o := range raw {
			switch {
			case o.Value == "":
			case described && shown(backend, o.Name):
				o.Value = userinfoRe.ReplaceAllString(o.Value, "://[redacted]@")
			default:
				o = SectionOption{Name: o.Name, Hidden: true}
			}
			s.Options = append(s.Options, o)
		}
		raw = nil
	}
	for line := range strings.SplitSeq(conf, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if m := sectionHeaderRe.FindStringSubmatch(line); m != nil {
			flush()
			sections = append(sections, Section{Name: strings.TrimSpace(m[1]), Options: []SectionOption{}})
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || len(sections) == 0 {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key == "type" {
			sections[len(sections)-1].Type = value
			continue
		}
		raw = append(raw, SectionOption{Name: key, Value: value})
	}
	flush()
	return sections
}

// shown reports whether the value of a backend's option may leave the server.
func shown(b Backend, name string) bool {
	if secretNameRe.MatchString(name) {
		return false
	}
	for _, o := range b.Options {
		if o.Name == name {
			return !o.Secret
		}
	}
	return false
}
