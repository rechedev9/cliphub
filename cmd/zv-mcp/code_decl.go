package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	scopeMinTools   = 10
	declOutputDepth = 4
	declNoteRunes   = 140
	declSummaryRune = 220
)

type declSchema struct {
	Type        any                    `json:"type"`
	Description string                 `json:"description"`
	Properties  map[string]*declSchema `json:"properties"`
	Items       *declSchema            `json:"items"`
	Required    []string               `json:"required"`
	Enum        []any                  `json:"enum"`
}

type declTool struct {
	Name    string
	Summary string
	Input   *declSchema
	Output  *declSchema
}

type declCatalog struct {
	Tools []declTool
	Scope map[string]*declSchema
}

func newDeclCatalog(tools []*mcp.Tool) declCatalog {
	type field struct{ name, description string }
	cat := declCatalog{Scope: map[string]*declSchema{}}
	seen := map[field]int{}
	first := map[field]*declSchema{}
	for _, t := range tools {
		d := declTool{Name: t.Name, Summary: firstSentence(t.Description), Input: toDeclSchema(t.InputSchema), Output: toDeclSchema(t.OutputSchema)}
		cat.Tools = append(cat.Tools, d)
		if d.Input == nil {
			continue
		}
		for name, prop := range d.Input.Properties {
			if slices.Contains(d.Input.Required, name) || prop == nil {
				continue
			}
			f := field{name, prop.Description}
			seen[f]++
			if first[f] == nil {
				first[f] = prop
			}
		}
	}
	best := map[string]int{}
	for f, n := range seen {
		if n >= scopeMinTools && n > best[f.name] {
			best[f.name] = n
			cat.Scope[f.name] = first[f]
		}
	}
	slices.SortFunc(cat.Tools, func(a, b declTool) int { return strings.Compare(a.Name, b.Name) })
	return cat
}

func toDeclSchema(v any) *declSchema {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var s declSchema
	if json.Unmarshal(raw, &s) != nil {
		return nil
	}
	return &s
}

func (c declCatalog) tool(name string) (declTool, bool) {
	i := slices.IndexFunc(c.Tools, func(t declTool) bool { return t.Name == name })
	if i < 0 {
		return declTool{}, false
	}
	return c.Tools[i], true
}

func (c declCatalog) declare(tools []declTool) string {
	var b strings.Builder
	usesScope := slices.ContainsFunc(tools, c.usesScope)
	if usesScope {
		b.WriteString("type Scope = {\n")
		for _, name := range sortedKeys(c.Scope) {
			writeDeclField(&b, name, c.Scope[name], false)
		}
		b.WriteString("};\n\n")
	}
	b.WriteString("declare const studio: {\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "  /** %s */\n  %s(args: {\n", t.Summary, t.Name)
		if t.Input != nil {
			for _, name := range sortedKeys(t.Input.Properties) {
				if c.inScope(t, name) {
					continue
				}
				writeDeclField(&b, name, t.Input.Properties[name], slices.Contains(t.Input.Required, name))
			}
		}
		b.WriteString("  }")
		if c.usesScope(t) {
			b.WriteString(" & Scope")
		}
		fmt.Fprintf(&b, "): Promise<%s>;\n", declType(t.Output, declOutputDepth))
	}
	b.WriteString("};\n")
	return b.String()
}

func (c declCatalog) usesScope(t declTool) bool {
	if t.Input == nil {
		return false
	}
	for name := range t.Input.Properties {
		if c.inScope(t, name) {
			return true
		}
	}
	return false
}

func (c declCatalog) inScope(t declTool, name string) bool {
	shared, prop := c.Scope[name], t.Input.Properties[name]
	return shared != nil && prop != nil && shared.Description == prop.Description && !slices.Contains(t.Input.Required, name)
}

func writeDeclField(b *strings.Builder, name string, s *declSchema, required bool) {
	optional := "?"
	if required {
		optional = ""
	}
	fmt.Fprintf(b, "    %s%s: %s;", name, optional, declType(s, 1))
	if s != nil && s.Description != "" {
		fmt.Fprintf(b, " // %s", clip(s.Description, declNoteRunes))
	}
	b.WriteString("\n")
}

func declType(s *declSchema, depth int) string {
	if s == nil {
		return "unknown"
	}
	if len(s.Enum) > 0 {
		parts := make([]string, 0, len(s.Enum))
		for _, v := range s.Enum {
			raw, err := json.Marshal(v)
			if err == nil {
				parts = append(parts, string(raw))
			}
		}
		return strings.Join(parts, " | ")
	}
	switch declKind(s) {
	case "array":
		inner := declType(s.Items, depth)
		if strings.ContainsAny(inner, " |") && !strings.HasPrefix(inner, "{") {
			inner = "(" + inner + ")"
		}
		return inner + "[]"
	case "object":
		if len(s.Properties) == 0 {
			return "Record<string, unknown>"
		}
		if depth <= 0 {
			return "object"
		}
		fields := make([]string, 0, len(s.Properties))
		for _, name := range sortedKeys(s.Properties) {
			optional := "?"
			if slices.Contains(s.Required, name) {
				optional = ""
			}
			fields = append(fields, name+optional+": "+declType(s.Properties[name], depth-1))
		}
		return "{ " + strings.Join(fields, "; ") + " }"
	case "integer", "number":
		return "number"
	case "string", "boolean", "null":
		return declKind(s)
	}
	return "unknown"
}

func declKind(s *declSchema) string {
	switch t := s.Type.(type) {
	case string:
		return t
	case []any:
		for _, v := range t {
			if name, ok := v.(string); ok && name != "null" {
				return name
			}
		}
	}
	if s.Properties != nil {
		return "object"
	}
	return ""
}

func firstSentence(text string) string {
	if i := strings.Index(text, ". "); i >= 0 {
		text = text[:i+1]
	}
	return clip(text, declSummaryRune)
}

func clip(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-1]) + "…"
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
