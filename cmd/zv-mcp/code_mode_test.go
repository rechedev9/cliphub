package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeMatchIn struct {
	ID string `json:"id" jsonschema:"the match"`
}

type fakePlayer struct {
	Name  string `json:"name"`
	Kills int    `json:"kills"`
}

type fakeMatchOut struct {
	Map     string       `json:"map"`
	Players []fakePlayer `json:"players"`
	Rounds  []int        `json:"rounds"`
}

type fakeEchoIn struct {
	Name string `json:"name"`
}

type fakeScopeIn struct {
	From   string `json:"from,omitempty" jsonschema:"first day of the sample"`
	TeamID string `json:"team_id" jsonschema:"the team"`
}

type fakeClipsIn struct {
	From string `json:"from,omitempty" jsonschema:"first second of the clip"`
}

type codeModeFixture struct {
	cs      *mcp.ClientSession
	deletes *atomic.Int32
}

func newCodeModeFixture(t *testing.T) codeModeFixture {
	t.Helper()
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	deletes := &atomic.Int32{}
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "fake_match", Annotations: readOnly, Description: "Read one match. It has every round."},
		func(_ context.Context, _ *mcp.CallToolRequest, in fakeMatchIn) (*mcp.CallToolResult, fakeMatchOut, error) {
			rounds := make([]int, 500)
			for i := range rounds {
				rounds[i] = i + 1
			}
			return nil, fakeMatchOut{Map: "de_ancient", Players: []fakePlayer{{"ZywOo", 31}, {"apEX", 12}}, Rounds: rounds}, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "fake_player", Annotations: readOnly, Description: "Read one player."},
		func(_ context.Context, _ *mcp.CallToolRequest, in fakeEchoIn) (*mcp.CallToolResult, fakePlayer, error) {
			return nil, fakePlayer{Name: in.Name, Kills: len(in.Name)}, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "fake_fail", Annotations: readOnly, Description: "Always fails."},
		func(context.Context, *mcp.CallToolRequest, fakeEchoIn) (*mcp.CallToolResult, fakePlayer, error) {
			return nil, fakePlayer{}, errors.New("not found: match")
		})
	mcp.AddTool(server, &mcp.Tool{Name: "fake_delete", Description: "Delete a match."},
		func(context.Context, *mcp.CallToolRequest, fakeMatchIn) (*mcp.CallToolResult, fakePlayer, error) {
			deletes.Add(1)
			return nil, fakePlayer{}, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "wait_for_fake", Annotations: readOnly, Description: "Wait for a match."},
		func(context.Context, *mcp.CallToolRequest, fakeMatchIn) (*mcp.CallToolResult, fakePlayer, error) {
			return nil, fakePlayer{}, nil
		})
	for i := range scopeMinTools {
		mcp.AddTool(server, &mcp.Tool{Name: fmt.Sprintf("fake_team_%d", i), Annotations: readOnly, Description: "Read a team report."},
			func(context.Context, *mcp.CallToolRequest, fakeScopeIn) (*mcp.CallToolResult, fakePlayer, error) {
				return nil, fakePlayer{}, nil
			})
	}
	mcp.AddTool(server, &mcp.Tool{Name: "fake_clips", Annotations: readOnly, Description: "Read the clips."},
		func(context.Context, *mcp.CallToolRequest, fakeClipsIn) (*mcp.CallToolResult, fakePlayer, error) {
			return nil, fakePlayer{}, nil
		})
	registerCodeTools(server)

	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return codeModeFixture{cs: cs, deletes: deletes}
}

