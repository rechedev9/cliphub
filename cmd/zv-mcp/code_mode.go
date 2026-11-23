package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	searchAPIName   = "search_api"
	runScriptName   = "run_script"
	searchExpand    = 6
	searchMoreLimit = 30
	searchNameLimit = 10
)

const refuseNotReadOnly = "it changes Studio or asks the user; call it as a normal tool"

type searchAPIIn struct {
	Query string   `json:"query,omitempty" jsonschema:"words to look for in tool names and descriptions, such as \"team grenades\"; returns the TypeScript of the best matches"`
	Names []string `json:"names,omitempty" jsonschema:"exact studio function names to declare, up to 10, instead of query"`
}

type runScriptIn struct {
	Code string `json:"code" jsonschema:"a JavaScript async arrow function that calls studio.<tool>(args) and returns only what the answer needs, such as async () => { const m = await studio.match_metrics({ match_id }); return m.players.map(p => [p.name, p.kills]) }"`
}

type runScriptOut struct {
	Result any `json:"result" jsonschema:"what the function returned"`
	Calls  int `json:"calls" jsonschema:"how many studio calls the script made"`
}

type codeBridge struct {
	server  *mcp.Server
	once    sync.Once
	err     error
	session *mcp.ClientSession
	catalog declCatalog
	refused map[string]string
}

func registerCodeTools(server *mcp.Server) {
	b := &codeBridge{server: server}
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: searchAPIName, Annotations: readOnly,
		Description: "Find the read-only Studio functions that run_script can call and get their TypeScript declarations: arguments, the shared Scope filters and the shape of what each returns. " +
			"Pass query words, or names for exact functions; with neither it lists every function with one line each."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in searchAPIIn) (*mcp.CallToolResult, any, error) {
			if err := b.open(ctx); err != nil {
				return nil, nil, err
			}
			text, err := b.search(in)
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: runScriptName, Annotations: readOnly,
		Description: "Run a JavaScript async arrow function that chains read-only Studio calls (studio.<tool>(args), declared by search_api) and returns only the result, so large analysis answers never enter the conversation. " +
			"Use it to combine or filter several analysis calls, such as a rival's scouting, a match review or the best moments of a job. " +
			"Only read-only tools exist under studio; imports, captures, renders, edits and deletes stay normal tool calls. " +
			"Limits: 60 s, 40 calls, a 64 KB result; there is no file, network or console access."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in runScriptIn) (*mcp.CallToolResult, runScriptOut, error) {
			if strings.TrimSpace(in.Code) == "" {
				return nil, runScriptOut{}, errScriptNotFunction
			}
			if err := b.open(ctx); err != nil {
				return nil, runScriptOut{}, err
			}
			res, err := runScript(ctx, scriptRun{Code: in.Code, Tools: b.scriptTools(), Refused: b.refused})
			if err != nil {
				return nil, runScriptOut{}, fmt.Errorf("%w (after %d studio calls)", err, res.Calls)
			}
			return nil, runScriptOut{Result: res.Value, Calls: res.Calls}, nil
		})
}

func (b *codeBridge) open(ctx context.Context) error {
	b.once.Do(func() { b.err = b.connect(ctx) })
	return b.err
}

func (b *codeBridge) connect(ctx context.Context) error {
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := b.server.Connect(context.WithoutCancel(ctx), serverT, nil); err != nil {
		return fmt.Errorf("open the script session: %w", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: runScriptName, Version: version}, nil).Connect(context.WithoutCancel(ctx), clientT, nil)
	if err != nil {
		return fmt.Errorf("open the script session: %w", err)
	}
	var scriptable []*mcp.Tool
	refused := map[string]string{}
	for tool, err := range cs.Tools(context.WithoutCancel(ctx), nil) {
		if err != nil {
			return fmt.Errorf("list the script tools: %w", err)
		}
		if reason, ok := refusal(tool); ok {
			refused[tool.Name] = reason
			continue
		}
		scriptable = append(scriptable, tool)
	}
	b.session, b.catalog, b.refused = cs, newDeclCatalog(scriptable), refused
	return nil
}

