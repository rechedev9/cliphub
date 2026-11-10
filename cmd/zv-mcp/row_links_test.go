package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
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
	if strings.Join(ct["zona"], ",") != "BombsiteA" || !strings.Contains(ct.Get("t"), "Defensa del sitio A en CT") || site.CT.RoundsLinked != 0 || len(site.CT.Refs) != 1 {
		t.Errorf("CT link = %v, rounds_linked %d, refs %v", ct, site.CT.RoundsLinked, site.CT.Refs)
	}
	wantRounds(t, site.T.StudioPath, 2, 3)
	if site.T.RoundsLinked != 2 || !strings.Contains(query(t, site.T.StudioPath).Get("t"), "Ataque al sitio A en T") {
		t.Errorf("T link = %s, rounds_linked %d: the attack counts 5 rounds, the server listed 2", site.T.StudioPath, site.T.RoundsLinked)
	}
}

func TestPlaybookRowsLinkAllTheirRounds(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var pb teamPlaybookOut
	callTool(t, cs, "team_playbook", map[string]any{"team_id": testTeamID, "map": "de_nuke", "side": "t"}, &pb)
	rush := pb.Categories[0]
	wantRounds(t, rush.StudioPath, 1, 2, 3, 4, 5, 6, 7)
	if len(rush.Refs) != maxGroupRefs || rush.RoundsLinked != 0 || !strings.Contains(query(t, rush.StudioPath).Get("t"), "Vitality · Rush en T · Nuke") {
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
	wantRounds(t, p.StudioPath, 7, 9, 11)
	if p.RoundsLinked != 3 || !strings.Contains(query(t, p.StudioPath).Get("t"), "Vitality · Ejecución B en T · Mirage") {
		t.Errorf("play = %+v, want its 3 listed rounds of 5", p)
	}
}

// scoutLinksStudio serves a scouting report whose buckets carry every round they count.
func scoutLinksStudio(t *testing.T) *mcp.ClientSession {
	t.Helper()
	refs := func(rounds ...int) string {
		parts := make([]string, 0, len(rounds))
		for _, r := range rounds {
			parts = append(parts, fmt.Sprintf(`{"match_id":"%s","round":%d,"tick":10}`, testMatchID, r))
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/scout", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"team_name":"Vitality","skipped":[],"matches":[],"maps":[{"map":"de_nuke"}],"tendencies":{"round_count":4,"wins":3,
			"buys":[{"buy":"full","rounds":3,"share":{"pct":75},"win_rate":{"pct":66.7},"refs":` + refs(2, 3, 4) + `},
				{"buy":"pistol","rounds":1,"share":{"pct":25},"win_rate":{"pct":100},"refs":` + refs(1) + `}],
			"sites":[{"site":"A","rounds":3,"share":{"pct":75},"win_rate":{"pct":66.7},"refs":` + refs(1, 2, 4) + `},
				{"site":"B","rounds":1,"share":{"pct":25},"win_rate":{"pct":100},"refs":` + refs(3) + `}],
			"t_patterns":[{"pattern":"eco_rush","rounds":2,"share":{"pct":50},"win_rate":{"pct":100},"refs":` + refs(1, 4) + `}],
			"ct_patterns":[{"pattern":"hold","rounds":5,"share":{"pct":100},"win_rate":{"pct":0},"refs":` + refs(5, 6) + `}],
			"openings":{},"players":[]},"executes":[{"site":"A","rounds":3,"win_rate":{"pct":66.7},"plants":{"pct":100},"refs":` + refs(1, 2, 4) + `}],"awp":{"sides":[]}}`))
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
	wantRounds(t, out.CTPatterns[0].StudioPath, 5, 6)
	if out.CTPatterns[0].RoundsLinked != 2 {
		t.Errorf("CT shape rounds_linked = %d, want 2 of 5", out.CTPatterns[0].RoundsLinked)
	}
	wantRounds(t, out.Executes[0].StudioPath, 1, 2, 4)
	for path, want := range map[string]string{
		out.Buys[0].StudioPath:       "Vitality · Compra full · Nuke",
		out.Sites[1].StudioPath:      "Vitality · Rondas en B · Nuke",
		out.TPatterns[0].StudioPath:  "Vitality · Rush de eco en T · Nuke",
		out.CTPatterns[0].StudioPath: "Vitality · Defensa en CT · Nuke",
		out.Executes[0].StudioPath:   "Vitality · Ejecución A en T · Nuke",
	} {
		if got := query(t, path).Get("t"); got != want {
			t.Errorf("title = %q, want %q", got, want)
		}
	}
}

func TestLinkTitlesReadLikeTheWeb(t *testing.T) {
	scope := titleScope{team: "Navi", side: "T", mapName: "de_mirage"}
	if got, want := scope.of("Rush"), "Navi · Rush en T · Mirage"; got != want {
		t.Errorf("playbook title = %q, want %q", got, want)
	}
	if got := (titleScope{team: "Navi", mapName: "de_nuke"}).of("Full contra eco en T"); got != "Navi · Full contra eco en T · Nuke" {
		t.Errorf("prep title = %q", got)
	}
	for place, want := range map[string]string{"BombsiteA": "Bombsite A", "CTSpawn": "CT Spawn", "TopofMid": "Top of Mid", "Floor50": "50th Floor"} {
		if got := placeLabel(place); got != want {
			t.Errorf("placeLabel(%s) = %q, want %q", place, got, want)
		}
	}
	tactic := tuiclient.PlaybookTactic{Kind: "utility", Grenade: "smoke", Value: "Window", Label: "Ventana"}
	if got := tacticLabel(tactic); got != "Humo en Ventana" {
		t.Errorf("tacticLabel = %q", got)
	}
	if got := playShapeLabel("execute", "A"); got != "Ejecución A" {
		t.Errorf("playShapeLabel = %q", got)
	}
	for _, c := range []struct {
		from, to int
		want     string
	}{{0, 0, "0-115"}, {0, 55, "0-55"}, {75, 0, "75-115"}, {0, 300, "0-115"}, {120, 0, ""}, {60, 60, ""}, {70, 20, ""}} {
		if got := windowTramo(c.from, c.to); got != c.want {
			t.Errorf("windowTramo(%d, %d) = %q, want %q", c.from, c.to, got, c.want)
		}
	}
}
