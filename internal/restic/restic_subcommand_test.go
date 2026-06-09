package restic

import "testing"

func TestSubcommand(t *testing.T) {
	t.Run("backup after -r repo", func(t *testing.T) {
		got := subcommand([]string{"-r", "/repo", "backup", "--json", "--", "/src"})
		if got != "backup" {
			t.Fatalf("got %q want %q", got, "backup")
		}
	})
	t.Run("restore after -r repo", func(t *testing.T) {
		got := subcommand([]string{"-r", "/repo", "restore", "--target", "/out", "--", "abc123"})
		if got != "restore" {
			t.Fatalf("got %q want %q", got, "restore")
		}
	})
	t.Run("all flags no positional falls back to unknown", func(t *testing.T) {
		// An argv with only flags and no positional word returns the documented
		// fallback so error messages still make sense.
		got := subcommand([]string{"-r", "/repo", "--json", "--insecure-no-password"})
		if got != "unknown" {
			t.Fatalf("got %q want %q", got, "unknown")
		}
	})
}

func TestLastReason(t *testing.T) {
	t.Run("picks last non-empty line", func(t *testing.T) {
		got := lastReason("starting restore\n\nFatal: something broke\n")
		if got != "Fatal: something broke" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("scrubs absolute paths", func(t *testing.T) {
		got := lastReason("Fatal: unable to open repository at /host/user/backups/containers")
		if got != "Fatal: unable to open repository at [path]" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("empty stderr yields empty reason", func(t *testing.T) {
		if got := lastReason("   \n\n"); got != "" {
			t.Fatalf("got %q", got)
		}
	})
}
