package template_test

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/template"
)

func TestRewriteHostPaths(t *testing.T) {
	xml := `<Container>` +
		`<Config Name="Appdata" Target="/config" Default="" Mode="rw" Type="Path" Display="always">/mnt/zfs/appdata/xo</Config>` +
		`<Config Name="Media" Target="/media" Default="" Mode="rw" Type="Path" Display="always">/mnt/user/media</Config>` +
		`<Config Name="GAME_ID" Target="GAME_ID" Default="/mnt/zfs/appdata/xo" Mode="" Type="Variable" Display="always">294420</Config>` +
		`</Container>`

	out := template.RewriteHostPaths(xml, map[string]string{"/mnt/zfs/appdata/xo": "/mnt/user/appdata/xo"})

	if !strings.Contains(out, ">/mnt/user/appdata/xo</Config>") {
		t.Fatalf("appdata host path not rewritten: %s", out)
	}
	if strings.Contains(out, ">/mnt/zfs/appdata/xo</Config>") {
		t.Fatalf("old appdata host path still present: %s", out)
	}
	if !strings.Contains(out, ">/mnt/user/media</Config>") {
		t.Fatalf("unrelated media path must be untouched: %s", out)
	}
	// The variable's Default equals the old host path and changes too; its
	// Target and value do not.
	if !strings.Contains(out, `Default="/mnt/user/appdata/xo"`) {
		t.Fatalf("matching Default attr should be rewritten: %s", out)
	}
	if !strings.Contains(out, ">294420</Config>") {
		t.Fatalf("env Variable value must be untouched: %s", out)
	}

	if template.RewriteHostPaths(xml, nil) != xml {
		t.Fatal("empty remap must return xml unchanged")
	}

	slashXML := `<Config Name="Appdata" Target="/config" Type="Path">/mnt/zfs/appdata/xo/</Config>`
	slashOut := template.RewriteHostPaths(slashXML, map[string]string{"/mnt/zfs/appdata/xo": "/mnt/user/appdata/xo"})
	if !strings.Contains(slashOut, ">/mnt/user/appdata/xo</Config>") {
		t.Fatalf("trailing-slash template path must be rewritten: %s", slashOut)
	}
}
