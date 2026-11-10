package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func attackStudio(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ref := func(round string) string {
		return `{"match_id":"` + testMatchID + `","round":` + round + `,"tick":100}`
	}
	group := func(rounds string, refs ...string) string {
		return `"rounds":` + rounds + `,"share":{"pct":50},"win_rate":{"pct":75,"reliable":true},"refs":[` + strings.Join(refs, ",") + `]`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/attack", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("map") != "de_nuke" || r.URL.Query().Has("side") {
			http.Error(w, `{"error":"map is required; drop side"}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"team_name":"Vitality","skipped":[],"map":"de_nuke","matches":2,"round_count":20,"framed":18,
			"last_played_at":"2026-09-01T18:00:00Z","days_since_last_match":70,
			"sites":[{"key":"A",` + group("2", ref("1"), ref("3")) + `},{"key":"none",` + group("0") + `}],
			"tempo":[{"key":"rush",` + group("1", ref("1")) + `}],
			"matchups":[{"buy":"full","opponent_buy":"full",` + group("2", ref("1"), ref("3")) + `,
				"sites":[{"key":"A",` + group("2", ref("1"), ref("3")) + `},{"key":"B",` + group("0") + `}],"tempo":[]}],
			"defaults":[{"formation":"2-1-2",` + group("1", ref("3")) + `,"zones":[{"zone":"Outside","label":"Outside",` + group("1", ref("3")) + `}]}],
			"executes":[{"buy":"full","opponent_buy":"full","site":"A","route":"Hut","route_label":"Hut",` + group("1", ref("1")) +
			`,"rushes":{"pct":100},"plants":{"pct":100},"contact_seconds":21}],
			"players":[{"steamid64":"` + testSteamID + `","name":"apEX","role":"igl","rounds":18,"windows":[{"from_seconds":0,"to_seconds":55,"rounds":18,
				"spots":[{"callout":"Ramp","share":{"count":9,"pct":50},"refs":[` + ref("1") + `],"arrival_seconds":12.5}],
				"pace":{"static":{"pct":20},"slow":{"pct":50},"fast":{"pct":30}}},{"from_seconds":75,"to_seconds":0,"rounds":4,"spots":[],"pace":{}}]}],
			"warnings":[{"kind":"stale","days":70},{"kind":"roster_changed","joined":["new"],"left":["old"]}]}`))
	})
	mux.HandleFunc("GET /api/library/maps/de_nuke/callouts", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("geometry") != "true" {
			http.Error(w, `{"error":"want the geometry"}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"map":"de_nuke","callouts":[],"geometry":{"cell_size":64,"levels":[{"name":"default","cells":[[0,0,3],[1,0,2],[3,0,1],[0,-1,1]]}],
			"callouts":[{"name":"Outside","level":"default","center":[96,32],"cells":[[0,0],[1,0]]},{"name":"Outside","level":"lower","center":[0,0],"cells":[[5,5]]}]}}`))
	})
	mux.HandleFunc("GET /api/library/maps/de_dust2/callouts", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"map":"de_dust2","callouts":[]}`))
	})
	return serveTools(t, mux, func(server *mcp.Server, s *studio) {
		registerTeamAttackTool(server, s)
		registerZoneMapTool(server, s)
		registerMatchPrepPrompt(server)
	})
}

// query reads a studio_path's query back.
func query(t *testing.T, path string) url.Values {
	t.Helper()
	_, raw, ok := strings.Cut(path, "?")
	if !ok || !strings.HasPrefix(path, "/replay?") {
		t.Fatalf("studio_path %q is not a Replay playlist", path)
	}
	v, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestTeamAttackLinksEveryFinding(t *testing.T) {
	cs := attackStudio(t)
	var out teamAttackOut
	callTool(t, cs, "team_attack", map[string]any{"team_id": testTeamID, "map": "de_nuke"}, &out)
	if out.Rounds != 20 || out.FramedRounds != 18 || out.DaysSinceLast != 70 || out.StudioPath != preparePath(testTeamID, tuiclient.AnalysisQuery{Map: "de_nuke"}) {
		t.Fatalf("attack = %+v", out)
	}
	if len(out.Warnings) != 2 || !strings.Contains(out.Warnings[0].Message, "70 days") || !strings.Contains(out.Warnings[1].Message, "new joined") {
		t.Errorf("warnings = %+v", out.Warnings)
	}
	site := query(t, out.Sites[0].StudioPath)
	if got := site["r"]; len(got) != 2 || got[0] != testMatchID+":1" || site.Get("zona") != "BombsiteA" || site.Get("t") != "Vitality · de_nuke · sitio A" {
		t.Errorf("site A link = %v", site)
	}
	if out.Sites[1].StudioPath != "" {
		t.Errorf("a site without rounds links %q", out.Sites[1].StudioPath)
	}
	if m := out.Matchups[0]; len(m.Sites) != 1 || m.Sites[0].Key != "A" || m.Sites[0].StudioPath != "" || len(m.Sites[0].Examples) != 0 ||
		!strings.Contains(query(t, m.StudioPath).Get("t"), "compra completa contra compra completa") {
		t.Errorf("matchup = %+v, want the empty B cell left out and the cells with counts only", m)
	}
	if out.AllRounds.Rounds != 2 || len(query(t, out.AllRounds.StudioPath)["r"]) != 2 || out.AllRounds.RoundsLinked != 0 {
		t.Errorf("all rounds = %+v", out.AllRounds)
	}
	if c := out.Executes[0].ContactSeconds; c == nil || *c != 21 {
		t.Errorf("contact = %v, want 21", c)
	}
	def := query(t, out.Defaults[0].StudioPath)
	if def.Get("tramo") != "0-55" || def.Get("zona") != "Outside" || out.Defaults[0].Zones[0].Callout != "Outside" {
		t.Errorf("default link = %v", def)
	}
	if exec := query(t, out.Executes[0].StudioPath); strings.Join(exec["zona"], ",") != "Hut,BombsiteA" || out.Executes[0].RushPct != 100 {
		t.Errorf("execute link = %v", exec)
	}
	w := out.Players[0].Windows
	if len(w) != 2 || w[0].Tramo != "0-55" || w[1].Tramo != "75-115" || w[0].SlowPct != 50 {
		t.Fatalf("windows = %+v", w)
	}
	spot := query(t, w[0].Spots[0].StudioPath)
	if spot.Get("jugador") != testSteamID || spot.Get("tramo") != "0-55" || spot.Get("zona") != "Ramp" || w[0].Spots[0].ArrivalSeconds != 12.5 {
		t.Errorf("spot link = %v", spot)
	}
	toolFails(t, cs, "team_attack", map[string]any{"team_id": testTeamID}, "missing properties")
}

func TestFindingLinkCapsAndDeduplicates(t *testing.T) {
	refs := []tuiclient.Evidence{{MatchID: testMatchID, Round: 2}, {MatchID: testMatchID, Round: 2}, {MatchID: "not-an-id", Round: 3}}
	for i := 1; i <= maxPlaylistRounds+5; i++ {
		refs = append(refs, tuiclient.Evidence{MatchID: testMatchID, Round: i})
	}
	v := query(t, findingLink{refs: refs, focus: []string{"bad"}, zones: make([]string, 20)}.path())
	if len(v["r"]) != maxPlaylistRounds || v["r"][0] != testMatchID+":2" || v.Has("jugador") || v.Has("zona") {
		t.Errorf("link = %d rounds, %v", len(v["r"]), v)
	}
	if got := (findingLink{refs: refs[2:3]}).path(); got != "" {
		t.Errorf("a link without a valid round = %q", got)
	}
}

func TestShowZonesDrawsTheMapInMeta(t *testing.T) {
	cs := attackStudio(t)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "show_zones", Arguments: map[string]any{"map": "de_nuke", "zones": []string{"Outside", "Heaven"}}})
	if err != nil || res.IsError {
		t.Fatalf("show_zones = %+v, %v", res, err)
	}
	drawing, ok := res.Meta[zoneMapMetaKey].(map[string]any)
	if !ok {
		t.Fatalf("meta = %v", res.Meta)
	}
	// Cells 0 and 1 of row 0 share block 0 and cell 3 is block 1; cell (0,-1) is block (0,-1), drawn first.
	if drawing["silhouette"] != "M0 0h128v128h-128zM0 -128h256v128h-256z" || drawing["view_box"] != "-256 -384 768 768" {
		t.Errorf("silhouette = %v, view box %v", drawing["silhouette"], drawing["view_box"])
	}
	zones, _ := drawing["zones"].([]any)
	if len(zones) != 2 {
		t.Fatalf("zones = %v, want Outside on both floors", zones)
	}
	if first, _ := zones[0].(map[string]any); first["path"] != "M0 -64h128v64h-128z" || first["y"] != float64(-32) {
		t.Errorf("Outside = %v", first)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Outside, Heaven") {
		t.Errorf("text = %q", text)
	}
	toolFails(t, cs, "show_zones", map[string]any{"map": "de_dust2", "zones": []string{"Long"}}, "no match of this map")
	toolFails(t, cs, "show_zones", map[string]any{"map": "de_nuke", "zones": []string{}}, "zones takes from 1")
}

func TestMatchPrepPromptLaysOutTheFlow(t *testing.T) {
	cs := attackStudio(t)
	got, err := cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "match_prep", Arguments: map[string]string{"rival": "Vitality", "maps": "de_nuke"}})
	if err != nil {
		t.Fatal(err)
	}
	text := got.Messages[0].Content.(*mcp.TextContent).Text
	for _, want := range []string{"against Vitality on de_nuke", "prepare_team", "show_zones", "create_report", "studio_path"} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt lacks %q: %s", want, text)
		}
	}
	if _, err := cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "match_prep", Arguments: map[string]string{}}); err == nil {
		t.Error("a prompt without a rival did not fail")
	}
}

// seededAttack is the attack read of a 20 match library: 300 T rounds, every row with all its refs.
func seededAttack() string {
	refs := make([]string, 0, 300)
	for m := range 20 {
		id := fmt.Sprintf("6f1c2a9e-3b7d-4c8a-9e21-%012d", m)
		for r := 1; r <= 15; r++ {
			refs = append(refs, fmt.Sprintf(`{"match_id":"%s","round":%d,"tick":%d}`, id, r, r*1000))
		}
	}
	group := func(from, n int) string {
		return fmt.Sprintf(`"rounds":%d,"share":{"count":%d,"total":300,"pct":10},"win_rate":{"pct":50,"reliable":true},"refs":[%s]`,
			n, n, strings.Join(refs[from:from+n], ","))
	}
	cells := func(keys ...string) string {
		out := []string{}
		for i, k := range keys {
			out = append(out, `{"key":"`+k+`",`+group(i*60, 60)+`}`)
		}
		return "[" + strings.Join(out, ",") + "]"
	}
	bands := []string{"pistol", "eco", "half", "full"}
	matchups, executes := []string{}, []string{}
	for i, b := range bands {
		for j, o := range bands[:3] {
			matchups = append(matchups, `{"buy":"`+b+`","opponent_buy":"`+o+`",`+group((i*3+j)*25, 25)+`,"sites":`+cells("A", "B", "both", "none")+`,"tempo":`+cells("rush", "set_strat", "mid_round", "no_site")+`}`)
		}
		for k := range 5 {
			executes = append(executes, `{"buy":"`+b+`","opponent_buy":"full","site":"A","route":"Hut",`+group(k*20, 20)+`,"rushes":{"pct":10},"plants":{"pct":50},"contact_seconds":20}`)
		}
	}
	defaults := []string{}
	for i := range 6 {
		defaults = append(defaults, fmt.Sprintf(`{"formation":"%d-1-2",%s,"zones":[{"zone":"Outside","label":"Outside",%s}]}`, i, group(i*40, 40), group(i*40, 20)))
	}
	players := []string{}
	for p := range 5 {
		windows := []string{}
		for w, span := range [][2]int{{0, 55}, {55, 75}, {75, 0}} {
			spots := []string{}
			for s := range 5 {
				spots = append(spots, fmt.Sprintf(`{"callout":"Spot%d","share":{"count":30,"pct":20},"refs":[%s],"arrival_seconds":10}`, s, strings.Join(refs[(w*5+s)*15:(w*5+s)*15+30], ",")))
			}
			windows = append(windows, fmt.Sprintf(`{"from_seconds":%d,"to_seconds":%d,"rounds":150,"spots":[%s],"pace":{}}`, span[0], span[1], strings.Join(spots, ",")))
		}
		players = append(players, fmt.Sprintf(`{"steamid64":"7656119800000000%d","name":"p%d","role":"rifler","rounds":150,"windows":[%s]}`, p, p, strings.Join(windows, ",")))
	}
	return `{"team_name":"Vitality","skipped":[],"map":"de_nuke","matches":20,"round_count":300,"framed":300,"days_since_last_match":3,` +
		`"sites":` + cells("A", "B", "both", "none") + `,"tempo":` + cells("rush", "set_strat", "mid_round", "no_site") +
		`,"matchups":[` + strings.Join(matchups, ",") + `],"defaults":[` + strings.Join(defaults, ",") + `],"executes":[` + strings.Join(executes, ",") +
		`],"players":[` + strings.Join(players, ",") + `],"warnings":[]}`
}

func TestTeamAttackStaysSmallOnABigLibrary(t *testing.T) {
	body := seededAttack()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/attack", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerTeamAttackTool(server, s) })
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "team_attack", Arguments: map[string]any{"team_id": testTeamID, "map": "de_nuke"}})
	if err != nil || res.IsError {
		t.Fatalf("team_attack = %+v, %v", res, err)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	// The read is about 600 KB of refs. Three section links of 200 rounds take some 23 KB of what is left.
	if len(raw) > 80_000 || len(body) < 500_000 {
		t.Errorf("team_attack output = %d bytes of a %d byte read, want at most 80000", len(raw), len(body))
	}
	var out teamAttackOut
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.AllRounds.Rounds != 240 || out.AllRounds.RoundsLinked != maxPlaylistRounds || len(query(t, out.AllRounds.StudioPath)["r"]) != maxPlaylistRounds {
		t.Errorf("all rounds = %d rounds, %d linked", out.AllRounds.Rounds, out.AllRounds.RoundsLinked)
	}
	if out.Sites[0].StudioPath == "" || out.Sites[1].StudioPath != "" || len(out.Sites[1].Examples) != maxExampleRefs {
		t.Errorf("sites = %+v, want the top row linked and the rest with 5 examples", out.Sites)
	}
}
