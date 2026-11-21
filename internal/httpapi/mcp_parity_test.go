package httpapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/store"
)

var mcpSourceDirs = []string{"../../cmd/zv-mcp", "../tuiclient"}

var (
	routeParam   = regexp.MustCompile(`^\{[^}]+\}$`)
	literalSplit = regexp.MustCompile(`[/?=&.]`)
)

func mcpPathWords(t *testing.T) map[string]bool {
	t.Helper()
	words := map[string]bool{}
	fset := token.NewFileSet()
	for _, dir := range mcpSourceDirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no Go files in %s: %v", dir, err)
		}
		for _, name := range files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				if value, err := strconv.Unquote(lit.Value); err == nil {
					for _, word := range literalSplit.Split(value, -1) {
						words[word] = true
					}
				}
				return true
			})
		}
	}
	return words
}

func TestEveryRouteHasAnMCPTool(t *testing.T) {
	words := mcpPathWords(t)
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{},
		WithLibrary(store.NewMemoryLibraryRepository()), WithLibraryUploads(""), WithSyncHub(store.NewSyncLog(nil)))
	served := map[Route]bool{}
	var problems []string
	err := chi.Walk(Routes(h), func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method == http.MethodHead || !strings.HasPrefix(pattern, "/api/") {
			return nil
		}
		r := Route{method, pattern}
		served[r] = true
		_, exempt := routesWithoutMCPTool[r]
		missing := ""
		for _, seg := range strings.Split(pattern, "/") {
			if seg != "" && !routeParam.MatchString(seg) && !words[seg] {
				missing = seg
				break
			}
		}
		switch {
		case missing != "" && !exempt:
			problems = append(problems, method+" "+pattern+": no MCP code names \""+missing+"\". Add its tool in cmd/zv-mcp (.claude/rules/mcp-parity.md), or list it in routesWithoutMCPTool with the reason")
		case missing == "" && exempt:
			problems = append(problems, method+" "+pattern+": MCP code now reaches it, remove it from routesWithoutMCPTool")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for r := range routesWithoutMCPTool {
		if !served[r] {
			problems = append(problems, r.Method+" "+r.Pattern+" is not registered: remove it from routesWithoutMCPTool")
		}
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}
