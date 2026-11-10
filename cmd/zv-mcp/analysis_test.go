package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	testTeamID  = "6f1c2a9e-3b7d-4c8a-9e21-5d4b3a2f1e0c"
	testMatchID = "0b9c1f9e-0f4d-4c7a-9a59-2f4f6c1d2e3a"
	testSteamID = "76561198000000001"
)

// fakeAnalysisStudio is an orchestrator with one team, one match of that team
// (CT start, rounds 1 and 2) and its player.
func fakeAnalysisStudio(t *testing.T) *mcp.ClientSession {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/library/teams", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"teams":[{"id":"` + testTeamID + `","name":"Vita\u0000lity","aliases":["VIT"],"match_count":3}]}`))
	})
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/scout", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("side") != "T" || r.URL.Query().Get("map") != "de_nuke" || r.URL.Query().Get("to") != "2026-10-31T23:59:59Z" {
			http.Error(w, `{"error":"unexpected query `+r.URL.RawQuery+`"}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"team_name":"Vitality","skipped":[{"match_id":"x","reason":"not_analyzed"}],
			"matches":[{"match_id":"` + testMatchID + `","map":"de_nuke","played_at":"2026-10-12T18:00:00Z","opponent":"MOUZ","score":13,"opponent_score":9}],
			"maps":[{"map":"de_nuke","matches":1,"match_wins":1,"rounds":22,"win_rate":{"pct":59.1},"ct_win_rate":{"pct":60},"t_win_rate":{"pct":58.3}}],
			"tendencies":{"round_count":22,"wins":13,"buys":[{"buy":"full","rounds":18,"share":{"pct":81.8},"win_rate":{"pct":61.1,"reliable":true}},{"buy":"eco","rounds":0}],
			"sites":[{"site":"A","rounds":12,"share":{"pct":54.5},"win_rate":{"pct":50,"reliable":true}}],
			"openings":{"rounds":20,"won":{"pct":55},"traded_on_loss":{"pct":40}},
			"players":[{"steamid64":"` + testSteamID + `","name":"ZywOo","rounds":22,"kills":25,"deaths":12,"adr":98.2,"opening_kills":6,"opening_deaths":2}]},
			"executes":[{"site":"B","rounds":6,"win_rate":{"pct":66.7},"plants":{"pct":83.3}}],
			"awp":{"sides":[{"side":"T","rounds":11,"kills":9,"opening_kills":4}]}}`))
	})
	mux.HandleFunc("GET /api/library/players/"+testSteamID+"/metrics", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"matches":2,"rounds":40,"skipped":[],"players":[{"steamid64":"` + testSteamID + `","name":"ZywOo","role":"awper",
			"matches":2,"rounds":40,"kills":32,"deaths":20,"swing":{"wpa":2.4,"per_round":0.06},
			"trades":{"trade_kills":{"count":5},"traded_deaths":{"count":8}},
			"utility":{"measured":true,"flash":{"thrown":10,"assists":{"count":2}},"he":{"enemy_damage":120},"molotov":{"enemy_damage":80}},
			"box":{"damage":3400,"kast_rounds":30,"opening_kills":6,"opening_deaths":4,"utility_rounds":20},
			"buys":[{"buy":"full","rounds":30,"won":18,"kills":26,"deaths":14,"wpa":2}]}]}`))
	})
	mux.HandleFunc("GET /api/library/baseline", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"scope":"` + r.URL.Query().Get("scope") + `","map":"","min_rounds":50,"players":12,"rounds":900,"matches":40,"skipped":0,
			"metrics":{"kpr":{"mean":0.68,"players":12}}}`))
	})
	mux.HandleFunc("GET /api/library/matches", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("team_id") == "" && r.URL.Query().Get("steamid") == "" {
			http.Error(w, `{"error":"no subject"}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"matches":[{"id":"` + testMatchID + `","map":"de_nuke","played_at":"2026-10-12T18:00:00Z",
			"teams":[{"key":"ct-start","clan_name":"Vitality","team_id":"` + testTeamID + `"},{"key":"t-start","clan_name":"MOUZ‮"}]}]}`))
	})
	mux.HandleFunc("GET /api/library/matches/"+testMatchID+"/tactical", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"demo":{"map":"de_nuke"},"players":[{"slot":0,"steamid64":"` + testSteamID + `","team_key":"ct-start"},{"slot":5,"steamid64":"2","team_key":"t-start"}],
			"rounds":[
			{"number":1,"winner":"CT","economy":{"ct_buy":"pistol","t_buy":"pistol"},"class":{"t_side":"fast","ct_side":"hold","site":"A"},"players":[{"slot":0,"side":"CT","kills":2},{"slot":5,"side":"T","deaths":1}]},
			{"number":2,"winner":"T","economy":{"ct_buy":"full","t_buy":"eco"},"class":{"t_side":"eco_rush","ct_side":"aggression","site":"B"},"players":[{"slot":0,"side":"CT","deaths":1},{"slot":5,"side":"T"}]}]}`))
	})
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/rounds", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		rounds := []string{}
		for _, round := range []struct{ json, side, buy string }{
			{`{"round":1,"side":"CT","winner":"CT","buy":"pistol","opponent_buy":"pistol","site":"A","pattern":"hold"}`, "CT", "pistol"},
			{`{"round":2,"side":"CT","winner":"T","buy":"full","opponent_buy":"eco","site":"B","pattern":"aggression"}`, "CT", "full"},
		} {
			if (q.Get("side") == "" || q.Get("side") == round.side) && (!q.Has("buy") || q.Get("buy") == round.buy) {
				rounds = append(rounds, round.json)
			}
		}
		matches := ""
		if !q.Has("match_id") || q.Get("match_id") == testMatchID {
			matches = `{"match_id":"` + testMatchID + `","map":"de_nuke","played_at":"2026-10-12T18:00:00Z","opponent":"MOUZ","total":2,"rounds":[` + strings.Join(rounds, ",") + `]}`
		}
		_, _ = w.Write([]byte(`{"team_name":"Vitality","rounds":` + fmt.Sprint(len(rounds)) + `,"matches":[` + matches + `],"skipped":[{"match_id":"x","reason":"over_match_limit"}]}`))
	})
	mux.HandleFunc("GET /api/library/matches/"+testMatchID, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"` + testMatchID + `","map":"de_nuke","played_at":"2026-10-12T18:00:00Z","teams":[{"key":"ct-start","team_id":"` + testTeamID + `"},{"key":"t-start","clan_name":"MOUZ"}]}`))
	})
	mux.HandleFunc("GET /api/library/plays/"+testPlayID, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"` + testPlayID + `","title":"Agresión en B","team_id":"` + testTeamID + `","map":"de_nuke","side":"CT","kind":"aggression","site":"B",
			"examples":[{"match_id":"` + testMatchID + `","round":2}]}`))
	})
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/plays", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"team_name":"Vitality","matches":1,"rounds":2,"compared":2,"plays":[{"map":"de_nuke","side":"CT","kind":"hold","site":"A","rounds":24,
			"seed":{"match_id":"` + testMatchID + `","round":1},"examples":[{"match_id":"` + testMatchID + `","round":1}]}]}`))
	})
	addTeamReportRoutes(mux)
	addFirstDeathRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := closedStudio(t)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)

	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerAnalysisTools(server, s)
	registerRoundTools(server, s)
	registerTeamReportTools(server, s)
	registerTeamGrenadesTool(server, s)
	registerPlayerReportTools(server, s)
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

