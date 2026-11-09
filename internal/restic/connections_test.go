package restic

import (
	"slices"
	"strings"
	"testing"
)

func connectionsOption(args []string) (string, bool) {
	for i, a := range args[:max(len(args)-1, 0)] {
		if a == "-o" && strings.Contains(args[i+1], ".connections=") {
			return args[i+1], true
		}
	}
	return "", false
}

func TestEveryNetworkBackendGetsItsOwnConnectionsOption(t *testing.T) {
	repos := map[string]string{
		"s3:https://s3.example.com/bucket/containers": "s3.connections=12",
		"rest:https://backup.example.com:8000/me":     "rest.connections=12",
		"b2:bucket:containers":                        "b2.connections=12",
		"azure:container:/containers":                 "azure.connections=12",
		"gs:bucket:/containers":                       "gs.connections=12",
		"swift:container:/containers":                 "swift.connections=12",
		"rclone:onedrive:bombvault/containers":        "rclone.connections=12",
		"sftp:me@nas.lan:/backups/containers":         "sftp.connections=12",
		"sftp://me@nas.lan:2222//backups/containers":  "sftp.connections=12",
	}
	for repo, want := range repos {
		got, ok := connectionsOption(BackupArgs(repo, []string{"/p"}, nil, Mode{Encrypted: true, Connections: 12}))
		if !ok || got != want {
			t.Errorf("%s: got %q, want %q", repo, got, want)
		}
	}
}

func TestALocalRepositoryGetsNoConnectionsOption(t *testing.T) {
	for _, repo := range []string{"/mnt/user/backups/containers", `C:\backups\containers`, "local:/backups"} {
		if got, ok := connectionsOption(BackupArgs(repo, []string{"/p"}, nil, Mode{Encrypted: true, Connections: 12})); ok {
			t.Errorf("%s: got %q, want no option", repo, got)
		}
	}
}

func TestNoConnectionsLeavesTheArgvAsItWas(t *testing.T) {
	repo := "s3:https://s3.example.com/bucket/containers"
	got := CheckArgs(repo, Mode{Encrypted: true})
	if slices.Contains(got, "-o") {
		t.Fatalf("an unset count must add nothing, got %v", got)
	}
}

func TestConnectionsReachEveryCommandThatOpensTheRepository(t *testing.T) {
	repo := "s3:https://s3.example.com/bucket/containers"
	m := Mode{Encrypted: true, Connections: 8}
	commands := map[string][]string{
		"init":     InitArgs(repo, m),
		"backup":   BackupArgs(repo, []string{"/p"}, nil, m),
		"copy":     CopyArgs(repo, "/mnt/user/backups/containers", nil, Limits{}, m),
		"check":    CheckArgs(repo, m),
		"read":     CheckDataArgs(repo, 5, m),
		"forget":   ForgetPolicyArgs(repo, RetentionPolicy{KeepLast: 3}, m, nil, true),
		"prune":    PruneArgs(repo, m),
		"restore":  RestoreAllArgs(repo, "abc123", "/restore", m),
		"snapshot": SnapshotsArgs(repo, m),
		"ls":       LsArgs(repo, "abc123", m),
		"find":     FindArgs(repo, []string{"abc123"}, "notes", m),
		"unlock":   UnlockArgs(repo, false, m),
	}
	for name, args := range commands {
		got, ok := connectionsOption(args)
		if !ok || got != "s3.connections=8" {
			t.Errorf("%s: got %q in %v", name, got, args)
			continue
		}
		at := slices.Index(args, "s3.connections=8")
		if sub := subcommand(args); slices.Index(args, sub) < at {
			t.Errorf("%s: the option must stand before the subcommand %q, got %v", name, sub, args)
		}
	}
}

func TestConnectionsStandBesideTheStorageClass(t *testing.T) {
	repo := "s3:https://s3.example.com/bucket/containers"
	got := InitArgs(repo, Mode{Encrypted: true, Connections: 4, StorageClass: "STANDARD_IA"})
	want := []string{"-r", repo, "-o", "s3.connections=4", "-o", "s3.storage-class=STANDARD_IA", "init"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestConnectionsInRange(t *testing.T) {
	for _, n := range []int{0, 1, MaxConnections} {
		if err := CheckConnections(n); err != nil {
			t.Errorf("%d: %v", n, err)
		}
	}
	for _, n := range []int{-1, MaxConnections + 1, 1000} {
		if err := CheckConnections(n); err == nil {
			t.Errorf("%d must be refused", n)
		}
	}
}
