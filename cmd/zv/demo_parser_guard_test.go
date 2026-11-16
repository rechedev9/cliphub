package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const demoinfocsImport = "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

// demoParserHelper is the one production file that may build a parser outside demosafe.NewParser.
const demoParserHelper = "internal/demosafe/demosafe.go"

var demoParserConstructors = map[string]bool{"NewParser": true, "NewParserWithConfig": true}

// TestDemoParsersOpenThroughDemosafe fails when production code builds a demoinfocs parser
// directly: the constructor panics on a read error, and only demosafe.NewParser recovers it.
func TestDemoParsersOpenThroughDemosafe(t *testing.T) {
	root := repoRoot(t)
	var problems []string
	for _, rel := range gitGoFiles(t, goFileListing{}) {
		if rel == demoParserHelper || strings.HasSuffix(rel, "_test.go") || strings.Contains(rel, "testdata/") {
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
		calls, err := directParserCalls(rel, data)
		if err != nil {
			t.Fatal(err)
		}
		problems = append(problems, calls...)
	}
	if len(problems) > 0 {
		t.Fatalf("build demo parsers with demosafe.NewParser, which returns a read error instead of panicking:\n%s", strings.Join(problems, "\n"))
	}
}

// directParserCalls lists the demoinfocs parser constructors the file at rel names, qualified or dot-imported.
func directParserCalls(rel string, data []byte) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, data, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", rel, err)
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
	if name == "." {
		// A dot-imported name stays unresolved, unlike a local one of the same name.
		for _, id := range f.Unresolved {
			if demoParserConstructors[id.Name] {
				problems = append(problems, fset.Position(id.Pos()).String()+": "+id.Name)
			}
		}
		return problems, nil
	}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || !demoParserConstructors[sel.Sel.Name] {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == name {
			problems = append(problems, fset.Position(sel.Pos()).String()+": "+name+"."+sel.Sel.Name)
		}
		return true
	})
	return problems, nil
}
