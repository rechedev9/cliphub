package httpapi

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// In the scout fixture Vitality fields player one at home and player two away.
const (
	fixturePlayerOne = "76561198000000001"
	fixturePlayerTwo = "76561198000000002"
)

func TestLibraryTeamLineupsGroupTheLatestMatchFirst(t *testing.T) {
	f := newScoutFixture(t)
	f.env.json(http.MethodPatch, "/api/library/matches/"+f.home.ID.String(), map[string]any{"played_at": "2026-10-01T18:00:00Z"}, http.StatusOK, nil)
	f.env.json(http.MethodPatch, "/api/library/matches/"+f.away.ID.String(), map[string]any{"played_at": "2026-11-01T18:00:00Z"}, http.StatusOK, nil)

	var resp teamLineupsResponse
	f.env.json(http.MethodGet, "/api/library/teams/"+f.vitality.String()+"/lineups", nil, http.StatusOK, &resp)
	if resp.LatestMatch == nil || resp.LatestMatch.MatchID != f.away.ID {
		t.Fatalf("latest match = %+v, want the away match", resp.LatestMatch)
	}
	if len(resp.Latest) != 1 || resp.Latest[0].SteamID64 != fixturePlayerTwo || len(resp.Others) != 1 || resp.Others[0].SteamID64 != fixturePlayerOne {
		t.Fatalf("groups = latest %+v, others %+v; want player two, then player one", resp.Latest, resp.Others)
	}
	if rw := f.env.do(http.MethodGet, "/api/library/teams/"+uuid.NewString()+"/lineups", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("lineups of an unknown team = %d, want 404", rw.Code)
	}
}

func TestLibraryTeamViewsDropTheMatchesTheLineupNeverPlayed(t *testing.T) {
	f := newScoutFixture(t)
	team := "/api/library/teams/" + f.vitality.String()

	// Every home round has player one for Vitality, so a lineup of player two leaves only the away match.
	scout := f.scout("?lineup=" + fixturePlayerTwo)
	if len(scout.Matches) != 1 || scout.Matches[0].MatchID != f.away.ID || len(scout.Filter.Lineup) != 1 {
		t.Fatalf("scout with player two = %+v (filter %+v), want only the away match", scout.Matches, scout.Filter)
	}
	var sites bombsitesResponse
	f.env.json(http.MethodGet, team+"/bombsites?lineup="+fixturePlayerTwo, nil, http.StatusOK, &sites)
	if sites.MatchCount != 1 {
		t.Fatalf("bombsites with player two = %d matches, want 1", sites.MatchCount)
	}
	var heat spatialResponse
	f.env.json(http.MethodGet, team+"/heatmap?lineup="+fixturePlayerTwo, nil, http.StatusOK, &heat)
	if len(heat.Matches) != 1 || heat.Matches[0].MatchID != f.away.ID {
		t.Fatalf("heatmap with player two = %+v, want the away match", heat.Matches)
	}
	if all := f.scout("?lineup=" + fixturePlayerOne + "&lineup=" + fixturePlayerTwo); len(all.Matches) != 2 || all.Tendencies.RoundCount != 4 {
		t.Fatalf("scout with both players = %d matches, %d rounds; want everything", len(all.Matches), all.Tendencies.RoundCount)
	}
	if rw := f.env.do(http.MethodGet, team+"/scout?lineup=nope", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("scout with a bad lineup = %d, want 400", rw.Code)
	}
}
