package main

import (
	"net/http"
	"testing"
)

// addFirstDeathRoutes serves the positions until the first death of fakeAnalysisStudio.
func addFirstDeathRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/first-death", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("side") != "T" || q.Get("map") != "de_nuke" || q.Get("t_from") != "0" || q.Get("t_to") != "15" || q.Get("movement") != "slow" {
			http.Error(w, `{"error":"unexpected query `+r.URL.RawQuery+`"}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"team_name":"Vitality","skipped":[],"map":"de_nuke","side":"T","window":{"from_seconds":0,"to_seconds":15},
			"movement":"slow","matches":1,"round_count":12,"framed":10,"players":[{"steamid64":"` + testSteamID + `","name":"ZywOo","role":"awp","rounds":10,
			"zones":[{"callout":"Outside","seconds":1.3,"rounds":7,"refs":[{"match_id":"` + testMatchID + `","round":2,"tick":100}],"samples":[[1,2,3]]}]}]}`))
	})
}

func TestTeamPrepReadsFirstDeathAndWarnings(t *testing.T) {
	cs := fakeAnalysisStudio(t)
	var prep teamPrepOut
	callTool(t, cs, "team_prep", map[string]any{"team_id": testTeamID, "map": "de_nuke", "side": "t", "movement": "slow"}, &prep)
	if len(prep.Warnings) != 1 || prep.Warnings[0].Kind != "small_sample" || prep.Warnings[0].Message != "Small sample: 22 rounds." {
		t.Errorf("warnings = %+v", prep.Warnings)
	}
	fd := prep.FirstDeath
	if fd == nil || fd.Window != "1:55 to 1:40" || fd.FramedRounds != 10 || len(fd.Players) != 1 {
		t.Fatalf("first death = %+v", fd)
	}
	z := fd.Players[0].Zones
	if fd.Players[0].RoundsPlayed != 10 || len(z) != 1 || z[0].Rank != 1 || z[0].Seconds != 1.3 || z[0].Refs[0] != testMatchID+"/2" {
		t.Errorf("player = %+v", fd.Players[0])
	}
	var plain teamPrepOut
	callTool(t, cs, "team_prep", map[string]any{"team_id": testTeamID, "map": "de_nuke"}, &plain)
	if plain.FirstDeath != nil {
		t.Errorf("first death without side = %+v", plain.FirstDeath)
	}
	toolFails(t, cs, "team_prep", map[string]any{"team_id": testTeamID, "map": "de_nuke", "movement": "fast"}, "need side")
	toolFails(t, cs, "team_prep", map[string]any{"team_id": testTeamID, "map": "de_nuke", "side": "T", "movement": "crouch"}, "movement must be")
}
