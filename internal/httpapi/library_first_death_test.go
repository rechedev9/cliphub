package httpapi

import (
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestLibraryTeamFirstDeathReadsOneSide(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("first-death", "Vitality", "MOUZ")
	base := "/api/library/teams/" + teamOfMatch(t, m, "ct-start").TeamID.String()

	var fd firstDeathResponse
	env.json(http.MethodGet, base+"/first-death?map=de_mirage&side=CT&t_from=0&t_to=15", nil, http.StatusOK, &fd)
	if fd.TeamName != "Vitality" || fd.Side != tacticalplan.SideCT || fd.Movement != tacticalplan.MovementStatic || fd.Window.ToSeconds != 15 {
		t.Fatalf("first death = %+v", fd.FirstDeathPositions)
	}
	if fd.Players == nil || fd.Thresholds.ZoneLimit != tacticalplan.FirstDeathZoneLimit {
		t.Fatalf("players = %+v, thresholds = %+v", fd.Players, fd.Thresholds)
	}
	for _, bad := range []string{"?map=de_mirage", "?map=de_mirage&side=T&movement=crouch", "?map=de_mirage&side=T&t_from=30&t_to=15", "?side=T"} {
		if rw := env.do(http.MethodGet, base+"/first-death"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET first-death%s = %d, want 400", bad, rw.Code)
		}
	}

	// The fixture's two rounds are a small sample for the prep and for the scout that Preparar reads.
	var prep prepResponse
	env.json(http.MethodGet, base+"/prep?map=de_mirage", nil, http.StatusOK, &prep)
	small := tacticalplan.DataWarning{Kind: tacticalplan.DataWarningSmallSample, Rounds: 2}
	if len(prep.Warnings) == 0 || prep.Warnings[len(prep.Warnings)-1] != small {
		t.Fatalf("prep warnings = %+v", prep.Warnings)
	}
	var scout scoutResponse
	env.json(http.MethodGet, base+"/scout?map=de_mirage", nil, http.StatusOK, &scout)
	if len(scout.Warnings) == 0 || scout.Warnings[len(scout.Warnings)-1] != small {
		t.Fatalf("scout warnings = %+v", scout.Warnings)
	}
}
