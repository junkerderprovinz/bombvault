package virshcli

import (
	"fmt"
	"testing"
)

func TestParseAutostartReadsDominfo(t *testing.T) {
	const dominfo = `Id:             3
Name:           Windows 11
UUID:           0f6cf1a7-6a25-4d3b-9a1f-2f4f5b0c6d11
OS Type:        hvm
State:          running
Persistent:     yes
Autostart:      %s
Managed save:   no
`
	for value, want := range map[string]bool{"enable": true, "disable": false} {
		got, err := parseAutostart(fmt.Sprintf(dominfo, value))
		if err != nil || got != want {
			t.Errorf("Autostart: %s gives %v, %v; want %v", value, got, err, want)
		}
	}
	if _, err := parseAutostart("Name: x\n"); err == nil {
		t.Error("dominfo without an autostart line must be an error, not off")
	}
}