func (f codeModeFixture) text(t *testing.T, params *mcp.CallToolParams) (string, bool) {
	t.Helper()
	res, err := f.cs.CallTool(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

type scriptReply struct {
	Result json.RawMessage `json:"result"`
	Calls  int             `json:"calls"`
}

func (f codeModeFixture) run(t *testing.T, code string) (scriptReply, string) {
	t.Helper()
	text, isErr := f.text(t, &mcp.CallToolParams{Name: runScriptName, Arguments: map[string]any{"code": code}})
	if isErr {
		return scriptReply{}, text
	}
	var out scriptReply
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("decode %s: %v", text, err)
	}
	return out, ""
}

func TestRunScriptChainsReadOnlyCallsAndReturnsOnlyTheResult(t *testing.T) {
	f := newCodeModeFixture(t)
	out, failure := f.run(t, `async () => {
		const m = await studio.fake_match({ id: "m1" });
		const star = [...m.players].sort((a, b) => b.kills - a.kills)[0];
		const [p, again] = await Promise.all([studio.fake_player({ name: star.name }), studio.fake_player({ name: "apEX" })]);
		return { map: m.map, rounds: m.rounds.length, star: p.name, kills: [p.kills, again.kills] };
	}`)
	if failure != "" {
		t.Fatal(failure)
	}
	if got := string(out.Result); got != `{"kills":[5,4],"map":"de_ancient","rounds":500,"star":"ZywOo"}` {
		t.Fatalf("result = %s", got)
	}
	if out.Calls != 3 {
		t.Fatalf("calls = %d, want 3", out.Calls)
	}
}

func TestRunScriptRefusesToolsThatChangeStudioOrWait(t *testing.T) {
	f := newCodeModeFixture(t)
	for tool, want := range map[string]string{"fake_delete": "call it as a normal tool", "wait_for_fake": "waits longer", "run_script": "cannot nest"} {
		_, failure := f.run(t, `async () => studio.`+tool+`({ id: "m1" })`)
		if !strings.Contains(failure, want) {
			t.Fatalf("%s: failure = %q, want %q", tool, failure, want)
		}
	}
	if n := f.deletes.Load(); n != 0 {
		t.Fatalf("fake_delete ran %d times from a script", n)
	}
}

func TestRunScriptLetsTheScriptCatchAToolError(t *testing.T) {
	f := newCodeModeFixture(t)
	out, failure := f.run(t, `async () => { try { await studio.fake_fail({ name: "x" }); return "no error"; } catch (e) { return String(e.message ?? e); } }`)
	if failure != "" {
		t.Fatal(failure)
	}
	if !strings.Contains(string(out.Result), "not found: match") {
		t.Fatalf("result = %s", out.Result)
	}
	_, failure = f.run(t, `async () => studio.fake_fail({ name: "x" })`)
	if !strings.Contains(failure, "not found: match") || !strings.Contains(failure, "after 1 studio calls") {
		t.Fatalf("uncaught failure = %q", failure)
	}
}

func TestRunScriptRefusesWhatIsNotAFunctionOrTooLarge(t *testing.T) {
	f := newCodeModeFixture(t)
	for code, want := range map[string]string{
		"1 + 1":                         "async arrow function",
		`async () => "x".repeat(70000)`: "byte limit",
		"async () => { throw new Error('boom') }":     "boom",
		"async () => { await new Promise(() => {}) }": "never settled",
		strings.Repeat("x", maxScriptCodeBytes+1):     "byte limit",
	} {
		if _, failure := f.run(t, code); !strings.Contains(failure, want) {
			t.Fatalf("code %.40q: failure = %q, want %q", code, failure, want)
		}
	}
}

func TestRunScriptCapsCallsAndMemory(t *testing.T) {
	f := newCodeModeFixture(t)
	if _, failure := f.run(t, `async () => { for (let i = 0; ; i++) await studio.fake_player({ name: "x" }); }`); !strings.Contains(failure, fmt.Sprintf("at most %d tool calls", maxScriptCalls)) {
		t.Fatalf("failure = %q", failure)
	}
	if _, failure := f.run(t, `async () => { const a = []; for (;;) a.push({ n: a.length, s: "round" + a.length }); }`); !strings.Contains(failure, "MiB of memory") {
		t.Fatalf("failure = %q", failure)
	}
}

func TestRunScriptStopsWhenTheCallIsCancelled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := runScript(ctx, scriptRun{Code: "async () => { for (;;) {} }"})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("err = %v, want the cancellation", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("the spinning script ran %s after its call ended", elapsed)
	}
}

