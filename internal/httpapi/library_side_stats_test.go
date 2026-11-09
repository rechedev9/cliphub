package httpapi

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestLibraryTeamSideStatsComparesWithTheLibraryOrChosenTeams(t *testing.T) {
	f := newScoutFixture(t)
	base := "/api/library/teams/" + f.vitality.String() + "/side-stats"

	var resp sideStatsResponse
	f.env.json(http.MethodGet, base, nil, http.StatusOK, &resp)
	if resp.TeamName != "Vitality" || resp.Matches != 2 || resp.Rounds != 4 || len(resp.MatchList) != 2 || resp.Compare != "library" {
		t.Fatalf("side stats = %s, %d matches, %d rounds, compare %s", resp.TeamName, resp.Matches, resp.Rounds, resp.Compare)
	}
	// Every other team of the library: MOUZ, Spirit, NAVI and G2.
	if resp.GroupTeams != 4 || resp.Overall.RoundsWon.GroupAvg == nil || resp.Overall.RoundsWon.GroupTeams != 4 {
		t.Fatalf("library group = %d teams, rounds won %+v", resp.GroupTeams, resp.Overall.RoundsWon)
	}
	if len(resp.Players) == 0 || resp.RatingFormula == "" {
		t.Fatalf("players = %+v, formula %q", resp.Players, resp.RatingFormula)
	}

	f.env.json(http.MethodGet, base+"?side=ct&compare_team="+f.spirit.String(), nil, http.StatusOK, &resp)
	if resp.Rounds != 2 || resp.Compare != "teams" || resp.GroupTeams != 1 || len(resp.CompareTeams) != 1 || resp.CompareTeams[0].Name != "Spirit" {
		t.Fatalf("CT against Spirit = %d rounds, compare %s %+v, %d group teams", resp.Rounds, resp.Compare, resp.CompareTeams, resp.GroupTeams)
	}
	if resp.Overall.MapsWon.Value == nil || resp.CT.MapsWon.Value != nil {
		t.Fatalf("maps won = overall %v, CT %v", resp.Overall.MapsWon.Value, resp.CT.MapsWon.Value)
	}

	f.env.json(http.MethodGet, base+"?map=de_nuke", nil, http.StatusOK, &resp)
	if resp.Matches != 0 || resp.Overall.RoundsWon.Value != nil || resp.GroupTeams != 0 || len(resp.Players) != 0 {
		t.Fatalf("nuke = %d matches, rounds won %v", resp.Matches, resp.Overall.RoundsWon.Value)
	}

	for _, bad := range []string{"?side=mid", "?buy=rich", "?opponent_buy=x", "?compare_team=nope"} {
		if rw := f.env.do(http.MethodGet, base+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("side stats%s = %d, want 400", bad, rw.Code)
		}
	}
	if rw := f.env.do(http.MethodGet, base+"?compare_team="+uuid.NewString(), nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("an unknown compare team = %d, want 404", rw.Code)
	}
}
