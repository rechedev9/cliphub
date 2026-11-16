package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const demoinfocsImport = "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

// demoParserHelper is the one production file that may build a parser outside demosafe.NewParser.
const demoParserHelper = "internal/demosafe/demosafe.go"

// TestDemoParsersOpenThroughDemosafe fails when production code builds a demoinfocs parser
// directly: the constructor panics on a read error, and only demosafe.NewParser recovers it.
func TestDemoParsersOpenThroughDemosafe(t *testing.T) {
	root := repoRoot(t)
	list := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "*.go")
	list.Dir = root
	out, err := list.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var problems []string
	for _, rel := range strings.Split(string(out), "\x00") {
		if rel == "" || rel == demoParserHelper || strings.HasSuffix(rel, "_test.go") || strings.Contains(rel, "testdata/") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !bytes.Contains(data, []byte(strconv.Quote(demoinfocsImport))) {
			continue
		}
		problems = append(problems, directParserCalls(t, rel, data)...)
	}
	if len(problems) > 0 {
		t.Fatalf("build demo parsers with demosafe.NewParser, which returns a read error instead of panicking:\n%s", strings.Join(problems, "\n"))
	}
}

func directParserCalls(t *testing.T, rel string, data []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, data, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	name := ""
	for _, spec := range f.Imports {
		if path, _ := strconv.Unquote(spec.Path.Value); path == demoinfocsImport {
			name = "demoinfocs"
			if spec.Name != nil {
				name = spec.Name.Name
			}
		}
	}
	var problems []string
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "NewParser" && sel.Sel.Name != "NewParserWithConfig") {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == name {
			problems = append(problems, fset.Position(sel.Pos()).String()+": "+name+"."+sel.Sel.Name)
		}
		return true
	})
	return problems
}
