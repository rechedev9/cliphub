package recapplan

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Code that only guards options, never acts on them.
var optionGuards = map[string]bool{"CanonicalNewOptions": true, "ValidateCurrentFullDemoPolicy": true}

// Packages that see Options but are not the render: the web API and publish
// metadata, and the CLI that writes plans. A read there does not reach the video.
var notRender = map[string]bool{
	"github.com/rechedev9/cliphub/internal/httpapi": true,
	"github.com/rechedev9/cliphub/cmd/zv":           true,
}

// Leaves the execution gate leaves open but Validate pins to one value. Shrink
// only: an entry the render starts reading, or Validate stops pinning, fails.
var pinnedByValidate = map[string]string{
	"profile_id":           "Validate accepts only ProfileChill",
	"sponsor.music_policy": `Validate accepts only "pause-resume"`,
}

// Regression for the Studio 3.0.1 FACEIT overlay incident: source_kind was
// validated and approved but nothing read it, so the render silently ignored
// it. Every Options leaf the execution gate lets a user change must be read,
// as that exact struct field, by render code other than validation.go and the
// gate itself; assignments do not count. A leaf the gate pins to its default is
// exempt because the render only ever sees that one value.
func TestEveryChangeableFullDemoOptionIsRead(t *testing.T) {
	read := readOptionFields(t)
	defaults := DefaultOptions()
	if err := defaults.Validate(); err != nil {
		t.Fatalf("defaults must validate: %v", err)
	}
	if err := defaults.ValidateCurrentFullDemoPolicy(); err != nil {
		t.Fatalf("defaults must pass the execution gate: %v", err)
	}
	changeable := 0
	for _, leaf := range optionLeaves(reflect.TypeFor[Options](), "", nil) {
		options := DefaultOptions()
		mutateLeaf(t, reflect.ValueOf(&options).Elem(), leaf.index)
		if options.ValidateCurrentFullDemoPolicy() != nil {
			continue
		}
		changeable++
		if reason, pinned := pinnedByValidate[leaf.path]; pinned {
			if options.Validate() == nil {
				t.Errorf("%s is exempt because %s, but Validate accepted a changed value", leaf.path, reason)
			}
			if read[leaf.field] {
				t.Errorf("%s is now read; remove it from pinnedByValidate", leaf.path)
			}
			continue
		}
		if !read[leaf.field] {
			t.Errorf("%s can be changed at execution but no render code outside validation reads %s; consume it or pin it in ValidateCurrentFullDemoPolicy", leaf.path, leaf.field)
		}
	}
	if changeable == 0 {
		t.Fatal("no changeable option found; the mutation walk is broken")
	}
}

type optionLeaf struct {
	// path is the JSON path; field is "pkgpath.Struct.Field", as readOptionFields keys it.
	path, field string
	index       []int
}

// optionLeaves lists every scalar, slice and pointer-to-scalar field reachable
// from typ, following nested and pointer structs.
func optionLeaves(typ reflect.Type, prefix string, index []int) []optionLeaf {
	var leaves []optionLeaf
	for i := range typ.NumField() {
		field := typ.Field(i)
		tag, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		path := prefix + tag
		at := append(slices.Clone(index), i)
		inner := field.Type
		if inner.Kind() == reflect.Pointer {
			inner = inner.Elem()
		}
		if inner.Kind() == reflect.Struct {
			leaves = append(leaves, optionLeaves(inner, path+".", at)...)
			continue
		}
		leaves = append(leaves, optionLeaf{path: path, field: typ.PkgPath() + "." + typ.Name() + "." + field.Name, index: at})
	}
	return leaves
}

// mutateLeaf gives the field at index a different value of the same type,
// allocating any nil pointer on the way.
func mutateLeaf(t *testing.T, v reflect.Value, index []int) {
	t.Helper()
	deref := func() {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
	}
	for _, i := range index {
		deref()
		v = v.Field(i)
	}
	deref()
	switch v.Kind() {
	case reflect.String:
		v.SetString(v.String() + "-changed")
	case reflect.Bool:
		v.SetBool(!v.Bool())
	case reflect.Float64:
		v.SetFloat(v.Float() + .5)
	case reflect.Int, reflect.Int64:
		v.SetInt(v.Int() + 1)
	case reflect.Slice:
		v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
	default:
		t.Fatalf("mutateLeaf cannot change a %s option", v.Kind())
	}
}

type listedPackage struct {
	ImportPath, Dir, Export string
	GoFiles, Deps           []string
	Module                  *struct{ Path string }
}

// readOptionFields type-checks every render package that can see Options and
// returns the struct fields it reads, keyed like optionLeaf.field.
// Imports resolve from `go list -export` data, as go vet does.
func readOptionFields(t *testing.T) map[string]bool {
	t.Helper()
	self := reflect.TypeFor[Options]().PkgPath()
	list := exec.Command("go", "list", "-e", "-export", "-deps", "-json=ImportPath,Dir,Export,GoFiles,Deps,Module", "./internal/...", "./cmd/...")
	list.Dir = filepath.Join("..", "..")
	list.Stderr = os.Stderr
	out, err := list.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	exports := map[string]string{}
	var roots []listedPackage
	for decoder := json.NewDecoder(bytes.NewReader(out)); ; {
		var p listedPackage
		if err := decoder.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		exports[p.ImportPath] = p.Export
		if p.Module != nil && p.Module.Path == "github.com/rechedev9/cliphub" && !notRender[p.ImportPath] && (p.ImportPath == self || slices.Contains(p.Deps, self)) {
			roots = append(roots, p)
		}
	}

	fset := token.NewFileSet()
	imports := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) { return os.Open(exports[path]) })
	read := map[string]bool{}
	for _, p := range roots {
		var files []*ast.File
		for _, name := range p.GoFiles {
			file, err := parser.ParseFile(fset, filepath.Join(p.Dir, name), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, file)
		}
		info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
		conf := types.Config{Importer: imports}
		if _, err := conf.Check(p.ImportPath, fset, files, info); err != nil {
			t.Fatalf("type-check %s: %v", p.ImportPath, err)
		}
		for i, file := range files {
			if p.ImportPath == self && p.GoFiles[i] == "validation.go" {
				continue
			}
			collectFieldReads(file, info, read)
		}
	}
	return read
}

// collectFieldReads marks each named-struct field that file reads outside
// optionGuards; the target of an assignment or ++/-- is a write, not a read.
func collectFieldReads(file *ast.File, info *types.Info, read map[string]bool) {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && optionGuards[fn.Name.Name] {
			continue
		}
		written := map[ast.Expr]bool{}
		ast.Inspect(decl, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range n.Lhs {
					written[lhs] = true
				}
			case *ast.IncDecStmt:
				written[n.X] = true
			case *ast.SelectorExpr:
				selection := info.Selections[n]
				if selection == nil || selection.Kind() != types.FieldVal || written[n] {
					return true
				}
				recv := selection.Recv()
				if pointer, ok := recv.(*types.Pointer); ok {
					recv = pointer.Elem()
				}
				if named, ok := recv.(*types.Named); ok && named.Obj().Pkg() != nil {
					read[named.Obj().Pkg().Path()+"."+named.Obj().Name()+"."+n.Sel.Name] = true
				}
			}
			return true
		})
	}
}
