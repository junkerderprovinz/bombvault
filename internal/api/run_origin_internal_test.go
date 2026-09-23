package api

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartRunStampsOriginAndGroupFromContext(t *testing.T) {
	st := newTestStore(t)
	s := &Service{store: st}

	ctx := WithRunGroup(WithRunOrigin(context.Background(), RunOrigin{Via: "mcp", KeyID: "k1"}), "g1")
	id, err := s.startRun(ctx, "t1", "backup")
	if err != nil {
		t.Fatalf("startRun: %v", err)
	}
	run, err := st.GetRun(id)
	if err != nil {
		t.Fatal(err)
	}
	if run.GroupID != "g1" || run.StartedVia != "mcp" || run.StartedViaKey != "k1" {
		t.Fatalf("group %q via %q key %q, want g1/mcp/k1", run.GroupID, run.StartedVia, run.StartedViaKey)
	}

	plain, err := s.startRun(context.Background(), "t1", "backup")
	if err != nil {
		t.Fatalf("startRun: %v", err)
	}
	run, err = st.GetRun(plain)
	if err != nil {
		t.Fatal(err)
	}
	if run.GroupID != "" || run.StartedVia != "" || run.StartedViaKey != "" {
		t.Fatalf("a plain context stamped group %q via %q key %q, want all empty", run.GroupID, run.StartedVia, run.StartedViaKey)
	}
}

func TestRunOriginSurvivesWithoutCancel(t *testing.T) {
	o := RunOrigin{Via: "mcp", KeyID: "k1"}
	if got := runOriginFromContext(context.WithoutCancel(WithRunOrigin(context.Background(), o))); got != o {
		t.Fatalf("origin after WithoutCancel = %+v, want %+v", got, o)
	}
	if got := runOriginFromContext(nil); got != (RunOrigin{}) { //nolint:staticcheck // SA1012: a nil ctx is exactly what this asserts
		t.Fatalf("origin of a nil context = %+v, want the zero value", got)
	}
}

// A run row started past the helper would carry neither the group nor the
// origin, and nothing else in the package would notice.
func TestRunStartsGoThroughTheOriginHelper(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, pErr := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if pErr != nil {
			t.Fatalf("parse %s: %v", name, pErr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Name.Name == "startRunWith" {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, isCall := n.(*ast.CallExpr)
				if !isCall {
					return true
				}
				sel, isSel := call.Fun.(*ast.SelectorExpr)
				if !isSel {
					return true
				}
				if sel.Sel.Name == "StartRun" || sel.Sel.Name == "StartRunWith" {
					t.Errorf("%s: %s calls %s directly; start runs through startRun/startRunWith so the group and the origin reach the INSERT",
						fset.Position(sel.Pos()), fn.Name.Name, sel.Sel.Name)
				}
				return true
			})
		}
	}
}
