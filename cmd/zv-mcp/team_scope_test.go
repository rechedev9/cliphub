package main

import (
	"net/url"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestTeamRoundScopeReplayFilters(t *testing.T) {
	in := teamRoundScopeIn{Site: []string{"A"}, Outcome: "WIN", PostPlant: true, Phase: "overtime", SecondRound: true,
		EndReason: []string{"bomb_exploded"}, Weapon: "hero_awp", OpponentWeapon: "no_scout", EquipFrom: 1000, EquipTo: 3250,
		OpponentEquipFrom: 4000, Tactic: "0b7b1f0e-6a4c-4a36-9a2b-6c1f5f0e8d11"}
	q, err := in.apply(tuiclient.AnalysisQuery{})
	if err != nil {
		t.Fatal(err)
	}
	want := url.Values{"site": {"a"}, "outcome": {"win"}, "tag": {"postplant"}, "phase": {"overtime"}, "second_round": {"true"},
		"end_reason": {"bomb_exploded"}, "weapon": {"hero_awp"}, "opponent_weapon": {"no_scout"}, "equip_from": {"1000"},
		"equip_to": {"3250"}, "opponent_equip_from": {"4000"}, "tactic": {"0b7b1f0e-6a4c-4a36-9a2b-6c1f5f0e8d11"}}
	if q.Scope.Encode() != want.Encode() {
		t.Fatalf("scope = %s, want %s", q.Scope.Encode(), want.Encode())
	}
	if !in.teamOnly() {
		t.Fatal("the Replay filters are team scope keys")
	}
	for name, bad := range map[string]teamRoundScopeIn{
		"weapon":  {Weapon: "knife"},
		"equip":   {EquipFrom: 4000, EquipTo: 1000},
		"reason":  {EndReason: []string{"surrender"}},
		"tactic":  {Tactic: "exec-a"},
		"outcome": {Outcome: "draw"},
	} {
		if _, err := bad.apply(tuiclient.AnalysisQuery{}); err == nil {
			t.Errorf("%s: applied without an error", name)
		}
	}
	path, err := teamReplayPath(replayPathIn{TeamID: "t1", Query: q})
	if err != nil {
		t.Fatal(err)
	}
	if u, _ := url.Parse(path); u.Query().Get("equip_to") != "3250" || u.Query().Get("tag") != "postplant" || u.Query().Get("tactic") == "" {
		t.Fatalf("studio_path %s lost a filter", path)
	}
	q.Scope["site"] = []string{"a", "b"}
	if _, err := teamReplayPath(replayPathIn{TeamID: "t1", Query: q}); err == nil {
		t.Fatal("the Replay holds one site")
	}
}
