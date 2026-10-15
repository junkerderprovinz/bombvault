package virshcli

import "testing"

func TestRewriteBlockSourcesMovesOnlyTheMappedZvols(t *testing.T) {
	xml := `<disk type='block'><source dev='/dev/zvol/tank/vms/a'/></disk>` +
		`<disk type='block'><source dev="/dev/zvol/tank/vms/b"/></disk>` +
		`<disk type='file'><source file='/dev/zvol/tank/vms/a'/></disk>`
	got := RewriteBlockSources(xml, map[string]string{"/dev/zvol/tank/vms/a": "/dev/zvol/flash/vms/a"})
	want := `<disk type='block'><source dev='/dev/zvol/flash/vms/a'/></disk>` +
		`<disk type='block'><source dev="/dev/zvol/tank/vms/b"/></disk>` +
		`<disk type='file'><source file='/dev/zvol/tank/vms/a'/></disk>`
	if got != want {
		t.Fatalf("RewriteBlockSources =\n%s\nwant\n%s", got, want)
	}
}