func refusal(t *mcp.Tool) (string, bool) {
	switch {
	case t.Name == searchAPIName || t.Name == runScriptName:
		return "scripts cannot nest", true
	case strings.HasPrefix(t.Name, "wait_for_"):
		return "it waits longer than a script runs; call it as a normal tool", true
	case strings.HasPrefix(t.Name, "choose_"):
		return "it shows the user a picker; call it as a normal tool", true
	case t.Annotations == nil || !t.Annotations.ReadOnlyHint:
		return refuseNotReadOnly, true
	}
	return "", false
}

func (b *codeBridge) scriptTools() map[string]scriptTool {
	tools := make(map[string]scriptTool, len(b.catalog.Tools))
	for _, t := range b.catalog.Tools {
		tools[t.Name] = func(ctx context.Context, args map[string]any) (string, error) {
			return b.call(ctx, t.Name, args)
		}
	}
	return tools
}

func (b *codeBridge) call(ctx context.Context, name string, args map[string]any) (string, error) {
	res, err := b.session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return "", err
	}
	var text strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			text.WriteString(t.Text)
		}
	}
	if res.IsError {
		return "", errors.New(text.String())
	}
	if text.Len() == 0 && res.StructuredContent != nil {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
	return text.String(), nil
}

func (b *codeBridge) search(in searchAPIIn) (string, error) {
	if len(in.Names) > searchNameLimit {
		return "", fmt.Errorf("names takes at most %d functions", searchNameLimit)
	}
	if len(in.Names) > 0 {
		var tools []declTool
		var unknown []string
		for _, name := range in.Names {
			if t, ok := b.catalog.tool(name); ok {
				tools = append(tools, t)
				continue
			}
			unknown = append(unknown, b.unknownName(name))
		}
		if len(tools) == 0 {
			return strings.Join(unknown, "\n") + "\n", nil
		}
		text := b.catalog.declare(tools)
		if len(unknown) > 0 {
			text += "\n" + strings.Join(unknown, "\n") + "\n"
		}
		return text, nil
	}
	if strings.TrimSpace(in.Query) == "" {
		lines := make([]string, 0, len(b.catalog.Tools))
		for _, t := range b.catalog.Tools {
			lines = append(lines, t.Name+": "+clip(t.Summary, 110))
		}
		return strings.Join(lines, "\n") + "\n", nil
	}
	ranked := b.rank(in.Query)
	if len(ranked) == 0 {
		return "", fmt.Errorf("no read-only function matches %q; call search_api without query for the full list", clip(in.Query, 60))
	}
	shown := ranked[:min(searchExpand, len(ranked))]
	text := b.catalog.declare(shown)
	if rest := ranked[len(shown):]; len(rest) > 0 {
		names := make([]string, 0, searchMoreLimit)
		for _, t := range rest[:min(searchMoreLimit, len(rest))] {
			names = append(names, t.Name)
		}
		text += "\nAlso matching (declare them with names): " + strings.Join(names, ", ") + "\n"
	}
	return text, nil
}

func (b *codeBridge) unknownName(name string) string {
	if reason, ok := b.refused[name]; ok {
		return fmt.Sprintf("%s is not available in run_script: %s", clip(name, 60), reason)
	}
	return fmt.Sprintf("%s is not a Studio function", clip(name, 60))
}

func (b *codeBridge) rank(query string) []declTool {
	words := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	type scored struct {
		tool  declTool
		score int
	}
	var hits []scored
	for _, t := range b.catalog.Tools {
		name, summary := strings.ToLower(t.Name), strings.ToLower(t.Summary)
		score := 0
		for _, w := range words {
			if strings.Contains(name, w) {
				score += 3
			}
			if strings.Contains(summary, w) {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, scored{t, score})
		}
	}
	slices.SortStableFunc(hits, func(a, b scored) int { return b.score - a.score })
	out := make([]declTool, len(hits))
	for i, h := range hits {
		out[i] = h.tool
	}
	return out
}
