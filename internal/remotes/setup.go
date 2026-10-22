package remotes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Question is a choice rclone's own setup of a backend leaves to the person
// setting it up, such as which of several drives a sign-in opens. The answer
// goes back as the setting Name.
type Question struct {
	Name    string    `json:"name"`
	Choices []Example `json:"choices"`
}

// setupStep is the part of rclone's interactive setup a form cannot do for
// one backend.
type setupStep struct {
	// due says whether a draft still needs the step.
	due func(settings map[string]string) bool
	// answers are the questions the form answers without asking.
	answers map[string]string
	// asked maps a question of the setup to the setting that answers it. The
	// setting stays out of the config the setup starts from.
	asked map[string]string
}

func hasToken(s map[string]string) bool { return strings.TrimSpace(s["token"]) != "" }

var setupSteps = map[string]setupStep{
	// Jottacloud signs in with a personal login token, which only its setup
	// trades for the OAuth token and the client it belongs to.
	"jottacloud": {
		due:     func(s map[string]string) bool { return hasToken(s) && !strings.HasPrefix(strings.TrimSpace(s["token"]), "{") },
		answers: map[string]string{"config_type": "standard", "config_non_standard": "false"},
		asked:   map[string]string{"config_login_token": "token"},
	},
	// OneDrive needs the drive the token opens, which the setup looks up.
	"onedrive": {
		due: func(s map[string]string) bool { return hasToken(s) && (s["drive_id"] == "" || s["drive_type"] == "") },
		answers: map[string]string{
			"config_refresh_token": "false", "config_type": "onedrive", "config_drive_ok": "true",
		},
	},
	// Zoho WorkDrive needs the workspace to work in.
	"zoho": {
		due:     func(s map[string]string) bool { return hasToken(s) && s["root_folder_id"] == "" },
		answers: map[string]string{"config_refresh_token": "false"},
	},
}

// SetupDue says whether rclone's own setup still has to run for a draft
// before the remote can be reached.
func SetupDue(d Draft) bool {
	step, ok := setupSteps[d.Backend]
	return ok && step.due(d.Settings)
}

// configOut is what `rclone config --non-interactive` prints: the next
// question, or an empty state once the setup is done.
type configOut struct {
	State  string
	Error  string
	Option *struct {
		Name     string
		Help     string
		Examples []struct{ Value, Help string }
	}
}

// Setup runs the part of rclone's own setup a draft still needs, on a config
// file of its own. picked holds the answers to questions asked before. What
// the setup wrote into the remote lands in d.Settings and d.Learned; a choice
// with more than one way to go comes back as a question.
func (r *Rclone) Setup(ctx context.Context, d *Draft, picked map[string]string) (*Question, error) {
	step, ok := setupSteps[d.Backend]
	if !ok || !step.due(d.Settings) {
		return nil, nil
	}
	settings, err := r.Settings(ctx, *d)
	if err != nil {
		return nil, err
	}
	answers := map[string]string{}
	for question, setting := range step.asked {
		answers[question] = settings[setting]
		delete(settings, setting)
	}
	section, err := draftSection(d.Backend, settings)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(filepath.Dir(r.Config), "rclone-setup-")
	if err != nil {
		return nil, fmt.Errorf("make room for rclone's setup: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	conf := filepath.Join(dir, "rclone.conf")
	if err := os.WriteFile(conf, []byte(section), 0o600); err != nil {
		return nil, fmt.Errorf("write the setup's rclone config: %w", err)
	}
	for k, v := range step.answers {
		answers[k] = v
	}
	for k, v := range picked {
		answers[k] = v
	}

	// A choice with a single way to go is taken, and the setup runs again
	// with it; each run starts over from what the last one wrote.
	for range 6 {
		q, err := r.runSetup(ctx, conf, answers)
		if err != nil {
			return nil, err
		}
		if q == nil {
			d.learn(readFile(conf), settings)
			return nil, nil
		}
		if len(q.Choices) != 1 {
			return q, nil
		}
		answers[q.Name] = q.Choices[0].Value
	}
	return nil, errors.New("rclone's setup kept asking")
}

// runSetup runs the setup once. The answers go on the command line, the only
// place rclone's setup takes them from; the one secret among them is a login
// token the run itself uses up.
func (r *Rclone) runSetup(ctx context.Context, conf string, answers map[string]string) (*Question, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	args := []string{"config", "update", DraftName}
	keys := make([]string, 0, len(answers))
	for k := range answers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, k+"="+answers[k])
	}
	out, err := r.run(ctx, []string{"RCLONE_CONFIG=" + conf}, "", append(args, "--non-interactive")...)
	if err != nil {
		return nil, err
	}
	var next configOut
	if err := json.Unmarshal(out, &next); err != nil {
		return nil, fmt.Errorf("read rclone's setup: %w", err)
	}
	switch {
	case next.Error != "":
		return nil, errors.New(firstLine(next.Error))
	case next.State == "" || next.Option == nil:
		return nil, nil
	case len(next.Option.Examples) == 0:
		return nil, fmt.Errorf("rclone's setup asks for %s, which this form cannot answer: %s", next.Option.Name, firstLine(next.Option.Help))
	}
	q := &Question{Name: next.Option.Name}
	for _, e := range next.Option.Examples {
		q.Choices = append(q.Choices, Example{Value: e.Value, Help: firstLine(e.Help)})
	}
	return q, nil
}

// draftSection renders the draft as the remote DraftName of a config file.
func draftSection(backend string, settings map[string]string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s]\ntype = %s\n", DraftName, backend)
	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.ContainsAny(k+settings[k], "\r\n") || strings.ContainsAny(k, "=[]") {
			return "", fmt.Errorf("the setting %s cannot go into an rclone config", k)
		}
		fmt.Fprintf(&b, "%s = %s\n", k, settings[k])
	}
	return b.String(), nil
}

// UnpackAuthorize replaces a token field that holds what `rclone authorize`
// prints when it is given a config blob, with the settings that blob carries.
// ShareFile puts its endpoint there and Zoho its region, next to the token.
// A setting the form already has is kept.
func UnpackAuthorize(settings map[string]string) {
	raw := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, settings["token"])
	if raw == "" || strings.HasPrefix(raw, "{") {
		return
	}
	decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(raw, "="))
	if err != nil {
		return
	}
	var blob map[string]string
	if json.Unmarshal(decoded, &blob) != nil || blob["token"] == "" {
		return
	}
	for k, v := range blob {
		if k == "token" || strings.TrimSpace(settings[k]) == "" {
			settings[k] = v
		}
	}
}
