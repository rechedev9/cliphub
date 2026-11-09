package httpapi

import (
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestLibraryTeamAttackReadsTheTSide(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("attack", "Vitality", "MOUZ")
	base := "/api/library/teams/" + teamOfMatch(t, m, "t-start").TeamID.String()

	var attack attackResponse
	env.json(http.MethodGet, base+"/attack?map=de_mirage", nil, http.StatusOK, &attack)
	if attack.TeamName != "MOUZ" || attack.Map != "de_mirage" || attack.Matches != 1 || attack.RoundCount != 2 || len(attack.Skipped) != 0 {
		t.Fatalf("attack = %+v", attack.AttackPrep)
	}
	if len(attack.Sites) != len(tacticalplan.AttackSites) || len(attack.Tempo) != len(tacticalplan.AttackTempos) || attack.LastPlayedAt == nil {
		t.Fatalf("sites %+v, tempo %+v, last played %v", attack.Sites, attack.Tempo, attack.LastPlayedAt)
	}
	if len(attack.Warnings) == 0 || attack.Warnings[0].Kind != tacticalplan.WarningFewRounds {
		t.Fatalf("warnings = %+v, want the thin sample first", attack.Warnings)
	}
	for _, bad := range []string{"", "?map=de_mirage&side=T", "?map=de_mirage&map=de_nuke"} {
		if rw := env.do(http.MethodGet, base+"/attack"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET attack%s = %d, want 400", bad, rw.Code)
		}
	}
}
