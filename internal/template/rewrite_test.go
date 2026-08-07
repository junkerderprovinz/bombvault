package template_test

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/template"
)

// TestRewriteHostPaths pins the #125 container template remap: only <Config>
// element text (and Default attrs) that EXACTLY match a remap key are rewritten;
// env Variables, container-side Target paths and unrelated paths are untouched,
// and an empty remap is the identity.
func TestRewriteHostPaths(t *testing.T) {
	xml := `<Container>` +
		`<Config Name="Appdata" Target="/config" Default="" Mode="rw" Type="Path" Display="always">/mnt/zfs/appdata/xo</Config>` +
		`<Config Name="Media" Target="/media" Default="" Mode="rw" Type="Path" Display="always">/mnt/user/media</Config>` +
		`<Config Name="GAME_ID" Target="GAME_ID" Default="/mnt/zfs/appdata/xo" Mode="" Type="Variable" Display="always">294420</Config>` +
		`</Container>`

	out := template.RewriteHostPaths(xml, map[string]string{"/mnt/zfs/appdata/xo": "/mnt/user/appdata/xo"})

	// The appdata Config's host path text is rewritten.
	if !strings.Contains(out, ">/mnt/user/appdata/xo</Config>") {
		t.Fatalf("appdata host path not rewritten: %s", out)
	}
	if strings.Contains(out, ">/mnt/zfs/appdata/xo</Config>") {
		t.Fatalf("old appdata host path still present: %s", out)
	}
	// The media Config (not in the remap) is untouched.
	if !strings.Contains(out, ">/mnt/user/media</Config>") {
		t.Fatalf("unrelated media path must be untouched: %s", out)
	}
	// The Variable's Default attribute happens to equal the old host path -> also
	// rewritten (exact match), but its Target="GAME_ID" and value 294420 are not.
	if !strings.Contains(out, `Default="/mnt/user/appdata/xo"`) {
		t.Fatalf("matching Default attr should be rewritten: %s", out)
	}
	if !strings.Contains(out, ">294420</Config>") {
		t.Fatalf("env Variable value must be untouched: %s", out)
	}

	// Empty remap is the identity.
	if template.RewriteHostPaths(xml, nil) != xml {
		t.Fatal("empty remap must return xml unchanged")
	}

	// A non-canonical template host path (trailing slash) is canonicalized before
	// the lookup, so it is still rewritten (symmetry with rewriteBinds).
	slashXML := `<Config Name="Appdata" Target="/config" Type="Path">/mnt/zfs/appdata/xo/</Config>`
	slashOut := template.RewriteHostPaths(slashXML, map[string]string{"/mnt/zfs/appdata/xo": "/mnt/user/appdata/xo"})
	if !strings.Contains(slashOut, ">/mnt/user/appdata/xo</Config>") {
		t.Fatalf("trailing-slash template path must be rewritten: %s", slashOut)
	}
}
