package recapplan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Code that only guards options, never acts on them.
var optionGuards = map[string]bool{"CanonicalNewOptions": true, "ValidateCurrentFullDemoPolicy": true}

// Leaves the execution gate leaves open but Validate pins to one value. Shrink
// only: an entry the render starts reading, or Validate stops pinning, fails.
var pinnedByValidate = map[string]string{
	"profile_id":           "Validate accepts only ProfileChill",
	"sponsor.music_policy": `Validate accepts only "pause-resume"`,
}

// Regression for the Studio 3.0.1 FACEIT overlay incident: source_kind was
// validated and approved but nothing read it, so the render silently ignored
// it. Every Options leaf the execution gate lets a user change must be read by
// Go code other than validation.go and the gate itself; a leaf the gate pins to
// its default is exempt because the render only ever sees that one value.
// Matching is by field name, so a generic name read elsewhere can mask a miss.
func TestEveryChangeableFullDemoOptionIsRead(t *testing.T) {
	read := readFieldNames(t, filepath.Join("..", ".."), "internal", "cmd")
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
		mutateLeaf(reflect.ValueOf(&options).Elem(), leaf.index)
		if options.ValidateCurrentFullDemoPolicy() != nil {
			continue
		}
		changeable++
		if reason, pinned := pinnedByValidate[leaf.path]; pinned {
			if options.Validate() == nil {
				t.Errorf("%s is exempt because %s, but Validate accepted a changed value", leaf.path, reason)
			}
			if read[leaf.name] {
				t.Errorf("%s is now read; remove it from pinnedByValidate", leaf.path)
			}
			continue
		}
		if !read[leaf.name] {
			t.Errorf("%s can be changed at execution but nothing outside validation reads %s; consume it or pin it in ValidateCurrentFullDemoPolicy", leaf.path, leaf.name)
		}
	}
	if changeable == 0 {
		t.Fatal("no changeable option found; the mutation walk is broken")
	}
}

type optionLeaf struct {
	path, name string
	index      []int
}

// optionLeaves lists every scalar, slice and pointer-to-scalar field reachable
// from typ, following nested and pointer structs.
func optionLeaves(typ reflect.Type, prefix string, index []int) []optionLeaf {
	var leaves []optionLeaf
	for i := range typ.NumField() {
		field := typ.Field(i)
		tag, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		path := prefix + tag
		at := append(append([]int(nil), index...), i)
		inner := field.Type
		if inner.Kind() == reflect.Pointer {
			inner = inner.Elem()
		}
		if inner.Kind() == reflect.Struct {
			leaves = append(leaves, optionLeaves(inner, path+".", at)...)
			continue
		}
		leaves = append(leaves, optionLeaf{path: path, name: field.Name, index: at})
	}
	return leaves
}

// mutateLeaf gives the field at index a different value of the same type,
// allocating any nil pointer on the way.
func mutateLeaf(v reflect.Value, index []int) {
	for _, i := range index {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
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
		panic("unhandled option kind " + v.Kind().String())
	}
}

// readFieldNames returns every selector name used by non-test Go code under
// root/dirs, excluding validation.go and the optionGuards functions.
func readFieldNames(t *testing.T, root string, dirs ...string) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	fset := token.NewFileSet()
	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			if filepath.ToSlash(path) == filepath.ToSlash(filepath.Join(root, "internal", "recapplan", "validation.go")) {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && optionGuards[fn.Name.Name] {
					continue
				}
				ast.Inspect(decl, func(n ast.Node) bool {
					if sel, ok := n.(*ast.SelectorExpr); ok {
						names[sel.Sel.Name] = true
					}
					return true
				})
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return names
}