// callTool decodes a successful tool call's JSON text into out.
func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any, out any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if res.IsError {
		t.Fatalf("%s: error result %s", name, text)
	}
	if err := json.Unmarshal([]byte(text), out); err != nil {
		t.Fatalf("%s: decode %s: %v", name, text, err)
	}
}

func toolFails(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any, want string) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !res.IsError || !strings.Contains(text, want) {
		t.Errorf("%s %v: got %q, want an error with %q", name, args, text, want)
	}
}

func TestListTeamsAndScoutTeam(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var teams listTeamsOut
	callTool(t, cs, "list_teams", nil, &teams)
	if len(teams.Teams) != 1 || teams.Teams[0].Name != "Vitality" || teams.Teams[0].Matches != 3 {
		t.Fatalf("teams = %+v", teams)
	}

	var scout scoutTeamOut
	callTool(t, cs, "scout_team", map[string]any{"team_id": testTeamID, "map": "de_nuke", "side": "t", "to": "2026-10-31"}, &scout)
	if scout.Team != "Vitality" || scout.Rounds != 22 || scout.RoundWinPct != 59.1 || scout.SkippedMatches != 1 {
		t.Fatalf("scout = %+v", scout)
	}
	if len(scout.Buys) != 1 || scout.Buys[0].Name != "full" || !scout.Buys[0].Reliable {
		t.Errorf("buys = %+v, want the full buy only", scout.Buys)
	}
	if len(scout.RecentMatches) != 1 || scout.RecentMatches[0].Score != "13-9" || scout.RecentMatches[0].PlayedAt != "2026-10-12" {
		t.Errorf("recent = %+v", scout.RecentMatches)
	}
	if len(scout.Executes) != 1 || scout.Executes[0].PlantPct != 83.3 || len(scout.AWP) != 1 || scout.Players[0].ADR != 98.2 {
		t.Errorf("executes %+v, awp %+v, players %+v", scout.Executes, scout.AWP, scout.Players)
	}
	toolFails(t, cs, "scout_team", map[string]any{"team_id": "Vitality"}, "team_id")
	toolFails(t, cs, "scout_team", map[string]any{"team_id": testTeamID, "side": "both"}, "side must be CT or T")
}

