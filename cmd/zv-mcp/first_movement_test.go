package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func fakeOverlayStudio(t *testing.T) *mcp.ClientSession {
	t.Helper()
	match := uuid.MustParse(testMatchID)
	run := func(id, name string, start, x int) tacticalplan.OverlayTrack {
		return tacticalplan.OverlayTrack{SteamID64: id, Name: name, Start: start, Points: [][3]int{{0, 0, 0}, {x, 0, 0}}}
	}
	overlay := tuiclient.TeamOverlay{Map: "de_nuke", Overlay: &tacticalplan.Overlay{StepSeconds: 0.5, Limit: 40, Total: 55, Rounds: []tacticalplan.OverlayRound{
		{MatchID: match, Round: 3, TickFreezeEnd: 1000, Tickrate: 64, Tracks: []tacticalplan.OverlayTrack{run(grenadeA, "Zy\u0000wOo", 20, 100), run(grenadeB, "apEX", 80, 500)}},
		{MatchID: match, Round: 4, TickFreezeEnd: 5000, Tickrate: 64, Tracks: []tacticalplan.OverlayTrack{run(grenadeA, "ZywOo", 4, 100)}},
	}}}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/overlay", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("side") != "T" || r.URL.Query().Has("limit") {
			http.Error(w, `{"error":"unexpected query `+r.URL.RawQuery+`"}`, http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(overlay)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := closedStudio(t)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerFirstMovementTool(server, s)
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

func TestTeamFirstMovementListsEachPlayersMovePerRound(t *testing.T) {
	cs := fakeOverlayStudio(t)
	base := map[string]any{"team_id": testTeamID, "side": "T"}
	var out firstMovementOut
	callTool(t, cs, "team_first_movement", base, &out)
	if out.Map != "de_nuke" || out.Rounds != 2 || out.Total != 55 || out.Moves != 3 || out.Matched != 2 || len(out.Rows) != 3 || out.Truncated {
		t.Fatalf("out = %+v", out)
	}
	if r := out.Rows[0]; r.Ref != testMatchID+"/3" || r.Player != "ZywOo" || r.Seconds != 10.5 || r.Clock != "1:45" || !strings.Contains(r.StudioPath, "jugador="+grenadeA) {
		t.Errorf("first row = %+v, want the name cleaned and a link focused on the player", r)
	}
	if len(out.Histogram) != 3 || out.Histogram[0].Span != "1:55-1:45" || out.Histogram[0].Moves != 1 || out.Histogram[1].Span != "1:45-1:35" {
		t.Errorf("histogram = %+v", out.Histogram)
	}
	for _, want := range []string{"/patrones/" + testTeamID + "?", "modo=jugadores", "map=de_nuke"} {
		if !strings.Contains(out.StudioPath, want) {
			t.Errorf("studio_path = %s, want %s", out.StudioPath, want)
		}
	}

	var one firstMovementOut
	callTool(t, cs, "team_first_movement", map[string]any{"team_id": testTeamID, "side": "T", "players": []string{grenadeB}, "clock_from": 30, "clock_to": 60, "limit": 1}, &one)
	if one.Moves != 1 || one.Rows[0].SteamID64 != grenadeB || !strings.Contains(one.StudioPath, "tramo=30-60") || !strings.Contains(one.StudioPath, "jugador="+grenadeB) {
		t.Errorf("player B from 30 s = %+v", one)
	}
	var zone firstMovementOut
	callTool(t, cs, "team_first_movement", map[string]any{"team_id": testTeamID, "side": "T", "area": map[string]any{"min_x": 400, "min_y": -10, "max_x": 600, "max_y": 10}, "limit": 1}, &zone)
	if zone.Matched != 1 || zone.Moves != 2 || !zone.Truncated || len(zone.Rows) != 1 {
		t.Errorf("through the zone = %+v, want round 3 only and the second row cut by limit", zone)
	}
	toolFails(t, cs, "team_first_movement", map[string]any{"team_id": testTeamID, "clock_from": 20, "clock_to": 10}, "clock_to must not be before clock_from")
	toolFails(t, cs, "team_first_movement", map[string]any{"team_id": testTeamID, "players": []string{"zywoo"}}, "SteamID64")
	toolFails(t, cs, "team_first_movement", map[string]any{"team_id": testTeamID, "limit": 101}, "limit must be")
}
