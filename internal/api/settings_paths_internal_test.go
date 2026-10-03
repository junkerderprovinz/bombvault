package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// settingsPages are the English names of the pages in the Settings column,
// as web/src/pages/settings/settingsPages.tsx lists them.
var settingsPages = map[string]bool{
	"General": true, "Look": true, "Storage": true, "Retention": true,
	"Schedules": true, "Containers": true, "Off-site": true, "Cloud access": true,
	"Notifications": true, "Integrity": true, "Security": true, "Pairing": true,
	"Integrations": true, "System": true,
}

// cardPages names the page each card a message points at lives on.
var cardPages = map[string]string{
	"MCP server":        "Integrations",
	"API tokens":        "Integrations",
	"Home Assistant":    "Integrations",
	"Host SSH":          "Integrations",
	"Repositories":      "Storage",
	"Backup paths":      "Storage",
	"Backup Everything": "Schedules",
}

var settingsPath = regexp.MustCompile(`Settings(?:, | > )(Cloud access|[A-Z][A-Za-z-]+)(?:(?:, | > )([A-Za-z][A-Za-z ]*[A-Za-z]))?`)

// A message that sends somebody to a page the card is not on costs them a
// search through all fourteen.
func TestMessagesNameTheSettingsPageOfTheirCard(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	var checked int
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "web", "node_modules", "testdata", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			for _, m := range settingsPath.FindAllStringSubmatch(s, -1) {
				checked++
				page, card := m[1], m[2]
				if strings.Contains(m[0], " > ") {
					t.Errorf("%s: %q, write it with commas as the web interface and the docs do", fset.Position(lit.Pos()), m[0])
				}
				if !settingsPages[page] {
					t.Errorf("%s: %q names %q, which is no page in Settings", fset.Position(lit.Pos()), m[0], page)
					continue
				}
				for name, want := range cardPages {
					if strings.HasPrefix(card, name) && page != want {
						t.Errorf("%s: %q puts %s on %s, but it lives on %s", fset.Position(lit.Pos()), m[0], name, page, want)
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("found no message that names a settings page, so the pattern no longer matches how they are written")
	}
}
