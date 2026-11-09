package httpapi

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
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

func overLimit(skipped []scoutSkipped) int {
	n := 0
	for _, s := range skipped {
		if s.Reason == scoutSkipBeyondLimits {
			n++
		}
	}
	return n
}

func TestTheMatchCapCountsOnlyTheMatchesTheLineupKeeps(t *testing.T) {
	f := newScoutFixture(t)
	team := "/api/library/teams/" + f.vitality.String()
	patch := func(m library.Match, at time.Time) {
		f.env.json(http.MethodPatch, "/api/library/matches/"+m.ID.String(), map[string]any{"played_at": at.Format(time.RFC3339)}, http.StatusOK, nil)
	}
	// Newer than the fixture's two: the cap's worth of home matches player two never played.
	start := time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC)
	for i := range library.MaxReportMatches {
		patch(f.env.seedLibraryMatch(fmt.Sprintf("cap-%d", i), "Vitality", "MOUZ"), start.Add(time.Duration(i)*time.Hour))
	}
	patch(f.home, start.Add(-48*time.Hour))
	patch(f.away, start.Add(-24*time.Hour))

	every := f.scout("")
	if len(every.Matches) != library.MaxReportMatches || overLimit(every.Skipped) != 2 {
		t.Fatalf("scout = %d matches, %d over the cap; want %d and the 2 oldest skipped", len(every.Matches), overLimit(every.Skipped), library.MaxReportMatches)
	}
	two := "?lineup=" + fixturePlayerTwo
	if scout := f.scout(two); len(scout.Matches) != 1 || scout.Matches[0].MatchID != f.away.ID || overLimit(scout.Skipped) != 0 {
		t.Fatalf("scout with player two = %+v (skipped %+v), want the away match", scout.Matches, scout.Skipped)
	}
	var prep prepResponse
	f.env.json(http.MethodGet, team+"/prep?map=de_mirage&lineup="+fixturePlayerTwo, nil, http.StatusOK, &prep)
	if prep.Matches != 1 || overLimit(prep.Skipped) != 0 {
		t.Fatalf("prep with player two = %d matches (skipped %+v), want the away match", prep.Matches, prep.Skipped)
	}
	var heat spatialResponse
	f.env.json(http.MethodGet, team+"/heatmap"+two, nil, http.StatusOK, &heat)
	if len(heat.Matches) != 1 || overLimit(heat.Skipped) != 0 {
		t.Fatalf("heatmap with player two = %+v (skipped %+v), want the away match", heat.Matches, heat.Skipped)
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
