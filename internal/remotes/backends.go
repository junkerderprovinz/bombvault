package remotes

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Option is one setting a backend takes, as rclone describes it.
type Option struct {
	Name     string `json:"name"`
	Help     string `json:"help"`
	Required bool   `json:"required"`
	// Secret hides the value in the form and keeps it out of every answer.
	Secret bool `json:"secret"`
	// Password means rclone reads the value obscured, so it is obscured
	// before it is stored or handed to rclone.
	Password bool `json:"password"`
	Advanced bool `json:"advanced"`
	// Essential means the remote does not work in practice without it.
	// rclone's Required describes its interactive setup, which prompts for
	// everything else with a default: s3 marks none of its options required.
	Essential bool      `json:"essential"`
	Default   string    `json:"default"`
	Examples  []Example `json:"examples,omitempty"`
	// Providers limits the option to these S3 providers; empty means all.
	Providers []string `json:"providers,omitempty"`
}

// Example is one suggested value. Provider limits it to one S3 provider.
type Example struct {
	Value    string `json:"value"`
	Help     string `json:"help"`
	Provider string `json:"provider,omitempty"`
}

// Backend is one kind of storage rclone can reach.
type Backend struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Options     []Option `json:"options"`
}

// essential lists, per backend, the settings without which a remote will not
// work. rclone marks a token advanced because its own setup fetches it in a
// browser; this form has no browser, so the token field has to be shown.
var essential = map[string]map[string]bool{
	"s3":                   {"provider": true, "access_key_id": true, "secret_access_key": true, "region": true, "endpoint": true},
	"smb":                  {"host": true, "user": true, "pass": true, "domain": true},
	"sftp":                 {"host": true, "user": true, "port": true},
	"webdav":               {"url": true, "vendor": true, "user": true, "pass": true},
	"ftp":                  {"host": true, "user": true, "pass": true, "port": true},
	"mega":                 {"user": true, "pass": true},
	"opendrive":            {"username": true, "password": true},
	"protondrive":          {"username": true, "password": true, "2fa": true},
	"seafile":              {"url": true, "user": true, "pass": true, "library": true},
	"koofr":                {"provider": true, "user": true, "password": true},
	"iclouddrive":          {"apple_id": true, "password": true},
	"mailru":               {"user": true, "pass": true},
	"filen":                {"email": true, "password": true, "api_key": true},
	"internxt":             {"email": true, "pass": true},
	"pikpak":               {"user": true, "pass": true},
	"azureblob":            {"account": true, "key": true},
	"azurefiles":           {"account": true, "key": true, "share_name": true},
	"swift":                {"auth": true, "user": true, "key": true},
	"qingstor":             {"access_key_id": true, "secret_access_key": true, "zone": true},
	"netstorage":           {"host": true, "account": true, "secret": true},
	"filescom":             {"site": true, "username": true, "password": true},
	"sugarsync":            {"app_id": true, "access_key_id": true, "private_access_key": true},
	"ulozto":               {"username": true, "password": true, "app_token": true},
	"sia":                  {"api_url": true, "api_password": true},
	"quatrix":              {"api_key": true, "host": true},
	"linkbox":              {"token": true},
	"pixeldrain":           {"api_key": true},
	"fichier":              {"api_key": true},
	"filelu":               {"key": true},
	"drime":                {"access_token": true},
	"shade":                {"api_key": true, "drive_id": true},
	"filefabric":           {"url": true, "permanent_token": true},
	"hdfs":                 {"namenode": true, "username": true},
	"oracleobjectstorage":  {"provider": true, "namespace": true, "compartment": true, "region": true, "endpoint": true},
	"dropbox":              {"token": true},
	"drive":                {"token": true, "scope": true},
	"onedrive":             {"token": true, "drive_id": true, "drive_type": true},
	"pcloud":               {"token": true, "hostname": true},
	"box":                  {"token": true},
	"jottacloud":           {"token": true},
	"yandex":               {"token": true},
	"hidrive":              {"token": true},
	"huaweidrive":          {"token": true},
	"putio":                {"token": true},
	"sharefile":            {"token": true},
	"zoho":                 {"token": true, "region": true},
	"premiumizeme":         {"token": true},
	"google cloud storage": {"token": true, "project_number": true, "service_account_file": true},
}

// rcloneOption is one option as `rclone config providers` prints it.
type rcloneOption struct {
	Name       string
	Help       string
	Provider   string
	Required   bool
	IsPassword bool
	Sensitive  bool
	Advanced   bool
	DefaultStr string
	Examples   []struct {
		Value    string
		Help     string
		Provider string
	}
}

// ParseBackends reads the JSON `rclone config providers` prints. Backends
// no provider uses are left out, and so are rclone's hidden options.
func ParseBackends(raw []byte) (map[string]Backend, error) {
	var in []struct {
		Name        string
		Description string
		Options     []rcloneOption
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("read rclone's backends: %w", err)
	}
	used := map[string]bool{}
	for _, p := range providers {
		if p.Backend != "" {
			used[p.Backend] = true
		}
	}
	out := map[string]Backend{}
	for _, b := range in {
		if !used[b.Name] {
			continue
		}
		be := Backend{Name: b.Name, Description: b.Description, Options: []Option{}}
		for _, o := range b.Options {
			be.Options = append(be.Options, toOption(b.Name, o))
		}
		sort.SliceStable(be.Options, func(i, j int) bool {
			return optionRank(be.Options[i]) < optionRank(be.Options[j])
		})
		out[b.Name] = be
	}
	return out, nil
}

func toOption(backend string, o rcloneOption) Option {
	secret := o.IsPassword || o.Sensitive
	out := Option{
		Name:      o.Name,
		Help:      firstLine(o.Help),
		Required:  o.Required,
		Secret:    secret,
		Password:  o.IsPassword,
		Advanced:  o.Advanced,
		Essential: essential[backend][o.Name],
		Default:   defaultText(o.DefaultStr),
	}
	if o.Provider != "" {
		out.Providers = strings.Split(o.Provider, ",")
	}
	if !secret {
		for _, e := range o.Examples {
			out.Examples = append(out.Examples, Example{Value: e.Value, Help: firstLine(e.Help), Provider: e.Provider})
		}
	}
	return out
}

// optionRank puts required and essential options first, then the ordinary
// ones, then the advanced ones.
func optionRank(o Option) int {
	switch {
	case o.Required, o.Essential:
		return 0
	case !o.Advanced:
		return 1
	default:
		return 2
	}
}

// firstLine keeps the first line of a help text, which is all a label beside
// a field has room for.
func firstLine(help string) string {
	line, _, _ := strings.Cut(help, "\n")
	return strings.TrimSpace(line)
}

func defaultText(v string) string {
	if v == "false" || v == "0" || v == "<nil>" {
		return ""
	}
	return v
}

// SecretOption reports whether a backend's option holds a secret.
func SecretOption(b Backend, name string) bool {
	for _, o := range b.Options {
		if o.Name == name {
			return o.Secret
		}
	}
	return false
}

// PasswordOption reports whether rclone reads a backend's option obscured.
func PasswordOption(b Backend, name string) bool {
	for _, o := range b.Options {
		if o.Name == name {
			return o.Password
		}
	}
	return false
}
