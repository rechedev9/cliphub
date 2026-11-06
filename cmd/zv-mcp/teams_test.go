package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testRivalID = "1a2b3c4d-5e6f-4a1b-8c2d-3e4f5a6b7c8d"

// serveTools connects a client to the tools register adds, over a fake orchestrator.
func serveTools(t *testing.T, mux *http.ServeMux, register func(*mcp.Server, *studio)) *mcp.ClientSession {
	t.Helper()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := closedStudio(t)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	register(server, s)
	return connectWith(t, server, nil)
}

// recorder keeps the last request each route got, as "METHOD path?query body".
type recorder struct {
	mu   sync.Mutex
	seen map[string]string
}

func (rec *recorder) handle(mux *http.ServeMux, pattern, reply string) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.mu.Lock()
		rec.seen[pattern] = strings.TrimSpace(r.Method + " " + r.URL.RequestURI() + " " + string(body))
		rec.mu.Unlock()
		_, _ = w.Write([]byte(reply))
	})
}

func (rec *recorder) get(pattern string) string {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.seen[pattern]
}

const testTeamJSON = `{"id":"` + testTeamID + `","name":"Vitality","aliases":["VIT"],"match_count":3,
	"roster":[{"steamid64":"` + testSteamID + `","nick":"ZywOo","first_played_at":"2026-01-02T00:00:00Z","last_played_at":"2026-10-12T00:00:00Z","matches":3}]}`

