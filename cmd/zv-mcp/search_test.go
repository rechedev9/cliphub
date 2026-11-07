package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSearchStudioPassesTheQuery(t *testing.T) {
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/search", `{"map":"de_mirage","grenade":"smoke","results":[
		{"kind":"lineup","id":"`+testLineupID+`","title":"Humo ventana","map":"de_mirage","side":"T","detail":"smoke"}]}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerSearchTool(server, s) })

	var out searchOut
	callTool(t, cs, "search_studio", map[string]any{"query": " humo ventana mirage ", "per_kind": 3}, &out)
	if got := rec.get("GET /api/library/search"); !strings.Contains(got, "q=humo+ventana+mirage") || !strings.Contains(got, "limit=3") {
		t.Errorf("search query = %s", got)
	}
	if out.Map != "de_mirage" || len(out.Results) != 1 || out.Results[0].Kind != "lineup" || out.Results[0].Title != "Humo ventana" {
		t.Fatalf("search = %+v", out)
	}
	toolFails(t, cs, "search_studio", map[string]any{"query": "  "}, "must not be empty")
	toolFails(t, cs, "search_studio", map[string]any{"query": "x", "per_kind": 30}, "per_kind")
}
