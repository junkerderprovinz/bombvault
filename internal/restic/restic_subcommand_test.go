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