func TestPlayerMetricsUsesTheBaselineDefinitions(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var m playerMetricsOut
	callTool(t, cs, "player_metrics", map[string]any{"steamid": testSteamID}, &m)
	if m.Name != "ZywOo" || m.KPR != 0.8 || m.DPR != 0.5 || m.ADR != 85 || m.KAST != 75 || m.WPA != 0.06 {
		t.Fatalf("metrics = %+v", m)
	}
	if m.OpeningWinPct == nil || *m.OpeningWinPct != 60 || m.TradePct == nil || *m.TradePct != 40 {
		t.Errorf("opening %v, trade %v", m.OpeningWinPct, m.TradePct)
	}
	if m.UtilityDamage == nil || *m.UtilityDamage != 10 || m.FlashAssists == nil || *m.FlashAssists != 0.1 {
		t.Errorf("utility damage %v, flash assists %v: want rates over the 20 scanned rounds", m.UtilityDamage, m.FlashAssists)
	}
	if len(m.Buys) != 1 || m.Buys[0].WinPct != 60 {
		t.Errorf("buys = %+v", m.Buys)
	}
	toolFails(t, cs, "player_metrics", map[string]any{"steamid": testSteamID, "limit": 51}, "limit")

	var b baselineOut
	callTool(t, cs, "get_baseline", map[string]any{"scope": "PRO"}, &b)
	if b.Scope != "pro" || b.Players != 12 || b.Metrics["kpr"].Mean != 0.68 {
		t.Fatalf("baseline = %+v", b)
	}
	toolFails(t, cs, "get_baseline", map[string]any{"scope": "faceit"}, "scope must be library or pro")
}

func TestFindRoundsFiltersByTeamOrPlayer(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var out findRoundsOut
	callTool(t, cs, "find_rounds", map[string]any{"team_id": testTeamID, "buy": []string{"full"}}, &out)
	if out.MatchesRead != 1 || len(out.Rounds) != 1 || out.MatchCap != 50 || !out.MoreMatches {
		t.Fatalf("team full buys = %+v, want one round, the 50 match cap and older matches left out", out)
	}
	r := out.Rounds[0]
	if r.Ref != testMatchID+"/2" || r.Side != "CT" || r.OpponentBuy != "eco" || r.Won || r.Pattern != "aggression" || r.Opponent != "MOUZ" || r.Kills != nil {
		t.Errorf("round = %+v", r)
	}

	out = findRoundsOut{}
	callTool(t, cs, "find_rounds", map[string]any{"steamid": testSteamID, "result": "won"}, &out)
	if len(out.Rounds) != 1 || out.Rounds[0].Round != 1 || out.Rounds[0].Kills == nil || *out.Rounds[0].Kills != 2 || out.MoreMatches {
		t.Fatalf("player wins = %+v", out)
	}
	callTool(t, cs, "find_rounds", map[string]any{"steamid": testSteamID, "opponent_buy": []string{"eco"}}, &out)
	if len(out.Rounds) != 1 || out.Rounds[0].Round != 2 {
		t.Errorf("player rounds against an eco = %+v, want round 2", out.Rounds)
	}
	callTool(t, cs, "find_rounds", map[string]any{"team_id": testTeamID, "limit": 1}, &out)
	if len(out.Rounds) != 1 || !out.Truncated {
		t.Errorf("limit 1 = %+v, want one round and truncated", out)
	}
	callTool(t, cs, "find_rounds", map[string]any{"team_id": testTeamID, "side": "T"}, &out)
	if len(out.Rounds) != 0 {
		t.Errorf("T rounds of a team that only played CT = %+v", out.Rounds)
	}
	toolFails(t, cs, "find_rounds", map[string]any{"buy": []string{"full"}}, "exactly one of team_id or steamid")
	toolFails(t, cs, "find_rounds", map[string]any{"team_id": testTeamID, "buy": []string{"rich"}}, "buy must be one of")
	toolFails(t, cs, "find_rounds", map[string]any{"steamid": testSteamID, "lineup": []string{testSteamID}}, "pass team_id")
}

// A play's rounds are the ones it opens in the Replay, whoever's rounds the call reads.
func TestFindRoundsKeepsAPlaysRounds(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var out findRoundsOut
	callTool(t, cs, "find_rounds", map[string]any{"play_id": testPlayID}, &out)
	if len(out.Rounds) != 1 || out.Rounds[0].Ref != testMatchID+"/2" || out.Play != "Agresión en B" {
		t.Fatalf("Libreta play on its own team = %+v, want its example round 2", out)
	}
	callTool(t, cs, "find_rounds", map[string]any{"play_id": testPlayID, "steamid": testSteamID}, &out)
	if len(out.Rounds) != 1 || out.Rounds[0].Round != 2 || out.Rounds[0].Deaths == nil {
		t.Errorf("Libreta play for a player = %+v", out.Rounds)
	}
	callTool(t, cs, "find_rounds", map[string]any{"play_id": testMatchID + "/1", "team_id": testTeamID}, &out)
	if len(out.Rounds) != 1 || out.Rounds[0].Round != 1 || out.Rounds[0].Pattern != "hold" || !out.PlayCapped || out.PlayRounds != 24 {
		t.Errorf("detected play = %+v, want its round 1, capped from 24", out)
	}
	toolFails(t, cs, "find_rounds", map[string]any{"play_id": testMatchID + "/1"}, "needs the team_id")
	toolFails(t, cs, "find_rounds", map[string]any{"play_id": testMatchID + "/9", "team_id": testTeamID}, "no detected play has that ref")
}
