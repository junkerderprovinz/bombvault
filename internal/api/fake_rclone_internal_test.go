package api

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/remotes"
)

// fakeRcloneEnv makes the test binary answer as rclone instead of running the
// tests, so a test can reach the code that calls the binary on a host without
// one. TestMain looks for it.
const fakeRcloneEnv = "BOMBVAULT_TEST_FAKE_RCLONE"

// useFakeRclone points the service at the test binary as its rclone.
func useFakeRclone(t *testing.T, s *Service) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(fakeRcloneEnv, "1")
	s.rcloneOnce.Do(func() { s.rclone = &remotes.Rclone{Bin: exe, Config: s.rcloneConfPath()} })
}

// runFakeRclone answers the calls the service makes. It describes the
// backends of rclone 1.75.1, obscures a password by putting "obscured-" in
// front of it, and lists no folders. A remote whose host is "down" refuses the
// listing. Like rclone it writes back what a listing refreshes: a token that
// starts with "rotate-" is replaced by a new one, and a Mega sign-in without
// a session opens one. `config update` plays the setup of OneDrive,
// Jottacloud and Zoho.
func runFakeRclone(args []string) int {
	switch {
	case len(args) >= 2 && args[0] == "config" && args[1] == "providers":
		raw, err := os.ReadFile("../remotes/testdata/rclone-1.75.1-providers.json")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		_, _ = os.Stdout.Write(raw)
	case len(args) >= 1 && args[0] == "obscure":
		plain, _ := io.ReadAll(os.Stdin)
		fmt.Print("obscured-" + string(plain))
	case len(args) >= 1 && args[0] == "lsjson":
		remote, dir, _ := strings.Cut(args[len(args)-1], ":")
		options := fakeRemoteOptions(remote)
		if options["host"] == "down" {
			fmt.Fprintln(os.Stderr, "2026/01/01 00:00:00 ERROR : error listing: connection refused")
			return 1
		}
		if token, ok := strings.CutPrefix(options["token"], "rotate-"); ok {
			fakeRcloneSets(remote, map[string]string{"token": "rotated-" + token})
		}
		if options["type"] == "mega" && options["session_id"] == "" {
			fakeRcloneSets(remote, map[string]string{"session_id": "session-of-" + options["user"], "master_key": "key"})
		}
		// The sign-in comes first, so a folder that is not there fails after
		// a refresh.
		if dir == "missing" {
			fmt.Fprintln(os.Stderr, "2026/01/01 00:00:00 ERROR : error listing: directory not found")
			return 1
		}
		fmt.Print("[]")
	case len(args) >= 3 && args[0] == "config" && args[1] == "update":
		return fakeRcloneSetup(args[2], args[3:])
	default:
		fmt.Fprintln(os.Stderr, "the fake rclone does not know:", args)
		return 2
	}
	return 0
}

// fakeRemoteOptions are a remote's settings as rclone sees them: the config
// file, with the environment over it.
func fakeRemoteOptions(remote string) map[string]string {
	conf, _ := os.ReadFile(os.Getenv("RCLONE_CONFIG")) //nolint:gosec // G703: the config the test wrote
	options, _ := remoteOptions(string(conf), remote)
	if options == nil {
		options = map[string]string{}
	}
	options["type"] = remoteType(string(conf), remote)
	prefix := "RCLONE_CONFIG_" + strings.ToUpper(remote) + "_"
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(k, prefix) {
			options[strings.ToLower(strings.TrimPrefix(k, prefix))] = v
		}
	}
	return options
}

// fakeRcloneSets writes values into one remote of the config file, as rclone
// does when it saves a refreshed sign-in.
func fakeRcloneSets(remote string, values map[string]string) {
	path := os.Getenv("RCLONE_CONFIG")
	raw, _ := os.ReadFile(path) //nolint:gosec // G703: the config the test wrote
	text := "[" + remote + "]\n"
	for _, b := range splitRcloneSections(string(raw)) {
		if b.name == remote {
			text = b.text
		}
	}
	for k, v := range values {
		text = setSectionOption(text, k, v)
	}
	_ = os.WriteFile(path, []byte(replaceRcloneSection(string(raw), remote, text)), 0o600) //nolint:gosec // G703: as above
}

// fakeRcloneSetup plays rclone's setup of a remote: it asks for the OneDrive
// drive among two, trades a Jottacloud login token, and picks the only Zoho
// workspace there is.
func fakeRcloneSetup(remote string, args []string) int {
	answers := map[string]string{}
	for _, a := range args {
		if k, v, ok := strings.Cut(a, "="); ok {
			answers[k] = v
		}
	}
	options := fakeRemoteOptions(remote)
	ask := func(name string, choices ...string) int {
		var examples []string
		for _, c := range choices {
			examples = append(examples, fmt.Sprintf(`{"Value":%q,"Help":"drive %s"}`, c, c))
		}
		fmt.Printf(`{"State":"next","Option":{"Name":%q,"Help":"Pick one","Examples":[%s]}}`, name, strings.Join(examples, ","))
		return 0
	}
	switch options["type"] {
	case "onedrive":
		if answers["config_refresh_token"] != "false" {
			fmt.Fprintln(os.Stderr, "the setup would sign in again in a browser")
			return 1
		}
		if answers["config_driveid"] == "" {
			return ask("config_driveid", "personal-drive", "team-drive")
		}
		fakeRcloneSets(remote, map[string]string{"drive_id": answers["config_driveid"], "drive_type": "business"})
	case "jottacloud":
		if options["token"] != "" || answers["config_login_token"] == "" {
			fmt.Fprintln(os.Stderr, "the login token has to answer the setup's question")
			return 1
		}
		fakeRcloneSets(remote, map[string]string{
			"token": `{"access_token":"for-` + answers["config_login_token"] + `"}`, "configVersion": "1", "client_id": "jottacli",
		})
	case "zoho":
		if answers["config_workspace"] == "" {
			return ask("config_workspace", "only-workspace")
		}
		fakeRcloneSets(remote, map[string]string{"root_folder_id": answers["config_workspace"]})
	}
	fmt.Print(`{"State":""}`)
	return 0
}
