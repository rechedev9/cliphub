package telemetry

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The desktop maps any stage, class or span name it does not allowlist to
// "unknown", and the collector rejects values it does not allowlist, so the two
// copies must stay equal or new failures lose their labels or get dropped.
const journalSourcePath = "../../desktop/src/telemetry-journal.ts"

var tsStringLiteral = regexp.MustCompile(`'([^'\\]*)'`)

func tsStringSet(source, name string) (map[string]bool, error) {
	pattern := regexp.MustCompile(`const ` + regexp.QuoteMeta(name) + ` = new Set\(\[([^\]]*)\]\)`)
	match := pattern.FindStringSubmatch(source)
	if match == nil {
		return nil, fmt.Errorf("%s is not a literal Set in %s", name, journalSourcePath)
	}
	set := map[string]bool{}
	for _, literal := range tsStringLiteral.FindAllStringSubmatch(match[1], -1) {
		set[literal[1]] = true
	}
	return set, nil
}

func allowlistDrift(tsSource string) ([]string, error) {
	var drift []string
	for _, pair := range []struct {
		name  string
		goSet map[string]bool
	}{
		{"ALLOWED_STAGES", allowedJournalStages},
		{"ALLOWED_CLASSES", allowedJournalClasses},
		{"ALLOWED_SPAN_NAMES", allowedTaskNames},
	} {
		tsSet, err := tsStringSet(tsSource, pair.name)
		if err != nil {
			return nil, err
		}
		for value := range pair.goSet {
			if !tsSet[value] {
				drift = append(drift, pair.name+": collector only "+value)
			}
		}
		for value := range tsSet {
			if !pair.goSet[value] {
				drift = append(drift, pair.name+": desktop only "+value)
			}
		}
	}
	sort.Strings(drift)
	return drift, nil
}

func readJournalSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(journalSourcePath)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func TestDesktopJournalAllowlistsMatchCollector(t *testing.T) {
	drift, err := allowlistDrift(readJournalSource(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) > 0 {
		t.Fatalf("desktop/collector allowlist drift:\n%s", strings.Join(drift, "\n"))
	}
}

func TestAllowlistContractDetectsARemovedDesktopClass(t *testing.T) {
	source := readJournalSource(t)
	mutated := strings.Replace(source, "'capture_flake', ", "", 1)
	if mutated == source {
		t.Fatal("fixture no longer contains 'capture_flake', ")
	}
	drift, err := allowlistDrift(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(drift, "\n") != "ALLOWED_CLASSES: collector only capture_flake" {
		t.Fatalf("drift = %q", drift)
	}
}

func TestEveryObsClassIsAllowlisted(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "../obs/class.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range value.Names {
				if !strings.HasPrefix(name.Name, "Class") || i >= len(value.Values) {
					continue
				}
				literal, ok := value.Values[i].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Fatalf("obs.%s is not a string literal", name.Name)
				}
				class, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatal(err)
				}
				found++
				if !allowedJournalClasses[class] {
					t.Errorf("obs.%s = %q is not in the collector class allowlist", name.Name, class)
				}
			}
		}
	}
	if found == 0 {
		t.Fatal("no obs.Class* constants found")
	}
}
