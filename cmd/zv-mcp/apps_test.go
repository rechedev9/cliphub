package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connectApps(t *testing.T, s *studio) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerApps(server, s)
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
	return cs
}

func closedStudio(t *testing.T) *studio {
	t.Helper()
	t.Setenv("ORCHESTRATOR_URL", "")
	return &studio{sessionPath: filepath.Join(t.TempDir(), "agent-session.json")}
}

func TestAppResourcesAreListedAndReadable(t *testing.T) {
	cs := connectApps(t, closedStudio(t))
	listed, err := cs.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Resources) != len(appPages) {
		t.Fatalf("listed %d resources, want %d", len(listed.Resources), len(appPages))
	}
	for _, r := range listed.Resources {
		if r.MIMEType != appMIMEType || !strings.HasPrefix(r.URI, "ui://") {
			t.Errorf("resource %+v: want ui:// URI and %s", r, appMIMEType)
		}
	}
	for _, p := range appPages {
		read, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: p.uri})
		if err != nil {
			t.Fatalf("%s: %v", p.uri, err)
		}
		if len(read.Contents) != 1 || read.Contents[0].MIMEType != appMIMEType {
			t.Fatalf("%s: contents %+v", p.uri, read.Contents)
		}
		html := read.Contents[0].Text
		for _, want := range []string{"<!doctype html>", "ui/initialize", "ui/notifications/initialized", "tools/call", `content: [{ type: "text"`} {
			if !strings.Contains(html, want) {
				t.Errorf("%s lacks %q", p.uri, want)
			}
		}
		if strings.Contains(html, "/*BASE_CSS*/") || strings.Contains(html, "/*BRIDGE_JS*/") {
			t.Errorf("%s still holds a marker", p.uri)
		}
		if strings.Contains(html, "http://") || strings.Contains(html, "https://") {
			t.Errorf("%s references the network, which the default CSP blocks", p.uri)
		}
		if strings.ContainsAny(html, "—–") {
			t.Errorf("%s has an em or en dash", p.uri)
		}
	}
}

func TestAppToolsLinkTheirUI(t *testing.T) {
	cs := connectApps(t, closedStudio(t))
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"choose_short_options": shortOptionsAppURI, "choose_hud": hudGalleryAppURI}
	for _, tool := range listed.Tools {
		uri, ok := want[tool.Name]
		if !ok {
			continue
		}
		ui, _ := tool.Meta["ui"].(map[string]any)
		if ui["resourceUri"] != uri || tool.Meta["ui/resourceUri"] != uri {
			t.Errorf("%s: _meta.ui = %v, want resourceUri %s", tool.Name, tool.Meta["ui"], uri)
		}
		delete(want, tool.Name)
	}
	if len(want) != 0 {
		t.Errorf("tools not registered: %v", want)
	}
}

func TestChooseHUDWorksWithoutUI(t *testing.T) {
	cs := connectApps(t, closedStudio(t))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "choose_hud", Arguments: map[string]any{"job_id": "6f1c2a9e-3b7d-4c8a-9e21-5d4b3a2f1e0c"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || len(res.Content) != 1 {
		t.Fatalf("result = %+v", res)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "plan_full_demo") || !strings.Contains(text, nativeHUDID+" ("+nativeHUDName+")") {
		t.Errorf("text = %q", text)
	}
	themes := hudOptions()
	if len(themes) < 2 || themes[len(themes)-1].ID != nativeHUDID {
		t.Errorf("themes = %+v", themes)
	}
}

func TestChooseShortOptionsListsChoicesAsText(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/api/songs", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"songs":[{"id":"s1","title":"Night\u0000 Drive","artist":"Neon","durationSec":125}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	s := closedStudio(t)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)
	cs := connectApps(t, s)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "choose_short_options", Arguments: map[string]any{"job_id": "6f1c2a9e-3b7d-4c8a-9e21-5d4b3a2f1e0c", "segment_ids": []string{"seg-001", "seg-002"}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("error result: %+v", res.Content)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	for _, want := range []string{"short-9x16*", "punch-in*", "flash*", "s1: Night Drive, Neon", "cover*", "generate_short"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	out, _ := res.StructuredContent.(map[string]any)
	if out["job_id"] != "6f1c2a9e-3b7d-4c8a-9e21-5d4b3a2f1e0c" || len(out["segment_ids"].([]any)) != 2 {
		t.Errorf("structured = %v", res.StructuredContent)
	}

	bad, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "choose_short_options", Arguments: map[string]any{"job_id": "6f1c2a9e-3b7d-4c8a-9e21-5d4b3a2f1e0c", "segment_ids": []string{}}})
	if err != nil || !bad.IsError {
		t.Errorf("empty segment_ids should be a tool error: %v %+v", err, bad)
	}
}

func TestChooseShortOptionsReportsClosedStudio(t *testing.T) {
	cs := connectApps(t, closedStudio(t))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "choose_short_options", Arguments: map[string]any{"job_id": "6f1c2a9e-3b7d-4c8a-9e21-5d4b3a2f1e0c", "segment_ids": []string{"seg-001"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "not running") {
		t.Errorf("result = %+v", res)
	}
}
