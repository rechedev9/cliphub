package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// wantRounds checks the r values of a studio_path, the match:round form the Replay parses.
func wantRounds(t *testing.T, path string, rounds ...int) {
	t.Helper()
	want := make([]string, 0, len(rounds))
	for _, r := range rounds {
		want = append(want, testMatchID+":"+strconv.Itoa(r))
	}
	if got := query(t, path)["r"]; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("rounds of %s = %v, want %v", path, got, want)
	}
}

func TestFindRoundsLinksItsRounds(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var out findRoundsOut
	callTool(t, cs, "find_rounds", map[string]any{"team_id": testTeamID}, &out)
	wantRounds(t, out.StudioPath, 1, 2)
	if q := query(t, out.StudioPath); q.Get("t") != "Vitality" || q.Has("jugador") || out.RoundsLinked != 0 {
		t.Errorf("team link = %v, rounds_linked %d", q, out.RoundsLinked)
	}

	callTool(t, cs, "find_rounds", map[string]any{"steamid": testSteamID, "result": "won"}, &out)
	wantRounds(t, out.StudioPath, 1)
	if got := query(t, out.StudioPath)["jugador"]; len(got) != 1 || got[0] != testSteamID {
		t.Errorf("player link focus = %v, want the player", got)
	}

	callTool(t, cs, "find_rounds", map[string]any{"play_id": testPlayID}, &out)
	wantRounds(t, out.StudioPath, 2)
	if title := query(t, out.StudioPath).Get("t"); !strings.Contains(title, "Agresión en B") {
		t.Errorf("play link title = %q", title)
	}

	var none findRoundsOut
	callTool(t, cs, "find_rounds", map[string]any{"team_id": testTeamID, "side": "T"}, &none)
	if none.StudioPath != "" {
		t.Errorf("no rounds link %q", none.StudioPath)
	}
}

func TestBombsiteRowsLinkTheirRounds(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var out teamBombsitesOut
	callTool(t, cs, "team_bombsites", map[string]any{"team_id": testTeamID}, &out)
	site := out.Maps[0].Sites[0]
	wantRounds(t, site.CT.StudioPath, 1, 2, 3, 4, 5, 6)
	ct := query(t, site.CT.StudioPath)
	if strings.Join(ct["zona"], ",") != "BombsiteA" || !strings.Contains(ct.Get("t"), "defensa del sitio A") || site.CT.RoundsLinked != 0 || len(site.CT.Refs) != 1 {
		t.Errorf("CT link = %v, rounds_linked %d, refs %v", ct, site.CT.RoundsLinked, site.CT.Refs)
	}
	wantRounds(t, site.T.StudioPath, 2, 3)
	if site.T.RoundsLinked != 2 || !strings.Contains(query(t, site.T.StudioPath).Get("t"), "ataque al sitio A") {
		t.Errorf("T link = %s, rounds_linked %d: the attack counts 5 rounds, the server listed 2", site.T.StudioPath, site.T.RoundsLinked)
	}
}

func TestPlaybookRowsLinkAllTheirRounds(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var pb teamPlaybookOut
	callTool(t, cs, "team_playbook", map[string]any{"team_id": testTeamID, "map": "de_nuke", "side": "t"}, &pb)
	rush := pb.Categories[0]
	wantRounds(t, rush.StudioPath, 1, 2, 3, 4, 5, 6, 7)
	if len(rush.Refs) != maxGroupRefs || rush.RoundsLinked != 0 || !strings.Contains(query(t, rush.StudioPath).Get("t"), "Vitality · de_nuke · T · rush") {
		t.Errorf("rush = %+v, want 5 refs and a path over its 7 rounds", rush)
	}
	zone := pb.Zones[0]
	wantRounds(t, zone.StudioPath, 3)
	if got := query(t, zone.StudioPath)["zona"]; len(got) != 1 || got[0] != "Outside" || zone.RoundsLinked != 1 {
		t.Errorf("zone link = %s, rounds_linked %d, want the zone token marked and 1 of 4 rounds", zone.StudioPath, zone.RoundsLinked)
	}
}

