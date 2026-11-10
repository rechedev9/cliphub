package main

import (
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testProMetaJSON = `{"version":1,"map":"de_nuke","recent_days":90,"fresh":true,
	"rounds_t":{"recent":200,"all":800},"rounds_ct":{"recent":200,"all":800},
	"lineups":[
	{"id":"old","title":"Humo a Outside","kind":"smoke","side":"T","land_place":"Outside","throws":{"recent":10,"all":200},"round_share":0.05,"trend":[2,6],"team_count":9},
	{"id":"new","title":"Humo a Heaven","kind":"smoke","side":"T","land_place":"Heaven","throws":{"recent":30,"all":30},"round_share":0.15,"trend":[12,0],"team_count":2,
	 "teams":[{"name":"FURIA","count":9,"detail":"45"},{"name":"MOUZ","count":4,"detail":"20"},{"name":"G2","count":2,"detail":"10"},{"name":"FaZe","count":1}]},
	{"id":"flash","title":"Flash a Ramp","kind":"flash","side":"T","throws":{"recent":20,"all":60},"trend":[10,9.5]},
	{"id":"gone","title":"Molotov a Hut","kind":"molotov","side":"T","throws":{"recent":0,"all":40}},
	{"id":"ct","title":"Humo a Ramp","kind":"smoke","side":"CT","throws":{"recent":50,"all":90}}]}`

func TestProGrenadeMetaRanksASidesGrenades(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/library/pro-lineups/{map}", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(testProMetaJSON)) })
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerProGrenadeMetaTool(server, s) })

	var recent proGrenadeMetaOut
	callTool(t, cs, "pro_grenade_meta", map[string]any{"map": "de_nuke", "side": "t"}, &recent)
	if recent.Total != 3 || recent.SideRounds != 200 || recent.PeriodDays != 90 || len(recent.Grenades) != 3 {
		t.Fatalf("recent = %+v, want the 3 T grenades thrown lately over 200 rounds", recent)
	}
	top := recent.Grenades[0]
	if top.ID != "new" || top.Rank != 1 || top.Per100 != 15 || top.Trend.Label != "new" || len(top.Teams) != 3 || top.Teams[0].Pct != "45" {
		t.Errorf("top = %+v, want the new Heaven smoke first with its teams", top)
	}
	if recent.Grenades[1].Trend.Label != "steady" || recent.Grenades[2].Trend.Label != "falling" {
		t.Errorf("trends = %+v", recent.Grenades)
	}
	if len(recent.Entering) != 1 || recent.Entering[0] != "new" {
		t.Errorf("entering = %v", recent.Entering)
	}

	var all proGrenadeMetaOut
	callTool(t, cs, "pro_grenade_meta", map[string]any{"map": "de_nuke", "side": "T", "period": "all", "kind": "smoke", "limit": 1}, &all)
	if all.Total != 2 || len(all.Grenades) != 1 || !all.Truncated || all.Grenades[0].ID != "old" || all.Grenades[0].Per100 != 25 || all.PeriodDays != 0 {
		t.Errorf("all time smokes = %+v, want the old Outside smoke on top", all)
	}
	toolFails(t, cs, "pro_grenade_meta", map[string]any{"map": "de_nuke", "side": " "}, "side is required")
	toolFails(t, cs, "pro_grenade_meta", map[string]any{"map": "de_nuke", "side": "T", "period": "week"}, "period must be")
}

func TestMetaTrendNeedsARealMove(t *testing.T) {
	for _, c := range []struct {
		now, before float64
		want        string
	}{{3, 0, "new"}, {0.4, 0, "steady"}, {0, 0, "steady"}, {9, 6, "rising"}, {1.2, 0.6, "steady"}, {2, 6, "falling"}, {6, 6.5, "steady"}} {
		if got := metaTrend(c.now, c.before); got != c.want {
			t.Errorf("metaTrend(%v, %v) = %s, want %s", c.now, c.before, got, c.want)
		}
	}
}
