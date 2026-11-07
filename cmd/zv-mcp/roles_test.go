package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTeamRolesReadSuggestionAndSave(t *testing.T) {
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/teams/{id}/roles", `{"team_name":"Vitality","matches":3,
		"sheet":{"map":"de_mirage","t":[{"name":"Entry","steamid64":"`+testSteamID+`","task":"abre B"}],"ct":[]},
		"suggested":{"t":[{"steamid64":"`+testSteamID+`","player":"ZywOo","role":"awp","rounds":40}],
		"ct":[{"steamid64":"`+testSteamID+`","player":"ZywOo","role":"anchor","callout":"BombsiteB","rounds":20}]}}`)
	rec.handle(mux, "PUT /api/library/teams/{id}/roles", `{"map":"de_mirage","t":[{"name":"AWP"}],"ct":[]}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerRoleTools(server, s) })

	var got teamRolesOut
	callTool(t, cs, "get_team_roles", map[string]any{"team_id": testTeamID, "map": "DE_MIRAGE"}, &got)
	if q := rec.get("GET /api/library/teams/{id}/roles"); !strings.Contains(q, "/"+testTeamID+"/roles?map=de_mirage") {
		t.Errorf("get query = %s", q)
	}
	if len(got.T) != 1 || got.T[0].Task != "abre B" || got.SuggestedCT[0].Callout != "BombsiteB" || got.SuggestedT[0].Role != "awp" || got.Matches != 3 {
		t.Fatalf("roles = %+v", got)
	}

	var saved savedRolesOut
	callTool(t, cs, "set_team_roles", map[string]any{"team_id": testTeamID, "map": "de_mirage",
		"t": []map[string]any{{"name": " AWP ", "steamid64": testSteamID}}, "ct": []map[string]any{}}, &saved)
	if body := rec.get("PUT /api/library/teams/{id}/roles"); !strings.Contains(body, `{"map":"de_mirage","t":[{"name":"AWP","steamid64":"`+testSteamID+`"}],"ct":[]}`) {
		t.Errorf("put body = %s", body)
	}
	six := []map[string]any{{"name": "a"}, {"name": "b"}, {"name": "c"}, {"name": "d"}, {"name": "e"}, {"name": "f"}}
	toolFails(t, cs, "set_team_roles", map[string]any{"team_id": testTeamID, "map": "de_mirage", "t": six, "ct": []map[string]any{}}, "at most 5 roles")
	toolFails(t, cs, "set_team_roles", map[string]any{"team_id": testTeamID, "map": "de_mirage", "t": []map[string]any{{"name": "x", "steamid64": "12"}}, "ct": []map[string]any{}}, "SteamID64")
	toolFails(t, cs, "get_team_roles", map[string]any{"team_id": testTeamID, "map": "../x"}, "map must be")
}

func TestUpdatePlaySendsRoles(t *testing.T) {
	cs, rec := stratbookStudio(t)
	var p playDetailOut
	callTool(t, cs, "update_play", map[string]any{"play_id": testPlayID, "roles": []map[string]any{{"name": "Lurker", "callout": "Palace"}}}, &p)
	if got := rec.get("PATCH /api/library/plays/{id}"); !strings.Contains(got, `"roles":[{"name":"Lurker","callout":"Palace"}]`) {
		t.Errorf("patch body = %s", got)
	}
	toolFails(t, cs, "update_play", map[string]any{"play_id": testPlayID, "roles": []map[string]any{{"name": ""}}}, "name takes 1 to 40")
}
