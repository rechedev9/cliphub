package main

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestPreparePathOpensTheSameLineup(t *testing.T) {
	if got, want := preparePath("t1", tuiclient.AnalysisQuery{Map: "de_nuke"}), "/equipos/t1?lineup=all&map=de_nuke&tab=preparar"; got != want {
		t.Errorf("no lineup = %s, want %s", got, want)
	}
	q := tuiclient.AnalysisQuery{Lineup: []string{"76561198000000002", "76561198000000001", "76561198000000002"}}
	if got, want := preparePath("t1", q), "/equipos/t1?lineup=76561198000000001%2C76561198000000002&tab=preparar"; got != want {
		t.Errorf("lineup = %s, want %s", got, want)
	}
}

// addTeamReportRoutes serves the team and player reports of fakeAnalysisStudio.
func addTeamReportRoutes(mux *http.ServeMux) {
	ref := `{"match_id":"` + testMatchID + `","round":%s,"tick":100}`
	refs := func(rounds ...string) string {
		parts := make([]string, 0, len(rounds))
		for _, r := range rounds {
			parts = append(parts, fmt.Sprintf(ref, r))
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/bombsites", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"team_name":"Vitality","skipped":[],"match_count":1,"maps":[{"map":"de_nuke","matches":1,"ct_rounds":12,"t_rounds":10,
			"sites":[{"site":"A","ct":{"hit":{"count":6,"pct":50},"held":{"count":4,"pct":66.7,"reliable":true},"plants_allowed":{"pct":33.3},"won":{"pct":66.7},
			"retakes_attempted":{"count":2},"retakes_won":{"pct":50},"examples":` + refs("1") + `},
			"t":{"attacks":{"count":5,"pct":50},"plants":{"pct":80},"won":{"pct":60},"post_plant_won":{"pct":75},"examples":[]}}]}]}`))
	})
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/playbook", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("side") != "T" || r.URL.Query().Get("map") != "de_nuke" {
			http.Error(w, `{"error":"map and side are required"}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"team_name":"Vitality","skipped":[],"map":"de_nuke","side":"T","matches":1,"round_count":10,"win_rate":{"pct":60},"framed":8,
			"categories":[{"category":"rush","rounds":7,"share":{"pct":70},"win_rate":{"pct":57.1,"reliable":true},"refs":` + refs("1", "2", "3", "4", "5", "6", "7") + `},
			{"category":"fake","rounds":0,"share":{},"win_rate":{},"refs":[]}],
			"tempo":{"samples":9,"median":22.5},"sites":[],"zones":[{"zone":"Outside","label":"Afuera","rounds":4,"share":{"pct":50},"win_rate":{"pct":75},"refs":` + refs("3") + `}]}`))
	})
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/prep", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("side") {
			http.Error(w, `{"error":"the prep reads both sides; drop side"}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"team_name":"Vitality","skipped":[],"map":"de_nuke","matches":1,"round_count":22,"framed":10,
			"matchups":[{"side":"T","buy":"full","opponent_buy":"full","rounds":6,"share":{"pct":60},"win_rate":{"pct":66.7},"examples":` + refs("2") + `,"refs":` + refs("2", "4") + `}],
			"tempo":[{"side":"T","buy":"full","rounds":6,"win_rate":{"pct":66.7},"tempo":{"samples":6,"median":50},"plant":{"samples":2,"median":60}}],
			"ct_players":[{"steamid64":"` + testSteamID + `","name":"Zyw‮Oo","role":"awp","rounds":10,
			"windows":[{"from_seconds":0,"to_seconds":20,"rounds":10,"spots":[{"callout":"Ramp","share":{"count":6,"pct":60},"refs":` + refs("13") + `}]},
			{"from_seconds":60,"to_seconds":0,"rounds":3,"spots":[]}]}]}`))
	})
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/grenades", func(w http.ResponseWriter, r *http.Request) {
		if kinds := r.URL.Query()["kind"]; len(kinds) != 2 || kinds[0] != "smoke" || kinds[1] != "flash" {
			http.Error(w, `{"error":"unexpected kinds `+r.URL.RawQuery+`"}`, http.StatusBadRequest)
			return
		}
		g := `{"match_id":"` + testMatchID + `","round":%s,"tick":%s000,"seconds":%s,"kind":"%s","side":"T","thrower_steamid64":"%s","thrower_name":"p",` +
			`"landing":[10.4,20.6,-3],"land_place":"Ramp","spot":"%s","after":{"zones":["Middle"],"fight":"Middle"},"round_won":%s,"tickrate":64}`
		row := func(round, secs, kind, thrower, spot string) string {
			return fmt.Sprintf(g, round, round, secs, kind, thrower, spot, strconv.FormatBool(round != "3"))
		}
		// The rivals' rounds keep the team's own buy in team_buy; round 3 was planted at 50 s.
		round := `{"match_id":"` + testMatchID + `","round":%d,"tick":100,"side":"T","buy":"%s","opponent_buy":"%[2]s","team_buy":"%[2]s","won":%t,"category":"execute","site":"A"%s}`
		if r.URL.Query().Get("opponents") == "1" {
			round = strings.Replace(round, `"buy":"%s"`, `"buy":"eco"`, 1)
		}
		rounds := []string{fmt.Sprintf(round, 1, "full", true, ""), fmt.Sprintf(round, 2, "full", true, ""),
			fmt.Sprintf(round, 3, "eco", false, `,"plant_seconds":50`), fmt.Sprintf(round, 4, "full", false, "")}
		grenades := []string{row("1", "5", "smoke", grenadeA, "smoke:one"), row("1", "7", "smoke", grenadeB, "smoke:two"),
			row("2", "3", "flash", grenadeA, "flash:three"),
			strings.Replace(row("2", "9", "smoke", grenadeA, "smoke:one"), `"landing"`, `"origin":[1,2,3],"throw_tick":640,"view":[90,-10],"landing"`, 1),
			row("3", "60", "smoke", grenadeB, "smoke:two")}
		// Like the server, buy keeps the rounds of the team's own buy, also with opponents: round 3 is the only eco.
		if r.URL.Query().Get("buy") == "full" {
			rounds = slices.Delete(rounds, 2, 3)
			grenades = grenades[:4]
		}
		_, _ = w.Write([]byte(`{"rounds":` + strconv.Itoa(len(rounds)) + `,"round_list":[` + strings.Join(rounds, ",") + `],"skipped":[],"grenades":[` +
			strings.Join(grenades, ",") + `]}`))
	})
	mux.HandleFunc("GET /api/library/maps/de_nuke/callouts", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"map":"de_nuke","callouts":[{"token":"Ramp","name":"Rampa"},{"token":"Hut"}]}`))
	})
	mux.HandleFunc("GET /api/library/players/"+testSteamID+"/positions", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"steamid64":"` + testSteamID + `","match_count":2,"window_seconds":25,"min_reliable_sample":5,"skipped":[],
			"maps":[{"map":"de_nuke","matches":2,"sides":[{"side":"CT","rounds":24,"sampled":20,"role":"a_anchor","map_role":{"role":"a_anchor","map_role":"Hut","label":"Ancla A"},
			"zones":[{"zone":"A","rounds":{"count":14,"pct":70}}],"places":[{"place":"Hut","label":"Hut","rounds":{"count":8,"pct":40}}],
			"awp_kills":{"pct":80},"awp_rounds":{"pct":45}}]}]}`))
	})
	mux.HandleFunc("GET /api/library/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("metric") != "adr" || q.Get("limit") != "20" {
			http.Error(w, `{"error":"unexpected query `+r.URL.RawQuery+`"}`, http.StatusBadRequest)
			return
		}
		if q.Get("scope") == "pro" {
			_, _ = w.Write([]byte(`{"scope":"pro","map":"","metric":"adr","higher_is_better":true,"min_rounds":50,"matches":0,"rounds":0,"skipped":0,"players":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"scope":"library","map":"","metric":"adr","higher_is_better":true,"min_rounds":50,"matches":40,"rounds":900,"skipped":1,
			"map_roles":[{"key":"Canal","label":"Canal","players":3},{"key":"allround","label":"Todoterreno","players":1}],"events":["IEM Cologne"],
			"players":[{"rank":1,"steamid64":"` + testSteamID + `","name":"ZywOo\u0007","rounds":400,"matches":18,"value":92.4,
				"metrics":{"adr":92.4,"rating":1.21},"roles":{"ct":"b_anchor","t":"b_lurk"},
				"breakdown":{"ct":{"role":"b_anchor","roles":[{"key":"b_anchor","pct":80},{"key":"rotator","pct":20}],"map_role":"Canal","places":[{"key":"Canal","label":"Canal","pct":60}]}},
				"examples":[{"match_id":"` + testMatchID + `","round":3,"tick":10}]}]}`))
	})
}

func TestTeamBombsitesPlaybookAndPrep(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var sites teamBombsitesOut
	callTool(t, cs, "team_bombsites", map[string]any{"team_id": testTeamID}, &sites)
	if len(sites.Maps) != 1 || len(sites.Maps[0].Sites) != 1 {
		t.Fatalf("bombsites = %+v", sites)
	}
	a := sites.Maps[0].Sites[0]
	if a.CT.HitRounds != 6 || a.CT.HeldPct != 66.7 || !a.CT.Reliable || a.CT.RetakeAttempts != 2 || a.T.Attacks != 5 || a.T.PostPlantWinPct != 75 {
		t.Errorf("site A = %+v", a)
	}
	if len(a.CT.Refs) != 1 || a.CT.Refs[0] != testMatchID+"/1" {
		t.Errorf("CT refs = %v", a.CT.Refs)
	}

	var pb teamPlaybookOut
	callTool(t, cs, "team_playbook", map[string]any{"team_id": testTeamID, "map": "de_nuke", "side": "t"}, &pb)
	if pb.Rounds != 10 || pb.FramedRounds != 8 || pb.Tempo.MedianSeconds != 22.5 || len(pb.Categories) != 1 {
		t.Fatalf("playbook = %+v, want the empty fake left out", pb)
	}
	if rush := pb.Categories[0]; rush.Name != "rush" || rush.Rounds != 7 || len(rush.Refs) != maxGroupRefs {
		t.Errorf("rush = %+v, want refs capped at %d", rush, maxGroupRefs)
	}
	if len(pb.Zones) != 1 || pb.Zones[0].Name != "Afuera" {
		t.Errorf("zones = %+v, want the label", pb.Zones)
	}
	toolFails(t, cs, "team_playbook", map[string]any{"team_id": testTeamID, "map": "de_nuke"}, "map and side are required")

	var prep teamPrepOut
	callTool(t, cs, "team_prep", map[string]any{"team_id": testTeamID, "map": "de_nuke"}, &prep)
	if len(prep.Matchups) != 1 || len(prep.Matchups[0].Refs) != 2 || prep.Matchups[0].OpponentBuy != "full" {
		t.Fatalf("matchups = %+v, want every ref", prep.Matchups)
	}
	if len(prep.Tempo) != 1 || prep.Tempo[0].Tempo.MedianSeconds != 50 || prep.Tempo[0].Plant.Samples != 2 {
		t.Errorf("tempo = %+v", prep.Tempo)
	}
	if len(prep.CTPlayers) != 1 || prep.CTPlayers[0].Name != "ZywOo" {
		t.Fatalf("players = %+v, want the name cleaned", prep.CTPlayers)
	}
	w := prep.CTPlayers[0].Windows
	if len(w) != 2 || w[0].Window != "1:55 to 1:35" || w[1].Window != "0:55 to the end" || w[0].Spots[0].Callout != "Ramp" || w[0].Spots[0].Rounds != 6 {
		t.Errorf("windows = %+v", w)
	}
	toolFails(t, cs, "team_prep", map[string]any{"team_id": testTeamID, "map": ""}, "map is required")
}

const (
	grenadeA = "76561198000000001"
	grenadeB = "76561198000000002"
)

func TestTeamGrenadesGroupsByKind(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var out teamGrenadesOut
	callTool(t, cs, "team_grenades", map[string]any{"team_id": testTeamID, "map": "de_nuke", "kind": []string{"Smoke", "flash"}, "limit": 3}, &out)
	if out.Total != 5 || out.Rounds != 4 || out.MatchingRounds != 3 || len(out.Kinds) != 2 || len(out.Grenades) != 3 || !out.Truncated || out.Note == "" {
		t.Fatalf("grenades = %+v", out)
	}
	smoke := out.Kinds[0]
	if smoke.Kind != "smoke" || smoke.Count != 4 || smoke.T != 4 || smoke.MedianSeconds != 8 || smoke.PerRound != 1 {
		t.Errorf("smoke = %+v", smoke)
	}
	if len(smoke.Throwers) != 2 || smoke.Throwers[0].SteamID64 != grenadeA || smoke.Throwers[0].Count != 2 {
		t.Errorf("throwers = %+v", smoke.Throwers)
	}
	if g := out.Grenades[0]; g.Ref != testMatchID+"/1" || g.Landing != [3]int{10, 21, -3} || g.Origin != nil || g.ThrowTick != nil {
		t.Errorf("first row = %+v", g)
	}
	var all teamGrenadesOut
	callTool(t, cs, "team_grenades", map[string]any{"team_id": testTeamID, "map": "de_nuke", "kind": []string{"smoke", "flash"}}, &all)
	if g := all.Grenades[3]; g.ThrowTick == nil || *g.ThrowTick != 640 || g.Origin == nil {
		t.Errorf("a recorded throw = %+v, want the throw_tick save_lineup reads", g)
	}
	toolFails(t, cs, "team_grenades", map[string]any{"team_id": testTeamID, "map": "de_nuke", "kind": []string{"c4"}}, "kind must be one of")
}

func TestTeamGrenadesReadsTheClockSpotsAndRounds(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	kinds := []string{"smoke", "flash"}
	var out teamGrenadesOut
	callTool(t, cs, "team_grenades", map[string]any{"team_id": testTeamID, "map": "de_nuke", "kind": kinds}, &out)
	if len(out.Clock) != 2 || out.Clock[0].Span != "1:55-1:45" || out.Clock[0].Throws != 4 || out.Clock[1].Span != "Tras la plant" {
		t.Fatalf("clock = %+v, want the opening bin and the smoke after the plant", out.Clock)
	}
	if p := out.Clock[0].Players; len(p) != 2 || p[0].SteamID64 != grenadeA || p[0].Throws != 3 || p[1].Throws != 1 {
		t.Errorf("opening bin players = %+v", p)
	}
	if len(out.Spots) != 3 || out.Spots[0].Key != "smoke:one" || out.Spots[0].Rounds != 2 || out.Spots[0].LandPlace != "Ramp" || len(out.Spots[0].After) != 3 {
		t.Fatalf("spots = %+v", out.Spots)
	}

	var spot teamGrenadesOut
	callTool(t, cs, "team_grenades", map[string]any{"team_id": testTeamID, "map": "de_nuke", "kind": kinds, "spot": "smoke:one"}, &spot)
	d := spot.Spot
	if d == nil || spot.MatchingRounds != 2 || d.Rounds != 2 || d.Won != 2 || len(d.RoundList) != 2 || d.RoundList[1].Ref != testMatchID+"/2" {
		t.Fatalf("spot = %+v over %d rounds", d, spot.MatchingRounds)
	}
	want := []followUpOut{{Kind: "play", Value: "execute@A", Rounds: 2, Won: 2}, {Kind: "fight", Value: "Middle", Rounds: 2, Won: 2}, {Kind: "zone", Value: "Middle", Rounds: 2, Won: 2}}
	if fmt.Sprint(d.After) != fmt.Sprint(want) {
		t.Errorf("after = %+v, want %+v", d.After, want)
	}
	if d.Setpos != "setpos 1.00 2.00 3.00;setang -10.00 90.00 0" {
		t.Errorf("setpos = %q", d.Setpos)
	}
	if !strings.Contains(d.StudioPath, "r="+testMatchID+"%3A1%3A808") || !strings.Contains(d.StudioPath, "%3A2%3A448") || !strings.HasSuffix(d.StudioPath, "s=1&t=Humo+a+Rampa+%C2%B7+2+rondas") {
		t.Errorf("studio_path = %s, want both rounds from before the throw, superposed", d.StudioPath)
	}

	var inverted teamGrenadesOut
	callTool(t, cs, "team_grenades", map[string]any{"team_id": testTeamID, "map": "de_nuke", "kind": kinds, "spot": "smoke:one", "invert": true}, &inverted)
	if inverted.MatchingRounds != 2 || len(inverted.Inverted) != 2 || inverted.Inverted[0].Ref != testMatchID+"/3" || inverted.Inverted[1].Play != "execute@A" {
		t.Errorf("inverted = %+v over %d rounds", inverted.Inverted, inverted.MatchingRounds)
	}

	var full teamGrenadesOut
	callTool(t, cs, "team_grenades", map[string]any{"team_id": testTeamID, "map": "de_nuke", "kind": kinds, "buy": []string{"full"}, "players": []string{grenadeB}}, &full)
	if full.Rounds != 3 || full.Total != 1 || full.MatchingRounds != 1 || full.Grenades[0].Ref != testMatchID+"/1" {
		t.Errorf("full buys of B = %+v, want B's one smoke over the 3 full rounds", full)
	}
	// With the rivals' utility, buy still reads the scouted team's buy.
	var rivals teamGrenadesOut
	callTool(t, cs, "team_grenades", map[string]any{"team_id": testTeamID, "map": "de_nuke", "kind": kinds, "buy": []string{"full"}, "opponents": true}, &rivals)
	if rivals.Rounds != 3 {
		t.Errorf("opponents with the team's full buys = %d rounds, want 3", rivals.Rounds)
	}
	toolFails(t, cs, "team_grenades", map[string]any{"team_id": testTeamID, "kind": kinds}, "map is required")
	toolFails(t, cs, "team_grenades", map[string]any{"team_id": testTeamID, "map": "de_nuke", "kind": kinds, "max_matches": 51}, "max_matches")
	toolFails(t, cs, "team_grenades", map[string]any{"team_id": testTeamID, "map": "de_nuke", "kind": kinds, "spot": "smoke:nope"}, "no spot")
	toolFails(t, cs, "team_grenades", map[string]any{"team_id": testTeamID, "map": "de_nuke", "kind": kinds, "players": []string{"zywoo"}}, "is not a SteamID64")
}

func TestTeamGrenadesSendsTheTeamScope(t *testing.T) {
	in := teamGrenadesIn{TeamID: testTeamID, Opponents: true, analysisFilterIn: analysisFilterIn{Map: "de_nuke"},
		teamScopeIn: teamScopeIn{teamRoundScopeIn: teamRoundScopeIn{MaxMatches: 5, Buy: []string{"full"}, AWP: "with", OpponentTop: 30, RoundFrom: 3}}}
	read, err := in.read()
	if err != nil {
		t.Fatal(err)
	}
	if got := read.Query.Scope.Encode(); got != "awp=with&buy=full&max_matches=5&opponent_top=30&round_from=3" {
		t.Errorf("scope = %s, want the team scope keys every team tool sends", got)
	}
	if !read.Opponents {
		t.Error("opponents was dropped")
	}
}

func TestPlayerPositionsAndLeaderboard(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var pos playerPositionsOut
	callTool(t, cs, "player_positions", map[string]any{"steamid": testSteamID, "map": "de_nuke"}, &pos)
	if pos.Matches != 2 || pos.WindowSeconds != 25 || len(pos.Maps) != 1 || len(pos.Maps[0].Sides) != 1 {
		t.Fatalf("positions = %+v", pos)
	}
	ct := pos.Maps[0].Sides[0]
	if ct.Role != "a_anchor" || ct.MapRole == nil || ct.MapRole.Label != "Ancla A" || ct.MapRole.Side != "CT" || !ct.ReliableShare || ct.Zones[0].Pct != 70 || ct.Places[0].Name != "Hut" || ct.AWPKillPct != 80 {
		t.Errorf("CT = %+v", ct)
	}

	var board leaderboardOut
	callTool(t, cs, "leaderboard", map[string]any{"metric": "ADR"}, &board)
	if !board.HigherIsBetter || board.SkippedMatches != 1 || len(board.Players) != 1 || board.Players[0].Name != "ZywOo" || board.Players[0].Value != 92.4 {
		t.Fatalf("leaderboard = %+v", board)
	}
	if row := board.Players[0]; row.Metrics["rating"] != 1.21 || row.Roles == nil || row.Roles.T != "b_lurk" || len(row.Refs) != 1 || row.Refs[0] != testMatchID+"/3" {
		t.Fatalf("row = %+v, want the metrics, the roles and the round ref", row)
	}
	if len(board.MapRoles) != 2 || board.MapRoles[0].Key != "Canal" || len(board.Events) != 1 {
		t.Fatalf("map roles = %+v events = %v, want the positions and the events", board.MapRoles, board.Events)
	}
	if b := board.Players[0].Breakdown; b == nil || b.CT == nil || b.CT.MapRole != "Canal" || len(b.CT.Roles) != 2 || b.CT.Places[0].Pct != 60 || b.T != nil {
		t.Fatalf("breakdown = %+v, want the CT shares", b)
	}
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "map_role": "Canal"}, "map_role needs a map and a side")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "map": "de_anubis", "map_role": "Canal"}, "map_role needs a map and a side")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "team_top": 15}, "days, top, team_top and event are for scope pro")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "event": "IEM"}, "days, top, team_top and event are for scope pro")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "scope": "pro", "team_top": 20}, "team_top must be")
	var proBoard leaderboardOut
	callTool(t, cs, "leaderboard", map[string]any{"metric": "adr", "scope": "pro"}, &proBoard)
	if len(proBoard.Players) != 0 || !strings.Contains(proBoard.Note, "not available yet") {
		t.Fatalf("empty pro board = %+v, want a note that the pro data is not available", proBoard)
	}
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "nope"}, "metric must be one of")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "role": "b_lurk"}, "role needs a map")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "scope": "pro", "role": "awp"}, "need scope library")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "scope": "pro", "side": "T", "map_role": "allround"}, "pro scope reads map_role on one map")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "map_role": "allround"}, "map_role needs a map and a side")
	callTool(t, cs, "leaderboard", map[string]any{"metric": "adr", "role": "awp"}, &board)
	callTool(t, cs, "leaderboard", map[string]any{"metric": "adr", "side": "T", "map_role": "allround"}, &board)
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "map": "de_anubis", "role": "boss"}, "role must be")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "source": "x"}, "source must be")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "days": 30}, "days, top, team_top and event are for scope pro")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "scope": "pro", "from": "2026-10-01"}, "for the library scope")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "scope": "pro", "top": 20}, "top must be")
	toolFails(t, cs, "leaderboard", map[string]any{"metric": "adr", "limit": 101}, "limit")
}
