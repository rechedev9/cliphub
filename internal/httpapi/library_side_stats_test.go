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
	// Every other team of the library (MOUZ, Spirit, NAVI, G2) over the 3 matches; 2 rounds each are too few for a mean.
	if resp.GroupTeams != 4 || resp.GroupMatches != 3 || resp.GroupCapped || resp.Overall.RoundsWon.GroupAvg != nil || resp.GroupMinRounds != 20 {
		t.Fatalf("library group = %d teams over %d matches, rounds won %+v", resp.GroupTeams, resp.GroupMatches, resp.Overall.RoundsWon)
	}
	if len(resp.Players) == 0 || resp.RatingFormula == "" {
		t.Fatalf("players = %+v, formula %q", resp.Players, resp.RatingFormula)
	}

	stale := uuid.NewString()
	query := "?side=ct&compare_team=" + f.spirit.String() + "&compare_team=" + f.spirit.String() + "&compare_team=" + stale + "&compare_team=" + f.vitality.String()
	f.env.json(http.MethodGet, base+query, nil, http.StatusOK, &resp)
	if resp.Rounds != 2 || resp.Compare != "teams" || resp.GroupTeams != 1 || len(resp.CompareTeams) != 1 || resp.CompareTeams[0].Name != "Spirit" {
		t.Fatalf("CT against Spirit = %d rounds, compare %s %+v, %d group teams", resp.Rounds, resp.Compare, resp.CompareTeams, resp.GroupTeams)
	}
	if len(resp.CompareSkipped) != 1 || resp.CompareSkipped[0].String() != stale {
		t.Fatalf("compare skipped = %v, want the unknown team only", resp.CompareSkipped)
	}
	if resp.Overall.MapsWon.Value == nil || resp.CT.MapsWon.Value != nil {
		t.Fatalf("maps won = overall %v, CT %v", resp.Overall.MapsWon.Value, resp.CT.MapsWon.Value)
	}

	f.env.json(http.MethodGet, base+"?map=de_nuke", nil, http.StatusOK, &resp)
	if resp.Matches != 0 || resp.Overall.RoundsWon.Value != nil || resp.GroupTeams != 0 || len(resp.Players) != 0 {
		t.Fatalf("nuke = %d matches, rounds won %v", resp.Matches, resp.Overall.RoundsWon.Value)
	}

	// The team scope keys apply as on every team view: the last match only, its rounds from 2 on.
	f.env.json(http.MethodGet, base+"?max_matches=1&round_from=2", nil, http.StatusOK, &resp)
	if resp.Matches != 1 || resp.Rounds != 1 {
		t.Fatalf("last match from round 2 = %d matches, %d rounds", resp.Matches, resp.Rounds)
	}

	for _, bad := range []string{"?side=mid", "?max_matches=0", "?whose=mine", "?buy=rich", "?opponent_buy=x", "?compare_team=nope"} {
		if rw := f.env.do(http.MethodGet, base+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("side stats%s = %d, want 400", bad, rw.Code)
		}
	}
}
