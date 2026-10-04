package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// repoWrite matches a statement that writes the repo column of an item table.
var repoWrite = regexp.MustCompile(`(?is)UPDATE\s+(targets|vms|file_sets)\s+SET\b.*\brepo\s*=|INSERT\s+INTO\s+(targets|vms|file_sets)\s*\([^)]*\brepo\b`)

func TestEveryRepoWriterSetsRepoChosen(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var writers []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "migrate.go" {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			owner := declName(decl)
			ast.Inspect(decl, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				sql, err := strconv.Unquote(lit.Value)
				if err != nil || !repoWrite.MatchString(sql) {
					return true
				}
				if !strings.Contains(sql, "repo_chosen") {
					t.Errorf("%s at %s writes repo without repo_chosen", owner, fset.Position(lit.Pos()))
				}
				if !slices.Contains(writers, owner) {
					writers = append(writers, owner)
				}
				return true
			})
		}
	}
	slices.Sort(writers)
	want := []string{"CreateFileSet", "UpsertTarget", "UpsertVMTarget", "itemHomeSQL"}
	if !slices.Equal(writers, want) {
		t.Errorf("repo writers = %v, want %v; a new writer sets repo_chosen in the same statement and joins this list", writers, want)
	}
}

func declName(d ast.Decl) string {
	switch d := d.(type) {
	case *ast.FuncDecl:
		return d.Name.Name
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			if v, ok := spec.(*ast.ValueSpec); ok && len(v.Names) > 0 {
				return v.Names[0].Name
			}
		}
	}
	return ""
}
