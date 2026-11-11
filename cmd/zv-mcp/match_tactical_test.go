package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testJobID = "5e6f7a8b-9cad-4ebf-a031-4c5d6e7f8091"

func matchTacticalStudio(t *testing.T) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/matches/{id}/tactical", `{"demo":{"map":"de_nuke","tickrate":64},
		"players":[{"slot":0,"steamid64":"`+testSteamID+`","name":"ZywOo","team_key":"ct-start"},{"slot":5,"steamid64":"76561198000000002","name":"torzsi","team_key":"t-start"}],
		"rounds":[{"number":2,"tick_freeze_end":1000,"tick_end":4200,"score_ct_before":1,"score_t_before":0,"winner":"CT","end_reason":"t_killed",
		"bomb":{"plant_tick":3200,"site":"A","planter_slot":5,"defuse_tick":4100},
		"timeouts":[{"side":"T","tick":200,"end_tick":680,"technical":true}],
		"economy":{"ct_equip_value":4000,"t_equip_value":22000,"ct_buy":"eco","t_buy":"full"},
		"class":{"t_side":"execute","ct_side":"retake","site":"A","tags":["anti_eco"],"reasons":["plant at 34 s"]},
		"players":[{"slot":0,"side":"CT","kills":1,"damage":100,"equip_value":800,"survived":true},{"slot":5,"side":"T","deaths":1,"opening_death":true}],
		"events":[{"tick":1640,"kind":"kill","actor_slot":0,"target_slot":5,"weapon":"usp_silencer","place":"Ramp","headshot":true,"opening":true},{"tick":1700,"kind":"smoke"}]}]}`)
	rec.handle(mux, "GET /api/library/matches/{id}/metrics", `{"players":[{"steamid64":"`+testSteamID+`","name":"ZywOo","rounds":10,"kills":10,"deaths":5,"box":{"damage":900}}],
		"timelines":[{"round":2,"winner":"CT","points":[{"seconds":0,"kind":"start","probability_ct":0.2},
		{"seconds":10,"kind":"kill","actor_slot":0,"target_slot":5,"probability_ct":0.35},
		{"seconds":20,"kind":"kill","actor_slot":5,"target_slot":0,"probability_ct":0.05},{"seconds":60,"kind":"end","probability_ct":1}]}]}`)
	rec.handle(mux, "GET /api/library/matches/{id}/tactical/aggregate", `{"round_count":12,"wins":9,"buys":[{"buy":"full","rounds":10,"share":{"pct":83.3},"win_rate":{"pct":80,"reliable":true}}]}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerMatchTacticalTools(server, s) })
	return cs, rec
}

func TestGetRoundNamesSlotsAndTimesFromFreezeEnd(t *testing.T) {
	cs, _ := matchTacticalStudio(t)
	var r roundOut
	callTool(t, cs, "get_round", map[string]any{"match_id": testMatchID, "round": 2}, &r)
	if r.ScoreBefore != "1-0" || r.Seconds != 50 || r.TPattern != "execute" || r.Reasons[0] != "plant at 34 s" {
		t.Fatalf("round = %+v", r)
	}
	if r.Bomb == nil || r.Bomb.Planter != "torzsi" || r.Bomb.PlantSecond != 34.4 || !r.Bomb.Defused || r.Bomb.Exploded {
		t.Errorf("bomb = %+v", r.Bomb)
	}
	if len(r.Kills) != 1 || r.Kills[0].Killer != "ZywOo" || r.Kills[0].Victim != "torzsi" || r.Kills[0].Second != 10 || !r.Kills[0].Opening {
		t.Errorf("kills = %+v, want the one kill, timed from freeze end", r.Kills)
	}
	if len(r.Players) != 2 || r.Players[1].Name != "torzsi" || !r.Players[1].OpeningDeath {
		t.Errorf("players = %+v", r.Players)
	}
	if len(r.Timeouts) != 1 || r.Timeouts[0].Side != "T" || r.Timeouts[0].FromSecond != -12.5 || r.Timeouts[0].ToSecond != -5 || !r.Timeouts[0].Technical {
		t.Errorf("timeouts = %+v, want the T technical timeout in the freeze time", r.Timeouts)
	}
	if len(r.WinTimeline) != 4 || r.WinTimeline[1].Event != "ZywOo > torzsi" || r.WinTimeline[1].CTPct != 35 {
		t.Errorf("timeline = %+v", r.WinTimeline)
	}
	toolFails(t, cs, "get_round", map[string]any{"match_id": testMatchID, "round": 7}, "has no round 7")
}

func TestMatchMetricsFindsTheBiggestSwing(t *testing.T) {
	cs, _ := matchTacticalStudio(t)
	var m matchMetricsOut
	callTool(t, cs, "match_metrics", map[string]any{"match_id": testMatchID}, &m)
	if len(m.Players) != 1 || m.Players[0].ADR != 90 || len(m.Rounds) != 1 {
		t.Fatalf("metrics = %+v", m)
	}
	r := m.Rounds[0]
	if r.CTStart != 20 || r.BigSwing == nil || r.BigSwing.Event != "torzsi > ZywOo" || r.BigSwing.DeltaPct != -30 {
		t.Errorf("round = %+v %+v, want the -30 point kill, not the round end", r, r.BigSwing)
	}
}

func TestMatchTendencies(t *testing.T) {
	cs, rec := matchTacticalStudio(t)
	var tend tendenciesOut
	callTool(t, cs, "match_tendencies", map[string]any{"match_id": testMatchID, "team_key": "t-start", "buy": []string{"full", "force"}, "result": "lost", "round_from": 13}, &tend)
	got := rec.get("GET /api/library/matches/{id}/tactical/aggregate")
	for _, want := range []string{"team=t-start", "buy=full&buy=force", "outcome=loss", "round_from=13"} {
		if !strings.Contains(got, want) {
			t.Errorf("query = %s, want %s", got, want)
		}
	}
	if tend.Rounds != 12 || tend.RoundWinPct != 75 || len(tend.Buys) != 1 {
		t.Errorf("tendencies = %+v", tend)
	}
	toolFails(t, cs, "match_tendencies", map[string]any{"match_id": testMatchID, "team_key": "t-start", "round_from": 10, "round_to": 5}, "in order")
	toolFails(t, cs, "match_tendencies", map[string]any{"match_id": testMatchID, "team_key": "T"}, "team_key")

}