func teamsStudio(t *testing.T) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/teams/{id}", testTeamJSON)
	rec.handle(mux, "PATCH /api/library/teams/{id}", `{"id":"`+testTeamID+`","name":"Team Vitality","aliases":[],"match_count":3}`)
	rec.handle(mux, "POST /api/library/teams/{id}/merge", `{"id":"`+testRivalID+`","name":"MOUZ","aliases":["VIT"],"match_count":7}`)
	rec.handle(mux, "PUT /api/library/matches/{id}/teams/{key}", `{"id":"`+testMatchID+`","teams":[{"key":"ct-start","start_side":"ct","clan_name":"VIT","team_id":"`+testTeamID+`","team_name":"Vitality","assignment":"manual"}]}`)
	rec.handle(mux, "GET /api/library/teams/{id}/scout", `{"team_name":"Vitality"}`)
	rec.handle(mux, "GET /api/library/baseline", `{"scope":"library","min_rounds":200,"metrics":{}}`)
	rec.handle(mux, "GET /api/library/players", `{"players":[{"steamid64":"`+testSteamID+`","nick":"ZywOo","match_count":3,"faceit_nickname":"zywoo"}]}`)
	rec.handle(mux, "GET /api/library/players/{steamid}", `{"steamid64":"`+testSteamID+`","nick":"ZywOo","match_count":3,
		"nicks":[{"nick":"ZywOo","first_seen_at":"2026-01-02T00:00:00Z","last_seen_at":"2026-10-12T00:00:00Z"}],
		"teams":[{"team_id":"`+testTeamID+`","team_name":"Vitality","first_played_at":"2026-01-02T00:00:00Z","last_played_at":"2026-10-12T00:00:00Z","matches":3}]}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) {
		registerTeamTools(server, s)
		registerPlayerTools(server, s)
		registerAnalysisTools(server, s)
	})
	return cs, rec
}

func TestTeamToolsSendWhatTheUISends(t *testing.T) {
	cs, rec := teamsStudio(t)
	var team teamDetailOut
	callTool(t, cs, "get_team", map[string]any{"team_id": testTeamID}, &team)
	if team.Name != "Vitality" || len(team.Roster) != 1 || team.Roster[0].First != "2026-01-02" {
		t.Fatalf("team = %+v", team)
	}

	var renamed teamOut
	callTool(t, cs, "update_team", map[string]any{"team_id": testTeamID, "name": " Team Vitality ", "aliases": []string{}}, &renamed)
	if got := rec.get("PATCH /api/library/teams/{id}"); !strings.HasSuffix(got, `{"name":"Team Vitality","aliases":[]}`) {
		t.Errorf("patch = %s, want the trimmed name and an empty alias list", got)
	}
	callTool(t, cs, "update_team", map[string]any{"team_id": testTeamID, "name": "Vitality"}, &renamed)
	if got := rec.get("PATCH /api/library/teams/{id}"); !strings.HasSuffix(got, `{"name":"Vitality"}`) {
		t.Errorf("rename only = %s, want aliases left out", got)
	}
	toolFails(t, cs, "update_team", map[string]any{"team_id": testTeamID}, "set name, aliases or both")
	toolFails(t, cs, "update_team", map[string]any{"team_id": testTeamID, "name": "a\nb"}, "control characters")

	var assigned assignTeamOut
	callTool(t, cs, "assign_match_team", map[string]any{"match_id": testMatchID, "side": "ct-start", "team_name": "VIT", "remember_alias": true}, &assigned)
	want := "PUT /api/library/matches/" + testMatchID + "/teams/ct-start " + `{"team_id":null,"team_name":"VIT","remember_alias":true}`
	if got := rec.get("PUT /api/library/matches/{id}/teams/{key}"); got != want {
		t.Errorf("assign = %s, want %s", got, want)
	}
	if len(assigned.Sides) != 1 || assigned.Sides[0].Assignment != "manual" {
		t.Errorf("sides = %+v", assigned.Sides)
	}
	callTool(t, cs, "assign_match_team", map[string]any{"match_id": testMatchID, "side": "t-start", "clear": true}, &assigned)
	if got := rec.get("PUT /api/library/matches/{id}/teams/{key}"); !strings.HasSuffix(got, `{"team_id":null,"remember_alias":false}`) {
		t.Errorf("clear = %s", got)
	}
	toolFails(t, cs, "assign_match_team", map[string]any{"match_id": testMatchID, "side": "ct", "clear": true}, "side must be ct-start or t-start")
	toolFails(t, cs, "assign_match_team", map[string]any{"match_id": testMatchID, "side": "ct-start", "team_id": testTeamID, "clear": true}, "exactly one")
}

func TestMergeTeamsNeedsApproval(t *testing.T) {
	cs, rec := teamsStudio(t)
	var asked mergeTeamsOut
	callTool(t, cs, "merge_teams", map[string]any{"team_id": testTeamID, "into_team_id": testRivalID}, &asked)
	if !asked.NeedsApproval || asked.ApprovalCode == "" || rec.get("POST /api/library/teams/{id}/merge") != "" {
		t.Fatalf("first call = %+v, want an approval code and no merge", asked)
	}
	var merged mergeTeamsOut
	callTool(t, cs, "merge_teams", map[string]any{"team_id": testTeamID, "into_team_id": testRivalID, "approve": asked.ApprovalCode}, &merged)
	if merged.Team == nil || merged.Team.ID != testRivalID {
		t.Fatalf("merged = %+v", merged)
	}
	if got := rec.get("POST /api/library/teams/{id}/merge"); !strings.HasSuffix(got, `{"into_team_id":"`+testRivalID+`"}`) {
		t.Errorf("merge = %s", got)
	}
	toolFails(t, cs, "merge_teams", map[string]any{"team_id": testTeamID, "into_team_id": testTeamID}, "same team")
}

func TestTeamScopeAndBaselineMinRounds(t *testing.T) {
	cs, rec := teamsStudio(t)
	var scout scoutTeamOut
	callTool(t, cs, "scout_team", map[string]any{"team_id": testTeamID, "opponent_id": []string{testRivalID}, "match_id": []string{testMatchID, testTeamID}}, &scout)
	got := rec.get("GET /api/library/teams/{id}/scout")
	if !strings.Contains(got, "match_id="+testMatchID+"&match_id="+testTeamID) || !strings.Contains(got, "opponent_id="+testRivalID) {
		t.Errorf("scout query = %s, want repeated match_id and opponent_id", got)
	}
	toolFails(t, cs, "scout_team", map[string]any{"team_id": testTeamID, "opponent_id": []string{"MOUZ"}}, "opponent_id")

	var b baselineOut
	callTool(t, cs, "get_baseline", map[string]any{"min_rounds": 200}, &b)
	if got := rec.get("GET /api/library/baseline"); !strings.Contains(got, "min_rounds=200") {
		t.Errorf("baseline query = %s", got)
	}
	toolFails(t, cs, "get_baseline", map[string]any{"min_rounds": 20000}, "min_rounds")
}

func TestPlayerToolsResolveANick(t *testing.T) {
	cs, rec := teamsStudio(t)
	var list listPlayersOut
	callTool(t, cs, "list_players", map[string]any{"search": "zywoo", "team_id": testTeamID}, &list)
	if len(list.Players) != 1 || list.Players[0].SteamID64 != testSteamID {
		t.Fatalf("players = %+v", list)
	}
	if got := rec.get("GET /api/library/players"); !strings.Contains(got, "limit=50") || !strings.Contains(got, "q=zywoo") || !strings.Contains(got, "team_id="+testTeamID) {
		t.Errorf("list query = %s", got)
	}
	var p playerProfileOut
	callTool(t, cs, "get_player", map[string]any{"steamid": testSteamID}, &p)
	if len(p.Nicks) != 1 || len(p.Teams) != 1 || p.Teams[0].Last != "2026-10-12" || p.Nick != "ZywOo" {
		t.Fatalf("profile = %+v", p)
	}
	toolFails(t, cs, "get_player", map[string]any{"steamid": "zywoo"}, "steamid")
}
