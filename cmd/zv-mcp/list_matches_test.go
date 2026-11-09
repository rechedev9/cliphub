package main

import (
	"net/http"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	listMatchA   = "a1111111-1111-4111-8111-111111111111"
	listMatchB   = "b2222222-2222-4222-8222-222222222222"
	listDemoA    = "c3333333-3333-4333-8333-333333333333"
	listDemoB    = "d4444444-4444-4444-8444-444444444444"
	listJobWithV = "e5555555-5555-4555-8555-555555555555"
)

func listMatchesStudio(t *testing.T) *mcp.ClientSession {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/library/matches", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"matches":[
			{"id":"` + listMatchA + `","demo_id":"` + listDemoA + `","series_id":"` + testSeriesID + `","source":"faceit","map":"de_mirage","played_at":"2026-11-05T20:00:00Z","round_count":24,"teams":[]},
			{"id":"` + listMatchB + `","demo_id":"` + listDemoB + `","source":"scrim","map":"de_nuke","played_at":"2026-11-01T20:00:00Z","round_count":26,"teams":[]}]}`))
	})
	mux.HandleFunc("GET /api/library/demos", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"demos":[
			{"id":"` + listDemoA + `","source":"faceit","tags":["ESL"],"job_ids":[],"imported_at":"2026-11-02T10:00:00Z"},
			{"id":"` + listDemoB + `","source":"scrim","tags":[],"job_ids":["` + listJobWithV + `"],"imported_at":"2026-11-09T10:00:00Z"}]}`))
	})
	mux.HandleFunc("GET /api/library/series", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"series":[{"id":"` + testSeriesID + `","format":"bo3","event":"IEM","teams":[{"slot":"a","team_id":"` + testTeamID + `","name":"Vitality"}],
			"vetoes":[{"team":"a","action":"ban","map":"de_nuke"},{"team":"a","action":"pick","map":"de_mirage"}],"match_ids":["` + listMatchA + `"],"updated_at":"2026-11-01T00:00:00Z"}]}`))
	})
	mux.HandleFunc("GET /api/jobs", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jobs":[{"id":"` + listJobWithV + `","status":"done"}]}`))
	})
	mux.HandleFunc("GET /api/presets", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"default":"short","presets":[{"name":"short"}]}`))
	})
	mux.HandleFunc("GET /api/jobs/batch-status", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"job_id":"` + listJobWithV + `","variant":"short","job":{"status":"done"},"render":{"status":"ready","videos":["clip"]}}]}`))
	})
	return serveTools(t, mux, func(server *mcp.Server, s *studio) { registerTools(server, s) })
}

func listedIDs(out listMatchesOut) []string {
	ids := []string{}
	for _, m := range out.Matches {
		ids = append(ids, m.ID)
	}
	return ids
}

func TestListMatchesShowsKindSeriesAndVeto(t *testing.T) {
	cs := listMatchesStudio(t)
	var out listMatchesOut
	callTool(t, cs, "list_matches", map[string]any{}, &out)
	if want := []string{listMatchA, listMatchB}; !slices.Equal(listedIDs(out), want) {
		t.Fatalf("ids = %v, want %v", listedIDs(out), want)
	}
	a := out.Matches[0]
	if a.Kind != "oficial" || a.Series == nil || a.Series.Format != "bo3" || a.Series.MapRole != "pick" || a.Series.PickedBy != "Vitality" || len(a.Series.Vetoes) != 2 || len(a.Tags) != 1 {
		t.Errorf("first match = %+v series %+v", a, a.Series)
	}
	if b := out.Matches[1]; b.Kind != "scrim" || b.Series != nil || b.HasPOV {
		t.Errorf("second match = %+v", b)
	}
}

func TestListMatchesFiltersByKindAndPOVAndSortsByImport(t *testing.T) {
	cs := listMatchesStudio(t)
	var out listMatchesOut
	callTool(t, cs, "list_matches", map[string]any{"kind": "oficial"}, &out)
	if want := []string{listMatchA}; !slices.Equal(listedIDs(out), want) {
		t.Errorf("kind oficial = %v, want %v", listedIDs(out), want)
	}
	callTool(t, cs, "list_matches", map[string]any{"has_pov": true}, &out)
	if want := []string{listMatchB}; !slices.Equal(listedIDs(out), want) || !out.Matches[0].HasPOV {
		t.Errorf("has_pov = %v, want %v", listedIDs(out), want)
	}
	callTool(t, cs, "list_matches", map[string]any{"sort": "imported"}, &out)
	if want := []string{listMatchB, listMatchA}; !slices.Equal(listedIDs(out), want) {
		t.Errorf("sort imported = %v, want %v", listedIDs(out), want)
	}
	toolFails(t, cs, "list_matches", map[string]any{"kind": "pro"}, "kind must be one of")
	toolFails(t, cs, "list_matches", map[string]any{"sort": "name"}, "sort must be")
}
