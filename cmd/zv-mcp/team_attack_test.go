package main

import (
	"context"
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
	if m := out.Matchups[0]; len(m.Sites) != 1 || m.Sites[0].Key != "A" || !strings.Contains(query(t, m.Sites[0].StudioPath).Get("t"), "compra completa contra compra completa") {
		t.Errorf("matchup = %+v, want the empty B cell left out", m)
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
