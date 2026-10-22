package restic

import (
	"slices"
	"testing"
)

// Aging a capped repository downloads and repacks over the same line its
// backups and copies are capped on.
func TestForgetAndPruneKeepToTheBandwidthCaps(t *testing.T) {
	m := Mode{Encrypted: true, Limits: Limits{UploadKBps: 500, DownloadKBps: 800}}
	caps := []string{"--limit-upload", "500", "--limit-download", "800"}
	for name, args := range map[string][]string{
		"forget":        ForgetArgs("rest:http://box/repo", []string{"abc"}, true, m),
		"forget policy": ForgetPolicyArgs("rest:http://box/repo", RetentionPolicy{KeepLast: 3}, m, nil, true),
		"prune":         PruneArgs("rest:http://box/repo", m),
	} {
		at := slices.Index(args, "--limit-upload")
		verb := slices.IndexFunc(args, func(a string) bool { return a == "forget" || a == "prune" })
		if at < 0 || !slices.Equal(args[at:at+len(caps)], caps) || at > verb {
			t.Errorf("%s = %v, want %v before the subcommand", name, args, caps)
		}
	}
	if args := PruneArgs("/repo", Mode{Encrypted: true}); slices.Contains(args, "--limit-upload") {
		t.Errorf("prune without caps = %v", args)
	}
}
