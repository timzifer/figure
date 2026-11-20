package geom_test

import (
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/timzifer/figure/internal/markprops"
)

var update = flag.Bool("update", false, "rewrite the option matrix in the package documentation")

const (
	matrixHeading = "// # Which options a mark reads"
	matrixWidth   = 78 // the width of a doc comment line, tab included
)

// declaredMarks is every Mark constant in describe.go, by its constant name
// without the prefix — which is also its constructor's name — and its value.
func declaredMarks(t *testing.T) map[string]string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "describe.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
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
			for i, name := range vs.Names {
				v, err := strconv.Unquote(vs.Values[i].(*ast.BasicLit).Value)
				if err != nil {
					t.Fatal(err)
				}
				out[strings.TrimPrefix(name.Name, "Mark")] = v
			}
		}
	}
	return out
}

// exported lists the package's exported functions whose result is the named
// type: the constructors of Geom and of Option.
func exported(t *testing.T, result string) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || !fd.Name.IsExported() || fd.Type.Results == nil {
				continue
			}
			if r := fd.Type.Results.List; len(r) == 1 {
				if id, ok := r[0].Type.(*ast.Ident); ok && id.Name == result {
					out = append(out, fd.Name.Name)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// packageDoc is geom.go's doc comment, the part before the package clause.
func packageDoc(t *testing.T) (src, doc string) {
	t.Helper()
	b, err := os.ReadFile("geom.go")
	if err != nil {
		t.Fatal(err)
	}
	src = string(b)
	end := strings.Index(src, "\npackage geom\n")
	if end < 0 {
		t.Fatal("geom.go has no package clause")
	}
	return src, src[:end]
}

// sectionNames are the [Name] links of the doc section that starts with the
// given heading, up to the next heading.
func sectionNames(t *testing.T, doc, heading string) []string {
	t.Helper()
	start := strings.Index(doc, "// # "+heading+"\n")
	if start < 0 {
		t.Fatalf("the package doc has no %q section", heading)
	}
	rest := doc[start+len(heading)+6:]
	if next := strings.Index(rest, "// # "); next >= 0 {
		rest = rest[:next]
	}
	var out []string
	for _, m := range regexp.MustCompile(`\[([A-Z][A-Za-z0-9]*)\]`).FindAllStringSubmatch(rest, -1) {
		out = append(out, m[1])
	}
	return out
}

// Every mark and every option has a place in the guide, exactly once, so the
// page is a map of the package rather than a selection from it.
func TestThePackageDocNamesEveryMarkAndOption(t *testing.T) {
	_, doc := packageDoc(t)
	for _, tc := range []struct{ heading, result string }{
		{"Marks", "Geom"},
		{"Options", "Option"},
	} {
		listed := sectionNames(t, doc, tc.heading)
		want := exported(t, tc.result)
		count := map[string]int{}
		for _, n := range listed {
			count[n]++
		}
		for _, n := range want {
			if count[n] != 1 {
				t.Errorf("%s: %s is listed %d times, want once", tc.heading, n, count[n])
			}
		}
		for n := range count {
			if sort.SearchStrings(want, n) == len(want) || want[sort.SearchStrings(want, n)] != n {
				t.Errorf("%s: lists %s, which is not an exported constructor of %s", tc.heading, n, tc.result)
			}
		}
	}
}

// The table every reader of the matrix relies on must cover every built-in
// mark: a mark missing from it is a row missing from the documentation.
func TestTheMarkTableCoversEveryBuiltInMark(t *testing.T) {
	marks := declaredMarks(t)
	for name, value := range marks {
		if _, ok := markprops.Reads[value]; !ok {
			t.Errorf("internal/markprops has no row for %s (%q)", name, value)
		}
	}
	if len(markprops.Reads) != len(marks) {
		t.Errorf("internal/markprops has %d rows for %d built-in marks", len(markprops.Reads), len(marks))
	}
}

// matrix renders the table as the doc comment's code block: one entry per
// mark, under its constructor's name, options wrapped under the first.
func matrix(t *testing.T) []string {
	marks := declaredMarks(t)
	names := make([]string, 0, len(marks))
	width := 0
	for name := range marks {
		names = append(names, name)
		width = max(width, len(name))
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	var lines []string
	for _, name := range names {
		reads := markprops.Reads[marks[name]]
		if len(reads) == 0 {
			reads = []string{"(none)"}
		}
		line := "//\t" + name + strings.Repeat(" ", width-len(name)+2)
		indent := "//\t" + strings.Repeat(" ", width+2)
		col := 8 + width + 2 // the tab renders as eight columns
		for i, opt := range reads {
			if i > 0 && col+1+len(opt) > matrixWidth {
				lines = append(lines, line)
				line, col = indent, 8+width+2
			} else if i > 0 {
				line += " "
				col++
			}
			line += opt
			col += len(opt)
		}
		lines = append(lines, line)
	}
	return lines
}

// The matrix in the package doc is generated from internal/markprops, which
// spec's tests hold to what the encoder writes. A stale one is documentation
// of a different library, which is why this fails rather than warns.
func TestTheOptionMatrixIsCurrent(t *testing.T) {
	src, doc := packageDoc(t)
	at := strings.Index(doc, matrixHeading+"\n")
	if at < 0 {
		t.Fatalf("the package doc has no %q section", matrixHeading)
	}
	lines := strings.Split(doc[at:], "\n")
	first := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "//\t") {
			first = i
			break
		}
	}
	if first < 0 {
		t.Fatal("the matrix section has no code block")
	}
	last := first
	for last < len(lines) && strings.HasPrefix(lines[last], "//\t") {
		last++
	}
	have := lines[first:last]
	want := matrix(t)
	if strings.Join(have, "\n") == strings.Join(want, "\n") {
		return
	}
	if !*update {
		t.Fatalf("the option matrix in geom.go is stale; run go test ./geom -run %s -update", t.Name())
	}
	fixed := append(append(append([]string{}, lines[:first]...), want...), lines[last:]...)
	out := src[:at] + strings.Join(fixed, "\n") + src[len(doc):]
	if err := os.WriteFile("geom.go", []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
}