func TestRunScriptHasNoBuffersOrHugeStrings(t *testing.T) {
	f := newCodeModeFixture(t)
	for code, want := range map[string]string{
		"async () => new Uint8Array(2 ** 31).length": "Uint8Array is not defined",
		"async () => new ArrayBuffer(2 ** 31)":       "ArrayBuffer is not defined",
		`async () => "ab".repeat(2 ** 30).length`:    "over 16777216 characters",
		`async () => "".padEnd(2 ** 30).length`:      "over 16777216 characters",
	} {
		if _, failure := f.run(t, code); !strings.Contains(failure, want) {
			t.Fatalf("code %q: failure = %q, want %q", code, failure, want)
		}
	}
	out, failure := f.run(t, `async () => "ab".repeat(3) + "x".padStart(3, "-")`)
	if failure != "" || string(out.Result) != `"ababab--x"` {
		t.Fatalf("small strings got %s, %q", out.Result, failure)
	}
}

func TestSearchAPIDeclaresOnlyScriptableTools(t *testing.T) {
	f := newCodeModeFixture(t)
	text, isErr := f.text(t, &mcp.CallToolParams{Name: searchAPIName, Arguments: map[string]any{"query": "match"}})
	if isErr {
		t.Fatal(text)
	}
	for _, want := range []string{"/** Read one match. */", "fake_match(args: {", "id: string; // the match", "Promise<{ map: string; players: { kills: number; name: string }[]; rounds: number[] }>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("search for match lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "fake_delete(") {
		t.Fatalf("search declared a tool that changes Studio:\n%s", text)
	}

	text, _ = f.text(t, &mcp.CallToolParams{Name: searchAPIName, Arguments: map[string]any{"names": []string{"fake_team_3", "fake_delete", "nope"}}})
	for _, want := range []string{"type Scope = {", "from?: string; // first day of the sample", "team_id: string; // the team", "} & Scope): Promise<", "fake_delete is not available in run_script", "nope is not a Studio function"} {
		if !strings.Contains(text, want) {
			t.Fatalf("names search lacks %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "from?: string") != 1 {
		t.Fatalf("the shared filter is declared more than once:\n%s", text)
	}

	text, _ = f.text(t, &mcp.CallToolParams{Name: searchAPIName, Arguments: map[string]any{"names": []string{"fake_clips"}}})
	if !strings.Contains(text, "from?: string; // first second of the clip") || strings.Contains(text, "Scope") {
		t.Fatalf("a field that only shares the name became a shared filter:\n%s", text)
	}

	text, _ = f.text(t, &mcp.CallToolParams{Name: searchAPIName, Arguments: map[string]any{"names": []string{"fake_delete"}}})
	if strings.Contains(text, "declare const studio") || !strings.Contains(text, "fake_delete is not available in run_script") {
		t.Fatalf("a search with only refused names:\n%s", text)
	}

	text, _ = f.text(t, &mcp.CallToolParams{Name: searchAPIName, Arguments: map[string]any{}})
	if !strings.Contains(text, "fake_player: Read one player.") || strings.Contains(text, "fake_delete") || strings.Contains(text, "wait_for_fake") {
		t.Fatalf("catalog:\n%s", text)
	}
}

func TestReadOnlyToolsChangeNothing(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerTools(server, closedStudio(t))
	registerCodeTools(server)
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	verbs := []string{"add_", "assign_", "attach_", "change_", "clipmind_disconnect", "clipmind_sync", "correct_", "create_", "delete_", "discard_", "edit_", "export_", "follow_", "generate_", "import_", "install_", "launch_", "link_", "merge_", "pick_", "reanalyze_", "record_", "rename_", "render_", "rescan_", "retry_", "reveal_", "save_", "set_", "sync_", "transcribe_", "unfollow_", "update_"}
	readOnly := 0
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			continue
		}
		readOnly++
		for _, verb := range verbs {
			if strings.HasPrefix(tool.Name, verb) {
				t.Errorf("%s is marked read-only, so run_script calls it without approval, but its name says it changes Studio", tool.Name)
			}
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), `"approve"`) {
			t.Errorf("%s asks for approval but is marked read-only, so run_script would call it without asking", tool.Name)
		}
	}
	if readOnly < 50 {
		t.Fatalf("only %d read-only tools; the registration changed", readOnly)
	}
}
