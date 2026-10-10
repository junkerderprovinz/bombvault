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

// runFakeRclone answers the three calls the service makes. It describes the
// backends of rclone 1.75.1, obscures a password by putting "obscured-" in
// front of it, and lists no folders. A remote whose host is "down" refuses the
// listing.
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
		remote, _, _ := strings.Cut(args[len(args)-1], ":")
		conf, _ := os.ReadFile(os.Getenv("RCLONE_CONFIG")) //nolint:gosec // G703: the config the test wrote
		options, _ := remoteOptions(string(conf), remote)
		if options["host"] == "down" || os.Getenv("RCLONE_CONFIG_"+remotes.DraftName+"_HOST") == "down" {
			fmt.Fprintln(os.Stderr, "2026/01/01 00:00:00 ERROR : error listing: connection refused")
			return 1
		}
		fmt.Print("[]")
	default:
		fmt.Fprintln(os.Stderr, "the fake rclone does not know:", args)
		return 2
	}
	return 0
}
