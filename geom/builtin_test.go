package geom_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/timzifer/figure/geom"
)

// builtinMarksDeclared reads every `MarkX Mark = "..."` constant out of
// describe.go. The constants are not enumerable at run time, and a list in the
// test would drift exactly the way the one in the package did: half the marks
// had been added to describe.go and never to builtin.
func builtinMarksDeclared(t *testing.T) []geom.Mark {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "describe.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var marks []geom.Mark
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "Mark" {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("a Mark constant that is not a string literal: %T", v)
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				marks = append(marks, geom.Mark(s))
			}
		}
	}
	if len(marks) < 40 {
		t.Fatalf("found only %d Mark constants in describe.go; the parse is reading the wrong thing", len(marks))
	}
	return marks
}

// Register is documented to panic for a built-in mark. A built-in that it let
// through would be shadowed for every JSON spec in the process.
func TestEveryBuiltInMarkIsRefusedByRegister(t *testing.T) {
	builder := func(geom.Desc) (geom.Geom, error) { return nil, nil }
	for _, m := range builtinMarksDeclared(t) {
		t.Run(string(m), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("Register(%q) did not panic, so a plug-in can replace a built-in mark", m)
				}
			}()
			geom.Register(m, builder)
		})
	}
}