func TestPrepRowsLinkTheirRounds(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var prep teamPrepOut
	callTool(t, cs, "team_prep", map[string]any{"team_id": testTeamID, "map": "de_nuke"}, &prep)
	m := prep.Matchups[0]
	wantRounds(t, m.StudioPath, 2, 4)
	if m.RoundsLinked != 2 {
		t.Errorf("matchup rounds_linked = %d, want 2 of its 6 rounds", m.RoundsLinked)
	}
	spot := prep.CTPlayers[0].Windows[0].Spots[0]
	wantRounds(t, spot.StudioPath, 13)
	if q := query(t, spot.StudioPath); q.Get("jugador") != testSteamID || q.Get("tramo") != "0-20" || q.Get("zona") != "Ramp" {
		t.Errorf("spot link = %v, want the player, the window and the zone", q)
	}

	var fd teamPrepOut
	callTool(t, cs, "team_prep", map[string]any{"team_id": testTeamID, "map": "de_nuke", "side": "t", "movement": "slow"}, &fd)
	zone := fd.FirstDeath.Players[0].Zones[0]
	wantRounds(t, zone.StudioPath, 2)
	if q := query(t, zone.StudioPath); q.Get("jugador") != testSteamID || q.Get("tramo") != "0-15" || q.Get("zona") != "Outside" || zone.RoundsLinked != 1 {
		t.Errorf("first death link = %v, rounds_linked %d", q, zone.RoundsLinked)
	}
}

func TestDetectedPlaysLinkTheirExamples(t *testing.T) {
	cs, _ := stratbookStudio(t)
	var out detectPlaysOut
	callTool(t, cs, "detect_team_plays", map[string]any{"team_id": testTeamID}, &out)
	p := out.Plays[0]
	wantRounds(t, p.StudioPath, 7, 9)
	if p.RoundsLinked != 2 || !strings.Contains(query(t, p.StudioPath).Get("t"), "Vitality · de_mirage · T · execute · B") {
		t.Errorf("play = %+v, want its 2 example rounds of 5", p)
	}
}

// scoutLinksStudio serves a scouting report and the T rounds its scope kept.
func scoutLinksStudio(t *testing.T) *mcp.ClientSession {
	t.Helper()
	round := func(n int, buy, site, pattern, winner string) string {
		return fmt.Sprintf(`{"round":%d,"side":"T","winner":"%s","buy":"%s","opponent_buy":"full","site":"%s","pattern":"%s"}`, n, winner, buy, site, pattern)
	}
	rounds := strings.Join([]string{round(1, "pistol", "A", "execute", "T"), round(2, "full", "A", "split", "CT"),
		round(3, "full", "B", "default", "T"), round(4, "full", "A", "execute", "T")}, ",")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/scout", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"team_name":"Vitality","skipped":[],"matches":[],"maps":[],"tendencies":{"round_count":4,"wins":3,
			"buys":[{"buy":"full","rounds":3,"share":{"pct":75},"win_rate":{"pct":66.7}},{"buy":"pistol","rounds":1,"share":{"pct":25},"win_rate":{"pct":100}}],
			"sites":[{"site":"A","rounds":3,"share":{"pct":75},"win_rate":{"pct":66.7}},{"site":"B","rounds":1,"share":{"pct":25},"win_rate":{"pct":100}}],
			"t_patterns":[{"pattern":"execute","rounds":2,"share":{"pct":50},"win_rate":{"pct":100}}],
			"ct_patterns":[{"pattern":"execute","rounds":1,"share":{"pct":100},"win_rate":{"pct":0}}],
			"openings":{},"players":[]},"executes":[{"site":"A","rounds":3,"win_rate":{"pct":66.7},"plants":{"pct":100}}],"awp":{"sides":[]}}`))
	})
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/rounds", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"team_name":"Vitality","rounds":4,"skipped":[],"matches":[{"match_id":"` + testMatchID +
			`","map":"de_nuke","played_at":"2026-10-12T18:00:00Z","opponent":"MOUZ","total":4,"rounds":[` + rounds + `]}]}`))
	})
	return serveTools(t, mux, func(server *mcp.Server, s *studio) { registerAnalysisTools(server, s) })
}

func TestScoutRowsLinkTheRoundsTheyCount(t *testing.T) {
	cs := scoutLinksStudio(t)
	var out scoutTeamOut
	callTool(t, cs, "scout_team", map[string]any{"team_id": testTeamID}, &out)
	wantRounds(t, out.Buys[0].StudioPath, 2, 3, 4)
	wantRounds(t, out.Buys[1].StudioPath, 1)
	wantRounds(t, out.Sites[0].StudioPath, 1, 2, 4)
	wantRounds(t, out.Sites[1].StudioPath, 3)
	wantRounds(t, out.TPatterns[0].StudioPath, 1, 4)
	if out.CTPatterns[0].StudioPath != "" || out.CTPatterns[0].RoundsLinked != 0 {
		t.Errorf("a CT shape with no CT rounds links %+v", out.CTPatterns[0].linkOut)
	}
	wantRounds(t, out.Executes[0].StudioPath, 1, 2, 4)
	if title := query(t, out.Buys[0].StudioPath).Get("t"); title != "Vitality · compra full" {
		t.Errorf("buy link title = %q", title)
	}
}
