package api

import "testing"

func TestScrubSecretsLeavesTheSentenceAroundAPath(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			name: "zvol-only export refusal",
			in:   `export vm: "win11" has only zvol disks (/dev/zvol/tank/vms/win11-disk0). They are included in its backups but cannot be exported as files. Restore the VM from a backup instead`,
			want: `export vm: "win11" has only zvol disks ([path]). They are included in its backups but cannot be exported as files. Restore the VM from a backup instead`,
		},
		{
			name: "two paths in parentheses",
			in:   `has only zvol disks (/dev/zvol/tank/a, /dev/zvol/tank/b).`,
			want: `has only zvol disks ([path], [path]).`,
		},
		{
			name: "path ends the sentence",
			in:   `cannot read /mnt/user/appdata/plex. Check the mount`,
			want: `cannot read [path]. Check the mount`,
		},
		{
			name: "path before a semicolon",
			in:   `missing /config/db.sqlite; using defaults`,
			want: `missing [path]; using defaults`,
		},
		{
			name: "path before a colon",
			in:   `open /mnt/cache/x: permission denied`,
			want: `open [path]: permission denied`,
		},
		{
			name: "quoted path",
			in:   `folder "/mnt/user/backups" is not empty`,
			want: `folder "[path]" is not empty`,
		},
		{
			name: "dots inside a path are part of it",
			in:   `wrote /tmp/bv.export.tar.gz.`,
			want: `wrote [path].`,
		},
		{
			name: "trailing slash stays in the path",
			in:   `under /mnt/user/ only`,
			want: `under [path] only`,
		},
		{
			name: "url in parentheses keeps host and the closing punctuation",
			in:   `could not reach the endpoint (http://ntfy.lan:8080/alerts).`,
			want: `could not reach the endpoint (http://ntfy.lan:8080/alerts).`,
		},
		{
			name: "url query is redacted without eating the sentence",
			in:   `post to https://hooks.example.com/x?token=abc123, then retry`,
			want: `post to https://hooks.example.com/x?[redacted], then retry`,
		},
		{
			name: "url with a path keeps it",
			in:   `dial http://ntfy.lan:8080/alerts failed, see /config/bombvault.sqlite`,
			want: `dial http://ntfy.lan:8080/alerts failed, see [path]`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scrubSecrets(tc.in); got != tc.want {
				t.Fatalf("scrubSecrets(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestScrubErrorKeepsTheZvolExportRefusalReadable(t *testing.T) {
	err := checkVMExportable("win11", 0, []string{"/dev/zvol/tank/vms/win11-disk0"})
	want := `export vm: "win11" has only zvol disks ([path]). They are included in its backups but cannot be exported as files. Restore the VM from a backup instead`
	if got := scrubError(err); got != want {
		t.Fatalf("scrubError = %q, want %q", got, want)
	}
}
