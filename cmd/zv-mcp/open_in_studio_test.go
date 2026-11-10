package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testWebOrigin = "http://127.0.0.1:4200"

func openStudio(t *testing.T) (*mcp.ClientSession, string) {
	t.Helper()
	dir := t.TempDir()
	session := `{"version":1,"enabled":true,"orchestratorUrl":"http://127.0.0.1:4100","token":"` + strings.Repeat("a", 64) + `","webOrigin":"` + testWebOrigin + `"}`
	if err := os.WriteFile(filepath.Join(dir, "agent-session.json"), []byte(session), 0o600); err != nil {
		t.Fatal(err)
	}
	old := openWait
	openWait = 300 * time.Millisecond
	t.Cleanup(func() { openWait = old })
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerOpenInStudioTools(server, &studio{sessionPath: filepath.Join(dir, "agent-session.json")})
	return connectWith(t, server, nil), dir
}

// fakeWindow answers the next request as the desktop does: it takes the file and writes the ack.
func fakeWindow(t *testing.T, dir string, ok bool) <-chan string {
	t.Helper()
	got := make(chan string, 1)
	go func() {
		request := filepath.Join(dir, openRequestFile)
		for range 100 {
			raw, err := os.ReadFile(request)
			if err != nil {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			_ = os.Remove(request)
			var req struct {
				ID   string `json:"id"`
				Path string `json:"path"`
			}
			_ = json.Unmarshal(raw, &req)
			ack, _ := json.Marshal(openAck{Version: openRequestVersion, ID: req.ID, OK: ok, Reason: "the page did not load"})
			_ = os.WriteFile(filepath.Join(dir, "open-ack-"+req.ID+".json"), ack, 0o600)
			got <- req.Path
			return
		}
		got <- ""
	}()
	return got
}

func TestOpenInStudioWaitsForTheWindowsAck(t *testing.T) {
	cs, dir := openStudio(t)
	got := fakeWindow(t, dir, true)
	var out openInStudioOut
	callTool(t, cs, "open_in_studio", map[string]any{"replay_url": testWebOrigin + "/replay?rounds=a%2F3#panel"}, &out)
	if p := <-got; p != "/replay?rounds=a%2F3#panel" {
		t.Fatalf("window got %q", p)
	}
	if !out.Opened || out.URL != testWebOrigin+"/replay?rounds=a%2F3#panel" {
		t.Fatalf("out = %+v, want opened on the full url", out)
	}

	got = fakeWindow(t, dir, false)
	callTool(t, cs, "open_in_studio", map[string]any{"studio_path": "/informes/x"}, &out)
	<-got
	if out.Opened || !strings.Contains(out.Note, "did not load") {
		t.Fatalf("refused load = %+v, want opened false with the reason", out)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "open-*")); len(left) != 0 {
		t.Fatalf("files left in the profile: %v", left)
	}
}

func TestOpenInStudioFallsBackToTheURL(t *testing.T) {
	cs, dir := openStudio(t)
	var out openInStudioOut
	callTool(t, cs, "open_in_studio", map[string]any{"studio_path": "/teams/x?tab=preparar"}, &out)
	if out.Opened || out.URL != testWebOrigin+"/teams/x?tab=preparar" || out.Note == "" {
		t.Fatalf("out = %+v, want the url to open by hand", out)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "open-*")); len(left) != 0 {
		t.Fatalf("a request nobody took stayed in the profile: %v", left)
	}

	for _, args := range []map[string]any{
		{},
		{"studio_path": "//evil.example/x"},
		{"studio_path": "replay"},
		{"studio_path": "/a\\b"},
		{"studio_path": "/api/library/reports"},
		{"studio_path": "/_next/static/app.js"},
		{"studio_path": "/hud/theme.png"},
		{"replay_url": "http://127.0.0.1:9999/replay"},
		{"studio_path": "/a", "replay_url": testWebOrigin + "/a"},
	} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "open_in_studio", Arguments: args})
		if err != nil || !res.IsError {
			t.Errorf("open_in_studio %v = %v, want a tool error", args, err)
		}
	}
}
