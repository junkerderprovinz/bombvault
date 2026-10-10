package remotes

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// markedSecret reads which options rclone itself marks as a password or as
// sensitive, straight from its JSON, so the check does not lean on the
// parsing it checks.
func markedSecret(t *testing.T) map[string]map[string]bool {
	t.Helper()
	raw, err := os.ReadFile("testdata/rclone-1.75.1-providers.json")
	if err != nil {
		t.Fatal(err)
	}
	var in []struct {
		Name    string
		Options []struct {
			Name       string
			IsPassword bool
			Sensitive  bool
		}
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]bool{}
	for _, b := range in {
		out[b.Name] = map[string]bool{}
		for _, o := range b.Options {
			out[b.Name][o.Name] = o.IsPassword || o.Sensitive
		}
	}
	return out
}

// everyOptionSet writes one remote per backend with every option rclone lists
// for it set to a value that names the backend and the option.
func everyOptionSet(backends map[string]Backend) string {
	var conf strings.Builder
	for name, b := range backends {
		fmt.Fprintf(&conf, "[%s]\ntype = %s\n", remoteNameOf(name), name)
		for _, o := range b.Options {
			fmt.Fprintf(&conf, "%s = value-of-%s-%s\n", o.Name, remoteNameOf(name), o.Name)
		}
		conf.WriteString("\n")
	}
	return conf.String()
}

func remoteNameOf(backend string) string { return strings.ReplaceAll(backend, " ", "_") }

var looksSecret = regexp.MustCompile(`pass|secret|token|key|credential`)

func TestRedactShowsNoValueOfAnOptionRcloneMarksOrThatLooksSecret(t *testing.T) {
	backends := fixture(t)
	marked := markedSecret(t)
	sections := Redact(everyOptionSet(backends), backends)
	if len(sections) != len(backends) {
		t.Fatalf("read %d remotes, want %d", len(sections), len(backends))
	}
	shownSomething := false
	for _, s := range sections {
		for _, o := range s.Options {
			if o.Value == "" {
				if !o.Hidden {
					t.Errorf("%s: %s has neither a value nor the mark that one is hidden", s.Type, o.Name)
				}
				continue
			}
			shownSomething = true
			if marked[s.Type][o.Name] {
				t.Errorf("%s: %s is a secret to rclone and its value is shown", s.Type, o.Name)
			}
			if looksSecret.MatchString(o.Name) {
				t.Errorf("%s: %s looks like a secret and its value is shown", s.Type, o.Name)
			}
		}
	}
	if !shownSomething {
		t.Fatal("no value is shown at all, so the checks above saw nothing")
	}
}

func TestRedactKeepsWhatALocationPageShows(t *testing.T) {
	conf := `# written by hand
[nas]
type = sftp
port = 2222
pass = b2JzY3VyZWQ
shell_type = unix

[box]
host = files.example.org
type = webdav
url = https://ada:hunter2@files.example.org/dav
vendor = nextcloud
headers = Authorization,Bearer abc123
`
	got := Redact(conf, fixture(t))

	want := []Section{
		{Name: "nas", Type: "sftp", Options: []SectionOption{
			{Name: "port", Value: "2222"}, {Name: "pass", Hidden: true}, {Name: "shell_type", Value: "unix"},
		}},
		{Name: "box", Type: "webdav", Options: []SectionOption{
			{Name: "host", Hidden: true}, {Name: "url", Value: "https://[redacted]@files.example.org/dav"},
			{Name: "vendor", Value: "nextcloud"}, {Name: "headers", Hidden: true},
		}},
	}
	if !slices.EqualFunc(got, want, func(a, b Section) bool {
		return a.Name == b.Name && a.Type == b.Type && slices.Equal(a.Options, b.Options)
	}) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestRedactHidesEveryValueOfABackendRcloneDidNotDescribe(t *testing.T) {
	conf := "[vault]\ntype = crypt\nremote = nas:secret-folder\npassword = b2JzY3VyZWQ\nsuffix =\n"

	for name, backends := range map[string]map[string]Backend{"no description at all": nil, "other backends only": fixture(t)} {
		got := Redact(conf, backends)

		if len(got) != 1 || got[0].Name != "vault" || got[0].Type != "crypt" {
			t.Fatalf("%s: got %+v, want the one remote with its type", name, got)
		}
		want := []SectionOption{{Name: "remote", Hidden: true}, {Name: "password", Hidden: true}, {Name: "suffix"}}
		if !slices.Equal(got[0].Options, want) {
			t.Errorf("%s: options = %+v, want %+v", name, got[0].Options, want)
		}
	}
}

func TestRedactOfAnEmptyConfigIsAnEmptyList(t *testing.T) {
	if got := Redact("", fixture(t)); got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want an empty list that encodes as []", got)
	}
}
