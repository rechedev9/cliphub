package httpapi

import (
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestLibraryTeamPlaybookReadsOneMapAndSide(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("playbook", "Vitality", "MOUZ")
	base := "/api/library/teams/" + teamOfMatch(t, m, "ct-start").TeamID.String()

	var pb playbookResponse
	env.json(http.MethodGet, base+"/playbook?map=de_mirage&side=CT", nil, http.StatusOK, &pb)
	if pb.TeamName != "Vitality" || pb.Map != "de_mirage" || pb.Side != tacticalplan.SideCT || pb.Matches != 1 || pb.RoundCount != 2 {
		t.Fatalf("playbook = %+v", pb.Playbook)
	}
	if pb.Framed != 2 || len(pb.Rounds) != 2 || pb.Rounds[0].MatchID != m.ID || pb.Rounds[0].Tick != 100 {
		t.Fatalf("playbook rounds = %+v, framed %d", pb.Rounds, pb.Framed)
	}
	for _, c := range pb.Categories {
		if c.Category == tacticalplan.PlaybookDefault && c.Rounds != 2 {
			t.Fatalf("default category = %+v, want both held rounds", c)
		}
	}
	if pb.Thresholds.RushSeconds != tacticalplan.PlaybookRushSeconds || pb.Thresholds.MapControlSeconds != tacticalplan.MapControlSeconds {
		t.Fatalf("thresholds = %+v", pb.Thresholds)
	}

	var other playbookResponse
	env.json(http.MethodGet, base+"/playbook?map=de_nuke&side=T", nil, http.StatusOK, &other)
	if other.Matches != 0 || len(other.Rounds) != 0 || len(other.Categories) != 6 {
		t.Fatalf("playbook on another map = %+v", other.Playbook)
	}
	for _, bad := range []string{"?map=de_mirage", "?side=T", "?map=de_mirage&map=de_nuke&side=T", "?map=de_mirage&side=mid"} {
		if rw := env.do(http.MethodGet, base+"/playbook"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET playbook%s = %d, want 400", bad, rw.Code)
		}
	}
}

func TestLibraryTeamPrepReadsBothSides(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("prep", "Vitality", "MOUZ")
	base := "/api/library/teams/" + teamOfMatch(t, m, "ct-start").TeamID.String()

	var prep prepResponse
	env.json(http.MethodGet, base+"/prep?map=de_mirage", nil, http.StatusOK, &prep)
	if prep.TeamName != "Vitality" || prep.Map != "de_mirage" || prep.RoundCount != 2 || len(prep.Skipped) != 0 {
		t.Fatalf("prep = %+v", prep.Prep)
	}
	if len(prep.Matchups) != 2 || prep.Matchups[0].Buy != tacticalplan.BuyPistol || prep.Matchups[1].OpponentBuy != tacticalplan.BuyEco {
		t.Fatalf("matchups = %+v, want pistol vs pistol then full vs eco", prep.Matchups)
	}
	if len(prep.Tempo) != 2 || len(prep.Thresholds.Windows) != len(tacticalplan.PrepWindows) {
		t.Fatalf("tempo = %+v, thresholds = %+v", prep.Tempo, prep.Thresholds)
	}
	// The fixture map has no callouts, so no position can be named.
	if prep.CTPlayers == nil || len(prep.CTPlayers) != 0 {
		t.Fatalf("CT players = %+v, want an empty list", prep.CTPlayers)
	}
	for _, bad := range []string{"", "?map=de_mirage&side=CT", "?map=Mirage"} {
		if rw := env.do(http.MethodGet, base+"/prep"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET prep%s = %d, want 400", bad, rw.Code)
		}
	}
}
